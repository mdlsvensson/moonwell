import { assertEquals, assertRejects } from "@std/assert";
import { join } from "@std/path";
import { isHeaderlessArchive, readW3iHeader } from "../../src/map/w3i.ts";
import { buildHm3wHeader } from "../../src/mpq/hm3w.ts";
import { packMap } from "../../src/map/pack.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { openMpq } from "../support/mpq-reader.ts";

function w3i(version: number, major = 0, minor = 0): Uint8Array {
  const bytes = new Uint8Array(64);
  const view = new DataView(bytes.buffer);
  view.setInt32(0, version, true);
  view.setUint32(12, major, true);
  view.setUint32(16, minor, true);
  return bytes;
}

async function mapDir(info: Uint8Array): Promise<string> {
  const dir = await Deno.makeTempDir();
  await Deno.writeFile(join(dir, "war3map.w3i"), info);
  await Deno.writeTextFile(join(dir, "war3map.lua"), "function main() end");
  await Deno.mkdir(join(dir, "war3mapImported"));
  await Deno.writeTextFile(join(dir, "war3mapImported", "a.txt"), "asset");
  return dir;
}

Deno.test("readW3iHeader reads version and game version", () => {
  assertEquals(readW3iHeader(w3i(39, 3, 0)), { version: 39, gameVersion: { major: 3, minor: 0 } });
  assertEquals(readW3iHeader(w3i(25)), { version: 25 });
});

Deno.test("isHeaderlessArchive only for v39 maps from 1.31 on", () => {
  assertEquals(isHeaderlessArchive({ version: 39, gameVersion: { major: 1, minor: 31 } }), true);
  assertEquals(isHeaderlessArchive({ version: 39, gameVersion: { major: 1, minor: 30 } }), false);
  assertEquals(isHeaderlessArchive({ version: 31, gameVersion: { major: 1, minor: 31 } }), false);
  assertEquals(isHeaderlessArchive({ version: 25 }), false);
});

Deno.test("buildHm3wHeader writes magic, name, flags and players", () => {
  const header = buildHm3wHeader("Hero", 4, 6);
  assertEquals(header.length, 512);
  assertEquals(new TextDecoder().decode(header.subarray(0, 4)), "HM3W");
  assertEquals(new TextDecoder().decode(header.subarray(8, 12)), "Hero");
  const view = new DataView(header.buffer);
  assertEquals(header[12], 0);
  assertEquals(view.getUint32(13, true), 4);
  assertEquals(view.getUint32(17, true), 6);
});

Deno.test("packMap writes a headerless archive for modern maps with backslash paths", async () => {
  const archive = openMpq(await packMap(await mapDir(w3i(39, 3, 0)), "map"));
  assertEquals(archive.headerOffset, 0);
  assertEquals(new TextDecoder().decode(await archive.read("war3mapImported\\a.txt")), "asset");
  assertEquals(new TextDecoder().decode(await archive.read("war3map.lua")), "function main() end");
});

Deno.test("packMap prefixes HM3W for older maps", async () => {
  const bytes = await packMap(await mapDir(w3i(25)), "Old Map");
  assertEquals(new TextDecoder().decode(bytes.subarray(0, 4)), "HM3W");
  assertEquals(openMpq(bytes).headerOffset, 512);
});

Deno.test("packMap requires war3map.w3i", async () => {
  const dir = await Deno.makeTempDir();
  await assertRejects(() => packMap(dir, "map"), MoonwellError, "war3map.w3i");
});

Deno.test("packMap skips stale archive metadata from the map folder", async () => {
  const dir = await mapDir(w3i(39, 3, 0));
  await Deno.writeTextFile(join(dir, "(attributes)"), "stale");
  await Deno.writeTextFile(join(dir, "(listfile)"), "stale\r\n");
  const archive = openMpq(await packMap(dir, "map"));
  assertEquals(await archive.read("(attributes)"), undefined);
  assertEquals((await archive.listfile()).sort(), ["war3map.lua", "war3map.w3i", "war3mapImported\\a.txt"]);
});
