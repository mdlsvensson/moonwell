import { assertEquals, assertRejects, assertThrows } from "@std/assert";
import { dirname, join } from "@std/path";
import { collectAssets } from "../../src/assets/collect.ts";
import { assetPath, pathKey, safeJoin, scanFiles, targetPath } from "../../src/assets/paths.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

const defaults = { paths: {}, exclude: [] };

async function put(root: string, file: string, value = "asset"): Promise<void> {
  const destination = join(root, ...file.split("/"));
  await Deno.mkdir(dirname(destination), { recursive: true });
  await Deno.writeTextFile(destination, value);
}

async function projectRoot(): Promise<string> {
  const root = await Deno.makeTempDir({ prefix: "moonwell-assets-" });
  await Deno.mkdir(join(root, "assets"));
  return root;
}

Deno.test("assetPath normalizes separators and rejects unsafe paths", () => {
  assertEquals(assetPath("Textures\\a.blp"), "Textures/a.blp");
  for (
    const bad of ["", "../escape", "/absolute", "C:\\escape", "a//b", "bad.", "trailing ", "CON.blp", "a/./b", "a|b"]
  ) {
    assertThrows(() => assetPath(bad), MoonwellError, "path");
  }
  assertEquals(pathKey("Textures\\A.BLP"), "textures/a.blp");
});

Deno.test("targetPath rejects map internals but allows war3mapImported", () => {
  for (const reserved of ["war3map.lua", "war3map.imp", "WAR3MAP.W3I", "scripts/war3map.j", "(listfile)"]) {
    assertThrows(() => targetPath(reserved), MoonwellError, "Reserved");
  }
  assertEquals(targetPath("war3mapImported/sound.wav"), "war3mapImported/sound.wav");
});

Deno.test("collectAssets maps, excludes, skips dotfiles and sorts by target", async () => {
  const root = await projectRoot();
  await put(root, "assets/ReplaceableTextures/CommandButtons/BTNSword.blp");
  await put(root, "assets/icons/disabled.blp", "disabled");
  await put(root, "assets/credits/readme.txt");
  await put(root, "assets/.gitkeep");
  const disabled = "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp";
  const assets = await collectAssets(root, { paths: { "icons/disabled.blp": disabled }, exclude: ["credits/"] });
  assertEquals(assets.map((asset) => [asset.source, asset.target]), [
    ["ReplaceableTextures/CommandButtons/BTNSword.blp", "ReplaceableTextures/CommandButtons/BTNSword.blp"],
    ["icons/disabled.blp", disabled],
  ]);
  assertEquals(new TextDecoder().decode(assets[1].bytes), "disabled");
  assertEquals(assets[1].hash.length, 64);
});

Deno.test("collectAssets returns nothing when assets/ is missing", async () => {
  const root = await Deno.makeTempDir();
  assertEquals(await collectAssets(root, defaults), []);
});

Deno.test("collectAssets rejects bad mappings, collisions and reserved targets", async () => {
  const root = await projectRoot();
  await put(root, "assets/a.blp");
  await put(root, "assets/b.blp");
  for (const target of ["../escape", "/absolute", "C:\\escape", "war3map.lua", "war3map.imp", "scripts/war3map.j"]) {
    await assertRejects(() => collectAssets(root, { paths: { "a.blp": target }, exclude: [] }), MoonwellError, "path");
  }
  await assertRejects(
    () => collectAssets(root, { paths: { missing: "x.blp" }, exclude: [] }),
    MoonwellError,
    "does not exist",
  );
  await assertRejects(
    () => collectAssets(root, { paths: { "a.blp": "X.blp", "b.blp": "x.blp" }, exclude: [] }),
    MoonwellError,
    "collision",
  );
  await assertRejects(
    () => collectAssets(root, { paths: { "a.blp": "x", "b.blp": "x/y" }, exclude: [] }),
    MoonwellError,
    "collision",
  );
  await assertRejects(
    () => collectAssets(root, { paths: { "a.blp": "x" }, exclude: ["a.blp"] }),
    MoonwellError,
    "excluded",
  );
});

Deno.test("scanFiles rejects case collisions and symlinked folders", async () => {
  const root = await projectRoot();
  await put(root, "assets/a.blp");
  if (Deno.build.os !== "windows") {
    // Windows folders are case-insensitive, so two such names cannot exist there.
    await put(root, "assets/A.blp");
    await assertRejects(() => scanFiles(join(root, "assets")), MoonwellError, "letter case");
    await Deno.remove(join(root, "assets", "A.blp"));
  }
  const external = join(root, "external");
  await Deno.mkdir(external);
  await Deno.symlink(external, join(root, "assets", "linked"), {
    type: Deno.build.os === "windows" ? "junction" : "dir",
  });
  await assertRejects(() => collectAssets(root, defaults), MoonwellError, "Symlinks");
});

Deno.test("safeJoin accepts a root that is itself a link but rejects a link below it", async () => {
  const parent = await projectRoot();
  const real = join(parent, "real");
  await put(real, "assets/a.blp");
  const type = Deno.build.os === "windows" ? "junction" : "dir";
  const linkedRoot = join(parent, "linked-root");
  await Deno.symlink(real, linkedRoot, { type });
  assertEquals(await safeJoin(linkedRoot, "assets/a.blp"), join(linkedRoot, "assets", "a.blp"));
  assertEquals((await collectAssets(linkedRoot, defaults)).map((asset) => asset.target), ["a.blp"]);

  await Deno.symlink(join(parent, "assets"), join(real, "inner"), { type });
  await assertRejects(() => safeJoin(linkedRoot, "inner/x.blp"), MoonwellError, "Symlinks");
});
