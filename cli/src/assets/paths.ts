import { join, resolve } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";

/** A path's identity as Warcraft III and Windows see it: `/` separators, any letter case. */
export function pathKey(path: string): string {
  return path.replaceAll("\\", "/").toLowerCase();
}

const DEVICE_NAME = /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i;

function unsafeSegment(segment: string): boolean {
  return segment === "" || segment === "." || segment === ".." ||
    [...segment].some((char) => char.charCodeAt(0) < 32) || /[<>:"|?*]/.test(segment) ||
    /[. ]$/.test(segment) || DEVICE_NAME.test(segment);
}

/** A portable relative path with `/` separators. Rejects anything that could escape its folder or fail on Windows. */
export function assetPath(value: string): string {
  const normalized = value.replaceAll("\\", "/");
  if (normalized === "" || normalized.split("/").some(unsafeSegment)) {
    throw new MoonwellError(`Invalid asset path: ${value}`, {
      hint: "Use a relative path such as icons/BTNSword.blp, without .., drive letters or characters Windows forbids.",
    });
  }
  return normalized;
}

/** An in-map path an asset may be imported as. Map internals such as war3map.lua are never replaced. */
export function targetPath(value: string): string {
  const normalized = assetPath(value);
  const internal = /^(?:war3map|war3campaign|\(listfile\)|\(attributes\)|\(signature\))/i.test(normalized) &&
    !/^war3mapImported\//i.test(normalized);
  if (internal || /^scripts\/war3map\./i.test(normalized)) {
    throw new MoonwellError(`Reserved map path: ${value}`, {
      hint: "Assets cannot replace map internals such as war3map.lua or war3map.imp.",
    });
  }
  return normalized;
}

export async function lstatOrUndefined(path: string): Promise<Deno.FileInfo | undefined> {
  try {
    return await Deno.lstat(path);
  } catch (error) {
    if (error instanceof Deno.errors.NotFound) return undefined;
    throw error;
  }
}

function symlinkError(path: string): MoonwellError {
  return new MoonwellError(`Symlinks are not supported: ${path}`, {
    hint: "Replace the link (or Windows junction) with the real files.",
  });
}

/**
 * Joins `relative` under `root`, refusing a symlink at every step below `root` so asset data cannot leave its folder.
 * `root` itself is trusted: it is chosen by the caller (such as a project opened through a junction), not an escape.
 */
export async function safeJoin(root: string, relative: string): Promise<string> {
  let current = resolve(root);
  for (const segment of assetPath(relative).split("/")) {
    current = join(current, segment);
    if ((await lstatOrUndefined(current))?.isSymlink) throw symlinkError(current);
  }
  return current;
}

const compare = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);

/** Every regular file below `root`, as case-insensitive key → path relative to `root` with `/`. Missing root: empty. */
export async function scanFiles(root: string): Promise<Map<string, string>> {
  const result = new Map<string, string>();
  const info = await lstatOrUndefined(root);
  if (info === undefined) return result;
  if (info.isSymlink) throw symlinkError(root);
  if (!info.isDirectory) throw new MoonwellError(`Expected a folder: ${root}`);
  const seen = new Set<string>();
  const visit = async (relative: string): Promise<void> => {
    const folder = relative === "" ? root : await safeJoin(root, relative);
    const entries = await Array.fromAsync(Deno.readDir(folder));
    entries.sort((a, b) => compare(a.name, b.name));
    for (const entry of entries) {
      const name = assetPath(relative === "" ? entry.name : `${relative}/${entry.name}`);
      if (seen.has(pathKey(name))) {
        throw new MoonwellError(`Two paths differ only in letter case: ${name}`, {
          hint: "Warcraft III paths ignore letter case; rename one of them.",
        });
      }
      seen.add(pathKey(name));
      if (entry.isSymlink) throw symlinkError(join(root, name));
      if (entry.isDirectory) await visit(name);
      else if (entry.isFile) result.set(pathKey(name), name);
      else throw new MoonwellError(`Not a regular file: ${join(root, name)}`);
    }
  };
  await visit("");
  return result;
}
