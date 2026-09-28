import { assert, assertEquals, assertRejects, assertThrows } from "@std/assert";
import { copy, exists } from "@std/fs";
import { fromFileUrl, join } from "@std/path";
import { type CommandContext, createContext } from "../../src/context.ts";
import { OBJECT_IDS_FILE, renderObjectIds } from "../../src/objectdata/ids.ts";
import { emptyObjects, type ManifestObject } from "../../src/objectdata/manifest.ts";
import { entryModuleName, prepareStage } from "../../src/pipeline.ts";
import { launchGame } from "../../src/launch.ts";
import type { Project } from "../../src/project/project.ts";
import { validateMapSettings } from "../../src/settings/options.ts";
import { formatError, MoonwellError, ObjectDataError, ProblemsError } from "../../src/shared/errors.ts";
import { MACROS_FILE } from "../../src/yue/macros.ts";
import { silentLogger } from "../support/logger.ts";

Deno.test("entryModuleName converts src paths to dotted names", () => {
  assertEquals(entryModuleName("src/main.yue"), "main");
  assertEquals(entryModuleName("./src/game/init.yue"), "game.init");
  assertEquals(entryModuleName("src\\testbed\\run.yue"), "testbed.run");
  assertThrows(() => entryModuleName("lib/main.yue"), MoonwellError, "under src/");
  assertThrows(() => entryModuleName("src/main.lua"), MoonwellError, "under src/");
});

Deno.test("launchGame explains a missing or wrong executable", async () => {
  const launch = { gameExecutable: null, args: [] };
  const missing = await assertRejects(() => launchGame(launch, "map"), MoonwellError, "gameExecutable is not set");
  assertEquals(missing.file, "moonwell.local.pkl");
  const missingExe = join(await Deno.makeTempDir(), "Warcraft III.exe");
  await assertRejects(
    () => launchGame({ gameExecutable: missingExe, args: [] }, "map"),
    MoonwellError,
    "not found",
  );
});

Deno.test("launchGame passes the launch args and -loadfile", async () => {
  const dir = await Deno.makeTempDir();
  const exe = join(dir, "Warcraft III.exe");
  await Deno.writeTextFile(exe, "");
  const calls: Array<[string, string[]]> = [];
  await launchGame(
    { gameExecutable: exe, args: ["-launch"] },
    "C:/map.w3x",
    (command, args) => calls.push([command, args]),
  );
  assertEquals(calls, [[exe, ["-launch", "-loadfile", "C:/map.w3x"]]]);
});

const TEMPLATE_MAP = fromFileUrl(new URL("../../../template/maps/map.w3x", import.meta.url));

const captain = (extra: Partial<ManifestObject> = {}): ManifestObject => ({
  id: "h000",
  base: "hfoo",
  source: "objects/units.pkl",
  typed: { name: "Captain", hitPointsMaximumBase: 500 },
  properties: {},
  ...extra,
});

/**
 * A project on a copy of the template map whose Yue compiler is a stand-in: each compile records what the pipeline
 * had done by then and writes an empty Lua module, so the order of the steps is observable without yue or Pkl.
 */
async function stageProject(objects: Project["objects"], settings: unknown = {}, globals = "") {
  const root = await Deno.makeTempDir({ prefix: "moonwell-pipeline-" });
  await copy(TEMPLATE_MAP, join(root, "maps", "map.w3x"));
  await Deno.mkdir(join(root, "src"));
  await Deno.writeTextFile(join(root, "src", "main.yue"), "x = 1\n");
  const yue = join(root, "yue-stand-in");
  await Deno.writeTextFile(yue, "");
  const logger = silentLogger();
  const events: string[] = [];
  const calls: Array<{ args: string[]; macros: boolean }> = [];
  const ctx: CommandContext = {
    ...createContext(root, logger),
    run: async (_command, args) => {
      calls.push({ args, macros: await exists(join(root, ...MACROS_FILE.split("/"))) });
      if (args[0] === "-g") return { code: 0, stdout: globals, stderr: "" };
      const generated = await exists(join(root, OBJECT_IDS_FILE));
      events.push(
        `compile ${args.at(-1)?.slice(join(root, "src").length + 1).replaceAll("\\", "/")} (objects.yue ${generated})`,
      );
      await Deno.writeTextFile(args[3], "local x = 1\n");
      return { code: 0, stdout: "", stderr: "" };
    },
  };
  ctx.install = {
    ...ctx.install,
    run: () => Promise.resolve({ code: 0, stdout: "Yuescript version: 0.34.2", stderr: "" }),
  };
  const project: Project = {
    root,
    manifest: "moonwell.local.pkl",
    map: { folder: "map.w3x", entry: "src/main.yue" },
    build: { folder: "dist/bin", minify: false },
    launch: { gameExecutable: null, args: [] },
    yue: { version: "0.34.2", path: yue },
    assets: { paths: {}, exclude: [] },
    lint: { unknownGlobals: "error", globals: [] },
    libraries: {},
    settings: validateMapSettings(settings),
    objects,
  };
  return { root, ctx, logger, events, project, calls };
}

