import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
import { validateMapSettings } from "../../src/settings/options.ts";
import { luaString, patchSettingsLua } from "../../src/settings/lua.ts";
import { readLuaFunctions } from "../../src/settings/lua-structure.ts";
import { patchMapInfo } from "../../src/w3i/patch.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { fixtureBytes, fixtureLua } from "../support/map-settings.ts";

async function patch(input: unknown, source?: string): Promise<string> {
  const settings = validateMapSettings(input);
  return patchSettingsLua(source ?? await fixtureLua(), settings, patchMapInfo(await fixtureBytes(), settings));
}

async function refuses(input: unknown, source: string): Promise<MoonwellError> {
  const settings = validateMapSettings(input);
  const bytes = patchMapInfo(await fixtureBytes(), settings);
  const error = assertThrows(() => patchSettingsLua(source, settings, bytes, "map/war3map.lua"), MoonwellError);
  assertEquals(error.file, "map/war3map.lua");
  return error;
}

/** The whole InitCustomTeams body after the editor's last call. */
function teamsTail(lua: string): string {
  const start = lua.indexOf("SetPlayerTeam(Player(11), 1)\r\n") + "SetPlayerTeam(Player(11), 1)\r\n".length;
  return lua.slice(start, lua.indexOf("end", start));
}

Deno.test("player and environment edits agree with patched map info", async () => {
  const settings = validateMapSettings({
    info: { name: 'A "quoted" map\n雪' },
    players: { "0": { name: "Hero", controller: "computer", race: "orc", fixedStart: false, x: 256 } },
    environment: { waterColor: [10, 20, 30, 255], fog: { enabled: true, start: 100, end: 1000 } },
  });
  const lua = patchSettingsLua(await fixtureLua(), settings, patchMapInfo(await fixtureBytes(), settings));
  for (
    const line of [
      'SetMapName("A \\"quoted\\" map\\010雪")',
      "SetPlayerController(Player(0), MAP_CONTROL_COMPUTER)",
      "SetPlayerRacePreference(Player(0), RACE_PREF_ORC)",
      "DefineStartLocation(0, 256, -896)",
      "SetWaterBaseColor(10, 20, 30, 255)",
    ]
  ) assertStringIncludes(lua, line);
  assertEquals(lua.includes("ForcePlayerStartLocation(Player(0)"), false);
  assertStringIncludes(lua, "ForcePlayerStartLocation(Player(1), 1)");
  assertStringIncludes(lua, 'BlzCreateUnitWithSkin(p, FourCC("Hblm")');
  assertEquals(luaString("\n123"), '"\\010123"');
});

Deno.test("missing or duplicate editor calls refuse an edit", async () => {
  const source = await fixtureLua();
  const s = validateMapSettings({ info: { name: "Name" } });
  const bytes = patchMapInfo(await fixtureBytes(), s);
  assertThrows(() => patchSettingsLua(source.replace("SetMapName(", "Other("), s, bytes), MoonwellError);
  assertThrows(() => patchSettingsLua(source + '\nfunction config() SetMapName("x") end', s, bytes), MoonwellError);
  assertThrows(() => patchSettingsLua(source.replace("SetMapName(", "object.SetMapName("), s, bytes), MoonwellError);
});

Deno.test("settings without Lua counterparts return the source unchanged without reading it", async () => {
  const source = await fixtureLua();
  assertEquals(await patch({}, source), source);
  const metadataOnly = {
    info: { author: "Author", recommendedPlayers: "" },
    loadingScreen: { title: "Title" },
    forces: { "0": { name: "Allies" } },
    gameplay: { heroMaxLevel: 20 },
  };
  assertEquals(await patch(metadataOnly, source), source);
  assertEquals(await patch({}, "function (((unreadable"), "function (((unreadable");
});

Deno.test("a map name edit changes only that call", async () => {
  const source = await fixtureLua();
  assertEquals(
    await patch({ info: { name: "Name", description: "" } }, source),
    source.replace('SetMapName("TRIGSTR_001")', 'SetMapName("Name")')
      .replace('SetMapDescription("TRIGSTR_003")', 'SetMapDescription("")'),
  );
});

