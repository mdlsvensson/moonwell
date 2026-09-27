import { assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { join } from "@std/path";
import { refreshEditorFiles } from "../../src/editor/refresh.ts";
import type { Natives } from "../../src/natives/natives.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

const NATIVES: Natives = {
  gameVersion: "9.9.9",
  types: [{ name: "agent", extends: "handle" }],
  functions: [{ name: "DoNothing", source: "common.j", constant: false, params: [], returns: "nothing" }],
  globals: [],
  lua: { globals: [], removed: [] },
};

async function project(withMap: boolean): Promise<string> {
  const root = await Deno.makeTempDir({ prefix: "moonwell-editor-" });
  if (withMap) {
    await Deno.mkdir(join(root, "maps", "map.w3x"), { recursive: true });
    await Deno.writeTextFile(join(root, "maps", "map.w3x", "war3map.lua"), "udg_Score = 0\nfunction main()\nend\n");
  }
  return root;
}

const inputs = {
  objects: [{ category: "units" as const, key: "captain", id: "h000" }],
  mapFolder: "maps/map.w3x",
  natives: NATIVES,
};

Deno.test("refreshEditorFiles writes the four declaration files, then only what changed", async () => {
  const root = await project(true);
  assertEquals(await refreshEditorFiles(root, inputs), [
    ".moonwell/types/natives.d.lua",
    ".moonwell/types/moonwell.d.lua",
    ".moonwell/types/objects.d.lua",
    ".moonwell/types/map.d.lua",
  ]);
  assertStringIncludes(await Deno.readTextFile(join(root, ".moonwell", "types", "map.d.lua")), "udg_Score = nil");
  assertEquals(await refreshEditorFiles(root, inputs), []);
  await Deno.writeTextFile(join(root, "maps", "map.w3x", "war3map.lua"), "udg_Other = 0\n");
  assertEquals(await refreshEditorFiles(root, inputs), [".moonwell/types/map.d.lua"]);
});

Deno.test("refreshEditorFiles works without a source map", async () => {
  const root = await project(false);
  await refreshEditorFiles(root, inputs);
  assertStringIncludes(
    await Deno.readTextFile(join(root, ".moonwell", "types", "map.d.lua")),
    "no maps/map.w3x/war3map.lua",
  );
});

const MAP_FOLDER_HINT = "map.folder must be a map World Editor saved in folder format; re-save it that way.";

Deno.test("a war3map.lua that is a folder fails with a MoonwellError", async () => {
  const root = await project(false);
  await Deno.mkdir(join(root, "maps", "map.w3x", "war3map.lua"), { recursive: true });
  const error = await assertRejects(() => refreshEditorFiles(root, inputs), MoonwellError);
  assertEquals(error.file, "maps/map.w3x/war3map.lua");
  assertEquals(error.hint, MAP_FOLDER_HINT);
});

Deno.test("a packed map file where the map folder should be is no source map or a MoonwellError", async () => {
  const root = await project(false);
  await Deno.mkdir(join(root, "maps"));
  await Deno.writeTextFile(join(root, "maps", "map.w3x"), "a packed map, not a folder");
  try {
    // On Windows, reading through a file reports NotFound, so there is no source map.
    await refreshEditorFiles(root, inputs);
  } catch (error) {
    if (!(error instanceof MoonwellError)) throw error;
    assertEquals(error.file, "maps/map.w3x/war3map.lua");
    assertEquals(error.hint, MAP_FOLDER_HINT);
  }
});

Deno.test("a .moonwell that cannot be written fails with a MoonwellError", async () => {
  const root = await project(false);
  await Deno.writeTextFile(join(root, ".moonwell"), "a file, not a folder");
  const error = await assertRejects(() => refreshEditorFiles(root, inputs), MoonwellError, ".moonwell");
  assertStringIncludes(error.hint ?? "", ".moonwell");
});
