import { assertEquals, assertThrows } from "@std/assert";
import { isMdx, readMdxPaths } from "../../src/models/mdx.ts";
import { describeModelPath } from "../../src/models/model-path.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import {
  attachment,
  chunk,
  concat,
  emitter,
  EMITTER_USES_MDL,
  EMITTER_USES_TGA,
  faceEffect,
  mdx,
  popcorn,
  setU32,
  texture,
  u32,
} from "../support/mdx.ts";

Deno.test("isMdx recognizes the MDLX magic", () => {
  assertEquals(isMdx(mdx()), true);
  assertEquals(isMdx(new TextEncoder().encode("Version {")), false);
  assertEquals(isMdx(new Uint8Array([0x4d, 0x44])), false);
});

Deno.test("readMdxPaths reads textures, including replaceable slots", () => {
  const bytes = mdx(
    chunk("VERS", u32(800)),
    chunk("TEXS", concat(texture("Textures\\Knight.blp"), texture("", 1), texture("", 2), texture("", 11))),
  );
  assertEquals(readMdxPaths(bytes, "knight.mdx"), [
    { kind: "texture", path: "Textures\\Knight.blp", replaceableId: 0 },
    { kind: "texture", path: null, replaceableId: 1 },
    { kind: "texture", path: null, replaceableId: 2 },
    { kind: "texture", path: null, replaceableId: 11 },
  ]);
});

Deno.test("readMdxPaths reads emitters, attachments, popcorn and face effects in file order", () => {
  const bytes = mdx(
    chunk("ZZZZ", new Uint8Array(13)), // an unknown chunk is skipped by its size
    chunk(
      "PREM",
      concat(
        emitter("Abilities\\Spells\\Human\\Heal.mdx", EMITTER_USES_MDL),
        emitter("Textures\\Spark.blp", EMITTER_USES_TGA),
        emitter("Models\\Default.mdx", 0),
        emitter(""),
      ),
    ),
    chunk("ATCH", concat(attachment("Models\\Sword.mdx"), attachment(""))),
    chunk("CORN", popcorn("Effects\\Fire.pkfx")),
    chunk("FAFX", faceEffect("Head", "FaceFX\\Knight.facefx")),
  );
  assertEquals(readMdxPaths(bytes, "knight.mdx"), [
    { kind: "particle model", path: "Abilities\\Spells\\Human\\Heal.mdx", replaceableId: 0 },
    { kind: "particle texture", path: "Textures\\Spark.blp", replaceableId: 0 },
    { kind: "particle model", path: "Models\\Default.mdx", replaceableId: 0 },
    { kind: "attachment", path: "Models\\Sword.mdx", replaceableId: 0 },
    { kind: "popcorn", path: "Effects\\Fire.pkfx", replaceableId: 0 },
    { kind: "face effect", path: "FaceFX\\Knight.facefx", replaceableId: 0 },
  ]);
});

Deno.test("readMdxPaths rejects damaged files with a MoonwellError naming the file", () => {
  const whole = mdx(chunk("TEXS", texture("a.blp")));
  const record = attachment("Models\\Sword.mdx");
  const damaged = [
    concat(mdx(), new Uint8Array([1, 2, 3])), // a chunk header cut off
    whole.subarray(0, 100), // a chunk running past the end of the file
    mdx(chunk("TEXS", new Uint8Array(100))), // not a whole number of textures
    mdx(chunk("FAFX", new Uint8Array(100))), // not a whole number of face effects
    mdx(chunk("ATCH", setU32(record, 0, 9999))), // a record running past its chunk
    mdx(chunk("ATCH", setU32(record, 4, 10))), // a node smaller than its fixed fields
    mdx(chunk("ATCH", setU32(record, 0, 4 + 96))), // a record with no room for its path
  ];
  for (const bytes of damaged) {
    const error = assertThrows(() => readMdxPaths(bytes, "assets/Knight.mdx"), MoonwellError, "not a readable model");
    assertEquals(error.file, "assets/Knight.mdx");
  }
});

Deno.test("describeModelPath labels replaceable textures", () => {
  assertEquals(describeModelPath({ kind: "texture", path: "Textures\\A.blp", replaceableId: 0 }), "Textures\\A.blp");
  assertEquals(describeModelPath({ kind: "texture", path: null, replaceableId: 1 }), "team colour (slot 1)");
  assertEquals(describeModelPath({ kind: "texture", path: null, replaceableId: 2 }), "team glow (slot 2)");
  assertEquals(describeModelPath({ kind: "texture", path: null, replaceableId: 11 }), "replaceable texture (slot 11)");
});
