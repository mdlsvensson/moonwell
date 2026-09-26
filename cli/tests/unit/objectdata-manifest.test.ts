import { assertEquals, assertThrows } from "@std/assert";
import { MoonwellError } from "../../src/shared/errors.ts";
import { emptyObjects, parseObjects, SCHEMA_HINT } from "../../src/objectdata/manifest.ts";

Deno.test("parseObjects: an absent objects key is an empty manifest", () => {
  assertEquals(parseObjects(undefined, "moonwell.pkl"), emptyObjects());
  assertEquals(emptyObjects(), {
    heroes: {},
    units: {},
    buildings: {},
    items: {},
    abilities: {},
    buffs: {},
    upgrades: {},
  });
});

Deno.test("parseObjects splits reserved keys from typed fields and keeps category order", () => {
  const objects = parseObjects({
    abilities: {
      holy: {
        base: "AHhb",
        source: "objects/a.pkl",
        properties: { Crs: [0.5], amountHealedOrDamaged: 3 },
        castRange: [1, 2.5],
        heroAbility: false,
        buffs: [["BHbd", "Bcrs"], []],
        levels: 4,
        id: "A000",
      },
    },
    units: { captain: { id: "h000", base: "hfoo", name: "", properties: {} } },
  }, "moonwell.pkl");
  assertEquals(Object.keys(objects), ["heroes", "units", "buildings", "items", "abilities", "buffs", "upgrades"]);
  assertEquals(objects.abilities.holy, {
    id: "A000",
    base: "AHhb",
    source: "objects/a.pkl",
    typed: { castRange: [1, 2.5], heroAbility: false, buffs: [["BHbd", "Bcrs"], []], levels: 4 },
    properties: { Crs: [0.5], amountHealedOrDamaged: 3 },
  });
  assertEquals(objects.units.captain.typed, { name: "" });
});

Deno.test("parseObjects: a missing source is the evaluated manifest (inline or moonwell.local.pkl objects)", () => {
  const objects = parseObjects(
    { units: { captain: { id: "h000", base: "hfoo", properties: {} } } },
    "moonwell.local.pkl",
  );
  assertEquals(objects.units.captain.source, "moonwell.local.pkl");
});

Deno.test("parseObjects skips null values and a missing properties block", () => {
  const objects = parseObjects(
    { units: { captain: { id: "h000", base: "hfoo", name: null } } },
    "moonwell.pkl",
  );
  assertEquals(objects.units.captain.typed, {});
  assertEquals(objects.units.captain.properties, {});
  const withNull = parseObjects(
    { units: { captain: { id: "h000", base: "hfoo", properties: { uhpm: null } } } },
    "moonwell.pkl",
  );
  assertEquals(withNull.units.captain.properties, {});
});

Deno.test("parseObjects rejects a wrong shape with the version hint and the manifest as file", () => {
  const cases: [unknown, string][] = [
    [[], "objects must be an object."],
    [{ spells: {} }, "objects.spells is not an object category."],
    [{ units: [] }, "objects.units must be an object."],
    [{ units: { a: "x" } }, 'objects.units["a"] must be an object.'],
    [{ units: { a: { base: "hfoo" } } }, 'objects.units["a"].id must be a string.'],
    [{ units: { a: { id: "h000" } } }, 'objects.units["a"].base must be a string.'],
    [{ units: { a: { id: "h000", base: "hfoo", source: 1 } } }, 'objects.units["a"].source must be a string.'],
    [
      { units: { a: { id: "h000", base: "hfoo", properties: [] } } },
      'objects.units["a"].properties must be an object.',
    ],
    [
      { units: { a: { id: "h000", base: "hfoo", name: { x: 1 } } } },
      'objects.units["a"].name must be a Boolean, number, string or List.',
    ],
    [
      { units: { a: { id: "h000", base: "hfoo", properties: { x: [[1]] } } } },
      'objects.units["a"].properties["x"] must be a Boolean, number, string or List.',
    ],
    [
      { units: { a: { id: "h000", base: "hfoo", name: [null] } } },
      'objects.units["a"].name must be a Boolean, number, string or List.',
    ],
  ];
  for (const [value, message] of cases) {
    const error = assertThrows(() => parseObjects(value, "moonwell.local.pkl"), MoonwellError);
    assertEquals([error.message, error.file, error.hint], [message, "moonwell.local.pkl", SCHEMA_HINT]);
  }
  assertEquals(SCHEMA_HINT, "Is the moonwell Pkl package the version this CLI expects?");
});
