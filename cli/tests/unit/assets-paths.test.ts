import { assertEquals, assertRejects } from "@std/assert";
import { join } from "@std/path";
import { assetsPaths } from "../../src/commands/assets-paths.ts";
import { createContext } from "../../src/context.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";
import { chunk, concat, emitter, mdx, texture } from "../support/mdx.ts";

async function outside() {
  const root = await Deno.makeTempDir({ prefix: "moonwell-paths-" });
  const logger = silentLogger();
  return { root, logger, ctx: { ...createContext(root, logger), logger } };
}

Deno.test("outside a project, assets:paths lists a model's paths without a found column", async () => {
  const { root, logger, ctx } = await outside();
  await Deno.writeFile(
    join(root, "knight.mdx"),
    mdx(
      chunk("TEXS", concat(texture("Textures\\Knight.blp"), texture("", 1))),
      chunk("PREM", emitter("Abilities\\Heal.mdx")),
    ),
  );
  const reports = await assetsPaths(ctx, "knight.mdx");
  assertEquals(reports.length, 1);
  assertEquals(reports[0].heading, "knight.mdx");
  assertEquals(reports[0].refs.map((ref) => ref.found), [undefined, undefined, undefined]);
  assertEquals(logger.lines, [
    "knight.mdx",
    `  ${"texture".padEnd(14)}  Textures\\Knight.blp`,
    `  ${"texture".padEnd(14)}  team colour (slot 1)`,
    `  ${"particle model".padEnd(14)}  Abilities\\Heal.mdx`,
    "1 model, 3 paths.",
  ]);
});

Deno.test("outside a project, assets:paths needs a file, and the file must exist", async () => {
  const { ctx } = await outside();
  await assertRejects(() => assetsPaths(ctx), MoonwellError, "needs a model file");
  await assertRejects(() => assetsPaths(ctx, "missing.mdx"), MoonwellError, "does not exist");
});
