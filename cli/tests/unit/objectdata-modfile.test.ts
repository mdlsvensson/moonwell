import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
import {
  appendObjects,
  type ModFile,
  type ModValue,
  type NewObject,
  type ObjectEntry,
  readModFile,
  type TableKind,
  tableKind,
} from "../../src/objectdata/modfile.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { buildModFile, namesFixtureBytes, type SyntheticObject } from "../support/objectdata.ts";

// The fixture README's table: one custom object per tab, its name as a TRIGSTR reference.
const NAMES = [
  { ext: "w3u", base: "hpea", id: "h000", field: "unam", skin: true, level: 0, value: "TRIGSTR_012" },
  { ext: "w3t", base: "ratf", id: "I000", field: "unam", skin: true, level: 0, value: "TRIGSTR_013" },
  { ext: "w3b", base: "DTrf", id: "B000", field: "bnam", skin: true, level: 0, value: "TRIGSTR_014" },
  { ext: "w3d", base: "UObb", id: "D000", field: "dnam", skin: false, level: 0, value: "TRIGSTR_015" },
  { ext: "w3a", base: "ANab", id: "A000", field: "anam", skin: true, level: 0, value: "TRIGSTR_016" },
  { ext: "w3h", base: "BNab", id: "B000", field: "fnam", skin: true, level: 0, value: "TRIGSTR_017" },
  { ext: "w3q", base: "Rhme", id: "R000", field: "gnam", skin: true, level: 1, value: "TRIGSTR_018" },
];

const expectFileError = (bytes: Uint8Array, problem: string, kind: "simple" | "leveled" = "simple") => {
  const error = assertThrows(() => readModFile(bytes, kind, "map/war3map.w3u"), MoonwellError);
  assertEquals(error.file, "map/war3map.w3u");
  assertStringIncludes(error.message, problem);
  assertStringIncludes(error.hint!, "World Editor 3.00");
};

const int32 = (n: number) => {
  const b = new Uint8Array(4);
  new DataView(b.buffer).setInt32(0, n, true);
  return b;
};

Deno.test("table kind is leveled for w3a, w3d and w3q, by extension and case-insensitively", () => {
  assertEquals(tableKind("war3map.w3u"), "simple");
  assertEquals(tableKind("war3mapSkin.W3T"), "simple");
  assertEquals(tableKind("war3map.w3b"), "simple");
  assertEquals(tableKind("war3map.w3h"), "simple");
  assertEquals(tableKind("war3map.w3a"), "leveled");
  assertEquals(tableKind("war3mapSkin.W3D"), "leveled");
  assertEquals(tableKind("map/war3map.w3q"), "leveled");
});

Deno.test("every World Editor names file parses as the fixture README records", async () => {
  for (const name of NAMES) {
    for (const skin of [false, true]) {
      const file = `${skin ? "war3mapSkin" : "war3map"}.${name.ext}`;
      const bytes = await namesFixtureBytes(file);
      const parsed = readModFile(bytes, tableKind(file), file);
      assertEquals(parsed.version, 3, file);
      assertEquals(parsed.original, { countOffset: 4, start: 8, stop: 8, objects: [] }, file);
      assertEquals(parsed.custom.countOffset, 8, file);
      assertEquals([parsed.custom.start, parsed.custom.stop], [12, bytes.length], file);
      assertEquals(parsed.custom.objects.length, 1, file);
      const [object] = parsed.custom.objects;
      assertEquals([object.base, object.id, object.start, object.stop], [name.base, name.id, 12, bytes.length], file);
      assertEquals(object.sets.length, 1, file);
      assertEquals(object.sets[0].flag, 0, file);
      const mods = object.sets[0].mods;
      if (skin !== name.skin) {
        assertEquals(mods, [], file);
        continue;
      }
      assertEquals(mods, [{
        field: name.field,
        level: name.level,
        column: 0,
        value: { type: "string", value: name.value },
        end: "\0\0\0\0",
        start: 32,
        stop: bytes.length,
      }], file);
    }
  }
});

