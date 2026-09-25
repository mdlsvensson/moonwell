import { assertEquals, assertThrows } from "@std/assert";
import { readMdlPaths } from "../../src/models/mdl.ts";
import { modelPaths } from "../../src/models/paths.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { chunk, mdx, texture } from "../support/mdx.ts";

const KNIGHT = String.raw`// Exported by a modelling tool
Version {
	FormatVersion 800,
}
Model "Knight" {
	NumGeosets 1,
}
Textures 3 {
	Bitmap {
		Image "Textures\Knight.blp",
	}
	Bitmap {
		Image "",
		ReplaceableId 1,
	}
	Bitmap {
		Image "",
		ReplaceableId 2,
	}
}
ParticleEmitter "Heal" {
	ObjectId 3,
	EmitterUsesMDL,
	static EmissionRate 1,
	Visibility 2 {
		DontInterp,
		0: 1,
		100: 0,
	}
	Translation 1 {
		Linear,
		0: { 1, 2, 3 },
	}
	Particle {
		static LifeSpan 1,
		static InitVelocity 0,
		Path "Abilities\Spells\Human\Heal.mdx",
	}
}
ParticleEmitter "Spark" {
	EmitterUsesTGA,
	Path "Textures\Spark.blp",
}
ParticleEmitter "Empty" {
	Path "",
}
Attachment "Hand" {
	AttachmentID 0,
	Path "Models\Sword.mdx",
}
ParticleEmitterPopcorn "Fire" {
	Path "Effects\Fire.pkfx",
}
FaceFX "Head" {
	Path "FaceFX\Knight.facefx",
}
`;

const EXPECTED = [
  { kind: "texture", path: "Textures\\Knight.blp", replaceableId: 0 },
  { kind: "texture", path: null, replaceableId: 1 },
  { kind: "texture", path: null, replaceableId: 2 },
  { kind: "particle model", path: "Abilities\\Spells\\Human\\Heal.mdx", replaceableId: 0 },
  { kind: "particle texture", path: "Textures\\Spark.blp", replaceableId: 0 },
  { kind: "attachment", path: "Models\\Sword.mdx", replaceableId: 0 },
  { kind: "popcorn", path: "Effects\\Fire.pkfx", replaceableId: 0 },
  { kind: "face effect", path: "FaceFX\\Knight.facefx", replaceableId: 0 },
];

Deno.test("readMdlPaths reads every path-bearing block and ignores the rest", () => {
  assertEquals(readMdlPaths(KNIGHT, "knight.mdl"), EXPECTED);
});

Deno.test("readMdlPaths rejects broken text with a MoonwellError naming the file", () => {
  for (const text of ['Bitmap { Image "Textures\\A.blp', "}", 'Textures 1 { Bitmap { Image "a.blp", }']) {
    const error = assertThrows(() => readMdlPaths(text, "assets/Knight.mdl"), MoonwellError, "not a readable model");
    assertEquals(error.file, "assets/Knight.mdl");
  }
});

Deno.test("modelPaths picks the reader from the content", () => {
  assertEquals(modelPaths(new TextEncoder().encode(KNIGHT), "knight.mdl"), EXPECTED);
  assertEquals(modelPaths(mdx(chunk("TEXS", texture("Textures\\A.blp"))), "a.mdx"), [
    { kind: "texture", path: "Textures\\A.blp", replaceableId: 0 },
  ]);
  const blp = new Uint8Array([0x42, 0x4c, 0x50, 0x31, 0, 0, 0, 0]); // a BLP texture, not a model
  assertThrows(() => modelPaths(blp, "icon.blp"), MoonwellError, "neither a binary MDX nor a text MDL");
});
