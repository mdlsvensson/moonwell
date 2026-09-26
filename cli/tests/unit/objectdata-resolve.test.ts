import { assertEquals, assertInstanceOf, assertThrows } from "@std/assert";
import { formatError, ObjectDataError } from "../../src/shared/errors.ts";
import type { Category } from "../../src/objectdata/metadata.ts";
import { emptyObjects, type ManifestObject, type ProjectObjects, SCHEMA_HINT } from "../../src/objectdata/manifest.ts";
import { resolveObjects } from "../../src/objectdata/resolve.ts";
import { miniMetadata } from "../support/objectdata.ts";

const metadata = miniMetadata();

type Entry = Partial<ManifestObject> & { id: string; base: string };

function objects(entries: [Category, string, Entry][]): ProjectObjects {
  const result = emptyObjects();
  for (const [category, key, entry] of entries) {
    result[category][key] = { source: "objects/a.pkl", typed: {}, properties: {}, ...entry };
  }
  return result;
}

function resolve(entries: [Category, string, Entry][], existing: string[] = []) {
  return resolveObjects(metadata, objects(entries), new Set(existing));
}

/** The one problem resolving `entries` reports, as `[message, hint]`. */
function problem(entries: [Category, string, Entry][], existing: string[] = []): [string, string | undefined] {
  const error = assertThrows(() => resolve(entries, existing), ObjectDataError);
  assertEquals(error.problems.length, 1, error.problems.map((p) => p.message).join("\n"));
  return [error.problems[0].message, error.problems[0].hint];
}

// Resolution

Deno.test("resolveObjects: an empty manifest resolves to nothing", () => {
  assertEquals(resolveObjects(metadata, emptyObjects(), new Set()), []);
});

Deno.test("resolveObjects resolves typed fields in category order, sorted by rawcode and level", () => {
  const resolved = resolve([
    ["abilities", "holy", {
      id: "A000",
      base: "AHhb",
      source: "objects/abilities.pkl",
      typed: { name: "Holier Light", castRange: [500, 600.5], manaCost: 75, heroAbility: true },
    }],
    ["units", "captain", { id: "h000", base: "hfoo", typed: { hitPointsMaximumBase: 500, scalingValue: 1.25 } }],
  ]);
  assertEquals(resolved, [
    {
      category: "units",
      key: "captain",
      id: "h000",
      base: "hfoo",
      source: "objects/a.pkl",
      fields: [
        {
          id: "uhpm",
          name: "hitPointsMaximumBase",
          level: 0,
          column: 0,
          skin: false,
          value: { type: "int", value: 500 },
        },
        { id: "usca", name: "scalingValue", level: 0, column: 0, skin: true, value: { type: "real", value: 1.25 } },
      ],
    },
    {
      category: "abilities",
      key: "holy",
      id: "A000",
      base: "AHhb",
      source: "objects/abilities.pkl",
      fields: [
        { id: "aher", name: "heroAbility", level: 0, column: 0, skin: false, value: { type: "int", value: 1 } },
        { id: "amcs", name: "manaCost", level: 1, column: 0, skin: false, value: { type: "int", value: 75 } },
        { id: "anam", name: "name", level: 0, column: 0, skin: true, value: { type: "string", value: "Holier Light" } },
        { id: "aran", name: "castRange", level: 1, column: 0, skin: false, value: { type: "unreal", value: 500 } },
        { id: "aran", name: "castRange", level: 2, column: 0, skin: false, value: { type: "unreal", value: 600.5 } },
      ],
    },
  ]);
});

Deno.test("resolveObjects: properties by rawcode and by friendly name, with the data column", () => {
  const [holy] = resolve([["abilities", "holy", {
    id: "A000",
    base: "AHhb",
    properties: { amountHealedOrDamaged: [200, 400], alev: 3 },
  }]]);
  assertEquals(holy.fields, [
    {
      id: "Hhb1",
      name: "amountHealedOrDamaged",
      level: 1,
      column: 1,
      skin: false,
      value: { type: "unreal", value: 200 },
    },
    {
      id: "Hhb1",
      name: "amountHealedOrDamaged",
      level: 2,
      column: 1,
      skin: false,
      value: { type: "unreal", value: 400 },
    },
    { id: "alev", name: "levels", level: 0, column: 0, skin: false, value: { type: "int", value: 3 } },
  ]);
});

