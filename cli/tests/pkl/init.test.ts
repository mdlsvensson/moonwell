import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { init } from "../../src/commands/init.ts";
import { createContext } from "../../src/context.ts";
import { loadProject } from "../../src/project/project.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";

Deno.test("init --link scaffolds a project that loads", async () => {
  const parent = await Deno.makeTempDir();
  const ctx = createContext(parent, silentLogger());
  const project = await init(join(parent, "my-map"), ctx, { link: true });
  for (
    const file of ["moonwell.pkl", "src/main.yue", "maps/map.w3x/war3map.lua", "PklProject.deps.json", ".gitignore"]
  ) {
    assert(await exists(join(project, file)), file);
  }
  assertStringIncludes(await Deno.readTextFile(join(project, "deno.json")), "cli/src/main.ts build");
  assertEquals((await loadProject(project)).map.folder, "map.w3x");
});

Deno.test("init refuses a non-empty directory", async () => {
  const dir = await Deno.makeTempDir();
  await Deno.writeTextFile(join(dir, "keep.txt"), "");
  await assertRejects(() => init(dir, createContext(dir, silentLogger()), { link: true }), MoonwellError, "not empty");
});