Deno.test("player edits replace, insert and remove only that player's calls", async () => {
  const source = await fixtureLua();
  assertEquals(
    await patch({ players: { "0": { name: "Hero", race: "selectable", fixedStart: false } } }, source),
    source
      .replace(
        "SetPlayerStartLocation(Player(0), 0)\r\nForcePlayerStartLocation(Player(0), 0)\r\n",
        'SetPlayerStartLocation(Player(0), 0)\r\nSetPlayerName(Player(0), "Hero")\r\n',
      )
      .replace(
        "SetPlayerRacePreference(Player(0), RACE_PREF_HUMAN)",
        "SetPlayerRacePreference(Player(0), RACE_PREF_USER_SELECTABLE)",
      )
      .replace("SetPlayerRaceSelectable(Player(0), false)", "SetPlayerRaceSelectable(Player(0), true)"),
  );
  const unforced = source.replace("ForcePlayerStartLocation(Player(1), 1)\r\n", "");
  assertEquals(await patch({ players: { "1": { fixedStart: true } } }, unforced), source);
  // Player 11 is the fifth record, so its start location is 4; an existing matching call is kept.
  assertEquals(await patch({ players: { "11": { fixedStart: true, controller: "computer" } } }, source), source);
  const named = source.replace(
    "SetPlayerColor(Player(1), ConvertPlayerColor(1))",
    'SetPlayerColor(Player(1), ConvertPlayerColor(1))\r\nSetPlayerName(Player(1), "TRIGSTR_006")',
  );
  assertEquals(
    await patch({ players: { "1": { name: "Tab\there ✓", controller: "rescuable" } } }, named),
    named.replace('SetPlayerName(Player(1), "TRIGSTR_006")', 'SetPlayerName(Player(1), "Tab\\009here ✓")')
      .replace(
        "SetPlayerController(Player(1), MAP_CONTROL_USER)",
        "SetPlayerController(Player(1), MAP_CONTROL_RESCUABLE)",
      ),
  );
});

Deno.test("start coordinates use effective float32 map-info values", async () => {
  const lua = await patch({ players: { "11": { x: 0.1 } } });
  assertStringIncludes(lua, "DefineStartLocation(4, 0.10000000149011612, -896)\r\n");
  assertStringIncludes(lua, "DefineStartLocation(0, 128.0, -896.0)\r\n");
  const fog = await patch({
    environment: { fog: { enabled: true, start: 0, density: 0.3, color: [255, 0, 51, 128] } },
  });
  assertStringIncludes(fog, "SetTerrainFogEx(0, 0, 5000, 0.30000001192092896, 1, 0, 0.2)\r\nCreateAllUnits()");
});

Deno.test("unsafe player, team and environment shapes are refused", async () => {
  const source = await fixtureLua();
  await refuses({ info: { name: "x" } }, source.replace('SetMapName("TRIGSTR_001")', 'SetMapName("TRIGSTR_001", 1)'));
  await refuses(
    { players: { "1": { controller: "computer" } } },
    source.replace("SetPlayerStartLocation(Player(1), 1)", "SetPlayerStartLocation(Player(1), 2)"),
  );
  await refuses(
    { players: { "1": { controller: "computer" } } },
    source.replace("SetPlayerStartLocation(Player(1), 1)\r\n", ""),
  );
  await refuses(
    { players: { "0": { controller: "computer" } } },
    source.replace(
      "SetPlayerController(Player(0), MAP_CONTROL_USER)",
      "SetPlayerController(Player(0 + 0), MAP_CONTROL_USER)",
    ),
  );
  await refuses(
    { players: { "0": { controller: "computer" } } },
    source.replace("SetPlayerController(Player(1), MAP_CONTROL_USER)", "SetPlayerController(p, MAP_CONTROL_USER)"),
  );
  await refuses({ players: { "0": { x: 1 } } }, source.replace("DefineStartLocation(0,", "DefineStartLocation(zero,"));
  await refuses({ players: { "0": { x: 1 } } }, source.replace("DefineStartLocation(1,", "DefineStartLocation(0,"));
  await refuses(
    { players: { "0": { fixedStart: false } } },
    source.replace(
      "ForcePlayerStartLocation(Player(0), 0)",
      "ForcePlayerStartLocation(Player(0), 0)\r\nForcePlayerStartLocation(Player(0), 0)",
    ),
  );
  await refuses(
    { players: { "0": { fixedStart: true } } },
    source.replace("ForcePlayerStartLocation(Player(0), 0)", "ForcePlayerStartLocation(Player(0), 1)"),
  );
  await refuses(
    { players: { "0": { name: "x" } } },
    source.replace(
      "SetPlayerColor(Player(0),",
      'SetPlayerName(Player(0), "a")\r\nSetPlayerName(Player(0), "b")\r\nSetPlayerColor(Player(0),',
    ),
  );
  await refuses(
    { players: { "0": { race: "orc" } } },
    source.replace("SetPlayerRaceSelectable(Player(0), false)\r\n", ""),
  );
  await refuses(
    { players: { "0": { name: "x" } } },
    source.replace("InitCustomPlayerSlots()\r\nInitCustomTeams()", "InitCustomTeams()"),
  );
  await refuses(
    { forces: { "0": { allied: true } } },
    source.replace("SetPlayerTeam(Player(3), 0)", "SetPlayerTeam(Player(3), 1)"),
  );
  await refuses(
    { forces: { "1": { allied: true } } },
    source.replace("SetPlayerTeam(Player(3), 0)", "SetPlayerTeam(Player(3), 1)"),
  );
  await refuses({ forces: { "0": { allied: true } } }, source.replace("SetPlayerTeam(Player(3), 0)\r\n", ""));
  await refuses(
    { forces: { "0": { allied: true } } },
    source.replace("SetPlayerTeam(Player(3), 0)", "SetPlayerTeam(Player(3), team)"),
  );
  await refuses(
    { forces: { "0": { allied: true } } },
    source.replace("InitCustomTeams()\r\nInitAllyPriorities()", "InitAllyPriorities()"),
  );
  await refuses(
    { environment: { waterColor: [1, 2, 3, 4] } },
    source.replace("CreateAllUnits()\r\nInitBlizzard()\r\n", ""),
  );
  await refuses(
    { environment: { waterColor: [1, 2, 3, 4] } },
    source.replace("CreateAllUnits()\r\nInitBlizzard()", "CreateAllUnits(1)\r\nInitBlizzard()"),
  );
  await refuses(
    { environment: { waterColor: [1, 2, 3, 4] } },
    source.replace(
      "CreateAllUnits()\r\nInitBlizzard()",
      "SetWaterBaseColor(1, 2, 3)\r\nCreateAllUnits()\r\nInitBlizzard()",
    ),
  );
  await refuses(
    { environment: { soundEnvironment: "Cave" } },
    source.replace(
      "CreateAllUnits()\r\nInitBlizzard()",
      'NewSoundEnvironment("Second")\r\nCreateAllUnits()\r\nInitBlizzard()',
    ),
  );
  await refuses(
    { environment: { fog: { enabled: false } } },
    source.replace("CreateAllUnits()\r\nInitBlizzard()", "ResetTerrainFog(1)\r\nCreateAllUnits()\r\nInitBlizzard()"),
  );
});