Deno.test("resolveObjects: the three-letter rawcode Crs resolves to the padded field", () => {
  const [curse] = resolve([["abilities", "curse", { id: "A000", base: "Acrs", properties: { Crs: 0.25 } }]]);
  assertEquals(curse.fields, [
    { id: "Crs\0", name: "chanceToMiss", level: 1, column: 1, skin: false, value: { type: "unreal", value: 0.25 } },
  ]);
});

Deno.test("resolveObjects writes Booleans as 1 and 0 and joins lists with commas", () => {
  const [peasant] = resolve([["units", "worker", {
    id: "h000",
    base: "hpea",
    properties: { structuresBuilt: ["htow", "hbar"] },
  }]]);
  assertEquals(peasant.fields.map((field) => field.value), [{ type: "string", value: "htow,hbar" }]);
  const [item] = resolve([["items", "orb", { id: "I000", base: "ratf", typed: { perishable: true } }]]);
  assertEquals(item.fields.map((field) => field.value), [{ type: "int", value: 1 }]);
  const [holy] = resolve([["abilities", "holy", {
    id: "A000",
    base: "AHhb",
    typed: { buffs: [["BHbd", "Bcrs"], ["Bcrs"]] },
  }]]);
  assertEquals(holy.fields.map((field) => [field.level, field.value]), [
    [1, { type: "string", value: "BHbd,Bcrs" }],
    [2, { type: "string", value: "Bcrs" }],
  ]);
  const [one] = resolve([["abilities", "one", { id: "A001", base: "AHhb", typed: { buffs: ["BHbd", "Bcrs"] } }]]);
  assertEquals(one.fields.map((field) => [field.level, field.value]), [[1, { type: "string", value: "BHbd,Bcrs" }]]);
});

Deno.test("resolveObjects keeps explicit false, 0, empty strings and empty lists as overrides", () => {
  const [holy] = resolve([["abilities", "holy", {
    id: "A000",
    base: "AHhb",
    typed: { heroAbility: false, manaCost: 0, name: "", buffs: [] },
  }]]);
  assertEquals(holy.fields.map((field) => [field.id, field.level, field.value]), [
    ["abuf", 1, { type: "string", value: "" }],
    ["aher", 0, { type: "int", value: 0 }],
    ["amcs", 1, { type: "int", value: 0 }],
    ["anam", 0, { type: "string", value: "" }],
  ]);
  const [peasant] = resolve([["units", "worker", { id: "h000", base: "hpea", properties: { ubui: [] } }]]);
  assertEquals(peasant.fields.map((field) => field.value), [{ type: "string", value: "" }]);
});

Deno.test("resolveObjects: upgrades are leveled; unit, item and buff fields have level and column 0", () => {
  const [swords] = resolve([["upgrades", "swords", { id: "R000", base: "Rhme", typed: { name: ["I", "II"] } }]]);
  assertEquals(swords.fields.map((field) => [field.level, field.column, field.skin]), [[1, 0, true], [2, 0, true]]);
  const [buff] = resolve([["buffs", "aura", { id: "B000", base: "Bcrs", typed: { tooltip: "t", isAnEffect: true } }]]);
  assertEquals(buff.fields.map((field) => [field.id, field.level, field.column]), [["feff", 0, 0], ["ftip", 0, 0]]);
});

// One test per rule of spec §5.1

Deno.test("rule: an id that is not four ASCII letters or digits", () => {
  assertEquals(problem([["abilities", "holy", { id: "A-00", base: "AHhb" }]]), [
    `abilities["holy"].id: 'A-00' is not four ASCII letters or digits.`,
    "Use an id such as 'A000'.",
  ]);
  assertEquals(
    problem([["heroes", "hero", { id: "h00", base: "Hpal" }]])[0],
    `heroes["hero"].id: 'h00' is not four ASCII letters or digits.`,
  );
});

