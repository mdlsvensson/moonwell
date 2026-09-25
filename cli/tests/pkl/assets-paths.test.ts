import { assertEquals, assertStringIncludes } from "@std/assert";
import { dirname, join } from "@std/path";
import { assetsPaths } from "../../src/commands/assets-paths.ts";
import { init } from "../../src/commands/init.ts";
import { createContext } from "../../src/context.ts";
import { silentLogger } from "../support/logger.ts";
import { chunk, concat, emitter, mdx, texture } from "../support/mdx.ts";

async function put(root: string, file: string, bytes: Uint8Array): Promise<void> {
  const path = join(root, ...file.split("/"));
  await Deno.mkdir(dirname(path), { recursive: true });
  await Deno.writeFile(path, bytes);
}

Deno.test("assets:paths marks references found through targets, mappings and .mdl/.mdx", async () => {
  const parent = await Deno.makeTempDir({ prefix: "moonwell-paths-" });
  const root = await init(join(parent, "my-map"), createContext(parent, silentLogger()), { link: true });
  await put(
    root,
    "assets/Models/Knight.mdx",
    mdx(
      chunk(
        "TEXS",
        concat(
          texture("Textures\\Knight.blp"),
          texture("Textures\\Cape.blp"),
          texture("Textures\\Missing.blp"),
          texture("", 1),
        ),
      ),
      chunk("PREM", concat(emitter("Models\\Glow.mdl"), emitter("Models\\Only.mdx"))),
    ),
  );
  await put(root, "assets/Textures/knight.BLP", new Uint8Array([1])); // letter case must not matter
  await put(root, "assets/art/cape.blp", new Uint8Array([2]));
  await put(root, "assets/Models/Glow.mdx", mdx());
  // An imported .mdl satisfies nothing: the game swaps a requested .mdl for the .mdx, never the other way.
  await put(root, "assets/Models/Only.mdl", new TextEncoder().encode("Version {\n\tFormatVersion 800,\n}\n"));
  const manifest = join(root, "moonwell.pkl");
  const text = await Deno.readTextFile(manifest);
  assertStringIncludes(text, "paths {}");
  await Deno.writeTextFile(
    manifest,
    text.replace("paths {}", String.raw`paths { ["art/cape.blp"] = #"Textures\Cape.blp"# }`),
  );

  const logger = silentLogger();
  const ctx = { ...createContext(root, logger), logger };
  const [knight] = await assetsPaths(ctx, "assets/Models/Knight.mdx");
  assertEquals(knight.heading, "assets/Models/Knight.mdx");
  assertEquals(knight.refs.map((ref) => [ref.path, ref.found]), [
    ["Textures\\Knight.blp", true],
    ["Textures\\Cape.blp", true],
    ["Textures\\Missing.blp", false],
    [null, undefined],
    ["Models\\Glow.mdl", true],
    ["Models\\Only.mdx", false],
  ]);
  assertEquals(
    logger.lines.at(-1),
    "1 model, 6 paths: 3 found in assets/, 2 not found (built-in game files or missing imports).",
  );

  const all = await assetsPaths(ctx);
  assertEquals(all.map((report) => report.heading), [
    "assets/Models/Glow.mdx",
    "assets/Models/Knight.mdx",
    "assets/Models/Only.mdl",
  ]);
  assertEquals(all[0].refs, []);
  assertEquals(all[2].refs, []);
  assertStringIncludes(logger.lines.join("\n"), "  (no referenced files)");
});
