import { assertEquals, assertRejects, assertStringIncludes, assertThrows } from "@std/assert";
import { dirname, join } from "@std/path";
import { collectAssets, collectProjectAssets } from "../../src/assets/collect.ts";
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
  // Reforged ignores war3mapPreview.tga (the map list shows war3mapMap.blp), so it stays reserved like the rest.
  for (
    const reserved of [
      "war3map.lua",
      "war3map.imp",
      "WAR3MAP.W3I",
      "scripts/war3map.j",
      "(listfile)",
      "war3mapMap.blp",
      "war3mapPreview.tga",
    ]
  ) {
    assertThrows(() => targetPath(reserved), MoonwellError, "Reserved");
  }
  // The names tried for a map list picture point at the setting that does it; other internals do not.
  const hint = (path: string) => assertThrows(() => targetPath(path), MoonwellError).hint!;
  for (const picture of ["war3mapPreview.tga", "WAR3MAPPREVIEW.BLP", "war3mapMap.blp", "war3mapMap.tga"]) {
    assertStringIncludes(hint(picture), "settings.info.preview");
  }
  for (const other of ["war3map.lua", "war3mapMisc.txt", "scripts/war3map.j", "war3mapPreviews/a.tga"]) {
    assertEquals(hint(other), "Assets cannot replace map internals such as war3map.lua or war3map.imp.");
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

Deno.test("library assets follow the map's own, by key, and import at their path in the library", async () => {
  const root = await projectRoot();
  await put(root, "assets/Models/Own.mdx", "own");
  await put(root, ".moonwell/library-assets/zeta/Sounds/Horn.wav", "horn");
  await put(root, ".moonwell/library-assets/alpha/war3mapImported/alpha/frames.toc", "toc");
  await put(root, ".moonwell/library-assets/alpha/.hidden/x.txt", "hidden");
  await put(root, ".moonwell/library-assets/other/never.txt", "not in the manifest");
  const { assets, replaced } = await collectProjectAssets(root, defaults, ["zeta", "alpha", "none"]);
  assertEquals(assets.map((asset) => [asset.library, asset.source, asset.target]), [
    [undefined, "Models/Own.mdx", "Models/Own.mdx"],
    ["zeta", "Sounds/Horn.wav", "Sounds/Horn.wav"],
    ["alpha", "war3mapImported/alpha/frames.toc", "war3mapImported/alpha/frames.toc"],
  ]);
  assertEquals(new TextDecoder().decode(assets[1].bytes), "horn");
  assertEquals(replaced, []);
  assertEquals(await collectProjectAssets(root, defaults, []), { assets: [assets[0]], replaced: [] });
});

Deno.test("the map's own asset replaces a library's at the same in-map path, in any letter case", async () => {
  const root = await projectRoot();
  await put(root, "assets/custom/golem.blp", "the map's");
  await put(root, "assets/Models/own.mdx", "the map's model");
  await put(root, ".moonwell/library-assets/lib/Textures/Golem.blp", "the library's");
  await put(root, ".moonwell/library-assets/lib/models/Own.mdx", "the library's model");
  await put(root, ".moonwell/library-assets/lib/Textures/Other.blp", "kept");
  const config = { paths: { "custom/golem.blp": "textures/golem.blp" }, exclude: [] };
  const { assets, replaced } = await collectProjectAssets(root, config, ["lib"]);
  assertEquals(assets.map((asset) => [asset.library, asset.target]), [
    [undefined, "Models/own.mdx"],
    [undefined, "textures/golem.blp"],
    ["lib", "Textures/Other.blp"],
  ]);
  assertEquals(replaced, [
    "assets/custom/golem.blp replaces library lib's Textures/Golem.blp",
    "assets/Models/own.mdx replaces library lib's models/Own.mdx",
  ]);
});

Deno.test("two libraries at one in-map path fail, unless the map's own file replaces both", async () => {
  const root = await projectRoot();
  await put(root, ".moonwell/library-assets/a/UI/Frame.fdf", "a");
  await put(root, ".moonwell/library-assets/b/ui/frame.fdf", "b");
  const error = await assertRejects(() => collectProjectAssets(root, defaults, ["b", "a"]), MoonwellError);
  assertEquals(error.message, "Libraries a and b both import ui\\frame.fdf.");
  assertEquals(error.file, "moonwell.pkl");
  assertEquals(
    error.hint,
    "Drop one of the libraries, or put your own file at that path under assets/ to replace both.",
  );
  await put(root, "assets/UI/Frame.fdf", "the map's");
  const { assets, replaced } = await collectProjectAssets(root, defaults, ["b", "a"]);
  assertEquals(assets.map((asset) => asset.library), [undefined]);
  assertEquals(replaced.length, 2);
});

Deno.test("a library file with a reserved path, or inside another asset file, is refused", async () => {
  const reserved = await projectRoot();
  await put(reserved, ".moonwell/library-assets/bad/war3map.lua", "script");
  const error = await assertRejects(() => collectProjectAssets(reserved, defaults, ["bad"]), MoonwellError);
  assertEquals(error.message, "Library bad: Reserved map path: war3map.lua");
  assertEquals(error.file, ".moonwell/library-assets/bad");
  assertEquals(error.hint, "Report it to the library's author, or use another version of the library.");

  const nested = await projectRoot();
  await put(nested, "assets/data", "a file");
  await put(nested, ".moonwell/library-assets/lib/data/inner.txt", "inside it");
  await assertRejects(() => collectProjectAssets(nested, defaults, ["lib"]), MoonwellError, "file/folder collision");
});
