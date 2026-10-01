import { dirname, join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { removeFileIfExists, sha256Hex } from "../shared/fs.ts";
import { type Asset, type AssetsConfig, collectProjectAssets } from "./collect.ts";
import { importPath, readImports, writeImports } from "./imports.ts";
import { assetPath, lstatOrUndefined, pathKey, safeJoin, scanFiles, targetPath } from "./paths.ts";

/** `.asset-state/<map folder>.json`: the source-map files assets:sync owns, by in-map path → SHA-256. */
export interface AssetState {
  version: 1;
  files: Record<string, string>;
}

/** One file to write (`after`) or delete (no `after`); `before` is its content at planning time, if it existed. */
export interface FileChange {
  file: string;
  before?: Uint8Array;
  after?: Uint8Array;
}

export interface AssetPlan {
  assets: Asset[];
  /** A line for each library file that one of the map's own assets replaces. */
  replaced: string[];
  changes: FileChange[];
  /** Ownership after applying the plan. */
  state: AssetState;
}

/** The source map folder and its ownership state file. */
export async function assetLocations(root: string, mapFolder: string): Promise<{ mapDir: string; stateFile: string }> {
  return {
    mapDir: await safeJoin(root, `maps/${mapFolder}`),
    stateFile: await safeJoin(root, `.asset-state/${mapFolder}.json`),
  };
}

function equalBytes(a: Uint8Array | undefined, b: Uint8Array | undefined): boolean {
  if (a === undefined || b === undefined) return a === b;
  return a.length === b.length && a.every((value, i) => value === b[i]);
}

async function readIfExists(file: string): Promise<Uint8Array | undefined> {
  return (await lstatOrUndefined(file)) === undefined ? undefined : await Deno.readFile(file);
}

async function readState(file: string): Promise<AssetState> {
  const bytes = await readIfExists(file);
  if (bytes === undefined) return { version: 1, files: {} };
  const hint = "Restore it from version control. It records which map files assets:sync owns.";
  const invalid = (problem: string): never => {
    throw new MoonwellError(`The asset ownership state is invalid: ${problem}.`, { file, hint });
  };
  let state: unknown;
  try {
    state = JSON.parse(new TextDecoder().decode(bytes));
  } catch {
    invalid("it is not JSON");
  }
  const record = state as { version?: unknown; files?: unknown };
  if (record?.version !== 1) invalid("version must be 1");
  if (record.files === null || typeof record.files !== "object" || Array.isArray(record.files)) {
    invalid("files must be an object");
  }
  const seen = new Set<string>();
  for (const [name, digest] of Object.entries(record.files as Record<string, unknown>)) {
    try {
      targetPath(name);
    } catch (error) {
      if (!(error instanceof MoonwellError)) throw error;
      throw new MoonwellError(`The asset ownership state is invalid: ${error.message}`, { file, hint, cause: error });
    }
    if (typeof digest !== "string" || !/^[a-f0-9]{64}$/.test(digest)) invalid(`${name} has no valid hash`);
    if (seen.has(pathKey(name))) invalid(`${name} is listed twice`);
    seen.add(pathKey(name));
  }
  return record as AssetState;
}

const INTERRUPTED_BEFORE_WRITING = "Interrupted; nothing was written.";

/**
 * Checks every asset (the map's own and those of `libraries`, by key), import and owned file, and returns the changes.
 * Writes nothing. Stops with an error between files once `signal` aborts (Ctrl+C).
 */
export async function planAssets(
  root: string,
  mapDir: string,
  stateFile: string,
  config: AssetsConfig,
  signal?: AbortSignal,
  libraries: readonly string[] = [],
): Promise<AssetPlan> {
  const stopIfInterrupted = () => {
    if (signal?.aborted) throw new MoonwellError(INTERRUPTED_BEFORE_WRITING);
  };
  const { assets, replaced } = await collectProjectAssets(root, config, libraries);
  stopIfInterrupted();
  if (!(await lstatOrUndefined(mapDir))?.isDirectory) {
    throw new MoonwellError(`The map folder ${mapDir} does not exist.`, {
      file: "moonwell.pkl",
      hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
    });
  }
  const managed = new Map(
    Object.entries((await readState(stateFile)).files).map(([name, digest]) => [pathKey(name), { name, digest }]),
  );
  // Nothing to import and nothing owned: leave the map (and its war3map.imp) completely alone.
  if (assets.length === 0 && managed.size === 0) {
    return { assets, replaced, changes: [], state: { version: 1, files: {} } };
  }
  const files = await scanFiles(mapDir);
  const impFile = await safeJoin(mapDir, files.get("war3map.imp") ?? "war3map.imp");
  const impBytes = await readIfExists(impFile);
  const imports = impBytes === undefined ? [] : readImports(impBytes, impFile);
  const importKeys = new Set<string>();
  for (const entry of imports) {
    const key = pathKey(assetPath(importPath(entry)));
    if (importKeys.has(key)) {
      throw new MoonwellError(`war3map.imp lists ${importPath(entry)} twice.`, {
        file: impFile,
        hint: "Remove the duplicate import in World Editor's Import Manager.",
      });
    }
    importKeys.add(key);
  }

  // Every owned file must be unchanged, including files this plan would delete.
  for (const [key] of managed) {
    stopIfInterrupted();
    const current = files.get(key);
    if (current === undefined) continue;
    const file = await safeJoin(mapDir, current);
    const bytes = await Deno.readFile(file);
    if ((await sha256Hex(bytes)) !== managed.get(key)!.digest) {
      throw new MoonwellError(`${current} was modified in the map after assets:sync wrote it.`, {
        file,
        hint: "assets:sync owns this file in the source map (a build stages a copy of it). " +
          "Move your edited copy into assets/, or restore the file in the source map, then run assets:sync.",
      });
    }
  }

  const changes: FileChange[] = [];
  const plannedFolders = new Map<string, string>();
  for (const asset of assets) {
    stopIfInterrupted();
    const key = pathKey(asset.target);
    if (!managed.has(key) && (files.has(key) || importKeys.has(key))) {
      const owner = asset.library === undefined ? "" : ` of library ${asset.library}`;
      throw new MoonwellError(`Asset ${asset.target}${owner} conflicts with a file or import already in the map.`, {
        hint: asset.library === undefined
          ? "Import it under another path with assets.paths, or remove the map's own copy."
          : "Remove the map's own copy in World Editor's Import Manager.",
      });
    }
    // Reuse the spelling of existing folders, and of folders planned earlier, so letter case stays consistent.
    const parts = asset.target.split("/");
    let relative = "";
    for (let i = 0; i < parts.length; i++) {
      const parent = relative === "" ? mapDir : await safeJoin(mapDir, relative);
      const info = await lstatOrUndefined(parent);
      if (info !== undefined && !info.isDirectory) {
        throw new MoonwellError(
          `${relative} in the map is a file, not a directory, so ${asset.target} cannot go there.`,
        );
      }
      const existing = info === undefined
        ? undefined
        : (await Array.fromAsync(Deno.readDir(parent))).find((entry) => pathKey(entry.name) === pathKey(parts[i]));
      if (existing?.isSymlink) {
        throw new MoonwellError(`Symlinks are not supported: ${join(parent, existing.name)}`);
      }
      const folderKey = pathKey(relative === "" ? parts[i] : `${relative}/${parts[i]}`);
      const segment = existing?.name ?? plannedFolders.get(folderKey) ?? parts[i];
      relative = relative === "" ? segment : `${relative}/${segment}`;
      if (i < parts.length - 1) plannedFolders.set(folderKey, segment);
    }
    const file = await safeJoin(mapDir, relative);
    if ((await lstatOrUndefined(file))?.isDirectory) {
      throw new MoonwellError(`Asset ${asset.target} would replace a folder in the map.`);
    }
    const before = await readIfExists(file);
    if (before === undefined || (await sha256Hex(before)) !== asset.hash) {
      changes.push({ file, before, after: asset.bytes });
    }
    asset.target = relative;
  }

  const wanted = new Set(assets.map((asset) => pathKey(asset.target)));
  for (const [key] of managed) {
    const current = files.get(key);
    if (current === undefined || wanted.has(key)) continue;
    const file = await safeJoin(mapDir, current);
    changes.push({ file, before: await Deno.readFile(file) });
  }

  if (assets.length > 0 || managed.size > 0) {
    // An owned entry keeps the flag World Editor saved it with (3.00 turns 13 into 29), so a save changes nothing.
    const ownedFlags = new Map<string, number>();
    const merged = imports.filter((entry) => {
      const key = pathKey(importPath(entry));
      if (managed.has(key)) ownedFlags.set(key, entry.flag);
      return !managed.has(key);
    });
    merged.push(...assets.map((asset) => ({
      flag: ownedFlags.get(pathKey(asset.target)) ?? 13,
      path: asset.target.replaceAll("/", "\\"),
    })));
    const after = writeImports(merged);
    if (!equalBytes(after, impBytes)) changes.push({ file: impFile, before: impBytes, after });
  }
  return {
    assets,
    replaced,
    changes,
    state: { version: 1, files: Object.fromEntries(assets.map((asset) => [asset.target, asset.hash])) },
  };
}

/**
 * Applies a plan, undoing every change already made if one fails or `signal` aborts (Ctrl+C). Writes `stateFile` too
 * when it is given.
 */
export async function applyAssetPlan(plan: AssetPlan, stateFile?: string, signal?: AbortSignal): Promise<void> {
  const changes = [...plan.changes];
  if (stateFile !== undefined) {
    // Owning nothing needs no state file, so an old one is removed.
    const before = await readIfExists(stateFile);
    const after = Object.keys(plan.state.files).length === 0
      ? undefined
      : new TextEncoder().encode(`${JSON.stringify(plan.state, null, 2)}\n`);
    if (!equalBytes(before, after)) changes.push({ file: stateFile, before, after });
  }
  const applied: FileChange[] = [];
  const interrupted = new Error("interrupted");
  try {
    for (const change of changes) {
      if (signal?.aborted) throw interrupted;
      if (!equalBytes(await readIfExists(change.file), change.before)) {
        throw new MoonwellError(`${change.file} changed after the assets were checked.`, {
          hint: "Close World Editor and anything else writing to the map, then retry.",
        });
      }
      applied.push(change);
      if (change.after === undefined) {
        await Deno.remove(change.file);
      } else {
        await Deno.mkdir(dirname(change.file), { recursive: true });
        await Deno.writeFile(change.file, change.after);
      }
    }
  } catch (error) {
    const reasonOf = (failure: unknown) => (failure instanceof Error ? failure.message : String(failure));
    const unrestored: string[] = [];
    for (const change of applied.reverse()) {
      try {
        if (change.before === undefined) await removeFileIfExists(change.file);
        else await Deno.writeFile(change.file, change.before);
      } catch (failure) {
        unrestored.push(`${change.file} (${reasonOf(failure)})`);
      }
    }
    if (unrestored.length > 0) {
      throw new MoonwellError(
        `Writing assets failed (${reasonOf(error)}), and these files could not be restored: ${unrestored.join(", ")}`,
        { cause: error, hint: "Restore the map folder from version control before retrying." },
      );
    }
    if (error instanceof MoonwellError) throw error;
    if (error === interrupted) {
      throw new MoonwellError(
        applied.length === 0 ? INTERRUPTED_BEFORE_WRITING : "Interrupted; every change was undone.",
      );
    }
    throw new MoonwellError(`Writing assets failed: ${reasonOf(error)}. Every change was undone.`, { cause: error });
  }
}
