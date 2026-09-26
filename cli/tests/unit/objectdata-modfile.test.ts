import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
import { type ModFile, readModFile, tableKind } from "../../src/objectdata/modfile.ts";
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
