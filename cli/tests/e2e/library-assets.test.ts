import { assertEquals, assertStringIncludes } from "@std/assert";
import { fromFileUrl, join } from "@std/path";
import { readImports } from "../../src/assets/imports.ts";
import { openMpq } from "../support/mpq-reader.ts";

const REPO = fromFileUrl(new URL("../../../", import.meta.url));
const MAIN = join(REPO, "cli", "src", "main.ts");

async function deno(args: string[], cwd: string) {
  const output = await new Deno.Command(Deno.execPath(), { args, cwd, stdout: "piped", stderr: "piped" }).output();
  const decoder = new TextDecoder();
  return { code: output.code, text: decoder.decode(output.stdout) + decoder.decode(output.stderr) };
}

async function write(folder: string, files: Record<string, string>) {
  for (const [path, data] of Object.entries(files)) {
    await Deno.mkdir(join(folder, ...path.split("/").slice(0, -1)), { recursive: true });
    await Deno.writeTextFile(join(folder, ...path.split("/")), data);
  }
}

Deno.test("a library's files are imported into the built map, and the map's own file replaces one", async () => {
  const project = join(await Deno.makeTempDir({ prefix: "moonwell-e2e-" }), "my-map");
  const created = await deno(["run", "-A", MAIN, "init", "--link", project], REPO);
  assertEquals(created.code, 0, created.text);
  const library = await Deno.makeTempDir({ prefix: "moonwell-lib-" });
  await write(library, {
    "moonwell-library.json": '{ "dir": "src", "assets": "assets" }\n',
    "src/golems/names.lua": 'return { first = "Granite" }\n',
    "assets/war3mapImported/golems/frames.toc": "toc from the library",
    "assets/Textures/Golem.blp": "texture from the library",
  });
  // The library's file says where its modules and files are: the map names only the path.
  const local = join(project, "moonwell.local.pkl");
  await Deno.writeTextFile(
    local,
    `${await Deno.readTextFile(local)}\nlibraries { ["golems"] { path = "${library.replaceAll("\\", "/")}" } }\n`,
  );
  const main = join(project, "src", "main.yue");
  await Deno.writeTextFile(main, `${await Deno.readTextFile(main)}\nimport "golems.names"\nprint names.first\n`);

  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 0, checked.text);
  assertStringIncludes(checked.text, "2 asset(s).");

  await write(project, { "assets/textures/golem.blp": "texture from the map" });
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  assertStringIncludes(built.text, "assets/textures/golem.blp replaces library golems's Textures/Golem.blp");
  assertStringIncludes(built.text, "Imported 2 asset(s).");

  const staged = join(project, "dist", "stage", "map.w3x");
  assertEquals(
    await Deno.readTextFile(join(staged, "war3mapImported", "golems", "frames.toc")),
    "toc from the library",
  );
  assertEquals(await Deno.readTextFile(join(staged, "textures", "golem.blp")), "texture from the map");
  const imports = readImports(await Deno.readFile(join(staged, "war3map.imp"))).map((entry) => entry.path);
  assertEquals(imports.sort(), ["textures\\golem.blp", "war3mapImported\\golems\\frames.toc"]);

  const archive = openMpq(await Deno.readFile(join(project, "dist", "bin", "map.w3x")));
  const decoder = new TextDecoder();
  assertEquals(decoder.decode(await archive.read("war3mapImported\\golems\\frames.toc")), "toc from the library");
  assertStringIncludes(decoder.decode(await archive.read("war3map.lua")), '__mw.define("golems.names"');
  // The source map is never written by a build.
  const source = join(project, "maps", "map.w3x");
  assertEquals(await Deno.stat(join(source, "war3mapImported")).then(() => true, () => false), false);
});
