import { assertEquals, assertRejects, assertThrows } from "@std/assert";
import { join } from "@std/path";
import { MoonwellError } from "../../src/shared/errors.ts";
import type { Runner } from "../../src/shared/process.ts";
import {
  checkPackageVersion,
  checkPkl,
  loadProject,
  parseProject,
  readPackageVersion,
} from "../../src/project/project.ts";

const FULL = {
  map: { folder: "map.w3x", entry: "src/main.yue" },
  build: { folder: "dist/bin", minify: false },
  launch: { args: ["-launch"] },
  yue: { version: "0.34.2" },
};

Deno.test("parseProject maps omitted nullable fields to null", () => {
  const project = parseProject("/p", FULL, "moonwell.pkl");
  assertEquals(project, {
    root: "/p",
    map: { folder: "map.w3x", entry: "src/main.yue" },
    build: { folder: "dist/bin", minify: false },
    launch: { gameExecutable: null, args: ["-launch"] },
    yue: { version: "0.34.2", path: null },
  });
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
      path: "../pkl",
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
  await Deno.writeTextFile(join(root, "PklProject.deps.json"), LOCAL_DEPS.replace("0.1.3", "0.1.0"));
  const run = fakeRunner({
    "pkl --version": { stdout: "Pkl 0.32.1" },
    "pkl eval --format json --project-dir . moonwell.local.pkl": { stdout: JSON.stringify(FULL) },
  });
  assertEquals((await loadProject(root, run)).map.folder, "map.w3x");
});

Deno.test("loadProject reports pkl evaluation errors", async () => {
  const root = await Deno.makeTempDir();
  await Deno.writeTextFile(join(root, "moonwell.pkl"), "");
  await Deno.writeTextFile(join(root, "PklProject.deps.json"), LOCAL_DEPS.replace("0.1.3", "0.1.0"));
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
