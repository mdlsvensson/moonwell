import { dirname, join, resolve } from "@std/path";
import type { Library } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { isWithin, listFiles, removeIfExists, writeTextIfChanged } from "../shared/fs.ts";
import type { Logger } from "../shared/log.ts";
import { filesHash } from "./archive.ts";
import { downloadTag, type Fetch, reasonOf } from "./download.ts";
import { LOCK_FILE, type LockEntry, readLock, writeLock } from "./lock.ts";
import { LIBRARY_FILE, parseLibraryFile } from "./manifest.ts";

/** Where libraries' modules live in a project (git-ignored with .moonwell/). */
export const LIBRARIES_DIR = ".moonwell/libraries";

/** Where the files libraries ship for the map live, by library key (spec: library assets §4.1). */
export const LIBRARY_ASSETS_DIR = ".moonwell/library-assets";

/** What a library folder holds: its lock entry, or the local path it was copied from. */
const STAMP = ".moonwell-library.json";

/** The stamp's layout: 2 since a library can ship assets. A folder with another layout is fetched again. */
const LAYOUT = 2;

export { archiveUrl } from "./download.ts";

export interface SyncDeps {
  fetch: Fetch;
  logger: Logger;
}

/** The two folders under `.moonwell/` that hold every library's modules and assets. */
interface Folders {
  modules: string;
  assets: string;
}

/**
 * Brings `.moonwell/libraries/<key>/` and `.moonwell/library-assets/<key>/` up to date for every library of the
 * manifest, and `moonwell.lock` with the GitHub ones (spec §4.2, §4.3; library assets §4). A GitHub library whose
 * folders already hold its lock entry is not downloaded again; a local library keeps the lock entry it had.
 */
export async function syncLibraries(
  root: string,
  libraries: Record<string, Library>,
  manifest: string,
  deps: SyncDeps,
): Promise<void> {
  const keys = Object.keys(libraries).sort();
  refuseCaseClashes(keys, manifest);
  const folders: Folders = {
    modules: join(root, ...LIBRARIES_DIR.split("/")),
    assets: join(root, ...LIBRARY_ASSETS_DIR.split("/")),
  };
  // Before syncing: on a case-insensitive file system, a stale folder `lib` would otherwise take key `Lib`'s files and
  // then be removed as stale.
  await removeStale(folders.modules, LIBRARIES_DIR, new Set(keys));
  await removeStale(folders.assets, LIBRARY_ASSETS_DIR, new Set(keys));
  const lock = await readLock(root);
  const next: Record<string, LockEntry> = {};
  for (const key of keys) {
    const library = libraries[key];
    if (library.path !== null) {
      await syncLocal(root, folders, key, library.path, library.dir, manifest);
      // A path usually comes from moonwell.local.pkl, which is not committed: keep the committed lock entry, so
      // switching back still checks the tag.
      if (lock[key] !== undefined) next[key] = lock[key];
    } else next[key] = await syncGitHub(folders, key, library, lock[key], manifest, deps);
  }
  try {
    await writeLock(root, next);
  } catch (cause) {
    throw new MoonwellError(`Writing ${LOCK_FILE} failed: ${reasonOf(cause)}`, {
      file: LOCK_FILE,
      cause,
      hint: "Close programs that have moonwell.lock open, and check it is not read-only.",
    });
  }
}

/** Keys such as `Lib` and `lib` would share one folder on Windows. */
function refuseCaseClashes(keys: string[], manifest: string): void {
  const seen = new Map<string, string>();
  for (const key of keys) {
    const other = seen.get(key.toLowerCase());
    if (other !== undefined) {
      throw new MoonwellError(`Libraries ${other} and ${key} differ only by case.`, {
        file: manifest,
        hint: "Rename one of them: each library gets a folder in .moonwell/libraries/.",
      });
    }
    seen.set(key.toLowerCase(), key);
  }
}

