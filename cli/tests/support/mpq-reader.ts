import { BLOCK_TABLE_KEY, decryptBlock, HASH_TABLE_KEY, hashString, HashType } from "../../src/mpq/crypto.ts";
import { inflate } from "../../src/shared/compression.ts";

/** Test-only MPQ v1 reader: enough to verify what writeMpq produces. */
export function openMpq(bytes: Uint8Array) {
  const at = (offset: number, length: number) => new DataView(bytes.buffer, bytes.byteOffset + offset, length);
  let base = -1;
  for (let offset = 0; offset + 32 <= bytes.length; offset += 512) {
    if (at(offset, 4).getUint32(0, true) === 0x1a51504d) {
      base = offset;
      break;
    }
  }
  if (base < 0) throw new Error("no MPQ header found");
  const header = at(base, 32);
  const sectorSize = 512 << header.getUint16(14, true);
  const hashSize = header.getUint32(24, true);
  const blockCount = header.getUint32(28, true);
  const table = (position: number, count: number, key: number) => {
    const view = at(base + position, count * 4);
    const words = new Uint32Array(count);
    for (let i = 0; i < count; i++) words[i] = view.getUint32(i * 4, true);
    decryptBlock(words, key);
    return words;
  };
  const hashes = table(header.getUint32(16, true), hashSize * 4, HASH_TABLE_KEY);
  const blocks = table(header.getUint32(20, true), blockCount * 4, BLOCK_TABLE_KEY);

  const find = (name: string): number | undefined => {
    const a = hashString(name, HashType.NameA);
    const b = hashString(name, HashType.NameB);
    let slot = hashString(name, HashType.TableOffset) & (hashSize - 1);
    for (let probes = 0; probes < hashSize; probes++) {
      const index = hashes[slot * 4 + 3];
      if (index === 0xffffffff) return undefined;
      if (hashes[slot * 4] === a && hashes[slot * 4 + 1] === b) return index;
      slot = (slot + 1) & (hashSize - 1);
    }
    return undefined;
  };

  const read = async (name: string): Promise<Uint8Array | undefined> => {
    const index = find(name);
    if (index === undefined) return undefined;
    const start = base + blocks[index * 4];
    const size = blocks[index * 4 + 2];
    const flags = blocks[index * 4 + 3];
    if (size === 0) return new Uint8Array(0);
    if ((flags & 0x200) === 0) return bytes.slice(start, start + size);
    const count = Math.ceil(size / sectorSize);
    const offsets = at(start, (count + 1) * 4);
    const out = new Uint8Array(size);
    for (let i = 0; i < count; i++) {
      const chunk = bytes.subarray(
        start + offsets.getUint32(i * 4, true),
        start + offsets.getUint32((i + 1) * 4, true),
      );
      const expected = Math.min(sectorSize, size - i * sectorSize);
      if (chunk.length < expected) {
        if (chunk[0] !== 0x02) throw new Error(`unsupported compression mask ${chunk[0]}`);
        out.set(await inflate(chunk.subarray(1)), i * sectorSize);
      } else {
        out.set(chunk, i * sectorSize);
      }
    }
    return out;
  };

  return {
    headerOffset: base,
    read,
    listfile: async () => new TextDecoder().decode(await read("(listfile)")).split("\r\n").filter(Boolean),
  };
}
