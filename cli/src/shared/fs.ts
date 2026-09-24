import { copy, walk } from "@std/fs";
import { dirname, relative, SEPARATOR } from "@std/path";

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
    if (!(error instanceof Deno.errors.NotFound)) throw error;
  }
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
