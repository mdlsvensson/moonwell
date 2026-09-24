import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { listFiles, removeIfExists, sha256Hex } from "../shared/fs.ts";
import { type Runner, runProcess } from "../shared/process.ts";

export interface CompiledModule {
  /** Dotted module name, e.g. "heroes.captain". */
  name: string;
  /** POSIX path relative to the project root, e.g. "src/heroes/captain.yue". */
  sourcePath: string;
  source: string;
}

export interface CompileOutput {
  outDir: string;
  load(name: string): CompiledModule | undefined;
}

interface Manifest {
  settings: string;
  files: Record<string, string>;
}

/** Compiles every .yue file under src/ into dist/stage/lua, recompiling only changed files. */
export async function compileSources(options: {
  yue: string;
  root: string;
  minify: boolean;
  run?: Runner;
  concurrency?: number;
}): Promise<CompileOutput> {
  const run = options.run ?? runProcess;
  const srcDir = join(options.root, "src");
  const outDir = join(options.root, "dist", "stage", "lua");
  if (!(await exists(srcDir))) throw new MoonwellError("The src/ folder is missing.", { file: options.root });

  const sources = (await listFiles(srcDir)).filter((file) => file.endsWith(".yue"));
  for (const file of sources) {
    if (file.slice(0, -4).split("/").some((segment) => segment.includes("."))) {
      throw new MoonwellError("Module file and folder names cannot contain dots.", {
        file: `src/${file}`,
        hint: "Dots separate module names in `import`; rename the file or folder.",
      });
    }
  }

  const manifestPath = join(outDir, ".hashes.json");
  const previous = await readManifest(manifestPath);
  const settings = `${options.yue}|${options.minify ? "minify" : "rewrite"}`;
  const hashes: Record<string, string> = {};
  const pending: string[] = [];
  for (const file of sources) {
    hashes[file] = await sha256Hex(await Deno.readFile(join(srcDir, file)));
    const upToDate = previous?.settings === settings && previous.files[file] === hashes[file] &&
      await exists(join(outDir, luaPath(file)));
    if (!upToDate) pending.push(file);
  }
  for (const file of Object.keys(previous?.files ?? {})) {
    if (!(file in hashes)) await removeIfExists(join(outDir, luaPath(file)));
  }

  const failures: MoonwellError[] = [];
  await forEachLimited(pending, options.concurrency ?? 8, async (file) => {
    const output = join(outDir, luaPath(file));
    await Deno.mkdir(dirname(output), { recursive: true });
    const mode = options.minify ? "-m" : "-r";
    const result = await run(options.yue, ["--target=5.3", mode, "-o", output, join(srcDir, file)]);
    if (result.code !== 0) {
      delete hashes[file];
      await removeIfExists(output);
      failures.push(compileError(`src/${file}`, `${result.stdout}\n${result.stderr}`));
    }
  });

  await Deno.mkdir(outDir, { recursive: true });
  await Deno.writeTextFile(manifestPath, JSON.stringify({ settings, files: hashes }, null, 2));
  if (failures.length > 0) {
    failures.sort((a, b) => (a.file ?? "").localeCompare(b.file ?? ""));
    if (failures.length === 1) throw failures[0];
    throw new MoonwellError(`${failures[0].message}\n(${failures.length - 1} more file(s) failed to compile)`, {
      file: failures[0].file,
      line: failures[0].line,
    });
  }

  const modules = new Set(sources.map((file) => file.slice(0, -4).split("/").join(".")));
  return {
    outDir,
    load(name: string): CompiledModule | undefined {
      if (!modules.has(name)) return undefined;
      const relative = name.split(".").join("/");
      try {
        return {
          name,
          sourcePath: `src/${relative}.yue`,
          source: Deno.readTextFileSync(join(outDir, `${relative}.lua`)),
        };
      } catch (error) {
        if (error instanceof Deno.errors.NotFound) return undefined;
        throw error;
      }
    },
  };
}

function luaPath(file: string): string {
  return file.replace(/\.yue$/, ".lua");
}

async function readManifest(path: string): Promise<Manifest | undefined> {
  try {
    return JSON.parse(await Deno.readTextFile(path)) as Manifest;
  } catch {
    return undefined;
  }
}

/** yue prints "Failed to compile: <file>", then "<line>: <message>" and a source excerpt. */
function compileError(file: string, output: string): MoonwellError {
  const detail = output.split(/\r?\n/).filter((line) => !line.startsWith("Failed to compile")).join("\n").trim();
  const match = /^(\d+): (.+)$/m.exec(output);
  return new MoonwellError(match ? `${match[2]}\n${detail}` : detail || "YueScript compilation failed.", {
    file,
    line: match ? Number(match[1]) : undefined,
  });
}

async function forEachLimited<T>(items: T[], limit: number, fn: (item: T) => Promise<void>): Promise<void> {
  let next = 0;
  const workers = Array.from({ length: Math.min(limit, items.length) }, async () => {
    while (next < items.length) await fn(items[next++]);
  });
  await Promise.all(workers);
}