Deno.test("synthetic files round-trip int, real, unreal and string values", () => {
  const custom: SyntheticObject[] = [
    {
      base: "hfoo",
      id: "h001",
      sets: [{
        flag: 0,
        mods: [
          { field: "uhpm", value: { type: "int", value: -250 } },
          { field: "umvs", value: { type: "real", value: 0.1 } },
          { field: "ucbs", value: { type: "unreal", value: 1.3 } },
          { field: "unam", value: { type: "string", value: "Møønwell" }, end: "h001" },
        ],
      }, { flag: 7, mods: [{ field: "utip", value: { type: "string", value: "" } }] }],
    },
  ];
  const original: SyntheticObject[] = [{
    base: "hpea",
    id: "\0\0\0\0",
    mods: [{ field: "ugol", value: { type: "int", value: 2147483647 } }],
  }];
  for (const kind of ["simple", "leveled"] as const) {
    const leveled = kind === "leveled";
    const bytes = buildModFile({ version: 3, original, custom }, kind);
    const parsed = readModFile(bytes, kind, "war3map.w3u");
    const values = (file: ModFile) =>
      file.custom.objects[0].sets.map((set) =>
        set.mods.map(({ field, value, end }) => [field, value.type, value.value, end])
      );
    assertEquals(values(parsed), [
      [
        ["uhpm", "int", -250, "\0\0\0\0"],
        ["umvs", "real", Math.fround(0.1), "\0\0\0\0"],
        ["ucbs", "unreal", Math.fround(1.3), "\0\0\0\0"],
        ["unam", "string", "Møønwell", "h001"],
      ],
      [["utip", "string", "", "\0\0\0\0"]],
    ]);
    assertEquals(parsed.custom.objects[0].sets.map((set) => set.flag), [0, 7]);
    assertEquals(parsed.original.objects[0].id, "\0\0\0\0");
    assertEquals(parsed.original.objects[0].sets[0].mods[0].value, { type: "int", value: 2147483647 });
    assertEquals(parsed.custom.stop, bytes.length);
    assertEquals(parsed.custom.objects[0].stop, bytes.length);
    // The first custom modification follows base, id, set count, set flag and modification count.
    const first = parsed.custom.objects[0].sets[0].mods[0];
    assertEquals(first.start, parsed.custom.start + 20);
    assertEquals(first.stop - first.start, leveled ? 24 : 16);
  }
  const leveled = buildModFile({
    version: 3,
    custom: [{
      base: "AHbz",
      id: "A001",
      mods: [{ field: "Hbz1", level: 3, column: 1, value: { type: "unreal", value: 2.5 } }],
    }],
  }, "leveled");
  const mod = readModFile(leveled, "leveled", "war3map.w3a").custom.objects[0].sets[0].mods[0];
  assertEquals([mod.field, mod.level, mod.column, mod.value], ["Hbz1", 3, 1, { type: "unreal", value: 2.5 }]);
});

Deno.test("synthetic v1 and v2 files without set fields parse with one implicit set", () => {
  for (const version of [1, 2]) {
    for (const kind of ["simple", "leveled"] as const) {
      const bytes = buildModFile({
        version,
        original: [{ base: "hpea", id: "\0\0\0\0", mods: [{ field: "ugol", value: { type: "int", value: 90 } }] }],
        custom: [
          { base: "hfoo", id: "h001", mods: [{ field: "unam", level: 2, value: { type: "string", value: "A" } }] },
          { base: "hfoo", id: "h002", mods: [] },
        ],
      }, kind);
      const parsed = readModFile(bytes, kind, "war3map.w3u");
      assertEquals(parsed.version, version);
      assertEquals(parsed.original.objects[0].sets, [{
        flag: 0,
        mods: [{
          field: "ugol",
          level: 0,
          column: 0,
          value: { type: "int", value: 90 },
          end: "\0\0\0\0",
          start: 20,
          stop: kind === "leveled" ? 44 : 36,
        }],
      }]);
      const objects = parsed.custom.objects;
      assertEquals(objects.map((o) => [o.id, o.sets.length, o.sets[0].flag]), [["h001", 1, 0], ["h002", 1, 0]]);
      assertEquals(objects[0].sets[0].mods[0].level, kind === "leveled" ? 2 : 0);
      assertEquals(objects[1].sets[0].mods, []);
      assertEquals(objects[1].stop, bytes.length);
    }
  }
});

