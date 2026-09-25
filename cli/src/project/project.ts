import { exists } from "@std/fs";
import { join } from "@std/path";
import { projectLocalPkl } from "../project-files.ts";
import { MoonwellError } from "../shared/errors.ts";
import { type Runner, runProcess } from "../shared/process.ts";
import { VERSION } from "../version.ts";

export interface Project {
  root: string;
  map: { folder: string; entry: string };
  build: { folder: string; minify: boolean };
  launch: { gameExecutable: string | null; args: string[] };
  yue: { version: string; path: string | null };
  assets: { paths: Record<string, string>; exclude: string[] };
}

export const PKL_INSTALL_HINT =
  "Install Pkl 0.32 or newer: https://pkl-lang.org/main/current/pkl-cli/index.html#installation";

/** Creates moonwell.local.pkl in `root` unless it exists; returns whether it did. Never overwrites. */
export async function ensureLocalManifest(root: string): Promise<boolean> {
  try {
    await Deno.writeTextFile(join(root, "moonwell.local.pkl"), projectLocalPkl(), { createNew: true });
    return true;
  } catch (error) {
    if (error instanceof Deno.errors.AlreadyExists) return false;
    throw error;
  }
}

/** Evaluates moonwell.local.pkl (or moonwell.pkl) in `root` and returns the typed project. */
export async function loadProject(root: string, run: Runner = runProcess): Promise<Project> {
  await checkPkl(run);
  const file = (await exists(join(root, "moonwell.local.pkl"))) ? "moonwell.local.pkl" : "moonwell.pkl";
  if (!(await exists(join(root, file)))) {
    throw new MoonwellError("No moonwell.pkl found in this directory.", {
      file: root,
      hint: "Run this command from a Moonwell project, or create one with `moonwell init <dir>`.",
    });
  }
  let deps: string;
  try {
    deps = await Deno.readTextFile(join(root, "PklProject.deps.json"));
  } catch (error) {
    if (!(error instanceof Deno.errors.NotFound)) throw error;
    throw new MoonwellError("PklProject.deps.json is missing.", {
      file: "PklProject",
      hint: "Run `pkl project resolve` in the project folder.",
    });
  }
  checkPackageVersion(readPackageVersion(deps), VERSION);
  const result = await run("pkl", ["eval", "--format", "json", "--project-dir", ".", file], {
    cwd: root,
    hint: PKL_INSTALL_HINT,
  });
  if (result.code !== 0) {
    throw new MoonwellError(`Evaluating ${file} failed:\n${(result.stderr || result.stdout).trim()}`, { file });
  }
  let value: unknown;
  try {
    value = JSON.parse(result.stdout);
  } catch (cause) {
    throw new MoonwellError(`pkl eval printed output that is not valid JSON:\n${result.stdout.trim().slice(0, 500)}`, {
      file,
      cause,
      hint: "Check that pkl on PATH is Pkl 0.32 or newer and that no other program is named pkl.",
    });
  }
  return parseProject(root, value, file);
}

export async function checkPkl(run: Runner): Promise<void> {
  const result = await run("pkl", ["--version"], { hint: PKL_INSTALL_HINT });
  const match = /Pkl (\d+)\.(\d+)\.(\d+)/.exec(result.stdout);
  const tooOld = match && (Number(match[1]) === 0 && Number(match[2]) < 32);
  if (!match || tooOld) {
    throw new MoonwellError(`Moonwell needs Pkl 0.32 or newer (found: ${result.stdout.trim() || "unknown"}).`, {
      hint: PKL_INSTALL_HINT,
    });
  }
}

/** The resolved version of the `moonwell` package in a PklProject.deps.json document. */
export function readPackageVersion(depsJson: string): string {
  let deps: { resolvedDependencies?: Record<string, { uri?: string }> };
  try {
    deps = JSON.parse(depsJson);
  } catch (cause) {
    throw new MoonwellError("PklProject.deps.json is not valid JSON.", {
      file: "PklProject.deps.json",
      cause,
      hint: "Fix or regenerate it with `pkl project resolve`.",
    });
  }
  for (const [key, dependency] of Object.entries(deps.resolvedDependencies ?? {})) {
    if (!/\/moonwell@\d+$/.test(key)) continue;
    const version = /@(\d+\.\d+\.\d+[^/]*)$/.exec(dependency.uri ?? "")?.[1];
    if (version) return version;
  }
  throw new MoonwellError("The moonwell Pkl package is not a resolved dependency.", {
    file: "PklProject.deps.json",
    hint: "Declare it in PklProject and run `pkl project resolve`.",
  });
}

export function checkPackageVersion(packageVersion: string, cliVersion: string): void {
  const [packageMajor, packageMinor] = packageVersion.split(".");
  const [cliMajor, cliMinor] = cliVersion.split(".");
  if (packageMajor === cliMajor && packageMinor === cliMinor) return;
  throw new MoonwellError(`Pkl package moonwell@${packageVersion} does not match Moonwell CLI ${cliVersion}.`, {
    file: "PklProject",
    hint:
      `Use moonwell@${cliMajor}.${cliMinor}.x in PklProject and @moonwell/cli@${cliMajor}.${cliMinor}.x in deno.json, then run \`pkl project resolve\`.`,
  });
}

/** Validates evaluated manifest JSON. Pkl omits null properties, so nullable fields may be absent. */
export function parseProject(root: string, value: unknown, file: string): Project {
  const fail = (path: string, expected: string): never => {
    throw new MoonwellError(`${path} must be ${expected}.`, {
      file,
      hint: "Is the moonwell Pkl package the version this CLI expects?",
    });
  };
  const record = (input: unknown, path: string): Record<string, unknown> =>
    input !== null && typeof input === "object" && !Array.isArray(input)
      ? input as Record<string, unknown>
      : fail(path, "an object");
  const string = (input: unknown, path: string): string => typeof input === "string" ? input : fail(path, "a string");
  const nullableString = (input: unknown, path: string): string | null =>
    input === undefined || input === null ? null : string(input, path);
  const boolean = (input: unknown, path: string): boolean =>
    typeof input === "boolean" ? input : fail(path, "a boolean");
  const strings = (input: unknown, path: string): string[] =>
    Array.isArray(input) && input.every((item) => typeof item === "string")
      ? input as string[]
      : fail(path, "a list of strings");
  const stringRecord = (input: unknown, path: string): Record<string, string> => {
    const value = record(input, path);
    return Object.values(value).every((item) => typeof item === "string")
      ? value as Record<string, string>
      : fail(path, "a mapping of strings");
  };

  const data = record(value, "the manifest");
  const map = record(data.map, "map");
  const buildConfig = record(data.build, "build");
  const launch = record(data.launch, "launch");
  const yue = record(data.yue, "yue");
  // Moonwell 0.1.0 schema packages have no assets block; within 0.1.x a missing one means no configuration.
  const assets = data.assets === undefined ? { paths: {}, exclude: [] } : record(data.assets, "assets");
  return {
    root,
    map: { folder: string(map.folder, "map.folder"), entry: string(map.entry, "map.entry") },
    build: { folder: string(buildConfig.folder, "build.folder"), minify: boolean(buildConfig.minify, "build.minify") },
    launch: {
      gameExecutable: nullableString(launch.gameExecutable, "launch.gameExecutable"),
      args: strings(launch.args, "launch.args"),
    },
    yue: { version: string(yue.version, "yue.version"), path: nullableString(yue.path, "yue.path") },
    assets: { paths: stringRecord(assets.paths, "assets.paths"), exclude: strings(assets.exclude, "assets.exclude") },
  };
}
