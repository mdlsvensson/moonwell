import { assert, assertEquals, assertRejects } from "@std/assert";
import { copy, exists } from "@std/fs";
import { dirname, join, relative } from "@std/path";
import { readImports, writeImports } from "../../src/assets/imports.ts";
import { applyAssetPlan, assetLocations, planAssets } from "../../src/assets/plan.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

const defaults = { paths: {}, exclude: [] };

async function put(root: string, file: string, value = "asset"): Promise<void> {
  const destination = join(root, ...file.split("/"));
  await Deno.mkdir(dirname(destination), { recursive: true });
  await Deno.writeTextFile(destination, value);
}

async function fixture(): Promise<{ root: string; map: string; state: string }> {
  const root = await Deno.makeTempDir({ prefix: "moonwell-assets-" });
  const { mapDir, stateFile } = await assetLocations(root, "map.w3x");
  await Deno.mkdir(mapDir, { recursive: true });
  await Deno.mkdir(join(root, "assets"));
  return { root, map: mapDir, state: stateFile };
}

const entries = async (map: string) => readImports(await Deno.readFile(join(map, "war3map.imp")));
const text = (map: string, file: string) => Deno.readTextFile(join(map, ...file.split("/")));

Deno.test("assetLocations places the state file under .asset-state/", async () => {
  const root = await Deno.makeTempDir();
  const { mapDir, stateFile } = await assetLocations(root, "map.w3x");
  assertEquals(mapDir, join(root, "maps", "map.w3x"));
  assertEquals(stateFile, join(root, ".asset-state", "map.w3x.json"));
});

Deno.test("sync imports mapped assets and keeps the editor's own imports", async () => {
  const { root, map, state } = await fixture();
  const icon = "ReplaceableTextures/CommandButtons/BTNSword.blp";
  const disabled = "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp";
  await put(root, `assets/${icon}`);
  await put(root, "assets/icons/disabled.blp", "disabled");
  await put(map, "war3mapImported/existing.wav", "editor");
  await Deno.writeFile(join(map, "war3map.imp"), writeImports([{ flag: 5, path: "existing.wav" }]));
  const config = { paths: { "icons/disabled.blp": disabled }, exclude: [] };

  const plan = await planAssets(root, map, state, config);
  assertEquals(plan.assets.length, 2);
  assertEquals(await exists(state), false, "planning must not write");
  await applyAssetPlan(plan, state);

  assertEquals(await entries(map), [
    { flag: 5, path: "existing.wav" },
    ...plan.assets.map((asset) => ({ flag: 13, path: asset.target.replaceAll("/", "\\") })),
  ]);
  assertEquals(await text(map, disabled), "disabled");
  assertEquals((await planAssets(root, map, state, config)).changes.length, 0, "a second sync changes nothing");
});

Deno.test("sync updates, renames and deletes only owned files; a staged copy leaves source state alone", async () => {
  const { root, map, state } = await fixture();
  await put(root, "assets/Models/unit.mdx", "first");
  await put(map, "unmanaged.txt", "keep");
  await applyAssetPlan(await planAssets(root, map, state, defaults), state);
  const previousState = await Deno.readTextFile(state);

  await put(root, "assets/Models/unit.mdx", "second");
  const staged = join(root, "stage");
  await copy(map, staged);
  await applyAssetPlan(await planAssets(root, staged, state, defaults));
  assertEquals(await text(staged, "Models/unit.mdx"), "second");
  assertEquals(await text(map, "Models/unit.mdx"), "first");
  assertEquals(await Deno.readTextFile(state), previousState);

  await applyAssetPlan(await planAssets(root, map, state, defaults), state);
  const renamed = { paths: { "Models/unit.mdx": "Models/renamed.mdx" }, exclude: [] };
  await applyAssetPlan(await planAssets(root, map, state, renamed), state);
  assertEquals(await exists(join(map, "Models", "unit.mdx")), false);
  assertEquals(await text(map, "Models/renamed.mdx"), "second");

  await Deno.remove(join(root, "assets", "Models", "unit.mdx"));
  await applyAssetPlan(await planAssets(root, map, state, defaults), state);
  assertEquals(await entries(map), []);
  assertEquals(await text(map, "unmanaged.txt"), "keep");
});

Deno.test("conflicts and edited owned files fail before anything changes", async () => {
  const { root, map, state } = await fixture();
  await put(root, "assets/a.blp");
  await put(map, "a.blp", "editor owned");
  await assertRejects(() => planAssets(root, map, state, defaults), MoonwellError, "conflicts");
  assertEquals(await text(map, "a.blp"), "editor owned");

  await Deno.remove(join(map, "a.blp"));
  await applyAssetPlan(await planAssets(root, map, state, defaults), state);
  await put(map, "a.blp", "manual edit");
  await assertRejects(() => planAssets(root, map, state, defaults), MoonwellError, "modified");
  await Deno.remove(join(root, "assets", "a.blp"));
  await assertRejects(() => planAssets(root, map, state, defaults), MoonwellError, "modified");
  assertEquals(await text(map, "a.blp"), "manual edit");
});

Deno.test("an asset whose folder is a file in the map is rejected", async () => {
  const { root, map, state } = await fixture();
  await put(root, "assets/a.blp");
  await put(map, "Textures", "file");
  await assertRejects(
    () => planAssets(root, map, state, { paths: { "a.blp": "textures/a.blp" }, exclude: [] }),
    MoonwellError,
    "not a directory",
  );
});

Deno.test("existing folder spelling is reused and forged state cannot target map internals", async () => {
  const { root, map, state } = await fixture();
  await put(map, "Textures/existing.blp");
  await put(root, "assets/textures/new.blp");
  const plan = await planAssets(root, map, state, defaults);
  assertEquals(plan.assets[0].target, "Textures/new.blp");

  await put(
    root,
    relative(root, state).replaceAll("\\", "/"),
    JSON.stringify({
      version: 1,
      files: { "war3map.lua": "0".repeat(64) },
    }),
  );
  await assertRejects(() => planAssets(root, map, state, defaults), MoonwellError, "Reserved");
});

Deno.test("new folders planned in one run share one spelling", async () => {
  const { root, map, state } = await fixture();
  await put(root, "assets/a.blp");
  await put(root, "assets/b.blp");
  const plan = await planAssets(root, map, state, {
    paths: { "a.blp": "Textures/a.blp", "b.blp": "textures/b.blp" },
    exclude: [],
  });
  assertEquals(plan.assets.map((asset) => asset.target), ["Textures/a.blp", "Textures/b.blp"]);
});

Deno.test("a failed sync undoes the writes it already made", async () => {
  const { root, map, state } = await fixture();
  await put(root, "assets/a.blp");
  await put(root, "assets/b.blp");
  const plan = await planAssets(root, map, state, defaults);
  // b.blp turning into a folder after planning makes the second write fail.
  await Deno.mkdir(join(map, "b.blp"));
  await assertRejects(() => applyAssetPlan(plan, state), MoonwellError);
  assertEquals(await exists(join(map, "a.blp")), false);
  assertEquals(await exists(join(map, "war3map.imp")), false);
  assertEquals(await exists(state), false);
  assert((await Deno.stat(join(map, "b.blp"))).isDirectory);
});

Deno.test("with no assets and nothing owned, war3map.imp is left untouched", async () => {
  const { root, map, state } = await fixture();
  const plan = await planAssets(root, map, state, defaults);
  assertEquals(plan.changes, []);
  assertEquals(await exists(join(map, "war3map.imp")), false);
});
