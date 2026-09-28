import { assertEquals, assertRejects, assertStringIncludes, assertThrows } from "@std/assert";
import { join } from "@std/path";
import { MoonwellError } from "../../src/shared/errors.ts";
import { validateMapSettings } from "../../src/settings/options.ts";
import { emptyObjects, SCHEMA_HINT } from "../../src/objectdata/manifest.ts";
import type { Runner } from "../../src/shared/process.ts";
import { projectLocalPkl } from "../../src/project-files.ts";
import { VERSION } from "../../src/version.ts";
import {
  checkPackageVersion,
  checkPkl,
  ensureLocalManifest,
  loadProject,
  parseProject,
  readPackageVersion,
} from "../../src/project/project.ts";

const FULL = {
  map: { folder: "map.w3x", entry: "src/main.yue" },
  build: { folder: "dist/bin", minify: false },
  launch: { args: ["-launch"] },
  yue: { version: "0.34.2" },
  assets: { paths: {}, exclude: [] },
};

Deno.test("parseProject maps omitted nullable fields to null", () => {
  const project = parseProject("/p", FULL, "moonwell.pkl");
  assertEquals(project, {
    root: "/p",
    manifest: "moonwell.pkl",
    map: { folder: "map.w3x", entry: "src/main.yue" },
    build: { folder: "dist/bin", minify: false },
    launch: { gameExecutable: null, args: ["-launch"] },
    yue: { version: "0.34.2", path: null },
    assets: { paths: {}, exclude: [] },
    settings: validateMapSettings({}),
    objects: emptyObjects(),
    lint: { unknownGlobals: "error", globals: [] },
    libraries: {},
  });
});

Deno.test("parseProject reads libraries and requires github with tag unless path is set", () => {
  const project = parseProject("/p", {
    ...FULL,
    libraries: {
      example: { github: "mdlsvensson/moonwell-example-lib", tag: "v0.1.0", dir: "src" },
      mine: { path: "../mine", dir: "" },
    },
  }, "m.pkl");
  assertEquals(project.libraries, {
    example: { github: "mdlsvensson/moonwell-example-lib", tag: "v0.1.0", path: null, dir: "src" },
    mine: { github: null, tag: null, path: "../mine", dir: "" },
  });
  const error = assertThrows(
    () => parseProject("/p", { ...FULL, libraries: { half: { github: "a/b", dir: "" } } }, "m.pkl"),
    MoonwellError,
    'libraries["half"] needs both github and tag, or a path.',
  );
  assertEquals(error.file, "m.pkl");
});

Deno.test("parseProject reads the lint block", () => {
  const project = parseProject("/p", { ...FULL, lint: { unknownGlobals: "warning", globals: ["MyLibrary"] } }, "m.pkl");
  assertEquals(project.lint, { unknownGlobals: "warning", globals: ["MyLibrary"] });
});

Deno.test("parseProject rejects an unknown lint level", () => {
  assertThrows(
    () => parseProject("/p", { ...FULL, lint: { unknownGlobals: "off", globals: [] } }, "m.pkl"),
    MoonwellError,
    'lint.unknownGlobals must be "error" or "warning"',
  );
});

Deno.test("parseProject reads objects, defaulting sources to the evaluated manifest", () => {
  const objects = {
    units: {
      captain: { id: "h000", base: "hfoo", source: "objects/units.pkl", properties: {} },
      local: { id: "h001", base: "hfoo", properties: {} },
    },
  };
  const project = parseProject("/p", { ...FULL, objects }, "moonwell.local.pkl");
  assertEquals(project.objects.units.captain.source, "objects/units.pkl");
  assertEquals(project.objects.units.local.source, "moonwell.local.pkl");
  const error = assertThrows(
    () => parseProject("/p", { ...FULL, objects: { units: { a: { base: "hfoo" } } } }, "moonwell.pkl"),
    MoonwellError,
    'objects.units["a"].id must be a string.',
  );
  assertEquals([error.file, error.hint], ["moonwell.pkl", SCHEMA_HINT]);
});