Deno.test("force flag edits append effective states after the editor's calls", async () => {
  const lua = await patch({
    forces: { "0": { allied: false, alliedVictory: true, sharedVision: false, sharedControl: true }, "1": {} },
  });
  const pairs = (native: string, value: boolean) =>
    [0, 1, 2, 3].flatMap((a) =>
      [0, 1, 2, 3].filter((b) => a !== b).map((b) => `${native}(Player(${a}), Player(${b}), ${value})\r\n`)
    );
  assertEquals(
    teamsTail(lua),
    [
      ...[0, 1, 2, 3].map((id) => `SetPlayerState(Player(${id}), PLAYER_STATE_ALLIED_VICTORY, 1)\r\n`),
      ...pairs("SetPlayerAllianceStateAllyBJ", false),
      ...pairs("SetPlayerAllianceStateVisionBJ", false),
      ...pairs("SetPlayerAllianceStateControlBJ", true),
      ...pairs("SetPlayerAllianceStateFullControlBJ", false),
    ].join(""),
  );
  assertStringIncludes(lua, "SetPlayerAllianceStateAllyBJ(Player(0), Player(1), true)\r\n");
  // Force 1 has one member: only its allied-victory state (inherited false) is written.
  assertEquals(
    teamsTail(await patch({ forces: { "1": { sharedVision: true } } })),
    "SetPlayerState(Player(11), PLAYER_STATE_ALLIED_VICTORY, 0)\r\n",
  );
});

Deno.test("environment edits replace old initialization immediately before the anchor", async () => {
  const source = await fixtureLua();
  const lua = await patch({ environment: { soundEnvironment: "", fog: { enabled: false } } }, source);
  assertEquals(
    lua,
    source.replace('NewSoundEnvironment("Default")\r\n', "").replace(
      "CreateAllUnits()\r\nInitBlizzard()",
      'NewSoundEnvironment("Default")\r\nResetTerrainFog()\r\nCreateAllUnits()\r\nInitBlizzard()',
    ),
  );
  const custom =
    'function main()\n  SetTerrainFogEx(0, 1, 2, 0.5, 1, 1, 1) ResetTerrainFog()\n  NewSoundEnvironment("Old")\n  InitBlizzard()\nend\n';
  assertEquals(
    await patch({ environment: { soundEnvironment: "Cave", fog: { enabled: true } } }, custom),
    'function main()\n  ; ;\n  NewSoundEnvironment("Cave")\n  SetTerrainFogEx(0, 3000, 5000, 0.5, 0, 0, 0)\n  InitBlizzard()\nend\n',
  );
});

Deno.test("removing a call never joins the statements around it", async () => {
  const fog = { environment: { fog: { enabled: false } } };
  for (
    const [source, expected] of [
      ["function main()\nx = b\nResetTerrainFog();(f)()\nInitBlizzard()\nend", "function main()\nx = b\n;(f)()\n"],
      ["function main()\nx = b\nResetTerrainFog();\n(f)()\nInitBlizzard()\nend", "function main()\nx = b\n;\n(f)()\n"],
      [
        "function main()\nx = b\nResetTerrainFog() -- c\ny()\nInitBlizzard()\nend",
        "function main()\nx = b\n; -- c\ny()\n",
      ],
    ]
  ) {
    const lua = await patch(fog, source);
    assertEquals(lua, `${expected}ResetTerrainFog()\nInitBlizzard()\nend`);
    readLuaFunctions(lua);
  }
});

Deno.test("Lua strings escape quotes, backslashes and control characters as decimal escapes", () => {
  assertEquals(luaString('Back\\slash "q" \t\x7f ✓ 雪 😀'), '"Back\\\\slash \\"q\\" \\009\\127 ✓ 雪 😀"');
  assertEquals(luaString("\r\n1\x001"), '"\\013\\0101\\0001"');
  assertEquals(luaString(""), '""');
});
