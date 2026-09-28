import { dirname, join, resolve } from "@std/path";
import type { Library } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { isWithin, listFiles, removeIfExists, writeTextIfChanged } from "../shared/fs.ts";
import type { Logger } from "../shared/log.ts";
import { filesHash } from "./archive.ts";
import { downloadTag, type Fetch, reasonOf } from "./download.ts";
import { LOCK_FILE, type LockEntry, readLock, writeLock } from "./lock.ts";

/** Where libraries live in a project (git-ignored with .moonwell/). */
export const LIBRARIES_DIR = ".moonwell/libraries";

/** What a library folder holds: its lock entry, or the local path it was copied from. */
const STAMP = ".moonwell-library.json";

export { archiveUrl } from "./download.ts";

export interface SyncDeps {
  fetch: Fetch;
  logger: Logger;
}

/**
 * Brings `.moonwell/libraries/<key>/` up to date for every library of the manifest, and `moonwell.lock` with the
 * GitHub ones (spec §4.2, §4.3). A GitHub library whose folder already holds its lock entry is not downloaded again;
 * a local library keeps the lock entry it had.
 */
export async function syncLibraries(
  root: string,
  libraries: Record<string, Library>,
  manifest: string,
  deps: SyncDeps,
): Promise<void> {
  const keys = Object.keys(libraries).sort();
  refuseCaseClashes(keys, manifest);
  const folder = join(root, ...LIBRARIES_DIR.split("/"));
  // Before syncing: on a case-insensitive file system, a stale folder `lib` would otherwise take key `Lib`'s files and
  // then be removed as stale.
  await removeStale(folder, new Set(keys));
  const lock = await readLock(root);
  const next: Record<string, LockEntry> = {};
  for (const key of keys) {
    const library = libraries[key];
    if (library.path !== null) {
      await syncLocal(root, folder, key, library.path, library.dir, manifest);
      // A path usually comes from moonwell.local.pkl, which is not committed: keep the committed lock entry, so
      // switching back still checks the tag.
      if (lock[key] !== undefined) next[key] = lock[key];
    } else next[key] = await syncGitHub(folder, key, library, lock[key], manifest, deps);
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
async function removeStale(folder: string, keys: Set<string>): Promise<void> {
  const names: string[] = [];
  await writing("", async () => {
    try {
      for await (const entry of Deno.readDir(folder)) names.push(entry.name);
    } catch (cause) {
      if (!(cause instanceof Deno.errors.NotFound)) throw cause;
    }
  });
  for (const name of names) if (!keys.has(name)) await writing(name, () => removeIfExists(join(folder, name)));
}

/**
 * Copies the modules (`.yue` and `.lua` files) under `<path>/<dir>`, outside dot-folders such as `.git/`, into the
 * library's folder, writing only changed files and removing every other file.
 */
async function syncLocal(root: string, folder: string, key: string, path: string, dir: string, manifest: string) {
  const source = resolve(root, path, dir);
  const info = await Deno.stat(source).catch(() => undefined);
  if (!info?.isDirectory) {
    throw new MoonwellError(`Library ${key}: ${source} is not a folder.`, {
      file: manifest,
      hint: "Set the library's path (and dir) to a folder that holds its modules.",
    });
  }
  if (isWithin(folder, source)) {
    throw new MoonwellError(`Library ${key}: ${source} contains this project's ${LIBRARIES_DIR}.`, {
      file: manifest,
      hint: "Point the library's path (and dir) at the folder that holds its modules, not at the project.",
    });
  }
  const files = new Map<string, Uint8Array>();
  try {
    for (const file of await listModuleFiles(source)) {
      files.set(file, await Deno.readFile(join(source, ...file.split("/"))));
    }
  } catch (cause) {
    throw new MoonwellError(`Reading library ${key} from ${source} failed: ${reasonOf(cause)}`, {
      file: manifest,
      cause,
      hint: "Check the library's path and that its files can be read.",
    });
  }
  const target = join(folder, key);
  await writing(key, async () => {
    for (const [file, data] of files) await writeBytesIfChanged(join(target, ...file.split("/")), data);
    await Deno.mkdir(target, { recursive: true });
    for (const file of await listFiles(target)) {
      if (file !== STAMP && !files.has(file)) await Deno.remove(join(target, ...file.split("/")));
    }
    await writeTextIfChanged(join(target, STAMP), stampText({ path: source }));
  });
}

/** The `.yue` and `.lua` files under `dir` as POSIX paths, never looking inside a folder whose name starts with `.`. */
async function listModuleFiles(dir: string, prefix = ""): Promise<string[]> {
  const files: string[] = [];
  for await (const entry of Deno.readDir(dir)) {
    if (entry.name.startsWith(".")) continue;
    const path = `${prefix}${entry.name}`;
    if (entry.isDirectory) files.push(...await listModuleFiles(join(dir, entry.name), `${path}/`));
    else if (/\.(yue|lua)$/.test(entry.name)) files.push(path);
  }
  return files;
}

/** Downloads a GitHub library unless its folder holds its lock entry; returns its lock entry. */
async function syncGitHub(
  folder: string,
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
  if (sameTag && sameEntry(await readStamp(join(folder, key)), locked)) return locked;
  const { commit, files } = await downloadTag(key, github, tag, manifest, deps.fetch);
  const kept = keepDir(files, dir);
  if (kept.size === 0) {
    throw new MoonwellError(`Library ${key} has no folder ${dir} at ${tag}.`, {
      file: manifest,
      hint: "Fix the library's dir.",
    });
  }
  const entry: LockEntry = { github, tag, dir, commit, files: await filesHash(kept) };
  if (sameTag && (entry.commit !== locked.commit || entry.files !== locked.files)) {
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
  await writing(key, () => replaceFolder(folder, key, kept, entry));
  deps.logger.info(`Fetched library ${key}: ${github} ${tag} (${entry.commit.slice(0, 7)}).`);
  return entry;
}

/**
 * The files under `dir` (all of them when it is empty), relative to it, except those in a folder or with a name that
 * starts with `.` (such as `.github/`, and the stamp).
 */
function keepDir(files: Map<string, Uint8Array>, dir: string): Map<string, Uint8Array> {
  const prefix = dir.split(/[\\/]/).filter((segment) => segment !== "" && segment !== ".").join("/");
  const kept = new Map<string, Uint8Array>();
  for (const [path, data] of files) {
    const relative = prefix === "" ? path : path.startsWith(`${prefix}/`) ? path.slice(prefix.length + 1) : undefined;
    if (relative !== undefined && !relative.split("/").some((segment) => segment.startsWith("."))) {
      kept.set(relative, data);
    }
  }
  return kept;
}

/** Writes the files and the stamp into `.<key>.tmp/`, which then replaces the library's folder. */
async function replaceFolder(folder: string, key: string, files: Map<string, Uint8Array>, entry: LockEntry) {
  const temp = join(folder, `.${key}.tmp`);
  await removeIfExists(temp);
  for (const [path, data] of files) {
    const file = join(temp, ...path.split("/"));
    await Deno.mkdir(dirname(file), { recursive: true });
    await Deno.writeFile(file, data);
  }
  await writeTextIfChanged(join(temp, STAMP), stampText(entry));
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
  return (["github", "tag", "dir", "commit", "files"] as const).every((field) => fields[field] === entry[field]);
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

/** Runs `body`, reporting any failure as one writing `.moonwell/libraries/<name>` (the folder itself for ""). */
async function writing<T>(name: string, body: () => Promise<T>): Promise<T> {
  try {
    return await body();
  } catch (cause) {
    const path = name === "" ? LIBRARIES_DIR : `${LIBRARIES_DIR}/${name}`;
    throw new MoonwellError(`Writing ${path} failed: ${reasonOf(cause)}`, {
      file: path,
      cause,
      hint: "Close programs that have files in .moonwell/ open, then retry.",
    });
  }
}
