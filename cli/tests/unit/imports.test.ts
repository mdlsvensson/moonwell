import { assertEquals, assertThrows } from "@std/assert";
import { importPath, readImports, writeImports } from "../../src/assets/imports.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

Deno.test("war3map.imp round trips default and custom-path entries", () => {
  const entries = [{ flag: 5, path: "default.blp" }, { flag: 13, path: "Textures\\custom.blp" }];
  assertEquals(readImports(writeImports(entries)), entries);
  assertEquals(importPath(entries[0]), "war3mapImported\\default.blp");
  assertEquals(importPath(entries[1]), "Textures\\custom.blp");
  assertEquals(readImports(writeImports([])), []);
});

Deno.test("readImports rejects corrupt data with a MoonwellError naming the file", () => {
  const valid = writeImports([{ flag: 13, path: "a.blp" }]);
  const wrongVersion = valid.slice();
  wrongVersion[0] = 2;
  const badFlag = valid.slice();
  badFlag[8] = 7;
  for (const bytes of [new Uint8Array([1, 0]), valid.subarray(0, 10), wrongVersion, badFlag]) {
    const error = assertThrows(() => readImports(bytes, "maps/map.w3x/war3map.imp"), MoonwellError);
    assertEquals(error.file, "maps/map.w3x/war3map.imp");
  }
  const trailing = new Uint8Array([...valid, 0]);
  assertThrows(() => readImports(trailing), MoonwellError, "trailing");
  const empty = writeImports([{ flag: 13, path: "x" }]);
  empty[9] = 0; // the path's only character becomes the terminator
  assertThrows(() => readImports(empty.subarray(0, 10)), MoonwellError, "empty path");
});
