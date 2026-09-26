import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
import { validateMapSettings } from "../../src/settings/options.ts";
import { gameplaySections, patchSettingsText } from "../../src/settings/text.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

Deno.test("text patches preserve unrelated content and update duplicate keys", () => {
  for (const nl of ["\n", "\r\n"]) {
    const original = [
      "// keep",
      "[Misc]",
      "HeroMaxLevel=10",
      "Keep=42",
      "[Other]",
      "X=y",
      "[misc]",
      "heromaxlevel=12",
      "",
    ].join(nl);
    const sections = { Misc: { HeroMaxLevel: "25", Added: "0" }, CustomSkin: { Text: "" } };
    const patched = patchSettingsText(original, sections);
    for (const line of ["// keep", "Keep=42", "X=y", "HeroMaxLevel=25", "heromaxlevel=25", "Added=0", "Text="]) {
      assertStringIncludes(patched, line);
    }
    assertStringIncludes(patched, `[Other]${nl}X=y`);
    assertEquals(patchSettingsText(patched, sections), patched);
    assertEquals(patchSettingsText(original, {}), original);
  }
  assertEquals(patchSettingsText("", { Misc: { FoodCeiling: "0" } }), "[Misc]\nFoodCeiling=0");
});

Deno.test("typed gameplay merges are case insensitive without mutating settings", () => {
  const s = validateMapSettings({ gameplay: { foodLimit: 200 }, gameplayConstants: { misc: { foodceiling: "200" } } });
  const before = JSON.stringify(s);
  assertEquals(gameplaySections(s), { misc: { foodceiling: "200" } });
  assertEquals(JSON.stringify(s), before);
  s.gameplayConstants.misc.foodceiling = "0200";
  assertThrows(() => gameplaySections(s), MoonwellError, "FoodCeiling");
});

Deno.test("text patches recognize tab-indented keys and case-only section and key matches", () => {
  const original = "[mIsC] // keep heading comment\r\n\tfoodceiling = 100\r\n";
  const patched = patchSettingsText(original, { Misc: { FoodCeiling: "200" } });
  assertStringIncludes(patched, "[mIsC] // keep heading comment\r\n\tfoodceiling=200\r\n");
});

Deno.test("text patches add keys to empty sections and create missing sections", () => {
  assertEquals(
    patchSettingsText("[Misc]\n[Skin]\n", { Misc: { FoodCeiling: "0" }, Skin: { Text: "" }, New: { Value: "1" } }),
    "[Misc]\nFoodCeiling=0\n[Skin]\n\nText=\n\n[New]\nValue=1",
  );
});

Deno.test("text patches replace the entire existing value line", () => {
  assertEquals(
    patchSettingsText("[Misc]\nFoodCeiling=100 ; stale note\n", { Misc: { FoodCeiling: "200" } }),
    "[Misc]\nFoodCeiling=200\n",
  );
});
