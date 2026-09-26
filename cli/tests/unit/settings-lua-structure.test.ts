import { assertEquals, assertThrows } from "@std/assert";
import { literalNumber, playerId, readLuaFunctions } from "../../src/settings/lua-structure.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

Deno.test("only direct standalone calls belong to an editor function", () => {
  const source = `--[=[ function config() Fake() end ]=]
function config()
  local text = [==[ end SetMapName("wrong") ]==]
  local ignored = SetMapName("expression")
  local callback = function() SetMapName("nested") end
  if true then SetMapName("conditional") elseif Test() then Other() else Other() end
  for i = 1, 2 do Other() end
  for k, v in pairs({}) do Other() end
  while false do Other() end
  do Other() end
  repeat Other() until Predicate() and object:Check()
  object:SetMapName("method")
  object.SetMapName("member")
  Factory()() Factory().member() (SetMapName)("parenthesized")
  SetMapName(
    "right" -- comment between arguments and closing parenthesis
  );
  SetPlayerController(Player(0), MAP_CONTROL_USER)
end`;
  const [fn] = readLuaFunctions(source);
  assertEquals(fn.name, "config");
  assertEquals(fn.calls.map((call) => call.name), ["SetMapName", "SetPlayerController"]);
  assertEquals(playerId(fn.calls[1].args[0]), 0);
  assertEquals(source.slice(fn.endStart, fn.end), "end");
});

Deno.test("structural ambiguity fails with a file error and re-save hint", () => {
  for (
    const source of [
      "function config()",
      'function config() X("unterminated) end',
      "function config() X([=[unterminated) end",
      "--[=[ no close",
      "function config() X({) end",
      "function config() if true then X() end",
      "function config() repeat X() end",
      "function config() X(1, ) end",
      "function config() local x = end",
      "function config() object.() end",
      "function config() 1 end",
      "function config() X() + Y() end",
      "function config() X(0x) end",
      "function config() X(1e+) end",
      "function config() X('line\nbreak') end",
      "function config() return X() Y() end",
      "end",
      "until X()",
    ]
  ) {
    const error = assertThrows(() => readLuaFunctions(source, "map.lua"), MoonwellError, undefined, source);
    assertEquals(error.file, "map.lua");
    assertEquals(error.hint?.includes("World Editor"), true);
  }
});

Deno.test("expressions consume tables, anonymous functions, operators and until conditions", () => {
  const source = `function config(...)
local a, b = { [Key()] = function(x, ...) Hidden() return x end; callback = function() Hidden() end, Hidden() }, ...
a[Key()], b.member = 1..2, 0x1.fp+2
local c = not #a + -2^2 // 3 % 4 * 5 / 6 - 7 << 2 >> 1 & 3 ~ 4 | 5 < 6 and true or nil
repeat Hidden() until (function() Hidden() return true end)() or Check {callback = function() Hidden() end}
::label:: goto label
while true do break end
Visible {nested = function() Hidden() end}; Visible "text"
return Hidden(), function() Hidden() end
end`;
  const [fn] = readLuaFunctions(source);
  assertEquals(fn.calls.map((call) => call.name), ["Visible", "Visible"]);
  assertEquals(fn.calls[0].args.length, 1);
  assertEquals(fn.calls[1].args[0][0].value, '"text"');
});

Deno.test("strings and comments at token boundaries cannot introduce calls", () => {
  const source = String.raw`function -- comment
config--[[]]
() local text = "\" end Fake() \\" local other = 'it\'s Fake()'
local escaped = "one\
two\z
  three"
SetPlayerController--[=[ gap ]=] (-- gap
Player -- gap
(-- gap
0-- gap
)-- gap
, MAP_CONTROL_USER-- gap
) -- trailing
end`;
  const [fn] = readLuaFunctions(source);
  assertEquals(fn.calls.map((call) => call.name), ["SetPlayerController"]);
  assertEquals(playerId(fn.calls[0].args[0]), 0);
  for (const token of fn.calls[0].args.flat()) {
    assertEquals(source.slice(token.start, token.end), token.value);
  }
});

Deno.test("ranges exclude trailing comments and cover multiline and adjacent statements", () => {
  const source = "function config()\nA();B() -- keep\nC(\n1,\n2\n); -- keep\nD() -- keep\n; end";
  const [fn] = readLuaFunctions(source);
  assertEquals(source.slice(fn.start, fn.end), source);
  assertEquals(fn.calls.map((call) => source.slice(call.start, call.end)), ["A();", "B()", "C(\n1,\n2\n);", "D()"]);
  assertEquals(source.slice(fn.endStart, fn.end), "end");
});

Deno.test("only top-level bare global declarations are exported, including duplicates", () => {
  const source = `local function config() Fake() end
function t.config() Fake() end
function t:config() Fake() end
config = function() Fake() end
do function config() Fake() end end
function config() function nested() Fake() end Real() end
function config() Other() end`;
  const functions = readLuaFunctions(source);
  assertEquals(functions.map((fn) => fn.name), ["config", "config"]);
  assertEquals(functions.map((fn) => fn.calls.map((call) => call.name)), [["Real"], ["Other"]]);
  assertEquals(
    source.slice(functions[0].start, functions[0].end),
    "function config() function nested() Fake() end Real() end",
  );
});

