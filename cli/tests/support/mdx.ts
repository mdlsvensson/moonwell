/** Test-only builder for binary MDX models, using the layouts in the model-paths spec §3.1. */
const encoder = new TextEncoder();

export const EMITTER_USES_MDL = 0x8000;
export const EMITTER_USES_TGA = 0x10000;

export function concat(...parts: Uint8Array[]): Uint8Array {
  const out = new Uint8Array(parts.reduce((size, part) => size + part.length, 0));
  let offset = 0;
  for (const part of parts) {
    out.set(part, offset);
    offset += part.length;
  }
  return out;
}

export function u32(value: number): Uint8Array {
  const bytes = new Uint8Array(4);
  new DataView(bytes.buffer).setUint32(0, value, true);
  return bytes;
}

export function f32(value: number): Uint8Array {
  const bytes = new Uint8Array(4);
  new DataView(bytes.buffer).setFloat32(0, value, true);
  return bytes;
}

/** A copy of `bytes` with the uint32 at `offset` replaced. */
export function setU32(bytes: Uint8Array, offset: number, value: number): Uint8Array {
  const copy = bytes.slice();
  new DataView(copy.buffer).setUint32(offset, value, true);
  return copy;
}

/** A NUL-padded fixed-size string field. */
export function fixed(value: string, size: number): Uint8Array {
  const bytes = new Uint8Array(size);
  bytes.set(encoder.encode(value).subarray(0, size - 1));
  return bytes;
}

export function chunk(tag: string, body: Uint8Array): Uint8Array {
  return concat(encoder.encode(tag), u32(body.length), body);
}

export function mdx(...chunks: Uint8Array[]): Uint8Array {
  return concat(encoder.encode("MDLX"), ...chunks);
}

export function texture(path: string, replaceableId = 0): Uint8Array {
  return concat(u32(replaceableId), fixed(path, 260), u32(0));
}

/** A node; `extra` stands in for animation tracks, so readers must step over nodes by their size. */
export function node(name: string, flags = 0, extra: Uint8Array = new Uint8Array(0)): Uint8Array {
  return concat(u32(96 + extra.length), fixed(name, 80), u32(0), u32(0xffffffff), u32(flags), extra);
}

/** A record whose leading uint32 counts the whole record, itself included. */
function record(...parts: Uint8Array[]): Uint8Array {
  const body = concat(...parts);
  return concat(u32(body.length + 4), body);
}

export function emitter(path: string, flags = EMITTER_USES_MDL): Uint8Array {
  return record(
    node("Emitter", flags, new Uint8Array(8)),
    f32(1), // emission rate
    f32(0), // gravity
    f32(0), // longitude
    f32(0), // latitude
    fixed(path, 260),
    f32(1), // life span
    f32(1), // speed
    new Uint8Array(12), // tracks
  );
}

export function attachment(path: string): Uint8Array {
  return record(node("Attachment"), fixed(path, 260), u32(0), new Uint8Array(6));
}

export function popcorn(path: string): Uint8Array {
  return record(
    node("Popcorn"),
    f32(1), // life span
    f32(1), // emission rate
    f32(1), // speed
    f32(1), // colour r
    f32(1), // colour g
    f32(1), // colour b
    f32(1), // alpha
    u32(0), // replaceable id
    fixed(path, 260),
    fixed("", 260), // animation visibility guide
  );
}

export function faceEffect(type: string, path: string): Uint8Array {
  return concat(fixed(type, 80), fixed(path, 260));
}
