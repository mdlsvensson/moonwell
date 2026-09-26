import { assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { dirname, join } from "@std/path";
import { generateMetadata, type Overrides } from "../../../tools/gen-metadata.ts";
import type { FieldMeta, Metadata } from "../../src/objectdata/metadata.ts";

// A hand-written miniature of the game's files, in the export's folder layout. Ids and labels are real where the
// test is about them (Holy Light, Footman); the values are made up.

/** SYLK text with a header row and one row per record; `undefined` cells are left out. */
function slk(columns: string[], rows: (string | number | undefined)[][]): string {
  const lines = ["ID;PWXL;N;E"];
  [columns, ...rows].forEach((row, y) => {
    let first = true;
    row.forEach((cell, x) => {
      if (cell === undefined) return;
      const value = typeof cell === "number" ? String(cell) : `"${cell}"`;
      lines.push(`C;X${x + 1};${first ? `Y${y + 1};` : ""}K${value}`);
      first = false;
    });
  });
  return [...lines, "E", ""].join("\r\n");
}

const UNIT_META = [
  "ID",
  "field",
  "slk",
  "index",
  "category",
  "displayName",
  "type",
  "useHero",
  "useUnit",
  "useBuilding",
  "useItem",
  "useSpecific",
  "netsafe",
];
const ABILITY_META = [
  "ID",
  "field",
  "slk",
  "index",
  "repeat",
  "data",
  "category",
  "displayName",
  "type",
  "useUnit",
  "useHero",
  "useItem",
  "useSpecific",
  "notSpecific",
  "netsafe",
];
const BUFF_META = ["ID", "field", "category", "displayName", "type", "netsafe"];
const UPGRADE_META = ["ID", "field", "repeat", "effectType", "category", "displayName", "type", "netsafe"];

const GAME_FILES: Record<string, string> = {
  "units/unitmetadata.slk": slk(UNIT_META, [
    ["uhpm", "HP", "UnitBalance", -1, "stats", "WESTRING_UHPM", "int", 1, 1, 1, 0, undefined, 0],
    ["unam", "Name", "Profile", 0, "text", "WESTRING_UNAM", "string", 1, 1, 1, 1, undefined, 1],
    ["umdl", "file", "Profile", 0, "art", "WESTRING_UMDL", "model", 1, 1, 0, 0, undefined, 1],
    ["ifil", "file", "ItemData", 0, "art", "WESTRING_IFIL", "model", 0, 0, 0, 1, undefined, 1],
    ["ushr", "shadowOnWater", "Profile", -1, "art", "WESTRING_USHR", "bool", 0, 0, 1, 0, undefined, 11],
    ["uabi", "abilList", "UnitAbilities", -1, "abil", "WESTRING_UABI", "abilityList", 1, 1, 1, 0, undefined, 0],
    ["udea", "deathType", "UnitData", -1, "stats", "WESTRING_UDEA", "deathType", 1, 1, 1, 0, undefined, 0],
    ["upro", "Propernames", "Profile", -1, "text", "WESTRING_UPRO", "stringList", 1, 0, 0, 0, undefined, 1],
    ["ucls", "class", "Profile", -1, "stats", "WESTRING_UCLS", "string", 1, 1, 1, 0, undefined, 0],
    ["uver", "fileVerFlags", "Profile", -1, "art", "WESTRING_UVER", "versionFlags", 1, 1, 1, 0, undefined, 1],
    [undefined, "orphan", "Profile", -1, "stats", "WESTRING_UHPM", "int", 1, 1, 1, 0, undefined, 0],
  ]),
  "units/abilitymetadata.slk": slk(ABILITY_META, [
    ["anam", "Name", "Profile", 0, 0, 0, "text", "WESTRING_ANAM", "string", 1, 1, 1, undefined, undefined, 1],
    ["alev", "levels", "AbilityData", -1, 0, 0, "stats", "WESTRING_ALEV", "int", 1, 1, 1, undefined, undefined, 0],
    ["acdn", "Cool", "AbilityData", -1, 4, 0, "stats", "WESTRING_ACDN", "unreal", 1, 1, 1, undefined, undefined, 0],
    ["aare", "Area", "AbilityData", -1, 4, 0, "stats", "WESTRING_AARE", "unreal", 1, 1, 1, undefined, "AHhb", 0],
    ["Hhb2", "Data", "AbilityData", -1, 4, 2, "data", "WESTRING_HHB2", "unreal", 1, 1, 1, "AHhb", undefined, 0],
    ["Hhb1", "Data", "AbilityData", -1, 4, 1, "data", "WESTRING_HHB1", "unreal", 1, 1, 1, "AHhb", undefined, ""],
    ["Htb1", "Data", "AbilityData", -1, 4, 1, "data", "WESTRING_HTB1", "unreal", 1, 1, 1, "AHtb", undefined, 0],
    ["Hdc1", "Data", "AbilityData", -1, 4, 12, "data", "WESTRING_HDC1", "int", 1, 1, 1, "AHtb,AHhb", undefined, 0],
    ["atp1", "Tip", "Profile", 0, 3, 0, "text", "WESTRING_ATP1", "string", 1, 1, 0, undefined, undefined, 1],
  ]),
  "units/abilitybuffmetadata.slk": slk(BUFF_META, [
    ["fnam", "EditorName", "text", "WESTRING_FNAM", "string", 1],
    ["fart", "Buffart", "art", "WESTRING_FART", "icon", 1],
  ]),
  "units/upgrademetadata.slk": slk(UPGRADE_META, [
    ["gnam", "Name", 1, undefined, "text", "WESTRING_GNAM", "string", 1],
    ["gef1", "effect1", 0, "EffectID", "data", "WESTRING_GEF1", "upgradeEffect", 0],
    ["gba1", "base1", 0, "Base", "data", "WESTRING_GBA1", "unreal", 0],
    ["gmo1", "mod1", 0, "Mod", "data", "WESTRING_GMO1", "unreal", 0],
    ["gpct", "pct", 0, undefined, "data", "WESTRING_GPCT", "unreal", 0],
  ]),
  "units/unitdata.slk": slk(["unitID", "comment(s)"], [["hfoo", "footman"], ["Hpal", "paladin"], [
    "hbar",
    "barracks",
  ], ["nzzz", "unnamed critter"]]),
  "units/unitbalance.slk": slk(["unitBalanceID", "isbldg", "Primary"], [
    ["hfoo", 0, "_"],
    ["Hpal", 0, "STR"],
    ["hbar", 1, "_"],
    ["nzzz", 0, "_"],
  ]),
  "units/itemdata.slk": slk(["itemID", "comment"], [["ratf", "claws"]]),
  "units/abilitydata.slk": slk(["alias", "comments", "levels"], [
    ["AHhb", "holy light", 3],
    ["AHtb", "storm bolt", 3],
    [undefined, "row without an id", 1],
  ]),
  "units/abilitybuffdata.slk": slk(["alias", "comments"], [["Binf", "inner fire"], ["BHbd", "blizzard"]]),
  "units/upgradedata.slk": slk(["upgradeid", "comments", "maxlevel"], [["Rhme", "swords", 3]]),
  "_locales/enus.w3mod/ui/worldeditstrings.txt": [
    "[WorldEditStrings]",
    "WESTRING_UHPM=Hit Points Maximum (Base)",
    "WESTRING_UNAM=Name",
    "WESTRING_UMDL=WESTRING_MODELFILE",
    "WESTRING_MODELFILE=Model File",
    "WESTRING_IFIL=Model File",
    "WESTRING_USHR=Shadow on Water",
    "WESTRING_UABI=Abilities - Normal",
    "WESTRING_UDEA=Death Type",
    "WESTRING_UPRO=Proper Names (Hero's +1.)",
    "WESTRING_UCLS=Class",
    "WESTRING_UVER=Model File - Extra Versions",
    "WESTRING_ANAM=Name",
    "WESTRING_ALEV=Levels",
    "WESTRING_ACDN=Cooldown",
    "WESTRING_AARE=Area of Effect",
    "WESTRING_HHB2=Area of Effect",
    "WESTRING_HHB1=Amount Healed/Damaged",
    "WESTRING_HTB1=Cooldown",
    "WESTRING_HDC1=Damage Dealt (%)",
    "WESTRING_ATP1=Tooltip - Normal",
    "WESTRING_FNAM=Name",
    "WESTRING_FART=Icon",
    "WESTRING_GNAM=Name",
    "WESTRING_GEF1=Effect 1",
    "WESTRING_GBA1=Effect 1 - %s",
    "WESTRING_GMO1=Effect 1 - %s",
    "WESTRING_GPCT=% Bonus & More",
    "",
  ].join("\r\n"),
  "_locales/enus.w3mod/units/humanunitstrings.txt": [
    "[hfoo]\t",
    "Name=Footman",
    "[Hpal]",
    'Name="|cffffcc00Paladin|r"',
    "[hbar]",
    "Name=Barracks",
  ].join("\r\n"),
  "_locales/enus.w3mod/units/humanabilitystrings.txt": [
    "[AHhb]",
    "Name=Holy Light",
    "[AHtb]",
    "Name=Storm Bolt",
    "[Binf]",
    "Bufftip=Inner Fire",
    "[BHbd]",
    "EditorName=Blizzard (Caster)",
    "Bufftip=Blizzard",
  ].join("\n"),
  "_locales/enus.w3mod/units/humanupgradestrings.txt": "[Rhme]\nName=Iron Forged Swords,Steel Forged Swords\n",
  "_locales/enus.w3mod/units/itemstrings.txt": "[ratf]\nName=Claws of Attack +15\n",
};

async function withGame(
  run: (folder: string, target: string) => Promise<void>,
  change: (files: Record<string, string>) => void = () => {},
): Promise<void> {
  const dir = await Deno.makeTempDir();
  try {
    const files = { ...GAME_FILES };
    change(files);
    for (const [path, text] of Object.entries(files)) {
      const file = join(dir, "game", "war3.w3mod", ...path.split("/"));
      await Deno.mkdir(dirname(file), { recursive: true });
      await Deno.writeTextFile(file, text);
    }
    await run(join(dir, "game"), join(dir, "metadata.json"));
  } finally {
    await Deno.remove(dir, { recursive: true });
  }
}

const OVERRIDES: Overrides = { names: { units: { ucls: "unitClass" } } };

async function generated(target: string): Promise<Metadata> {
  return JSON.parse(await Deno.readTextFile(target));
}

const byId = (fields: FieldMeta[]) => Object.fromEntries(fields.map((field) => [field.id, field]));

Deno.test("generateMetadata writes field records from the metadata SLKs", async () => {
  await withGame(async (folder, target) => {
    await generateMetadata(folder, "3.0.0.1", target, OVERRIDES);
    const metadata = await generated(target);
    assertEquals(metadata.format, 1);
    assertEquals(metadata.game, "3.0.0.1");
    const units = byId(metadata.fields.units);
    assertEquals(units.uhpm, {
      id: "uhpm",
      name: "hitPointsMaximumBase",
      label: "Hit Points Maximum (Base)",
      category: "stats",
      type: "int",
      storage: "int",
      list: false,
      perLevel: false,
      column: 0,
      skin: false,
      use: ["unit", "hero", "building"],
      specific: [],
      notSpecific: [],
    });
    // Rows without an id are skipped; fields only items use are not unit fields.
    assertEquals(Object.keys(units), ["uabi", "ucls", "udea", "uhpm", "umdl", "unam", "upro", "ushr", "uver"]);
    // Labels resolve through nested WESTRING references; only netsafe 1 marks a skin field (11 does not).
    assertEquals([units.umdl.label, units.umdl.skin, units.ushr.skin, units.unam.skin], [
      "Model File",
      true,
      false,
      true,
    ]);
    // bool, flag and enumeration types are stored as int; lists and other types as strings.
    assertEquals(
      [units.ushr.storage, units.udea.storage, units.uver.storage, units.uabi.storage, units.umdl.storage],
      ["int", "int", "int", "string", "string"],
    );
    assertEquals([units.uabi.list, units.upro.list, units.unam.list], [true, true, false]);

    const items = byId(metadata.fields.items);
    assertEquals(Object.keys(items), ["ifil", "unam"]);
    assertEquals(items.unam, units.unam);
    assertEquals(items.unam.use, ["unit", "hero", "building", "item"]);

    const abilities = byId(metadata.fields.abilities);
    assertEquals([abilities.anam.perLevel, abilities.atp1.perLevel, abilities.acdn.perLevel], [false, true, true]);
    assertEquals([abilities.Hhb1.column, abilities.Hdc1.column, abilities.acdn.column], [1, 12, 0]);
    assertEquals([abilities.Hdc1.specific, abilities.aare.notSpecific], [["AHtb", "AHhb"], ["AHhb"]]);
    assertEquals([abilities.Hhb1.skin, abilities.Hhb1.use], [false, []]);

    const upgrades = byId(metadata.fields.upgrades);
    assertEquals(upgrades.gnam.perLevel, true);
    // "%s" stands for the effect's own label in World Editor; the effect type names the field instead.
    assertEquals([upgrades.gba1.label, upgrades.gmo1.label], ["Effect 1 - Base", "Effect 1 - Mod"]);
    assertEquals(upgrades.gef1.storage, "string");
  });
});

Deno.test("generateMetadata derives friendly names from labels and renames clashes", async () => {
  await withGame(async (folder, target) => {
    const result = await generateMetadata(folder, "3.0.0.1", target, OVERRIDES);
    const metadata = await generated(target);
    const names = (fields: FieldMeta[]) => Object.fromEntries(fields.map((field) => [field.id, field.name]));
    assertEquals(names(metadata.fields.units), {
      uabi: "abilitiesNormal",
      ucls: "unitClass",
      udea: "deathType",
      uhpm: "hitPointsMaximumBase",
      umdl: "modelFile",
      unam: "name",
      upro: "properNamesHerosPlus1",
      ushr: "shadowOnWater",
      uver: "modelFileExtraVersions",
    });
    // "Model File" appears once per class: umdl is not an item field, ifil is not a unit field.
    assertEquals(names(metadata.fields.items), { ifil: "modelFile", unam: "name" });
    // Storm Bolt's own "Cooldown" clashes with the common one, so both take their category. Holy Light's own "Area
    // of Effect" does not clash with the common one, which does not apply to Holy Light (notSpecific).
    assertEquals(names(metadata.fields.abilities), {
      Hdc1: "damageDealtPercent",
      Hhb1: "amountHealedOrDamaged",
      Hhb2: "areaOfEffect",
      aare: "areaOfEffect",
      Htb1: "dataCooldown",
      acdn: "statsCooldown",
      alev: "levels",
      anam: "name",
      atp1: "tooltipNormal",
    });
    assertEquals(names(metadata.fields.upgrades), {
      gba1: "effect1Base",
      gef1: "effect1",
      gmo1: "effect1Mod",
      gnam: "name",
      gpct: "percentBonusAndMore",
    });
    assertEquals(result.renames, [
      'units ucls "class" -> "unitClass" (override)',
      'abilities Htb1 "cooldown" -> "dataCooldown" (category prefix)',
      'abilities acdn "cooldown" -> "statsCooldown" (category prefix)',
    ]);
  });
});

Deno.test("generateMetadata appends the rawcode when the category prefix leaves a clash", async () => {
  await withGame(async (folder, target) => {
    const result = await generateMetadata(folder, "3.0.0.1", target, OVERRIDES);
    const names = Object.fromEntries((await generated(target)).fields.abilities.map((field) => [field.id, field.name]));
    assertEquals([names.Hhb1, names.Hdc1], ["dataDamageHhb1", "dataDamageHdc1"]);
    assertStringIncludes(
      result.renames.join("\n"),
      'abilities Hhb1 "damage" -> "dataDamageHhb1" (category prefix and rawcode)',
    );
  }, (files) => {
    files["_locales/enus.w3mod/ui/worldeditstrings.txt"] += "WESTRING_HHB1=Damage\r\nWESTRING_HDC1=Damage\r\n";
  });
});

Deno.test("generateMetadata classifies bases and reads names and level counts", async () => {
  await withGame(async (folder, target) => {
    await generateMetadata(folder, "3.0.0.1", target, OVERRIDES);
    assertEquals((await generated(target)).bases, {
      heroes: { Hpal: { name: "Paladin" } },
      units: { hfoo: { name: "Footman" }, nzzz: { name: "unnamed critter" } },
      buildings: { hbar: { name: "Barracks" } },
      items: { ratf: { name: "Claws of Attack +15" } },
      abilities: { AHhb: { name: "Holy Light", levels: 3 }, AHtb: { name: "Storm Bolt", levels: 3 } },
      buffs: { BHbd: { name: "Blizzard (Caster)" }, Binf: { name: "Inner Fire" } },
      upgrades: { Rhme: { name: "Iron Forged Swords", levels: 3 } },
    });
  });
});

Deno.test("generateMetadata is deterministic and reports counts per category", async () => {
  await withGame(async (folder, target) => {
    const first = await generateMetadata(folder, "3.0.0.1", target, OVERRIDES);
    const text = await Deno.readTextFile(target);
    const second = await generateMetadata(folder, "3.0.0.1", target, OVERRIDES);
    assertEquals(await Deno.readTextFile(target), text);
    assertEquals(first.counts, second.counts);
    assertEquals(first.counts, {
      fields: { units: 9, items: 2, abilities: 9, buffs: 2, upgrades: 5 },
      bases: { heroes: 1, units: 2, buildings: 1, items: 1, abilities: 2, buffs: 2, upgrades: 1 },
    });
    // One record per line keeps the file small and its diffs readable.
    assertStringIncludes(text, '\n      {"id":"uhpm","name":"hitPointsMaximumBase",');
    assertStringIncludes(text, '\n      "AHhb": {"name":"Holy Light","levels":3},');
  });
});

Deno.test("generateMetadata fails, without writing, on a Pkl keyword or reserved name without an override", async () => {
  await withGame(async (folder, target) => {
    const error = await assertRejects(() => generateMetadata(folder, "3.0.0.1", target, {}), Error);
    assertStringIncludes(error.message, 'units ucls "class" (Class)');
    assertStringIncludes(error.message, "overrides.json");
    await assertRejects(() => Deno.stat(target), Deno.errors.NotFound);
  });
  await withGame(async (folder, target) => {
    const error = await assertRejects(() => generateMetadata(folder, "3.0.0.1", target, OVERRIDES), Error);
    assertStringIncludes(error.message, 'buffs fart "base" (Base)');
  }, (files) => {
    files["_locales/enus.w3mod/ui/worldeditstrings.txt"] += "WESTRING_FART=Base\r\n";
  });
});

Deno.test("generateMetadata fails when a released friendly name would change or disappear", async () => {
  await withGame(async (folder, target) => {
    await generateMetadata(folder, "3.0.0.1", target, OVERRIDES);
    const released = await Deno.readTextFile(target);
    const renamed = released.replace('"name":"hitPointsMaximumBase"', '"name":"hitPoints"')
      .replace('{"id":"gpct"', '{"id":"gold"');
    await Deno.writeTextFile(target, renamed);

    const error = await assertRejects(() => generateMetadata(folder, "3.0.0.2", target, OVERRIDES), Error);
    assertStringIncludes(error.message, 'units uhpm "hitPoints" would become "hitPointsMaximumBase"');
    assertStringIncludes(error.message, 'upgrades gold "percentBonusAndMore" would disappear');
    assertEquals(await Deno.readTextFile(target), renamed);

    // An override pins the released name; `removed` acknowledges a field the game no longer has.
    await generateMetadata(folder, "3.0.0.2", target, {
      names: { units: { ucls: "unitClass", uhpm: "hitPoints" } },
      removed: { upgrades: ["gold"] },
    });
    assertEquals(byId((await generated(target)).fields.units).uhpm.name, "hitPoints");
  });
});

Deno.test("generateMetadata fails when an uppercase unit id is not a hero in the balance data", async () => {
  await withGame(async (folder, target) => {
    const error = await assertRejects(() => generateMetadata(folder, "3.0.0.1", target, OVERRIDES), Error);
    assertStringIncludes(error.message, "Hpal");
    assertStringIncludes(error.message, "nhro");
  }, (files) => {
    files["units/unitbalance.slk"] = slk(["unitBalanceID", "isbldg", "Primary"], [
      ["hfoo", 0, "_"],
      ["Hpal", 0, "_"],
      ["hbar", 1, "_"],
      ["nzzz", 0, "_"],
      ["nhro", 0, "AGI"],
    ]);
    files["units/unitdata.slk"] = slk(["unitID"], [["hfoo"], ["Hpal"], ["hbar"], ["nzzz"], ["nhro"]]);
  });
});

Deno.test("generateMetadata names a missing game file", async () => {
  await withGame(async (folder, target) => {
    const error = await assertRejects(() => generateMetadata(folder, "3.0.0.1", target, OVERRIDES), Error);
    assertStringIncludes(error.message, "war3.w3mod/units/upgradedata.slk");
  }, (files) => {
    delete files["units/upgradedata.slk"];
  });
});

Deno.test("generateMetadata pads a three-letter field id with NUL and reads '.' as a list separator", async () => {
  await withGame(async (folder, target) => {
    await generateMetadata(folder, "3.0.0.1", target, OVERRIDES);
    const field = (await generated(target)).fields.abilities.find((field) => field.label === "Chance to Miss");
    assertEquals([field?.id, field?.specific], ["Crs\0", ["AHtb", "AHhb"]]);
  }, (files) => {
    files["units/abilitymetadata.slk"] = files["units/abilitymetadata.slk"].replace(
      "\r\nE\r\n",
      '\r\nC;X1;Y11;K"Crs"\r\nC;X2;K"Data"\r\nC;X5;K4\r\nC;X6;K1\r\nC;X7;K"data"\r\nC;X8;K"WESTRING_CRS"\r\n' +
        'C;X9;K"unreal"\r\nC;X13;K"AHtb.AHhb"\r\nE\r\n',
    );
    files["_locales/enus.w3mod/ui/worldeditstrings.txt"] += "WESTRING_CRS=Chance to Miss\r\n";
  });
});
