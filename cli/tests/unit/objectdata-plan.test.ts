import { assertEquals, assertInstanceOf, assertRejects, assertStringIncludes } from "@std/assert";
import { join } from "@std/path";
import { renderObjectIds } from "../../src/objectdata/ids.ts";
import { emptyObjects, type ManifestObject, type ProjectObjects } from "../../src/objectdata/manifest.ts";
import { type Category, loadMetadata } from "../../src/objectdata/metadata.ts";
import { type ModFile, readModFile, tableKind } from "../../src/objectdata/modfile.ts";
import { applyObjectPlan, planObjectData } from "../../src/objectdata/plan.ts";
import { MoonwellError, ObjectDataError } from "../../src/shared/errors.ts";
import { buildModFile, miniMetadata, namesFixtureBytes } from "../support/objectdata.ts";

const SOURCE = "maps/map.w3x";
const options = { metadata: miniMetadata(), manifest: "moonwell.pkl", sourceLabel: SOURCE };
const OBJECT_FILES = ["w3u", "w3t", "w3h", "w3a", "w3q"].flatMap((ext) => [`war3map.${ext}`, `war3mapSkin.${ext}`]);
const FIXTURE_FILES = [...OBJECT_FILES, "war3map.w3b", "war3mapSkin.w3b", "war3map.w3d", "war3mapSkin.w3d"];

type Entry = Partial<ManifestObject> & { id: string; base: string };

function objects(entries: [Category, string, Entry][]): ProjectObjects {
  const result = emptyObjects();
  for (const [category, key, entry] of entries) {
    result[category][key] = { source: "objects/a.pkl", typed: {}, properties: {}, ...entry };
  }
  return result;
}

async function withDir(run: (dir: string) => Promise<void>): Promise<void> {
  const dir = await Deno.makeTempDir();
  try {
    await run(dir);
  } finally {
    await Deno.remove(dir, { recursive: true });
  }
}

async function copyFixture(dir: string, files = FIXTURE_FILES): Promise<void> {
  for (const file of files) await Deno.writeFile(join(dir, file), await namesFixtureBytes(file));
}

/** Every entry below `dir` with its bytes, to prove planning wrote nothing. */
async function snapshot(dir: string): Promise<Record<string, Uint8Array>> {
  const result: Record<string, Uint8Array> = {};
  for await (const entry of Deno.readDir(dir)) result[entry.name] = await Deno.readFile(join(dir, entry.name));
  return result;
}

const parse = (name: string, bytes: Uint8Array): ModFile => readModFile(bytes, tableKind(name), name);
const customObjects = (name: string, bytes: Uint8Array) =>
  parse(name, bytes).custom.objects.map((object) => ({
    base: object.base,
    id: object.id,
    mods: object.sets.flatMap((set) => set.mods.map((mod) => [mod.field, mod.level, mod.column, mod.value.value])),
  }));

// No objects

Deno.test("planObjectData: no objects needs no map folder and returns the empty generated module", async () => {
  const plan = await planObjectData("/definitely/not/a/map", emptyObjects(), options);
  assertEquals(plan, { changes: [], generated: renderObjectIds([]), objects: [] });
});

// Known answer

Deno.test("known answer: the names fixture's objects planned against an empty map are World Editor's files", async () => {
  // Moonwell writes strings literally, so the fixture's TRIGSTR references are the names given here (fixture README).
  const names = objects([
    ["units", "peasant", { id: "h000", base: "hpea", typed: { name: "TRIGSTR_012" } }],
    ["items", "claws", { id: "I000", base: "ratf", typed: { name: "TRIGSTR_013" } }],
    ["abilities", "acid", { id: "A000", base: "ANab", typed: { name: "TRIGSTR_016" } }],
    ["buffs", "acid", { id: "B000", base: "BNab", typed: { nameEditorOnly: "TRIGSTR_017" } }],
    ["upgrades", "swords", { id: "R000", base: "Rhme", typed: { name: "TRIGSTR_018" } }],
  ]);
  await withDir(async (dir) => {
    const plan = await planObjectData(dir, names, {
      metadata: await loadMetadata(),
      manifest: "moonwell.pkl",
      sourceLabel: SOURCE,
    });
    assertEquals(plan.changes.map((change) => change.name), [
      "war3map.w3u",
      "war3map.w3t",
      "war3map.w3h",
      "war3map.w3a",
      "war3map.w3q",
      "war3mapSkin.w3u",
      "war3mapSkin.w3t",
      "war3mapSkin.w3h",
      "war3mapSkin.w3a",
      "war3mapSkin.w3q",
    ]);
    for (const { name, bytes } of plan.changes) assertEquals(bytes, await namesFixtureBytes(name), name);
    assertEquals(plan.objects.map((object) => object.id), ["h000", "I000", "A000", "B000", "R000"]);
    assertEquals(plan.generated, renderObjectIds(plan.objects));
    assertEquals(await snapshot(dir), {});
  });
});

