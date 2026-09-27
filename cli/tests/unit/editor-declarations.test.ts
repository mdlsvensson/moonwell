import { assertEquals, assertStringIncludes } from "@std/assert";
import {
  luaType,
  renderNativesDeclarations,
  renderObjectDeclarations,
  RUNTIME_DECLARATIONS,
} from "../../src/editor/declarations.ts";
import type { Natives } from "../../src/natives/natives.ts";

const NATIVES: Natives = {
  gameVersion: "9.9.9",
  types: [
    { name: "agent", extends: "handle" },
    { name: "unit", extends: "widget" },
    { name: "widget", extends: "agent" },
  ],
  functions: [
    {
      name: "CreateThing",
      source: "common.j",
      constant: false,
      params: [{ name: "id", type: "player" }, { name: "end", type: "real" }, { name: "cb", type: "code" }],
      returns: "unit",
    },
    { name: "DoNothing", source: "blizzard.j", constant: false, params: [], returns: "nothing" },
    { name: "FourCC", source: "lua", constant: false, params: [{ name: "id", type: "string" }], returns: "integer" },
  ],
  globals: [
    { name: "MAX_THINGS", source: "common.j", type: "integer", constant: true, array: false },
    { name: "counts", source: "blizzard.j", type: "real", constant: false, array: true },
  ],
  lua: { globals: ["print"], removed: ["io"] },
};

Deno.test("JASS types map to Lua types; handle types keep their names", () => {
  assertEquals(
    ["integer", "real", "boolean", "string", "code", "handle", "unit", "any"].map(luaType),
    ["integer", "number", "boolean", "string", "function", "handle", "unit", "any"],
  );
});

Deno.test("natives.d.lua declares classes, functions and globals", () => {
  const text = renderNativesDeclarations(NATIVES);
  assertEquals(text.split("\n")[0], "---@meta");
  assertStringIncludes(text, "Warcraft III 9.9.9");
  assertStringIncludes(text, "---@class handle\n");
  assertStringIncludes(text, "---@class agent: handle\n");
  assertStringIncludes(text, "---@class unit: widget\n");
  assertStringIncludes(
    text,
    "---@param id player\n---@param end_ number\n---@param cb function\n---@return unit\n" +
      "function CreateThing(id, end_, cb) end\n",
  );
  assertStringIncludes(text, "function DoNothing() end\n");
  assertEquals(text.includes("---@return nothing"), false);
  assertStringIncludes(text, "---@param id string\n---@return integer\nfunction FourCC(id) end\n");
  assertStringIncludes(text, "---@type integer\nMAX_THINGS = nil\n");
  assertStringIncludes(text, "---@type number[]\ncounts = nil\n");
});

Deno.test("classes are declared parents first, so LuaLS never sees an unknown parent", () => {
  const text = renderNativesDeclarations(NATIVES);
  const order = ["handle", "agent", "widget", "unit"].map((name) => text.indexOf(`---@class ${name}`));
  assertEquals([...order].sort((a, b) => a - b), order);
});

Deno.test("moonwell.d.lua declares the runtime module's hooks", () => {
  assertEquals(RUNTIME_DECLARATIONS.split("\n")[0], "---@meta moonwell");
  for (const hook of ["before_config", "on_config", "before_main", "on_main"]) {
    assertStringIncludes(RUNTIME_DECLARATIONS, `---@param fn fun()\nfunction moonwell.${hook}(fn) end\n`);
  }
  assertStringIncludes(RUNTIME_DECLARATIONS, "function moonwell.format_error(message) end\n");
});

Deno.test("objects.d.lua declares every category, keys sorted, with rawcodes", () => {
  const text = renderObjectDeclarations([
    { category: "units", key: "captain", id: "h000" },
    { category: "units", key: "archer", id: "h001" },
    { category: "abilities", key: "holyLight", id: "A000" },
  ]);
  assertEquals(text.split("\n")[0], "---@meta generated.objects");
  assertEquals(text.split("\n").slice(0, 2), ["---@meta generated.objects", "---@diagnostic disable: missing-fields"]);
  assertStringIncludes(
    text,
    "---@class generated.objects.units\n---@field archer integer h001\n---@field captain integer h000\n",
  );
  assertStringIncludes(text, "---@class generated.objects.heroes\n\n");
  assertStringIncludes(text, "---@type generated.objects.abilities\nobjects.abilities = {}\n");
  assertStringIncludes(text, "return objects\n");
});
