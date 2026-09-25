import { assertEquals } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { readImports } from "../../src/assets/imports.ts";
import { assets } from "../../src/commands/assets.ts";
import { init } from "../../src/commands/init.ts";
import { createContext } from "../../src/context.ts";
import { silentLogger } from "../support/logger.ts";

async function project(): Promise<string> {
  const parent = await Deno.makeTempDir({ prefix: "moonwell-assets-" });
  return await init(join(parent, "my-map"), createContext(parent, silentLogger()), { link: true });
}

Deno.test("assets:check plans without writing; assets:sync writes the source map and ownership state", async () => {
  const root = await project();
  await Deno.mkdir(join(root, "assets", "icons"), { recursive: true });
  await Deno.writeTextFile(join(root, "assets", "icons", "a.blp"), "icon");
  const logger = silentLogger();
  const ctx = createContext(root, logger);
  const map = join(root, "maps", "map.w3x");

  const checked = await assets(ctx, "check");
  assertEquals(checked.assets.map((asset) => asset.target), ["icons/a.blp"]);
  assertEquals(logger.lines.filter((line) => /^(write|delete) /.test(line)), [
    "write maps/map.w3x/icons/a.blp",
    "write maps/map.w3x/war3map.imp",
  ]);
  assertEquals(await exists(join(map, "icons", "a.blp")), false);
  assertEquals(await exists(join(root, ".asset-state")), false);

  await assets(ctx, "sync");
  assertEquals(await Deno.readTextFile(join(map, "icons", "a.blp")), "icon");
  const imports = readImports(await Deno.readFile(join(map, "war3map.imp")));
  assertEquals(imports.filter((entry) => entry.path === "icons\\a.blp").length, 1);
  const state = JSON.parse(await Deno.readTextFile(join(root, ".asset-state", "map.w3x.json")));
  assertEquals(Object.keys(state.files), ["icons/a.blp"]);

  assertEquals((await assets(ctx, "check")).changes, [], "after a sync there is nothing left to do");
});