/** Removes folders of libraries no longer in the manifest, and leftover `.<key>.tmp` folders. */
async function removeStale(folder: string, label: string, keys: Set<string>): Promise<void> {
  const names: string[] = [];
  await writing(label, "", async () => {
    try {
      for await (const entry of Deno.readDir(folder)) names.push(entry.name);
    } catch (cause) {
      if (!(cause instanceof Deno.errors.NotFound)) throw cause;
    }
  });
  for (const name of names) {
    if (!keys.has(name)) await writing(label, name, () => removeIfExists(join(folder, name)));
  }
}

/** A file's bytes, or `undefined` when it cannot be read: a library without the file is the usual case. */
async function readIfFile(path: string): Promise<Uint8Array | undefined> {
  try {
    return await Deno.readFile(path);
  } catch {
    return undefined;
  }
}

/**
 * Copies a local library from `<path>`: its modules (`.yue` and `.lua` files) under the module folder and, when its
 * `moonwell-library.json` names one, every file under its assets folder; both outside dot-names such as `.git/`.
 * Only changed files are written, and every other file is removed.
 */
async function syncLocal(root: string, folders: Folders, key: string, path: string, dir: string, manifest: string) {
  const base = resolve(root, path);
  const file = join(base, LIBRARY_FILE);
  const described = parseLibraryFile(key, await readIfFile(file), file);
  const source = resolve(base, dir !== "" ? dir : described.dir ?? "");
  const info = await Deno.stat(source).catch(() => undefined);
  if (!info?.isDirectory) {
    throw new MoonwellError(`Library ${key}: ${source} is not a folder.`, {
      file: manifest,
      hint: "Set the library's path (and dir) to a folder that holds its modules.",
    });
  }
  if (isWithin(folders.modules, source)) {
    throw new MoonwellError(`Library ${key}: ${source} contains this project's ${LIBRARIES_DIR}.`, {
      file: manifest,
      hint: "Point the library's path (and dir) at the folder that holds its modules, not at the project.",
    });
  }
  const assetsSource = described.assets === null ? undefined : resolve(base, described.assets);
  if (assetsSource !== undefined && !(await Deno.stat(assetsSource).catch(() => undefined))?.isDirectory) {
    throw new MoonwellError(`Library ${key}: ${assetsSource} is not a folder.`, {
      file,
      hint: `Create the folder, or fix assets in the library's ${LIBRARY_FILE}.`,
    });
  }
  const files = new Map<string, Uint8Array>();
  const assets = new Map<string, Uint8Array>();
  try {
    for (const name of await listLocalFiles(source, /\.(yue|lua)$/, assetsSource)) {
      files.set(name, await Deno.readFile(join(source, ...name.split("/"))));
    }
    if (assetsSource !== undefined) {
      for (const name of await listLocalFiles(assetsSource, /^/)) {
        assets.set(name, await Deno.readFile(join(assetsSource, ...name.split("/"))));
      }
    }
  } catch (cause) {
    throw new MoonwellError(`Reading library ${key} from ${source} failed: ${reasonOf(cause)}`, {
      file: manifest,
      cause,
      hint: "Check the library's path and that its files can be read.",
    });
  }
  await writing(LIBRARY_ASSETS_DIR, key, async () => {
    if (assetsSource === undefined) await removeIfExists(join(folders.assets, key));
    else await mirror(join(folders.assets, key), assets);
  });
  const target = join(folders.modules, key);
  await writing(LIBRARIES_DIR, key, async () => {
    await mirror(target, files, STAMP);
    await writeTextIfChanged(join(target, STAMP), stampText({ path: source }));
  });
}

/** Makes `target` hold exactly `files` (and `keep`, when named), writing only the files that changed. */
async function mirror(target: string, files: Map<string, Uint8Array>, keep?: string): Promise<void> {
  for (const [file, data] of files) await writeBytesIfChanged(join(target, ...file.split("/")), data);
  await Deno.mkdir(target, { recursive: true });
  for (const file of await listFiles(target)) {
    if (file !== keep && !files.has(file)) await Deno.remove(join(target, ...file.split("/")));
  }
}

/**
 * The files under `dir` whose name matches, as POSIX paths. It never takes a file or looks inside a folder whose name
 * starts with `.`, and never looks inside `skip` (the assets folder, when it lies inside the module folder).
 */
