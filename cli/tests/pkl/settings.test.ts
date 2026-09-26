import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { basename, join } from "@std/path";
import { init } from "../../src/commands/init.ts";
import { settingsCheck } from "../../src/commands/settings-check.ts";
import { type CommandContext, createContext } from "../../src/context.ts";
import { main } from "../../src/main.ts";
import { loadProject } from "../../src/project/project.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { runProcess } from "../../src/shared/process.ts";
import { hasSettings, validateMapSettings } from "../../src/settings/options.ts";
import { silentLogger } from "../support/logger.ts";

/** Scaffolds an `init --link` project in a temporary folder, runs `body`, and removes the folder. */
async function withProject(body: (root: string) => Promise<void>): Promise<void> {
  const parent = await Deno.makeTempDir({ prefix: "moonwell-settings-" });
  try {
    await body(await init(join(parent, "map"), createContext(parent, silentLogger()), { link: true }));
  } finally {
    await Deno.remove(parent, { recursive: true });
  }
}

const writeLocal = (root: string, body: string) =>
  Deno.writeTextFile(join(root, "moonwell.local.pkl"), `amends "moonwell.pkl"\n${body}\n`);

/** Every file under `dir` with its bytes, for before/after comparisons. */
async function snapshot(dir: string): Promise<Map<string, string>> {
  const files = new Map<string, string>();
  for await (const entry of Deno.readDir(dir)) {
    const path = join(dir, entry.name);
    if (entry.isDirectory) { for (const [file, bytes] of await snapshot(path)) files.set(file, bytes); }
    else files.set(path, (await Deno.readFile(path)).join(","));
  }
  return files;
}

/** A context whose runner only lets `pkl` through and records every command, proving Yue is never involved. */
function pklOnlyContext(
  root: string,
): { ctx: CommandContext; commands: string[]; logger: ReturnType<typeof silentLogger> } {
  const logger = silentLogger();
  const commands: string[] = [];
  const ctx = createContext(root, logger);
  ctx.run = (command, args, options) => {
    commands.push(command);
    if (command !== "pkl") throw new Error(`settings:check ran ${command}`);
    return runProcess(command, args, options);
  };
  ctx.install = {
    ...ctx.install,
    fetch: () => Promise.reject(new Error("settings:check tried to download")),
    run: () => Promise.reject(new Error("settings:check tried to install")),
  };
  return { ctx, commands, logger };
}

Deno.test("settings:check evaluates the manifest without staging or compiling", async () => {
  await withProject(async (root) => {
    const path = join(root, "maps", "map.w3x", "war3map.w3i");
    const before = await Deno.readFile(path);
    await writeLocal(root, 'settings { info { name = "Checked" } gameplay { foodLimit = 200 } }');
    const { ctx, commands, logger } = pklOnlyContext(root);
    const changes = await settingsCheck(ctx);
    assertEquals(changes.length, 3);
    assertEquals(logger.lines, [
      "  war3map.w3i",
      "  war3map.lua",
      "  war3mapMisc.txt",
      "Map settings valid: 3 internal file(s) would change during build.",
    ]);
    assertEquals(await Deno.readFile(path), before);
    assertEquals(await exists(join(root, "dist", "stage")), false);
    assertEquals(await exists(join(root, "dist", ".lock")), false);
    assert(commands.length > 0 && commands.every((command) => command === "pkl"), commands.join(", "));
  });
});

Deno.test("the template's settings block is a no-op on the template map", async () => {
  await withProject(async (root) => {
    const manifest = await Deno.readTextFile(join(root, "moonwell.pkl"));
    for (const field of ["recommendedPlayers = null", "background = null", "fixedStart = null", "density = null"]) {
      assertStringIncludes(manifest, field);
    }
    assertEquals(manifest.includes("Listing"), false);
    assertEquals(manifest.includes("gameplayConstants {"), false, "advanced raw overrides stay out of the template");
    const project = await loadProject(root);
    assertEquals(project.settings, validateMapSettings({}));
    assertEquals(hasSettings(project.settings), false);
    const before = await snapshot(root);
    const { ctx, logger } = pklOnlyContext(root);
    assertEquals(await settingsCheck(ctx), []);
    assertEquals(logger.lines, ["Map settings valid: 0 internal file(s) would change during build."]);
    assertEquals(await snapshot(root), before);
  });
});

Deno.test("settings:check does not take or wait for the build lock", async () => {
  await withProject(async (root) => {
    await Deno.mkdir(join(root, "dist"), { recursive: true });
    await Deno.writeTextFile(join(root, "dist", ".lock"), "999999");
    await writeLocal(root, 'settings { info { author = "Locked out" } }');
    const { ctx } = pklOnlyContext(root);
    assertEquals((await settingsCheck(ctx)).map((change) => basename(change.file)), ["war3map.w3i"]);
    assertEquals(await Deno.readTextFile(join(root, "dist", ".lock")), "999999");
  });
});

