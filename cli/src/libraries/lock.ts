import { join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { removeFileIfExists, writeTextIfChanged } from "../shared/fs.ts";

/** The committed lock file, at the project root (spec §4.3). */
export const LOCK_FILE = "moonwell.lock";

/** What a GitHub library resolved to: the manifest's `github`, `tag` and `dir`, the tag's commit and the kept files. */
export interface LockEntry {
  github: string;
  tag: string;
  dir: string;
  commit: string;
  files: string;
}

const HINT = "Fix it, or delete it: the next check downloads every library again and writes a new one.";

function isEntry(value: unknown): value is LockEntry {
  const entry = value as Record<string, unknown>;
  return typeof value === "object" && value !== null &&
    ["github", "tag", "dir", "commit", "files"].every((key) => typeof entry[key] === "string");
}

/** The lock's entries by library key; none when there is no lock file. */
export async function readLock(root: string): Promise<Record<string, LockEntry>> {
  let text: string;
  try {
    text = await Deno.readTextFile(join(root, LOCK_FILE));
  } catch (cause) {
    if (cause instanceof Deno.errors.NotFound) return {};
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Reading ${LOCK_FILE} failed: ${reason}`, { file: LOCK_FILE, cause, hint: HINT });
  }
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch (cause) {
    throw new MoonwellError(`${LOCK_FILE} is not valid JSON.`, { file: LOCK_FILE, cause, hint: HINT });
  }
  const libraries = (data as { libraries?: unknown } | null)?.libraries;
  if (
    typeof libraries !== "object" || libraries === null || Array.isArray(libraries) ||
    !Object.values(libraries).every(isEntry)
  ) {
    throw new MoonwellError(`${LOCK_FILE} is not a Moonwell lock file.`, { file: LOCK_FILE, hint: HINT });
  }
  return libraries as Record<string, LockEntry>;
}

/**
 * Writes the entries sorted by key, each as github, tag, dir, commit, files, with two-space indentation; only when
 * the text changes. No entries removes the file (plan decision).
 */
export async function writeLock(root: string, libraries: Record<string, LockEntry>): Promise<void> {
  const path = join(root, LOCK_FILE);
  const keys = Object.keys(libraries).sort();
  if (keys.length === 0) {
    await removeFileIfExists(path);
    return;
  }
  const sorted = Object.fromEntries(keys.map((key) => {
    const { github, tag, dir, commit, files } = libraries[key];
    return [key, { github, tag, dir, commit, files }];
  }));
  await writeTextIfChanged(path, `${JSON.stringify({ libraries: sorted }, null, 2)}\n`);
}