Deno.test("parseProject defaults absent settings and validates malformed settings with the manifest path", () => {
  assertEquals(parseProject("/p", FULL, "moonwell.pkl").settings, validateMapSettings({}));
  const error = assertThrows(
    () => parseProject("/p", { ...FULL, settings: { players: { "24": {} } } }, "moonwell.local.pkl"),
    MoonwellError,
    "players",
  );
  assertEquals(error.file, "moonwell.local.pkl");
});

Deno.test("parseProject rejects conflicting typed and raw gameplay constants and keeps settings unmerged", () => {
  const settings = { gameplay: { foodLimit: 200 }, gameplayConstants: { misc: { foodceiling: "100" } } };
  const error = assertThrows(
    () => parseProject("/p", { ...FULL, settings }, "moonwell.local.pkl"),
    MoonwellError,
    "FoodCeiling",
  );
  assertEquals(error.file, "moonwell.local.pkl");
  const agreeing = { ...settings, gameplayConstants: { misc: { foodceiling: "200" } } };
  const project = parseProject("/p", { ...FULL, settings: agreeing }, "moonwell.pkl");
  assertEquals(project.manifest, "moonwell.pkl");
  assertEquals(project.settings.gameplayConstants, { misc: { foodceiling: "200" } });
});

Deno.test("parseProject reads assets and rejects a wrong shape", () => {
  const assets = { paths: { "a.blp": "Textures\\a.blp" }, exclude: ["credits/"] };
  assertEquals(parseProject("/p", { ...FULL, assets }, "moonwell.pkl").assets, assets);
  const cases: [unknown, string][] = [
    [null, "assets must be an object"],
    [{ paths: [], exclude: [] }, "assets.paths must be"],
    [{ paths: { a: 1 }, exclude: [] }, "assets.paths must be"],
    [{ paths: {}, exclude: "x" }, "assets.exclude must be"],
  ];
  for (const [bad, message] of cases) {
    assertThrows(() => parseProject("/p", { ...FULL, assets: bad }, "moonwell.pkl"), MoonwellError, message);
  }
});

Deno.test("parseProject defaults a missing assets block, as 0.1.0 schema packages have none", () => {
  const { assets: _, ...withoutAssets } = FULL;
  assertEquals(parseProject("/p", withoutAssets, "moonwell.pkl").assets, { paths: {}, exclude: [] });
});

Deno.test("parseProject rejects a schema mismatch", () => {
  const error = assertThrows(
    () => parseProject("/p", { ...FULL, map: { folder: 3 } }, "moonwell.pkl"),
    MoonwellError,
  );
  assertEquals(error.file, "moonwell.pkl");
});

const LOCAL_DEPS = JSON.stringify({
  schemaVersion: 1,
  resolvedDependencies: {
    "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0": {
      type: "local",
      uri: "projectpackage://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0.1.3",
      path: "../schema",
    },
  },
});

Deno.test("readPackageVersion finds the resolved moonwell version", () => {
  assertEquals(readPackageVersion(LOCAL_DEPS), "0.1.3");
});

Deno.test("readPackageVersion fails without a moonwell dependency", () => {
  assertThrows(() => readPackageVersion(JSON.stringify({ resolvedDependencies: {} })), MoonwellError, "not a resolved");
});

Deno.test("checkPackageVersion compares major.minor only", () => {
  checkPackageVersion("0.1.9", "0.1.0");
  assertThrows(() => checkPackageVersion("0.2.0", "0.1.0"), MoonwellError, "does not match");
});

const fakeRunner =
  (outputs: Record<string, { code?: number; stdout?: string; stderr?: string }>): Runner => (command, args) => {
    const key = [command, ...args].join(" ");
    const match = Object.entries(outputs).find(([prefix]) => key.startsWith(prefix));
    if (!match) return Promise.reject(new Error(`unexpected command: ${key}`));
    return Promise.resolve({ code: match[1].code ?? 0, stdout: match[1].stdout ?? "", stderr: match[1].stderr ?? "" });
  };

