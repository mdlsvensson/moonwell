import { assertEquals, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { readImports } from "../../src/assets/imports.ts";
import { assetsPaths } from "../../src/commands/assets-paths.ts";
import { assets } from "../../src/commands/assets.ts";
import { init } from "../../src/commands/init.ts";
import { createContext } from "../../src/context.ts";
import { parseGamePaths } from "../../src/models/game-paths.ts";
import { silentLogger } from "../support/logger.ts";
import { chunk, mdx, texture } from "../support/mdx.ts";

const text = (value: string) => new TextEncoder().encode(value);

async function put(root: string, file: string, bytes: Uint8Array): Promise<void> {
  const path = join(root, ...file.split("/"));
  await Deno.mkdir(dirname(path), { recursive: true });
  await Deno.writeFile(path, bytes);
}

/** A fresh project and, beside it, a local library that ships a model, its texture and a frame template list. */
async function projectWithLibrary(): Promise<{ root: string; library: string; manifest: string; plain: string }> {
  const parent = await Deno.makeTempDir({ prefix: "moonwell-libassets-" });
  const root = await init(join(parent, "my-map"), createContext(parent, silentLogger()), { link: true });
  const library = join(parent, "golems");
  await put(library, "moonwell-library.json", text('{"dir":"src","assets":"assets"}'));
  await put(library, "src/golems/spawn.lua", text("return {}\n"));
  await put(library, "assets/Models/Golem.mdx", mdx(chunk("TEXS", texture("Textures\\Golem.blp"))));
  await put(library, "assets/Textures/Golem.blp", new Uint8Array([1]));
  await put(library, "assets/war3mapImported/golems/frames.toc", text("war3mapImported\\golems\\frames.fdf\n"));
  const manifest = join(root, "moonwell.pkl");
  const plain = await Deno.readTextFile(manifest);
  assertStringIncludes(plain, "libraries {\n");
  await Deno.writeTextFile(
    manifest,
    plain.replace("libraries {\n", `libraries {\n  ["golems"] { path = "${library.replaceAll("\\", "/")}" }\n`),
  );
  return { root, library, manifest, plain };
}

Deno.test("assets:check lists a library's files; assets:sync writes them, and removes them with the library", async () => {
  const { root, manifest, plain } = await projectWithLibrary();
  await put(root, "assets/Textures/golem.blp", new Uint8Array([2])); // the map's own, in another letter case
  const logger = silentLogger();
  const ctx = createContext(root, logger);
  const map = join(root, "maps", "map.w3x");

  const checked = await assets(ctx, "check");
  assertEquals(checked.assets.map((asset) => [asset.library, asset.target]), [
    ["golems", "Models/Golem.mdx"],
    [undefined, "Textures/golem.blp"],
    ["golems", "war3mapImported/golems/frames.toc"],
  ]);
  assertEquals(logger.lines.filter((line) => line.includes(" -> ") || line.includes("replaces")), [
    "library golems: Models/Golem.mdx -> Models\\Golem.mdx",
    "Textures/golem.blp -> Textures\\golem.blp",
    "library golems: war3mapImported/golems/frames.toc -> war3mapImported\\golems\\frames.toc",
    "assets/Textures/golem.blp replaces library golems's Textures/Golem.blp",
  ]);
  assertEquals(await exists(join(map, "Models")), false, "check writes nothing into the map");
  assertEquals(await exists(join(root, ".moonwell", "library-assets", "golems", "Models", "Golem.mdx")), true);

  await assets(ctx, "sync");
  assertEquals(await Deno.readFile(join(map, "Textures", "golem.blp")), new Uint8Array([2]));
  assertEquals(
    await Deno.readTextFile(join(map, "war3mapImported", "golems", "frames.toc")),
    "war3mapImported\\golems\\frames.fdf\n",
  );
  const imported = () => Deno.readFile(join(map, "war3map.imp")).then((bytes) => readImports(bytes).map((e) => e.path));
  assertEquals((await imported()).sort(), [
    "Models\\Golem.mdx",
    "Textures\\golem.blp",
    "war3mapImported\\golems\\frames.toc",
  ]);

  await Deno.writeTextFile(manifest, plain);
  await assets(ctx, "sync");
  assertEquals(await exists(join(map, "Models", "Golem.mdx")), false);
  assertEquals(await exists(join(map, "war3mapImported", "golems", "frames.toc")), false);
  assertEquals(await imported(), ["Textures\\golem.blp"]);
  assertEquals(await exists(join(root, ".moonwell", "library-assets", "golems")), false);
});

Deno.test("assets:paths counts a library's files as imported and checks its models", async () => {
  const { root } = await projectWithLibrary();
  await put(root, "assets/Models/Own.mdx", mdx(chunk("TEXS", texture("textures\\golem.BLP"))));
  const logger = silentLogger();
  const ctx = { ...createContext(root, logger), logger };
  const reports = await assetsPaths(ctx, undefined, { gamePaths: parseGamePaths("# test\n") });
  assertEquals(reports.map((report) => [report.heading, report.refs.map((ref) => ref.status)]), [
    ["library golems: Models/Golem.mdx", ["custom path, imported"]],
    ["assets/Models/Own.mdx", ["custom path, imported"]],
  ]);
});