const withCaptain = () => ({ ...emptyObjects(), units: { captain: captain() } });
const stagedFile = (root: string, name: string) => join(root, "dist", "stage", "map.w3x", name);

Deno.test("prepareStage plans objects before compiling and applies them to the staged copy only", async () => {
  const { root, ctx, logger, events, project } = await stageProject(withCaptain());
  try {
    await prepareStage(ctx, project, {});
    // The generated module exists before the first compile, so gameplay compiles against current ids.
    assertEquals(events.sort(), [
      "compile generated/objects.yue (objects.yue true)",
      "compile main.yue (objects.yue true)",
    ]);
    assertEquals(
      await Deno.readTextFile(join(root, OBJECT_IDS_FILE)),
      renderObjectIds([{ category: "units", key: "captain", id: "h000" }]),
    );
    for (const name of ["war3map.w3u", "war3mapSkin.w3u"]) {
      assert(await exists(stagedFile(root, name)), `${name} is staged`);
      assertEquals(await exists(join(root, "maps", "map.w3x", name)), false, `${name} stays out of the source map`);
    }
    assert(logger.lines.includes("Added 1 custom object(s) to 2 file(s)."), logger.lines.join("\n"));
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("prepareStage without objects logs nothing about them and creates no generated module", async () => {
  const { root, ctx, logger, project } = await stageProject(emptyObjects());
  try {
    await prepareStage(ctx, project, {});
    assertEquals(await exists(join(root, OBJECT_IDS_FILE)), false);
    assertEquals(await exists(stagedFile(root, "war3map.w3u")), false);
    assertEquals(logger.lines.filter((line) => line.startsWith("Added ")), []);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("prepareStage fails on invalid objects before compiling or staging", async () => {
  const objects = { ...emptyObjects(), units: { captain: captain({ base: "zzzz" }) } };
  const { root, ctx, events, project } = await stageProject(objects);
  try {
    const error = await assertRejects(() => prepareStage(ctx, project, {}), ObjectDataError);
    assertEquals(error.file, "objects/units.pkl");
    assertEquals(events, []);
    assertEquals(await exists(join(root, OBJECT_IDS_FILE)), false);
    assertEquals(await exists(join(root, "dist", "stage", "map.w3x")), false);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("prepareStage fails on an unknown global after compiling, before staging the map", async () => {
  const { root, ctx, project } = await stageProject(emptyObjects(), {}, "CreatUnit 1 1\n");
  try {
    const error = await assertRejects(() => prepareStage(ctx, project, {}), ProblemsError);
    assertEquals(
      formatError(error),
      "error: src/main.yue:1:1 › Unknown global CreatUnit.\n" +
        "hint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.",
    );
    assertEquals(await exists(join(root, "dist", "stage", "map.w3x")), false);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("prepareStage applies objects after staging and before map settings", async () => {
  // Settings fail on a map without war3map.w3i; by then the object files must already be in the staged copy.
  const { root, ctx, project } = await stageProject(withCaptain(), { info: { name: "Ordered" } });
  try {
    await Deno.remove(join(root, "maps", "map.w3x", "war3map.w3i"));
    await assertRejects(() => prepareStage(ctx, project, {}), MoonwellError, "needed by the configured settings");
    assert(await exists(stagedFile(root, "war3map.w3u")));
    assert(await exists(stagedFile(root, "war3mapSkin.w3u")));
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("prepareStage writes the macro module before any yue run and gives every run its path", async () => {
  const { root, ctx, project, calls } = await stageProject(emptyObjects());
  try {
    await prepareStage(ctx, project, {});
    const path = join(root, ".moonwell", "yue", "?.lua");
    assertEquals(calls.map((call) => call.args[0] === "-g" ? "-g" : "compile"), ["compile", "-g"]);
    for (const call of calls) {
      assert(call.macros, "the macro module exists before yue runs");
      const at = call.args.indexOf("--path");
      assertEquals(call.args[at + 1], path);
      assertEquals(at + 2, call.args.length - 1, "--path comes right before the source file");
    }
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
