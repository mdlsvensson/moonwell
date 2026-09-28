import { exists } from "@std/fs";
import { join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { listFiles, readSourceText } from "../shared/fs.ts";
import type { CompiledModule } from "../yue/compile.ts";

export type ModuleKind = "yue" | "lua";

/** Modules provided by the runtime rather than by src/ or lua/; no project module may take their names. */
export const BUILTIN_MODULES: ReadonlySet<string> = new Set(["moonwell"]);

/** A gameplay module on disk (spec §5.1). */
export interface SourceModule {
  /** Dotted name, e.g. "utils.timer". */
  name: string;
  /** POSIX path relative to the project root, e.g. "lua/utils/timer.lua". */
  path: string;
  kind: ModuleKind;
  /** A Lua module's text; YueScript modules are read when they are compiled. */
  source?: string;
}

/** A folder of modules, relative to the project root, and the kind of file it holds. */
export interface ModuleRoot {
  /** POSIX path relative to the project root, e.g. "src". */
  dir: string;
  kind: ModuleKind;
  /** Whether a missing folder is an error. */
  required: boolean;
}

/** The project's own modules: YueScript in src/ and Lua in lua/ (spec §3.1). src/**\/*.lua is the editor's output. */
export const PROJECT_MODULE_ROOTS: readonly ModuleRoot[] = [
  { dir: "src", kind: "yue", required: true },
  { dir: "lua", kind: "lua", required: false },
];

/**
 * Lists every module under `roots`, in root order and then by path. Fails on a dotted file or folder name (spec §3.1),
 * on two files with one name (spec §3.2) and on a file that takes a built-in module's name. Lua sources are read as
 * Lua's loadfile reads them (`readSourceText`).
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
    for (const file of (await listFiles(dir)).filter((path) => path.endsWith(extension))) {
      const path = `${moduleRoot.dir}/${file}`;
      const stem = file.slice(0, -extension.length);
      if (stem.split("/").some((segment) => segment.includes("."))) {
        throw new MoonwellError("Module file and folder names cannot contain dots.", {
          file: path,
          hint: "Dots separate module names in `import`; rename the file or folder.",
        });
      }
      const module: SourceModule = { name: stem.split("/").join("."), path, kind: moduleRoot.kind };
      for (const name of claimedNames(module.name)) {
        if (BUILTIN_MODULES.has(name)) {
          throw new MoonwellError(`Module ${name} is built into Moonwell; rename ${path}.`, {
            file: path,
            hint: "`require` of a built-in name always loads the built-in module, never a project file.",
          });
        }
        const clash = byName.get(name);
        if (clash !== undefined) {
          throw new MoonwellError(`Module ${name} is defined by ${clash.path} and ${path}.`, {
            file: path,
            hint: "Rename one of them: module names are shared by src/ and lua/.",
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

/** The names a module answers to: its own and, for `<parent>.init`, `<parent>` too (`moduleLoader`, spec §3.2). */
function claimedNames(name: string): string[] {
  return name.endsWith(".init") ? [name, name.slice(0, -".init".length)] : [name];
}

/**
 * Resolves `require` names for the bundler: the name, then `<name>.init`, Lua's `?/init.lua` convention (spec §3.2).
 * A Lua module is its own source; a YueScript module is its compiled output from `loadCompiled`. Either is returned
 * under the name it was required by, which is the name the bundle defines it with.
 */
export function moduleLoader(
  modules: readonly SourceModule[],
  loadCompiled: (name: string) => CompiledModule | undefined,
): (name: string) => CompiledModule | undefined {
  const byName = new Map(modules.map((module) => [module.name, module]));
  return (name) => {
    const module = byName.get(name) ?? byName.get(`${name}.init`);
    if (module === undefined) return undefined;
    if (module.kind === "lua") return { name, sourcePath: module.path, source: module.source ?? "", kind: "lua" };
    const compiled = loadCompiled(module.name);
    return compiled && { ...compiled, name };
  };
}