Deno.test("literal helpers accept only finite literal numeric shapes", () => {
  const cases: [string, number | undefined][] = [
    ["0", 0],
    ["0xF", 15],
    ["-0xF", -15],
    ["-.5", -.5],
    ["1.", 1],
    ["2e-3", .002],
    ["0x1.fp2", undefined],
    ["1e999", undefined],
    ["+1", undefined],
    ["1+2", undefined],
    ["(1)", undefined],
    ['"1"', undefined],
  ];
  for (const [value, want] of cases) {
    // Unary plus is not Lua syntax, so exercise the helper with an explicit token shape.
    if (value === "+1") {
      assertEquals(
        literalNumber([{ kind: "symbol", value: "+", start: 0, end: 1 }, {
          kind: "number",
          value: "1",
          start: 1,
          end: 2,
        }]),
        want,
      );
    } else {
      const [fn] = readLuaFunctions(`function test() Capture(${value}) end`);
      assertEquals(literalNumber(fn.calls[0].args[0]), want, value);
    }
  }
  for (
    const [value, want] of [
      ["Player(0)", 0],
      ["Player(-1)", -1],
      ["Player(0x17)", 23],
      ["Player(1.5)", undefined],
      ["Player(1+2)", undefined],
      ["Player(0,1)", undefined],
      ["object.Player(0)", undefined],
      ["Player(0).id", undefined],
      ["(Player(0))", undefined],
    ] as const
  ) {
    const [fn] = readLuaFunctions(`function test() Capture(${value}) end`);
    assertEquals(playerId(fn.calls[0].args[0]), want, value);
  }
});

Deno.test("real World Editor Lua exposes the expected settings functions and calls", async () => {
  const source = await Deno.readTextFile(new URL("../fixtures/map-settings-v39/war3map.lua", import.meta.url));
  const functions = readLuaFunctions(source);
  for (const name of ["config", "main", "InitCustomPlayerSlots", "InitCustomTeams"]) {
    assertEquals(functions.filter((fn) => fn.name === name).length, 1);
  }
  const config = functions.find((fn) => fn.name === "config")!;
  assertEquals(config.calls.map((call) => call.name), [
    "SetMapName",
    "SetMapDescription",
    "SetPlayers",
    "SetTeams",
    "SetGamePlacement",
    "DefineStartLocation",
    "DefineStartLocation",
    "DefineStartLocation",
    "DefineStartLocation",
    "DefineStartLocation",
    "InitCustomPlayerSlots",
    "InitCustomTeams",
    "InitAllyPriorities",
  ]);
  const players = functions.find((fn) => fn.name === "InitCustomPlayerSlots")!;
  assertEquals(
    players.calls.filter((call) => call.name === "SetPlayerController").map((call) => playerId(call.args[0])),
    [0, 1, 2, 3, 11],
  );
  assertEquals(
    functions.find((fn) => fn.name === "InitCustomTeams")!.calls.filter((call) => call.name === "SetPlayerTeam").length,
    5,
  );
  assertEquals(functions.find((fn) => fn.name === "main")!.calls.map((call) => call.name), [
    "SetCameraBounds",
    "SetDayNightModels",
    "SetHDWaterParamsEx",
    "NewSoundEnvironment",
    "SetAmbientDaySound",
    "SetAmbientNightSound",
    "SetMapMusic",
    "CreateAllUnits",
    "InitBlizzard",
    "InitGlobals",
    "InitCustomTriggers",
    "RunInitializationTriggers",
  ]);
});

Deno.test("deeply nested input fails safely instead of overflowing the JavaScript stack", () => {
  for (
    const source of [
      `function config() Capture(${"(".repeat(20000)}1${")".repeat(20000)}) end`,
      `${"do ".repeat(20000)}${"end ".repeat(20000)}`,
    ]
  ) {
    const error = assertThrows(() => readLuaFunctions(source, "map.lua"), MoonwellError);
    assertEquals(error.file, "map.lua");
  }
});

Deno.test("token ranges preserve numeral, operator and string spellings", () => {
  const source =
    '-- 🌙\r\nfunction config() Capture(1..2, 0X.8p-2, .5E+2, a//b, a<<b, a>>b, a~=b, a<=b, a>=b, a==b, [=[text]=], "one\\\r\ntwo"); end';
  const [fn] = readLuaFunctions(source);
  assertEquals(source.slice(fn.start, fn.start + 8), "function");
  assertEquals(fn.calls[0].args.map((tokens) => tokens.map((token) => token.value)), [
    ["1", "..", "2"],
    ["0X.8p-2"],
    [".5E+2"],
    ["a", "//", "b"],
    ["a", "<<", "b"],
    ["a", ">>", "b"],
    ["a", "~=", "b"],
    ["a", "<=", "b"],
    ["a", ">=", "b"],
    ["a", "==", "b"],
    ["[=[text]=]"],
    ['"one\\\r\ntwo"'],
  ]);
  for (const token of fn.calls[0].args.flat()) assertEquals(source.slice(token.start, token.end), token.value);
});

Deno.test("mismatched delimiters and invalid statement shapes are refused", () => {
  for (
    const body of [
      "local x = (1]",
      "local x = {[1] 2}",
      "local x = {1 2}",
      "local x = {end=1}",
      "A()[1",
      "A():method",
      "A() = 1",
      "a, A() = 1, 2",
      "(a) = 1",
      "local end = 1",
      "function t:config.extra() end",
      "local function t.x() end",
      "for x = 1 do end",
      "for x in do end",
      "for x = 1, 2, 3, 4 do end",
      "goto end",
      "::name:",
      "if x then else elseif y then end",
      "repeat until",
      "local x = @",
      "A(1.2.3)",
    ]
  ) assertThrows(() => readLuaFunctions(`function config() ${body} end`), MoonwellError, undefined, body);
});
