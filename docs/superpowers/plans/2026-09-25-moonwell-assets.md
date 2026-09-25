# Moonwell Assets (Plan 2a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Files under a project's `assets/` folder are imported into the built map, and `assets:check` / `assets:sync`
validate them and write them into the World Editor source map, with the behavior of wc3-dev-framework.

**Architecture:** A port of wc3-dev-framework's `scripts/assets/` into `cli/src/assets/`, made async and switched to
`MoonwellError`. `imports.ts` reads and writes `war3map.imp`. `paths.ts` validates paths and scans folders safely.
`collect.ts` turns `assets/` plus the manifest's `assets` block into a list of imports. `plan.ts` computes every file
change before writing anything, and applies the changes with rollback. The build pipeline applies a plan to the staged
map with no ownership state. `assets:sync` applies it to the source map and records ownership in
`.asset-state/<map folder>.json`.

**Tech Stack:** Deno 2.9, TypeScript, `jsr:@std/*` only, Pkl 0.32.

**Spec:** `docs/superpowers/specs/2026-09-24-moonwell-core-design.md`, §6.2 Assets. The settings-file rules are in §3.3,
and the pipeline order is in §5. The reference implementation is `C:\Users\mdlsvensson\Repo\wc3-dev-framework\scripts\assets\assets.ts`
and `imports.ts`, with tests in `scripts\tests\assets.ts`. Read them; this plan ports them.

**Roadmap context:** the spec's data layers are three independent subsystems, planned separately: **2a Assets (this
plan)**, 2b Map settings, 2c Object data. This plan must not touch map settings or object data.

## Global Constraints

- No Node.js: no `package.json`, no `node_modules`, no `npm:` or `node:` specifiers. Only `jsr:@std/*` imports, taken
  from the existing import map in `cli/deno.json` (`@std/assert`, `@std/cli`, `@std/encoding`, `@std/fs`, `@std/path`).
- Expected failures throw `MoonwellError` (`cli/src/shared/errors.ts`), with `file` and `hint` where they help. Anything
  else reaching the CLI is reported as an internal error.
- File system code is async (`Deno.readFile`, `Deno.lstat`, ...), matching the rest of `cli/src`.
- Formatting and lint: `deno fmt` (line width 120) and `deno lint` must be clean. Run `deno fmt` on changed files
  before every commit.
- Commits go directly on `main`. Every commit message ends with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- Settings-file rule (spec §3.3): the template's `moonwell.pkl` writes out everyday settings with their default values.
  Nothing is commented out unless it needs a World Editor step first. Lists in the schema are `List`, not `Listing`.
- After changing anything under `template/`, run `deno task gen`. `cli/tests/unit/embedded.test.ts` fails when
  `cli/src/embedded/` is stale.
- Checks, from the repo root: `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test` (unit),
  `deno task test:pkl` (needs pkl), `deno task test:e2e` (needs pkl and yue). All must pass at the end of every task.
- The version stays 0.1.0; releasing is not part of this plan. User-facing changes go under a new
  `## Unreleased` heading at the top of `CHANGELOG.md`, created by Task 6.

## File Structure

| File | Responsibility |
| --- | --- |
| `cli/src/assets/imports.ts` (create) | `war3map.imp` v1 reader and writer, and the in-map path of an import entry |
| `cli/src/assets/paths.ts` (create) | Path validation (`assetPath`, `targetPath`), case-insensitive keys, symlink-safe joins, folder scans |
| `cli/src/assets/collect.ts` (create) | `assets/` plus the manifest's `paths`/`exclude` → the sorted list of imports |
| `cli/src/assets/plan.ts` (create) | Ownership state, `planAssets` (all checks, no writes), `applyAssetPlan` (writes with rollback), `assetLocations` |
| `cli/src/commands/assets.ts` (create) | The `assets:check` and `assets:sync` commands |
| `schema/Project.pkl` (modify) | `AssetsConfig` class and the `assets` property |
| `cli/src/project/project.ts` (modify) | `Project.assets` and its parsing |
| `cli/src/pipeline.ts` (modify) | Import assets into the staged map |
| `cli/src/commands/check.ts`, `dev.ts` (modify) | `check` validates assets; `dev` watches `assets/` |
| `cli/src/main.ts`, `cli/src/project-files.ts` (modify) | Command dispatch and project tasks |
| `template/moonwell.pkl`, `template/assets/.gitkeep`, `template/deno.json` (modify/create) | Template project |

---

### Task 1: `war3map.imp` reader and writer

**Files:**
- Create: `cli/src/assets/imports.ts`
- Test: `cli/tests/unit/imports.test.ts`

**Interfaces:**
- Produces: `interface ImportEntry { flag: number; path: string }`,
  `importPath(entry: ImportEntry): string`, `readImports(bytes: Uint8Array, file?: string): ImportEntry[]`,
  `writeImports(entries: ImportEntry[]): Uint8Array`.

The format (version 1): little-endian `uint32` version (must be 1), `uint32` entry count, then per entry one flag byte
followed by a NUL-terminated UTF-8 path. Flags 5 and 8 (and legacy 0) store the file under `war3mapImported\`. Flags 10
and 13 mean the path is the full in-map path. Moonwell writes flag 13.

- [ ] **Step 1: Write the failing test**

`cli/tests/unit/imports.test.ts`:

```ts
import { assertEquals, assertThrows } from "@std/assert";
import { importPath, readImports, writeImports } from "../../src/assets/imports.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

Deno.test("war3map.imp round trips default and custom-path entries", () => {
  const entries = [{ flag: 5, path: "default.blp" }, { flag: 13, path: "Textures\\custom.blp" }];
  assertEquals(readImports(writeImports(entries)), entries);
  assertEquals(importPath(entries[0]), "war3mapImported\\default.blp");
  assertEquals(importPath(entries[1]), "Textures\\custom.blp");
  assertEquals(readImports(writeImports([])), []);
});

