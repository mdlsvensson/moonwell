import { assert, assertEquals, assertStringIncludes } from "@std/assert";
import { fromFileUrl, join } from "@std/path";
import { listFiles } from "../../src/shared/fs.ts";
import { OBJECT_IDS_FILE } from "../../src/objectdata/ids.ts";
import { type ModFile, readModFile } from "../../src/objectdata/modfile.ts";
import { openMpq } from "../support/mpq-reader.ts";

// The template's Captain end to end (spec §10.3): init, build twice, and read the objects back from the archive.

const REPO = fromFileUrl(new URL("../../../", import.meta.url));
const MAIN = join(REPO, "cli", "src", "main.ts");

async function deno(args: string[], cwd: string) {
  const output = await new Deno.Command(Deno.execPath(), { args, cwd, stdout: "piped", stderr: "piped" }).output();
  const decoder = new TextDecoder();
  return { code: output.code, text: decoder.decode(output.stdout) + decoder.decode(output.stderr) };
}

async function snapshot(dir: string): Promise<Map<string, Uint8Array>> {
  const files = new Map<string, Uint8Array>();
  for (const path of await listFiles(dir)) files.set(path, await Deno.readFile(join(dir, ...path.split("/"))));
  return files;
}

function strings(file: ModFile, id: string): Record<string, string> {
  const entry = file.custom.objects.find((object) => object.id === id);
  assert(entry, `${id} is missing from the custom table`);
  const values: Record<string, string> = {};
  for (const mod of entry.sets.flatMap((set) => set.mods)) {
    if (mod.value.type === "string") values[mod.field] = mod.value.value;
  }
  return values;
}

Deno.test("the template's Captain is built into the map's unit files, the same on every build", async () => {
  const project = join(await Deno.makeTempDir({ prefix: "moonwell-e2e-objects-" }), "my-map");
  const init = await deno(["run", "-A", MAIN, "init", "--link", project], REPO);
  assertEquals(init.code, 0, init.text);
  const mapDir = join(project, "maps", "map.w3x");
  const before = await snapshot(mapDir);
  const generated = await Deno.readTextFile(join(project, OBJECT_IDS_FILE));

  const archives: Uint8Array[] = [];
  for (let build = 0; build < 2; build++) {
    const built = await deno(["task", "build"], project);
    assertEquals(built.code, 0, built.text);
    assertStringIncludes(built.text, "Added 1 custom object(s) to 2 file(s).");
    archives.push(await Deno.readFile(join(project, "dist", "bin", "map.w3x")));
  }
  assertEquals(archives[1], archives[0], "a second build produced a different archive");

  const archive = openMpq(archives[0]);
  const main = await archive.read("war3map.w3u");
  const skin = await archive.read("war3mapSkin.w3u");
  assert(main && skin, "the archive lacks war3map.w3u or war3mapSkin.w3u");

  const mainFile = readModFile(main, "simple", "war3map.w3u");
  assertEquals(mainFile.custom.objects.map(({ base, id }) => ({ base, id })), [{ base: "hfoo", id: "h000" }]);
  const skinFile = readModFile(skin, "simple", "war3mapSkin.w3u");
  assertEquals(skinFile.custom.objects.map(({ base, id }) => ({ base, id })), [{ base: "hfoo", id: "h000" }]);
  assertEquals(strings(skinFile, "h000"), {
    unam: "Captain",
    umdl: "units\\human\\TheCaptain\\TheCaptain",
    uico: "ReplaceableTextures\\CommandButtons\\BTNTheCaptain.blp",
  });

  const lua = new TextDecoder().decode(await archive.read("war3map.lua"));
  assertStringIncludes(lua, '__mw.define("generated.objects", function(...)');

  assertEquals(await snapshot(mapDir), before, "a build changed the source map");
  assertEquals(await Deno.readTextFile(join(project, OBJECT_IDS_FILE)), generated, "a build rewrote objects.yue");
  const checked = await deno(["task", "objects:check"], project);
  assertEquals(checked.code, 0, checked.text);
  assertStringIncludes(checked.text, `${OBJECT_IDS_FILE}: current`);
});
