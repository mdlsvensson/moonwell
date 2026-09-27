import { dirname, join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { type Runner, runProcess } from "../shared/process.ts";
import { compileError, forEachLimited } from "../yue/compile.ts";

/** One global a source file reads or writes, at its 1-based position. */
export interface GlobalUse {
  name: string;
  line: number;
  column: number;
}

/** The `yue -g` results, next to the compile hashes (`.hashes.json`). */
export const USES_CACHE = "dist/stage/lua/.globals.json";

interface CacheEntry {
  hash: string;
  uses: Array<[string, number, number]>;
}

interface Cache {
  /** The compiler path; another compiler lists again. */
  settings: string;
  files: Record<string, CacheEntry>;
}

/** `yue -g` output: one `NAME LINE COLUMN` per line (spec §2). `file` names the source in errors. */
export function parseGlobalUses(output: string, file: string): GlobalUse[] {
  const uses: GlobalUse[] = [];
  for (const raw of output.split(/\r?\n/)) {
    const line = raw.trim();
    if (line === "") continue;
    const match = /^(\S+) (\d+) (\d+)$/.exec(line);
    if (!match) {
      throw new MoonwellError(`yue -g printed a line Moonwell cannot read: ${line}`, {
        file,
        hint: "Use a YueScript version Moonwell supports: remove yue.version and yue.path from the manifests.",
      });
    }
    uses.push({ name: match[1], line: Number(match[2]), column: Number(match[3]) });
  }
  return uses;
}

function isEntry(value: unknown): value is CacheEntry {
  const entry = value as CacheEntry;
  return typeof entry === "object" && entry !== null && typeof entry.hash === "string" && Array.isArray(entry.uses);
}

async function readCache(path: string): Promise<Cache | undefined> {
  try {
    const cache = JSON.parse(await Deno.readTextFile(path)) as Cache;
    if (typeof cache?.settings !== "string" || typeof cache.files !== "object" || cache.files === null) {
      return undefined;
    }
    return Object.values(cache.files).every(isEntry) ? cache : undefined;
  } catch {
    return undefined;
  }
}

/**
 * The globals each source uses, keyed like `hashes` (paths under src/). Runs `yue -g` only for files whose hash or
 * compiler changed since the last run (spec §5.2); `yue -g` cannot share a run with compiling, so it runs separately.
 */
export async function listGlobalUses(options: {
  yue: string;
  root: string;
  hashes: Record<string, string>;
  run?: Runner;
  concurrency?: number;
}): Promise<Record<string, GlobalUse[]>> {
  const run = options.run ?? runProcess;
  const cachePath = join(options.root, ...USES_CACHE.split("/"));
  const previous = await readCache(cachePath);
  const files: Record<string, CacheEntry> = {};
  const pending: string[] = [];
  for (const [file, hash] of Object.entries(options.hashes)) {
    const cached = previous?.settings === options.yue ? previous.files[file] : undefined;
    if (cached?.hash === hash) files[file] = cached;
    else pending.push(file);
  }

  const failures: MoonwellError[] = [];
  await forEachLimited(pending, options.concurrency ?? 8, async (file) => {
    const label = `src/${file}`;
    const result = await run(options.yue, ["-g", join(options.root, "src", ...file.split("/"))]);
    if (result.code !== 0) {
      failures.push(compileError(label, `${result.stdout}\n${result.stderr}`));
      return;
    }
    const uses = parseGlobalUses(result.stdout, label);
    files[file] = { hash: options.hashes[file], uses: uses.map((use) => [use.name, use.line, use.column]) };
  });

  const sorted = Object.fromEntries(Object.entries(files).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)));
  await Deno.mkdir(dirname(cachePath), { recursive: true });
  await Deno.writeTextFile(cachePath, JSON.stringify({ settings: options.yue, files: sorted }, null, 2));
  if (failures.length > 0) {
    failures.sort((a, b) => (a.file ?? "").localeCompare(b.file ?? ""));
    throw failures[0];
  }
  return Object.fromEntries(
    Object.entries(sorted).map(([file, entry]) => [
      file,
      entry.uses.map(([name, line, column]) => ({ name, line, column })),
    ]),
  );
}
