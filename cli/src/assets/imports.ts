import { MoonwellError } from "../shared/errors.ts";

/** One entry of war3map.imp (version 1), the World Editor's import index. */
export interface ImportEntry {
  /** 0, 5 or 8: the file lives under war3mapImported\. 10 or 13: `path` is the full in-map path. */
  flag: number;
  path: string;
}

const FLAGS = new Set([0, 5, 8, 10, 13]);

/** The in-map path of an import. */
export function importPath(entry: ImportEntry): string {
  return entry.flag === 10 || entry.flag === 13 ? entry.path : `war3mapImported\\${entry.path}`;
}

export function readImports(bytes: Uint8Array, file = "war3map.imp"): ImportEntry[] {
  const corrupt = (problem: string): never => {
    throw new MoonwellError(`war3map.imp is unreadable: ${problem}.`, {
      file,
      hint: "Open and re-save the map in World Editor.",
    });
  };
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  if (bytes.length < 8) corrupt("it is truncated");
  if (view.getUint32(0, true) !== 1) corrupt(`version ${view.getUint32(0, true)} is not supported (expected 1)`);
  const count = view.getUint32(4, true);
  const decoder = new TextDecoder("utf-8", { fatal: true });
  const entries: ImportEntry[] = [];
  let offset = 8;
  for (let i = 0; i < count; i++) {
    if (offset >= bytes.length) corrupt("it is truncated");
    const flag = bytes[offset++];
    if (!FLAGS.has(flag)) corrupt(`entry ${i} has unknown flag ${flag}`);
    const end = bytes.indexOf(0, offset);
    if (end < 0) corrupt("it is truncated");
    if (end === offset) corrupt(`entry ${i} has an empty path`);
    let path = "";
    try {
      path = decoder.decode(bytes.subarray(offset, end));
    } catch {
      corrupt(`entry ${i} is not valid UTF-8`);
    }
    entries.push({ flag, path });
    offset = end + 1;
  }
  if (offset !== bytes.length) corrupt("it has trailing data");
  return entries;
}

export function writeImports(entries: ImportEntry[]): Uint8Array {
  const encoder = new TextEncoder();
  const paths = entries.map((entry) => encoder.encode(entry.path));
  const bytes = new Uint8Array(8 + paths.reduce((size, path) => size + path.length + 2, 0));
  const view = new DataView(bytes.buffer);
  view.setUint32(0, 1, true);
  view.setUint32(4, entries.length, true);
  let offset = 8;
  entries.forEach((entry, i) => {
    bytes[offset++] = entry.flag;
    bytes.set(paths[i], offset);
    offset += paths[i].length + 1; // the terminating NUL is already 0
  });
  return bytes;
}