/** mdx-m3-viewer-th `Modification.save`, restated: id, var type, [level, data pointer], value, end token (`u1`, 0). */
function referenceModification(
  mod: { id: string; variableType: number; level: number; dataPointer: number; value: number | string },
  useOptionalInts: boolean,
): Uint8Array {
  const out: number[] = [];
  const int = (n: number) => out.push(...new Uint8Array(new Int32Array([n]).buffer));
  out.push(...Array.from(mod.id, (c) => c.charCodeAt(0)));
  int(mod.variableType);
  if (useOptionalInts) {
    int(mod.level);
    int(mod.dataPointer);
  }
  if (mod.variableType === 0) int(mod.value as number);
  else if (mod.variableType === 1 || mod.variableType === 2) {
    out.push(...new Uint8Array(new Float32Array([mod.value as number]).buffer));
  } else out.push(...new TextEncoder().encode(mod.value as string), 0);
  int(0);
  return new Uint8Array(out);
}

/** mdx-m3-viewer-th `War3MapW3u.save` for a version 2 file with custom objects only, restated. */
function referenceV2File(
  custom: { oldId: string; newId: string; mods: Parameters<typeof referenceModification>[0][] }[],
  useOptionalInts: boolean,
): Uint8Array {
  const out: number[] = [];
  const int = (n: number) => out.push(...new Uint8Array(new Int32Array([n]).buffer));
  int(2);
  int(0);
  int(custom.length);
  for (const object of custom) {
    out.push(...Array.from(object.oldId + object.newId, (c) => c.charCodeAt(0)));
    int(object.mods.length);
    for (const mod of object.mods) out.push(...referenceModification(mod, useOptionalInts));
  }
  return new Uint8Array(out);
}

Deno.test("known answer: each var type encodes as the reference library's Modification.save layout", async () => {
  const manifest = objects([
    ["units", "captain", {
      id: "h000",
      base: "hfoo",
      typed: { hitPointsMaximumBase: 500, scalingValue: 1.25, acquisitionRange: 600.5, name: "Captain" },
    }],
    ["abilities", "holy", {
      id: "A000",
      base: "AHhb",
      typed: { heroAbility: true, manaCost: [75, 80], name: "Holier" },
      properties: { Hhb1: [200.5] },
    }],
  ]);
  await withDir(async (dir) => {
    const plan = await planObjectData(dir, manifest, options);
    const bytes = Object.fromEntries(plan.changes.map((change) => [change.name, change.bytes]));
    const expected: Record<string, Parameters<typeof referenceModification>[0][]> = {
      "war3map.w3u": [
        { id: "uacq", variableType: 2, level: 0, dataPointer: 0, value: 600.5 },
        { id: "uhpm", variableType: 0, level: 0, dataPointer: 0, value: 500 },
      ],
      "war3mapSkin.w3u": [
        { id: "unam", variableType: 3, level: 0, dataPointer: 0, value: "Captain" },
        { id: "usca", variableType: 1, level: 0, dataPointer: 0, value: 1.25 },
      ],
      "war3map.w3a": [
        { id: "Hhb1", variableType: 2, level: 1, dataPointer: 1, value: 200.5 },
        { id: "aher", variableType: 0, level: 0, dataPointer: 0, value: 1 },
        { id: "amcs", variableType: 0, level: 1, dataPointer: 0, value: 75 },
        { id: "amcs", variableType: 0, level: 2, dataPointer: 0, value: 80 },
      ],
      "war3mapSkin.w3a": [{ id: "anam", variableType: 3, level: 0, dataPointer: 0, value: "Holier" }],
    };
    assertEquals(Object.keys(bytes).sort(), Object.keys(expected).sort());
    for (const [name, mods] of Object.entries(expected)) {
      const file = bytes[name];
      const [object] = parse(name, file).custom.objects;
      const written = object.sets[0].mods.map((mod) => file.subarray(mod.start, mod.stop));
      assertEquals(written, mods.map((mod) => referenceModification(mod, tableKind(name) === "leveled")), name);
    }
  });
});

