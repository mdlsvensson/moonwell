import { assert, assertEquals } from "@std/assert";
import { decodeBase64 } from "@std/encoding/base64";
import { join } from "@std/path";
import { GAME_PATHS_GZIP_BASE64 } from "../../src/embedded/game-paths.ts";
import { METADATA_GZIP_BASE64 } from "../../src/embedded/metadata.ts";
import { NATIVES_GZIP_BASE64 } from "../../src/embedded/natives.ts";
import { TEMPLATE_FILES } from "../../src/embedded/template.ts";
import { gunzip } from "../../src/shared/compression.ts";
import { renderEmbedded, renderGeneratedSchema, REPO, templateEntries } from "../../../tools/gen.ts";

Deno.test("the embedded runtime is up to date (run `deno task gen`)", async () => {
  const path = "cli/src/embedded/runtime.ts";
  const expected = (await renderEmbedded()).get(path);
  // A plain boolean: on a mismatch, printing both copies of the runtime would bury the hint.
  assert(await Deno.readTextFile(join(REPO, path)) === expected, `${path} is stale: run \`deno task gen\`.`);
});

Deno.test("the embedded macro module is up to date (run `deno task gen`)", async () => {
  const path = "cli/src/embedded/macros.ts";
  const expected = (await renderEmbedded()).get(path);
  assert(await Deno.readTextFile(join(REPO, path)) === expected, `${path} is stale: run \`deno task gen\`.`);
});

Deno.test("the embedded template matches template/ (run `deno task gen`)", async () => {
  const embedded = new Map(TEMPLATE_FILES.map((file) => [file.path, file.base64]));
  const actual = new Map((await templateEntries()).map((file) => [file.path, file.base64]));
  const differences = [
    ...[...actual.keys()].filter((path) => !embedded.has(path)).map((path) => `added:   template/${path}`),
    ...[...embedded.keys()].filter((path) => !actual.has(path)).map((path) => `removed: template/${path}`),
    ...[...actual].filter(([path, base64]) => embedded.has(path) && embedded.get(path) !== base64)
      .map(([path]) => `changed: template/${path}`),
  ];
  assertEquals(
    differences,
    [],
    "cli/src/embedded/template.ts does not match template/. Run `deno task gen` if these changes belong in the " +
      "template; otherwise remove them.",
  );
});

Deno.test("templateEntries skips what a project generates: dist/, .moonwell/ and src/**/*.lua", async () => {
  const repo = await Deno.makeTempDir({ prefix: "moonwell-template-" });
  const files = [
    "template/.moonwell/types/x.d.lua",
    "template/src/main.lua",
    "template/src/main.yue",
    "template/dist/a",
  ];
  for (const path of files) {
    const target = join(repo, ...path.split("/"));
    await Deno.mkdir(join(target, ".."), { recursive: true });
    await Deno.writeTextFile(target, "x\n");
  }
  assertEquals((await templateEntries(repo)).map((file) => file.path), ["src/main.yue"]);
});

Deno.test("the embedded in-game path list matches cli/data/game-paths.txt (run `deno task gen`)", async () => {
  const embedded = new TextDecoder().decode(await gunzip(decodeBase64(GAME_PATHS_GZIP_BASE64)));
  const source = await Deno.readTextFile(join(REPO, "cli", "data", "game-paths.txt"));
  assert(embedded === source, "cli/src/embedded/game-paths.ts is stale: run `deno task gen`.");
});

Deno.test("the embedded object metadata matches cli/data/metadata.json (run `deno task gen`)", async () => {
  const embedded = new TextDecoder().decode(await gunzip(decodeBase64(METADATA_GZIP_BASE64)));
  const source = await Deno.readTextFile(join(REPO, "cli", "data", "metadata.json"));
  assert(embedded === source, "cli/src/embedded/metadata.ts is stale: run `deno task gen`.");
});

Deno.test("the embedded natives match cli/data/natives.json (run `deno task gen`)", async () => {
  const embedded = new TextDecoder().decode(await gunzip(decodeBase64(NATIVES_GZIP_BASE64)));
  const source = await Deno.readTextFile(join(REPO, "cli", "data", "natives.json"));
  assert(embedded === source, "cli/src/embedded/natives.ts is stale: run `deno task gen`.");
});

Deno.test("schema/generated matches cli/data/metadata.json (run `deno task gen`)", async () => {
  const expected = await renderGeneratedSchema();
  const folder = join(REPO, "schema", "generated");
  const actual = new Map<string, string>();
  for await (const entry of Deno.readDir(folder)) {
    actual.set(`schema/generated/${entry.name}`, await Deno.readTextFile(join(folder, entry.name)));
  }
  const differences = [
    ...[...actual.keys()].filter((path) => !expected.has(path)).map((path) => `stray:   ${path}`),
    ...[...expected.keys()].filter((path) => !actual.has(path)).map((path) => `missing: ${path}`),
    ...[...expected].filter(([path, text]) => actual.has(path) && actual.get(path) !== text).map(([path]) =>
      `stale:   ${path}`
    ),
  ].sort();
  assertEquals(differences, [], "schema/generated is out of date: run `deno task gen`.");
});

Deno.test("the template's .gitattributes never converts line endings in maps/ or assets/", () => {
  // Both are copied into the map byte for byte; a normalized CRLF file would build differently on each checkout.
  const file = TEMPLATE_FILES.find((entry) => entry.path === ".gitattributes");
  assert(file !== undefined, "template/.gitattributes is missing");
  const lines = new TextDecoder().decode(decodeBase64(file.base64)).split("\n");
  assert(lines.includes("maps/** binary"), "maps/** must be binary");
  assert(lines.includes("assets/** -text"), "assets/** must be -text");
});