async function listLocalFiles(dir: string, name: RegExp, skip?: string, prefix = ""): Promise<string[]> {
  const files: string[] = [];
  for await (const entry of Deno.readDir(dir)) {
    if (entry.name.startsWith(".")) continue;
    const path = `${prefix}${entry.name}`;
    const full = join(dir, entry.name);
    if (entry.isDirectory) {
      if (skip !== undefined && isWithin(full, skip)) continue;
      files.push(...await listLocalFiles(full, name, skip, `${path}/`));
    } else if (name.test(entry.name)) files.push(path);
  }
  return files;
}

/** Downloads a GitHub library unless its folders hold its lock entry; returns its lock entry. */
async function syncGitHub(
  folders: Folders,
  key: string,
  library: Library,
  locked: LockEntry | undefined,
  manifest: string,
  deps: SyncDeps,
): Promise<LockEntry> {
  const [github, tag, dir] = [library.github!, library.tag!, library.dir];
  const repository = github.split("/")[1];
  if (tag.split("/").some((segment) => segment === "." || segment === "..")) {
    throw new MoonwellError(`Library ${key}: ${tag} is not a tag name.`, {
      file: manifest,
      hint: `Use the tag's name as it appears at https://github.com/${github}/tags.`,
    });
  }
  if (repository === "." || repository === "..") {
    throw new MoonwellError(`Library ${key}: ${github} is not a GitHub repository.`, {
      file: manifest,
      hint: 'Write it as "owner/repo".',
    });
  }
  const sameTag = locked !== undefined && locked.github === github && locked.tag === tag && locked.dir === dir;
  if (sameTag && await holds(folders, key, locked)) return locked;
  const { commit, files } = await downloadTag(key, github, tag, manifest, deps.fetch);
  const where = `https://github.com/${github}/blob/${tag}/${LIBRARY_FILE}`;
  const described = parseLibraryFile(key, files.get(LIBRARY_FILE), where);
  const moduleDir = dir !== "" ? dir : described.dir ?? "";
  const kept = keepDir(files, moduleDir, described.assets ?? undefined);
  if (kept.size === 0) {
    throw new MoonwellError(`Library ${key} has no folder ${moduleDir} at ${tag}.`, {
      file: dir !== "" ? manifest : where,
      hint: dir !== "" ? "Fix the library's dir." : `Its ${LIBRARY_FILE} names a dir that has no files.`,
    });
  }
  const assets = described.assets === null ? undefined : keepDir(files, described.assets);
  if (assets !== undefined && assets.size === 0) {
    throw new MoonwellError(`Library ${key} has no folder ${described.assets} at ${tag}.`, {
      file: where,
      hint: `Its ${LIBRARY_FILE} names an assets folder that has no files; report it to the library's author.`,
    });
  }
  const entry: LockEntry = {
    github,
    tag,
    dir,
    commit,
    files: await filesHash(kept),
    ...(assets === undefined ? {} : { assets: await filesHash(assets) }),
  };
  // A lock from before libraries shipped assets has no assets hash, and its module hash may count files that are
  // assets now: then only the commit is compared (spec: library assets §4.4).
  const comparable = sameTag && (locked.assets === undefined) === (entry.assets === undefined);
  if (
    sameTag &&
    (entry.commit !== locked.commit || (comparable && (entry.files !== locked.files || entry.assets !== locked.assets)))
  ) {
    throw new MoonwellError(
      `Library ${key}: tag ${tag} of ${github} moved from ${locked.commit.slice(0, 12)} to ${
        entry.commit.slice(0, 12)
      } since moonwell.lock recorded it.`,
      {
        file: LOCK_FILE,
        hint: "If the move was intended, delete the library's entry from moonwell.lock and run the command again.",
      },
    );
  }
  // The assets first and the stamp last: an interrupted sync leaves folders that are fetched again.
  await writing(LIBRARY_ASSETS_DIR, key, async () => {
    if (assets === undefined) await removeIfExists(join(folders.assets, key));
    else await replaceFolder(folders.assets, key, assets);
  });
  await writing(LIBRARIES_DIR, key, () => replaceFolder(folders.modules, key, kept, entry));
  deps.logger.info(`Fetched library ${key}: ${github} ${tag} (${entry.commit.slice(0, 7)}).`);
  return entry;
}

