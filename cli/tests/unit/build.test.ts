import { assertEquals, assertRejects } from "@std/assert";
import { join } from "@std/path";
import { archivePath } from "../../src/commands/build.ts";
import type { Project } from "../../src/project/project.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { validateMapSettings } from "../../src/settings/options.ts";

function project(root: string, folder: string): Project {
  return {
    root,
    map: { folder: "map.w3x", entry: "src/main.yue" },
    build: { folder, minify: false },
    launch: { gameExecutable: null, args: [] },
    yue: { version: "0.34.2", path: null },
    assets: { paths: {}, exclude: [] },
    settings: validateMapSettings({}),
  };
}

Deno.test("archivePath places the archive under build.folder", async () => {
  const root = await Deno.makeTempDir();
  assertEquals(await archivePath(root, project(root, "dist/bin")), join(root, "dist", "bin", "map.w3x"));
});

Deno.test("archivePath refuses a directory, such as the source map", async () => {
  const root = await Deno.makeTempDir();
  await Deno.mkdir(join(root, "maps", "map.w3x"), { recursive: true });
  await assertRejects(() => archivePath(root, project(root, "maps")), MoonwellError, "is a directory");
});

Deno.test("archivePath refuses a path outside the project", async () => {
  const root = await Deno.makeTempDir();
  await assertRejects(() => archivePath(root, project(root, "..")), MoonwellError, "outside the project");
  await assertRejects(() => archivePath(root, project(root, "../other")), MoonwellError, "outside the project");
});
