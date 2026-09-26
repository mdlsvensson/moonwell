import { MoonwellError } from "../shared/errors.ts";

/** Offset-based reader; fields this tool does not own stay in their original bytes. */
export interface Field<T> {
  start: number;
  end: number;
  value: T;
}

class MapInfoReader {
  offset = 0;
  readonly view: DataView;

  constructor(readonly bytes: Uint8Array, readonly file: string) {
    this.view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  }

  invalid(problem: string): never {
    throw new MoonwellError(`Cannot read map settings: ${problem}.`, {
      file: this.file,
      hint: "Open and re-save this map in World Editor; extended settings require Lua script mode.",
    });
  }

  skip(size: number): void {
    if (!Number.isSafeInteger(size) || size < 0 || this.offset + size > this.bytes.length) {
      this.invalid("truncated war3map.w3i");
    }
    this.offset += size;
  }

  number(float = false, size = 4): Field<number> {
    const start = this.offset;
    this.skip(size);
    return {
      start,
      end: this.offset,
      value: size === 1
        ? this.view.getUint8(start)
        : float
        ? this.view.getFloat32(start, true)
        : this.view.getInt32(start, true),
    };
  }

  text(): Field<string> {
    const start = this.offset;
    const end = this.bytes.indexOf(0, start);
    if (end < 0) this.invalid("unterminated string in war3map.w3i");
    this.offset = end + 1;
    try {
      return {
        start,
        end: this.offset,
        value: new TextDecoder("utf-8", { fatal: true }).decode(this.bytes.subarray(start, end)),
      };
    } catch (cause) {
      throw new MoonwellError("Cannot read map settings: invalid UTF-8 in war3map.w3i.", {
        file: this.file,
        hint: "Open and re-save this map in World Editor.",
        cause,
      });
    }
  }
}

export function readMapInfo(bytes: Uint8Array, extended = false, file = "war3map.w3i") {
  const r = new MapInfoReader(bytes, file);
  const version = r.number().value;
  if (![18, 25, 28, 31, 32, 33, 39].includes(version)) r.invalid(`unsupported war3map.w3i version ${version}`);
  r.skip(version >= 28 ? 24 : 8);
  const info = { name: r.text(), author: r.text(), description: r.text(), recommendedPlayers: r.text() };
  r.skip(56);
  const flags = r.number();
  r.skip(1);
  const background = r.number();
  if (version === 39) r.skip(4);
  const model = version >= 25 ? r.text() : undefined;
  const loadingScreen = { background, model, text: r.text(), title: r.text(), subtitle: r.text() };
  const prefix = { version, info, flags, loadingScreen };
  if (!extended) return { ...prefix, details: undefined };
  if (version < 28) r.invalid("player, force and environment overrides require w3i version 28 or later");
  r.skip(4);
  for (let i = 0; i < 4; i++) r.text();
  const fog = {
    style: r.number(),
    start: r.number(true),
    end: r.number(true),
    density: r.number(true),
    color: Array.from({ length: 4 }, () => r.number(false, 1)),
  };
  r.skip(4);
  if (version === 39) r.skip(24);
  const soundEnvironment = r.text();
  r.skip(1);
  const waterColor = Array.from({ length: 4 }, () => r.number(false, 1));
  if (r.number().value !== 1) r.invalid("map settings require Lua script mode");
  if (version >= 31) r.skip(8);
  if (version >= 32) r.skip(8);
  if (version >= 33) r.skip(4);
  if (version === 39) r.skip(40);
  const playerCount = r.number().value;
  if (playerCount < 1 || playerCount > 24) r.invalid("invalid player count in war3map.w3i");
  const players = Array.from({ length: playerCount }, () => {
    const id = r.number(), controller = r.number(), race = r.number();
    if (version === 39) r.skip(4);
    const fixedStart = r.number(), name = r.text(), x = r.number(true), y = r.number(true);
    r.skip(version >= 31 ? 16 : 8);
    return { id, controller, race, fixedStart, name, x, y };
  });
  if (
    new Set(players.map((p) => p.id.value)).size !== players.length ||
    players.some((p) =>
      p.id.value < 0 || p.id.value > 23 || p.controller.value < 1 || p.controller.value > 4 ||
      p.race.value < 0 || p.race.value > 4 || ![0, 1].includes(p.fixedStart.value) ||
      !Number.isFinite(p.x.value) || !Number.isFinite(p.y.value)
    )
  ) r.invalid("invalid player records in war3map.w3i");
  const forceCount = r.number().value;
  if (forceCount < 1 || forceCount > 24) r.invalid("invalid force count in war3map.w3i");
  const forces = Array.from({ length: forceCount }, () => ({ flags: r.number(), players: r.number(), name: r.text() }));
  return { ...prefix, details: { fog, soundEnvironment, waterColor, players, forces } };
}

export interface ByteEdit {
  start: number;
  end: number;
  bytes: Uint8Array;
}

export function fieldEdit(field: Field<string> | Field<number>, value: string | number, float = false): ByteEdit {
  let bytes: Uint8Array;
  if (typeof value === "string") bytes = new TextEncoder().encode(value + "\0");
  else {
    bytes = new Uint8Array(field.end - field.start);
    const view = new DataView(bytes.buffer);
    if (bytes.length === 1) view.setUint8(0, value);
    else if (float) view.setFloat32(0, value, true);
    else view.setInt32(0, value, true);
  }
  return { start: field.start, end: field.end, bytes };
}

export function applyByteEdits(source: Uint8Array, edits: ByteEdit[]): Uint8Array {
  const ordered = [...edits].sort((a, b) => a.start - b.start);
  let input = 0;
  let size = source.length;
  for (const edit of ordered) {
    if (
      !Number.isSafeInteger(edit.start) || !Number.isSafeInteger(edit.end) || edit.start < 0 ||
      edit.end < edit.start || edit.end > source.length || edit.start < input
    ) {
      throw new Error("Invalid or overlapping map-info edits.");
    }
    size += edit.bytes.length - (edit.end - edit.start);
    input = edit.end;
  }
  if (!Number.isSafeInteger(size) || size < 0) throw new Error("Invalid map-info result size.");
  const result = new Uint8Array(size);
  input = 0;
  let output = 0;
  for (const edit of ordered) {
    result.set(source.subarray(input, edit.start), output);
    output += edit.start - input;
    result.set(edit.bytes, output);
    output += edit.bytes.length;
    input = edit.end;
  }
  result.set(source.subarray(input), output);
  return result;
}
