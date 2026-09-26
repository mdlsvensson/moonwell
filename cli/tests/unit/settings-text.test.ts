import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
import { validateMapSettings } from "../../src/settings/options.ts";
import { gameplaySections, patchSettingsText } from "../../src/settings/text.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

Deno.test("text patches preserve unrelated content and update duplicate keys", () => {
  for (const nl of ["\n", "\r\n"]) {
    const original = [
      "// keep",
      "[Misc]",
      "MaxHeroLevel=10",
      "Keep=42",
      "[Other]",
      "X=y",
      "[misc]",
      "maxherolevel=12",
      "",
    ].join(nl);
    const sections = { Misc: { MaxHeroLevel: "25", Added: "0" }, CustomSkin: { Text: "" } };
    const patched = patchSettingsText(original, sections);
    for (const line of ["// keep", "Keep=42", "X=y", "MaxHeroLevel=25", "maxherolevel=25", "Added=0", "Text="]) {
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
    "[Misc]\nFoodCeiling=0\n[Skin]\nText=\n\n[New]\nValue=1\n",
  );
});

Deno.test("text patches replace the entire existing value line", () => {
  assertEquals(
    patchSettingsText("[Misc]\nFoodCeiling=100 ; stale note\n", { Misc: { FoodCeiling: "200" } }),
    "[Misc]\nFoodCeiling=200\n",
  );
});

Deno.test("text patches keep the INI layout: no stray blank lines, final newline kept", () => {
  for (const nl of ["\n", "\r\n"]) {
    const lines = (...parts: string[]) => parts.join(nl);
    const cases: [string, Record<string, Record<string, string>>, string][] = [
      // A new key follows the section's last entry, and the final newline survives.
      [lines("[Misc]", "A=1", ""), { Misc: { B: "2" } }, lines("[Misc]", "A=1", "B=2", "")],
      // A blank line separating sections stays between them, not before the new key.
      [lines("[A]", "X=1", "", "[B]", "Y=2", ""), { A: { K: "v" } }, lines("[A]", "X=1", "K=v", "", "[B]", "Y=2", "")],
      // A new section gets one blank separator line and keeps the final newline.
      [lines("[A]", "X=1", ""), { New: { K: "v" } }, lines("[A]", "X=1", "", "[New]", "K=v", "")],
      // No second separator when the source already ends with a blank line.
      [lines("[A]", "X=1", "", ""), { New: { K: "v" } }, lines("[A]", "X=1", "", "[New]", "K=v", "")],
      // Without a final newline in the source, none is added.
      [lines("[A]", "X=1"), { New: { K: "v" } }, lines("[A]", "X=1", "", "[New]", "K=v")],
    ];
    for (const [source, sections, expected] of cases) {
      const patched = patchSettingsText(source, sections);
      assertEquals(patched, expected);
      assertEquals(patchSettingsText(patched, sections), patched);
    }
  }
});

Deno.test("section headers followed by ; or // comments are recognised", () => {
  for (const header of ["[Misc] ; comment", "[Misc]; comment", "[Misc] // comment", "  [Misc]\t;"]) {
    assertEquals(
      patchSettingsText(`${header}\nA=1\n`, { Misc: { B: "2" } }),
      `${header}\nA=1\nB=2\n`,
    );
  }
});
