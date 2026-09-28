import { exists } from "@std/fs";
import { join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { listFiles, readSourceText } from "../shared/fs.ts";
import type { CompiledModule } from "../yue/compile.ts";

export type ModuleKind = "yue" | "lua";

/** Modules provided by the runtime rather than by src/ or lua/; no project module may take their names. */
export const BUILTIN_MODULES: ReadonlySet<string> = new Set(["moonwell"]);

/** The end of a hint for a library's file, which is not the project's to rename. */
const NARROW_DIR = "narrow the library's `dir` in moonwell.pkl so it leaves this file out.";

/** A gameplay module on disk (spec §5.1). */
export interface SourceModule {
  /** Dotted name, e.g. "utils.timer". */
  name: string;
  /** POSIX path relative to the project root, e.g. "lua/utils/timer.lua". */
  path: string;
  kind: ModuleKind;
  /** A Lua module's text; YueScript modules are read when they are compiled. */
  source?: string;
  /** The library key for a library's module. */
  library?: string;
}

/** A folder of modules, relative to the project root, and the kind of file it holds. */
export interface ModuleRoot {
  /** POSIX path relative to the project root, e.g. "src". */
  dir: string;
  kind: ModuleKind;
  /** Whether a missing folder is an error. */
  required: boolean;
  /** The library key when the folder is a library's (spec §4.4). */
  library?: string;
}

/** The project's own modules: YueScript in src/ and Lua in lua/ (spec §3.1). src/**\/*.lua is the editor's output. */
export const PROJECT_MODULE_ROOTS: readonly ModuleRoot[] = [
  { dir: "src", kind: "yue", required: true },
  { dir: "lua", kind: "lua", required: false },
];

/**
 * Lists every module under `roots`, in root order and then by path. Fails on a dotted file or folder name (spec §3.1),
 * on two files with one name (spec §3.2) and on a file that takes a built-in module's name. In a library root, a `.lua`
 * file beside a `.yue` file of the same stem is that module's compiled output and is skipped (spec §4.4). Lua sources
 * are read as Lua's loadfile reads them (`readSourceText`).
 */
export async function collectModules(
  root: string,
  roots: readonly ModuleRoot[] = PROJECT_MODULE_ROOTS,
): Promise<SourceModule[]> {
  const modules: SourceModule[] = [];
  const byName = new Map<string, SourceModule>();
  for (const moduleRoot of roots) {
    const dir = join(root, ...moduleRoot.dir.split("/"));
    if (!(await exists(dir, { isDirectory: true }))) {
      if (moduleRoot.required) throw new MoonwellError(`The ${moduleRoot.dir}/ folder is missing.`, { file: root });
      continue;
    }
    const extension = `.${moduleRoot.kind}`;
    const files = await listFiles(dir);
    // In a library, YueScript and Lua share one folder: a .lua beside a .yue of the same stem is its compiled output.
    const compiled = moduleRoot.library !== undefined && moduleRoot.kind === "lua"
      ? new Set(files.filter((file) => file.endsWith(".yue")).map((file) => `${file.slice(0, -".yue".length)}.lua`))
      : new Set<string>();
    for (const file of files.filter((path) => path.endsWith(extension) && !compiled.has(path))) {
      const path = `${moduleRoot.dir}/${file}`;
      const stem = file.slice(0, -extension.length);
      if (stem.split("/").some((segment) => segment.includes("."))) {
        throw new MoonwellError("Module file and folder names cannot contain dots.", {
          file: path,
          hint: `Dots separate module names in \`import\`; ${
            moduleRoot.library === undefined ? "rename the file or folder." : NARROW_DIR
          }`,
        });
      }
      const module: SourceModule = { name: stem.split("/").join("."), path, kind: moduleRoot.kind };
      if (moduleRoot.library !== undefined) module.library = moduleRoot.library;
      for (const name of claimedNames(module.name)) {
        if (BUILTIN_MODULES.has(name)) {
          if (module.library !== undefined) {
            throw new MoonwellError(`Module ${name} is built into Moonwell; ${path} takes its name.`, {
              file: path,
              hint: `\`require\` of a built-in name always loads the built-in module; ${NARROW_DIR}`,
            });
          }
          throw new MoonwellError(`Module ${name} is built into Moonwell; rename ${path}.`, {
            file: path,
            hint: "`require` of a built-in name always loads the built-in module, never a project file.",
          });
        }
        const clash = byName.get(name);
        if (clash !== undefined) {
          throw new MoonwellError(`Module ${name} is defined by ${clash.path} and ${path}.`, {
            file: path,
            hint: module.library !== undefined || clash.library !== undefined
              ? "Rename one of them, or narrow the library's `dir`: module names are shared by src/, lua/ and libraries."
              : "Rename one of them: module names are shared by src/ and lua/.",
          });
        }
        byName.set(name, module);
      }
      if (module.kind === "lua") module.source = await readSourceText(join(dir, ...file.split("/")), path);
      modules.push(module);
    }
  }
  return modules;
}

/** A YueScript and a Lua root over `.moonwell/libraries/<key>` for each library, in key order (spec §4.4). */
export function libraryModuleRoots(keys: readonly string[]): ModuleRoot[] {
  return [...keys].sort().flatMap((key) =>
    (["yue", "lua"] as const).map((kind) => ({
      dir: `.moonwell/libraries/${key}`,
      kind,
      required: false,
      library: key,
    }))
  );
}

/** The names a module answers to: its own and, for `<parent>.init`, `<parent>` too (`moduleLoader`, spec §3.2). */
function claimedNames(name: string): string[] {
  return name.endsWith(".init") ? [name, name.slice(0, -".init".length)] : [name];
}

/**
 * Resolves `require` names for the bundler: the name, then `<name>.init`, Lua's `?/init.lua` convention (spec §3.2).
 * A Lua module is its own source; a YueScript module is its compiled output, which `loadCompiled` gives for the
 * module (from src/ or a library). Either is returned under the name it was required by, which is the name the bundle
 * defines it with.
 */
export function moduleLoader(
  modules: readonly SourceModule[],
  loadCompiled: (module: SourceModule) => CompiledModule | undefined,
): (name: string) => CompiledModule | undefined {
  const byName = new Map(modules.map((module) => [module.name, module]));
  return (name) => {
    const module = byName.get(name) ?? byName.get(`${name}.init`);
    if (module === undefined) return undefined;
    if (module.kind === "lua") return { name, sourcePath: module.path, source: module.source ?? "", kind: "lua" };
    const compiled = loadCompiled(module);
    return compiled && { ...compiled, name };
  };
}
