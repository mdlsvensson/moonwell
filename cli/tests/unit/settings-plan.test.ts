import { assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { join } from "@std/path";
import { validateMapSettings } from "../../src/settings/options.ts";
import { applySettingsPlan, planMapSettings, settingsMapDir } from "../../src/settings/plan.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { fixtureBytes, fixtureLua } from "../support/map-settings.ts";

async function withDir(run: (dir: string) => Promise<void>): Promise<void> {
  const dir = await Deno.makeTempDir();
  try {
    await run(dir);
  } finally {
    await Deno.remove(dir, { recursive: true });
  }
}

async function writeFixture(dir: string): Promise<void> {
  await Deno.writeFile(join(dir, "war3map.w3i"), await fixtureBytes());
  await Deno.writeTextFile(join(dir, "war3map.lua"), await fixtureLua());
}

/** Every entry below `dir` with its bytes (directories as null), to prove a failed plan wrote nothing. */
async function snapshot(dir: string): Promise<Record<string, Uint8Array | null>> {
  const result: Record<string, Uint8Array | null> = {};
  const visit = async (path: string, name: string) => {
    for await (const entry of Deno.readDir(path)) {
      const child = join(path, entry.name), key = `${name}${entry.name}`;
      if (entry.isDirectory) {
        result[key] = null;
        await visit(child, `${key}/`);
      } else result[key] = await Deno.readFile(child);
    }
  };
  await visit(dir, "");
  return result;
}

async function refusesWithoutWrites(dir: string, input: unknown, file: string): Promise<MoonwellError> {
  const before = await snapshot(dir);
  const error = await assertRejects(() => planMapSettings(dir, validateMapSettings(input)), MoonwellError);
  assertEquals(error.file, file);
  assertEquals(Boolean(error.hint), true);
  assertEquals(await snapshot(dir), before);
  return error;
}

const decode = (bytes: Uint8Array) => new TextDecoder().decode(bytes);

Deno.test("planning validates every change before writes and application filters unchanged files", async () => {
  await withDir(async (dir) => {
    const bytes = await fixtureBytes(), lua = await fixtureLua();
    await Deno.writeFile(join(dir, "war3map.w3i"), bytes);
    await Deno.writeTextFile(join(dir, "war3map.lua"), lua);
    const s = validateMapSettings({ info: { name: "Planned" }, gameplay: { foodLimit: 200 } });
    const plan = await planMapSettings(dir, s);
    assertEquals(plan.map((c) => c.file), ["war3map.w3i", "war3map.lua", "war3mapMisc.txt"].map((f) => join(dir, f)));
    assertEquals(await Deno.readFile(join(dir, "war3map.w3i")), bytes);
    assertEquals(await Deno.readTextFile(join(dir, "war3map.lua")), lua);
    await applySettingsPlan(plan);
    assertEquals(await planMapSettings(dir, s), []);
    const before = await Deno.readFile(join(dir, "war3map.w3i"));
    await assertRejects(
      () =>
        planMapSettings(
          dir,
          validateMapSettings({
            info: { name: "Not written" },
            players: { "5": { name: "Absent" } },
          }),
        ),
      MoonwellError,
    );
    assertEquals(await Deno.readFile(join(dir, "war3map.w3i")), before);
    assertEquals(await planMapSettings(join(dir, "missing"), validateMapSettings({})), []);
  });
});

Deno.test("applied plans contain the patched bytes of each internal file", async () => {
  await withDir(async (dir) => {
    await writeFixture(dir);
    await Deno.writeTextFile(join(dir, "war3mapMisc.txt"), "[Misc]\r\nKeep=1\r\n");
    const plan = await planMapSettings(
      dir,
      validateMapSettings({ info: { name: "Planned" }, gameplay: { heroMaxLevel: 25 } }),
    );
    assertEquals(plan.length, 3);
    await applySettingsPlan(plan);
    assertStringIncludes(await Deno.readTextFile(join(dir, "war3map.lua")), 'SetMapName("Planned")');
    assertEquals(await Deno.readTextFile(join(dir, "war3mapMisc.txt")), "[Misc]\r\nKeep=1\r\nHeroMaxLevel=25\r\n");
  });
});

Deno.test("all four internal files are returned in stable order", async () => {
  await withDir(async (dir) => {
    await writeFixture(dir);
    await Deno.writeTextFile(join(dir, "war3mapSkin.txt"), "[Existing]\nX=1\n");
    const plan = await planMapSettings(
      dir,
      validateMapSettings({
        gameInterface: { CustomSkin: { Test: "value" } },
        gameplayConstants: { Misc: { GoldCost: "1" } },
        info: { description: "Described" },
      }),
    );
    assertEquals(
      plan.map((change) => change.file),
      ["war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt"].map((file) => join(dir, file)),
    );
    assertEquals(decode(plan[2].bytes), "[Misc]\nGoldCost=1");
    assertEquals(decode(plan[3].bytes), "[Existing]\nX=1\n\n[CustomSkin]\nTest=value\n");
  });
});

Deno.test("text-only settings need neither map info nor Lua, and null-only settings read nothing", async () => {
  await withDir(async (dir) => {
    const plan = await planMapSettings(dir, validateMapSettings({ gameInterface: { CustomSkin: { Test: "" } } }));
    assertEquals(plan.map((change) => change.file), [join(dir, "war3mapSkin.txt")]);
    assertEquals(decode(plan[0].bytes), "[CustomSkin]\nTest=");
    const missing = join(dir, "missing");
    const nullOnly = validateMapSettings({
      info: { name: null },
      players: { "5": { name: null } },
      environment: { fog: {} },
      gameplayConstants: { Misc: {} },
      gameInterface: { CustomSkin: {} },
    });
    assertEquals(await planMapSettings(missing, nullOnly), []);
    // A Misc merge that already matches the source file is not a change.
    await Deno.writeTextFile(join(dir, "war3mapMisc.txt"), "[Misc]\nFoodCeiling=100\n");
    assertEquals(await planMapSettings(dir, validateMapSettings({ gameplay: { foodLimit: 100 } })), []);
  });
});

Deno.test("loading-screen and author settings patch map info without reading Lua", async () => {
  await withDir(async (dir) => {
    await Deno.writeFile(join(dir, "war3map.w3i"), await fixtureBytes());
    const plan = await planMapSettings(
      dir,
      validateMapSettings({ info: { author: "Someone" }, loadingScreen: { title: "T" } }),
    );
    assertEquals(plan.map((change) => change.file), [join(dir, "war3map.w3i")]);
  });
});

Deno.test("missing required map files are file errors", async () => {
  await withDir(async (dir) => {
    await refusesWithoutWrites(dir, { loadingScreen: { title: "T" } }, join(dir, "war3map.w3i"));
    await Deno.writeFile(join(dir, "war3map.w3i"), await fixtureBytes());
    await refusesWithoutWrites(dir, { info: { name: "Needs Lua" } }, join(dir, "war3map.lua"));
    await refusesWithoutWrites(dir, { environment: { soundEnvironment: "Mountains" } }, join(dir, "war3map.lua"));
  });
});

Deno.test("an unreadable optional text file is an error, not an empty file", async () => {
  await withDir(async (dir) => {
    await Deno.mkdir(join(dir, "war3mapMisc.txt"));
    await refusesWithoutWrites(dir, { gameplay: { heroMaxLevel: 5 } }, join(dir, "war3mapMisc.txt"));
    await Deno.mkdir(join(dir, "war3mapSkin.txt"));
    await refusesWithoutWrites(dir, { gameInterface: { A: { B: "c" } } }, join(dir, "war3mapSkin.txt"));
  });
});

Deno.test("map files that are not UTF-8 text are file errors", async () => {
  await withDir(async (dir) => {
    await Deno.writeFile(join(dir, "war3map.w3i"), await fixtureBytes());
    await Deno.writeFile(join(dir, "war3map.lua"), new Uint8Array([0x66, 0xff, 0x66]));
    await refusesWithoutWrites(dir, { info: { name: "X" } }, join(dir, "war3map.lua"));
    await Deno.writeFile(join(dir, "war3mapSkin.txt"), new Uint8Array([0xc3]));
    await refusesWithoutWrites(dir, { gameInterface: { A: { B: "c" } } }, join(dir, "war3mapSkin.txt"));
  });
});

Deno.test("a Lua refusal after successful binary planning writes nothing", async () => {
  await withDir(async (dir) => {
    await Deno.writeFile(join(dir, "war3map.w3i"), await fixtureBytes());
    await Deno.writeTextFile(join(dir, "war3map.lua"), (await fixtureLua()).replace("SetMapName(", "Other("));
    await refusesWithoutWrites(
      dir,
      { info: { name: "Refused" }, gameplay: { foodLimit: 1 }, gameInterface: { A: { B: "c" } } },
      join(dir, "war3map.lua"),
    );
  });
});

Deno.test("conflicting typed and raw constants fail before any map file is read", async () => {
  await withDir(async (dir) => {
    const settings = validateMapSettings({
      gameplay: { foodLimit: 200 },
      gameplayConstants: { MISC: { foodCeiling: "1" } },
    });
    const error = await assertRejects(
      () => planMapSettings(join(dir, "missing"), settings, "moonwell.local.pkl"),
      MoonwellError,
      "FoodCeiling",
    );
    assertEquals(error.file, "moonwell.local.pkl");
  });
});

Deno.test("a UTF-8 byte-order mark survives Lua and text edits", async () => {
  await withDir(async (dir) => {
    const bom = [0xef, 0xbb, 0xbf];
    await Deno.writeFile(join(dir, "war3map.w3i"), await fixtureBytes());
    const lua = new TextEncoder().encode(await fixtureLua());
    await Deno.writeFile(join(dir, "war3map.lua"), new Uint8Array([...bom, ...lua]));
    await Deno.writeFile(join(dir, "war3mapSkin.txt"), new Uint8Array([...bom, ...new TextEncoder().encode("[A]\n")]));
    const plan = await planMapSettings(
      dir,
      validateMapSettings({ info: { name: "BOM" }, gameInterface: { A: { B: "c" } } }),
    );
    assertEquals(plan.length, 3);
    for (const change of plan.slice(1)) assertEquals([...change.bytes.subarray(0, 3)], bom);
    assertEquals(plan[1].bytes[3], lua[0]);
    assertEquals(decode(plan[2].bytes), "[A]\nB=c\n");
  });
});

Deno.test("planning and applying never mutate the normalized settings", async () => {
  await withDir(async (dir) => {
    await writeFixture(dir);
    const settings = validateMapSettings({
      info: { name: "Name", description: "" },
      players: { "0": { name: "Hero", controller: "computer", fixedStart: false, x: 256 } },
      forces: { "0": { allied: false, alliedVictory: true } },
      environment: { soundEnvironment: "", waterColor: [1, 2, 3, 4], fog: { enabled: true, start: 1, end: 2 } },
      gameplay: { heroMaxLevel: 20, foodLimit: 150 },
      gameplayConstants: { misc: { Other: "1" } },
      gameInterface: { CustomSkin: { A: "b" } },
    });
    const before = structuredClone(settings);
    const plan = await planMapSettings(dir, settings);
    assertEquals(plan.length, 4);
    await applySettingsPlan(plan);
    assertEquals(settings, before);
  });
});

Deno.test("staged application reports the file it could not write", async () => {
  await withDir(async (dir) => {
    const file = join(dir, "absent", "war3map.w3i");
    const error = await assertRejects(
      () => applySettingsPlan([{ file, bytes: new Uint8Array([1]) }]),
      MoonwellError,
    );
    assertEquals(error.file, file);
    assertEquals(Boolean(error.hint), true);
  });
});

Deno.test("the settings source map must be an existing folder under maps/", async () => {
  await withDir(async (root) => {
    await Deno.mkdir(join(root, "maps", "map.w3x"), { recursive: true });
    await Deno.writeTextFile(join(root, "maps", "file.w3x"), "");
    assertEquals(await settingsMapDir(root, "map.w3x"), join(root, "maps", "map.w3x"));
    for (const folder of ["absent.w3x", "file.w3x", "", ".", "../maps/map.w3x/..", "../outside"]) {
      const error = await assertRejects(() => settingsMapDir(root, folder), MoonwellError);
      assertEquals(Boolean(error.hint), true);
    }
    await Deno.mkdir(join(root, "outside"));
    // Build stages through safeJoin, which refuses links below the project; settings checks agree with it.
    const type = Deno.build.os === "windows" ? "junction" : "dir";
    await Deno.symlink(join(root, "outside"), join(root, "maps", "link.w3x"), { type });
    await assertRejects(() => settingsMapDir(root, "link.w3x"), MoonwellError, "Symlinks");
    await Deno.rename(join(root, "maps"), join(root, "real-maps"));
    await Deno.symlink(join(root, "real-maps"), join(root, "maps"), { type });
    await assertRejects(() => settingsMapDir(root, "map.w3x"), MoonwellError, "Symlinks");
    const outside = await assertRejects(() => settingsMapDir(root, "../outside", "moonwell.local.pkl"), MoonwellError);
    assertStringIncludes(outside.message, "maps/");
    assertEquals(outside.file, "moonwell.local.pkl");
  });
});

Deno.test("a source label names the map file the user edits while changes keep absolute staged paths", async () => {
  await withDir(async (dir) => {
    const label = "maps/map.w3x";
    const plan = (input: unknown) => planMapSettings(dir, validateMapSettings(input), "moonwell.local.pkl", label);
    const refuses = async (input: unknown, file: string) => {
      const error = await assertRejects(() => plan(input), MoonwellError);
      assertEquals(error.file, file);
    };
    await refuses({ loadingScreen: { title: "T" } }, `${label}/war3map.w3i`);
    await Deno.writeFile(join(dir, "war3map.w3i"), await fixtureBytes());
    await refuses({ players: { "5": { name: "Absent" } } }, `${label}/war3map.w3i`);
    await refuses({ info: { name: "Needs Lua" } }, `${label}/war3map.lua`);
    await Deno.writeTextFile(join(dir, "war3map.lua"), (await fixtureLua()).replace("SetMapName(", "Other("));
    await refuses({ info: { name: "Refused" } }, `${label}/war3map.lua`);
    await Deno.writeTextFile(join(dir, "war3map.lua"), await fixtureLua());
    await Deno.mkdir(join(dir, "war3mapSkin.txt"));
    await refuses({ gameInterface: { A: { B: "c" } } }, `${label}/war3mapSkin.txt`);
    await Deno.writeFile(join(dir, "war3mapMisc.txt"), new Uint8Array([0xc3]));
    await refuses({ gameplay: { foodLimit: 1 } }, `${label}/war3mapMisc.txt`);
    const changes = await plan({ info: { name: "Labelled" } });
    assertEquals(changes.map((change) => change.file), ["war3map.w3i", "war3map.lua"].map((f) => join(dir, f)));
  });
});
