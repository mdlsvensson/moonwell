import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { collectModules, type SourceModule } from "../bundle/modules.ts";
import { MoonwellError } from "../shared/errors.ts";
import { removeIfExists, sha256Hex } from "../shared/fs.ts";
import { type Runner, runProcess } from "../shared/process.ts";
import { macroPathArgs, type MacroSearch } from "./macros.ts";

export interface CompiledModule {
  /** Dotted module name, e.g. "heroes.captain". */
  name: string;
  /** POSIX path relative to the project root, e.g. "src/heroes/captain.yue". */
  sourcePath: string;
  source: string;
  /** "lua" for a Lua module, which the bundle never minifies; YueScript when absent. */
  kind?: "yue" | "lua";
}

export interface CompileOutput {
  outDir: string;
  /** SHA-256 of each compiled source, keyed by its POSIX path under src/, e.g. "heroes/captain.yue". */
  hashes: Record<string, string>;
  /** The text of each compiled source (the bytes that were hashed, as UTF-8), keyed like `hashes`. */
  sources: Record<string, string>;
  /** The text of every compiled YueScript source, src/ and libraries, keyed by its POSIX path under the project root. */
  texts: Record<string, string>;
  /** A src/ module's compiled output by dotted name. */
  load(name: string): CompiledModule | undefined;
  /** Any compiled YueScript module's output; its `sourcePath` is the module's project path. */
  loadModule(module: SourceModule): CompiledModule | undefined;
}

interface Manifest {
  settings: string;
  files: Record<string, string>;
}

/**
 * Where a YueScript source compiles to, under dist/stage/lua: `src/x.yue` → `x.lua`,
 * `.moonwell/libraries/<key>/x.yue` → `.libraries/<key>/x.lua`.
 */
export function outputPathOf(path: string): string {
  const lua = path.replace(/\.yue$/, ".lua");
  if (lua.startsWith("src/")) return lua.slice("src/".length);
  if (lua.startsWith(".moonwell/libraries/")) return `.libraries/${lua.slice(".moonwell/libraries/".length)}`;
  throw new Error(`No compile output location for ${path}.`);
}

/** Compiles every YueScript module, src/ and libraries, into dist/stage/lua, recompiling only changed files. */
export async function compileSources(options: {
  yue: string;
  root: string;
  minify: boolean;
  /** Where `import "moonwell.macros"` is found; every yue run gets its `--path` (spec §6). */
  macros?: MacroSearch;
  /** The project's modules (`collectModules`); listed from disk when absent. Every YueScript module compiles. */
  modules?: readonly SourceModule[];
  run?: Runner;
  concurrency?: number;
}): Promise<CompileOutput> {
  const run = options.run ?? runProcess;
  const outDir = join(options.root, "dist", "stage", "lua");
  const modules = options.modules ?? await collectModules(options.root);
  const yueModules = modules.filter((module) => module.kind === "yue");
  const outputOf = (path: string) => join(outDir, ...outputPathOf(path).split("/"));

  const manifestPath = join(outDir, ".hashes.json");
  const previous = await readManifest(manifestPath);
  // "paths|" marks a manifest keyed by project path; an older one was keyed by the path under src/.
  const format = "paths|";
  const flavour = options.minify ? "minify" : "rewrite";
  const settings = `${format}${options.yue}|${flavour}|${options.macros?.hash ?? "no macros"}`;
  /** Keyed by project path, like the manifest. */
  const fileHashes: Record<string, string> = {};
  const texts: Record<string, string> = {};
  const pending: string[] = [];
  const decoder = new TextDecoder();
  for (const { path } of yueModules) {
    const bytes = await Deno.readFile(join(options.root, ...path.split("/")));
    fileHashes[path] = await sha256Hex(bytes);
    texts[path] = decoder.decode(bytes);
    const upToDate = previous?.settings === settings && previous.files[path] === fileHashes[path] &&
      await exists(outputOf(path));
    if (!upToDate) pending.push(path);
  }
  // `outputPathOf` does not map an older manifest's keys, so its outputs are left in place.
  if (previous?.settings?.startsWith(format)) {
    for (const path of Object.keys(previous.files ?? {})) {
      if (!(path in fileHashes)) await removeIfExists(outputOf(path));
    }
  }

  const failures: MoonwellError[] = [];
  await forEachLimited(pending, options.concurrency ?? 8, async (path) => {
    const output = outputOf(path);
    await Deno.mkdir(dirname(output), { recursive: true });
    const mode = options.minify ? "-m" : "-r";
    const result = await run(options.yue, [
      "--target=5.3",
      mode,
      "-o",
      output,
      ...macroPathArgs(options.macros),
      join(options.root, ...path.split("/")),
    ]);
    const failure = result.code !== 0
      ? compileError(path, `${result.stdout}\n${result.stderr}`)
      : await emptyOutputError(path, output, texts[path]);
    if (failure) {
      delete fileHashes[path];
      delete texts[path];
      await removeIfExists(output);
      failures.push(failure);
    }
  });

  await Deno.mkdir(outDir, { recursive: true });
  await Deno.writeTextFile(manifestPath, JSON.stringify({ settings, files: fileHashes }, null, 2));
  if (failures.length > 0) {
    failures.sort((a, b) => (a.file ?? "").localeCompare(b.file ?? ""));
    if (failures.length === 1) throw failures[0];
    throw new MoonwellError(`${failures[0].message}\n(${failures.length - 1} more file(s) failed to compile)`, {
      file: failures[0].file,
      line: failures[0].line,
    });
  }

  const hashes: Record<string, string> = {};
  const sources: Record<string, string> = {};
  for (const [path, hash] of Object.entries(fileHashes)) {
    if (!path.startsWith("src/")) continue;
    hashes[path.slice("src/".length)] = hash;
    sources[path.slice("src/".length)] = texts[path];
  }
  const srcModules = new Map(
    yueModules.filter((module) => module.path.startsWith("src/")).map((module) => [module.name, module]),
  );
  function loadModule(module: SourceModule): CompiledModule | undefined {
    if (module.kind !== "yue" || !(module.path in texts)) return undefined;
    try {
      return {
        name: module.name,
        sourcePath: module.path,
        source: Deno.readTextFileSync(outputOf(module.path)),
      };
    } catch (error) {
      if (error instanceof Deno.errors.NotFound) return undefined;
      throw error;
    }
  }
  return {
    outDir,
    hashes,
    sources,
    texts,
    load(name: string): CompiledModule | undefined {
      const module = srcModules.get(name);
      return module && loadModule(module);
    },
    loadModule,
  };
}

