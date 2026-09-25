import { assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { join } from "@std/path";
import { assetsPaths } from "../../src/commands/assets-paths.ts";
import { createContext } from "../../src/context.ts";
import { parseGamePaths } from "../../src/models/game-paths.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";
import { chunk, concat, emitter, mdx, texture } from "../support/mdx.ts";

async function outside() {
  const root = await Deno.makeTempDir({ prefix: "moonwell-paths-" });
  const logger = silentLogger();
  return { root, logger, ctx: { ...createContext(root, logger), logger } };
}

Deno.test("outside a project, assets:paths tells in-game paths from custom ones, shown with backslashes", async () => {
  const { root, logger, ctx } = await outside();
  await Deno.writeFile(
    join(root, "knight.mdx"),
    mdx(
      chunk("TEXS", concat(texture("Textures/Knight.blp"), texture("", 1))),
      chunk("PREM", emitter("Abilities\\Heal.mdx")),
    ),
  );
  const gamePaths = parseGamePaths("# test\ntextures/knight.dds\n");
  const reports = await assetsPaths(ctx, "knight.mdx", { gamePaths });
  assertEquals(reports[0].refs.map((ref) => ref.status), ["in-game path", undefined, "custom path"]);
  const width = "team colour (slot 1)".length;
  assertEquals(logger.lines, [
    "knight.mdx",
    `  ${"texture".padEnd(14)}  ${"Textures\\Knight.blp".padEnd(width)}  in-game path`,
    `  ${"texture".padEnd(14)}  team colour (slot 1)`,
    `  ${"particle model".padEnd(14)}  ${"Abilities\\Heal.mdx".padEnd(width)}  custom path`,
    "1 model, 3 paths: 1 in-game, 1 custom.",
  ]);
});

Deno.test("a Reforged .tif reference matches the game's .dds", async () => {
  const { root, ctx } = await outside();
  await Deno.writeFile(join(root, "grass.mdx"), mdx(chunk("TEXS", texture("Doodads/Corn/plant1_Normal.tif"))));
  const gamePaths = parseGamePaths("doodads/corn/plant1_normal.dds\n");
  const [grass] = await assetsPaths(ctx, "grass.mdx", { gamePaths });
  assertEquals(grass.refs[0].status, "in-game path");
});

Deno.test("an empty in-game path list is announced", async () => {
  const { root, logger, ctx } = await outside();
  await Deno.writeFile(join(root, "a.mdx"), mdx(chunk("TEXS", texture("Textures\\A.blp"))));
  await assetsPaths(ctx, "a.mdx", { gamePaths: new Set() });
  assertEquals(logger.lines[0], "warning: Moonwell's in-game path list is empty, so every path shows as custom.");
});

Deno.test("outside a project, assets:paths needs a file, and the file must exist", async () => {
  const { ctx } = await outside();
  await assertRejects(() => assetsPaths(ctx), MoonwellError, "needs a model file");
  const missing = await assertRejects(() => assetsPaths(ctx, "missing.mdx"), MoonwellError, "does not exist");
  assertStringIncludes(missing.hint ?? "", "relative to the project folder");
});

Deno.test("assets:paths reports a folder passed as the model as a user error", async () => {
  const { ctx } = await outside();
  await assertRejects(() => assetsPaths(ctx, "."), MoonwellError, "is a folder, not a model file");
});
