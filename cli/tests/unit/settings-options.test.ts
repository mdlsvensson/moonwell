import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
import { hasExtendedSettings, hasSettings, validateMapSettings } from "../../src/settings/options.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

Deno.test("settings inherit by default and retain explicit false, zero and empty text", () => {
  assertEquals(
    hasSettings(validateMapSettings({
      info: { name: null },
      players: { "23": { name: null } },
      environment: { fog: {} },
    })),
    false,
  );
  const s = validateMapSettings({
    info: { name: "" },
    players: { "0": { fixedStart: false, x: 0 } },
    gameplay: { foodLimit: 0 },
  });
  assertEquals(s.info, { name: "" });
  assertEquals(s.players["0"], { fixedStart: false, x: 0 });
  assertEquals(s.gameplay.foodLimit, 0);
  assertEquals(hasExtendedSettings(s), true);
  assertEquals(hasSettings(s), true);
});

Deno.test("invalid settings identify the manifest and field", () => {
  const cases: unknown[] = [
    null,
    [],
    { typo: null },
    { info: { typo: null } },
    { info: { name: "bad\0text" } },
    { loadingScreen: { background: -2 } },
    { loadingScreen: { background: 1.5 } },
    { players: { "00": {} } },
    { players: { "24": {} } },
    { players: { "0": { race: "elf" } } },
    { players: { "0": { x: Infinity } } },
    { environment: { waterColor: [1, 2, 3] } },
    { environment: { fog: { density: 1.01 } } },
    { gameplay: { heroMaxLevel: 0 } },
    { gameplay: { foodLimit: 301 } },
    { gameplayConstants: { Misc: {}, misc: {} } },
    { gameInterface: { Frame: { X: "a", x: "b" } } },
    { gameplayConstants: { Misc: { X: "x\nY=z" } } },
  ];
  for (const input of cases) {
    const error = assertThrows(() => validateMapSettings(input, "moonwell.local.pkl"), MoonwellError);
    assertEquals(error.file, "moonwell.local.pkl");
    assertEquals(Boolean(error.hint), true);
  }
  const raw = validateMapSettings(JSON.parse('{"gameInterface":{"constructor":{"constructor":"ok"}}}'));
  assertEquals(raw.gameInterface["constructor"]["constructor"], "ok");
  assertThrows(() => validateMapSettings(JSON.parse('{"players":{"0":{"constructor":null}}}')), MoonwellError);
});

Deno.test("settings enforce numeric boundaries and accepted names", () => {
  const accepted: unknown[] = [
    { loadingScreen: { background: -1 } },
    { loadingScreen: { background: 2147483647 } },
    { gameplay: { heroMaxLevel: 1, foodLimit: 0 } },
    { gameplay: { heroMaxLevel: 10000, foodLimit: 300 } },
    { players: { "23": { x: -10000000, y: 10000000 } } },
    { environment: { fog: { style: 0, density: 0, start: -10000000, end: 10000000 } } },
    { environment: { fog: { style: 2, density: 1 }, waterColor: [0, 255, 0, 255] } },
  ];
  const rejected: unknown[] = [
    { loadingScreen: { background: 2147483648 } },
    { gameplay: { heroMaxLevel: 10001 } },
    { gameplay: { foodLimit: -1 } },
    { players: { "0": { x: -10000000.01 } } },
    { players: { "0": { y: 10000000.01 } } },
    { environment: { fog: { style: 3 } } },
    { environment: { fog: { density: -0.01 } } },
    { environment: { fog: { start: 10000000.01 } } },
    { environment: { waterColor: [1, 2, 3] } },
    { environment: { waterColor: [1, 2, 3, 4.5] } },
    { environment: { fog: { color: [0, 0, 0, 256] } } },
  ];
  for (const input of accepted) validateMapSettings(input);
  for (const input of rejected) assertThrows(() => validateMapSettings(input), MoonwellError);
  for (const controller of ["user", "computer", "neutral", "rescuable"]) {
    assertEquals(validateMapSettings({ players: { "0": { controller } } }).players["0"].controller, controller);
  }
  for (const race of ["selectable", "human", "orc", "undead", "nightelf"]) {
    assertEquals(validateMapSettings({ players: { "0": { race } } }).players["0"].race, race);
  }
  for (const key of ["allied", "alliedVictory", "sharedVision", "sharedControl", "sharedAdvancedControl"]) {
    assertEquals(
      (validateMapSettings({ forces: { "0": { [key]: false } } }).forces["0"] as Record<string, unknown>)[key],
      false,
    );
    assertThrows(() => validateMapSettings({ forces: { "0": { [key]: 0 } } }), MoonwellError);
  }
});

Deno.test("settings copy nested values and retain raw constructor keys", () => {
  const input = { environment: { waterColor: [1, 2, 3, 4] }, gameplayConstants: { Misc: { FoodCeiling: "0" } } };
  const result = validateMapSettings(input);
  input.environment.waterColor[0] = 10;
  input.gameplayConstants.Misc.FoodCeiling = "1";
  assertEquals(result.environment.waterColor, [1, 2, 3, 4]);
  assertEquals(result.gameplayConstants.Misc.FoodCeiling, "0");
});

Deno.test("settings validation errors name the settings path once", () => {
  const cases: [unknown, string][] = [
    [[], "settings must be an object."],
    [{ other: {} }, "Unknown map setting: settings.other"],
    [{ info: [] }, "settings.info must be an object."],
    [{ info: { title: "x" } }, "Unknown map setting: settings.info.title"],
    [{ info: { name: 1 } }, "Invalid map setting: settings.info.name"],
    [{ players: { "24": {} } }, "settings.players keys must be IDs from 0 to 23: 24"],
    [{ players: { "7": { x: "1" } } }, 'Invalid map setting: settings.players["7"].x'],
    [{ forces: { "0": { team: 1 } } }, 'Unknown map setting: settings.forces["0"].team'],
    [{ environment: { fog: { density: 2 } } }, "Invalid map setting: settings.environment.fog.density"],
    [{ gameplayConstants: { Misc: {}, misc: {} } }, "Invalid or duplicate settings.gameplayConstants section: misc"],
    [{ gameInterface: { Frame: { X: "a", x: "b" } } }, 'Invalid or duplicate settings.gameInterface["Frame"] key: x'],
    [{ gameplayConstants: { Misc: { X: "a\nb" } } }, 'settings.gameplayConstants["Misc"]["X"] must be a single-line'],
    [{ gameInterface: { Frame: [] } }, 'settings.gameInterface["Frame"] must be an object.'],
  ];
  for (const [input, message] of cases) {
    const error = assertThrows(() => validateMapSettings(input, "moonwell.local.pkl"), MoonwellError);
    assertStringIncludes(error.message, message);
    assertEquals(error.message.includes("settings.settings"), false, error.message);
  }
});

Deno.test("whole-number coordinates and fog values are accepted as numbers", () => {
  const settings = validateMapSettings({
    players: { "0": { x: 256, y: -896 } },
    environment: { fog: { start: 100, end: 1000, density: 1 } },
  });
  assertEquals(settings.players["0"], { x: 256, y: -896 });
  assertEquals(settings.environment.fog, { start: 100, end: 1000, density: 1 });
});