Deno.test("known answer: a version 2 map gets the reference library's version 2 layout, all fields in the main file", async () => {
  await withDir(async (dir) => {
    await Deno.writeFile(join(dir, "war3map.w3a"), buildModFile({ version: 2 }, "leveled"));
    const plan = await planObjectData(
      dir,
      objects([["abilities", "holy", { id: "A000", base: "AHhb", typed: { name: "Holier", castRange: [500] } }]]),
      options,
    );
    assertEquals(plan.changes.map((change) => change.name), ["war3map.w3a"]);
    assertEquals(
      plan.changes[0].bytes,
      referenceV2File([{
        oldId: "AHhb",
        newId: "A000",
        mods: [
          { id: "anam", variableType: 3, level: 0, dataPointer: 0, value: "Holier" },
          { id: "aran", variableType: 2, level: 1, dataPointer: 0, value: 500 },
        ],
      }], true),
    );
  });
});

// Planning against an existing map

Deno.test("planObjectData appends to the existing files, splits by skin, sorts by id and writes nothing", async () => {
  await withDir(async (dir) => {
    await copyFixture(dir);
    const before = await snapshot(dir);
    const plan = await planObjectData(
      dir,
      objects([
        ["units", "knight", { id: "h002", base: "hkni", typed: { hitPointsMaximumBase: 900 } }],
        ["units", "captain", { id: "h001", base: "hfoo", typed: { name: "Captain", hitPointsMaximumBase: 500 } }],
        ["upgrades", "plating", { id: "R001", base: "Rhar", typed: { name: ["I", "II"] } }],
      ]),
      options,
    );
    assertEquals(await snapshot(dir), before);
    assertEquals(plan.changes.map((change) => change.name), [
      "war3map.w3u",
      "war3map.w3q",
      "war3mapSkin.w3u",
      "war3mapSkin.w3q",
    ]);
    const bytes = Object.fromEntries(plan.changes.map((change) => [change.name, change.bytes]));
    // The existing bytes are kept (count aside) and the new objects follow, sorted by id, on both sides.
    for (const name of Object.keys(bytes)) {
      const source = before[name];
      assertEquals(bytes[name].subarray(0, 8), source.subarray(0, 8), name);
      assertEquals(bytes[name].subarray(12, source.length), source.subarray(12), name);
    }
    assertEquals(customObjects("war3map.w3u", bytes["war3map.w3u"]), [
      { base: "hpea", id: "h000", mods: [] },
      { base: "hfoo", id: "h001", mods: [["uhpm", 0, 0, 500]] },
      { base: "hkni", id: "h002", mods: [["uhpm", 0, 0, 900]] },
    ]);
    assertEquals(customObjects("war3mapSkin.w3u", bytes["war3mapSkin.w3u"]), [
      { base: "hpea", id: "h000", mods: [["unam", 0, 0, "TRIGSTR_012"]] },
      { base: "hfoo", id: "h001", mods: [["unam", 0, 0, "Captain"]] },
      { base: "hkni", id: "h002", mods: [] },
    ]);
    assertEquals(customObjects("war3map.w3q", bytes["war3map.w3q"]).at(-1), { base: "Rhar", id: "R001", mods: [] });
    assertEquals(customObjects("war3mapSkin.w3q", bytes["war3mapSkin.w3q"]).at(-1), {
      base: "Rhar",
      id: "R001",
      mods: [["gnam", 1, 0, "I"], ["gnam", 2, 0, "II"]],
    });
    // Resolved objects stay in category, then manifest order; only the files are sorted by id.
    assertEquals(plan.objects.map((object) => object.key), ["knight", "captain", "plating"]);
    assertEquals(plan.generated, renderObjectIds(plan.objects));
  });
});

Deno.test("heroes, units and buildings share the w3u files, sorted by id across the three", async () => {
  await withDir(async (dir) => {
    const plan = await planObjectData(
      dir,
      objects([
        ["units", "footman", { id: "h002", base: "hfoo" }],
        ["buildings", "barracks", { id: "h001", base: "hbar" }],
        ["heroes", "paladin", { id: "H000", base: "Hpal" }],
      ]),
      options,
    );
    assertEquals(plan.changes.map((change) => change.name), ["war3map.w3u", "war3mapSkin.w3u"]);
    for (const { name, bytes } of plan.changes) {
      assertEquals(customObjects(name, bytes), [
        { base: "Hpal", id: "H000", mods: [] },
        { base: "hbar", id: "h001", mods: [] },
        { base: "hfoo", id: "h002", mods: [] },
      ]);
    }
  });
});