async function readManifest(path: string): Promise<Manifest | undefined> {
  try {
    return JSON.parse(await Deno.readTextFile(path)) as Manifest;
  } catch {
    return undefined;
  }
}

/**
 * yue prints "Failed to compile: <file>", then "<line>: <message>" and a source excerpt. A failing macro's message
 * starts with "failed to expand macro: (macro <name>):<line>: ", a line of the macro module rather than of the file, so
 * the first line of the error drops it; the excerpt keeps the compiler's full text.
 */
/**
 * yue 0.34.2 reports success but writes an empty file for a source that uses floor division (`//`), with both `-r` and
 * `-m`; the module would silently vanish from the build. A source without code (blank or comment lines) writes no file.
 */
async function emptyOutputError(file: string, output: string, source: string): Promise<MoonwellError | undefined> {
  const info = await Deno.stat(output).catch(() => undefined);
  if (info?.size !== 0) return undefined;
  if (source.split(/\r?\n/).every((line) => /^\s*(--.*)?$/.test(line))) return undefined;
  return new MoonwellError(`YueScript reported success but wrote no Lua for ${file}, although the file has code.`, {
    file,
    hint: "YueScript 0.34.2 does this for a file that uses the floor division operator `//`. " +
      "Write math.floor(a / b) instead.",
  });
}

export function compileError(file: string, output: string): MoonwellError {
  const detail = output.split(/\r?\n/).filter((line) => !line.startsWith("Failed to compile")).join("\n").trim();
  const match = /^(\d+): (.+)$/m.exec(output);
  const headline = match?.[2].replace(/^failed to expand macro: \(macro [^)]*\):\d+: /, "");
  return new MoonwellError(match ? `${headline}\n${detail}` : detail || "YueScript compilation failed.", {
    file,
    line: match ? Number(match[1]) : undefined,
  });
}

export async function forEachLimited<T>(items: T[], limit: number, fn: (item: T) => Promise<void>): Promise<void> {
  let next = 0;
  const workers = Array.from({ length: Math.min(limit, items.length) }, async () => {
    while (next < items.length) await fn(items[next++]);
  });
  await Promise.all(workers);
}
