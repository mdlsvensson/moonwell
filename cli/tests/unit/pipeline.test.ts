import { assertEquals, assertRejects, assertThrows } from "@std/assert";
import { join } from "@std/path";
import { entryModuleName } from "../../src/pipeline.ts";
import { launchGame } from "../../src/launch.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

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