Deno.test("malformed files are file errors that point to World Editor 3.00", async () => {
  const valid = await namesFixtureBytes("war3mapSkin.w3u");
  const withVersion = (version: number) => new Uint8Array([...int32(version), ...valid.subarray(4)]);
  expectFileError(withVersion(0), "unsupported version 0");
  expectFileError(withVersion(4), "unsupported version 4");
  expectFileError(valid.subarray(0, 2), "truncated");
  expectFileError(valid.subarray(0, 6), "truncated");
  for (let length = 9; length < valid.length; length++) expectFileError(valid.subarray(0, length), "");
  expectFileError(new Uint8Array([...valid.subarray(0, 8), ...int32(1000), ...valid.subarray(12)]), "count");
  expectFileError(new Uint8Array([...valid.subarray(0, 8), ...int32(-1), ...valid.subarray(12)]), "count");
  expectFileError(new Uint8Array([...valid.subarray(0, 28), ...int32(0x7fffffff), ...valid.subarray(32)]), "count");
  const withVarType = new Uint8Array(valid);
  withVarType.set(int32(4), 36);
  expectFileError(withVarType, "unknown value type 4");
  // Replaces the string's NUL and the end token with text, so no NUL remains after the string starts.
  expectFileError(
    new Uint8Array([...valid.subarray(0, 51), ...new TextEncoder().encode("xxxxx")]),
    "unterminated string",
  );
  const invalidUtf8 = new Uint8Array(valid);
  invalidUtf8[40] = 0xff;
  expectFileError(invalidUtf8, "invalid UTF-8");
  expectFileError(new Uint8Array([...valid, 0]), "trailing bytes");
  const withSets = (count: number) =>
    new Uint8Array([...valid.subarray(0, 20), ...int32(count), ...valid.subarray(24)]);
  expectFileError(withSets(0), "set count 0");
  expectFileError(withSets(1000), "set count 1000");
  expectFileError(withSets(-1), "set count -1");
});

Deno.test("a view with a nonzero byte offset parses the same as a copy", async () => {
  for (const file of ["war3mapSkin.w3q", "war3map.w3d", "war3mapSkin.w3u"]) {
    const bytes = await namesFixtureBytes(file);
    const padded = new Uint8Array(bytes.length + 7);
    padded.set(bytes, 3);
    const view = padded.subarray(3, 3 + bytes.length);
    assertEquals(readModFile(view, tableKind(file), file), readModFile(bytes, tableKind(file), file));
  }
});

const APPENDED: NewObject[] = [
  {
    base: "hfoo",
    id: "X001",
    mods: [
      { field: "uhpm", level: 0, column: 0, value: { type: "int", value: -250 } },
      { field: "umvs", level: 2, column: 1, value: { type: "real", value: 0.1 } },
      { field: "ucbs", level: 0, column: 3, value: { type: "unreal", value: 1.3 } },
      { field: "unam", level: 1, column: 0, value: { type: "string", value: "Møønwell" } },
    ],
  },
  { base: "hpea", id: "X002", mods: [] },
];

// APPENDED as the table kind allows it: simple tables have no level or column.
const appendedFor = (kind: TableKind): NewObject[] =>
  APPENDED.map(({ base, id, mods }) => ({
    base,
    id,
    mods: kind === "leveled" ? mods : mods.map((mod) => ({ ...mod, level: 0, column: 0 })),
  }));

// What the reader should return for APPENDED, given the table kind.
const appendedAsRead = (kind: TableKind) =>
  appendedFor(kind).map((object) => ({
    base: object.base,
    id: object.id,
    sets: [{
      flag: 0,
      mods: object.mods.map(({ field, level, column, value }) => ({
        field,
        level,
        column,
        value: value.type === "string" ? value : { type: value.type, value: Math.fround(value.value) },
        end: "\0\0\0\0",
      })),
    }],
  }));

const withoutOffsets = (objects: ObjectEntry[]) =>
  objects.map(({ base, id, sets }) => ({
    base,
    id,
    sets: sets.map(({ flag, mods }) => ({ flag, mods: mods.map(({ start: _, stop: __, ...mod }) => mod) })),
  }));

Deno.test("appending each names fixture object to no file reproduces World Editor's main and skin files", async () => {
  for (const name of NAMES) {
    for (const skin of [false, true]) {
      const file = `${skin ? "war3mapSkin" : "war3map"}.${name.ext}`;
      const mods = skin === name.skin
        ? [{ field: name.field, level: name.level, column: 0, value: { type: "string" as const, value: name.value } }]
        : [];
      const bytes = appendObjects(undefined, tableKind(file), [{ base: name.base, id: name.id, mods }], file);
      assertEquals(bytes, await namesFixtureBytes(file), file);
    }
  }
});

Deno.test("appending to every names fixture file keeps its bytes and adds the objects last", async () => {
  for (const name of NAMES) {
    for (const skin of ["war3map", "war3mapSkin"]) {
      const file = `${skin}.${name.ext}`;
      const kind = tableKind(file);
      const source = await namesFixtureBytes(file);
      const before = readModFile(source, kind, file);
      const bytes = appendObjects(source, kind, appendedFor(kind), file);
      const { countOffset, start, stop } = before.custom;
      assertEquals(bytes.subarray(0, countOffset), source.subarray(0, countOffset), file);
      assertEquals(bytes.subarray(countOffset, start), int32(before.custom.objects.length + 2), file);
      assertEquals(bytes.subarray(start, stop), source.subarray(start, stop), file);
      const after = readModFile(bytes, kind, file);
      assertEquals(after.version, 3, file);
      assertEquals(after.original, before.original, file);
      assertEquals(after.custom.objects.slice(0, -2), before.custom.objects, file);
      assertEquals(withoutOffsets(after.custom.objects.slice(-2)), appendedAsRead(kind), file);
      assertEquals(after.custom.stop, bytes.length, file);
    }
  }
});