Deno.test("rule: hero ids start with an uppercase letter and unit or building ids do not", () => {
  assertEquals(problem([["heroes", "paladin", { id: "h000", base: "Hpal" }]]), [
    `heroes["paladin"].id: 'h000' must start with an uppercase letter: the game treats exactly those unit ids as heroes.`,
    "Use an id such as 'H000'.",
  ]);
  assertEquals(problem([["buildings", "hall", { id: "H000", base: "htow" }]]), [
    `buildings["hall"].id: 'H000' must not start with an uppercase letter: the game would treat it as a hero.`,
    "Use an id such as 'h000', or make the object a hero.",
  ]);
});

Deno.test("rule: a duplicate id across categories", () => {
  const error = assertThrows(() =>
    resolveObjects(
      metadata,
      {
        ...objects([["items", "orb", { id: "A000", base: "ratf", source: "objects/items.pkl" }]]),
        abilities: {
          holy: { id: "A000", base: "AHhb", source: "objects/abilities.pkl", typed: {}, properties: {} },
        },
      },
      new Set(),
    ), ObjectDataError);
  assertEquals(error.problems, [{
    file: "objects/abilities.pkl",
    message: `abilities["holy"].id: 'A000' is also the id of items["orb"] (objects/items.pkl).`,
    hint: "Give each object its own id.",
  }]);
});

Deno.test("rule: a base that is not a standard object of the category, with the nearest ids", () => {
  assertEquals(problem([["heroes", "paladin", { id: "H000", base: "Hpla" }]]), [
    `heroes["paladin"].base: 'Hpla' is not a standard hero.`,
    "Did you mean 'Hpal' (Paladin), 'Hamg' (Archmage) or 'Hmkg' (Mountain King)?",
  ]);
  assertEquals(problem([["buildings", "hall", { id: "h000", base: "hfoo" }]]), [
    `buildings["hall"].base: 'hfoo' is not a standard building.`,
    "'hfoo' is a standard unit (Footman). Did you mean 'htow' (Town Hall) or 'hbar' (Barracks)?",
  ]);
});

Deno.test("rule: an id equal to a standard object id of any category", () => {
  assertEquals(problem([["abilities", "curse", { id: "hfoo", base: "Acrs" }]]), [
    `abilities["curse"].id: 'hfoo' is the id of a standard unit (Footman).`,
    "Moonwell adds custom objects and cannot modify standard ones: pick an id no standard object uses.",
  ]);
});

Deno.test("rule: an id equal to a custom object id already in the source map", () => {
  assertEquals(problem([["units", "captain", { id: "h000", base: "hfoo" }]], ["h000"]), [
    `units["captain"].id: 'h000' is already the id of a custom object in the map.`,
    "Change the id in Pkl, or delete the object in World Editor.",
  ]);
});

Deno.test("rule: an unknown properties key, with the nearest friendly names", () => {
  assertEquals(problem([["abilities", "holy", { id: "A000", base: "AHhb", properties: { amountHealed: 1 } }]]), [
    `abilities["holy"].properties["amountHealed"]: no field that applies to 'AHhb' (Holy Light) has this rawcode or name.`,
    "Did you mean 'amountHealedOrDamaged'?",
  ]);
  assertEquals(problem([["abilities", "holy", { id: "A000", base: "AHhb", properties: { zzzz: 2 } }]]), [
    `abilities["holy"].properties["zzzz"]: no field that applies to 'AHhb' (Holy Light) has this rawcode or name.`,
    "Keys are field rawcodes, or friendly names of the fields that apply to the base.",
  ]);
  assertEquals(
    problem([["abilities", "holy", { id: "A000", base: "AHhb", properties: { manaCots: 1 } }]])[1],
    "Did you mean 'manaCost'?",
  );
});

Deno.test("rule: a friendly name shared by base-specific fields none of which applies is an unknown key", () => {
  // 'damage' names Hbz2 (Blizzard) and Ucs1 (Carrion Swarm) only; neither is the one field to blame for Holy Light.
  assertEquals(problem([["abilities", "holy", { id: "A000", base: "AHhb", properties: { damage: 1 } }]]), [
    `abilities["holy"].properties["damage"]: no field that applies to 'AHhb' (Holy Light) has this rawcode or name.`,
    "Did you mean 'amountHealedOrDamaged'?",
  ]);
  // A name that only one field has still says why that field does not apply.
  assertEquals(
    problem([["abilities", "holy", { id: "A000", base: "AHhb", properties: { chanceToMiss: 1 } }]])[0],
    `abilities["holy"].properties["chanceToMiss"]: 'Crs' (Chance to Miss) does not apply to 'AHhb' (Holy Light).`,
  );
});

