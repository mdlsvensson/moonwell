import { MoonwellError } from "../shared/errors.ts";
import { readGitHubArchive } from "./archive.ts";

/** How libraries are downloaded: `fetch`, or a stand-in in tests. */
export type Fetch = (url: string) => Promise<Response>;

/** The zip archive of a GitHub tag (spec §2); a tag's `/` stays a path separator. */
export function archiveUrl(github: string, tag: string): string {
  return `https://codeload.github.com/${github}/zip/refs/tags/${encodeURIComponent(tag).replaceAll("%2F", "/")}`;
}

export const reasonOf = (cause: unknown) => cause instanceof Error ? cause.message : String(cause);

/**
 * Downloads a tag of library `key` from GitHub: its commit and files, without the archive's top folder, every path
 * checked to stay inside the library's folder. Failures are MoonwellErrors naming `manifest`.
 */
export async function downloadTag(
  key: string,
  github: string,
  tag: string,
  manifest: string,
  fetch: Fetch,
): Promise<{ commit: string; files: Map<string, Uint8Array> }> {
  const url = archiveUrl(github, tag);
  let bytes: Uint8Array;
  try {
    const response = await fetch(url);
    if (!response.ok) {
      await response.body?.cancel().catch(() => {});
      if (response.status === 404) {
        throw new MoonwellError(`Library ${key}: ${github} has no tag ${tag}.`, {
          file: manifest,
          hint: `See the tags at https://github.com/${github}/tags.`,
        });
      }
      throw new MoonwellError(`Downloading library ${key} failed: HTTP ${response.status}.`, {
        file: manifest,
        hint: "Try again later.",
      });
    }
    bytes = new Uint8Array(await response.arrayBuffer());
  } catch (cause) {
    if (cause instanceof MoonwellError) throw cause;
    throw new MoonwellError(`Downloading library ${key} failed: ${reasonOf(cause)}`, {
      file: manifest,
      cause,
      hint: `Check your connection and that https://github.com/${github} exists.`,
    });
  }
  const hint = `Check ${url} in a browser.`;
  let archive: { commit: string; files: Map<string, Uint8Array> };
  try {
    archive = await readGitHubArchive(bytes);
  } catch (cause) {
    const message = `The download of library ${key} is not a GitHub tag archive: ${reasonOf(cause)}`;
    throw new MoonwellError(message, { file: manifest, cause, hint });
  }
  for (const path of archive.files.keys()) {
    if (!isSafePath(path)) {
      throw new MoonwellError(`The download of library ${key} has an unsafe path: ${path}`, { file: manifest, hint });
    }
  }
  return archive;
}

/** A relative POSIX path of plain names: no leading `/`, no empty, `.` or `..` segment, no `\` or `:`. */
function isSafePath(path: string): boolean {
  return !path.startsWith("/") && !/[\\:]/.test(path) &&
    path.split("/").every((segment) => segment !== "" && segment !== "." && segment !== "..");
}
