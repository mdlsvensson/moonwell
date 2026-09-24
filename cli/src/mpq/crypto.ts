export const HashType = { TableOffset: 0, NameA: 1, NameB: 2, FileKey: 3 } as const;
export type HashType = typeof HashType[keyof typeof HashType];

const CRYPT_TABLE = (() => {
  const table = new Uint32Array(0x500);
  let seed = 0x00100001;
  for (let index1 = 0; index1 < 0x100; index1++) {
    for (let i = 0, index2 = index1; i < 5; i++, index2 += 0x100) {
      seed = (seed * 125 + 3) % 0x2aaaab;
      const high = (seed & 0xffff) << 0x10;
      seed = (seed * 125 + 3) % 0x2aaaab;
      table[index2] = (high | (seed & 0xffff)) >>> 0;
    }
  }
  return table;
})();

/** The MPQ string hash; names are upper-cased (ASCII only) as the game does. */
export function hashString(text: string, type: HashType): number {
  let seed1 = 0x7fed7fed;
  let seed2 = 0xeeeeeeee;
  for (const byte of new TextEncoder().encode(text)) {
    const char = byte >= 0x61 && byte <= 0x7a ? byte - 0x20 : byte;
    seed1 = (CRYPT_TABLE[(type << 8) + char] ^ ((seed1 + seed2) >>> 0)) >>> 0;
    seed2 = (char + seed1 + seed2 + ((seed2 << 5) >>> 0) + 3) >>> 0;
  }
  return seed1;
}

export const HASH_TABLE_KEY = hashString("(hash table)", HashType.FileKey);
export const BLOCK_TABLE_KEY = hashString("(block table)", HashType.FileKey);

function nextKey(key: number): number {
  return ((((~key << 0x15) >>> 0) + 0x11111111) >>> 0 | (key >>> 0x0b)) >>> 0;
}

export function encryptBlock(data: Uint32Array, key: number): void {
  let seed = 0xeeeeeeee;
  for (let i = 0; i < data.length; i++) {
    seed = (seed + CRYPT_TABLE[0x400 + (key & 0xff)]) >>> 0;
    const plain = data[i];
    data[i] = (plain ^ ((key + seed) >>> 0)) >>> 0;
    key = nextKey(key);
    seed = (plain + seed + ((seed << 5) >>> 0) + 3) >>> 0;
  }
}

export function decryptBlock(data: Uint32Array, key: number): void {
  let seed = 0xeeeeeeee;
  for (let i = 0; i < data.length; i++) {
    seed = (seed + CRYPT_TABLE[0x400 + (key & 0xff)]) >>> 0;
    const plain = (data[i] ^ ((key + seed) >>> 0)) >>> 0;
    data[i] = plain;
    key = nextKey(key);
    seed = (plain + seed + ((seed << 5) >>> 0) + 3) >>> 0;
  }
}
