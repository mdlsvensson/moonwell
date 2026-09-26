import { assert, assertEquals, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { fromFileUrl, join } from "@std/path";
import { readImports } from "../../src/assets/imports.ts";
import { build } from "../../src/commands/build.ts";
import { test } from "../../src/commands/test.ts";
import { type CommandContext, createContext } from "../../src/context.ts";
import { readMapInfo } from "../../src/w3i/map-info.ts";
import { silentLogger } from "../support/logger.ts";
import { SETTINGS_FIXTURE } from "../support/map-settings.ts";
import { openMpq } from "../support/mpq-reader.ts";

const REPO = fromFileUrl(new URL("../../../", import.meta.url));
const MAIN = join(REPO, "cli", "src", "main.ts");
const SETTINGS_FILES = ["war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt"];
const decode = (bytes: Uint8Array | undefined) => new TextDecoder().decode(bytes);

/** The representative override from the plan: every settings group that touches the four internal files. */
const SETTINGS = `settings {
  info { name = "Moonwell settings test"; description = "Built settings" }
  players { ["0"] { controller = "computer"; race = "orc"; fixedStart = false; x = 256.0 } }
  forces { ["0"] { allied = false; sharedVision = false; alliedVictory = true } }
  environment {
    soundEnvironment = "Mountains"
    waterColor = List(10, 20, 30, 255)
    fog { enabled = true; start = 100.0; end = 1000.0 }
  }
  gameplay { heroMaxLevel = 25; foodLimit = 200 }
  gameInterface { ["CustomSkin"] { ["Test"] = "value" } }
}
`;

async function deno(args: string[], cwd: string) {
  const output = await new Deno.Command(Deno.execPath(), { args, cwd, stdout: "piped", stderr: "piped" }).output();
  return { code: output.code, text: decode(output.stdout) + decode(output.stderr) };
}

/** An `init --link` project whose source map is the copied World Editor v39 map-info/Lua pair. */
async function newProject(): Promise<string> {
  const project = join(await Deno.makeTempDir({ prefix: "moonwell-e2e-settings-" }), "my-map");
  const result = await deno(["run", "-A", MAIN, "init", "--link", project], REPO);
  assertEquals(result.code, 0, result.text);
  for (const name of ["war3map.w3i", "war3map.lua"]) {
    await Deno.copyFile(fromFileUrl(new URL(name, SETTINGS_FIXTURE)), join(project, "maps", "map.w3x", name));
  }
  return project;
}

const writeLocal = (project: string, body: string) =>
  Deno.writeTextFile(join(project, "moonwell.local.pkl"), `amends "moonwell.pkl"\n${body}`);

/** Every file under `dir` by relative path, for byte-for-byte before/after comparisons. */
async function snapshot(dir: string, prefix = ""): Promise<Record<string, Uint8Array>> {
  const files: Record<string, Uint8Array> = {};
  for await (const entry of Deno.readDir(dir)) {
    const path = join(dir, entry.name);
    if (entry.isDirectory) Object.assign(files, await snapshot(path, `${prefix}${entry.name}/`));
    else files[`${prefix}${entry.name}`] = await Deno.readFile(path);
  }
  return files;
}

const sourceMap = (project: string) => join(project, "maps", "map.w3x");
const archiveOf = (project: string) => join(project, "dist", "bin", "map.w3x");

function assertSettingsLua(lua: string) {
  const boot = lua.indexOf('__mw.boot("main")');
  assert(boot > 0, "the bundle was not injected");
  for (
    const call of [
      'SetMapName("Moonwell settings test")',
      'SetMapDescription("Built settings")',
      "SetPlayerController(Player(0), MAP_CONTROL_COMPUTER)",
      "SetPlayerRacePreference(Player(0), RACE_PREF_ORC)",
      'NewSoundEnvironment("Mountains")',
      "SetWaterBaseColor(10, 20, 30, 255)",
      "SetTerrainFogEx(",
    ]
  ) {
    const at = lua.indexOf(call);
    assert(at >= 0 && at < boot, `${call} missing or after the bundle`);
  }
}

Deno.test("build applies settings to the archive only, repeatably, and never to the source map", async () => {
  const project = await newProject();
  await writeLocal(project, SETTINGS);
  const before = await snapshot(sourceMap(project));

  const first = await deno(["task", "build"], project);
  assertEquals(first.code, 0, first.text);
  assertStringIncludes(first.text, "Applied map settings to 4 internal file(s).");
  const archive = openMpq(await Deno.readFile(archiveOf(project)));

  const info = readMapInfo((await archive.read("war3map.w3i"))!, true);
  assertEquals(info.info.name.value, "Moonwell settings test");
  assertEquals(info.info.description.value, "Built settings");
  const details = info.details!;
  const player = details.players.find((entry) => entry.id.value === 0)!;
  assertEquals([player.controller.value, player.race.value, player.fixedStart.value], [2, 2, 0]);
  assertEquals(player.x.value, 256);
  assertEquals(details.soundEnvironment.value, "Mountains");
  assertEquals(details.waterColor.map((channel) => channel.value), [10, 20, 30, 255]);
  assertEquals([details.fog.start.value, details.fog.end.value], [100, 1000]);

  assertSettingsLua(decode(await archive.read("war3map.lua")));
  const misc = decode(await archive.read("war3mapMisc.txt"));
  assertStringIncludes(misc, "HeroMaxLevel=25");
  assertStringIncludes(misc, "FoodCeiling=200");
  assertStringIncludes(decode(await archive.read("war3mapSkin.txt")), "[CustomSkin]\nTest=value");
  const imp = await archive.read("war3map.imp");
  const imported = imp === undefined ? [] : readImports(imp).map((entry) => entry.path.toLowerCase());
  for (const name of SETTINGS_FILES) assert(!imported.includes(name.toLowerCase()), `${name} imported as an asset`);

  const firstFiles = await Promise.all(SETTINGS_FILES.map((name) => archive.read(name)));
  const second = await deno(["task", "build"], project);
  assertEquals(second.code, 0, second.text);
  const rebuilt = openMpq(await Deno.readFile(archiveOf(project)));
  assertEquals(await Promise.all(SETTINGS_FILES.map((name) => rebuilt.read(name))), firstFiles);
  assertEquals(await snapshot(sourceMap(project)), before);
});

Deno.test("a settings failure after a successful build removes the old archive and leaves the source untouched", async () => {
  const project = await newProject();
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  assert(await exists(archiveOf(project)));

  // The map-info planner refuses a player slot the map does not have.
  await writeLocal(project, 'settings { players { ["5"] { name = "Absent" } } }\n');
  const before = await snapshot(sourceMap(project));
  const absent = await deno(["task", "build"], project);
  assertEquals(absent.code, 1, absent.text);
  assertStringIncludes(absent.text, "error: maps/map.w3x/war3map.w3i › Player 5 does not exist in the source map.");
  assertEquals(await exists(archiveOf(project)), false, "the stale archive survived a failed settings build");
  assertEquals(await snapshot(sourceMap(project)), before);

  // Binary planning succeeds, then the Lua edit is refused: nothing may be applied, even to the staged copy.
  await writeLocal(project, 'settings { info { name = "Refused" } }\n');
  const renamed = await deno(["task", "build"], project);
  assertEquals(renamed.code, 0, renamed.text);
  assert(await exists(archiveOf(project)));
  const lua = join(sourceMap(project), "war3map.lua");
  const script = await Deno.readTextFile(lua);
  assertStringIncludes(script, 'SetMapName("TRIGSTR_001")');
  await Deno.writeTextFile(lua, script.replace(/^SetMapName\(.*\)\r?$/m, ""));
  const beforeLua = await snapshot(sourceMap(project));
  const refused = await deno(["task", "build"], project);
  assertEquals(refused.code, 1, refused.text);
  assertStringIncludes(refused.text, "error: maps/map.w3x/war3map.lua › Cannot apply map settings to Lua");
  assertEquals(await exists(archiveOf(project)), false, "the stale archive survived a Lua settings refusal");
  assertEquals(await snapshot(sourceMap(project)), beforeLua);
  const staged = join(project, "dist", "stage", "map.w3x", "war3map.w3i");
  assertEquals(await Deno.readFile(staged), beforeLua["war3map.w3i"], "map info was written before Lua planning");
});

Deno.test("check validates settings against the source map without staging", async () => {
  const project = await newProject();
  const lua = join(sourceMap(project), "war3map.lua");
  await Deno.writeTextFile(
    lua,
    (await Deno.readTextFile(lua)).replace("SetPlayerTeam(Player(11), 1)", "SetPlayerTeam(Player(11), 0)"),
  );
  await writeLocal(project, 'settings { forces { ["0"] { allied = false } } }\n');
  const before = await snapshot(sourceMap(project));
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(checked.text, "error: maps/map.w3x/war3map.lua › ");
  assertStringIncludes(checked.text, "disagrees with force 0");
  // Compilation caches Lua under dist/stage/lua; the map itself is never staged by check.
  assertEquals(await exists(join(project, "dist", "stage", "map.w3x")), false, "check staged the map");
  assertEquals(await snapshot(sourceMap(project)), before);

  await writeLocal(
    project,
    'settings { gameplay { foodLimit = 200 } gameplayConstants { ["Misc"] { ["FoodCeiling"] = "1" } } }\n',
  );
  const conflict = await deno(["task", "check"], project);
  assertEquals(conflict.code, 1, conflict.text);
  assertStringIncludes(conflict.text, "error: moonwell.local.pkl › Conflicting typed and raw gameplay constant");

  await writeLocal(project, 'settings { info { name = "Checked" } }\n');
  const valid = await deno(["task", "check"], project);
  assertEquals(valid.code, 0, valid.text);
  assertEquals(await exists(join(project, "dist", "stage", "map.w3x")), false, "check staged the map");
  assertEquals(await snapshot(sourceMap(project)), before, "check wrote planned settings into the source map");

  // With no active settings, check keeps working without a source map.
  await writeLocal(project, "");
  await Deno.remove(sourceMap(project), { recursive: true });
  const unmapped = await deno(["task", "check"], project);
  assertEquals(unmapped.code, 0, unmapped.text);
  await writeLocal(project, 'settings { info { name = "Needs a map" } }\n');
  const missing = await deno(["task", "check"], project);
  assertEquals(missing.code, 1, missing.text);
  assertStringIncludes(missing.text, "error: maps/map.w3x › Source map folder maps/map.w3x not found.");
});

Deno.test("test stages settings with the runtime, and a minified build keeps settings before the bundle", async () => {
  const project = await newProject();
  const exe = join(project, "Warcraft III.exe");
  await Deno.writeTextFile(exe, "");
  await writeLocal(project, `launch { gameExecutable = #"${exe}"# }\n${SETTINGS}`);
  const before = await snapshot(sourceMap(project));
  const calls: Array<[string, string[]]> = [];
  const ctx: CommandContext = {
    ...createContext(project, silentLogger()),
    spawn: (command, args) => calls.push([command, args]),
  };
  await test(ctx, {});

  const staged = join(project, "dist", "stage", "map.w3x");
  assertEquals(calls, [[exe, ["-launch", "-windowmode", "windowed", "-loadfile", staged]]]);
  assertEquals(readMapInfo(await Deno.readFile(join(staged, "war3map.w3i"))).info.name.value, "Moonwell settings test");
  assertSettingsLua(await Deno.readTextFile(join(staged, "war3map.lua")));
  assertStringIncludes(await Deno.readTextFile(join(staged, "war3mapSkin.txt")), "Test=value");
  assertEquals(await snapshot(sourceMap(project)), before);

  await build(ctx, { minify: true });
  const lua = decode(await openMpq(await Deno.readFile(archiveOf(project))).read("war3map.lua"));
  assertSettingsLua(lua);
  const bundle = lua.indexOf("__mw.lines = {");
  assert(bundle > lua.indexOf('SetMapName("Moonwell settings test")'), "module error metadata precedes settings");
  assert(lua.indexOf("__mw.minified = true") > bundle, "the minified bundle was not emitted");
  assertEquals(await snapshot(sourceMap(project)), before);
});

Deno.test("dev reports a settings error when the manifest changes", async () => {
  const project = await newProject();
  const child = new Deno.Command(Deno.execPath(), {
    args: ["run", "-A", MAIN, "dev"],
    cwd: project,
    stdout: "null",
    stderr: "piped",
  }).spawn();
  const reader = child.stderr.pipeThrough(new TextDecoderStream()).getReader();
  let seen = "";
  const waitFor = async (text: string) => {
    const deadline = Date.now() + 60_000;
    while (!seen.includes(text)) {
      // Race each read against the time left, so a silent dev process cannot hang the test.
      let timer: ReturnType<typeof setTimeout> | undefined;
      const timeout = new Promise<never>((_, reject) => {
        timer = setTimeout(
          () => reject(new Error(`timed out waiting for "${text}"; output:\n${seen}`)),
          Math.max(0, deadline - Date.now()),
        );
      });
      try {
        const { value, done } = await Promise.race([reader.read(), timeout]);
        if (done) throw new Error(`dev exited early; output:\n${seen}`);
        seen += value;
      } finally {
        clearTimeout(timer);
      }
    }
  };
  try {
    await waitFor("Watching src/");
    await writeLocal(project, 'settings { players { ["5"] { name = "Absent" } } }\n');
    await waitFor("error: maps/map.w3x/war3map.w3i › Player 5 does not exist in the source map.");
  } finally {
    child.kill();
    await reader.cancel();
    await child.status;
  }
});
