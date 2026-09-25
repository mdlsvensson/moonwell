import { assertEquals } from "@std/assert";
import { gunzip, gzip } from "../../src/shared/compression.ts";
import { gamePathKey, normalizeGamePath, parseGamePaths, renderGamePaths } from "../../src/models/game-paths.ts";

Deno.test("normalizeGamePath strips storage prefixes and keeps only model-referenced file types", () => {
  assertEquals(normalizeGamePath("war3.w3mod:Units\\Human\\Footman\\Footman.mdx"), "units/human/footman/footman.mdx");
  assertEquals(
    normalizeGamePath("war3.w3mod:_hd.w3mod:Doodads/LordaeronSummer/Plants/Corn/plant1_Normal.dds"),
    "doodads/lordaeronsummer/plants/corn/plant1_normal.dds",
  );
  assertEquals(normalizeGamePath("war3.w3mod:_locales/enus.w3mod:Textures/Black32.blp"), "textures/black32.blp");
  assertEquals(normalizeGamePath("_hd.w3mod/_locales/dede.w3mod/Textures/Black32.blp"), "textures/black32.blp");
  assertEquals(
    normalizeGamePath("war3.mpq:Abilities\\Spells\\Human\\Heal\\Heal.mdl"),
    "abilities/spells/human/heal/heal.mdl",
  );
  assertEquals(normalizeGamePath("  Effects/Fire.pkfx  "), "effects/fire.pkfx");
  assertEquals(normalizeGamePath("war3.w3mod:Sound/Music/mp3Music/ArthasTheme.mp3"), undefined);
  assertEquals(normalizeGamePath("war3.w3mod:Units/UnitData.slk"), undefined);
  assertEquals(normalizeGamePath(""), undefined);
  assertEquals(normalizeGamePath("   "), undefined);
});

Deno.test("renderGamePaths writes a header and sorted, unique paths", () => {
  const list = [
    "war3.w3mod:Textures/Black32.blp",
    "war3.w3mod:_hd.w3mod:Textures/Black32.blp",
    "war3.w3mod:Units/Human/Footman/Footman.mdx",
    "war3.w3mod:Sound/Hit.wav",
    "war3.w3mod:Abilities/Spells/Human/Heal/Heal.mdx",
  ].join("\r\n");
  assertEquals(
    renderGamePaths(list, "3.0.0.24268"),
    [
      "# Warcraft III 3.0.0.24268",
      "abilities/spells/human/heal/heal.mdx",
      "textures/black32.blp",
      "units/human/footman/footman.mdx",
      "",
    ].join("\n"),
  );
});

Deno.test("gamePathKey ignores texture extensions, letter case and separators, and reads .mdl as .mdx", () => {
  assertEquals(
    gamePathKey("Doodads\\LordaeronSummer\\Plants\\Corn\\plant1_Normal.tif"),
    gamePathKey("doodads/lordaeronsummer/plants/corn/plant1_normal.dds"),
  );
  assertEquals(gamePathKey("Textures\\Black32.BLP"), gamePathKey("textures/black32.dds"));
  assertEquals(gamePathKey("Models\\Glow.mdl"), gamePathKey("models/glow.mdx"));
  assertEquals(gamePathKey("Models\\Glow.mdx") === gamePathKey("models/glow.blp"), false);
  assertEquals(gamePathKey("Textures\\Other\\Black32.blp") === gamePathKey("textures/black32.blp"), false);
});

Deno.test("parseGamePaths skips comments and blank lines", () => {
  const keys = parseGamePaths("# Warcraft III 3.0\n\ntextures/black32.blp\nunits/human/footman/footman.mdx\n");
  assertEquals(keys.size, 2);
  assertEquals(keys.has(gamePathKey("Textures\\Black32.dds")), true);
  assertEquals(keys.has(gamePathKey("Units\\Human\\Footman\\Footman.mdl")), true);
  assertEquals(parseGamePaths("# not generated yet\n").size, 0);
});

Deno.test("gzip and gunzip round trip", async () => {
  const text = new TextEncoder().encode("textures/black32.blp\n".repeat(100));
  assertEquals(await gunzip(await gzip(text)), text);
});