Deno.test("appending no objects returns the source bytes", async () => {
  for (const file of ["war3map.w3u", "war3mapSkin.w3q", "war3map.w3d"]) {
    const source = await namesFixtureBytes(file);
    assertEquals(appendObjects(source, tableKind(file), [], file), source, file);
  }
});

Deno.test("synthetic v1, v2 and v3 sources get objects in their own version's shape", () => {
  const original: SyntheticObject[] = [{
    base: "hpea",
    id: "\0\0\0\0",
    mods: [{ field: "ugol", value: { type: "int", value: 90 } }],
  }];
  const existing: SyntheticObject[] = [
    { base: "hfoo", id: "h001", mods: [{ field: "unam", level: 2, value: { type: "string", value: "A" } }] },
    { base: "hfoo", id: "h002", mods: [{ field: "utip", value: { type: "string", value: "B" }, end: "h002" }] },
  ];
  // A v3 object with two sets and a nonzero flag, which Moonwell never writes, must still be copied verbatim.
  const multiSet: SyntheticObject = {
    base: "hfoo",
    id: "h003",
    sets: [{ flag: 7, mods: [] }, { flag: 0, mods: [{ field: "uhpm", value: { type: "int", value: 5 } }] }],
  };
  for (const version of [1, 2, 3]) {
    for (const kind of ["simple", "leveled"] as const) {
      const custom = version >= 3 ? [...existing, multiSet] : existing;
      const source = buildModFile({ version, original, custom }, kind);
      const appended = appendedFor(kind);
      assertEquals(
        appendObjects(source, kind, appended, "war3map.w3u"),
        buildModFile({ version, original, custom: [...custom, ...appended] }, kind),
        `v${version} ${kind}`,
      );
    }
  }
});

Deno.test("appending to a malformed source is the reader's file error", async () => {
  const valid = await namesFixtureBytes("war3mapSkin.w3u");
  const error = assertThrows(
    () => appendObjects(valid.subarray(0, 20), "simple", appendedFor("simple"), "map/war3mapSkin.w3u"),
    MoonwellError,
  );
  assertEquals(error.file, "map/war3mapSkin.w3u");
});

Deno.test("values the resolver should have rejected are internal errors", () => {
  const append = (object: Partial<NewObject>, value?: ModValue) => () =>
    appendObjects(undefined, "leveled", [{
      base: "hfoo",
      id: "X001",
      mods: value ? [{ field: "unam", level: 0, column: 0, value }] : [],
      ...object,
    }], "war3map.w3a");
  const expectInternal = (fn: () => unknown, problem: string) => {
    const error = assertThrows(fn, Error, problem);
    assertEquals(error instanceof MoonwellError, false, problem);
  };
  expectInternal(append({}, { type: "string", value: "a\0b" }), "NUL");
  expectInternal(append({}, { type: "string", value: "a\uD800b" }), "surrogate");
  for (const value of [NaN, Infinity, -Infinity, 3.5e38, -1e39]) {
    expectInternal(append({}, { type: "real", value }), "float32");
    expectInternal(append({}, { type: "unreal", value }), "float32");
  }
  for (const value of [2147483648, -2147483649, 1.5, NaN]) {
    expectInternal(append({}, { type: "int", value }), "int32");
  }
  expectInternal(
    append({ mods: [{ field: "unam", level: 2 ** 31, column: 0, value: { type: "int", value: 1 } }] }),
    "int32",
  );
  for (const id of ["X01", "X0001", "X00€", ""]) {
    expectInternal(append({ id }), "object id");
    expectInternal(append({ base: id }), "object id");
    expectInternal(
      append({ mods: [{ field: id, level: 0, column: 0, value: { type: "int", value: 1 } }] }),
      "object id",
    );
  }
  for (const [level, column] of [[1, 0], [0, 1]]) {
    const mods = [{ field: "unam", level, column, value: { type: "int" as const, value: 1 } }];
    expectInternal(
      () => appendObjects(undefined, "simple", [{ base: "hfoo", id: "X001", mods }], "war3map.w3u"),
      "simple table",
    );
  }
  // The largest finite float32 and Latin-1 ids are accepted.
  append({ id: "\0\xff\xe9A" }, { type: "real", value: -3.4028234663852886e38 })();
});
