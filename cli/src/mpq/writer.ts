import { deflate } from "../shared/compression.ts";
import { MoonwellError } from "../shared/errors.ts";
import { BLOCK_TABLE_KEY, encryptBlock, HASH_TABLE_KEY, hashString, HashType } from "./crypto.ts";

export interface MpqFile {
  /** Archive path with backslash separators, e.g. "war3mapImported\\icon.blp". */
  name: string;
  data: Uint8Array;
}

const HEADER_SIZE = 32;
const MPQ_MAGIC = 0x1a51504d; // "MPQ\x1A"
const FILE_EXISTS = 0x80000000;
const FILE_COMPRESS = 0x00000200;
const EMPTY = 0xffffffff;

/** Writes an MPQ format-1 archive, optionally after a 512-byte-aligned prefix (the HM3W map header). */
export async function writeMpq(
  files: MpqFile[],
  options: { prefix?: Uint8Array; sectorSizeShift?: number } = {},
): Promise<Uint8Array> {
  const prefix = options.prefix ?? new Uint8Array(0);
  if (prefix.length % 512 !== 0) throw new MoonwellError("The archive prefix must be a multiple of 512 bytes.");
  const shift = options.sectorSizeShift ?? 3;
  const sectorSize = 512 << shift;

  const seen = new Map<string, string>();
  for (const file of files) {
    const key = file.name.toUpperCase();
    const previous = seen.get(key);
    if (previous !== undefined) {
      throw new MoonwellError(`Duplicate archive path '${file.name}' (also '${previous}').`, {
        hint: "Archive paths are case-insensitive; rename one of the files.",
      });
    }
    seen.set(key, file.name);
  }
  const content = files.filter((file) => file.name.toUpperCase() !== "(LISTFILE)");
  const listfile = new TextEncoder().encode(content.map((file) => `${file.name}\r\n`).join(""));
  const entries: MpqFile[] = [...content, { name: "(listfile)", data: listfile }];

  const bodies: Uint8Array[] = [];
  const blocks = new Uint32Array(entries.length * 4);
  let offset = HEADER_SIZE;
  for (let i = 0; i < entries.length; i++) {
    const body = await encodeFile(entries[i].data, sectorSize);
    blocks[i * 4] = offset;
    blocks[i * 4 + 1] = body.length;
    blocks[i * 4 + 2] = entries[i].data.length;
    blocks[i * 4 + 3] = entries[i].data.length > 0 ? (FILE_EXISTS | FILE_COMPRESS) >>> 0 : FILE_EXISTS;
    bodies.push(body);
    offset += body.length;
  }

  let hashSize = 16;
  while (hashSize < entries.length * 1.5) hashSize *= 2;
  const hashes = new Uint32Array(hashSize * 4).fill(EMPTY);
  entries.forEach((entry, index) => {
    let slot = hashString(entry.name, HashType.TableOffset) & (hashSize - 1);
    while (hashes[slot * 4 + 3] !== EMPTY) slot = (slot + 1) & (hashSize - 1);
    hashes[slot * 4] = hashString(entry.name, HashType.NameA);
    hashes[slot * 4 + 1] = hashString(entry.name, HashType.NameB);
    hashes[slot * 4 + 2] = 0; // locale 0 (neutral), platform 0
    hashes[slot * 4 + 3] = index;
  });
  encryptBlock(hashes, HASH_TABLE_KEY);
  encryptBlock(blocks, BLOCK_TABLE_KEY);

  const hashPosition = offset;
  const blockPosition = hashPosition + hashSize * 16;
  const archiveSize = blockPosition + entries.length * 16;
  const out = new Uint8Array(prefix.length + archiveSize);
  out.set(prefix, 0);
  const header = new DataView(out.buffer, prefix.length, HEADER_SIZE);
  header.setUint32(0, MPQ_MAGIC, true);
  header.setUint32(4, HEADER_SIZE, true);
  header.setUint32(8, archiveSize, true);
  header.setUint16(12, 0, true); // format version 1
  header.setUint16(14, shift, true);
  header.setUint32(16, hashPosition, true);
  header.setUint32(20, blockPosition, true);
  header.setUint32(24, hashSize, true);
  header.setUint32(28, entries.length, true);
  let at = prefix.length + HEADER_SIZE;
  for (const body of bodies) {
    out.set(body, at);
    at += body.length;
  }
  writeWords(out, prefix.length + hashPosition, hashes);
  writeWords(out, prefix.length + blockPosition, blocks);
  return out;
}

function writeWords(target: Uint8Array, at: number, words: Uint32Array): void {
  const view = new DataView(target.buffer, target.byteOffset + at, words.length * 4);
  words.forEach((word, i) => view.setUint32(i * 4, word, true));
}

/** Sector offset table followed by sectors; each sector is zlib (mask 0x02) or raw when compression does not help. */
async function encodeFile(data: Uint8Array, sectorSize: number): Promise<Uint8Array> {
  if (data.length === 0) return new Uint8Array(0);
  const count = Math.ceil(data.length / sectorSize);
  const sectors: Uint8Array[] = [];
  for (let i = 0; i < count; i++) {
    const raw = data.subarray(i * sectorSize, Math.min(data.length, (i + 1) * sectorSize));
    const packed = await deflate(raw);
    if (packed.length + 1 < raw.length) {
      const sector = new Uint8Array(packed.length + 1);
      sector[0] = 0x02;
      sector.set(packed, 1);
      sectors.push(sector);
    } else {
      sectors.push(raw);
    }
  }
  const tableSize = (count + 1) * 4;
  const out = new Uint8Array(tableSize + sectors.reduce((sum, sector) => sum + sector.length, 0));
  const table = new DataView(out.buffer);
  let position = tableSize;
  sectors.forEach((sector, i) => {
    table.setUint32(i * 4, position, true);
    out.set(sector, position);
    position += sector.length;
  });
  table.setUint32(count * 4, position, true);
  return out;
}