Deno.test("readImports rejects corrupt data with a MoonwellError naming the file", () => {
  const valid = writeImports([{ flag: 13, path: "a.blp" }]);
  const wrongVersion = valid.slice();
  wrongVersion[0] = 2;
  const badFlag = valid.slice();
  badFlag[8] = 7;
  for (const bytes of [new Uint8Array([1, 0]), valid.subarray(0, 10), wrongVersion, badFlag]) {
    const error = assertThrows(() => readImports(bytes, "maps/map.w3x/war3map.imp"), MoonwellError);
    assertEquals(error.file, "maps/map.w3x/war3map.imp");
  }
  const trailing = new Uint8Array([...valid, 0]);
  assertThrows(() => readImports(trailing), MoonwellError, "trailing");
  const empty = writeImports([{ flag: 13, path: "x" }]);
  empty[9] = 0; // the path's only character becomes the terminator
  assertThrows(() => readImports(empty.subarray(0, 10)), MoonwellError, "empty path");
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `deno test -A cli/tests/unit/imports.test.ts`
Expected: FAIL, module `../../src/assets/imports.ts` not found.

- [ ] **Step 3: Write the implementation**

`cli/src/assets/imports.ts`:

```ts
import { MoonwellError } from "../shared/errors.ts";

/** One entry of war3map.imp (version 1), the World Editor's import index. */
export interface ImportEntry {
  /** 0, 5 or 8: the file lives under war3mapImported\. 10 or 13: `path` is the full in-map path. */
  flag: number;
  path: string;
}

const FLAGS = new Set([0, 5, 8, 10, 13]);

/** The in-map path of an import. */
export function importPath(entry: ImportEntry): string {
  return entry.flag === 10 || entry.flag === 13 ? entry.path : `war3mapImported\\${entry.path}`;
}

export function readImports(bytes: Uint8Array, file = "war3map.imp"): ImportEntry[] {
  const corrupt = (problem: string): never => {
    throw new MoonwellError(`war3map.imp is unreadable: ${problem}.`, {
      file,
      hint: "Open and re-save the map in World Editor.",
    });
  };
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  if (bytes.length < 8) corrupt("it is truncated");
  if (view.getUint32(0, true) !== 1) corrupt(`version ${view.getUint32(0, true)} is not supported (expected 1)`);
  const count = view.getUint32(4, true);
  const decoder = new TextDecoder("utf-8", { fatal: true });
  const entries: ImportEntry[] = [];
  let offset = 8;
  for (let i = 0; i < count; i++) {
    if (offset >= bytes.length) corrupt("it is truncated");
    const flag = bytes[offset++];
    if (!FLAGS.has(flag)) corrupt(`entry ${i} has unknown flag ${flag}`);
    const end = bytes.indexOf(0, offset);
    if (end < 0) corrupt("it is truncated");
    if (end === offset) corrupt(`entry ${i} has an empty path`);
    let path = "";
    try {
      path = decoder.decode(bytes.subarray(offset, end));
    } catch {
      corrupt(`entry ${i} is not valid UTF-8`);
    }
    entries.push({ flag, path });
    offset = end + 1;
  }
  if (offset !== bytes.length) corrupt("it has trailing data");
  return entries;
}

export function writeImports(entries: ImportEntry[]): Uint8Array {
  const encoder = new TextEncoder();
  const paths = entries.map((entry) => encoder.encode(entry.path));
  const bytes = new Uint8Array(8 + paths.reduce((size, path) => size + path.length + 2, 0));
  const view = new DataView(bytes.buffer);
  view.setUint32(0, 1, true);
  view.setUint32(4, entries.length, true);
  let offset = 8;
  entries.forEach((entry, i) => {
    bytes[offset++] = entry.flag;
    bytes.set(paths[i], offset);
    offset += paths[i].length + 1; // the terminating NUL is already 0
  });
  return bytes;
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `deno test -A cli/tests/unit/imports.test.ts`
Expected: PASS (2 tests). The corrupt-data test expects the substrings `trailing` and `empty path`, which the messages
above contain.

- [ ] **Step 5: Format, lint and commit**

```bash
deno fmt cli/src/assets/imports.ts cli/tests/unit/imports.test.ts
deno lint
git add cli/src/assets/imports.ts cli/tests/unit/imports.test.ts
git commit -m "feat(assets): read and write war3map.imp" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Asset paths and collection

**Files:**
- Create: `cli/src/assets/paths.ts`, `cli/src/assets/collect.ts`
- Test: `cli/tests/unit/assets-collect.test.ts`

**Interfaces:**
- Consumes: nothing from Task 1.
- Produces (`paths.ts`): `pathKey(path: string): string`, `assetPath(value: string): string`,
  `targetPath(value: string): string`, `lstatOrUndefined(path: string): Promise<Deno.FileInfo | undefined>`,
  `safeJoin(root: string, relative: string): Promise<string>`, `scanFiles(root: string): Promise<Map<string, string>>`
  (case-insensitive key → relative path with `/`).
- Produces (`collect.ts`): `interface AssetsConfig { paths: Record<string, string>; exclude: string[] }`,
  `interface Asset { source: string; target: string; bytes: Uint8Array; hash: string }`,
  `collectAssets(root: string, config: AssetsConfig): Promise<Asset[]>`.

Rules, from the reference implementation:
- A path is relative, uses `/` after normalizing `\`, and has no empty, `.` or `..` segment. No segment contains control
  characters or `<>:"|?*`, ends in `.` or a space, or is a Windows device name (`con`, `prn`, `aux`, `nul`, `com1`-`com9`,
  `lpt1`-`lpt9`, with or without an extension).
- A target cannot be a map internal: anything starting `war3map`, `war3campaign`, `(listfile)`, `(attributes)` or
  `(signature)`, except under `war3mapImported/`, and nothing under `scripts/war3map.`.
- Paths compare case-insensitively (`pathKey` lowercases and uses `/`).
- Scans refuse symlinks (and Windows junctions), two names differing only in case, and anything that is not a regular
  file or folder.
- Files and folders whose name starts with `.` are never imported (`.gitkeep`).
- `exclude` entries are exact files, or folder prefixes when they end in `/` or `\`.
- `paths` must name existing, non-excluded files, each once. Two assets cannot share a target, and a target cannot sit
  under another target used as a file.
- The result is sorted by target key.

- [ ] **Step 1: Write the failing test**

`cli/tests/unit/assets-collect.test.ts`:

```ts
import { assertEquals, assertRejects, assertThrows } from "@std/assert";
import { dirname, join } from "@std/path";
import { collectAssets } from "../../src/assets/collect.ts";
import { assetPath, pathKey, scanFiles, targetPath } from "../../src/assets/paths.ts";
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
  for (const bad of ["", "../escape", "/absolute", "C:\\escape", "a//b", "bad.", "trailing ", "CON.blp", "a/./b", "a|b"]) {
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `deno test -A cli/tests/unit/assets-collect.test.ts`
Expected: FAIL, modules `../../src/assets/collect.ts` and `../../src/assets/paths.ts` not found.

- [ ] **Step 3: Write `paths.ts`**

`cli/src/assets/paths.ts`:

```ts
import { join, resolve } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";

/** A path's identity as Warcraft III and Windows see it: `/` separators, any letter case. */
export function pathKey(path: string): string {
  return path.replaceAll("\\", "/").toLowerCase();
}

const DEVICE_NAME = /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i;

function unsafeSegment(segment: string): boolean {
  return segment === "" || segment === "." || segment === ".." ||
    [...segment].some((char) => char.charCodeAt(0) < 32) || /[<>:"|?*]/.test(segment) ||
    /[. ]$/.test(segment) || DEVICE_NAME.test(segment);
}

/** A portable relative path with `/` separators. Rejects anything that could escape its folder or fail on Windows. */
export function assetPath(value: string): string {
  const normalized = value.replaceAll("\\", "/");
  if (normalized === "" || normalized.split("/").some(unsafeSegment)) {
    throw new MoonwellError(`Invalid asset path: ${value}`, {
      hint: "Use a relative path such as icons/BTNSword.blp, without .., drive letters or characters Windows forbids.",
    });
  }
  return normalized;
}

/** An in-map path an asset may be imported as. Map internals such as war3map.lua are never replaced. */
export function targetPath(value: string): string {
  const normalized = assetPath(value);
  const internal = /^(?:war3map|war3campaign|\(listfile\)|\(attributes\)|\(signature\))/i.test(normalized) &&
    !/^war3mapImported\//i.test(normalized);
  if (internal || /^scripts\/war3map\./i.test(normalized)) {
    throw new MoonwellError(`Reserved map path: ${value}`, {
      hint: "Assets cannot replace map internals such as war3map.lua or war3map.imp.",
    });
  }
  return normalized;
}

export async function lstatOrUndefined(path: string): Promise<Deno.FileInfo | undefined> {
  try {
    return await Deno.lstat(path);
  } catch (error) {
    if (error instanceof Deno.errors.NotFound) return undefined;
    throw error;
  }
}

function symlinkError(path: string): MoonwellError {
  return new MoonwellError(`Symlinks are not supported: ${path}`, {
    hint: "Replace the link (or Windows junction) with the real files.",
  });
}

/** Joins `relative` under `root`, refusing a symlink at every step so asset data cannot leave its folder. */
export async function safeJoin(root: string, relative: string): Promise<string> {
  let current = resolve(root);
  if ((await lstatOrUndefined(current))?.isSymlink) throw symlinkError(current);
  for (const segment of assetPath(relative).split("/")) {
    current = join(current, segment);
    if ((await lstatOrUndefined(current))?.isSymlink) throw symlinkError(current);
  }
  return current;
}

const compare = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);

/** Every regular file below `root`, as case-insensitive key → path relative to `root` with `/`. Missing root: empty. */
export async function scanFiles(root: string): Promise<Map<string, string>> {
  const result = new Map<string, string>();
  const info = await lstatOrUndefined(root);
  if (info === undefined) return result;
  if (info.isSymlink) throw symlinkError(root);
  if (!info.isDirectory) throw new MoonwellError(`Expected a folder: ${root}`);
  const seen = new Set<string>();
  const visit = async (relative: string): Promise<void> => {
    const folder = relative === "" ? root : await safeJoin(root, relative);
    const entries = await Array.fromAsync(Deno.readDir(folder));
    entries.sort((a, b) => compare(a.name, b.name));
    for (const entry of entries) {
      const name = assetPath(relative === "" ? entry.name : `${relative}/${entry.name}`);
      if (seen.has(pathKey(name))) {
        throw new MoonwellError(`Two paths differ only in letter case: ${name}`, {
          hint: "Warcraft III paths ignore letter case; rename one of them.",
        });
      }
      seen.add(pathKey(name));
      if (entry.isSymlink) throw symlinkError(join(root, name));
      if (entry.isDirectory) await visit(name);
      else if (entry.isFile) result.set(pathKey(name), name);
      else throw new MoonwellError(`Not a regular file: ${join(root, name)}`);
    }
  };
  await visit("");
  return result;
}
```

- [ ] **Step 4: Write `collect.ts`**

`cli/src/assets/collect.ts`:

```ts
import { MoonwellError } from "../shared/errors.ts";
import { sha256Hex } from "../shared/fs.ts";
import { assetPath, pathKey, safeJoin, scanFiles, targetPath } from "./paths.ts";

/** The manifest's `assets` block. */
export interface AssetsConfig {
  /** Path under assets/ → exact in-map path. */
  paths: Record<string, string>;
  /** Files under assets/, or folders ending in `/`, that are not imported. */
  exclude: string[];
}

/** A file under assets/ and the in-map path it is imported as. */
export interface Asset {
  /** Path under assets/, with `/`. */
  source: string;
  /** In-map path, with `/`. */
  target: string;
  bytes: Uint8Array;
  /** SHA-256 of `bytes`, lowercase hex. */
  hash: string;
}

function configError(message: string): MoonwellError {
  return new MoonwellError(message, { file: "moonwell.pkl", hint: "Fix the assets block in moonwell.pkl." });
}

const compare = (a: string, b: string) => (a < b ? -1 : a > b ? 1 : 0);

/** Reads assets/ and resolves every file's in-map target. Nothing is written. */
export async function collectAssets(root: string, config: AssetsConfig): Promise<Asset[]> {
  const folder = await safeJoin(root, "assets");
  const files = await scanFiles(folder);
  const exclusions = config.exclude.map((value) => ({
    folder: /[\\/]$/.test(value),
    key: pathKey(assetPath(value.replace(/[\\/]$/, ""))),
  }));
  const excluded = (key: string) =>
    key.split("/").some((part) => part.startsWith(".")) ||
    exclusions.some((rule) => key === rule.key || (rule.folder && key.startsWith(`${rule.key}/`)));

  const mappings = new Map<string, string>();
  for (const [source, target] of Object.entries(config.paths)) {
    const key = pathKey(assetPath(source));
    if (!files.has(key)) throw configError(`assets.paths names a file that does not exist: assets/${source}`);
    if (excluded(key)) throw configError(`assets.paths names an excluded file: ${source}`);
    if (mappings.has(key)) throw configError(`assets.paths names ${source} twice.`);
    mappings.set(key, targetPath(target));
  }

  const targets = new Set<string>();
  const assets: Asset[] = [];
  for (const [key, source] of files) {
    if (excluded(key)) continue;
    const target = mappings.get(key) ?? targetPath(source);
    if (targets.has(pathKey(target))) throw configError(`Two assets would be imported as ${target} (target collision).`);
    targets.add(pathKey(target));
    const bytes = await Deno.readFile(await safeJoin(folder, source));
    assets.push({ source, target, bytes, hash: await sha256Hex(bytes) });
  }
  for (const target of targets) {
    const parts = target.split("/");
    while (parts.length > 1) {
      parts.pop();
      if (targets.has(parts.join("/"))) {
        throw configError(`Asset ${target} would sit inside the asset file ${parts.join("/")} (file/folder collision).`);
      }
    }
  }
  return assets.sort((a, b) => compare(pathKey(a.target), pathKey(b.target)));
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/assets-collect.test.ts`
Expected: PASS (6 tests). On Windows, the case-collision half of the last test is skipped by design.

- [ ] **Step 6: Format, lint and commit**

```bash
deno fmt cli/src/assets cli/tests/unit/assets-collect.test.ts
deno lint
git add cli/src/assets/paths.ts cli/src/assets/collect.ts cli/tests/unit/assets-collect.test.ts
git commit -m "feat(assets): validate asset paths and collect imports from assets/" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Asset planning, ownership and rollback

**Files:**
- Create: `cli/src/assets/plan.ts`
- Test: `cli/tests/unit/assets-plan.test.ts`

**Interfaces:**
- Consumes: `ImportEntry`, `importPath`, `readImports`, `writeImports` (Task 1); `pathKey`, `assetPath`, `targetPath`,
  `lstatOrUndefined`, `safeJoin`, `scanFiles` (Task 2); `AssetsConfig`, `Asset`, `collectAssets` (Task 2).
- Produces: `interface AssetState { version: 1; files: Record<string, string> }`,
  `interface FileChange { file: string; before?: Uint8Array; after?: Uint8Array }`,
  `interface AssetPlan { assets: Asset[]; changes: FileChange[]; state: AssetState }`,
  `planAssets(root: string, mapDir: string, stateFile: string, config: AssetsConfig): Promise<AssetPlan>`,
  `applyAssetPlan(plan: AssetPlan, stateFile?: string): Promise<void>`,
  `assetLocations(root: string, mapFolder: string): Promise<{ mapDir: string; stateFile: string }>`.

Behavior, from the reference implementation:
- `planAssets` reads everything and writes nothing. It fails, before any change, when:
  - a file listed in the ownership state was edited in the map, including a file about to be deleted ("modified");
  - a new asset's target already exists in the map, or is already in `war3map.imp`, without being owned ("conflicts");
  - a folder on a target's path is a file in the map ("not a directory"), or the target itself is a folder;
  - the state file is malformed, or names a reserved path (forged state cannot delete map internals).
- New targets reuse the spelling of existing map folders, and of folders planned earlier in the same plan, so
  `Textures/` and `textures/` end up in one folder.
- Owned files that are no longer wanted are deleted.
- `war3map.imp` keeps every entry that is not owned, then lists each asset with flag 13 and a `\` path. When there are no
  assets and nothing is owned, `war3map.imp` is left exactly as it is, or absent.
- The new state lists every asset target with its hash.
- `applyAssetPlan` re-checks each file against its planned `before` bytes just before changing it. On any failure it
  restores every change already made, in reverse order, then throws a `MoonwellError`. The state file is written only
  when `stateFile` is passed. Builds never pass it, so building never changes source-map ownership.

- [ ] **Step 1: Write the failing test**

`cli/tests/unit/assets-plan.test.ts`:

```ts
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

  await put(root, relative(root, state).replaceAll("\\", "/"), JSON.stringify({
    version: 1,
    files: { "war3map.lua": "0".repeat(64) },
  }));
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
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `deno test -A cli/tests/unit/assets-plan.test.ts`
Expected: FAIL, module `../../src/assets/plan.ts` not found.

- [ ] **Step 3: Write the implementation**

`cli/src/assets/plan.ts`:

```ts
import { dirname, join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { removeFileIfExists, sha256Hex } from "../shared/fs.ts";
import { type Asset, type AssetsConfig, collectAssets } from "./collect.ts";
import { importPath, readImports, writeImports } from "./imports.ts";
import { assetPath, lstatOrUndefined, pathKey, safeJoin, scanFiles, targetPath } from "./paths.ts";

/** `.asset-state/<map folder>.json`: the source-map files assets:sync owns, by in-map path → SHA-256. */
export interface AssetState {
  version: 1;
  files: Record<string, string>;
}

/** One file to write (`after`) or delete (no `after`); `before` is its content at planning time, if it existed. */
export interface FileChange {
  file: string;
  before?: Uint8Array;
  after?: Uint8Array;
}

export interface AssetPlan {
  assets: Asset[];
  changes: FileChange[];
  /** Ownership after applying the plan. */
  state: AssetState;
}

/** The source map folder and its ownership state file. */
export async function assetLocations(root: string, mapFolder: string): Promise<{ mapDir: string; stateFile: string }> {
  return {
    mapDir: await safeJoin(root, `maps/${mapFolder}`),
    stateFile: await safeJoin(root, `.asset-state/${mapFolder}.json`),
  };
}

function equalBytes(a: Uint8Array | undefined, b: Uint8Array | undefined): boolean {
  if (a === undefined || b === undefined) return a === b;
  return a.length === b.length && a.every((value, i) => value === b[i]);
}

async function readIfExists(file: string): Promise<Uint8Array | undefined> {
  return (await lstatOrUndefined(file)) === undefined ? undefined : await Deno.readFile(file);
}

async function readState(file: string): Promise<AssetState> {
  const bytes = await readIfExists(file);
  if (bytes === undefined) return { version: 1, files: {} };
  const invalid = (problem: string): never => {
    throw new MoonwellError(`The asset ownership state is invalid: ${problem}.`, {
      file,
      hint: "Restore it from version control. It records which map files assets:sync owns.",
    });
  };
  let state: unknown;
  try {
    state = JSON.parse(new TextDecoder().decode(bytes));
  } catch {
    invalid("it is not JSON");
  }
  const record = state as { version?: unknown; files?: unknown };
  if (record?.version !== 1) invalid("version must be 1");
  if (record.files === null || typeof record.files !== "object" || Array.isArray(record.files)) {
    invalid("files must be an object");
  }
  const seen = new Set<string>();
  for (const [name, digest] of Object.entries(record.files as Record<string, unknown>)) {
    targetPath(name);
    if (typeof digest !== "string" || !/^[a-f0-9]{64}$/.test(digest)) invalid(`${name} has no valid hash`);
    if (seen.has(pathKey(name))) invalid(`${name} is listed twice`);
    seen.add(pathKey(name));
  }
  return record as AssetState;
}

/** Checks every asset, import and owned file, and returns the changes. Writes nothing. */
export async function planAssets(
  root: string,
  mapDir: string,
  stateFile: string,
  config: AssetsConfig,
): Promise<AssetPlan> {
  const assets = await collectAssets(root, config);
  if (!(await lstatOrUndefined(mapDir))?.isDirectory) {
    throw new MoonwellError(`The map folder ${mapDir} does not exist.`, {
      file: "moonwell.pkl",
      hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
    });
  }
  const files = await scanFiles(mapDir);
  const managed = new Map(
    Object.entries((await readState(stateFile)).files).map(([name, digest]) => [pathKey(name), { name, digest }]),
  );
  const impFile = await safeJoin(mapDir, files.get("war3map.imp") ?? "war3map.imp");
  const impBytes = await readIfExists(impFile);
  const imports = impBytes === undefined ? [] : readImports(impBytes, impFile);
  const importKeys = new Set<string>();
  for (const entry of imports) {
    const key = pathKey(assetPath(importPath(entry)));
    if (importKeys.has(key)) {
      throw new MoonwellError(`war3map.imp lists ${importPath(entry)} twice.`, {
        file: impFile,
        hint: "Remove the duplicate import in World Editor's Import Manager.",
      });
    }
    importKeys.add(key);
  }

  // Every owned file must be unchanged, including files this plan would delete.
  for (const [key] of managed) {
    const current = files.get(key);
    if (current === undefined) continue;
    const bytes = await Deno.readFile(await safeJoin(mapDir, current));
    if ((await sha256Hex(bytes)) !== managed.get(key)!.digest) {
      throw new MoonwellError(`${current} was modified in the map after assets:sync wrote it.`, {
        hint: "Move your edited copy into assets/ or restore the file, then sync again.",
      });
    }
  }

  const changes: FileChange[] = [];
  const plannedFolders = new Map<string, string>();
  for (const asset of assets) {
    const key = pathKey(asset.target);
    if (!managed.has(key) && (files.has(key) || importKeys.has(key))) {
      throw new MoonwellError(`Asset ${asset.target} conflicts with a file or import already in the map.`, {
        hint: "Import it under another path with assets.paths, or remove the map's own copy.",
      });
    }
    // Reuse the spelling of existing folders, and of folders planned earlier, so letter case stays consistent.
    const parts = asset.target.split("/");
    let relative = "";
    for (let i = 0; i < parts.length; i++) {
      const parent = relative === "" ? mapDir : await safeJoin(mapDir, relative);
      const info = await lstatOrUndefined(parent);
      if (info !== undefined && !info.isDirectory) {
        throw new MoonwellError(`${relative} in the map is a file, not a directory, so ${asset.target} cannot go there.`);
      }
      const existing = info === undefined
        ? undefined
        : (await Array.fromAsync(Deno.readDir(parent))).find((entry) => pathKey(entry.name) === pathKey(parts[i]));
      if (existing?.isSymlink) {
        throw new MoonwellError(`Symlinks are not supported: ${join(parent, existing.name)}`);
      }
      const folderKey = pathKey(relative === "" ? parts[i] : `${relative}/${parts[i]}`);
      const segment = existing?.name ?? plannedFolders.get(folderKey) ?? parts[i];
      relative = relative === "" ? segment : `${relative}/${segment}`;
      if (i < parts.length - 1) plannedFolders.set(folderKey, segment);
    }
    const file = await safeJoin(mapDir, relative);
    if ((await lstatOrUndefined(file))?.isDirectory) {
      throw new MoonwellError(`Asset ${asset.target} would replace a folder in the map.`);
    }
    const before = await readIfExists(file);
    if (before === undefined || (await sha256Hex(before)) !== asset.hash) changes.push({ file, before, after: asset.bytes });
    asset.target = relative;
  }

  const wanted = new Set(assets.map((asset) => pathKey(asset.target)));
  for (const [key] of managed) {
    const current = files.get(key);
    if (current === undefined || wanted.has(key)) continue;
    const file = await safeJoin(mapDir, current);
    changes.push({ file, before: await Deno.readFile(file) });
  }

  if (assets.length > 0 || managed.size > 0) {
    const merged = imports.filter((entry) => !managed.has(pathKey(importPath(entry))));
    merged.push(...assets.map((asset) => ({ flag: 13, path: asset.target.replaceAll("/", "\\") })));
    const after = writeImports(merged);
    if (!equalBytes(after, impBytes)) changes.push({ file: impFile, before: impBytes, after });
  }
  return {
    assets,
    changes,
    state: { version: 1, files: Object.fromEntries(assets.map((asset) => [asset.target, asset.hash])) },
  };
}

/** Applies a plan, undoing every change already made if one fails. Writes `stateFile` too when it is given. */
export async function applyAssetPlan(plan: AssetPlan, stateFile?: string): Promise<void> {
  const changes = [...plan.changes];
  if (stateFile !== undefined) {
    const before = await readIfExists(stateFile);
    const after = new TextEncoder().encode(`${JSON.stringify(plan.state, null, 2)}\n`);
    if (!equalBytes(before, after)) changes.push({ file: stateFile, before, after });
  }
  const applied: FileChange[] = [];
  try {
    for (const change of changes) {
      if (!equalBytes(await readIfExists(change.file), change.before)) {
        throw new MoonwellError(`${change.file} changed after the assets were checked.`, {
          hint: "Close World Editor and anything else writing to the map, then retry.",
        });
      }
      applied.push(change);
      if (change.after === undefined) {
        await Deno.remove(change.file);
      } else {
        await Deno.mkdir(dirname(change.file), { recursive: true });
        await Deno.writeFile(change.file, change.after);
      }
    }
  } catch (error) {
    let undone = true;
    for (const change of applied.reverse()) {
      try {
        if (change.before === undefined) await removeFileIfExists(change.file);
        else await Deno.writeFile(change.file, change.before);
      } catch {
        undone = false;
      }
    }
    if (!undone) {
      throw new MoonwellError("Writing assets failed and not every change could be undone.", {
        cause: error,
        hint: "Restore the map folder from version control before retrying.",
      });
    }
    if (error instanceof MoonwellError) throw error;
    const reason = error instanceof Error ? error.message : String(error);
    throw new MoonwellError(`Writing assets failed: ${reason}. Every change was undone.`, { cause: error });
  }
}
```

Note: a folder standing where a planned file should be makes `readIfExists` throw. That happens before the change joins
`applied`, so rollback never deletes that folder, which the rollback test checks.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/assets-plan.test.ts`
Expected: PASS (9 tests).

- [ ] **Step 5: Run the whole unit suite**

Run: `deno task test`
Expected: all pass.

- [ ] **Step 6: Format, lint and commit**

```bash
deno fmt cli/src/assets cli/tests/unit/assets-plan.test.ts
deno lint
git add cli/src/assets/plan.ts cli/tests/unit/assets-plan.test.ts
git commit -m "feat(assets): plan and apply asset imports with ownership and rollback" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: The `assets` manifest block

**Files:**
- Modify: `schema/Project.pkl` (add the `AssetsConfig` class and the `assets` property)
- Modify: `schema/tests/Project.pkl` (facts)
- Modify: `cli/src/project/project.ts` (the `Project` type and `parseProject`)
- Modify: `cli/src/assets/collect.ts` (make `AssetsConfig` an alias of `Project["assets"]`)
- Modify: `cli/tests/unit/project.test.ts`, `cli/tests/unit/build.test.ts` (fixtures), `cli/tests/pkl/project.test.ts`
- Modify: `template/moonwell.pkl`; create `template/assets/.gitkeep`
- Modify: `docs/superpowers/specs/2026-09-24-moonwell-core-design.md` (§3.1 layout line)
- Regenerate: `cli/src/embedded/template.ts` (`deno task gen`)

**Interfaces:**
- Consumes: `AssetsConfig` (Task 2).
- Produces: `Project.assets: { paths: Record<string, string>; exclude: string[] }`, always present after
  `loadProject`/`parseProject`.

The spec's layout lists an `Assets.pkl` file. With only two properties, this plan instead keeps the class in
`Project.pkl`, next to `MapConfig` and `BuildConfig`, and updates the spec's layout line.

- [ ] **Step 1: Write the failing Pkl facts**

In `schema/tests/Project.pkl`, add to the `["defaults"]` fact:

```pkl
    Project.assets.paths.isEmpty
    Project.assets.exclude == List()
```

Add these facts before `["an amending project renders as JSON"]`:

```pkl
  ["assets accept paths and excludes"] {
    (Project) { assets { paths { ["icons/a.blp"] = #"ReplaceableTextures\CommandButtons\BTNA.blp"# } } }
      .assets.paths["icons/a.blp"] == #"ReplaceableTextures\CommandButtons\BTNA.blp"#
    (Project) { assets { exclude = List("credits/") } }.assets.exclude == List("credits/")
  }
  ["asset paths and excludes cannot be empty"] {
    t.catch(() -> (Project) { assets { paths { [""] = "x.blp" } } }.assets.paths.toMap()).contains("isEmpty")
    t.catch(() -> (Project) { assets { paths { ["a.blp"] = "" } } }.assets.paths.toMap()).contains("isEmpty")
    t.catch(() -> (Project) { assets { exclude = List("") } }.assets.exclude).contains("isEmpty")
  }
```

In `["an amending project renders as JSON"]`, add:

```pkl
    parsed.assets.paths is Dynamic
    parsed.assets.exclude.toList() == List()
```

- [ ] **Step 2: Run the Pkl tests to verify they fail**

Run: `pkl test schema/tests/Project.pkl`
Expected: FAIL; `assets` is not a property of the module.

- [ ] **Step 3: Add the schema**

In `schema/Project.pkl`, after the `YueConfig` class, add:

```pkl
class AssetsConfig {
  /// Files under `assets/` keep their relative path in the map. Map one here to import it at an exact in-map path,
  /// such as `["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"#`.
  paths: Mapping<String(!isEmpty), String(!isEmpty)> = new {}

  /// Files under `assets/`, or folders ending in `/`, that are not imported. Names starting with `.` never are.
  exclude: List<String(!isEmpty)> = List()
}
```

and after `yue: YueConfig = new {}`:

```pkl
assets: AssetsConfig = new {}
```

- [ ] **Step 4: Run the Pkl tests to verify they pass**

Run: `pkl test schema/tests/Project.pkl`
Expected: PASS. (Checked with Pkl 0.32.1: `Mapping` values are type-checked lazily, which is why the facts force them
with `toMap()`; the JSON parser reads `{}` as `Dynamic` and `[]` as a `Listing`.)

- [ ] **Step 5: Write the failing TypeScript tests**

In `cli/tests/unit/project.test.ts`, add `assets: { paths: {}, exclude: [] }` to the `FULL` constant, and
`assets: { paths: {}, exclude: [] }` to the expected object in "parseProject maps omitted nullable fields to null". Then
add:

```ts
Deno.test("parseProject reads assets and rejects a wrong shape", () => {
  const assets = { paths: { "a.blp": "Textures\\a.blp" }, exclude: ["credits/"] };
  assertEquals(parseProject("/p", { ...FULL, assets }, "moonwell.pkl").assets, assets);
  for (const bad of [{ paths: [], exclude: [] }, { paths: { a: 1 }, exclude: [] }, { paths: {}, exclude: "x" }]) {
    assertThrows(() => parseProject("/p", { ...FULL, assets: bad }, "moonwell.pkl"), MoonwellError, "assets");
  }
});
```

In `cli/tests/unit/build.test.ts`, add `assets: { paths: {}, exclude: [] },` to the object returned by `project()`.

In `cli/tests/pkl/project.test.ts`, add after the existing `project.yue` assertion:

```ts
  assertEquals(project.assets, { paths: {}, exclude: [] });
```

- [ ] **Step 6: Run the unit tests to verify they fail**

Run: `deno task test`
Expected: FAIL, because `Project` has no `assets` property (a type error in `build.test.ts` and `project.test.ts`).

- [ ] **Step 7: Parse `assets`**

In `cli/src/project/project.ts`, add to the `Project` interface:

```ts
  assets: { paths: Record<string, string>; exclude: string[] };
```

In `parseProject`, add after the `strings` helper:

```ts
  const stringRecord = (input: unknown, path: string): Record<string, string> => {
    const value = record(input, path);
    return Object.values(value).every((item) => typeof item === "string")
      ? value as Record<string, string>
      : fail(path, "a mapping of strings");
  };
```

and after `const yue = record(data.yue, "yue");`:

```ts
  const assets = record(data.assets, "assets");
```

and add this property to the returned object, after `yue`:

```ts
    assets: { paths: stringRecord(assets.paths, "assets.paths"), exclude: strings(assets.exclude, "assets.exclude") },
```

`record()` fails on an array, so `paths: []` is rejected too.

In `cli/src/assets/collect.ts`, replace the `AssetsConfig` interface with an alias, so the type has one source of truth:

```ts
/** The manifest's `assets` block: `paths` maps assets/ files to exact in-map paths; `exclude` leaves files out. */
export type AssetsConfig = Project["assets"];
```

and add `import type { Project } from "../project/project.ts";` to its imports.

- [ ] **Step 8: Run the tests to verify they pass**

Run: `deno task test` and `deno task test:pkl`
Expected: PASS.

- [ ] **Step 9: Write out the template's `assets` block**

In `template/moonwell.pkl`, add after the `launch` block:

```pkl

assets {
  paths {}                // assets/ file → exact in-map path, e.g. ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"#
  exclude = List()        // files under assets/, or folders ending in /, to leave out
}
```

Create `template/assets/.gitkeep` as an empty file. It keeps `assets/` in the scaffolded project, and dotfiles are
never imported.

In `docs/superpowers/specs/2026-09-24-moonwell-core-design.md` §3.1, change the line
`    MapSettings.pkl  Assets.pkl  Objects.pkl  ObjectFile.pkl` to
`    MapSettings.pkl  Objects.pkl  ObjectFile.pkl   (AssetsConfig lives in Project.pkl)`.

- [ ] **Step 10: Regenerate and verify the template**

Run: `deno task gen`, then `deno task test`, `deno task test:pkl` and `deno task test:e2e`.
Expected: all pass. The pkl `init` test loads the new template, so the `assets` block is evaluated for real.

- [ ] **Step 11: Format, lint and commit**

```bash
deno fmt
deno lint
git add schema cli/src/project/project.ts cli/src/assets/collect.ts cli/tests template cli/src/embedded docs/superpowers/specs
git commit -m "feat(assets): assets block in the manifest and the template" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Import assets in builds; `check` and `dev` cover them

**Files:**
- Modify: `cli/src/pipeline.ts` (`prepareStage`)
- Modify: `cli/src/commands/check.ts`
- Modify: `cli/src/commands/dev.ts` (`isRelevantChange`, watchers)
- Test: `cli/tests/unit/dev.test.ts`, `cli/tests/e2e/project.test.ts`

**Interfaces:**
- Consumes: `planAssets`, `applyAssetPlan`, `assetLocations` (Task 3); `collectAssets` (Task 2); `Project.assets` (Task 4).
- Produces: `check()` now returns `{ modules: number; entry: string; assets: number }`.

Spec §5 applies object data, then settings, then assets to the staged map, before the bundle is injected. Only assets
exist so far, so they go right after staging. The build passes no state file, so it never changes source-map ownership.
`check` plans against the source map read-only. When the source map is missing, it only collects `assets/`, because
`check` never required a map.

- [ ] **Step 1: Write the failing tests**

In `cli/tests/unit/dev.test.ts`, add to the `isRelevantChange` test:

```ts
  assertEquals(check("assets", "icons", "a.blp"), true);
```

In `cli/tests/e2e/project.test.ts`, add:

```ts
Deno.test("build imports assets/ into the archive and war3map.imp", async () => {
  const project = await newProject();
  const bytes = new Uint8Array([0, 1, 2, 250, 255]);
  await Deno.mkdir(join(project, "assets", "Models"), { recursive: true });
  await Deno.writeFile(join(project, "assets", "Models", "unit.mdx"), bytes);
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);

  const archive = openMpq(await Deno.readFile(join(project, "dist", "bin", "map.w3x")));
  assertEquals(await archive.read("Models\\unit.mdx"), bytes);
  const imports = readImports((await archive.read("war3map.imp"))!);
  assert(imports.some((entry) => entry.flag === 13 && entry.path === "Models\\unit.mdx"), JSON.stringify(imports));
  assertEquals(await exists(join(project, ".asset-state")), false, "a build never writes ownership state");
  assertEquals(await exists(join(project, "maps", "map.w3x", "Models")), false, "a build never touches the source map");
});

Deno.test("check reports an asset problem", async () => {
  const project = await newProject();
  const manifest = join(project, "moonwell.pkl");
  const text = await Deno.readTextFile(manifest);
  assertStringIncludes(text, "paths {}");
  await Deno.writeTextFile(manifest, text.replace("paths {}", 'paths { ["missing.blp"] = "x.blp" }'));
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(checked.text, "does not exist");
});
```

and add `import { readImports } from "../../src/assets/imports.ts";` to its imports.

`openMpq(...).read(name)` (in `cli/tests/support/mpq-reader.ts`) returns `Promise<Uint8Array | undefined>`, so
comparing with `bytes` also fails cleanly when the file is missing. The `!` on the `war3map.imp` read is safe because
the assertion above it already proved the archive has files.

The existing dev e2e test waits for `"Watching src/"`, which still matches after Step 5 changes the log line.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/dev.test.ts` → FAIL on `assets`.
Run: `deno task test:e2e` → FAIL: the archive has no `Models\unit.mdx`, and `check` passes despite the bad mapping.

- [ ] **Step 3: Apply assets to the staged map**

In `cli/src/pipeline.ts`, add the import:

```ts
import { applyAssetPlan, assetLocations, planAssets } from "./assets/plan.ts";
```

In `prepareStage`, directly after the `try { await replaceDir(source, mapDir); } catch ... {}` block, add:

```ts
  // No state file: a build imports into the staged copy only and never changes source-map ownership.
  const { stateFile } = await assetLocations(ctx.root, project.map.folder);
  const assets = await planAssets(ctx.root, mapDir, stateFile, project.assets);
  await applyAssetPlan(assets);
  if (assets.assets.length > 0) ctx.logger.info(`Imported ${assets.assets.length} asset(s).`);
```

- [ ] **Step 4: Validate assets in `check`**

Replace the body of `check` inside `withBuildLock` in `cli/src/commands/check.ts` with:

```ts
    const { modules, entry } = await compileProject(ctx, project, {});
    const { mapDir, stateFile } = await assetLocations(ctx.root, project.map.folder);
    const assets = (await exists(mapDir))
      ? (await planAssets(ctx.root, mapDir, stateFile, project.assets)).assets
      : await collectAssets(ctx.root, project.assets);
    ctx.logger.info(`Check passed: ${modules.length} module(s) reachable from ${entry}, ${assets.length} asset(s).`);
    return { modules: modules.length, entry, assets: assets.length };
```

Update the return type to `Promise<{ modules: number; entry: string; assets: number }>`, and add the imports:

```ts
import { exists } from "@std/fs";
import { collectAssets } from "../assets/collect.ts";
import { assetLocations, planAssets } from "../assets/plan.ts";
```

Search the tests for assertions on the old `Check passed:` text (`grep -rn "Check passed" cli/tests`) and on the old
return value, and update them to the new text and shape.

- [ ] **Step 5: Watch `assets/` in `dev`**

In `cli/src/commands/dev.ts`, in `isRelevantChange`, add before the final `return`:

```ts
  if (rel.startsWith("assets/")) return true;
```

Replace the `watchers` array with:

```ts
  const watchers = [
    Deno.watchFs(join(ctx.root, "src"), { recursive: true }),
    Deno.watchFs(ctx.root, { recursive: false }),
  ];
  // assets/ is optional; a folder created after dev starts is picked up on the next dev run.
  if (await exists(join(ctx.root, "assets"), { isDirectory: true })) {
    watchers.push(Deno.watchFs(join(ctx.root, "assets"), { recursive: true }));
  }
```

Change the log line to `"Watching src/, assets/ and the project manifests. Press Ctrl+C to stop."`, and update any
test that asserts the old text.

- [ ] **Step 6: Run everything**

Run: `deno task check`, `deno task test`, `deno task test:pkl`, `deno task test:e2e`
Expected: all pass, including the two new e2e tests.

- [ ] **Step 7: Format, lint and commit**

```bash
deno fmt
deno lint
git add cli/src/pipeline.ts cli/src/commands/check.ts cli/src/commands/dev.ts cli/tests
git commit -m "feat(assets): import assets into builds; check and dev cover assets" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: `assets:check` and `assets:sync` commands

**Files:**
- Create: `cli/src/commands/assets.ts`
- Modify: `cli/src/main.ts` (usage, dispatch, Ctrl+C handling)
- Modify: `cli/src/project-files.ts` (`PROJECT_TASKS`)
- Modify: `template/deno.json` (regenerated from `projectDenoJson`), `cli/src/embedded/template.ts` (`deno task gen`)
- Modify: `README.md`, `CHANGELOG.md`
- Test: `cli/tests/pkl/assets.test.ts` (create), `cli/tests/unit/project-files.test.ts`, `cli/tests/unit/main.test.ts`

**Interfaces:**
- Consumes: `planAssets`, `applyAssetPlan`, `assetLocations`, `AssetPlan` (Task 3); `loadProject`; `withBuildLock`.
- Produces: `assets(ctx: CommandContext, mode: "check" | "sync"): Promise<AssetPlan>`; CLI commands `assets:check`
  and `assets:sync`; project tasks of the same names.

`assets:sync` is the only command that writes into `maps/<folder>`. It requires a complete source map
(`war3map.lua` and `war3map.w3i`), takes the build lock, and records ownership in `.asset-state/<map folder>.json`,
which the project commits. The map must be closed in World Editor.

- [ ] **Step 1: Write the failing tests**

In `cli/tests/unit/project-files.test.ts`, change the expected task list to:

```ts
  assertEquals(Object.keys(json.tasks), ["build", "test", "dev", "check", "setup", "assets:check", "assets:sync"]);
```

Create `cli/tests/pkl/assets.test.ts`:

```ts
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
  const ctx = createContext(root, silentLogger());
  const map = join(root, "maps", "map.w3x");

  const checked = await assets(ctx, "check");
  assertEquals(checked.assets.map((asset) => asset.target), ["icons/a.blp"]);
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
```

In `cli/tests/unit/main.test.ts`, add a test proving both commands are dispatched rather than reported as unknown. It
uses the file's existing `run` helper. Outside a project the command fails; unit tests must not need `pkl`, so the test
only asserts that the failure is not "Unknown command":

```ts
Deno.test("assets:check and assets:sync are known commands", async () => {
  for (const command of ["assets:check", "assets:sync"]) {
    const { code, output } = await run([command]);
    assertEquals(code, 1);
    assertEquals(output.includes("Unknown command"), false, output);
  }
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno task test` → FAIL: the task list differs, and `main.test.ts` reports "Unknown command".
Run: `deno test -A cli/tests/pkl/assets.test.ts` → FAIL: `../../src/commands/assets.ts` not found.

- [ ] **Step 3: Write the command**

`cli/src/commands/assets.ts`:

```ts
import { exists } from "@std/fs";
import { join } from "@std/path";
import { type AssetPlan, applyAssetPlan, assetLocations, planAssets } from "../assets/plan.ts";
import type { CommandContext } from "../context.ts";
import { loadProject } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { withBuildLock } from "../shared/lock.ts";

/** assets:check shows what assets:sync would change; assets:sync writes assets/ into the source map for World Editor. */
export async function assets(ctx: CommandContext, mode: "check" | "sync"): Promise<AssetPlan> {
  // Loading is read-only; doing it before taking the lock creates nothing outside a project.
  const project = await loadProject(ctx.root, ctx.run);
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const { mapDir, stateFile } = await assetLocations(ctx.root, project.map.folder);
    for (const name of ["war3map.lua", "war3map.w3i"]) {
      if (!(await exists(join(mapDir, name), { isFile: true }))) {
        throw new MoonwellError(`The source map has no ${name}.`, {
          file: `maps/${project.map.folder}`,
          hint: "Save the map in World Editor in folder format with Lua as the script language.",
        });
      }
    }
    const plan = await planAssets(ctx.root, mapDir, stateFile, project.assets);
    for (const asset of plan.assets) ctx.logger.info(`${asset.source} -> ${asset.target.replaceAll("/", "\\")}`);
    if (mode === "sync") {
      await applyAssetPlan(plan, stateFile);
      ctx.logger.info(
        `Synced ${plan.assets.length} asset(s) into maps/${project.map.folder} (${plan.changes.length} file change(s)). ` +
          "Reopen the map in World Editor.",
      );
    } else {
      ctx.logger.info(
        `Checked ${plan.assets.length} asset(s); assets:sync would make ${plan.changes.length} file change(s). ` +
          "Nothing was written.",
      );
    }
    return plan;
  });
}
```

- [ ] **Step 4: Dispatch the commands**

In `cli/src/main.ts`:
- import `{ assets } from "./commands/assets.ts"`;
- add two lines to `USAGE`, after `check`:

  ```
    assets:check                   Show what assets:sync would change in the source map
    assets:sync                    Write assets/ into the source map (close it in World Editor first)
  ```

- add both commands to the `handlesSigint` list, since they hold the build lock:
  `["build", "test", "check", "dev", "assets:check", "assets:sync"]`;
- add the cases before `default`:

  ```ts
      case "assets:check":
        await assets(ctx, "check");
        break;
      case "assets:sync":
        await assets(ctx, "sync");
        break;
  ```

In `cli/src/project-files.ts`, change `PROJECT_TASKS` and its comment to:

```ts
/** Tasks every project's deno.json exposes. */
export const PROJECT_TASKS = ["build", "test", "dev", "check", "setup", "assets:check", "assets:sync"] as const;
```

- [ ] **Step 5: Regenerate the template's `deno.json`**

`cli/tests/unit/project-files.test.ts` requires `template/deno.json` to equal `projectDenoJson("../cli/src/main.ts")`.
Regenerate it and the embedded template:

```bash
deno eval "import { projectDenoJson } from './cli/src/project-files.ts'; await Deno.writeTextFile('template/deno.json', projectDenoJson('../cli/src/main.ts'))"
deno task gen
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `deno task test`, `deno task test:pkl`, `deno task test:e2e`
Expected: all pass.

- [ ] **Step 7: Document**

In `README.md`, add to the Commands table, after the `deno task check` row:

```md
| `deno task assets:check` | Show what `assets:sync` would change in the source map |
| `deno task assets:sync`  | Write `assets/` into the source map for World Editor (close the map first) |
```

In the "A project" table, add rows for `assets/` ("Files to import into the map") and `.asset-state/` ("Which
source-map files `assets:sync` owns; commit it"). After the table, add this section:

```md
## Assets

Every file under `assets/` is imported into the built map at its relative path: `assets/Models/unit.mdx` becomes
`Models\unit.mdx`. Names starting with `.` are skipped. The `assets` block in `moonwell.pkl` maps files to exact in-map
paths and leaves files out:

```pkl
assets {
  paths { ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"# }
  exclude = List("credits/")
}
```

Builds import assets into the staged copy only. To see them in World Editor, close the map there and run
`deno task assets:sync`. It writes the files and `war3map.imp` into `maps/<folder>`, and records what it owns in
`.asset-state/`. It never overwrites or deletes a file it does not own, and it refuses to touch an owned file you edited
in the map.
```

Run `deno fmt README.md` so the tables realign.

At the top of `CHANGELOG.md`, below `# Changelog`, add:

```md
## Unreleased

- Assets: files under `assets/` are imported into builds. The manifest's `assets` block maps them to exact paths and
  excludes files. The `assets:check` and `assets:sync` commands write them into the source map for World Editor, with
  ownership tracking and rollback.
```

- [ ] **Step 8: Final verification**

Run: `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test`, `deno task test:runtime`,
`deno task test:pkl`, `deno task test:e2e`
Expected: all pass.

- [ ] **Step 9: Commit**

```bash
git add cli/src/commands/assets.ts cli/src/main.ts cli/src/project-files.ts template/deno.json cli/src/embedded cli/tests README.md CHANGELOG.md
git commit -m "feat(assets): assets:check and assets:sync commands" -m "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## Manual check (after all tasks; needs Warcraft III and World Editor)

1. In `template/`, put a custom icon at `assets/ReplaceableTextures/CommandButtons/BTNMoonwell.blp`, run
   `deno task test`, and confirm the game loads the map.
2. Close World Editor, run `deno task assets:sync`, open `maps/map.w3x` in World Editor, and confirm the icon is listed
   in the Import Manager at `ReplaceableTextures\CommandButtons\BTNMoonwell.blp`.
3. Remove the icon from `assets/`, sync again, and confirm World Editor no longer lists it.
4. Delete the test icon and `.asset-state/` from `template/` afterwards; the template ships with neither.