Deno.test("rule: an unknown typed field is a schema version problem", () => {
  assertEquals(problem([["units", "captain", { id: "h000", base: "hfoo", typed: { hitPoints: 1 } }]]), [
    `units["captain"].hitPoints: 'hitPoints' is not a field of units.`,
    SCHEMA_HINT,
  ]);
});

Deno.test("rule: a field that does not apply to the base (use, specific, notSpecific)", () => {
  assertEquals(
    problem([["buildings", "hall", { id: "h000", base: "htow", properties: { structuresBuilt: "hbar" } }]]),
    [
      `buildings["hall"].properties["structuresBuilt"]: 'ubui' (Structures Built) does not apply to 'htow' (Town Hall).`,
      "It is a field of units and heroes only.",
    ],
  );
  assertEquals(problem([["abilities", "holy", { id: "A000", base: "AHhb", properties: { Crs: 0.5 } }]]), [
    `abilities["holy"].properties["Crs"]: 'Crs' (Chance to Miss) does not apply to 'AHhb' (Holy Light).`,
    "It applies only to copies of 'Acrs' (Curse).",
  ]);
  assertEquals(problem([["abilities", "attack", { id: "A000", base: "Aatk", typed: { tooltipLearn: "x" } }]]), [
    `abilities["attack"].tooltipLearn: 'aret' (Tooltip - Learn) does not apply to 'Aatk' (Attack).`,
    "The game's metadata excludes 'Aatk' (Attack) from it.",
  ]);
});

Deno.test("rule: a List on a field that is not per level", () => {
  assertEquals(problem([["units", "captain", { id: "h000", base: "hfoo", typed: { hitPointsMaximumBase: [1, 2] } }]]), [
    `units["captain"].hitPointsMaximumBase: 'uhpm' (Hit Points Maximum (Base)) is not per level, so it takes one value, not a List.`,
    "Write a single value.",
  ]);
  assertEquals(problem([["units", "worker", { id: "h000", base: "hpea", properties: { ubui: [["htow"]] } }]]), [
    `units["worker"].properties["ubui"]: 'ubui' (Structures Built) is not per level, so it takes one list, not a List of lists.`,
    "Write one List<String>.",
  ]);
});

Deno.test("rule: an empty List on a per-level field that is not a list field sets no levels", () => {
  assertEquals(problem([["abilities", "holy", { id: "A000", base: "AHhb", typed: { manaCost: [] } }]]), [
    `abilities["holy"].manaCost: an empty List sets no levels.`,
    "Use null to inherit every level from the base.",
  ]);
  assertEquals(
    problem([["upgrades", "swords", { id: "R000", base: "Rhme", properties: { gnam: [] } }]])[0],
    `upgrades["swords"].properties["gnam"]: an empty List sets no levels.`,
  );
});

Deno.test("rule: an object's own levels below 1", () => {
  assertEquals(problem([["abilities", "holy", { id: "A000", base: "AHhb", typed: { levels: 0, manaCost: [1] } }]]), [
    `abilities["holy"].levels: 'alev' (Levels) must be at least 1, got 0.`,
    "Every object has at least one level; use null to keep the base's.",
  ]);
  assertEquals(
    problem([["upgrades", "swords", { id: "R000", base: "Rhme", properties: { glvl: -2 } }]])[0],
    `upgrades["swords"].properties["glvl"]: 'glvl' (Levels) must be at least 1, got -2.`,
  );
});

Deno.test("rule: more List entries than the base's levels", () => {
  assertEquals(problem([["abilities", "holy", { id: "A000", base: "AHhb", typed: { castRange: [1, 2, 3, 4] } }]]), [
    `abilities["holy"].castRange: 4 levels given, but 'AHhb' (Holy Light) has 3.`,
    "Set levels = 4 to add levels, or remove values.",
  ]);
  assertEquals(problem([["upgrades", "swords", { id: "R000", base: "Rhme", typed: { name: ["1", "2", "3", "4"] } }]]), [
    `upgrades["swords"].name: 4 levels given, but 'Rhme' (Iron Forged Swords) has 3.`,
    "Set levels = 4 to add levels, or remove values.",
  ]);
});

