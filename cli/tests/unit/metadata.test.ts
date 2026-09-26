import { assert, assertEquals } from "@std/assert";
import { join } from "@std/path";
import {
  baseOf,
  CATEGORIES,
  FIELD_CATEGORIES,
  fieldByName,
  fieldByRawcode,
  fieldsFor,
  loadMetadata,
  type Metadata,
  nearestBases,
} from "../../src/objectdata/metadata.ts";
import { REPO } from "../../../tools/gen.ts";

// Invariants of the committed metadata.json (generated from the game's files by `deno task gen:metadata`).
const metadata: Metadata = JSON.parse(await Deno.readTextFile(join(REPO, "cli", "data", "metadata.json")));

/** Values that occur more than once. */
const duplicates = (values: string[]) => values.filter((value, i) => values.indexOf(value) !== i);

Deno.test("loadMetadata returns the committed metadata.json, decompressed once", async () => {
  const loaded = loadMetadata();
  assertEquals(await loaded, metadata);
  assert(loadMetadata() === loaded);
});

Deno.test("metadata.json: header and unique rawcodes per category", () => {
  assertEquals([metadata.format, metadata.game], [1, "3.0.0.24268"]);
  for (const category of FIELD_CATEGORIES) {
    const ids = metadata.fields[category].map((field) => field.id);
    assertEquals(duplicates(ids), [], category);
    assertEquals(ids, [...ids].sort((a, b) => a < b ? -1 : a > b ? 1 : 0), `${category} is sorted by rawcode`);
    // Curse's "Chance to Miss" is the game's one three-letter field id ("Crs" in the SLK); modification files store it
    // padded with a NUL byte, and so does the metadata, so every id is the 4 characters the writer requires.
    const padded = ids.filter((id) => id.includes("\0"));
    assertEquals(padded, category === "abilities" ? ["Crs\0"] : [], category);
    for (const id of ids) assert(/^[A-Za-z0-9]{3}[A-Za-z0-9\0]$/.test(id), `${category} ${id}`);
  }
});

Deno.test("metadata.json: friendly names are valid and unique among the fields an object can have", () => {
  for (const category of FIELD_CATEGORIES) {
    for (const field of metadata.fields[category]) {
      assert(/^[a-z][A-Za-z0-9]*$/.test(field.name), `${category} ${field.id} "${field.name}"`);
      assert(!["id", "base", "source", "properties"].includes(field.name), `${category} ${field.id} is reserved`);
    }
  }
  // Every class (Unit, Hero, Building, Item, Buff, Upgrade) and every standard ability's fields, which include the
  // Ability class's common fields.
  for (const category of CATEGORIES) {
    const bases = category === "abilities"
      ? [
        ...Object.keys(metadata.bases.abilities),
        ...metadata.fields.abilities.flatMap((field) => field.specific),
      ]
      : [Object.keys(metadata.bases[category])[0]];
    for (const base of new Set(bases)) {
      assertEquals(
        duplicates(fieldsFor(metadata, category, base).map((field) => field.name)),
        [],
        `${category} ${base}`,
      );
    }
  }
  const common = metadata.fields.abilities.filter((field) => field.specific.length === 0).map((field) => field.name);
  assertEquals(duplicates(common), []);
});

Deno.test("metadata.json: storage types, data columns and applicability are consistent", () => {
  for (const category of FIELD_CATEGORIES) {
    for (const field of metadata.fields[category]) {
      const where = `${category} ${field.id}`;
      assert(["int", "real", "unreal", "string"].includes(field.storage), where);
      if (field.type === "bool" || field.type === "int") assertEquals(field.storage, "int", where);
      if (field.type === "real" || field.type === "unreal") assertEquals(field.storage, field.type, where);
      if (field.list) assertEquals(field.storage, "string", where);
      assert(Number.isInteger(field.column) && field.column >= 0 && field.column <= 26, where);
      if (category !== "abilities") assertEquals(field.column, 0, where);
      if (category === "units") assert(field.use.some((use) => use !== "item"), where);
      if (category === "items") assert(field.use.includes("item"), where);
      if (category !== "units" && category !== "items") assertEquals(field.use, [], where);
    }
  }
});