Deno.test("checkPkl requires Pkl 0.32 or newer", async () => {
  await checkPkl(fakeRunner({ "pkl --version": { stdout: "Pkl 0.32.1 (Windows 10.0, native)" } }));
  await assertRejects(
    () => checkPkl(fakeRunner({ "pkl --version": { stdout: "Pkl 0.31.0 (Linux)" } })),
    MoonwellError,
    "0.32",
  );
});

Deno.test("loadProject prefers moonwell.local.pkl and parses pkl output", async () => {
  const root = await Deno.makeTempDir();
  await Deno.writeTextFile(join(root, "moonwell.pkl"), "");
  await Deno.writeTextFile(join(root, "moonwell.local.pkl"), "");
  await Deno.writeTextFile(join(root, "PklProject.deps.json"), LOCAL_DEPS.replace("0.1.3", VERSION));
  const run = fakeRunner({
    "pkl --version": { stdout: "Pkl 0.32.1" },
    "pkl eval --format json --project-dir . moonwell.local.pkl": { stdout: JSON.stringify(FULL) },
  });
  assertEquals((await loadProject(root, run)).map.folder, "map.w3x");
});

Deno.test("loadProject reports pkl evaluation errors", async () => {
  const root = await Deno.makeTempDir();
  await Deno.writeTextFile(join(root, "moonwell.pkl"), "");
  await Deno.writeTextFile(join(root, "PklProject.deps.json"), LOCAL_DEPS.replace("0.1.3", VERSION));
  const run = fakeRunner({
    "pkl --version": { stdout: "Pkl 0.32.1" },
    "pkl eval": { code: 1, stderr: "–– Pkl Error ––\nType constraint violated" },
  });
  await assertRejects(() => loadProject(root, run), MoonwellError, "Type constraint violated");
});

Deno.test("loadProject explains a missing manifest", async () => {
  const root = await Deno.makeTempDir();
  const run = fakeRunner({ "pkl --version": { stdout: "Pkl 0.32.1" } });
  await assertRejects(() => loadProject(root, run), MoonwellError, "No moonwell.pkl");
});

Deno.test("loadProject explains a corrupt PklProject.deps.json", async () => {
  const root = await Deno.makeTempDir();
  await Deno.writeTextFile(join(root, "moonwell.pkl"), "");
  await Deno.writeTextFile(join(root, "PklProject.deps.json"), "{ not json");
  const run = fakeRunner({ "pkl --version": { stdout: "Pkl 0.32.1" } });
  const error = await assertRejects(() => loadProject(root, run), MoonwellError, "PklProject.deps.json");
  assertStringIncludes(error.hint ?? "", "pkl project resolve");
});

Deno.test("loadProject explains pkl output that is not JSON", async () => {
  const root = await Deno.makeTempDir();
  await Deno.writeTextFile(join(root, "moonwell.pkl"), "");
  await Deno.writeTextFile(join(root, "PklProject.deps.json"), LOCAL_DEPS.replace("0.1.3", VERSION));
  const run = fakeRunner({ "pkl --version": { stdout: "Pkl 0.32.1" }, "pkl eval": { stdout: "map { }" } });
  await assertRejects(() => loadProject(root, run), MoonwellError, "not valid JSON");
});

Deno.test("ensureLocalManifest creates moonwell.local.pkl once and never overwrites it", async () => {
  const root = await Deno.makeTempDir();
  const local = join(root, "moonwell.local.pkl");
  assertEquals(await ensureLocalManifest(root), true);
  assertEquals(await Deno.readTextFile(local), projectLocalPkl());
  await Deno.writeTextFile(local, "mine");
  assertEquals(await ensureLocalManifest(root), false);
  assertEquals(await Deno.readTextFile(local), "mine");
});