Deno.test("rule: more List entries than the object's own levels, set typed or through properties", () => {
  assertEquals(
    problem([["abilities", "holy", { id: "A000", base: "AHhb", typed: { levels: 2, manaCost: [1, 2, 3] } }]]),
    [
      `abilities["holy"].manaCost: 3 levels given, but levels is 2.`,
      "Raise levels to 3, or remove values.",
    ],
  );
  assertEquals(
    problem([["abilities", "holy", { id: "A000", base: "AHhb", properties: { alev: 1, amcs: [1, 2] } }]])[0],
    `abilities["holy"].properties["amcs"]: 2 levels given, but levels is 1.`,
  );
  // The object's own levels may exceed the base's.
  const [holy] = resolve([["abilities", "holy", {
    id: "A000",
    base: "AHhb",
    typed: { levels: 5, manaCost: [1, 2, 3, 4, 5] },
  }]]);
  assertEquals(holy.fields.filter((field) => field.id === "amcs").length, 5);
  const [swords] = resolve([["upgrades", "swords", {
    id: "R000",
    base: "Rhme",
    typed: { levels: 4, name: ["1", "2", "3", "4"] },
  }]]);
  assertEquals(swords.fields.filter((field) => field.id === "gnam").length, 4);
});

Deno.test("rule: a base ability with 0 levels in the metadata counts as 1", () => {
  const [build] = resolve([["abilities", "build", { id: "A000", base: "AHbu", typed: { manaCost: [5] } }]]);
  assertEquals(build.fields.map((field) => field.level), [1]);
  assertEquals(problem([["abilities", "build", { id: "A000", base: "AHbu", typed: { manaCost: [5, 6] } }]]), [
    `abilities["build"].manaCost: 2 levels given, but 'AHbu' (Build (Human)) has 1.`,
    "Set levels = 2 to add levels, or remove values.",
  ]);
});

Deno.test("rule: a value of the wrong storage type in properties", () => {
  const unit = (
    properties: Record<string, unknown>,
  ): [Category, string, Entry] => ["units", "captain", { id: "h000", base: "hfoo", properties } as Entry];
  assertEquals(problem([unit({ uhpm: 1.5 })]), [
    `units["captain"].properties["uhpm"]: expected an integer, got 1.5.`,
    "'uhpm' (Hit Points Maximum (Base)) is stored as an integer.",
  ]);
  assertEquals(problem([unit({ uhpm: 2 ** 31 })]), [
    `units["captain"].properties["uhpm"]: 2147483648 is out of range for an integer.`,
    "'uhpm' (Hit Points Maximum (Base)) is stored as an integer.",
  ]);
  assertEquals(problem([unit({ uhpm: "10" })]), [
    `units["captain"].properties["uhpm"]: expected an integer, got "10".`,
    "'uhpm' (Hit Points Maximum (Base)) is stored as an integer.",
  ]);
  assertEquals(problem([unit({ uacq: 1e39 })]), [
    `units["captain"].properties["uacq"]: 1e+39 is out of range for a real number.`,
    "'uacq' (Acquisition Range) is stored as a real number.",
  ]);
  assertEquals(
    problem([unit({ uacq: Infinity })])[0],
    `units["captain"].properties["uacq"]: Infinity is out of range for a real number.`,
  );
  assertEquals(problem([unit({ usca: true })]), [
    `units["captain"].properties["usca"]: expected a number, got true.`,
    "'usca' (Scaling Value) is stored as a real number.",
  ]);
  assertEquals(problem([unit({ unam: "a\0b" })]), [
    `units["captain"].properties["unam"]: the string contains a NUL character.`,
    "Remove it: the game ends strings at NUL.",
  ]);
  assertEquals(problem([unit({ unam: 3 })]), [
    `units["captain"].properties["unam"]: expected a string, got 3.`,
    "'unam' (Name) is stored as a string.",
  ]);
  assertEquals(problem([["units", "worker", { id: "h000", base: "hpea", properties: { ubui: ["htow", 1] } }]]), [
    `units["worker"].properties["ubui"][1]: expected a string, got 1.`,
    "'ubui' (Structures Built) is stored as a comma-separated list.",
  ]);
  assertEquals(problem([["abilities", "holy", { id: "A000", base: "AHhb", typed: { manaCost: [1, 2.5] } }]]), [
    `abilities["holy"].manaCost[1]: expected an integer, got 2.5.`,
    "'amcs' (Mana Cost) is stored as an integer.",
  ]);
  // Integers are accepted for real fields; Booleans only for int fields.
  const [captain] = resolve([unit({ uacq: 600, uhpm: false })]);
  assertEquals(captain.fields.map((field) => field.value), [{ type: "unreal", value: 600 }, { type: "int", value: 0 }]);
});