Deno.test("loadProject evaluates every settings group from the template manifest", async () => {
  await withProject(async (root) => {
    // The shared manifest sets a color; the local one replaces it rather than appending to it.
    const manifest = join(root, "moonwell.pkl");
    const shared = await Deno.readTextFile(manifest);
    await Deno.writeTextFile(
      manifest,
      shared.replace("waterColor = null ", "waterColor = List(1, 2, 3, 4)"),
    );
    await writeLocal(
      root,
      `settings {
  info { name = "N"; author = ""; description = "D"; recommendedPlayers = "1-4" }
  loadingScreen { background = -1; model = "Load.mdx"; text = "T"; title = "Ti"; subtitle = "S" }
  gameplay { heroMaxLevel = 25; foodLimit = 0 }
  gameplayConstants { ["Misc"] { ["FoodCeiling"] = "0"; ["Other"] = "" } }
  gameInterface { ["CustomSkin"] { ["constructor"] = "value" } }
  players { ["0"] { name = "P"; controller = "computer"; race = "orc"; fixedStart = false; x = 1.5; y = -2.0 } }
  forces { ["1"] { name = "F"; allied = false; alliedVictory = true; sharedVision = false; sharedControl = true
    sharedAdvancedControl = false } }
  environment {
    soundEnvironment = "Mountains"
    waterColor = List(10, 20, 30, 255)
    fog { enabled = true; style = 2; start = 100.0; end = 1000.0; density = 0.5; color = List(5, 6, 7, 8) }
  }
}`,
    );
    const project = await loadProject(root);
    assertEquals(project.manifest, "moonwell.local.pkl");
    assertEquals(project.settings, {
      info: { name: "N", author: "", description: "D", recommendedPlayers: "1-4" },
      loadingScreen: { background: -1, model: "Load.mdx", text: "T", title: "Ti", subtitle: "S" },
      gameplay: { heroMaxLevel: 25, foodLimit: 0 },
      gameplayConstants: { Misc: { FoodCeiling: "0", Other: "" } },
      gameInterface: { CustomSkin: { constructor: "value" } },
      players: { "0": { name: "P", controller: "computer", race: "orc", fixedStart: false, x: 1.5, y: -2 } },
      forces: {
        "1": {
          name: "F",
          allied: false,
          alliedVictory: true,
          sharedVision: false,
          sharedControl: true,
          sharedAdvancedControl: false,
        },
      },
      environment: {
        soundEnvironment: "Mountains",
        waterColor: [10, 20, 30, 255],
        fog: { enabled: true, style: 2, start: 100, end: 1000, density: 0.5, color: [5, 6, 7, 8] },
      },
    });
    // The shared color alone survives when the local manifest leaves it null.
    await writeLocal(root, "");
    assertEquals((await loadProject(root)).settings.environment, { waterColor: [1, 2, 3, 4] });
  });
});

Deno.test("manifest settings errors name the evaluated local manifest", async () => {
  await withProject(async (root) => {
    const cases: [string, string][] = [
      ['settings { players { ["24"] { name = "x" } } }', "players"],
      ['settings { gameplayConstants { ["Misc"] { ["X"] = "a\\nb" } } }', "gameplayConstants"],
      [
        'settings { gameplay { foodLimit = 200 } gameplayConstants { ["misc"] { ["foodceiling"] = "100" } } }',
        "FoodCeiling",
      ],
    ];
    for (const [body, message] of cases) {
      await writeLocal(root, body);
      const error = await assertRejects(() => loadProject(root), MoonwellError, message);
      assertEquals(error.file, "moonwell.local.pkl", body);
      const lines: string[] = [];
      assertEquals(await main(["settings:check"], root, (line) => lines.push(line)), 1);
      assertStringIncludes(lines.join("\n"), "moonwell.local.pkl");
    }
  });
});

Deno.test("settings:check names the map file when a coordinated Lua edit is unsafe", async () => {
  await withProject(async (root) => {
    const lua = join(root, "maps", "map.w3x", "war3map.lua");
    await Deno.writeTextFile(lua, (await Deno.readTextFile(lua)).replace(/^SetMapName\(.*\)$/m, ""));
    const before = await snapshot(join(root, "maps"));
    await writeLocal(root, 'settings { info { name = "No call" } }');
    const error = await assertRejects(() => settingsCheck(pklOnlyContext(root).ctx), MoonwellError);
    // Relative like build, check and the missing-folder error, so all commands name the same file.
    assertEquals(error.file, "maps/map.w3x/war3map.lua");
    assertEquals(await snapshot(join(root, "maps")), before);
  });
});

Deno.test("settings:check refuses a missing source map folder and names it", async () => {
  await withProject(async (root) => {
    await writeLocal(root, 'map { folder = "other.w3x" }\nsettings { info { name = "x" } }');
    const error = await assertRejects(() => settingsCheck(pklOnlyContext(root).ctx), MoonwellError, "not found");
    // Named like build names it: map.folder in the evaluated manifest is what to fix.
    assertEquals(error.file, "moonwell.local.pkl");
  });
});
