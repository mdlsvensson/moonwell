import { MoonwellError } from "../shared/errors.ts";

/** Leveled tables (`w3a`, `w3d`, `w3q`) store a level and a data pointer with each modification. */
export type TableKind = "simple" | "leveled";

export type ModValue =
  | { type: "int"; value: number }
  | { type: "real" | "unreal"; value: number }
  | { type: "string"; value: string };

/** `level` and `column` (the data pointer) are 0 in simple tables; `end` is the end token, `"\0\0\0\0"` for 0. */
export interface Modification {
  field: string;
  level: number;
  column: number;
  value: ModValue;
  end: string;
  start: number;
  stop: number;
}

/** v1/v2 objects have one implicit set with flag 0. `start`/`stop` let the writer copy the object verbatim. */
export interface ObjectEntry {
  base: string;
  id: string;
  sets: { flag: number; mods: Modification[] }[];
  start: number;
  stop: number;
}

/** `countOffset` is the object count's offset; `start`/`stop` span the objects after it. */
export interface ModTable {
  countOffset: number;
  start: number;
  stop: number;
  objects: ObjectEntry[];
}

export interface ModFile {
  version: number;
  original: ModTable;
  custom: ModTable;
}

const VAR_TYPES = ["int", "real", "unreal", "string"] as const;
// World Editor 3.00 writes one set per object; the limit only rejects garbage counts.
const MAX_SETS = 64;

export function tableKind(fileName: string): TableKind {
  return /\.(w3a|w3d|w3q)$/i.test(fileName) ? "leveled" : "simple";
}

class ModFileReader {
  offset = 0;
  readonly view: DataView;

  constructor(readonly bytes: Uint8Array, readonly file: string) {
    this.view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  }

  invalid(problem: string, cause?: unknown): never {
    throw new MoonwellError(`Cannot read object data: ${problem}.`, {
      file: this.file,
      hint: "Open and re-save this map in World Editor 3.00.",
      cause,
    });
  }

  skip(size: number): number {
    const start = this.offset;
    if (this.offset + size > this.bytes.length) this.invalid("truncated file");
    this.offset += size;
    return start;
  }

  int(): number {
    return this.view.getInt32(this.skip(4), true);
  }

  float(): number {
    return this.view.getFloat32(this.skip(4), true);
  }

  id(): string {
    const start = this.skip(4);
    return String.fromCharCode(...this.bytes.subarray(start, start + 4));
  }

  text(): string {
    const start = this.offset;
    const end = this.bytes.indexOf(0, start);
    if (end < 0) this.invalid("unterminated string");
    this.offset = end + 1;
    try {
      return new TextDecoder("utf-8", { fatal: true, ignoreBOM: true }).decode(this.bytes.subarray(start, end));
    } catch (cause) {
      return this.invalid("invalid UTF-8 in a string", cause);
    }
  }

  /** Reads a count and rejects one whose items could not fit in the remaining bytes. */
  count(what: string, minSize: number): number {
    const count = this.int();
    if (count < 0 || count * minSize > this.bytes.length - this.offset) {
      this.invalid(`${what} count ${count} past end of file`);
    }
    return count;
  }
}

function readTable(r: ModFileReader, version: number, kind: TableKind): ModTable {
  const leveled = kind === "leveled";
  // Smallest encodings: an empty object, and a modification with a one-byte (empty) string.
  const minObject = version >= 3 ? 20 : 12;
  const minMod = (leveled ? 16 : 8) + 1 + 4;
  const countOffset = r.offset;
  const objects = Array.from({ length: r.count("object", minObject) }, (): ObjectEntry => {
    const start = r.offset;
    const base = r.id(), id = r.id();
    const setCount = version >= 3 ? r.int() : 1;
    if (setCount < 1 || setCount > MAX_SETS) r.invalid(`unsupported set count ${setCount}`);
    const sets = Array.from({ length: setCount }, () => {
      const flag = version >= 3 ? r.int() : 0;
      const mods = Array.from({ length: r.count("modification", minMod) }, (): Modification => {
        const start = r.offset;
        const field = r.id();
        const varType = r.int();
        const level = leveled ? r.int() : 0;
        const column = leveled ? r.int() : 0;
        const type = VAR_TYPES[varType];
        if (type === undefined) r.invalid(`unknown value type ${varType}`);
        const value: ModValue = type === "string"
          ? { type, value: r.text() }
          : type === "int"
          ? { type, value: r.int() }
          : { type, value: r.float() };
        return { field, level, column, value, end: r.id(), start, stop: r.offset };
      });
      return { flag, mods };
    });
    return { base, id, sets, start, stop: r.offset };
  });
  return { countOffset, start: countOffset + 4, stop: r.offset, objects };
}

