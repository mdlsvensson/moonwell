import { assert, assertEquals } from "@std/assert";
import { join } from "@std/path";
import { patchSettingsLua } from "../../src/settings/lua.ts";
import { validateMapSettings } from "../../src/settings/options.ts";
import { runProcess } from "../../src/shared/process.ts";
import { patchMapInfo } from "../../src/w3i/patch.ts";
import { fixtureBytes, fixtureLua } from "../support/map-settings.ts";
import { testYue } from "../support/yue.ts";

async function runLua(script: string): Promise<string> {
  const dir = await Deno.makeTempDir();
  try {
    const file = join(dir, "settings.lua");
    await Deno.writeTextFile(file, script);
    const result = await runProcess(await testYue(), ["-e", file], { cwd: dir });
    assertEquals(result.code, 0, `${result.stdout}\n${result.stderr}`);
    return result.stdout.replace(/\r\n/g, "\n");
  } finally {
    await Deno.remove(dir, { recursive: true });
  }
}

async function patched(input: unknown, source?: string): Promise<string> {
  const settings = validateMapSettings(input);
  return patchSettingsLua(source ?? await fixtureLua(), settings, patchMapInfo(await fixtureBytes(), settings));
}

Deno.test("an old fog reset cannot undo a new fog override, and escapes stay unambiguous", async () => {
  const source = `
function config() SetMapName("old") end
function main() SetTerrainFogEx(0, 1, 2, 0.5, 1, 1, 1) ResetTerrainFog() InitBlizzard() end
`;
  const lua = await patched({
    info: { name: "\n123" },
    environment: { fog: { enabled: true, start: 100, end: 1000 } },
  }, source);
  const stdout = await runLua(`
local events = {}
function SetMapName(value) assert(value == "\\010123") end
function SetTerrainFogEx(style, first, last) assert(first == 100 and last == 1000) events[#events+1] = "fog" end
function ResetTerrainFog() error("old fog reset survived") end
function InitBlizzard() events[#events+1] = "init" end
${lua}
config()
main()
assert(table.concat(events, ",") == "fog,init")
io.write("settings-ok")
`);
  assertEquals(stdout, "settings-ok");
});

Deno.test("a removed call keeps the statements around it separate", async () => {
  const lua = await patched(
    { environment: { fog: { enabled: false } } },
    "function main()\nx = b\nResetTerrainFog();(f)()\nInitBlizzard()\nend",
  );
  const stdout = await runLua(`
local events = {}
b = function() error("statements were joined") end
f = function() events[#events+1] = "f" end
function ResetTerrainFog() events[#events+1] = "reset" end
function InitBlizzard() events[#events+1] = "init" end
${lua}
main()
io.write(table.concat(events, ","))
`);
  assertEquals(stdout, "f,reset,init");
});

// Natives the fixture calls, plus the ones settings edits may add. Anything else is an undefined global and fails.
const NATIVES = [
  "BlzCreateUnitWithSkin",
  "ConditionalTriggerExecute",
  "ConvertPlayerColor",
  "CreateTrigger",
  "DefineStartLocation",
  "ForcePlayerStartLocation",
  "FourCC",
  "GetCameraMargin",
  "InitBlizzard",
  "NewSoundEnvironment",
  "SelectUnitForPlayerSingle",
  "SetAmbientDaySound",
  "SetAmbientNightSound",
  "SetCameraBounds",
  "SetDayNightModels",
  "SetGamePlacement",
  "SetHDWaterParamsEx",
  "SetMapDescription",
  "SetMapMusic",
  "SetMapName",
  "SetPlayerAllianceStateAllyBJ",
  "SetPlayerAllianceStateVisionBJ",
  "SetPlayerColor",
  "SetPlayerController",
  "SetPlayerRacePreference",
  "SetPlayerRaceSelectable",
  "SetPlayerRaceSkin",
  "SetPlayerStartLocation",
  "SetPlayerTeam",
  "SetPlayers",
  "SetStartLocPrio",
  "SetStartLocPrioCount",
  "SetTeams",
  "TriggerAddAction",
  // Introduced only by settings edits.
  "ResetTerrainFog",
  "SetPlayerAllianceStateControlBJ",
  "SetPlayerAllianceStateFullControlBJ",
  "SetPlayerName",
  "SetPlayerState",
  "SetTerrainFogEx",
  "SetWaterBaseColor",
];

