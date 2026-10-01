import { MoonwellError } from "../shared/errors.ts";

/** The file a library describes its own layout with, at its root (spec: library assets §2). */
export const LIBRARY_FILE = "moonwell-library.json";

/** What a library says about itself. `null` is "not given". Both are folders inside the library, with `/`. */
export interface LibraryFile {
  /** The folder module names start from. */
  dir: string | null;
  /** The folder whose files the map imports. */
  assets: string | null;
}

const KEYS = ["dir", "assets"] as const;

/** A relative folder path of plain names: no empty, `.` or `..` segment, no `\` and no `:`. */
function isFolder(value: unknown): value is string {
  return typeof value === "string" && !/[\\:]/.test(value) &&
    value.split("/").every((segment) => segment !== "" && segment !== "." && segment !== "..");
}

/**
 * Reads a library's `moonwell-library.json`. `bytes` is the file's content, or `undefined` for a library without one,
 * which ships nothing but modules from its root. `where` names the file in errors: a path, or a URL for a download.
 */
export function parseLibraryFile(key: string, bytes: Uint8Array | undefined, where: string): LibraryFile {
  if (bytes === undefined) return { dir: null, assets: null };
  const fail = (problem: string, hint: string): never => {
    throw new MoonwellError(`Library ${key}: ${LIBRARY_FILE} ${problem}`, { file: where, hint });
  };
  const report = "Report it to the library's author, or use another tag of the library.";
  let data: unknown;
  try {
    data = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes));
  } catch {
    fail("is not valid JSON.", report);
  }
  if (typeof data !== "object" || data === null || Array.isArray(data)) fail("is not a JSON object.", report);
  const fields = data as Record<string, unknown>;
  for (const name of Object.keys(fields).sort()) {
    if (!(KEYS as readonly string[]).includes(name)) {
      fail(
        `has an unknown key "${name}".`,
        `This Moonwell knows ${KEYS.join(" and ")}; the library may need a newer Moonwell.`,
      );
    }
  }
  const folder = (name: (typeof KEYS)[number]): string | null => {
    const value = fields[name];
    if (value === undefined) return null;
    if (!isFolder(value)) {
      fail(`has ${name} = ${JSON.stringify(value)}, which is not a folder inside the library.`, report);
    }
    return value as string;
  };
  return { dir: folder("dir"), assets: folder("assets") };
}
