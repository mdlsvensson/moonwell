import { assertEquals, assertThrows } from "@std/assert";
import { parseJass } from "../../../tools/natives/jass.ts";
import { buildNatives } from "../../../tools/gen-natives.ts";

// Hand-written miniature JASS in the shape of common.j and blizzard.j; never copied from the game files.
const COMMON = `// a leading comment
type agent extends handle
type widget   extends agent  // trailing comment
type unit extends widget

globals
    constant integer MAX_THINGS = 24
    constant string SLASHES = "http://example"   // the // inside the string is not a comment
    integer array counts
endglobals

native CreateThing takes player id, integer unitid, real x, real y, real face returns unit
constant native GetThing takes nothing returns unit
native DoNothing takes code func returns nothing
`;

const BLIZZARD = `globals
    real bj_ANGLE = 0.0
endglobals

function HelperBJ takes unit whichUnit, boolean flag returns nothing
    local integer i = 0
    // not a declaration
    call DoNothing(null)
endfunction

constant function ConstantBJ takes nothing returns integer
    return 1
endfunction
`;

Deno.test("parseJass reads types, natives and globals from common.j", () => {
  const file = parseJass(COMMON, "common.j");
  assertEquals(file.types, [
    { name: "agent", extends: "handle" },
    { name: "widget", extends: "agent" },
    { name: "unit", extends: "widget" },
  ]);
  assertEquals(file.globals, [
    { name: "MAX_THINGS", source: "common.j", type: "integer", constant: true, array: false },
    { name: "SLASHES", source: "common.j", type: "string", constant: true, array: false },
    { name: "counts", source: "common.j", type: "integer", constant: false, array: true },
  ]);
  assertEquals(file.functions, [
    {
      name: "CreateThing",
      source: "common.j",
      constant: false,
      params: [
        { name: "id", type: "player" },
        { name: "unitid", type: "integer" },
        { name: "x", type: "real" },
        { name: "y", type: "real" },
        { name: "face", type: "real" },
      ],
      returns: "unit",
    },
    { name: "GetThing", source: "common.j", constant: true, params: [], returns: "unit" },
    {
      name: "DoNothing",
      source: "common.j",
      constant: false,
      params: [{ name: "func", type: "code" }],
      returns: "nothing",
    },
  ]);
});

Deno.test("parseJass reads blizzard.j function headers and skips their bodies", () => {
  const file = parseJass(BLIZZARD, "blizzard.j");
  assertEquals(file.globals, [{ name: "bj_ANGLE", source: "blizzard.j", type: "real", constant: false, array: false }]);
  assertEquals(file.functions.map((f) => [f.name, f.constant, f.params.length, f.returns]), [
    ["HelperBJ", false, 2, "nothing"],
    ["ConstantBJ", true, 0, "integer"],
  ]);
});

Deno.test("parseJass names the file and line of anything it does not understand", () => {
  assertThrows(() => parseJass("type unit extends widget\nlibrary Foo\n", "common.j"), Error, "common.j:2");
  assertThrows(() => parseJass("function F takes nothing returns nothing\n", "blizzard.j"), Error, "blizzard.j:1");
  assertThrows(() => parseJass("globals\n    what is this\nendglobals\n", "common.j"), Error, "common.j:2");
});

Deno.test("buildNatives merges both files and the Lua extras, sorted by name", () => {
  const natives = buildNatives(
    "9.9.9",
    parseJass(COMMON, "common.j"),
    parseJass(BLIZZARD, "blizzard.j"),
    {
      functions: [{ name: "FourCC", params: [{ name: "id", type: "string" }], returns: "integer" }],
      globals: ["print", "math"],
      removed: ["io"],
    },
  );
  assertEquals(natives.gameVersion, "9.9.9");
  assertEquals(natives.types.map((t) => t.name), ["agent", "unit", "widget"]);
  assertEquals(natives.functions.map((f) => `${f.source}:${f.name}`), [
    "blizzard.j:ConstantBJ",
    "common.j:CreateThing",
    "common.j:DoNothing",
    "lua:FourCC",
    "common.j:GetThing",
    "blizzard.j:HelperBJ",
  ]);
  assertEquals(natives.globals.map((g) => g.name), ["MAX_THINGS", "SLASHES", "bj_ANGLE", "counts"]);
  assertEquals(natives.lua, { globals: ["math", "print"], removed: ["io"] });
});

Deno.test("buildNatives refuses a name declared twice", () => {
  const common = parseJass(COMMON, "common.j");
  assertThrows(
    () => buildNatives("9.9.9", common, common, { functions: [], globals: [], removed: [] }),
    Error,
    "CreateThing",
  );
});
