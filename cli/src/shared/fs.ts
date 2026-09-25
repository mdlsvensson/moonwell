import { copy, walk } from "@std/fs";
import { dirname, relative, SEPARATOR } from "@std/path";
import { MoonwellError } from "./errors.ts";

export function toPosix(path: string): string {
  return path.split(SEPARATOR).join("/");
}

/** All files below `dir`, as sorted POSIX paths relative to `dir`. */
export async function listFiles(dir: string): Promise<string[]> {
  const files: string[] = [];
  for await (const entry of walk(dir, { includeDirs: false })) files.push(toPosix(relative(dir, entry.path)));
  return files.sort();
}

export async function removeIfExists(path: string): Promise<void> {
  try {
    await Deno.remove(path, { recursive: true });
  } catch (error) {
    if (!(error instanceof Deno.errors.NotFound)) throw inUseError(error, path);
  }
}

/** Removes a file. Never recursive, so a directory with contents fails instead of being deleted. */
export async function removeFileIfExists(path: string): Promise<void> {
  try {
    await Deno.remove(path);
  } catch (error) {
    if (!(error instanceof Deno.errors.NotFound)) throw inUseError(error, path);
  }
}

/** A clear error when another program (on Windows: EBUSY) holds `path` open, else `error` unchanged. */
function inUseError(error: unknown, path: string): unknown {
  if ((error as { code?: string }).code !== "EBUSY") return error;
  return new MoonwellError(`${path} is in use by another program.`, {
    hint: "Close Warcraft III or World Editor if it has this map open, then try again.",
  });
}

/** Replaces `destination` with a copy of `source`. */
export async function replaceDir(source: string, destination: string): Promise<void> {
  await removeIfExists(destination);
  await Deno.mkdir(dirname(destination), { recursive: true });
  await copy(source, destination);
}

/** Writes `text` unless the file already has exactly that content. Returns whether it wrote. */
export async function writeTextIfChanged(path: string, text: string): Promise<boolean> {
  try {
    if (await Deno.readTextFile(path) === text) return false;
  } catch (error) {
    if (!(error instanceof Deno.errors.NotFound)) throw error;
  }
  await Deno.mkdir(dirname(path), { recursive: true });
  await Deno.writeTextFile(path, text);
  return true;
}

export async function sha256Hex(bytes: Uint8Array): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", bytes.slice());
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
}
