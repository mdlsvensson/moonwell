import { assertEquals, assertNotEquals, assertRejects } from "@std/assert";
import {
  BLOCK_TABLE_KEY,
  decryptBlock,
  encryptBlock,
  HASH_TABLE_KEY,
  hashString,
  HashType,
} from "../../src/mpq/crypto.ts";
import { writeMpq } from "../../src/mpq/writer.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { openMpq } from "../support/mpq-reader.ts";

Deno.test("hashString matches the well-known table keys", () => {
  assertEquals(HASH_TABLE_KEY, 0xc3af3770);
  assertEquals(BLOCK_TABLE_KEY, 0xec83b3a3);
  assertEquals(hashString("war3map.lua", HashType.NameA), hashString("WAR3MAP.LUA", HashType.NameA));
});

Deno.test("encryptBlock and decryptBlock round trip", () => {
  const words = new Uint32Array([1, 2, 3, 0xffffffff]);
  encryptBlock(words, HASH_TABLE_KEY);
  assertNotEquals([...words], [1, 2, 3, 0xffffffff]);
  decryptBlock(words, HASH_TABLE_KEY);
  assertEquals([...words], [1, 2, 3, 0xffffffff]);
});

function noise(length: number): Uint8Array {
  const out = new Uint8Array(length);
  let x = 1;
  for (let i = 0; i < length; i++) {
    x = (Math.imul(x, 1103515245) + 12345) >>> 0;
    out[i] = x >>> 24;
  }
  return out;
}

Deno.test("writeMpq round trips compressible, incompressible and empty files", async () => {
  const lua = new TextEncoder().encode("print('moonwell')\n".repeat(600));
  const files = [
    { name: "war3map.lua", data: lua },
    { name: "war3mapImported\\noise.bin", data: noise(10000) },
    { name: "empty.txt", data: new Uint8Array() },
  ];
  const archive = openMpq(await writeMpq(files));
  assertEquals(archive.headerOffset, 0);
  assertEquals(await archive.read("war3map.lua"), lua);
  assertEquals(await archive.read("WAR3MAP.LUA"), lua);
  assertEquals(await archive.read("war3mapImported\\noise.bin"), noise(10000));
  assertEquals(await archive.read("empty.txt"), new Uint8Array());
  assertEquals(await archive.read("missing.txt"), undefined);
  assertEquals(await archive.listfile(), ["war3map.lua", "war3mapImported\\noise.bin", "empty.txt"]);
});

Deno.test("writeMpq places the MPQ header after a 512-byte prefix", async () => {
  const prefix = new Uint8Array(512);
  prefix.set([0x48, 0x4d, 0x33, 0x57]);
  const bytes = await writeMpq([{ name: "a.txt", data: new TextEncoder().encode("a") }], { prefix });
  assertEquals(bytes.subarray(0, 4), new Uint8Array([0x48, 0x4d, 0x33, 0x57]));
  const archive = openMpq(bytes);
  assertEquals(archive.headerOffset, 512);
  assertEquals(await archive.read("a.txt"), new TextEncoder().encode("a"));
});

Deno.test("writeMpq rejects case-insensitive duplicates and unaligned prefixes", async () => {
  const data = new Uint8Array([1]);
  await assertRejects(
    () => writeMpq([{ name: "A.txt", data }, { name: "a.TXT", data }]),
    MoonwellError,
    "Duplicate archive path",
  );
  await assertRejects(() => writeMpq([], { prefix: new Uint8Array(100) }), MoonwellError, "512");
});