/** Reads a modification file (`w3u`, `w3t`, `w3b`, `w3d`, `w3a`, `w3h`, `w3q` and their skin files). */
export function readModFile(bytes: Uint8Array, kind: TableKind, file: string): ModFile {
  const r = new ModFileReader(bytes, file);
  const version = r.int();
  if (![1, 2, 3].includes(version)) r.invalid(`unsupported version ${version}`);
  const original = readTable(r, version, kind);
  const custom = readTable(r, version, kind);
  if (r.offset !== bytes.length) r.invalid("trailing bytes after the custom objects");
  return { version, original, custom };
}

/** An object Moonwell adds to the custom table. `level` and `column` must be 0 in simple tables. */
export interface NewObject {
  base: string;
  id: string;
  mods: { field: string; level: number; column: number; value: ModValue }[];
}

// v3: set count 1 and flag 0 per object, end token 0 after each custom modification (names fixture, V3 and V4).
// v1/v2: no set fields and end token 0, as the reference library writes version 2 files (mdx-m3-viewer-th
// `ModifiedObject.save` and `Modification.save`, whose end token defaults to 0; war3-objectdata-th, used in game).
const SET_COUNT = 1;
const SET_FLAG = 0;
const END_TOKEN = 0;
// Version and empty original table of a file World Editor 3.00 writes when the map has none (names fixture).
export const NEW_FILE_VERSION = 3;
const FLOAT32_MAX = 3.4028234663852886e38;

/** Encodes Moonwell's objects. Bad values are internal errors: the resolver rejects them before planning. */
class ModFileWriter {
  readonly chunks: Uint8Array[] = [];

  int(value: number): void {
    if (!Number.isInteger(value) || value < -(2 ** 31) || value >= 2 ** 31) {
      throw new Error(`Cannot write ${value} as an int32.`);
    }
    const bytes = new Uint8Array(4);
    new DataView(bytes.buffer).setInt32(0, value, true);
    this.chunks.push(bytes);
  }

  float(value: number): void {
    if (!Number.isFinite(value) || Math.abs(value) > FLOAT32_MAX) {
      throw new Error(`Cannot write ${value} as a float32.`);
    }
    const bytes = new Uint8Array(4);
    new DataView(bytes.buffer).setFloat32(0, value, true);
    this.chunks.push(bytes);
  }

  id(value: string): void {
    const codes = Array.from(value, (c) => c.charCodeAt(0));
    if (codes.length !== 4 || codes.some((code) => code > 0xff)) {
      throw new Error(`Cannot write ${JSON.stringify(value)} as an object id: it must be 4 Latin-1 characters.`);
    }
    this.chunks.push(new Uint8Array(codes));
  }

  text(value: string): void {
    if (value.includes("\0") || !value.isWellFormed()) {
      throw new Error(`Cannot write ${JSON.stringify(value)}: it contains NUL or an unpaired surrogate.`);
    }
    this.chunks.push(new TextEncoder().encode(value), new Uint8Array(1));
  }

  object({ base, id, mods }: NewObject, version: number, kind: TableKind): void {
    this.id(base);
    this.id(id);
    if (version >= 3) {
      this.int(SET_COUNT);
      this.int(SET_FLAG);
    }
    this.int(mods.length);
    for (const { field, level, column, value } of mods) {
      this.id(field);
      this.int(VAR_TYPES.indexOf(value.type));
      if (kind === "leveled") {
        this.int(level);
        this.int(column);
      } else if (level !== 0 || column !== 0) {
        throw new Error(`Cannot write ${field} at level ${level}, column ${column}: a simple table has neither.`);
      }
      if (value.type === "string") this.text(value.value);
      else if (value.type === "int") this.int(value.value);
      else this.float(value.value);
      this.int(END_TOKEN);
    }
  }

  bytes(): Uint8Array {
    const out = new Uint8Array(this.chunks.reduce((size, chunk) => size + chunk.length, 0));
    let offset = 0;
    for (const chunk of this.chunks) {
      out.set(chunk, offset);
      offset += chunk.length;
    }
    return out;
  }
}

/**
 * Appends `objects`, in the order given, to the custom table of `source`, copying every existing byte verbatim.
 * Without `source`, writes a new file with an empty original table.
 */
export function appendObjects(
  source: Uint8Array | undefined,
  kind: TableKind,
  objects: NewObject[],
  file: string,
): Uint8Array {
  const w = new ModFileWriter();
  if (source === undefined) {
    w.int(NEW_FILE_VERSION);
    w.int(0);
    w.int(objects.length);
    for (const object of objects) w.object(object, NEW_FILE_VERSION, kind);
    return w.bytes();
  }
  const { version, custom } = readModFile(source, kind, file);
  w.chunks.push(source.subarray(0, custom.countOffset));
  w.int(custom.objects.length + objects.length);
  // The custom table runs to the end of the file: the reader rejects trailing bytes.
  w.chunks.push(source.subarray(custom.start, custom.stop));
  for (const object of objects) w.object(object, version, kind);
  return w.bytes();
}
