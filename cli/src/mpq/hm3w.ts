/** The legacy 512-byte "HM3W" map header: magic, 4 unused bytes, name (NUL-terminated), flags, max players. */
export function buildHm3wHeader(name: string, flags = 0, maxPlayers = 0): Uint8Array {
  const out = new Uint8Array(512);
  const view = new DataView(out.buffer);
  out.set([0x48, 0x4d, 0x33, 0x57], 0);
  const encoded = new TextEncoder().encode(name).subarray(0, 512 - 8 - 1 - 8);
  out.set(encoded, 8);
  const at = 8 + encoded.length + 1;
  view.setUint32(at, flags >>> 0, true);
  view.setUint32(at + 4, maxPlayers >>> 0, true);
  return out;
}