/** Runs the real config() and main() with recording stubs; returns one "Name(arg,...)" line per native call. */
async function execute(lua: string): Promise<string[]> {
  const constants = new Set((await fixtureLua()).match(/\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b/g));
  // Written by the edits below but absent from the fixture.
  for (const name of ["PLAYER_STATE_ALLIED_VICTORY", "MAP_CONTROL_RESCUABLE"]) constants.add(name);
  const stdout = await runLua(`
local calls = {}
local function record(name, ...)
  local parts = {}
  for i = 1, select("#", ...) do
    local value = select(i, ...)
    parts[#parts + 1] = type(value) == "number" and string.format("%.17g", value) or tostring(value)
  end
  calls[#calls + 1] = name .. "(" .. table.concat(parts, ",") .. ")"
end
for _, name in ipairs({${[...constants].map((name) => `"${name}"`).join(", ")}}) do _G[name] = name end
for _, name in ipairs({${NATIVES.map((name) => `"${name}"`).join(", ")}}) do
  _G[name] = function(...) record(name, ...) return 0 end
end
function Player(id) return id end
local stderr, exit = io.stderr, os.exit
setmetatable(_G, { __index = function(_, name) stderr:write("undefined global " .. tostring(name)) exit(1) end })
${lua}
config()
main()
io.write(table.concat(calls, "\\n"))
`);
  return stdout.split("\n");
}

Deno.test("the unpatched fixture runs under the stubs", async () => {
  const source = await fixtureLua();
  assertEquals(await patched({}), source);
  const calls = await execute(source);
  assert(calls.includes("SetPlayerController(11,MAP_CONTROL_COMPUTER)"));
  assert(calls.includes("ForcePlayerStartLocation(0,0)"));
  assertEquals(calls.filter((call) => call.startsWith("SetPlayerState(")), []);
});

Deno.test("patched fixture initialization applies overrides before unit creation", async () => {
  const calls = await execute(
    await patched({
      info: { name: "Moonwell", description: "" },
      players: {
        "0": { name: "", controller: "computer", race: "selectable", fixedStart: false, x: 0, y: 0.1 },
        "11": { controller: "rescuable", race: "undead" },
      },
      forces: { "0": { allied: false, alliedVictory: true, sharedVision: false, sharedControl: true } },
      environment: {
        soundEnvironment: "",
        waterColor: [0, 0, 0, 0],
        fog: { enabled: true, start: 0, color: [255, 0, 0, 0] },
      },
    }),
  );
  const expected = [
    "SetMapName(Moonwell)",
    "SetMapDescription()",
    "DefineStartLocation(0,0,0.10000000149011612)",
    "SetPlayerRacePreference(0,RACE_PREF_USER_SELECTABLE)",
    "SetPlayerRaceSelectable(0,true)",
    "SetPlayerController(0,MAP_CONTROL_COMPUTER)",
    "SetPlayerRacePreference(11,RACE_PREF_UNDEAD)",
    "SetPlayerController(11,MAP_CONTROL_RESCUABLE)",
    "SetPlayerState(3,PLAYER_STATE_ALLIED_VICTORY,1)",
    "SetPlayerAllianceStateAllyBJ(0,1,false)",
    "SetPlayerAllianceStateVisionBJ(3,2,false)",
    "SetPlayerAllianceStateControlBJ(1,0,true)",
    "SetPlayerAllianceStateFullControlBJ(2,3,false)",
    "SetPlayerTeam(11,1)",
    "NewSoundEnvironment(Default)",
    "SetWaterBaseColor(0,0,0,0)",
    "SetTerrainFogEx(0,0,5000,0.5,1,0,0)",
  ];
  for (const call of expected) assert(calls.includes(call), `missing ${call}`);
  assertEquals(calls.filter((call) => call.startsWith("ForcePlayerStartLocation(0,")), []);
  assertEquals(calls.filter((call) => call.startsWith("NewSoundEnvironment(")).length, 1);
  // Appended team states come after the editor's own alliance calls, so they take effect.
  assert(
    calls.lastIndexOf("SetPlayerAllianceStateAllyBJ(0,1,true)") <
      calls.indexOf("SetPlayerAllianceStateAllyBJ(0,1,false)"),
  );
  const firstUnit = calls.findIndex((call) => call.startsWith("BlzCreateUnitWithSkin("));
  for (const native of ["NewSoundEnvironment(", "SetWaterBaseColor(", "SetTerrainFogEx("]) {
    const at = calls.findIndex((call) => call.startsWith(native));
    assert(at >= 0 && at < firstUnit, `${native} must precede unit creation`);
  }
  assert(firstUnit < calls.indexOf("InitBlizzard()"));
});

Deno.test("disabled fog executes a reset before unit creation", async () => {
  const calls = await execute(await patched({ environment: { fog: { enabled: false, density: 0 } } }));
  const reset = calls.indexOf("ResetTerrainFog()");
  assert(reset >= 0 && reset < calls.findIndex((call) => call.startsWith("BlzCreateUnitWithSkin(")));
  assertEquals(calls.filter((call) => call.startsWith("SetTerrainFogEx(")), []);
});