Deno.test("metadata.json: skin flags match the names fixture", () => {
  // The names fixture wrote these fields to war3mapSkin.* files.
  assertEquals(fieldByRawcode(metadata, "units", "unam")?.skin, true);
  assertEquals(fieldByRawcode(metadata, "items", "unam")?.skin, true);
  assertEquals(fieldByRawcode(metadata, "abilities", "anam")?.skin, true);
  assertEquals(fieldByRawcode(metadata, "buffs", "fnam")?.skin, true);
  assertEquals(fieldByRawcode(metadata, "upgrades", "gnam")?.skin, true);
  assertEquals(fieldByRawcode(metadata, "units", "uhpm")?.skin, false);
});

Deno.test("metadata.json: base ids are classified and abilities and upgrades carry level counts", () => {
  for (const category of CATEGORIES) {
    for (const [id, base] of Object.entries(metadata.bases[category])) {
      assert(/^[A-Za-z0-9]{4}$/.test(id) && base.name !== "", `${category} ${id}`);
      if (category === "heroes") assert(/^[A-Z]/.test(id), id);
      if (category === "units" || category === "buildings") assert(!/^[A-Z]/.test(id), id);
      const leveled = category === "abilities" || category === "upgrades";
      assertEquals(Number.isInteger(base.levels), leveled, `${category} ${id} levels`);
    }
  }
  assertEquals(metadata.bases.units.hfoo, { name: "Footman" });
  assertEquals(metadata.bases.heroes.Hpal, { name: "Paladin" });
  assertEquals(metadata.bases.buildings.hbla, { name: "Blacksmith" });
  assertEquals(metadata.bases.abilities.AHhb, { name: "Holy Light", levels: 3 });
  assertEquals(metadata.bases.upgrades.Rhme, { name: "Iron Forged Swords", levels: 3 });
});

Deno.test("fieldsFor and fieldByName apply use, specific and notSpecific", () => {
  const ids = (category: typeof CATEGORIES[number], base: string) =>
    fieldsFor(metadata, category, base).map((field) => field.id);
  assert(ids("heroes", "Hpal").includes("upra"));
  assert(!ids("units", "hfoo").includes("upra"));
  assert(!ids("units", "hfoo").includes("iabi"));
  assert(ids("items", "ratf").includes("iabi"));
  assert(!ids("items", "ratf").includes("uhpm"));
  assert(ids("abilities", "AHhb").includes("Hhb1"));
  assert(!ids("abilities", "AHtb").includes("Hhb1"));
  // A common field does not apply to the bases its notSpecific column lists.
  const excluded = metadata.fields.abilities.find((field) => field.notSpecific.length > 0)!;
  assert(!ids("abilities", excluded.notSpecific[0]).includes(excluded.id));

  assertEquals(fieldByName(metadata, "abilities", "AHhb", "amountHealedOrDamaged")?.id, "Hhb1");
  assertEquals(fieldByName(metadata, "abilities", "AHtb", "amountHealedOrDamaged"), undefined);
  assertEquals(fieldByName(metadata, "heroes", "Hpal", "hitPointsMaximumBase")?.id, "uhpm");
  assertEquals(fieldByRawcode(metadata, "heroes", "uhpm")?.name, "hitPointsMaximumBase");
  assertEquals(fieldByRawcode(metadata, "heroes", "anam"), undefined);
});

Deno.test("baseOf finds a standard id in any category and nearestBases suggests close ids", () => {
  assertEquals(baseOf(metadata, "hfoo")?.category, "units");
  assertEquals(baseOf(metadata, "Hpal")?.category, "heroes");
  assertEquals(baseOf(metadata, "AHhb")?.category, "abilities");
  assertEquals(baseOf(metadata, "h000"), undefined);
  assertEquals(nearestBases(metadata, "heroes", "Hpla", 1), [{ id: "Hpal", name: "Paladin" }]);
  assertEquals(nearestBases(metadata, "units", "HFOO", 1), [{ id: "hfoo", name: "Footman" }]);
  assertEquals(nearestBases(metadata, "units", "hfoo", 3).length, 3);
});
