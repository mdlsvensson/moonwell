import { assertEquals } from "@std/assert";
import { luaTopLevelGlobals } from "../../src/lint/lua-globals.ts";

Deno.test("luaTopLevelGlobals finds top-level global functions and assignments only", () => {
  const source = [
    "function OnInit(fn) end",
    "Timer = {}",
    "A, B = 1, 2",
    "local hidden = 1",
    "local function helper() end",
    "function Timer.start() end",
    "function Timer:stop() end",
    "Config.value = 3",
    "if ready then Inner = 1 end",
    "local t = {",
    "  Field = 1,",
    "}",
    "do Scoped = 2 end",
    "for i = 1, 3 do Looped = i end",
    "x = 1; Y = 2",
    'print("Z = 1") -- W = 2',
    "--[[ V = 3 ]]",
    "Last = function() Nested = 1 end",
    "if a == b then end",
    "Timer = nil",
  ].join("\n");
  assertEquals(luaTopLevelGlobals(source), ["OnInit", "Timer", "A", "B", "x", "Y", "Last"]);
});

Deno.test("luaTopLevelGlobals leaves out names declared local at the top level", () => {
  const source = [
    "local Timer",
    "Timer = {}",
    "local Name",
    "function Name() end",
    "local A, B = 1, 2",
    "A, B, C = 3, 4, 5",
    "local function helper() end",
    "helper = nil",
    "function f()",
    "  local Inner",
    "end",
    "Inner = 1",
    "local t = { local_like = 1 }",
    "Global = 1",
  ].join("\n");
  assertEquals(luaTopLevelGlobals(source), ["C", "f", "Inner", "Global"]);
});

Deno.test("luaTopLevelGlobals finds nothing in a module that returns a table", () => {
  assertEquals(luaTopLevelGlobals("local M = {}\nfunction M.greet() end\nreturn M\n"), []);
});