Deno.test("applyObjectPlan writes the planned bytes into the staged folder under their names", async () => {
  await withDir(async (dir) => {
    const source = join(dir, "source"), staged = join(dir, "staged");
    await Deno.mkdir(source);
    await Deno.mkdir(staged);
    await copyFixture(source);
    await copyFixture(staged);
    const plan = await planObjectData(
      source,
      objects([["items", "orb", { id: "I001", base: "ckng", typed: { perishable: true } }]]),
      options,
    );
    await applyObjectPlan(plan, staged);
    for (const { name, bytes } of plan.changes) assertEquals(await Deno.readFile(join(staged, name)), bytes);
    assertEquals(await Deno.readFile(join(staged, "war3map.w3u")), await namesFixtureBytes("war3map.w3u"));
    const blocked = join(dir, "blocked");
    await Deno.mkdir(join(blocked, "war3map.w3t"), { recursive: true });
    const error = await assertRejects(() => applyObjectPlan(plan, blocked), MoonwellError);
    assertEquals(error.file, join(blocked, "war3map.w3t"));
  });
});

Deno.test("a v3 main file without its skin file gets a new skin file; v1 and v2 maps get none", async () => {
  await withDir(async (dir) => {
    await copyFixture(dir, ["war3map.w3u"]);
    const captain = objects([["units", "captain", { id: "h001", base: "hfoo", typed: { name: "Captain" } }]]);
    const plan = await planObjectData(dir, captain, options);
    assertEquals(plan.changes.map((change) => change.name), ["war3map.w3u", "war3mapSkin.w3u"]);
    assertEquals(parse("war3mapSkin.w3u", plan.changes[1].bytes).version, 3);
    for (const version of [1, 2]) {
      await Deno.writeFile(join(dir, "war3map.w3u"), buildModFile({ version }, "simple"));
      // A skin file next to an old main file is left alone: the main file's version decides the split.
      await Deno.writeFile(join(dir, "war3mapSkin.w3u"), await namesFixtureBytes("war3mapSkin.w3u"));
      const old = await planObjectData(dir, captain, options);
      assertEquals(old.changes.map((change) => change.name), ["war3map.w3u"]);
      assertEquals(customObjects("war3map.w3u", old.changes[0].bytes), [
        { base: "hfoo", id: "h001", mods: [["unam", 0, 0, "Captain"]] },
      ]);
      assertEquals(parse("war3map.w3u", old.changes[0].bytes).version, version);
      await Deno.remove(join(dir, "war3mapSkin.w3u"));
    }
  });
});

// Map-file rules

Deno.test("an id already used by a custom object in any of the map's tables fails and writes nothing", async () => {
  await withDir(async (dir) => {
    await copyFixture(dir);
    const before = await snapshot(dir);
    // h000 is the fixture's custom unit and B000 its custom buff; either collides in any category.
    const error = await assertRejects(
      () =>
        planObjectData(
          dir,
          objects([
            ["abilities", "holy", { id: "h000", base: "AHhb" }],
            ["items", "orb", { id: "B000", base: "ckng" }],
            ["units", "captain", { id: "h001", base: "hfoo" }],
          ]),
          options,
        ),
      ObjectDataError,
    );
    assertEquals(error.problems.map((problem) => problem.message), [
      `items["orb"].id: 'B000' is already the id of a custom object in the map.`,
      `abilities["holy"].id: 'h000' is already the id of a custom object in the map.`,
    ]);
    assertEquals(await snapshot(dir), before);
  });
});

Deno.test("a custom id only in a skin file still counts as existing", async () => {
  await withDir(async (dir) => {
    await Deno.writeFile(
      join(dir, "war3mapSkin.w3h"),
      buildModFile({ version: 3, custom: [{ base: "BNab", id: "X001" }] }, "simple"),
    );
    const error = await assertRejects(
      () => planObjectData(dir, objects([["abilities", "holy", { id: "X001", base: "AHhb" }]]), options),
      ObjectDataError,
    );
    assertEquals(error.problems.map((problem) => problem.message), [
      `abilities["holy"].id: 'X001' is already the id of a custom object in the map.`,
    ]);
  });
});

