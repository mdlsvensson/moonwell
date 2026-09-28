import { MoonwellError } from "../shared/errors.ts";
import { sha256Hex } from "../shared/fs.ts";
import { extractZip, zipComment } from "../yue/unzip.ts";

/**
 * A GitHub tag archive's files, without their single top folder (whose name GitHub derives from the repository and
 * tag), and the commit SHA from the zip comment (spec §2, §4.2). `extractZip` skips directory entries, so every name
 * here is a file.
 */
export async function readGitHubArchive(
  bytes: Uint8Array,
): Promise<{ commit: string; files: Map<string, Uint8Array> }> {
  const commit = zipComment(bytes).trim();
  if (!/^[0-9a-f]{40}$/.test(commit)) throw new MoonwellError("The archive's comment is not a commit SHA.");
  const files = new Map<string, Uint8Array>();
  let top: string | undefined;
  for (const [name, data] of await extractZip(bytes)) {
    const slash = name.indexOf("/");
    const first = slash < 0 ? undefined : name.slice(0, slash);
    if (first === undefined || (top !== undefined && first !== top)) {
      throw new MoonwellError("The archive does not have a single top folder.");
    }
    top = first;
    files.set(name.slice(slash + 1), data);
  }
  return { commit, files };
}

/** `sha256:` and the SHA-256 of `<path>\n<sha256 of its bytes>\n` for each file, sorted by path (spec §4.3). */
export async function filesHash(files: ReadonlyMap<string, Uint8Array>): Promise<string> {
  let text = "";
  for (const path of [...files.keys()].sort()) text += `${path}\n${await sha256Hex(files.get(path)!)}\n`;
  return `sha256:${await sha256Hex(new TextEncoder().encode(text))}`;
}