/** Whether the library's folders hold `entry`: the stamp says so, and its assets folder is there when it has assets. */
async function holds(folders: Folders, key: string, entry: LockEntry): Promise<boolean> {
  if (!sameEntry(await readStamp(join(folders.modules, key)), entry)) return false;
  if (entry.assets === undefined) return true;
  return (await Deno.stat(join(folders.assets, key)).catch(() => undefined))?.isDirectory === true;
}

/**
 * The files under `dir` (all of them when it is empty), relative to it, except those in a folder or with a name that
 * starts with `.` (such as `.github/`, and the stamp), and except those under the folder `except`.
 */
function keepDir(files: Map<string, Uint8Array>, dir: string, except?: string): Map<string, Uint8Array> {
  const prefix = dir.split(/[\\/]/).filter((segment) => segment !== "" && segment !== ".").join("/");
  const kept = new Map<string, Uint8Array>();
  for (const [path, data] of files) {
    if (except !== undefined && path.startsWith(`${except}/`)) continue;
    const relative = prefix === "" ? path : path.startsWith(`${prefix}/`) ? path.slice(prefix.length + 1) : undefined;
    if (relative !== undefined && !relative.split("/").some((segment) => segment.startsWith("."))) {
      kept.set(relative, data);
    }
  }
  return kept;
}

/** Writes the files, and the stamp of `entry` when given, into `.<key>.tmp/`, which then replaces `<folder>/<key>`. */
async function replaceFolder(folder: string, key: string, files: Map<string, Uint8Array>, entry?: LockEntry) {
  const temp = join(folder, `.${key}.tmp`);
  await removeIfExists(temp);
  for (const [path, data] of files) {
    const file = join(temp, ...path.split("/"));
    await Deno.mkdir(dirname(file), { recursive: true });
    await Deno.writeFile(file, data);
  }
  if (entry !== undefined) await writeTextIfChanged(join(temp, STAMP), stampText({ ...entry, layout: LAYOUT }));
  const target = join(folder, key);
  await removeIfExists(target);
  await Deno.rename(temp, target);
}

/** The stamp in a library's folder; `undefined` when there is none or it is unreadable. */
async function readStamp(target: string): Promise<unknown> {
  try {
    return JSON.parse(await Deno.readTextFile(join(target, STAMP)));
  } catch {
    return undefined;
  }
}

function sameEntry(stamp: unknown, entry: LockEntry): boolean {
  if (typeof stamp !== "object" || stamp === null) return false;
  const fields = stamp as Record<string, unknown>;
  return fields.layout === LAYOUT &&
    (["github", "tag", "dir", "commit", "files", "assets"] as const).every((field) => fields[field] === entry[field]);
}

const stampText = (value: object) => `${JSON.stringify(value, null, 2)}\n`;

async function writeBytesIfChanged(path: string, bytes: Uint8Array): Promise<void> {
  try {
    const old = await Deno.readFile(path);
    if (old.length === bytes.length && old.every((byte, index) => byte === bytes[index])) return;
  } catch (cause) {
    if (!(cause instanceof Deno.errors.NotFound)) throw cause;
  }
  await Deno.mkdir(dirname(path), { recursive: true });
  await Deno.writeFile(path, bytes);
}

/** Runs `body`, reporting any failure as one writing `<label>/<name>` (the folder `label` itself for ""). */
async function writing<T>(label: string, name: string, body: () => Promise<T>): Promise<T> {
  try {
    return await body();
  } catch (cause) {
    const path = name === "" ? label : `${label}/${name}`;
    throw new MoonwellError(`Writing ${path} failed: ${reasonOf(cause)}`, {
      file: path,
      cause,
      hint: "Close programs that have files in .moonwell/ open, then retry.",
    });
  }
}