Deno.test("a map file with the same custom id twice fails naming that file", async () => {
  await withDir(async (dir) => {
    await Deno.writeFile(
      join(dir, "war3map.w3a"),
      buildModFile({ version: 3, custom: [{ base: "AHhb", id: "A001" }, { base: "Acrs", id: "A001" }] }, "leveled"),
    );
    const before = await snapshot(dir);
    const error = await assertRejects(
      () => planObjectData(dir, objects([["units", "captain", { id: "h001", base: "hfoo" }]]), options),
      MoonwellError,
    );
    assertEquals(error.message, "Custom object 'A001' appears twice in this file.");
    assertEquals(error.file, "maps/map.w3x/war3map.w3a");
    assertStringIncludes(error.hint!, "World Editor 3.00");
    assertEquals(await snapshot(dir), before);
  });
});

Deno.test("a malformed map file is a file error naming the source map file", async () => {
  await withDir(async (dir) => {
    await Deno.writeFile(join(dir, "war3mapSkin.w3q"), new Uint8Array([3, 0, 0]));
    const error = await assertRejects(
      () => planObjectData(dir, objects([["units", "captain", { id: "h001", base: "hfoo" }]]), options),
      MoonwellError,
    );
    assertEquals(error.file, "maps/map.w3x/war3mapSkin.w3q");
    assertStringIncludes(error.hint!, "World Editor 3.00");
  });
});

Deno.test("object files are found in any letter case and written back under the existing name", async () => {
  await withDir(async (dir) => {
    await Deno.writeFile(join(dir, "WAR3MAP.W3U"), await namesFixtureBytes("war3map.w3u"));
    await Deno.writeFile(join(dir, "war3mapskin.w3u"), await namesFixtureBytes("war3mapSkin.w3u"));
    const plan = await planObjectData(
      dir,
      objects([["units", "captain", { id: "h001", base: "hfoo", typed: { name: "Captain" } }]]),
      options,
    );
    assertEquals(plan.changes.map((change) => change.name), ["WAR3MAP.W3U", "war3mapskin.w3u"]);
    assertEquals(customObjects("war3mapskin.w3u", plan.changes[1].bytes).map((object) => object.id), ["h000", "h001"]);
    // The existing h000 is found under its other spelling too.
    const error = await assertRejects(
      () => planObjectData(dir, objects([["units", "peasant", { id: "h000", base: "hpea" }]]), options),
      ObjectDataError,
    );
    assertStringIncludes(error.message, "already the id of a custom object");
  });
});

Deno.test("two object files differing only in letter case fail naming the file and write nothing", async () => {
  await withDir(async (dir) => {
    await Deno.writeFile(join(dir, "war3map.w3a"), await namesFixtureBytes("war3map.w3a"));
    await Deno.writeFile(join(dir, "war3map.W3A"), await namesFixtureBytes("war3map.w3a"));
    const before = await snapshot(dir);
    const error = await assertRejects(
      () => planObjectData(dir, objects([["units", "captain", { id: "h001", base: "hfoo" }]]), options),
      MoonwellError,
    );
    assertEquals(error.message, "Map files war3map.W3A and war3map.w3a differ only in letter case.");
    assertEquals(error.file, "maps/map.w3x/war3map.w3a");
    assertStringIncludes(error.hint!, "letter case");
    assertEquals(await snapshot(dir), before);
  });
});

Deno.test("with objects, a missing map folder is an error naming the manifest", async () => {
  await withDir(async (dir) => {
    const error = await assertRejects(
      () => planObjectData(join(dir, "missing"), objects([["units", "c", { id: "h001", base: "hfoo" }]]), options),
      MoonwellError,
    );
    assertEquals([error.message, error.file], ["Source map folder maps/map.w3x not found.", "moonwell.pkl"]);
  });
});

Deno.test("invalid objects fail with every problem before any map file is written", async () => {
  await withDir(async (dir) => {
    await copyFixture(dir);
    const before = await snapshot(dir);
    const error = await assertRejects(
      () =>
        planObjectData(
          dir,
          objects([
            ["units", "a", { id: "h001", base: "hfox" }],
            ["units", "b", { id: "h002", base: "hfoo", properties: { uhpm: 1.5 } }],
          ]),
          options,
        ),
      ObjectDataError,
    );
    assertInstanceOf(error, ObjectDataError);
    assertEquals(error.problems.length, 2);
    assertEquals(await snapshot(dir), before);
  });
});