Deno.test("rule: setting the same field twice, typed and in properties or by rawcode and by name", () => {
  assertEquals(
    problem([["units", "captain", {
      id: "h000",
      base: "hfoo",
      typed: { hitPointsMaximumBase: 1 },
      properties: { uhpm: 2 },
    }]]),
    [
      `units["captain"].properties["uhpm"]: 'uhpm' (Hit Points Maximum (Base)) is already set by hitPointsMaximumBase.`,
      "Set each field once.",
    ],
  );
  assertEquals(
    problem([["abilities", "curse", { id: "A000", base: "Acrs", properties: { Crs: 1, chanceToMiss: 2 } }]])[0],
    `abilities["curse"].properties["chanceToMiss"]: 'Crs' (Chance to Miss) is already set by properties["Crs"].`,
  );
});

// Reporting

Deno.test("resolveObjects reports every problem; file is the first problem's source", () => {
  const error = assertThrows(() =>
    resolveObjects(
      metadata,
      {
        ...emptyObjects(),
        heroes: { paladin: { id: "H000", base: "Hpla", source: "objects/heroes.pkl", typed: {}, properties: {} } },
        units: {
          captain: { id: "hfoo", base: "hfoo", source: "objects/units.pkl", typed: {}, properties: { uhpm: 1.5 } },
        },
      },
      new Set(),
    ), ObjectDataError);
  assertEquals(error.problems.map((p) => [p.file, p.message]), [
    ["objects/heroes.pkl", `heroes["paladin"].base: 'Hpla' is not a standard hero.`],
    ["objects/units.pkl", `units["captain"].id: 'hfoo' is the id of a standard unit (Footman).`],
    ["objects/units.pkl", `units["captain"].properties["uhpm"]: expected an integer, got 1.5.`],
  ]);
  assertEquals(error.file, "objects/heroes.pkl");
  assertEquals(error.message, error.problems[0].message);
  assertEquals(
    formatError(error),
    [
      `error: objects/heroes.pkl › heroes["paladin"].base: 'Hpla' is not a standard hero.`,
      "hint: Did you mean 'Hpal' (Paladin), 'Hamg' (Archmage) or 'Hmkg' (Mountain King)?",
      `error: objects/units.pkl › units["captain"].id: 'hfoo' is the id of a standard unit (Footman).`,
      "hint: Moonwell adds custom objects and cannot modify standard ones: pick an id no standard object uses.",
      `error: objects/units.pkl › units["captain"].properties["uhpm"]: expected an integer, got 1.5.`,
      "hint: 'uhpm' (Hit Points Maximum (Base)) is stored as an integer.",
    ].join("\n"),
  );
});

Deno.test("resolveObjects renders at most 20 problems, then how many more", () => {
  const entries: [Category, string, Entry][] = Array.from(
    { length: 23 },
    (_, i) => ["units", `u${i}`, { id: `h${String(i).padStart(3, "0")}`, base: "hfoo" }],
  );
  const error = assertThrows(() => resolve(entries, entries.map(([, , entry]) => entry.id)), ObjectDataError);
  assertInstanceOf(error, ObjectDataError);
  assertEquals(error.problems.length, 23);
  const lines = formatError(error).split("\n");
  assertEquals(lines.length, 41);
  assertEquals(
    lines[38],
    `error: objects/a.pkl › units["u19"].id: 'h019' is already the id of a custom object in the map.`,
  );
  assertEquals(lines[40], "and 3 more");
});
