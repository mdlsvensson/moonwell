# Moonwell Core Toolchain Implementation Plan (Plan 1 of 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A working Moonwell CLI with these abilities:
- scaffold a project;
- compile YueScript gameplay;
- bundle it into a World Editor map's `war3map.lua` with runtime hooks and source-mapped errors;
- pack the map into a `.w3x` with Moonwell's own MPQ writer;
- launch Warcraft III on it.

**Architecture:**
- A Deno CLI (`cli/`, published as `jsr:@moonwell/cli`) drives two external tools:
  - the pinned `yue` binary, downloaded into a per-user cache;
  - `pkl`, which evaluates each project's `moonwell.pkl` against the `moonwell` Pkl package (`pkl/`).
- A require-graph bundler emits only reachable modules and appends them to the map script, and a small Lua runtime wraps `main`/`config`.
- Map projects are thin. `init` scaffolds them from the committed `template/`.

**Tech Stack:**
- Deno 2.9+, TypeScript, `jsr:@std/*` only;
- YueScript 0.34.2 (`yue` CLI);
- Pkl 0.32+;
- Lua 5.3 (Warcraft III).

**Spec:** `docs/superpowers/specs/2026-09-24-moonwell-core-design.md`. This plan implements §3, §4 (except the data commands), §5, §7, §8, §9 and §10 for the toolchain. **Plan 2** (the data layers, spec §6) is written after this plan lands, so it can build on the real interfaces defined here.

## Global Constraints

- No Node.js:
  - no `package.json`, no `node_modules`, no `npm:` specifiers anywhere;
  - the root `deno.json` sets `"nodeModulesDir": "none"`. Workspace members must not set it, because Deno warns. Generated projects rely on Deno's default of no `node_modules` when there is no `package.json`;
  - only `jsr:@std/*` imports.
- Gameplay Lua targets Lua 5.3: `yue --target=5.3`. The runtime Lua uses only 5.3-compatible features.
- Pkl minimum version: 0.32.
- YueScript pinned default: `0.34.2`.
  - `yue-windows-x64.7z` SHA-256 `367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf`
  - `yue-linux-x86_64.zip` SHA-256 `fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d`
- Versions:
  - the CLI version, `pkl/PklProject` `package.version` and `cli/deno.json` `version` are all `0.1.0` and must always be equal;
  - the CLI rejects a Pkl package whose major.minor differs.
- Pkl package base URI: `package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell`. The release tag is `moonwell@<version>`.
- Every expected failure throws `MoonwellError { message, file?, line?, hint? }`. Exit code is 1 on any failure.
- Line endings are LF in the repo (`.gitattributes`). Template map files are binary (never normalized).
- Windows is the primary platform, and every unit test must also pass on Linux.
- Commit after every task. End each commit message with:
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`

## File Structure

```
moonwell/
  .gitattributes .gitignore LICENSE README.md CONTRIBUTING.md
  deno.json                       workspace root (members: cli, template); repo tasks; imports for tools/
  cli/
    deno.json                     @moonwell/cli; imports; publish config
    README.md                     JSR landing text
    runtime/moonwell.lua          Lua runtime prelude (source of embedded copy)
    src/
      main.ts                     CLI entry: arg parsing, dispatch, exit codes
      version.ts                  VERSION constant
      context.ts                  CommandContext + createContext
      pipeline.ts                 compileProject, prepareStage, entryModuleName
      launch.ts                   launchGame
      project-files.ts            projectDenoJson, projectPklProject (init + template)
      commands/{build,test,check,setup,init,dev}.ts
      shared/errors.ts            MoonwellError, formatError
      shared/log.ts               Logger, createLogger
      shared/process.ts           Runner, runProcess
      shared/fs.ts                listFiles, replaceDir, writeTextIfChanged, sha256Hex, removeIfExists, toPosix
      shared/lock.ts              withBuildLock
      shared/compression.ts       deflate, inflate, deflateRaw, inflateRaw
      project/project.ts          Project, loadProject, parseProject, readPackageVersion, checkPackageVersion, checkPkl
      yue/versions.ts             Platform, YueAsset, KNOWN_YUE, DEFAULT_YUE_VERSION, currentPlatform
      yue/unzip.ts                extractZip
      yue/install.ts              InstallDeps, ensureYue, defaultInstallDeps, defaultCacheRoot, yueVersion
      yue/compile.ts              CompiledModule, compileSources
      bundle/lexer.ts             tokenize, findRequires
      bundle/graph.ts             resolveGraph
      bundle/emit.ts              emitBundle, injectBundle
      mpq/crypto.ts               hashString, HashType, encryptBlock, decryptBlock
      mpq/writer.ts               MpqFile, writeMpq
      mpq/hm3w.ts                 buildHm3wHeader
      map/w3i.ts                  readW3iHeader, isHeaderlessArchive
      map/pack.ts                 packMap
      embedded/runtime.ts         GENERATED: RUNTIME_LUA
      embedded/template.ts        GENERATED: TEMPLATE_FILES
    tests/
      support/{zip.ts,mpq-reader.ts,yue.ts,logger.ts}
      unit/*.test.ts              no external tools
      yue/*.test.ts               need yue (downloaded to cache on first run)
      pkl/*.test.ts               need pkl
      e2e/*.test.ts               need pkl + yue
  pkl/
    PklProject                    package block
    Project.pkl                   manifest schema (Plan 2 adds settings/assets/objects)
    tests/Project.pkl             pkl test facts
  template/                       project scaffolded by init (local-linked for dev)
    deno.json PklProject PklProject.deps.json moonwell.pkl .gitignore
    src/main.yue
    maps/map.w3x/...              copied from wc3-dev-framework
  tools/gen.ts                    renders cli/src/embedded/*
```

---

### Task 1: Repository scaffold, errors, version

**Files:**
- Create: `.gitattributes`, `.gitignore`, `deno.json`, `cli/deno.json`, `cli/README.md`
- Create: `cli/src/version.ts`, `cli/src/shared/errors.ts`
- Test: `cli/tests/unit/errors.test.ts`

**Interfaces:**
- Produces:
  - `class MoonwellError extends Error { file?: string; line?: number; hint?: string; constructor(message: string, options?: { file?: string; line?: number; hint?: string; cause?: unknown }) }`
  - `formatError(error: unknown): string`
  - `VERSION: string` from `cli/src/version.ts`

- [ ] **Step 1: Create repo config files**

`.gitattributes`:
```
* text=auto eol=lf
template/maps/** binary
cli/tests/fixtures/** binary
```

`.gitignore`:
```
dist/
*.log
template/dist/
template/moonwell.local.pkl
```

`deno.json`:
```json
{
  "workspace": ["./cli"],
  "nodeModulesDir": "none",
  "imports": {
    "@std/assert": "jsr:@std/assert@^1.0.13",
    "@std/encoding": "jsr:@std/encoding@^1.0.10",
    "@std/fs": "jsr:@std/fs@^1.0.19",
    "@std/path": "jsr:@std/path@^1.1.2"
  },
  "tasks": {
    "test": "deno test -A cli/tests/unit",
    "test:runtime": "deno test -A cli/tests/yue",
    "test:pkl": "pkl test pkl/tests/Project.pkl && deno test -A cli/tests/pkl",
    "test:e2e": "deno test -A cli/tests/e2e",
    "check": "deno check cli/src/main.ts tools/gen.ts",
    "lint": "deno lint",
    "fmt": "deno fmt",
    "gen": "deno run -A tools/gen.ts"
  },
  "fmt": { "lineWidth": 120, "exclude": ["cli/src/embedded", "template", "docs"] },
  "lint": { "exclude": ["cli/src/embedded", "template"] }
}
```

`cli/deno.json`:
```json
{
  "name": "@moonwell/cli",
  "version": "0.1.0",
  "exports": "./src/main.ts",
  "imports": {
    "@std/assert": "jsr:@std/assert@^1.0.13",
    "@std/cli": "jsr:@std/cli@^1.0.20",
    "@std/encoding": "jsr:@std/encoding@^1.0.10",
    "@std/fs": "jsr:@std/fs@^1.0.19",
    "@std/path": "jsr:@std/path@^1.1.2"
  },
  "publish": { "include": ["src/**/*.ts", "README.md"] }
}
```

`cli/README.md`:
```markdown
# @moonwell/cli

The Moonwell command-line tool: build Warcraft III maps with YueScript gameplay and Pkl data.

    deno run -A jsr:@moonwell/cli init my-map

See https://github.com/mdlsvensson/moonwell for documentation.
```

`cli/src/version.ts`:
```ts
/** Moonwell CLI version. Must equal cli/deno.json "version" and pkl/PklProject package.version. */
export const VERSION = "0.1.0";
```

- [ ] **Step 2: Write the failing test**

`cli/tests/unit/errors.test.ts`:
```ts
import { assertEquals, assertStringIncludes } from "@std/assert";
import { formatError, MoonwellError } from "../../src/shared/errors.ts";

Deno.test("formatError prints file, line, message and hint", () => {
  const error = new MoonwellError("unexpected symbol", { file: "src/main.yue", line: 3, hint: "check the indentation" });
  assertEquals(formatError(error), "error: src/main.yue:3 › unexpected symbol\nhint: check the indentation");
});

Deno.test("formatError without location or hint", () => {
  assertEquals(formatError(new MoonwellError("no project")), "error: no project");
});

Deno.test("formatError with file but no line", () => {
  assertEquals(formatError(new MoonwellError("bad", { file: "moonwell.pkl" })), "error: moonwell.pkl › bad");
});

Deno.test("formatError treats other errors as internal", () => {
  const text = formatError(new TypeError("boom"));
  assertStringIncludes(text, "internal error:");
  assertStringIncludes(text, "boom");
  assertStringIncludes(text, "please report");
});
```

- [ ] **Step 3: Run test to verify it fails**

Run: `deno task test`
Expected: FAIL. The module `../../src/shared/errors.ts` is not found.

- [ ] **Step 4: Implement**

`cli/src/shared/errors.ts`:
```ts
/** An expected, user-facing failure. Anything else reaching the CLI is reported as an internal error. */
export class MoonwellError extends Error {
  readonly file?: string;
  readonly line?: number;
  readonly hint?: string;

  constructor(message: string, options: { file?: string; line?: number; hint?: string; cause?: unknown } = {}) {
    super(message, { cause: options.cause });
    this.name = "MoonwellError";
    this.file = options.file;
    this.line = options.line;
    this.hint = options.hint;
  }
}

/** Renders an error for the terminal and the log file. */
export function formatError(error: unknown): string {
  if (error instanceof MoonwellError) {
    const where = error.file === undefined ? "" : `${error.file}${error.line === undefined ? "" : `:${error.line}`} › `;
    return `error: ${where}${error.message}${error.hint ? `\nhint: ${error.hint}` : ""}`;
  }
  const detail = error instanceof Error ? (error.stack ?? error.message) : String(error);
  return `internal error: ${detail}\nThis is a bug in Moonwell; please report it.`;
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `deno task test`
Expected: PASS (4 tests). `deno.lock` is created. Commit it.

- [ ] **Step 6: Commit**

```bash
git add .gitattributes .gitignore deno.json deno.lock cli
git commit -m "feat: scaffold repository, MoonwellError and version

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Shared utilities (log, process, fs, lock, compression)

**Files:**
- Create: `cli/src/shared/log.ts`, `cli/src/shared/process.ts`, `cli/src/shared/fs.ts`, `cli/src/shared/lock.ts`, `cli/src/shared/compression.ts`
- Create: `cli/tests/support/logger.ts`
- Test: `cli/tests/unit/shared.test.ts`

**Interfaces:**
- Consumes: `MoonwellError` (Task 1).
- Produces:
  - `interface Logger { info(message: string): void; warn(message: string): void; error(message: string): void }`
  - `createLogger(options?: { file?: string; write?: (line: string) => void }): Logger`
  - `interface RunResult { code: number; stdout: string; stderr: string }`
  - `type Runner = (command: string, args: string[], options?: { cwd?: string; notFoundHint?: string }) => Promise<RunResult>`
  - `runProcess: Runner`
  - `toPosix(path: string): string`
  - `listFiles(dir: string): Promise<string[]>`: sorted POSIX paths relative to `dir`
  - `replaceDir(source: string, destination: string): Promise<void>`
  - `writeTextIfChanged(path: string, text: string): Promise<boolean>`
  - `removeIfExists(path: string): Promise<void>`
  - `sha256Hex(bytes: Uint8Array): Promise<string>`
  - `withBuildLock<T>(distDir: string, fn: () => Promise<T>): Promise<T>`
  - `deflate`, `inflate`, `deflateRaw`, `inflateRaw`: each `(data: Uint8Array) => Promise<Uint8Array>`
  - test helper `silentLogger(): Logger & { lines: string[] }`

- [ ] **Step 1: Write the failing tests**

`cli/tests/support/logger.ts`:
```ts
import type { Logger } from "../../src/shared/log.ts";

/** A logger that records lines instead of printing them. */
export function silentLogger(): Logger & { lines: string[] } {
  const lines: string[] = [];
  return {
    lines,
    info: (message) => lines.push(message),
    warn: (message) => lines.push(`warning: ${message}`),
    error: (message) => lines.push(message),
  };
}
```

`cli/tests/unit/shared.test.ts`:
```ts
import { assert, assertEquals, assertRejects } from "@std/assert";
import { join } from "@std/path";
import { MoonwellError } from "../../src/shared/errors.ts";
import { createLogger } from "../../src/shared/log.ts";
import { runProcess } from "../../src/shared/process.ts";
import { listFiles, removeIfExists, replaceDir, sha256Hex, writeTextIfChanged } from "../../src/shared/fs.ts";
import { withBuildLock } from "../../src/shared/lock.ts";
import { deflate, deflateRaw, inflate, inflateRaw } from "../../src/shared/compression.ts";

Deno.test("createLogger writes to the sink and appends to the log file", async () => {
  const dir = await Deno.makeTempDir();
  const file = join(dir, "nested", "moonwell.log");
  const lines: string[] = [];
  const logger = createLogger({ file, write: (line) => lines.push(line) });
  logger.info("hello");
  logger.warn("careful");
  assertEquals(lines, ["hello", "warning: careful"]);
  const text = await Deno.readTextFile(file);
  assert(text.includes("info: hello"));
  assert(text.includes("warn: warning: careful"));
});

Deno.test("runProcess captures output and exit code", async () => {
  const result = await runProcess(Deno.execPath(), ["eval", "console.log('out'); console.error('err'); Deno.exit(3)"]);
  assertEquals(result.code, 3);
  assertEquals(result.stdout.trim(), "out");
  assertEquals(result.stderr.trim(), "err");
});

Deno.test("runProcess reports a missing command as MoonwellError with the hint", async () => {
  const error = await assertRejects(
    () => runProcess("definitely-not-a-command-moonwell", [], { notFoundHint: "install it" }),
    MoonwellError,
  );
  assertEquals(error.hint, "install it");
});

Deno.test("listFiles returns sorted posix relative paths", async () => {
  const dir = await Deno.makeTempDir();
  await Deno.mkdir(join(dir, "a", "b"), { recursive: true });
  await Deno.writeTextFile(join(dir, "z.txt"), "");
  await Deno.writeTextFile(join(dir, "a", "b", "c.txt"), "");
  assertEquals(await listFiles(dir), ["a/b/c.txt", "z.txt"]);
});

Deno.test("replaceDir replaces destination contents", async () => {
  const dir = await Deno.makeTempDir();
  await Deno.mkdir(join(dir, "src"));
  await Deno.writeTextFile(join(dir, "src", "new.txt"), "new");
  await Deno.mkdir(join(dir, "dest"));
  await Deno.writeTextFile(join(dir, "dest", "old.txt"), "old");
  await replaceDir(join(dir, "src"), join(dir, "dest"));
  assertEquals(await listFiles(join(dir, "dest")), ["new.txt"]);
});

Deno.test("writeTextIfChanged only writes differing content", async () => {
  const dir = await Deno.makeTempDir();
  const file = join(dir, "x", "y.txt");
  assertEquals(await writeTextIfChanged(file, "a"), true);
  assertEquals(await writeTextIfChanged(file, "a"), false);
  assertEquals(await writeTextIfChanged(file, "b"), true);
  assertEquals(await Deno.readTextFile(file), "b");
});

Deno.test("removeIfExists ignores missing paths", async () => {
  const dir = await Deno.makeTempDir();
  await removeIfExists(join(dir, "missing"));
  await Deno.writeTextFile(join(dir, "f"), "");
  await removeIfExists(join(dir, "f"));
  assertEquals(await listFiles(dir), []);
});

Deno.test("sha256Hex hashes bytes", async () => {
  assertEquals(
    await sha256Hex(new TextEncoder().encode("abc")),
    "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad",
  );
});

Deno.test("withBuildLock rejects a concurrent build and releases afterwards", async () => {
  const dir = await Deno.makeTempDir();
  await withBuildLock(dir, async () => {
    await assertRejects(() => withBuildLock(dir, () => Promise.resolve()), MoonwellError, "Another Moonwell build");
  });
  assertEquals(await withBuildLock(dir, () => Promise.resolve(7)), 7);
});

Deno.test("compression round trips; deflate emits a zlib header", async () => {
  const data = new TextEncoder().encode("moonwell ".repeat(100));
  const zlib = await deflate(data);
  assertEquals(zlib[0], 0x78);
  assertEquals(await inflate(zlib), data);
  assertEquals(await inflateRaw(await deflateRaw(data)), data);
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `deno task test`
Expected: FAIL with module-not-found errors for the shared modules.

- [ ] **Step 3: Implement**

`cli/src/shared/log.ts`:
```ts
import { dirname } from "@std/path";

export interface Logger {
  info(message: string): void;
  warn(message: string): void;
  error(message: string): void;
}

/** Logs to stderr (or `write`) and appends timestamped lines to `file` when given. Logging never throws. */
export function createLogger(options: { file?: string; write?: (line: string) => void } = {}): Logger {
  const write = options.write ?? ((line: string) => console.error(line));
  const log = (level: "info" | "warn" | "error", message: string) => {
    write(message);
    if (options.file === undefined) return;
    try {
      Deno.mkdirSync(dirname(options.file), { recursive: true });
      Deno.writeTextFileSync(options.file, `[${new Date().toISOString()}] ${level}: ${message}\n`, { append: true });
    } catch {
      // A broken log file must never fail a build.
    }
  };
  return {
    info: (message) => log("info", message),
    warn: (message) => log("warn", `warning: ${message}`),
    error: (message) => log("error", message),
  };
}
```

`cli/src/shared/process.ts`:
```ts
import { MoonwellError } from "./errors.ts";

export interface RunResult {
  code: number;
  stdout: string;
  stderr: string;
}

export type Runner = (
  command: string,
  args: string[],
  options?: { cwd?: string; notFoundHint?: string },
) => Promise<RunResult>;

/** Runs a command to completion and captures its output. */
export const runProcess: Runner = async (command, args, options = {}) => {
  let output: Deno.CommandOutput;
  try {
    output = await new Deno.Command(command, { args, cwd: options.cwd, stdout: "piped", stderr: "piped" }).output();
  } catch (error) {
    if (error instanceof Deno.errors.NotFound) {
      throw new MoonwellError(`Cannot run '${command}': command not found.`, { hint: options.notFoundHint, cause: error });
    }
    throw error;
  }
  const decoder = new TextDecoder();
  return { code: output.code, stdout: decoder.decode(output.stdout), stderr: decoder.decode(output.stderr) };
};
```

`cli/src/shared/fs.ts`:
```ts
import { copy, walk } from "@std/fs";
import { dirname, relative, SEPARATOR } from "@std/path";

export function toPosix(path: string): string {
  return path.split(SEPARATOR).join("/");
}

/** All files below `dir`, as sorted POSIX paths relative to `dir`. */
export async function listFiles(dir: string): Promise<string[]> {
  const files: string[] = [];
  for await (const entry of walk(dir, { includeDirs: false })) files.push(toPosix(relative(dir, entry.path)));
  return files.sort();
}

export async function removeIfExists(path: string): Promise<void> {
  try {
    await Deno.remove(path, { recursive: true });
  } catch (error) {
    if (!(error instanceof Deno.errors.NotFound)) throw error;
  }
}

/** Replaces `destination` with a copy of `source`. */
export async function replaceDir(source: string, destination: string): Promise<void> {
  await removeIfExists(destination);
  await Deno.mkdir(dirname(destination), { recursive: true });
  await copy(source, destination);
}

/** Writes `text` unless the file already has exactly that content. Returns whether it wrote. */
export async function writeTextIfChanged(path: string, text: string): Promise<boolean> {
  try {
    if (await Deno.readTextFile(path) === text) return false;
  } catch (error) {
    if (!(error instanceof Deno.errors.NotFound)) throw error;
  }
  await Deno.mkdir(dirname(path), { recursive: true });
  await Deno.writeTextFile(path, text);
  return true;
}

export async function sha256Hex(bytes: Uint8Array): Promise<string> {
  const digest = await crypto.subtle.digest("SHA-256", bytes.slice());
  return Array.from(new Uint8Array(digest), (byte) => byte.toString(16).padStart(2, "0")).join("");
}
```

`cli/src/shared/lock.ts`:
```ts
import { join } from "@std/path";
import { MoonwellError } from "./errors.ts";

/** Runs `fn` while holding `<distDir>/.lock`; a second concurrent build fails fast. */
export async function withBuildLock<T>(distDir: string, fn: () => Promise<T>): Promise<T> {
  await Deno.mkdir(distDir, { recursive: true });
  const lockPath = join(distDir, ".lock");
  try {
    (await Deno.open(lockPath, { createNew: true, write: true })).close();
  } catch (error) {
    if (error instanceof Deno.errors.AlreadyExists) {
      throw new MoonwellError("Another Moonwell build is running in this project.", {
        file: lockPath,
        hint: "Wait for it to finish. If no build is running, delete the lock file.",
      });
    }
    throw error;
  }
  try {
    return await fn();
  } finally {
    await Deno.remove(lockPath).catch(() => {});
  }
}
```

`cli/src/shared/compression.ts`:
```ts
async function transform(data: Uint8Array, stream: TransformStream<Uint8Array, Uint8Array>): Promise<Uint8Array> {
  const output = new Blob([data.slice()]).stream().pipeThrough(stream);
  return new Uint8Array(await new Response(output).arrayBuffer());
}

/** zlib-wrapped deflate (what MPQ compression type 0x02 expects). */
export const deflate = (data: Uint8Array) => transform(data, new CompressionStream("deflate"));
export const inflate = (data: Uint8Array) => transform(data, new DecompressionStream("deflate"));
/** Raw deflate (what zip method 8 uses). */
export const deflateRaw = (data: Uint8Array) => transform(data, new CompressionStream("deflate-raw"));
export const inflateRaw = (data: Uint8Array) => transform(data, new DecompressionStream("deflate-raw"));
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `deno task test`
Expected: PASS (all tests in `errors.test.ts` and `shared.test.ts`). If `deno check` complains that `CompressionStream` is not assignable to `TransformStream<Uint8Array, Uint8Array>`, change the parameter type to `{ readable: ReadableStream<Uint8Array>; writable: WritableStream<Uint8Array> }` and cast at the call sites with `as unknown as`.

- [ ] **Step 5: Commit**

```bash
git add cli
git commit -m "feat: shared logging, process, fs, lock and compression helpers

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: Pkl package with the Project schema

**Files:**
- Create: `pkl/PklProject`, `pkl/Project.pkl`
- Test: `pkl/tests/Project.pkl`, `cli/tests/unit/versions-consistent.test.ts`

**Interfaces:**
- Produces:
  - the Pkl module `@moonwell/Project.pkl`, with properties `map { folder; entry }`, `build { folder; minify }`, `launch { gameExecutable?; args }`, `yue { version; path? }`;
  - its JSON rendering. Null properties are omitted; Listings become arrays.

- [ ] **Step 1: Write the failing Pkl test**

`pkl/tests/Project.pkl`:
```pkl
amends "pkl:test"

import "pkl:test" as t
import "../Project.pkl"

facts {
  ["defaults"] {
    Project.map.folder == "map.w3x"
    Project.map.entry == "src/main.yue"
    Project.build.folder == "dist/bin"
    Project.build.minify == false
    Project.launch.gameExecutable == null
    Project.launch.args.toList() == List("-launch", "-windowmode", "windowed")
    Project.yue.version == "0.34.2"
    Project.yue.path == null
  }
  ["overrides are accepted"] {
    (Project) { map { folder = "hero.w3x"; entry = "src/game/init.yue" } }.map.entry == "src/game/init.yue"
  }
  ["map folder must end in .w3x"] {
    t.catch(() -> (Project) { map { folder = "hero" } }.map.folder).contains("endsWith")
  }
  ["entry must be a .yue file under src/"] {
    t.catch(() -> (Project) { map { entry = "main.yue" } }.map.entry).contains("startsWith")
    t.catch(() -> (Project) { map { entry = "src/main.lua" } }.map.entry).contains("endsWith")
  }
  ["yue version must be semver"] {
    t.catch(() -> (Project) { yue { version = "latest" } }.yue.version).contains("matches")
  }
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `pkl test pkl/tests/Project.pkl`
Expected: FAIL. `../Project.pkl` cannot be found.

- [ ] **Step 3: Implement the package**

`pkl/PklProject`:
```pkl
amends "pkl:Project"

package {
  name = "moonwell"
  baseUri = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/\(name)"
  version = "0.1.0"
  packageZipUrl = "https://github.com/mdlsvensson/moonwell/releases/download/\(name)@\(version)/\(name)@\(version).zip"
  description = "Schemas for Moonwell Warcraft III map projects."
  license = "MIT"
}
```

`pkl/Project.pkl`:
```pkl
/// Root schema for a Moonwell project's `moonwell.pkl`.
///
/// Every property has a default; a project only writes what it changes.
/// Machine-specific values belong in `moonwell.local.pkl`, which amends `moonwell.pkl`.
module moonwell.Project

class MapConfig {
  /// Folder under `maps/` holding the World Editor source map (folder format, Lua script mode).
  folder: String(endsWith(".w3x")) = "map.w3x"

  /// Gameplay entry file, relative to the project root.
  entry: String(startsWith("src/"), endsWith(".yue")) = "src/main.yue"
}

class BuildConfig {
  /// Folder, relative to the project root, that receives the packed `.w3x`.
  folder: String = "dist/bin"

  /// Minify compiled Lua. Error positions then report module names only.
  minify: Boolean = false
}

class LaunchConfig {
  /// Path to `Warcraft III.exe`. Set this in `moonwell.local.pkl`.
  gameExecutable: String?

  /// Arguments passed before `-loadfile <map>`.
  args: Listing<String> = new { "-launch"; "-windowmode"; "windowed" }
}

class YueConfig {
  /// YueScript compiler version, downloaded and verified by Moonwell.
  version: String(matches(Regex(#"\d+\.\d+\.\d+"#))) = "0.34.2"

  /// Use this compiler binary instead of downloading one.
  path: String?
}

map: MapConfig = new {}
build: BuildConfig = new {}
launch: LaunchConfig = new {}
yue: YueConfig = new {}
```

- [ ] **Step 4: Run the Pkl tests to verify they pass**

Run: `pkl test pkl/tests/Project.pkl`
Expected: `100.0% tests pass`.

- [ ] **Step 5: Add a version-consistency unit test**

`cli/tests/unit/versions-consistent.test.ts`:
```ts
import { assertEquals } from "@std/assert";
import { fromFileUrl } from "@std/path";
import { VERSION } from "../../src/version.ts";

const repo = (path: string) => fromFileUrl(new URL(`../../../${path}`, import.meta.url));

Deno.test("cli/deno.json, pkl/PklProject and VERSION agree", async () => {
  const cli = JSON.parse(await Deno.readTextFile(repo("cli/deno.json"))) as { version: string };
  const pkl = /version = "([^"]+)"/.exec(await Deno.readTextFile(repo("pkl/PklProject")))?.[1];
  assertEquals(cli.version, VERSION);
  assertEquals(pkl, VERSION);
});
```

Run: `deno task test`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkl cli/tests/unit/versions-consistent.test.ts
git commit -m "feat: moonwell Pkl package with Project schema

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Project loading

**Files:**
- Create: `cli/src/project/project.ts`
- Test: `cli/tests/unit/project.test.ts`, `cli/tests/pkl/project.test.ts`

**Interfaces:**
- Consumes: `Runner`, `runProcess` (Task 2); `MoonwellError`, `VERSION` (Task 1).
- Produces:
  - ```ts
    interface Project {
      root: string;
      map: { folder: string; entry: string };
      build: { folder: string; minify: boolean };
      launch: { gameExecutable: string | null; args: string[] };
      yue: { version: string; path: string | null };
    }
    ```
  - `loadProject(root: string, run?: Runner): Promise<Project>`
  - `parseProject(root: string, value: unknown, file: string): Project`
  - `readPackageVersion(depsJson: string): string`
  - `checkPackageVersion(packageVersion: string, cliVersion: string): void`
  - `checkPkl(run: Runner): Promise<void>`

- [ ] **Step 1: Write the failing unit tests**

`cli/tests/unit/project.test.ts`:
```ts
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

const fakeRunner = (outputs: Record<string, { code?: number; stdout?: string; stderr?: string }>): Runner =>
async (command, args) => {
  const key = [command, ...args].join(" ");
  const match = Object.entries(outputs).find(([prefix]) => key.startsWith(prefix));
  if (!match) throw new Error(`unexpected command: ${key}`);
  return { code: match[1].code ?? 0, stdout: match[1].stdout ?? "", stderr: match[1].stderr ?? "" };
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
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test`
Expected: FAIL. `project.ts` is not found.

- [ ] **Step 3: Implement**

`cli/src/project/project.ts`:
```ts
import { exists } from "@std/fs";
import { join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { type Runner, runProcess } from "../shared/process.ts";
import { VERSION } from "../version.ts";

export interface Project {
  root: string;
  map: { folder: string; entry: string };
  build: { folder: string; minify: boolean };
  launch: { gameExecutable: string | null; args: string[] };
  yue: { version: string; path: string | null };
}

const PKL_INSTALL_HINT = "Install Pkl 0.32 or newer: https://pkl-lang.org/main/current/pkl-cli/index.html#installation";

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
    notFoundHint: PKL_INSTALL_HINT,
  });
  if (result.code !== 0) {
    throw new MoonwellError(`Evaluating ${file} failed:\n${(result.stderr || result.stdout).trim()}`, { file });
  }
  return parseProject(root, JSON.parse(result.stdout), file);
}

export async function checkPkl(run: Runner): Promise<void> {
  const result = await run("pkl", ["--version"], { notFoundHint: PKL_INSTALL_HINT });
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
  const deps = JSON.parse(depsJson) as { resolvedDependencies?: Record<string, { uri?: string }> };
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

  const data = record(value, "the manifest");
  const map = record(data.map, "map");
  const buildConfig = record(data.build, "build");
  const launch = record(data.launch, "launch");
  const yue = record(data.yue, "yue");
  return {
    root,
    map: { folder: string(map.folder, "map.folder"), entry: string(map.entry, "map.entry") },
    build: { folder: string(buildConfig.folder, "build.folder"), minify: boolean(buildConfig.minify, "build.minify") },
    launch: {
      gameExecutable: nullableString(launch.gameExecutable, "launch.gameExecutable"),
      args: strings(launch.args, "launch.args"),
    },
    yue: { version: string(yue.version, "yue.version"), path: nullableString(yue.path, "yue.path") },
  };
}
```

- [ ] **Step 4: Run unit tests to verify they pass**

Run: `deno task test`
Expected: PASS.

- [ ] **Step 5: Add the real-Pkl integration test**

`cli/tests/pkl/project.test.ts`:
```ts
import { assertEquals } from "@std/assert";
import { fromFileUrl, join } from "@std/path";
import { loadProject } from "../../src/project/project.ts";
import { runProcess } from "../../src/shared/process.ts";

const PKL_DIR = fromFileUrl(new URL("../../../pkl", import.meta.url));

Deno.test("loadProject evaluates a real project against the local package", async () => {
  const root = await Deno.makeTempDir();
  const pkl = join(root, "pkl");
  await Deno.mkdir(pkl);
  for (const file of ["PklProject", "Project.pkl"]) await Deno.copyFile(join(PKL_DIR, file), join(pkl, file));
  await Deno.writeTextFile(
    join(root, "PklProject"),
    `amends "pkl:Project"\n\ndependencies {\n  ["moonwell"] = import("pkl/PklProject")\n}\n`,
  );
  await Deno.writeTextFile(join(root, "moonwell.pkl"), `amends "@moonwell/Project.pkl"\n\nmap { folder = "hero.w3x" }\n`);
  await Deno.writeTextFile(
    join(root, "moonwell.local.pkl"),
    `amends "moonwell.pkl"\n\nlaunch { gameExecutable = "C:/wc3.exe" }\n`,
  );
  const resolved = await runProcess("pkl", ["project", "resolve"], { cwd: root });
  assertEquals(resolved.code, 0, resolved.stderr);

  const project = await loadProject(root);
  assertEquals(project.map.folder, "hero.w3x");
  assertEquals(project.launch.gameExecutable, "C:/wc3.exe");
  assertEquals(project.yue, { version: "0.34.2", path: null });
});
```

Run: `deno task test:pkl`
Expected: the Pkl facts pass and the Deno test passes.

- [ ] **Step 6: Commit**

```bash
git add cli
git commit -m "feat: load and validate moonwell.pkl projects

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Zip extraction

**Files:**
- Create: `cli/src/yue/unzip.ts`, `cli/tests/support/zip.ts`
- Test: `cli/tests/unit/unzip.test.ts`

**Interfaces:**
- Consumes: `inflateRaw`, `deflateRaw` (Task 2); `MoonwellError`.
- Produces:
  - `extractZip(bytes: Uint8Array): Promise<Map<string, Uint8Array>>`: file entries only, keyed by the path stored in the zip
  - test helper `makeZip(entries: Array<{ name: string; data: Uint8Array; deflate?: boolean }>): Promise<Uint8Array>`

- [ ] **Step 1: Write the test helper and the failing test**

`cli/tests/support/zip.ts`:
```ts
import { deflateRaw } from "../../src/shared/compression.ts";

/** Minimal zip writer for tests (CRC fields are zero; the reader under test does not check them). */
export async function makeZip(entries: Array<{ name: string; data: Uint8Array; deflate?: boolean }>): Promise<Uint8Array> {
  const encoder = new TextEncoder();
  const locals: Uint8Array[] = [];
  const centrals: Uint8Array[] = [];
  let offset = 0;
  for (const entry of entries) {
    const name = encoder.encode(entry.name);
    const body = entry.deflate ? await deflateRaw(entry.data) : entry.data;
    const method = entry.deflate ? 8 : 0;
    const local = new Uint8Array(30 + name.length + body.length);
    const lv = new DataView(local.buffer);
    lv.setUint32(0, 0x04034b50, true);
    lv.setUint16(4, 20, true);
    lv.setUint16(8, method, true);
    lv.setUint32(18, body.length, true);
    lv.setUint32(22, entry.data.length, true);
    lv.setUint16(26, name.length, true);
    local.set(name, 30);
    local.set(body, 30 + name.length);
    const central = new Uint8Array(46 + name.length);
    const cv = new DataView(central.buffer);
    cv.setUint32(0, 0x02014b50, true);
    cv.setUint16(4, 20, true);
    cv.setUint16(6, 20, true);
    cv.setUint16(10, method, true);
    cv.setUint32(20, body.length, true);
    cv.setUint32(24, entry.data.length, true);
    cv.setUint16(28, name.length, true);
    cv.setUint32(42, offset, true);
    central.set(name, 46);
    locals.push(local);
    centrals.push(central);
    offset += local.length;
  }
  const centralSize = centrals.reduce((sum, part) => sum + part.length, 0);
  const end = new Uint8Array(22);
  const ev = new DataView(end.buffer);
  ev.setUint32(0, 0x06054b50, true);
  ev.setUint16(8, entries.length, true);
  ev.setUint16(10, entries.length, true);
  ev.setUint32(12, centralSize, true);
  ev.setUint32(16, offset, true);
  const parts = [...locals, ...centrals, end];
  const out = new Uint8Array(parts.reduce((sum, part) => sum + part.length, 0));
  let at = 0;
  for (const part of parts) {
    out.set(part, at);
    at += part.length;
  }
  return out;
}
```

`cli/tests/unit/unzip.test.ts`:
```ts
import { assertEquals, assertRejects } from "@std/assert";
import { extractZip } from "../../src/yue/unzip.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { makeZip } from "../support/zip.ts";

const text = (value: string) => new TextEncoder().encode(value);

Deno.test("extractZip reads stored and deflated entries and skips directories", async () => {
  const zip = await makeZip([
    { name: "bin/", data: new Uint8Array() },
    { name: "bin/yue", data: text("binary ".repeat(50)), deflate: true },
    { name: "README", data: text("hello") },
  ]);
  const files = await extractZip(zip);
  assertEquals([...files.keys()], ["bin/yue", "README"]);
  assertEquals(files.get("bin/yue"), text("binary ".repeat(50)));
  assertEquals(files.get("README"), text("hello"));
});

Deno.test("extractZip rejects data that is not a zip", async () => {
  await assertRejects(() => extractZip(text("not a zip at all, definitely not")), MoonwellError, "Invalid zip");
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `deno task test`
Expected: FAIL. `unzip.ts` is not found.

- [ ] **Step 3: Implement**

`cli/src/yue/unzip.ts`:
```ts
import { MoonwellError } from "../shared/errors.ts";
import { inflateRaw } from "../shared/compression.ts";

const END_OF_CENTRAL_DIRECTORY = 0x06054b50;
const CENTRAL_FILE_HEADER = 0x02014b50;
const LOCAL_FILE_HEADER = 0x04034b50;

/** Extracts every file entry (stored or deflated) of a non-ZIP64 archive. */
export async function extractZip(bytes: Uint8Array): Promise<Map<string, Uint8Array>> {
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  let end = -1;
  for (let i = bytes.length - 22; i >= Math.max(0, bytes.length - 22 - 0xffff); i--) {
    if (view.getUint32(i, true) === END_OF_CENTRAL_DIRECTORY) {
      end = i;
      break;
    }
  }
  if (end < 0) throw new MoonwellError("Invalid zip archive: end of central directory not found.");
  const count = view.getUint16(end + 10, true);
  let offset = view.getUint32(end + 16, true);
  if (count === 0xffff || offset === 0xffffffff) throw new MoonwellError("ZIP64 archives are not supported.");

  const decoder = new TextDecoder();
  const files = new Map<string, Uint8Array>();
  for (let i = 0; i < count; i++) {
    if (view.getUint32(offset, true) !== CENTRAL_FILE_HEADER) {
      throw new MoonwellError("Invalid zip archive: bad central directory entry.");
    }
    const method = view.getUint16(offset + 10, true);
    const compressedSize = view.getUint32(offset + 20, true);
    const nameLength = view.getUint16(offset + 28, true);
    const extraLength = view.getUint16(offset + 30, true);
    const commentLength = view.getUint16(offset + 32, true);
    const localOffset = view.getUint32(offset + 42, true);
    const name = decoder.decode(bytes.subarray(offset + 46, offset + 46 + nameLength));
    offset += 46 + nameLength + extraLength + commentLength;
    if (name.endsWith("/")) continue;

    if (view.getUint32(localOffset, true) !== LOCAL_FILE_HEADER) {
      throw new MoonwellError(`Invalid zip archive: bad local header for ${name}.`);
    }
    const dataStart = localOffset + 30 + view.getUint16(localOffset + 26, true) + view.getUint16(localOffset + 28, true);
    const data = bytes.subarray(dataStart, dataStart + compressedSize);
    if (method === 0) files.set(name, data.slice());
    else if (method === 8) files.set(name, await inflateRaw(data));
    else throw new MoonwellError(`Unsupported zip compression method ${method} for ${name}.`);
  }
  return files;
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `deno task test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli
git commit -m "feat: minimal zip extraction for compiler downloads

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: YueScript compiler install

**Files:**
- Create: `cli/src/yue/versions.ts`, `cli/src/yue/install.ts`
- Test: `cli/tests/unit/install.test.ts`

**Interfaces:**
- Consumes: `extractZip` (Task 5); `Runner`, `runProcess`, `sha256Hex`, `removeIfExists`, `Logger` (Task 2).
- Produces:
  - `type Platform = "windows-x86_64" | "linux-x86_64"`
  - `interface YueAsset { url: string; sha256: string; archive: "7z" | "zip"; binary: string }`
  - `type KnownVersions = Record<string, Partial<Record<Platform, YueAsset>>>`
  - `KNOWN_YUE: KnownVersions`
  - `DEFAULT_YUE_VERSION = "0.34.2"`
  - `currentPlatform(os?: string, arch?: string): Platform | undefined`
  - ```ts
    interface InstallDeps {
      fetch: (url: string) => Promise<Response>;
      run: Runner;
      cacheRoot: string;
      platform: Platform | undefined;
      known: KnownVersions;
      logger: Logger;
    }
    ```
  - `defaultInstallDeps(logger: Logger, run?: Runner): InstallDeps`
  - `defaultCacheRoot(): string`
  - `ensureYue(config: { version: string; path: string | null }, deps: InstallDeps): Promise<string>`: returns the binary path
  - `yueVersion(binary: string, run: Runner): Promise<string | undefined>`

- [ ] **Step 1: Write the failing tests**

`cli/tests/unit/install.test.ts`:
```ts
import { assertEquals, assertRejects } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { MoonwellError } from "../../src/shared/errors.ts";
import { sha256Hex } from "../../src/shared/fs.ts";
import type { Runner } from "../../src/shared/process.ts";
import { currentPlatform, KNOWN_YUE } from "../../src/yue/versions.ts";
import { ensureYue, type InstallDeps } from "../../src/yue/install.ts";
import { makeZip } from "../support/zip.ts";
import { silentLogger } from "../support/logger.ts";

const versionRunner = (version: string): Runner => () =>
  Promise.resolve({ code: 0, stdout: `Yuescript version: ${version}\n`, stderr: "" });

async function setup(options: { sha?: string } = {}) {
  const zip = await makeZip([{ name: "yue", data: new TextEncoder().encode("fake-binary"), deflate: true }]);
  const cacheRoot = await Deno.makeTempDir();
  let fetches = 0;
  const deps: InstallDeps = {
    fetch: () => {
      fetches++;
      return Promise.resolve(new Response(zip.slice()));
    },
    run: versionRunner("9.9.9"),
    cacheRoot,
    platform: "linux-x86_64",
    known: {
      "9.9.9": {
        "linux-x86_64": {
          url: "https://example.test/yue.zip",
          sha256: options.sha ?? await sha256Hex(zip),
          archive: "zip",
          binary: "yue",
        },
      },
    },
    logger: silentLogger(),
  };
  return { deps, cacheRoot, fetchCount: () => fetches };
}

Deno.test("ensureYue downloads, verifies and caches the compiler once", async () => {
  const { deps, cacheRoot, fetchCount } = await setup();
  const binary = await ensureYue({ version: "9.9.9", path: null }, deps);
  assertEquals(binary, join(cacheRoot, "yue", "9.9.9", "yue"));
  assertEquals(await Deno.readTextFile(binary), "fake-binary");
  await ensureYue({ version: "9.9.9", path: null }, deps);
  assertEquals(fetchCount(), 1);
});

Deno.test("ensureYue rejects a checksum mismatch and installs nothing", async () => {
  const { deps, cacheRoot } = await setup({ sha: "0".repeat(64) });
  await assertRejects(() => ensureYue({ version: "9.9.9", path: null }, deps), MoonwellError, "checksum");
  assertEquals(await exists(join(cacheRoot, "yue", "9.9.9")), false);
});

Deno.test("ensureYue lists known versions for an unknown one", async () => {
  const { deps } = await setup();
  await assertRejects(() => ensureYue({ version: "1.0.0", path: null }, deps), MoonwellError, "9.9.9");
});

Deno.test("ensureYue asks for yue.path on unsupported platforms", async () => {
  const { deps } = await setup();
  const error = await assertRejects(
    () => ensureYue({ version: "9.9.9", path: null }, { ...deps, platform: undefined }),
    MoonwellError,
  );
  assertEquals(error.hint?.includes("yue.path"), true);
});

Deno.test("ensureYue uses yue.path and warns on a version mismatch", async () => {
  const { deps } = await setup();
  const dir = await Deno.makeTempDir();
  const local = join(dir, "yue");
  await Deno.writeTextFile(local, "");
  const logger = silentLogger();
  const binary = await ensureYue({ version: "9.9.9", path: local }, { ...deps, run: versionRunner("0.1.0"), logger });
  assertEquals(binary, local);
  assertEquals(logger.lines.some((line) => line.includes("0.1.0")), true);
});

Deno.test("known versions pin 0.34.2 for Windows and Linux", () => {
  assertEquals(KNOWN_YUE["0.34.2"]["windows-x86_64"]?.sha256, "367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf");
  assertEquals(KNOWN_YUE["0.34.2"]["linux-x86_64"]?.sha256, "fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d");
  assertEquals(currentPlatform("windows", "x86_64"), "windows-x86_64");
  assertEquals(currentPlatform("darwin", "aarch64"), undefined);
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test`
Expected: FAIL. The yue modules are not found.

- [ ] **Step 3: Implement**

`cli/src/yue/versions.ts`:
```ts
export type Platform = "windows-x86_64" | "linux-x86_64";

export interface YueAsset {
  url: string;
  sha256: string;
  archive: "7z" | "zip";
  /** Path of the compiler inside the archive. */
  binary: string;
}

export type KnownVersions = Record<string, Partial<Record<Platform, YueAsset>>>;

export const DEFAULT_YUE_VERSION = "0.34.2";

const RELEASES = "https://github.com/IppClub/YueScript/releases/download";

/** Compiler builds Moonwell can install, with checksums verified at pin time. */
export const KNOWN_YUE: KnownVersions = {
  "0.34.2": {
    "windows-x86_64": {
      url: `${RELEASES}/v0.34.2/yue-windows-x64.7z`,
      sha256: "367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf",
      archive: "7z",
      binary: "yue.exe",
    },
    "linux-x86_64": {
      url: `${RELEASES}/v0.34.2/yue-linux-x86_64.zip`,
      sha256: "fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d",
      archive: "zip",
      binary: "yue",
    },
  },
};

export function currentPlatform(os: string = Deno.build.os, arch: string = Deno.build.arch): Platform | undefined {
  if (arch !== "x86_64") return undefined;
  if (os === "windows") return "windows-x86_64";
  if (os === "linux") return "linux-x86_64";
  return undefined;
}
```

`cli/src/yue/install.ts`:
```ts
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { removeIfExists, sha256Hex } from "../shared/fs.ts";
import type { Logger } from "../shared/log.ts";
import { type Runner, runProcess } from "../shared/process.ts";
import { extractZip } from "./unzip.ts";
import { currentPlatform, KNOWN_YUE, type KnownVersions, type Platform } from "./versions.ts";

export interface InstallDeps {
  fetch: (url: string) => Promise<Response>;
  run: Runner;
  cacheRoot: string;
  platform: Platform | undefined;
  known: KnownVersions;
  logger: Logger;
}

/** Per-user cache: MOONWELL_CACHE, else %LOCALAPPDATA%\moonwell, else $XDG_CACHE_HOME/moonwell or ~/.cache/moonwell. */
export function defaultCacheRoot(): string {
  const override = Deno.env.get("MOONWELL_CACHE");
  if (override) return override;
  if (Deno.build.os === "windows") {
    const local = Deno.env.get("LOCALAPPDATA");
    if (local) return join(local, "moonwell");
  }
  const xdg = Deno.env.get("XDG_CACHE_HOME");
  if (xdg) return join(xdg, "moonwell");
  return join(Deno.env.get("HOME") ?? Deno.env.get("USERPROFILE") ?? ".", ".cache", "moonwell");
}

export function defaultInstallDeps(logger: Logger, run: Runner = runProcess): InstallDeps {
  return {
    fetch: (url) => fetch(url),
    run,
    cacheRoot: defaultCacheRoot(),
    platform: currentPlatform(),
    known: KNOWN_YUE,
    logger,
  };
}

export async function yueVersion(binary: string, run: Runner): Promise<string | undefined> {
  const result = await run(binary, ["-v"]);
  return /Yuescript version: (\S+)/.exec(result.stdout + result.stderr)?.[1];
}

/** Returns a path to a verified compiler for `config`, installing it into the cache when needed. */
export async function ensureYue(config: { version: string; path: string | null }, deps: InstallDeps): Promise<string> {
  if (config.path !== null) {
    if (!(await exists(config.path))) {
      throw new MoonwellError(`yue.path does not exist: ${config.path}`, { file: "moonwell.local.pkl" });
    }
    const found = await yueVersion(config.path, deps.run);
    if (found !== config.version) {
      deps.logger.warn(`yue.path reports version ${found ?? "unknown"}, expected ${config.version}.`);
    }
    return config.path;
  }

  const versions = deps.known[config.version];
  if (!versions) {
    throw new MoonwellError(`Unknown YueScript version ${config.version}.`, {
      file: "moonwell.pkl",
      hint: `Known versions: ${Object.keys(deps.known).join(", ")}. Or set yue.path to a local compiler.`,
    });
  }
  const asset = deps.platform === undefined ? undefined : versions[deps.platform];
  if (!asset) {
    throw new MoonwellError(`Moonwell cannot download YueScript for this platform (${Deno.build.os}/${Deno.build.arch}).`, {
      hint: "Build or install yue yourself and set yue.path in moonwell.local.pkl.",
    });
  }

  const installDir = join(deps.cacheRoot, "yue", config.version);
  const binary = join(installDir, asset.binary);
  if (await exists(binary)) return binary;

  deps.logger.info(`Downloading YueScript ${config.version}...`);
  const response = await deps.fetch(asset.url).catch((cause) => {
    throw new MoonwellError(`Downloading ${asset.url} failed.`, { cause, hint: "Check your connection and retry, or set yue.path." });
  });
  if (!response.ok) {
    throw new MoonwellError(`Downloading ${asset.url} failed with HTTP ${response.status}.`, {
      hint: "Retry later, or set yue.path in moonwell.local.pkl.",
    });
  }
  const archive = new Uint8Array(await response.arrayBuffer());
  const actual = await sha256Hex(archive);
  if (actual !== asset.sha256) {
    throw new MoonwellError(`YueScript download checksum mismatch (expected ${asset.sha256}, got ${actual}).`, {
      hint: "Retry the download. If it keeps failing, report it; do not bypass the check.",
    });
  }

  await Deno.mkdir(dirname(installDir), { recursive: true });
  const staging = await Deno.makeTempDir({ dir: dirname(installDir), prefix: ".install-" });
  try {
    if (asset.archive === "zip") {
      for (const [name, data] of await extractZip(archive)) {
        const target = join(staging, ...name.split("/"));
        await Deno.mkdir(dirname(target), { recursive: true });
        await Deno.writeFile(target, data);
      }
    } else {
      const archivePath = join(staging, "archive.7z");
      await Deno.writeFile(archivePath, archive);
      const tar = join(Deno.env.get("SystemRoot") ?? "C:\\Windows", "System32", "tar.exe");
      const result = await deps.run(tar, ["-xf", archivePath, "-C", staging]);
      if (result.code !== 0) throw new MoonwellError(`Extracting YueScript failed:\n${result.stderr.trim()}`);
      await Deno.remove(archivePath);
    }
    const stagedBinary = join(staging, asset.binary);
    if (!(await exists(stagedBinary))) throw new MoonwellError(`The YueScript archive has no ${asset.binary}.`);
    if (Deno.build.os !== "windows") await Deno.chmod(stagedBinary, 0o755);
    const found = await yueVersion(stagedBinary, deps.run);
    if (found !== config.version) {
      throw new MoonwellError(`Downloaded compiler reports version ${found ?? "unknown"}, expected ${config.version}.`);
    }
    try {
      await Deno.rename(staging, installDir);
    } catch (error) {
      // Another process finished the same install first; keep theirs.
      if (!(await exists(binary))) throw error;
    }
  } finally {
    await removeIfExists(staging);
  }
  return binary;
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `deno task test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli
git commit -m "feat: download, verify and cache the YueScript compiler

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 7: Compile YueScript sources incrementally

**Files:**
- Create: `cli/src/yue/compile.ts`, `cli/tests/support/yue.ts`
- Test: `cli/tests/yue/compile.test.ts`

**Interfaces:**
- Consumes: `ensureYue`, `defaultInstallDeps` (Task 6); `DEFAULT_YUE_VERSION`; `listFiles`, `sha256Hex`, `removeIfExists`, `Runner`, `runProcess` (Task 2).
- Produces:
  - `interface CompiledModule { name: string; sourcePath: string; source: string }`. `name` is the dotted module name. `sourcePath` is a POSIX path relative to the project root (e.g. `src/a/b.yue`).
  - `interface CompileOutput { outDir: string; load(name: string): CompiledModule | undefined }`
  - `compileSources(options: { yue: string; root: string; minify: boolean; run?: Runner; concurrency?: number }): Promise<CompileOutput>`
  - Output goes to `<root>/dist/stage/lua/`, and the manifest to `<root>/dist/stage/lua/.hashes.json`.
  - test helper `testYue(): Promise<string>`

- [ ] **Step 1: Write the helper and the failing tests**

`cli/tests/support/yue.ts`:
```ts
import { defaultInstallDeps, ensureYue } from "../../src/yue/install.ts";
import { DEFAULT_YUE_VERSION } from "../../src/yue/versions.ts";
import { silentLogger } from "./logger.ts";

/** A real compiler for tests: MOONWELL_TEST_YUE, else the pinned version from the user cache (downloads once). */
export function testYue(): Promise<string> {
  return ensureYue(
    { version: DEFAULT_YUE_VERSION, path: Deno.env.get("MOONWELL_TEST_YUE") ?? null },
    defaultInstallDeps(silentLogger()),
  );
}
```

`cli/tests/yue/compile.test.ts`:
```ts
import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { MoonwellError } from "../../src/shared/errors.ts";
import { type Runner, runProcess } from "../../src/shared/process.ts";
import { compileSources } from "../../src/yue/compile.ts";
import { testYue } from "../support/yue.ts";

async function project(files: Record<string, string>): Promise<string> {
  const root = await Deno.makeTempDir();
  for (const [path, text] of Object.entries(files)) {
    const full = join(root, path);
    await Deno.mkdir(dirname(full), { recursive: true });
    await Deno.writeTextFile(full, text);
  }
  return root;
}

function countingRunner(): { run: Runner; compiled: string[] } {
  const compiled: string[] = [];
  const run: Runner = (command, args, options) => {
    compiled.push(args[args.length - 1]);
    return runProcess(command, args, options);
  };
  return { run, compiled };
}

Deno.test("compileSources compiles modules and loads them by dotted name", async () => {
  const yue = await testYue();
  const root = await project({
    "src/main.yue": 'import "util.math" as M\nexport answer = M.double 21\n',
    "src/util/math.yue": "export double = (x) -> x * 2\n",
  });
  const output = await compileSources({ yue, root, minify: false });
  const main = output.load("main")!;
  assertEquals(main.sourcePath, "src/main.yue");
  assertStringIncludes(main.source, 'require("util.math")');
  assertEquals(output.load("util.math")?.sourcePath, "src/util/math.yue");
  assertEquals(output.load("missing"), undefined);
});

Deno.test("compileSources only recompiles changed files and removes deleted outputs", async () => {
  const yue = await testYue();
  const root = await project({ "src/a.yue": "export x = 1\n", "src/b.yue": "export y = 2\n" });
  await compileSources({ yue, root, minify: false });

  await Deno.writeTextFile(join(root, "src/a.yue"), "export x = 3\n");
  await Deno.remove(join(root, "src/b.yue"));
  const counted = countingRunner();
  const output = await compileSources({ yue, root, minify: false, run: counted.run });
  assertEquals(counted.compiled.length, 1);
  assert(counted.compiled[0].endsWith("a.yue"));
  assertStringIncludes(output.load("a")!.source, "3");
  assertEquals(await exists(join(root, "dist/stage/lua/b.lua")), false);

  const minified = countingRunner();
  await compileSources({ yue, root, minify: true, run: minified.run });
  assertEquals(minified.compiled.length, 1);
});

Deno.test("compileSources reports syntax errors with file and line", async () => {
  const yue = await testYue();
  const root = await project({ "src/ok.yue": "export x = 1\n", "src/bad.yue": "x = 1\ny = \n  if then\n" });
  const error = await assertRejects(() => compileSources({ yue, root, minify: false }), MoonwellError);
  assertEquals(error.file, "src/bad.yue");
  assertEquals(error.line, 2);
});

Deno.test("compileSources rejects dots in file names", async () => {
  const yue = await testYue();
  const root = await project({ "src/a.b.yue": "export x = 1\n" });
  await assertRejects(() => compileSources({ yue, root, minify: false }), MoonwellError, "dots");
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test:runtime`
Expected: FAIL. `compile.ts` is not found. The first run may also download yue into the cache.

- [ ] **Step 3: Implement**

`cli/src/yue/compile.ts`:
```ts
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { listFiles, removeIfExists, sha256Hex } from "../shared/fs.ts";
import { type Runner, runProcess } from "../shared/process.ts";

export interface CompiledModule {
  /** Dotted module name, e.g. "heroes.captain". */
  name: string;
  /** POSIX path relative to the project root, e.g. "src/heroes/captain.yue". */
  sourcePath: string;
  source: string;
}

export interface CompileOutput {
  outDir: string;
  load(name: string): CompiledModule | undefined;
}

interface Manifest {
  settings: string;
  files: Record<string, string>;
}

/** Compiles every .yue file under src/ into dist/stage/lua, recompiling only changed files. */
export async function compileSources(options: {
  yue: string;
  root: string;
  minify: boolean;
  run?: Runner;
  concurrency?: number;
}): Promise<CompileOutput> {
  const run = options.run ?? runProcess;
  const srcDir = join(options.root, "src");
  const outDir = join(options.root, "dist", "stage", "lua");
  if (!(await exists(srcDir))) throw new MoonwellError("The src/ folder is missing.", { file: options.root });

  const sources = (await listFiles(srcDir)).filter((file) => file.endsWith(".yue"));
  for (const file of sources) {
    if (file.slice(0, -4).split("/").some((segment) => segment.includes("."))) {
      throw new MoonwellError("Module file and folder names cannot contain dots.", {
        file: `src/${file}`,
        hint: "Dots separate module names in `import`; rename the file or folder.",
      });
    }
  }

  const manifestPath = join(outDir, ".hashes.json");
  const previous = await readManifest(manifestPath);
  const settings = `${options.yue}|${options.minify ? "minify" : "rewrite"}`;
  const hashes: Record<string, string> = {};
  const pending: string[] = [];
  for (const file of sources) {
    hashes[file] = await sha256Hex(await Deno.readFile(join(srcDir, file)));
    const upToDate = previous?.settings === settings && previous.files[file] === hashes[file] &&
      await exists(join(outDir, luaPath(file)));
    if (!upToDate) pending.push(file);
  }
  for (const file of Object.keys(previous?.files ?? {})) {
    if (!(file in hashes)) await removeIfExists(join(outDir, luaPath(file)));
  }

  const failures: MoonwellError[] = [];
  await forEachLimited(pending, options.concurrency ?? 8, async (file) => {
    const output = join(outDir, luaPath(file));
    await Deno.mkdir(dirname(output), { recursive: true });
    const mode = options.minify ? "-m" : "-r";
    const result = await run(options.yue, ["--target=5.3", mode, "-o", output, join(srcDir, file)]);
    if (result.code !== 0) {
      delete hashes[file];
      await removeIfExists(output);
      failures.push(compileError(`src/${file}`, `${result.stdout}\n${result.stderr}`));
    }
  });

  await Deno.mkdir(outDir, { recursive: true });
  await Deno.writeTextFile(manifestPath, JSON.stringify({ settings, files: hashes }, null, 2));
  if (failures.length > 0) {
    failures.sort((a, b) => (a.file ?? "").localeCompare(b.file ?? ""));
    if (failures.length === 1) throw failures[0];
    throw new MoonwellError(`${failures[0].message}\n(${failures.length - 1} more file(s) failed to compile)`, {
      file: failures[0].file,
      line: failures[0].line,
    });
  }

  return {
    outDir,
    load(name: string): CompiledModule | undefined {
      const relative = name.split(".").join("/");
      try {
        return {
          name,
          sourcePath: `src/${relative}.yue`,
          source: Deno.readTextFileSync(join(outDir, `${relative}.lua`)),
        };
      } catch (error) {
        if (error instanceof Deno.errors.NotFound) return undefined;
        throw error;
      }
    },
  };
}

function luaPath(file: string): string {
  return file.replace(/\.yue$/, ".lua");
}

async function readManifest(path: string): Promise<Manifest | undefined> {
  try {
    return JSON.parse(await Deno.readTextFile(path)) as Manifest;
  } catch {
    return undefined;
  }
}

/** yue prints "Failed to compile: <file>", then "<line>: <message>" and a source excerpt. */
function compileError(file: string, output: string): MoonwellError {
  const detail = output.split(/\r?\n/).filter((line) => !line.startsWith("Failed to compile")).join("\n").trim();
  const match = /^(\d+): (.+)$/m.exec(output);
  return new MoonwellError(match ? `${match[2]}\n${detail}` : detail || "YueScript compilation failed.", {
    file,
    line: match ? Number(match[1]) : undefined,
  });
}

async function forEachLimited<T>(items: T[], limit: number, fn: (item: T) => Promise<void>): Promise<void> {
  let next = 0;
  const workers = Array.from({ length: Math.min(limit, items.length) }, async () => {
    while (next < items.length) await fn(items[next++]);
  });
  await Promise.all(workers);
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `deno task test:runtime`
Expected: PASS (4 tests). If the syntax-error test reports line 3 instead of 2, change the test's expected line to the line yue prints for `y = ` followed by a bad `if`. The requirement is that the error carries the file and the line number yue reports.

- [ ] **Step 5: Commit**

```bash
git add cli
git commit -m "feat: incremental YueScript compilation with located errors

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 8: Lua lexer and require detection

**Files:**
- Create: `cli/src/bundle/lexer.ts`
- Test: `cli/tests/unit/lexer.test.ts`

**Interfaces:**
- Produces:
  - `interface Token { kind: "name" | "string" | "punct"; value: string; line: number; escaped?: boolean }`
  - `tokenize(source: string): Token[]`
  - `interface RequireCall { line: number; name?: string }`. `name` is undefined when the argument is not a single plain string literal.
  - `findRequires(source: string): RequireCall[]`

- [ ] **Step 1: Write the failing tests**

`cli/tests/unit/lexer.test.ts`:
```ts
import { assertEquals } from "@std/assert";
import { findRequires, tokenize } from "../../src/bundle/lexer.ts";

Deno.test("finds literal requires in all call forms with line numbers", () => {
  const source = 'local a = require("a.b")\n\nlocal c = require "c"\nlocal d = require [[d.e]]\n';
  assertEquals(findRequires(source), [
    { line: 1, name: "a.b" },
    { line: 3, name: "c" },
    { line: 4, name: "d.e" },
  ]);
});

Deno.test("ignores require inside comments and strings", () => {
  const source = [
    '-- require("nope")',
    "--[[ require('nope')",
    "]]",
    '--[==[ require("nope") ]==]',
    "local s = \"require('nope')\"",
    "local t = [[",
    'require("nope")',
    "]]",
    'local real = require("yes")',
  ].join("\n");
  assertEquals(findRequires(source), [{ line: 9, name: "yes" }]);
});

Deno.test("marks non-literal requires as dynamic", () => {
  assertEquals(findRequires('local m = require(prefix .. "x")\nlocal n = require("a" .. b)'), [
    { line: 1 },
    { line: 2 },
  ]);
});

Deno.test("treats escaped string literals as dynamic", () => {
  assertEquals(findRequires('require("a\\65")'), [{ line: 1 }]);
});

Deno.test("skips method calls, field access and redefinitions", () => {
  const source = 'obj:require("x")\nobj.require("y")\nlocal function require(n) end\nlocal require = f\nf(require)';
  assertEquals(findRequires(source), []);
});

Deno.test("does not mistake concatenation for field access", () => {
  assertEquals(findRequires('local s = "a" .. require("b")'), [{ line: 1, name: "b" }]);
});

Deno.test("tokenize tracks lines across long strings and escaped newlines", () => {
  const tokens = tokenize('x = [[\n\n]]\ny = "a\\\nb"\nz');
  assertEquals(tokens.find((token) => token.value === "y")?.line, 4);
  assertEquals(tokens.find((token) => token.value === "z")?.line, 6);
});

Deno.test("numbers with exponents do not produce tokens", () => {
  assertEquals(tokenize("x = 1e-5 + 0x1F").map((token) => token.value), ["x", "=", "+"]);
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test`
Expected: FAIL. `lexer.ts` is not found.

- [ ] **Step 3: Implement**

`cli/src/bundle/lexer.ts`:
```ts
export interface Token {
  kind: "name" | "string" | "punct";
  value: string;
  line: number;
  /** True for quoted strings containing escape sequences (their value is left raw). */
  escaped?: boolean;
}

export interface RequireCall {
  line: number;
  /** Module name; undefined when the argument is not a single plain string literal. */
  name?: string;
}

const NAME_START = /[A-Za-z_]/;
const NAME_PART = /[A-Za-z0-9_]/;
const DIGIT = /[0-9]/;

/** Tokenizes Lua 5.3 source, dropping comments, whitespace and numbers. */
export function tokenize(source: string): Token[] {
  const tokens: Token[] = [];
  const length = source.length;
  let i = 0;
  let line = 1;

  /** At a "[", returns the long-bracket level ("[==[" is 2), or -1 when it is not a long bracket. */
  const longBracketLevel = (at: number): number => {
    let j = at + 1;
    let level = 0;
    while (source[j] === "=") {
      level++;
      j++;
    }
    return source[j] === "[" ? level : -1;
  };

  const readLong = (at: number, level: number): { end: number; text: string } => {
    const open = at + level + 2;
    const close = `]${"=".repeat(level)}]`;
    const found = source.indexOf(close, open);
    const stop = found < 0 ? length : found;
    const text = source.slice(open, stop);
    for (const char of text) if (char === "\n") line++;
    return { end: found < 0 ? length : found + close.length, text };
  };

  while (i < length) {
    const char = source[i];
    if (char === "\n") {
      line++;
      i++;
    } else if (char === " " || char === "\t" || char === "\r" || char === "\f" || char === "\v") {
      i++;
    } else if (char === "-" && source[i + 1] === "-") {
      const level = source[i + 2] === "[" ? longBracketLevel(i + 2) : -1;
      if (level >= 0) {
        i = readLong(i + 2, level).end;
      } else {
        while (i < length && source[i] !== "\n") i++;
      }
    } else if (char === "[" && longBracketLevel(i) >= 0) {
      const startLine = line;
      const { end, text } = readLong(i, longBracketLevel(i));
      tokens.push({ kind: "string", value: text.replace(/^\r?\n/, ""), line: startLine });
      i = end;
    } else if (char === '"' || char === "'") {
      const startLine = line;
      let j = i + 1;
      let value = "";
      let escaped = false;
      while (j < length && source[j] !== char && source[j] !== "\n") {
        if (source[j] === "\\") {
          escaped = true;
          if (source[j + 1] === "\n") line++;
          value += source.slice(j, j + 2);
          j += 2;
        } else {
          value += source[j];
          j++;
        }
      }
      tokens.push({ kind: "string", value, line: startLine, escaped });
      i = j + 1;
    } else if (NAME_START.test(char)) {
      let j = i + 1;
      while (j < length && NAME_PART.test(source[j])) j++;
      tokens.push({ kind: "name", value: source.slice(i, j), line });
      i = j;
    } else if (DIGIT.test(char) || (char === "." && DIGIT.test(source[i + 1] ?? ""))) {
      let j = i + 1;
      while (
        j < length &&
        (/[0-9A-Za-z_.]/.test(source[j]) || ((source[j] === "+" || source[j] === "-") && /[eEpP]/.test(source[j - 1])))
      ) j++;
      i = j;
    } else if (char === ".") {
      let j = i;
      while (source[j] === "." && j - i < 3) j++;
      tokens.push({ kind: "punct", value: source.slice(i, j), line });
      i = j;
    } else {
      tokens.push({ kind: "punct", value: char, line });
      i++;
    }
  }
  return tokens;
}

/** Finds calls to the global `require`, the only form Moonwell's bundler follows. */
export function findRequires(source: string): RequireCall[] {
  const tokens = tokenize(source);
  const calls: RequireCall[] = [];
  const literal = (token: Token, line: number): RequireCall => token.escaped ? { line } : { line, name: token.value };

  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i];
    if (token.kind !== "name" || token.value !== "require") continue;
    const previous = tokens[i - 1];
    if (previous?.kind === "punct" && (previous.value === "." || previous.value === ":")) continue;
    if (previous?.kind === "name" && (previous.value === "function" || previous.value === "local")) continue;

    const next = tokens[i + 1];
    if (next?.kind === "string") {
      calls.push(literal(next, token.line));
    } else if (next?.kind === "punct" && next.value === "(") {
      const argument = tokens[i + 2];
      const close = tokens[i + 3];
      if (argument?.kind === "string" && close?.kind === "punct" && close.value === ")") {
        calls.push(literal(argument, token.line));
      } else {
        calls.push({ line: token.line });
      }
    }
  }
  return calls;
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `deno task test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli
git commit -m "feat: Lua lexer that finds literal require calls

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 9: Require graph

**Files:**
- Create: `cli/src/bundle/graph.ts`
- Test: `cli/tests/unit/graph.test.ts`

**Interfaces:**
- Consumes: `findRequires` (Task 8); `CompiledModule` (Task 7, type only).
- Produces: `resolveGraph(entry: string, load: (name: string) => CompiledModule | undefined, builtins: ReadonlySet<string>): CompiledModule[]`. It returns only reachable modules, dependencies first, and each module once.

- [ ] **Step 1: Write the failing tests**

`cli/tests/unit/graph.test.ts`:
```ts
import { assertEquals, assertThrows } from "@std/assert";
import { resolveGraph } from "../../src/bundle/graph.ts";
import type { CompiledModule } from "../../src/yue/compile.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

function modules(sources: Record<string, string>) {
  return (name: string): CompiledModule | undefined =>
    name in sources ? { name, sourcePath: `src/${name.split(".").join("/")}.yue`, source: sources[name] } : undefined;
}
const BUILTINS = new Set(["moonwell"]);

Deno.test("resolveGraph returns reachable modules dependencies-first", () => {
  const load = modules({
    main: 'local mw = require("moonwell")\nlocal a = require("a")\nlocal b = require("b")',
    a: 'local c = require("c")',
    b: 'local c = require("c")',
    c: "return {}",
    unused: "return {}",
  });
  assertEquals(resolveGraph("main", load, BUILTINS).map((module) => module.name), ["c", "a", "b", "main"]);
});

Deno.test("resolveGraph reports a missing module where it is required", () => {
  const error = assertThrows(
    () => resolveGraph("main", modules({ main: '\n\nrequire("nope")' }), BUILTINS),
    MoonwellError,
    "Module 'nope' not found",
  );
  assertEquals(error.file, "src/main.yue");
  assertEquals(error.line, 3);
});

Deno.test("resolveGraph reports a missing entry", () => {
  assertThrows(() => resolveGraph("main", modules({}), BUILTINS), MoonwellError, "Module 'main' not found");
});

Deno.test("resolveGraph rejects dynamic requires", () => {
  const error = assertThrows(
    () => resolveGraph("main", modules({ main: "require(name)" }), BUILTINS),
    MoonwellError,
    "string literal",
  );
  assertEquals(error.line, 1);
});

Deno.test("resolveGraph reports cycles with the chain", () => {
  const load = modules({ main: 'require("a")', a: 'require("b")', b: 'require("a")' });
  assertThrows(() => resolveGraph("main", load, BUILTINS), MoonwellError, "a → b → a");
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test`
Expected: FAIL. `graph.ts` is not found.

- [ ] **Step 3: Implement**

`cli/src/bundle/graph.ts`:
```ts
import { MoonwellError } from "../shared/errors.ts";
import type { CompiledModule } from "../yue/compile.ts";
import { findRequires } from "./lexer.ts";

/** Walks literal requires from `entry`; returns reachable modules with dependencies before dependents. */
export function resolveGraph(
  entry: string,
  load: (name: string) => CompiledModule | undefined,
  builtins: ReadonlySet<string>,
): CompiledModule[] {
  const ordered: CompiledModule[] = [];
  const state = new Map<string, "visiting" | "done">();
  const stack: string[] = [];

  const visit = (name: string, from?: { module: CompiledModule; line: number }) => {
    if (builtins.has(name) || state.get(name) === "done") return;
    if (state.get(name) === "visiting") {
      const cycle = [...stack.slice(stack.indexOf(name)), name].join(" → ");
      throw new MoonwellError(`Circular require: ${cycle}`, {
        file: from?.module.sourcePath,
        line: from?.line,
        hint: "Move the shared code into a module that both can require.",
      });
    }
    const module = load(name);
    if (!module) {
      throw new MoonwellError(`Module '${name}' not found.`, {
        file: from?.module.sourcePath,
        line: from?.line,
        hint: `Expected src/${name.split(".").join("/")}.yue. Built-in modules: ${[...builtins].join(", ")}.`,
      });
    }
    state.set(name, "visiting");
    stack.push(name);
    for (const call of findRequires(module.source)) {
      if (call.name === undefined) {
        throw new MoonwellError("require must be called with a single string literal.", {
          file: module.sourcePath,
          line: call.line,
          hint: "Moonwell bundles modules at build time and cannot follow computed module names.",
        });
      }
      visit(call.name, { module, line: call.line });
    }
    stack.pop();
    state.set(name, "done");
    ordered.push(module);
  };

  visit(entry);
  return ordered;
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `deno task test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli
git commit -m "feat: require graph with missing, dynamic and cycle errors

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 10: Bundle emission and injection

**Files:**
- Create: `cli/src/bundle/emit.ts`
- Test: `cli/tests/unit/emit.test.ts`

**Interfaces:**
- Consumes: `CompiledModule` (Task 7); `MoonwellError`.
- Produces:
  - `emitBundle(input: { runtime: string; modules: CompiledModule[]; entry: string; firstLine: number }): string`. The result is a `do ... end` block. `firstLine` is the `war3map.lua` line on which the block's `do` sits.
  - `injectBundle(war3mapLua: string, bundle: (firstLine: number) => string, file?: string): string`

Bundle layout (the runtime in Task 11 relies on these names):
```lua
do
<runtime lines>                                   -- defines local __mw
__mw.define("name", function(...)
<module source lines>
end)
__mw.lines = {
{<start>, <stop>, "name", "src/name.yue"},
}
__mw.install()
__mw.boot("entry")
end
```

- [ ] **Step 1: Write the failing tests**

`cli/tests/unit/emit.test.ts`:
```ts
import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
import { emitBundle, injectBundle } from "../../src/bundle/emit.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

const modules = [
  { name: "util", sourcePath: "src/util.yue", source: "local M = {}\nreturn M\n" },
  { name: "main", sourcePath: "src/main.yue", source: 'local u = require("util")\nprint(u)\nreturn nil' },
];

Deno.test("emitBundle wraps modules and records absolute line ranges", () => {
  const bundle = emitBundle({ runtime: "local __mw = {}\n-- runtime", modules, entry: "main", firstLine: 10 });
  const lines = bundle.split("\n");
  assertEquals(lines[0], "do");
  // Line 10 = "do", 11-12 = runtime, 13 = define(util), 14-15 = util source.
  assertEquals(lines[13 - 10], '__mw.define("util", function(...)');
  assertEquals(lines[14 - 10], "local M = {}");
  assertStringIncludes(bundle, '{14, 15, "util", "src/util.yue"},');
  // 16 = end), 17 = define(main), 18-20 = main source.
  assertStringIncludes(bundle, '{18, 20, "main", "src/main.yue"},');
  assertEquals(lines[18 - 10], 'local u = require("util")');
  assertStringIncludes(bundle, '__mw.install()\n__mw.boot("main")\nend\n');
});

Deno.test("injectBundle appends after the map script and passes the first line", () => {
  const script = "function config()\nend\nfunction main()\nend";
  let seen = 0;
  const result = injectBundle(script, (first) => {
    seen = first;
    return "do\nend\n";
  });
  assertEquals(seen, 5);
  assertEquals(result.split("\n")[4], "do");
});

Deno.test("injectBundle handles CRLF scripts", () => {
  const script = "function config()\r\nend\r\nfunction main()\r\nend\r\n";
  let seen = 0;
  injectBundle(script, (first) => {
    seen = first;
    return "";
  });
  assertEquals(seen, 5);
});

Deno.test("injectBundle requires main and config", () => {
  const error = assertThrows(
    () => injectBundle("function main()\nend\n", () => "", "maps/map.w3x/war3map.lua"),
    MoonwellError,
    "config",
  );
  assertEquals(error.file, "maps/map.w3x/war3map.lua");
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test`
Expected: FAIL. `emit.ts` is not found.

- [ ] **Step 3: Implement**

`cli/src/bundle/emit.ts`:
```ts
import { MoonwellError } from "../shared/errors.ts";
import type { CompiledModule } from "../yue/compile.ts";

function sourceLines(text: string): string[] {
  const lines = text.replace(/\r\n/g, "\n").split("\n");
  if (lines.length > 1 && lines[lines.length - 1] === "") lines.pop();
  return lines;
}

/** Renders the bundle block; `firstLine` is the war3map.lua line number the leading "do" will occupy. */
export function emitBundle(
  input: { runtime: string; modules: CompiledModule[]; entry: string; firstLine: number },
): string {
  const out: string[] = ["do", ...sourceLines(input.runtime)];
  const ranges: string[] = [];
  const nextLine = () => input.firstLine + out.length;
  for (const module of input.modules) {
    out.push(`__mw.define(${JSON.stringify(module.name)}, function(...)`);
    const start = nextLine();
    const body = sourceLines(module.source);
    out.push(...body);
    ranges.push(
      `{${start}, ${start + body.length - 1}, ${JSON.stringify(module.name)}, ${JSON.stringify(module.sourcePath)}},`,
    );
    out.push("end)");
  }
  out.push("__mw.lines = {", ...ranges, "}", "__mw.install()", `__mw.boot(${JSON.stringify(input.entry)})`, "end");
  return out.join("\n") + "\n";
}

/** Appends the bundle to a World Editor war3map.lua that defines global main and config. */
export function injectBundle(war3mapLua: string, bundle: (firstLine: number) => string, file = "war3map.lua"): string {
  for (const name of ["main", "config"]) {
    if (!new RegExp(`^\\s*function\\s+${name}\\s*\\(`, "m").test(war3mapLua)) {
      throw new MoonwellError(`The map script does not define function ${name}().`, {
        file,
        hint: "Save the map in World Editor with Lua as the script language (Scenario › Map Options).",
      });
    }
  }
  const base = war3mapLua.endsWith("\n") ? war3mapLua : `${war3mapLua}\n`;
  const firstLine = base.split("\n").length;
  return base + bundle(firstLine);
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `deno task test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli
git commit -m "feat: emit module bundle and inject it into war3map.lua

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 11: Lua runtime and embedding

**Files:**
- Create: `cli/runtime/moonwell.lua`, `tools/gen.ts`, `cli/src/embedded/runtime.ts` (generated)
- Test: `cli/tests/yue/runtime.test.ts`, `cli/tests/unit/embedded.test.ts`

**Interfaces:**
- Consumes: `emitBundle`, `injectBundle` (Task 10); `writeTextIfChanged` (Task 2); `testYue` (Task 7); `runProcess`.
- Produces:
  - `RUNTIME_LUA: string` from `cli/src/embedded/runtime.ts`
  - `REPO: string` and `renderEmbedded(repo?: string): Promise<Map<string, string>>` from `tools/gen.ts`. The map goes from a repo-relative POSIX path to file content, and Task 15 extends it.
  - The Lua module `moonwell`, with API `before_config(fn)`, `on_config(fn)`, `before_main(fn)`, `on_main(fn)`, `format_error(message)`.

Note: `yue -e war3map.lua` runs the file as plain Lua 5.4 with the chunk name `[string "war3map.lua"]` (verified). The real game may report `war3map.lua:<n>:` instead, so `format_error` rewrites both forms.

- [ ] **Step 1: Write the failing runtime tests**

`cli/tests/yue/runtime.test.ts`:
```ts
import { assertEquals, assertStringIncludes } from "@std/assert";
import { join } from "@std/path";
import { emitBundle, injectBundle } from "../../src/bundle/emit.ts";
import { RUNTIME_LUA } from "../../src/embedded/runtime.ts";
import { runProcess } from "../../src/shared/process.ts";
import { testYue } from "../support/yue.ts";

const FAKE_MAP = [
  "LOG = {}",
  "PRINTED = {}",
  "print = function(...) local parts = {} for i = 1, select('#', ...) do parts[#parts + 1] = tostring((select(i, ...))) end PRINTED[#PRINTED + 1] = table.concat(parts, ' ') end",
  "function log(text) LOG[#LOG + 1] = text end",
  "function config() log('config') end",
  "function main() log('main') end",
  "",
].join("\n");

const REPORT = "\nconfig()\nmain()\nio.write(table.concat(LOG, '|'), '\\n', table.concat(PRINTED, '\\n'), '\\n')\n";

async function runMap(modules: Array<{ name: string; source: string }>): Promise<{ log: string; printed: string }> {
  const compiled = modules.map((module) => ({ ...module, sourcePath: `src/${module.name.split(".").join("/")}.yue` }));
  const script = injectBundle(
    FAKE_MAP,
    (firstLine) => emitBundle({ runtime: RUNTIME_LUA, modules: compiled, entry: "main", firstLine }),
  ) + REPORT;
  const dir = await Deno.makeTempDir();
  await Deno.writeTextFile(join(dir, "war3map.lua"), script);
  const result = await runProcess(await testYue(), ["-e", "war3map.lua"], { cwd: dir });
  assertEquals(result.code, 0, `${result.stdout}\n${result.stderr}`);
  const [log, ...printed] = result.stdout.split("\n");
  return { log, printed: printed.join("\n") };
}

Deno.test("hooks run around config and main in order; failures are isolated and source-mapped", async () => {
  const { log, printed } = await runMap([
    { name: "util.helper", source: "return { value = 42 }" },
    {
      name: "main",
      source: [
        'local mw = require("moonwell")',
        'local helper = require("util.helper")',
        'mw.before_config(function() log("before_config") end)',
        'mw.on_config(function() log("on_config") end)',
        'mw.before_main(function() log("before_main") end)',
        'mw.on_main(function() log("on_main " .. helper.value) end)',
        'mw.on_main(function() error("hook failed") end)',
        'mw.on_main(function() log("after failure") end)',
        "return {}",
      ].join("\n"),
    },
  ]);
  assertEquals(log, "before_config|config|on_config|before_main|main|on_main 42|after failure");
  assertStringIncludes(printed, "[moonwell] on_main failed");
  assertStringIncludes(printed, "src/main.yue:7: hook failed");
});

Deno.test("an entry that fails to load is reported and the map still runs", async () => {
  const { log, printed } = await runMap([{ name: "main", source: 'error("boot failed")' }]);
  assertEquals(log, "config|main");
  assertStringIncludes(printed, "[moonwell] load main failed");
  assertStringIncludes(printed, "src/main.yue:1: boot failed");
});

Deno.test("format_error leaves positions outside modules untouched", async () => {
  const { log } = await runMap([{
    name: "main",
    source: 'local mw = require("moonwell")\nlog(mw.format_error("war3map.lua:1: x"))\nreturn {}',
  }]);
  assertEquals(log, "war3map.lua:1: x|config|main");
});

Deno.test("hook registration rejects non-functions", async () => {
  const { printed } = await runMap([{ name: "main", source: 'require("moonwell").on_main(42)' }]);
  assertStringIncludes(printed, "moonwell.on_main expects a function");
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test:runtime`
Expected: FAIL. `../../src/embedded/runtime.ts` is not found.

- [ ] **Step 3: Write the runtime**

`cli/runtime/moonwell.lua`:
```lua
-- Moonwell runtime prelude. Emitted at the top of the bundle's `do ... end` block.
-- Lua 5.3 compatible. Everything here is local to the bundle except the globals
-- `require`, `config` and `main`, which it replaces.
local __mw = {
  modules = {},
  loaded = {},
  lines = {},
  hooks = { before_config = {}, on_config = {}, before_main = {}, on_main = {} },
}

function __mw.define(name, loader)
  __mw.modules[name] = loader
end

function __mw.require(name)
  local cached = __mw.loaded[name]
  if cached ~= nil then
    return cached
  end
  local loader = __mw.modules[name]
  if loader == nil then
    error("module '" .. tostring(name) .. "' is not in the bundle", 2)
  end
  local result = loader(name)
  if result == nil then
    result = true
  end
  __mw.loaded[name] = result
  return result
end

-- Maps an absolute war3map.lua line to "src/file.yue:<line>", or nil outside modules.
local function map_line(line)
  local lines = __mw.lines
  for i = #lines, 1, -1 do
    local entry = lines[i]
    if line >= entry[1] then
      if line <= entry[2] then
        return entry[4] .. ":" .. (line - entry[1] + 1)
      end
      return nil
    end
  end
  return nil
end

local function remap(line)
  local mapped = map_line(tonumber(line))
  if mapped ~= nil then
    return mapped .. ":"
  end
  return nil
end

function __mw.format_error(message)
  local text = tostring(message)
  text = text:gsub('%[string "war3map%.lua"%]:(%d+):', remap)
  text = text:gsub("war3map%.lua:(%d+):", remap)
  return text
end

local function handler(err)
  local text = tostring(err)
  if debug ~= nil and debug.traceback ~= nil then
    text = debug.traceback(text, 2)
  end
  return __mw.format_error(text)
end

function __mw.report(label, err)
  print("|cffff4040[moonwell] " .. label .. " failed:|r " .. tostring(err))
end

local function protect(label, fn, ...)
  local ok, err = xpcall(fn, handler, ...)
  if not ok then
    __mw.report(label, err)
  end
  return ok
end

local function run_hooks(phase)
  local list = __mw.hooks[phase]
  for i = 1, #list do
    protect(phase, list[i])
  end
end

function __mw.install()
  local original_config, original_main = config, main
  config = function()
    run_hooks("before_config")
    if original_config ~= nil then
      protect("config", original_config)
    end
    run_hooks("on_config")
  end
  main = function()
    run_hooks("before_main")
    if original_main ~= nil then
      protect("main", original_main)
    end
    run_hooks("on_main")
  end
end

function __mw.boot(entry)
  protect("load " .. entry, __mw.require, entry)
end

local function register(phase)
  return function(fn)
    if type(fn) ~= "function" then
      error("moonwell." .. phase .. " expects a function", 2)
    end
    local list = __mw.hooks[phase]
    list[#list + 1] = fn
  end
end

__mw.loaded["moonwell"] = {
  before_config = register("before_config"),
  on_config = register("on_config"),
  before_main = register("before_main"),
  on_main = register("on_main"),
  format_error = __mw.format_error,
}

require = __mw.require
```

- [ ] **Step 4: Write the generator and produce the embedded module**

`tools/gen.ts`:
```ts
/** Renders cli/src/embedded/*: JSR modules cannot read their own non-TS files when run remotely. */
import { dirname, fromFileUrl, join } from "@std/path";
import { writeTextIfChanged } from "../cli/src/shared/fs.ts";

export const REPO = join(dirname(fromFileUrl(import.meta.url)), "..");
const HEADER = "// GENERATED by `deno task gen` — do not edit.\n";

export async function renderEmbedded(repo: string = REPO): Promise<Map<string, string>> {
  const out = new Map<string, string>();
  const runtime = await Deno.readTextFile(join(repo, "cli", "runtime", "moonwell.lua"));
  out.set("cli/src/embedded/runtime.ts", `${HEADER}export const RUNTIME_LUA: string = ${JSON.stringify(runtime)};\n`);
  return out;
}

if (import.meta.main) {
  for (const [path, text] of await renderEmbedded()) {
    if (await writeTextIfChanged(join(REPO, ...path.split("/")), text)) console.log(`wrote ${path}`);
  }
}
```

Run: `deno task gen`
Expected: `wrote cli/src/embedded/runtime.ts`

`cli/tests/unit/embedded.test.ts`:
```ts
import { assertEquals } from "@std/assert";
import { join } from "@std/path";
import { renderEmbedded, REPO } from "../../../tools/gen.ts";

Deno.test("embedded modules are up to date (run `deno task gen`)", async () => {
  for (const [path, expected] of await renderEmbedded()) {
    assertEquals(await Deno.readTextFile(join(REPO, ...path.split("/"))), expected, path);
  }
});
```

- [ ] **Step 5: Run all tests to verify they pass**

Run: `deno task test && deno task test:runtime`
Expected: PASS. The runtime tests show hook order, error isolation and `src/main.yue:7:` mapping under the Lua VM embedded in yue.

- [ ] **Step 6: Commit**

```bash
git add cli tools
git commit -m "feat: Lua runtime with hooks, require shim and source-mapped errors

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 12: MPQ crypto and archive writer

**Files:**
- Create: `cli/src/mpq/crypto.ts`, `cli/src/mpq/writer.ts`, `cli/tests/support/mpq-reader.ts`
- Test: `cli/tests/unit/mpq.test.ts`

**Interfaces:**
- Consumes: `deflate`, `inflate` (Task 2); `MoonwellError`.
- Produces:
  - `HashType` (`TableOffset = 0`, `NameA = 1`, `NameB = 2`, `FileKey = 3`)
  - `hashString(text: string, type: HashType): number`
  - `HASH_TABLE_KEY`, `BLOCK_TABLE_KEY` (numbers)
  - `encryptBlock(data: Uint32Array, key: number): void`
  - `decryptBlock(data: Uint32Array, key: number): void`
  - `interface MpqFile { name: string; data: Uint8Array }`. Names use backslashes.
  - `writeMpq(files: MpqFile[], options?: { prefix?: Uint8Array; sectorSizeShift?: number }): Promise<Uint8Array>`
  - test helper `openMpq(bytes: Uint8Array): { headerOffset: number; read(name: string): Promise<Uint8Array | undefined>; listfile(): Promise<string[]> }`

The archive format is MPQ v1: a 32-byte header, then file bodies, the encrypted hash table and the encrypted block table.
- The sector size is `512 << 3` (4096).
- Each sector is zlib-deflated with compression mask byte `0x02`, or stored raw when that isn't smaller.
- A `(listfile)` is always written.

- [ ] **Step 1: Write the test reader and the failing tests**

`cli/tests/support/mpq-reader.ts`:
```ts
import { BLOCK_TABLE_KEY, decryptBlock, HASH_TABLE_KEY, hashString, HashType } from "../../src/mpq/crypto.ts";
import { inflate } from "../../src/shared/compression.ts";

/** Test-only MPQ v1 reader: enough to verify what writeMpq produces. */
export function openMpq(bytes: Uint8Array) {
  const at = (offset: number, length: number) => new DataView(bytes.buffer, bytes.byteOffset + offset, length);
  let base = -1;
  for (let offset = 0; offset + 32 <= bytes.length; offset += 512) {
    if (at(offset, 4).getUint32(0, true) === 0x1a51504d) {
      base = offset;
      break;
    }
  }
  if (base < 0) throw new Error("no MPQ header found");
  const header = at(base, 32);
  const sectorSize = 512 << header.getUint16(14, true);
  const hashSize = header.getUint32(24, true);
  const blockCount = header.getUint32(28, true);
  const table = (position: number, count: number, key: number) => {
    const view = at(base + position, count * 4);
    const words = new Uint32Array(count);
    for (let i = 0; i < count; i++) words[i] = view.getUint32(i * 4, true);
    decryptBlock(words, key);
    return words;
  };
  const hashes = table(header.getUint32(16, true), hashSize * 4, HASH_TABLE_KEY);
  const blocks = table(header.getUint32(20, true), blockCount * 4, BLOCK_TABLE_KEY);

  const find = (name: string): number | undefined => {
    const a = hashString(name, HashType.NameA);
    const b = hashString(name, HashType.NameB);
    let slot = hashString(name, HashType.TableOffset) & (hashSize - 1);
    for (let probes = 0; probes < hashSize; probes++) {
      const index = hashes[slot * 4 + 3];
      if (index === 0xffffffff) return undefined;
      if (hashes[slot * 4] === a && hashes[slot * 4 + 1] === b) return index;
      slot = (slot + 1) & (hashSize - 1);
    }
    return undefined;
  };

  const read = async (name: string): Promise<Uint8Array | undefined> => {
    const index = find(name);
    if (index === undefined) return undefined;
    const start = base + blocks[index * 4];
    const size = blocks[index * 4 + 2];
    const flags = blocks[index * 4 + 3];
    if (size === 0) return new Uint8Array(0);
    if ((flags & 0x200) === 0) return bytes.slice(start, start + size);
    const count = Math.ceil(size / sectorSize);
    const offsets = at(start, (count + 1) * 4);
    const out = new Uint8Array(size);
    for (let i = 0; i < count; i++) {
      const chunk = bytes.subarray(start + offsets.getUint32(i * 4, true), start + offsets.getUint32((i + 1) * 4, true));
      const expected = Math.min(sectorSize, size - i * sectorSize);
      if (chunk.length < expected) {
        if (chunk[0] !== 0x02) throw new Error(`unsupported compression mask ${chunk[0]}`);
        out.set(await inflate(chunk.subarray(1)), i * sectorSize);
      } else {
        out.set(chunk, i * sectorSize);
      }
    }
    return out;
  };

  return {
    headerOffset: base,
    read,
    listfile: async () => new TextDecoder().decode(await read("(listfile)")).split("\r\n").filter(Boolean),
  };
}
```

`cli/tests/unit/mpq.test.ts`:
```ts
import { assertEquals, assertNotEquals, assertRejects } from "@std/assert";
import { BLOCK_TABLE_KEY, decryptBlock, encryptBlock, HASH_TABLE_KEY, hashString, HashType } from "../../src/mpq/crypto.ts";
import { writeMpq } from "../../src/mpq/writer.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { openMpq } from "../support/mpq-reader.ts";

Deno.test("hashString matches the well-known table keys", () => {
  assertEquals(HASH_TABLE_KEY, 0xc3af3770);
  assertEquals(BLOCK_TABLE_KEY, 0xec83b3a3);
  assertEquals(hashString("war3map.lua", HashType.NameA), hashString("WAR3MAP.LUA", HashType.NameA));
});

Deno.test("encryptBlock and decryptBlock round trip", () => {
  const words = new Uint32Array([1, 2, 3, 0xffffffff]);
  encryptBlock(words, HASH_TABLE_KEY);
  assertNotEquals([...words], [1, 2, 3, 0xffffffff]);
  decryptBlock(words, HASH_TABLE_KEY);
  assertEquals([...words], [1, 2, 3, 0xffffffff]);
});

function noise(length: number): Uint8Array {
  const out = new Uint8Array(length);
  let x = 1;
  for (let i = 0; i < length; i++) {
    x = (Math.imul(x, 1103515245) + 12345) >>> 0;
    out[i] = x >>> 24;
  }
  return out;
}

Deno.test("writeMpq round trips compressible, incompressible and empty files", async () => {
  const lua = new TextEncoder().encode("print('moonwell')\n".repeat(600));
  const files = [
    { name: "war3map.lua", data: lua },
    { name: "war3mapImported\\noise.bin", data: noise(10000) },
    { name: "empty.txt", data: new Uint8Array() },
  ];
  const archive = openMpq(await writeMpq(files));
  assertEquals(archive.headerOffset, 0);
  assertEquals(await archive.read("war3map.lua"), lua);
  assertEquals(await archive.read("WAR3MAP.LUA"), lua);
  assertEquals(await archive.read("war3mapImported\\noise.bin"), noise(10000));
  assertEquals(await archive.read("empty.txt"), new Uint8Array());
  assertEquals(await archive.read("missing.txt"), undefined);
  assertEquals(await archive.listfile(), ["war3map.lua", "war3mapImported\\noise.bin", "empty.txt"]);
});

Deno.test("writeMpq places the MPQ header after a 512-byte prefix", async () => {
  const prefix = new Uint8Array(512);
  prefix.set([0x48, 0x4d, 0x33, 0x57]);
  const bytes = await writeMpq([{ name: "a.txt", data: new TextEncoder().encode("a") }], { prefix });
  assertEquals(bytes.subarray(0, 4), new Uint8Array([0x48, 0x4d, 0x33, 0x57]));
  const archive = openMpq(bytes);
  assertEquals(archive.headerOffset, 512);
  assertEquals(await archive.read("a.txt"), new TextEncoder().encode("a"));
});

Deno.test("writeMpq rejects case-insensitive duplicates and unaligned prefixes", async () => {
  const data = new Uint8Array([1]);
  await assertRejects(
    () => writeMpq([{ name: "A.txt", data }, { name: "a.TXT", data }]),
    MoonwellError,
    "Duplicate archive path",
  );
  await assertRejects(() => writeMpq([], { prefix: new Uint8Array(100) }), MoonwellError, "512");
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test`
Expected: FAIL. The MPQ modules are not found.

- [ ] **Step 3: Implement crypto**

`cli/src/mpq/crypto.ts`:
```ts
export const HashType = { TableOffset: 0, NameA: 1, NameB: 2, FileKey: 3 } as const;
export type HashType = typeof HashType[keyof typeof HashType];

const CRYPT_TABLE = (() => {
  const table = new Uint32Array(0x500);
  let seed = 0x00100001;
  for (let index1 = 0; index1 < 0x100; index1++) {
    for (let i = 0, index2 = index1; i < 5; i++, index2 += 0x100) {
      seed = (seed * 125 + 3) % 0x2aaaab;
      const high = (seed & 0xffff) << 0x10;
      seed = (seed * 125 + 3) % 0x2aaaab;
      table[index2] = (high | (seed & 0xffff)) >>> 0;
    }
  }
  return table;
})();

/** The MPQ string hash; names are upper-cased (ASCII only) as the game does. */
export function hashString(text: string, type: HashType): number {
  let seed1 = 0x7fed7fed;
  let seed2 = 0xeeeeeeee;
  for (const byte of new TextEncoder().encode(text)) {
    const char = byte >= 0x61 && byte <= 0x7a ? byte - 0x20 : byte;
    seed1 = (CRYPT_TABLE[(type << 8) + char] ^ ((seed1 + seed2) >>> 0)) >>> 0;
    seed2 = (char + seed1 + seed2 + ((seed2 << 5) >>> 0) + 3) >>> 0;
  }
  return seed1;
}

export const HASH_TABLE_KEY = hashString("(hash table)", HashType.FileKey);
export const BLOCK_TABLE_KEY = hashString("(block table)", HashType.FileKey);

function nextKey(key: number): number {
  return ((((~key << 0x15) >>> 0) + 0x11111111) >>> 0 | (key >>> 0x0b)) >>> 0;
}

export function encryptBlock(data: Uint32Array, key: number): void {
  let seed = 0xeeeeeeee;
  for (let i = 0; i < data.length; i++) {
    seed = (seed + CRYPT_TABLE[0x400 + (key & 0xff)]) >>> 0;
    const plain = data[i];
    data[i] = (plain ^ ((key + seed) >>> 0)) >>> 0;
    key = nextKey(key);
    seed = (plain + seed + ((seed << 5) >>> 0) + 3) >>> 0;
  }
}

export function decryptBlock(data: Uint32Array, key: number): void {
  let seed = 0xeeeeeeee;
  for (let i = 0; i < data.length; i++) {
    seed = (seed + CRYPT_TABLE[0x400 + (key & 0xff)]) >>> 0;
    const plain = (data[i] ^ ((key + seed) >>> 0)) >>> 0;
    data[i] = plain;
    key = nextKey(key);
    seed = (plain + seed + ((seed << 5) >>> 0) + 3) >>> 0;
  }
}
```

- [ ] **Step 4: Implement the writer**

`cli/src/mpq/writer.ts`:
```ts
import { deflate } from "../shared/compression.ts";
import { MoonwellError } from "../shared/errors.ts";
import { BLOCK_TABLE_KEY, encryptBlock, HASH_TABLE_KEY, hashString, HashType } from "./crypto.ts";

export interface MpqFile {
  /** Archive path with backslash separators, e.g. "war3mapImported\\icon.blp". */
  name: string;
  data: Uint8Array;
}

const HEADER_SIZE = 32;
const MPQ_MAGIC = 0x1a51504d; // "MPQ\x1A"
const FILE_EXISTS = 0x80000000;
const FILE_COMPRESS = 0x00000200;
const EMPTY = 0xffffffff;

/** Writes an MPQ format-1 archive, optionally after a 512-byte-aligned prefix (the HM3W map header). */
export async function writeMpq(
  files: MpqFile[],
  options: { prefix?: Uint8Array; sectorSizeShift?: number } = {},
): Promise<Uint8Array> {
  const prefix = options.prefix ?? new Uint8Array(0);
  if (prefix.length % 512 !== 0) throw new MoonwellError("The archive prefix must be a multiple of 512 bytes.");
  const shift = options.sectorSizeShift ?? 3;
  const sectorSize = 512 << shift;

  const seen = new Map<string, string>();
  for (const file of files) {
    const key = file.name.toUpperCase();
    const previous = seen.get(key);
    if (previous !== undefined) {
      throw new MoonwellError(`Duplicate archive path '${file.name}' (also '${previous}').`, {
        hint: "Archive paths are case-insensitive; rename one of the files.",
      });
    }
    seen.set(key, file.name);
  }
  const content = files.filter((file) => file.name.toUpperCase() !== "(LISTFILE)");
  const listfile = new TextEncoder().encode(content.map((file) => `${file.name}\r\n`).join(""));
  const entries: MpqFile[] = [...content, { name: "(listfile)", data: listfile }];

  const bodies: Uint8Array[] = [];
  const blocks = new Uint32Array(entries.length * 4);
  let offset = HEADER_SIZE;
  for (let i = 0; i < entries.length; i++) {
    const body = await encodeFile(entries[i].data, sectorSize);
    blocks[i * 4] = offset;
    blocks[i * 4 + 1] = body.length;
    blocks[i * 4 + 2] = entries[i].data.length;
    blocks[i * 4 + 3] = entries[i].data.length > 0 ? (FILE_EXISTS | FILE_COMPRESS) >>> 0 : FILE_EXISTS;
    bodies.push(body);
    offset += body.length;
  }

  let hashSize = 16;
  while (hashSize < entries.length * 1.5) hashSize *= 2;
  const hashes = new Uint32Array(hashSize * 4).fill(EMPTY);
  entries.forEach((entry, index) => {
    let slot = hashString(entry.name, HashType.TableOffset) & (hashSize - 1);
    while (hashes[slot * 4 + 3] !== EMPTY) slot = (slot + 1) & (hashSize - 1);
    hashes[slot * 4] = hashString(entry.name, HashType.NameA);
    hashes[slot * 4 + 1] = hashString(entry.name, HashType.NameB);
    hashes[slot * 4 + 2] = 0; // locale 0 (neutral), platform 0
    hashes[slot * 4 + 3] = index;
  });
  encryptBlock(hashes, HASH_TABLE_KEY);
  encryptBlock(blocks, BLOCK_TABLE_KEY);

  const hashPosition = offset;
  const blockPosition = hashPosition + hashSize * 16;
  const archiveSize = blockPosition + entries.length * 16;
  const out = new Uint8Array(prefix.length + archiveSize);
  out.set(prefix, 0);
  const header = new DataView(out.buffer, prefix.length, HEADER_SIZE);
  header.setUint32(0, MPQ_MAGIC, true);
  header.setUint32(4, HEADER_SIZE, true);
  header.setUint32(8, archiveSize, true);
  header.setUint16(12, 0, true); // format version 1
  header.setUint16(14, shift, true);
  header.setUint32(16, hashPosition, true);
  header.setUint32(20, blockPosition, true);
  header.setUint32(24, hashSize, true);
  header.setUint32(28, entries.length, true);
  let at = prefix.length + HEADER_SIZE;
  for (const body of bodies) {
    out.set(body, at);
    at += body.length;
  }
  writeWords(out, prefix.length + hashPosition, hashes);
  writeWords(out, prefix.length + blockPosition, blocks);
  return out;
}

function writeWords(target: Uint8Array, at: number, words: Uint32Array): void {
  const view = new DataView(target.buffer, target.byteOffset + at, words.length * 4);
  words.forEach((word, i) => view.setUint32(i * 4, word, true));
}

/** Sector offset table followed by sectors; each sector is zlib (mask 0x02) or raw when compression does not help. */
async function encodeFile(data: Uint8Array, sectorSize: number): Promise<Uint8Array> {
  if (data.length === 0) return new Uint8Array(0);
  const count = Math.ceil(data.length / sectorSize);
  const sectors: Uint8Array[] = [];
  for (let i = 0; i < count; i++) {
    const raw = data.subarray(i * sectorSize, Math.min(data.length, (i + 1) * sectorSize));
    const packed = await deflate(raw);
    if (packed.length + 1 < raw.length) {
      const sector = new Uint8Array(packed.length + 1);
      sector[0] = 0x02;
      sector.set(packed, 1);
      sectors.push(sector);
    } else {
      sectors.push(raw);
    }
  }
  const tableSize = (count + 1) * 4;
  const out = new Uint8Array(tableSize + sectors.reduce((sum, sector) => sum + sector.length, 0));
  const table = new DataView(out.buffer);
  let position = tableSize;
  sectors.forEach((sector, i) => {
    table.setUint32(i * 4, position, true);
    out.set(sector, position);
    position += sector.length;
  });
  table.setUint32(count * 4, position, true);
  return out;
}
```

- [ ] **Step 5: Run to verify they pass**

Run: `deno task test`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cli
git commit -m "feat: MPQ v1 archive writer with compressed sectors

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 13: Map packing (w3i header and HM3W)

**Files:**
- Create: `cli/src/map/w3i.ts`, `cli/src/mpq/hm3w.ts`, `cli/src/map/pack.ts`
- Test: `cli/tests/unit/pack.test.ts`

**Interfaces:**
- Consumes: `writeMpq` (Task 12); `listFiles` (Task 2); `openMpq` (test support).
- Produces:
  - `interface W3iHeader { version: number; gameVersion?: { major: number; minor: number } }`
  - `readW3iHeader(bytes: Uint8Array): W3iHeader`
  - `isHeaderlessArchive(header: W3iHeader): boolean`
  - `buildHm3wHeader(name: string, flags?: number, maxPlayers?: number): Uint8Array` (512 bytes)
  - `packMap(mapDir: string, mapName: string): Promise<Uint8Array>`

Interim note: Plan 2 adds a full `war3map.w3i` reader. It will pass the real flags and max-player count to `buildHm3wHeader`. Until then, pre-1.31 maps get flags `0` and max players `0`. This matches spec §11's open item on HM3W correctness.

- [ ] **Step 1: Write the failing tests**

`cli/tests/unit/pack.test.ts`:
```ts
import { assertEquals, assertRejects } from "@std/assert";
import { join } from "@std/path";
import { isHeaderlessArchive, readW3iHeader } from "../../src/map/w3i.ts";
import { buildHm3wHeader } from "../../src/mpq/hm3w.ts";
import { packMap } from "../../src/map/pack.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { openMpq } from "../support/mpq-reader.ts";

function w3i(version: number, major = 0, minor = 0): Uint8Array {
  const bytes = new Uint8Array(64);
  const view = new DataView(bytes.buffer);
  view.setInt32(0, version, true);
  view.setUint32(12, major, true);
  view.setUint32(16, minor, true);
  return bytes;
}

async function mapDir(info: Uint8Array): Promise<string> {
  const dir = await Deno.makeTempDir();
  await Deno.writeFile(join(dir, "war3map.w3i"), info);
  await Deno.writeTextFile(join(dir, "war3map.lua"), "function main() end");
  await Deno.mkdir(join(dir, "war3mapImported"));
  await Deno.writeTextFile(join(dir, "war3mapImported", "a.txt"), "asset");
  return dir;
}

Deno.test("readW3iHeader reads version and game version", () => {
  assertEquals(readW3iHeader(w3i(39, 3, 0)), { version: 39, gameVersion: { major: 3, minor: 0 } });
  assertEquals(readW3iHeader(w3i(25)), { version: 25 });
});

Deno.test("isHeaderlessArchive only for v39 maps from 1.31 on", () => {
  assertEquals(isHeaderlessArchive({ version: 39, gameVersion: { major: 1, minor: 31 } }), true);
  assertEquals(isHeaderlessArchive({ version: 39, gameVersion: { major: 1, minor: 30 } }), false);
  assertEquals(isHeaderlessArchive({ version: 31, gameVersion: { major: 1, minor: 31 } }), false);
  assertEquals(isHeaderlessArchive({ version: 25 }), false);
});

Deno.test("buildHm3wHeader writes magic, name, flags and players", () => {
  const header = buildHm3wHeader("Hero", 4, 6);
  assertEquals(header.length, 512);
  assertEquals(new TextDecoder().decode(header.subarray(0, 4)), "HM3W");
  assertEquals(new TextDecoder().decode(header.subarray(8, 12)), "Hero");
  const view = new DataView(header.buffer);
  assertEquals(header[12], 0);
  assertEquals(view.getUint32(13, true), 4);
  assertEquals(view.getUint32(17, true), 6);
});

Deno.test("packMap writes a headerless archive for modern maps with backslash paths", async () => {
  const archive = openMpq(await packMap(await mapDir(w3i(39, 3, 0)), "map"));
  assertEquals(archive.headerOffset, 0);
  assertEquals(new TextDecoder().decode(await archive.read("war3mapImported\\a.txt")), "asset");
  assertEquals(new TextDecoder().decode(await archive.read("war3map.lua")), "function main() end");
});

Deno.test("packMap prefixes HM3W for older maps", async () => {
  const bytes = await packMap(await mapDir(w3i(25)), "Old Map");
  assertEquals(new TextDecoder().decode(bytes.subarray(0, 4)), "HM3W");
  assertEquals(openMpq(bytes).headerOffset, 512);
});

Deno.test("packMap requires war3map.w3i", async () => {
  const dir = await Deno.makeTempDir();
  await assertRejects(() => packMap(dir, "map"), MoonwellError, "war3map.w3i");
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test`
Expected: FAIL. The modules are not found.

- [ ] **Step 3: Implement**

`cli/src/map/w3i.ts`:
```ts
import { MoonwellError } from "../shared/errors.ts";

export interface W3iHeader {
  version: number;
  gameVersion?: { major: number; minor: number };
}

/** Reads the format version and, for version 28+, the saving game's major/minor version. */
export function readW3iHeader(bytes: Uint8Array): W3iHeader {
  if (bytes.length < 4) throw new MoonwellError("war3map.w3i is truncated.");
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const version = view.getInt32(0, true);
  if (version >= 28 && bytes.length >= 20) {
    return { version, gameVersion: { major: view.getUint32(12, true), minor: view.getUint32(16, true) } };
  }
  return { version };
}

/** v39 maps saved by 1.31+ ship without the legacy HM3W header (the behaviour wc3-dev-framework verified in game). */
export function isHeaderlessArchive(header: W3iHeader): boolean {
  if (header.version !== 39 || header.gameVersion === undefined) return false;
  return header.gameVersion.major * 100 + header.gameVersion.minor >= 131;
}
```

`cli/src/mpq/hm3w.ts`:
```ts
/** The legacy 512-byte "HM3W" map header: magic, 4 unused bytes, name (NUL-terminated), flags, max players. */
export function buildHm3wHeader(name: string, flags = 0, maxPlayers = 0): Uint8Array {
  const out = new Uint8Array(512);
  const view = new DataView(out.buffer);
  out.set([0x48, 0x4d, 0x33, 0x57], 0);
  const encoded = new TextEncoder().encode(name).subarray(0, 512 - 8 - 1 - 8);
  out.set(encoded, 8);
  const at = 8 + encoded.length + 1;
  view.setUint32(at, flags >>> 0, true);
  view.setUint32(at + 4, maxPlayers >>> 0, true);
  return out;
}
```

`cli/src/map/pack.ts`:
```ts
import { join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { listFiles } from "../shared/fs.ts";
import { buildHm3wHeader } from "../mpq/hm3w.ts";
import { writeMpq } from "../mpq/writer.ts";
import { isHeaderlessArchive, readW3iHeader } from "./w3i.ts";

/** Packs a staged map folder into .w3x bytes. */
export async function packMap(mapDir: string, mapName: string): Promise<Uint8Array> {
  let info: Uint8Array;
  try {
    info = await Deno.readFile(join(mapDir, "war3map.w3i"));
  } catch (error) {
    if (!(error instanceof Deno.errors.NotFound)) throw error;
    throw new MoonwellError("war3map.w3i is missing from the map folder.", {
      file: mapDir,
      hint: "Save the source map from World Editor in folder format.",
    });
  }
  const files = await Promise.all(
    (await listFiles(mapDir)).map(async (file) => ({
      name: file.split("/").join("\\"),
      data: await Deno.readFile(join(mapDir, ...file.split("/"))),
    })),
  );
  const prefix = isHeaderlessArchive(readW3iHeader(info)) ? undefined : buildHm3wHeader(mapName);
  return writeMpq(files, { prefix });
}
```

- [ ] **Step 4: Run to verify they pass**

Run: `deno task test`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli
git commit -m "feat: pack staged map folders into w3x archives

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 14: Pipeline, commands and CLI entry

**Files:**
- Create: `cli/src/context.ts`, `cli/src/launch.ts`, `cli/src/pipeline.ts`
- Create: `cli/src/commands/build.ts`, `cli/src/commands/test.ts`, `cli/src/commands/check.ts`, `cli/src/commands/setup.ts`, `cli/src/main.ts`
- Test: `cli/tests/unit/pipeline.test.ts`, `cli/tests/unit/main.test.ts`

**Interfaces:**
- Consumes everything above:
  - `loadProject`, `Project` (Task 4)
  - `ensureYue`, `defaultInstallDeps`, `InstallDeps` (Task 6)
  - `compileSources`, `CompiledModule` (Task 7)
  - `resolveGraph` (Task 9)
  - `emitBundle`, `injectBundle` (Task 10)
  - `RUNTIME_LUA` (Task 11)
  - `packMap` (Task 13)
  - `withBuildLock`, `replaceDir`, `removeIfExists`, `toPosix`, `createLogger`, `runProcess` (Task 2)
- Produces:
  - `type Spawn = (command: string, args: string[]) => void`
  - `spawnDetached: Spawn`
  - `launchGame(launch: Project["launch"], mapPath: string, spawn?: Spawn): Promise<void>`
  - `interface CommandContext { root: string; logger: Logger; run: Runner; install: InstallDeps; spawn: Spawn }`
  - `createContext(root: string, logger: Logger): CommandContext`
  - `interface StageOptions { entry?: string; minify?: boolean }`
  - `BUILTIN_MODULES: ReadonlySet<string>`
  - `entryModuleName(entryPath: string): string`
  - `compileProject(ctx: CommandContext, project: Project, options: StageOptions): Promise<{ modules: CompiledModule[]; entry: string }>`
  - `prepareStage(ctx: CommandContext, project: Project, options: StageOptions): Promise<{ mapDir: string; modules: CompiledModule[] }>`. Plan 2 inserts the data layers between staging and injection here.
  - `build(ctx, options?): Promise<string>` (the archive path), `test(ctx, options?): Promise<void>`, `check(ctx): Promise<{ modules: number; entry: string }>`, `setup(ctx): Promise<string>`
  - `main(args: string[], root?: string, write?: (line: string) => void): Promise<number>`

- [ ] **Step 1: Write the failing unit tests**

`cli/tests/unit/pipeline.test.ts`:
```ts
import { assertEquals, assertRejects, assertThrows } from "@std/assert";
import { join } from "@std/path";
import { entryModuleName } from "../../src/pipeline.ts";
import { launchGame } from "../../src/launch.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

Deno.test("entryModuleName converts src paths to dotted names", () => {
  assertEquals(entryModuleName("src/main.yue"), "main");
  assertEquals(entryModuleName("./src/game/init.yue"), "game.init");
  assertEquals(entryModuleName("src\\testbed\\run.yue"), "testbed.run");
  assertThrows(() => entryModuleName("lib/main.yue"), MoonwellError, "under src/");
  assertThrows(() => entryModuleName("src/main.lua"), MoonwellError, "under src/");
});

Deno.test("launchGame explains a missing or wrong executable", async () => {
  const launch = { gameExecutable: null, args: [] };
  const missing = await assertRejects(() => launchGame(launch, "map"), MoonwellError, "gameExecutable is not set");
  assertEquals(missing.file, "moonwell.local.pkl");
  const missingExe = join(await Deno.makeTempDir(), "Warcraft III.exe");
  await assertRejects(
    () => launchGame({ gameExecutable: missingExe, args: [] }, "map"),
    MoonwellError,
    "not found",
  );
});

Deno.test("launchGame passes the launch args and -loadfile", async () => {
  const dir = await Deno.makeTempDir();
  const exe = join(dir, "Warcraft III.exe");
  await Deno.writeTextFile(exe, "");
  const calls: Array<[string, string[]]> = [];
  await launchGame({ gameExecutable: exe, args: ["-launch"] }, "C:/map.w3x", (command, args) => calls.push([command, args]));
  assertEquals(calls, [[exe, ["-launch", "-loadfile", "C:/map.w3x"]]]);
});
```

`cli/tests/unit/main.test.ts`:
```ts
import { assertEquals, assertStringIncludes } from "@std/assert";
import { main } from "../../src/main.ts";
import { VERSION } from "../../src/version.ts";

async function run(args: string[], root?: string) {
  const lines: string[] = [];
  const code = await main(args, root ?? await Deno.makeTempDir(), (line) => lines.push(line));
  return { code, output: lines.join("\n") };
}

Deno.test("--help and no command print usage", async () => {
  for (const args of [["--help"], []]) {
    const { code, output } = await run(args);
    assertEquals(code, 0);
    assertStringIncludes(output, "Usage: moonwell <command>");
  }
});

Deno.test("--version prints the version", async () => {
  assertEquals(await run(["--version"]), { code: 0, output: VERSION });
});

Deno.test("unknown commands fail with usage", async () => {
  const { code, output } = await run(["frobnicate"]);
  assertEquals(code, 1);
  assertStringIncludes(output, "Unknown command 'frobnicate'");
});

Deno.test("command failures are formatted and return 1", async () => {
  const { code, output } = await run(["check"]);
  assertEquals(code, 1);
  assertStringIncludes(output, "error:");
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test`
Expected: FAIL. The modules are not found.

- [ ] **Step 3: Implement launch and context**

`cli/src/launch.ts`:
```ts
import { exists } from "@std/fs";
import { MoonwellError } from "./shared/errors.ts";
import type { Project } from "./project/project.ts";

export type Spawn = (command: string, args: string[]) => void;

export const spawnDetached: Spawn = (command, args) => {
  const child = new Deno.Command(command, { args, stdin: "null", stdout: "null", stderr: "null" }).spawn();
  child.unref();
};

const LOCAL_EXAMPLE = [
  "Create moonwell.local.pkl next to moonwell.pkl:",
  '  amends "moonwell.pkl"',
  '  launch { gameExecutable = "C:\\\\Program Files (x86)\\\\Warcraft III\\\\_retail_\\\\x86_64\\\\Warcraft III.exe" }',
].join("\n");

/** Starts Warcraft III on `mapPath` (a staged folder map or a .w3x). */
export async function launchGame(launch: Project["launch"], mapPath: string, spawn: Spawn = spawnDetached): Promise<void> {
  const executable = launch.gameExecutable;
  if (executable === null) {
    throw new MoonwellError("launch.gameExecutable is not set.", { file: "moonwell.local.pkl", hint: LOCAL_EXAMPLE });
  }
  if (!(await exists(executable))) {
    throw new MoonwellError(`Game executable not found: ${executable}`, {
      file: "moonwell.local.pkl",
      hint: "Fix launch.gameExecutable to point at Warcraft III.exe.",
    });
  }
  spawn(executable, [...launch.args, "-loadfile", mapPath]);
}
```

`cli/src/context.ts`:
```ts
import type { Logger } from "./shared/log.ts";
import { type Runner, runProcess } from "./shared/process.ts";
import { defaultInstallDeps, type InstallDeps } from "./yue/install.ts";
import { type Spawn, spawnDetached } from "./launch.ts";

/** Everything a command needs from the outside world; tests substitute parts of it. */
export interface CommandContext {
  root: string;
  logger: Logger;
  run: Runner;
  install: InstallDeps;
  spawn: Spawn;
}

export function createContext(root: string, logger: Logger): CommandContext {
  return { root, logger, run: runProcess, install: defaultInstallDeps(logger), spawn: spawnDetached };
}
```

- [ ] **Step 4: Implement the pipeline**

`cli/src/pipeline.ts`:
```ts
import { exists } from "@std/fs";
import { join } from "@std/path";
import { emitBundle, injectBundle } from "./bundle/emit.ts";
import { resolveGraph } from "./bundle/graph.ts";
import type { CommandContext } from "./context.ts";
import { RUNTIME_LUA } from "./embedded/runtime.ts";
import type { Project } from "./project/project.ts";
import { MoonwellError } from "./shared/errors.ts";
import { replaceDir } from "./shared/fs.ts";
import { type CompiledModule, compileSources } from "./yue/compile.ts";
import { ensureYue } from "./yue/install.ts";

export interface StageOptions {
  /** Entry file overriding map.entry, relative to the project root. */
  entry?: string;
  /** Overrides build.minify when set. */
  minify?: boolean;
}

/** Modules provided by the runtime rather than by src/. */
export const BUILTIN_MODULES: ReadonlySet<string> = new Set(["moonwell"]);

export function entryModuleName(entryPath: string): string {
  const posix = entryPath.replaceAll("\\", "/").replace(/^\.\//, "");
  if (!posix.startsWith("src/") || !posix.endsWith(".yue")) {
    throw new MoonwellError(`Entry '${entryPath}' must be a .yue file under src/.`, { hint: "For example: src/main.yue" });
  }
  return posix.slice("src/".length, -".yue".length).split("/").join(".");
}

/** Compiles src/ and resolves the reachable module graph from the entry. */
export async function compileProject(
  ctx: CommandContext,
  project: Project,
  options: StageOptions,
): Promise<{ modules: CompiledModule[]; entry: string }> {
  const yue = await ensureYue(project.yue, ctx.install);
  const output = await compileSources({
    yue,
    root: ctx.root,
    minify: options.minify ?? project.build.minify,
    run: ctx.run,
  });
  const entry = entryModuleName(options.entry ?? project.map.entry);
  return { modules: resolveGraph(entry, output.load, BUILTIN_MODULES), entry };
}

/** Compiles gameplay, stages the source map into dist/stage/map and injects the bundle. */
export async function prepareStage(
  ctx: CommandContext,
  project: Project,
  options: StageOptions,
): Promise<{ mapDir: string; modules: CompiledModule[] }> {
  const { modules, entry } = await compileProject(ctx, project, options);
  const source = join(ctx.root, "maps", project.map.folder);
  if (!(await exists(source))) {
    throw new MoonwellError(`Source map folder maps/${project.map.folder} not found.`, {
      file: "moonwell.pkl",
      hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
    });
  }
  const mapDir = join(ctx.root, "dist", "stage", "map");
  await replaceDir(source, mapDir);

  const scriptPath = join(mapDir, "war3map.lua");
  const scriptLabel = `maps/${project.map.folder}/war3map.lua`;
  if (!(await exists(scriptPath))) {
    throw new MoonwellError("The map has no war3map.lua.", {
      file: scriptLabel,
      hint: "Save the map in World Editor with Lua as the script language.",
    });
  }
  const script = await Deno.readTextFile(scriptPath);
  const bundled = injectBundle(
    script,
    (firstLine) => emitBundle({ runtime: RUNTIME_LUA, modules, entry, firstLine }),
    scriptLabel,
  );
  await Deno.writeTextFile(scriptPath, bundled);
  return { mapDir, modules };
}
```

- [ ] **Step 5: Implement the commands**

`cli/src/commands/build.ts`:
```ts
import { basename, dirname, join, relative } from "@std/path";
import type { CommandContext } from "../context.ts";
import { packMap } from "../map/pack.ts";
import { prepareStage, type StageOptions } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";
import { removeIfExists, toPosix } from "../shared/fs.ts";
import { withBuildLock } from "../shared/lock.ts";

/** Builds <build.folder>/<map.folder>; a failed build leaves no archive behind. */
export function build(ctx: CommandContext, options: StageOptions = {}): Promise<string> {
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const project = await loadProject(ctx.root, ctx.run);
    const output = join(ctx.root, project.build.folder, project.map.folder);
    await removeIfExists(output);
    try {
      const stage = await prepareStage(ctx, project, options);
      ctx.logger.info("Packing archive...");
      const archive = await packMap(stage.mapDir, basename(project.map.folder, ".w3x"));
      await Deno.mkdir(dirname(output), { recursive: true });
      await Deno.writeFile(output, archive);
      ctx.logger.info(`Built ${toPosix(relative(ctx.root, output))} (${stage.modules.length} module(s)).`);
      return output;
    } catch (error) {
      await removeIfExists(output);
      throw error;
    }
  });
}
```

`cli/src/commands/test.ts`:
```ts
import { join, relative } from "@std/path";
import type { CommandContext } from "../context.ts";
import { launchGame } from "../launch.ts";
import { prepareStage, type StageOptions } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";
import { toPosix } from "../shared/fs.ts";
import { withBuildLock } from "../shared/lock.ts";

/** Stages the map as a folder and launches Warcraft III on it. */
export function test(ctx: CommandContext, options: StageOptions = {}): Promise<void> {
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const project = await loadProject(ctx.root, ctx.run);
    const stage = await prepareStage(ctx, project, options);
    await launchGame(project.launch, stage.mapDir, ctx.spawn);
    ctx.logger.info(`Launched Warcraft III with ${toPosix(relative(ctx.root, stage.mapDir))}.`);
  });
}
```

`cli/src/commands/check.ts`:
```ts
import { join } from "@std/path";
import type { CommandContext } from "../context.ts";
import { compileProject } from "../pipeline.ts";
import { loadProject } from "../project/project.ts";
import { withBuildLock } from "../shared/lock.ts";

/** Compiles every module and resolves the graph; no map is staged or packed. */
export function check(ctx: CommandContext): Promise<{ modules: number; entry: string }> {
  return withBuildLock(join(ctx.root, "dist"), async () => {
    const project = await loadProject(ctx.root, ctx.run);
    const { modules, entry } = await compileProject(ctx, project, {});
    ctx.logger.info(`Check passed: ${modules.length} module(s) reachable from ${entry}.`);
    return { modules: modules.length, entry };
  });
}
```

`cli/src/commands/setup.ts`:
```ts
import type { CommandContext } from "../context.ts";
import { loadProject } from "../project/project.ts";
import { ensureYue } from "../yue/install.ts";

/** Installs the project's pinned compiler into the user cache. */
export async function setup(ctx: CommandContext): Promise<string> {
  const project = await loadProject(ctx.root, ctx.run);
  const binary = await ensureYue(project.yue, ctx.install);
  ctx.logger.info(`YueScript ${project.yue.version}: ${binary}`);
  return binary;
}
```

`cli/src/main.ts`:
```ts
import { parseArgs } from "@std/cli/parse-args";
import { join } from "@std/path";
import { build } from "./commands/build.ts";
import { check } from "./commands/check.ts";
import { setup } from "./commands/setup.ts";
import { test } from "./commands/test.ts";
import { createContext } from "./context.ts";
import { formatError } from "./shared/errors.ts";
import { createLogger } from "./shared/log.ts";
import { VERSION } from "./version.ts";

const USAGE = `Moonwell ${VERSION}: Warcraft III maps with YueScript gameplay and Pkl data

Usage: moonwell <command> [options]

Commands:
  init <dir> [--link]            Create a project (--link: use this local Moonwell checkout)
  setup                          Install the pinned YueScript compiler
  build [--entry f] [--minify]   Build <build.folder>/<map.folder>
  test [--entry f]               Stage the map and launch Warcraft III
  dev                            Watch sources and report errors on save
  check                          Compile and validate without building a map

Options:
  -h, --help                     Show this help
  -v, --version                  Show the version`;

export async function main(
  args: string[],
  root: string = Deno.cwd(),
  write: (line: string) => void = (line) => console.error(line),
): Promise<number> {
  const flags = parseArgs(args, {
    string: ["entry"],
    boolean: ["minify", "link", "help", "version"],
    alias: { h: "help", v: "version" },
  });
  const [command] = flags._.map(String);
  if (flags.version) {
    write(VERSION);
    return 0;
  }
  if (flags.help || command === undefined) {
    write(USAGE);
    return 0;
  }
  const logger = createLogger({ write, file: command === "init" ? undefined : join(root, "dist", "moonwell.log") });
  const ctx = createContext(root, logger);
  const stage = { entry: flags.entry, minify: flags.minify ? true : undefined };
  try {
    switch (command) {
      case "setup":
        await setup(ctx);
        break;
      case "build":
        await build(ctx, stage);
        break;
      case "test":
        await test(ctx, stage);
        break;
      case "check":
        await check(ctx);
        break;
      default:
        write(`Unknown command '${command}'.\n\n${USAGE}`);
        return 1;
    }
    return 0;
  } catch (error) {
    logger.error(formatError(error));
    return 1;
  }
}

if (import.meta.main) Deno.exit(await main(Deno.args));
```

- [ ] **Step 6: Run to verify they pass**

Run: `deno task test && deno task check`
Expected: PASS, and `deno check` reports no type errors.

- [ ] **Step 7: Commit**

```bash
git add cli
git commit -m "feat: build, test, check and setup commands with CLI entry

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 15: Template project and `init`

**Files:**
- Create: `cli/src/project-files.ts`, `cli/src/commands/init.ts`, `cli/src/embedded/template.ts` (generated)
- Create: `template/deno.json`, `template/PklProject`, `template/PklProject.deps.json` (generated by pkl), `template/moonwell.pkl`, `template/.gitignore`, `template/src/main.yue`, `template/maps/map.w3x/**` (copied)
- Modify: `tools/gen.ts` (embed the template), `cli/src/main.ts` (add `init`), `deno.json` (add `./template` to the workspace)
- Test: `cli/tests/unit/project-files.test.ts`, `cli/tests/pkl/init.test.ts`

**Interfaces:**
- Consumes: `VERSION`; `Runner`; `MoonwellError`; `CommandContext`; `listFiles`; `renderEmbedded` (Task 11).
- Produces:
  - `PROJECT_TASKS: readonly string[]`
  - `projectDenoJson(cli: string): string`
  - `PACKAGE_BASE_URI: string`
  - `projectPklProject(dependency: { version: string } | { local: string }): string`
  - `TEMPLATE_EXCLUDE: readonly string[]`
  - `TEMPLATE_FILES: ReadonlyArray<{ path: string; base64: string }>`
  - `init(dir: string, ctx: CommandContext, options?: { link?: boolean }): Promise<string>`: returns the absolute project path

- [ ] **Step 1: Write the failing unit tests**

`cli/tests/unit/project-files.test.ts`:
```ts
import { assertEquals, assertStringIncludes } from "@std/assert";
import { fromFileUrl } from "@std/path";
import { projectDenoJson, projectPklProject } from "../../src/project-files.ts";

const repo = (path: string) => fromFileUrl(new URL(`../../../${path}`, import.meta.url));

Deno.test("projectDenoJson runs every task through the given CLI", () => {
  const json = JSON.parse(projectDenoJson("jsr:@moonwell/cli@0.1.0"));
  assertEquals(json.tasks.build, "deno run -A jsr:@moonwell/cli@0.1.0 build");
  assertEquals(Object.keys(json.tasks), ["build", "test", "dev", "check", "setup"]);
  assertEquals("nodeModulesDir" in json, false);
});

Deno.test("projectPklProject declares a remote or local moonwell dependency", () => {
  assertStringIncludes(
    projectPklProject({ version: "0.1.0" }),
    '["moonwell"] { uri = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0.1.0" }',
  );
  assertStringIncludes(projectPklProject({ local: "../pkl" }), '["moonwell"] = import("../pkl/PklProject")');
});

Deno.test("the committed template uses the local-link forms", async () => {
  assertEquals(await Deno.readTextFile(repo("template/deno.json")), projectDenoJson("../cli/src/main.ts"));
  assertEquals(await Deno.readTextFile(repo("template/PklProject")), projectPklProject({ local: "../pkl" }));
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `deno task test`
Expected: FAIL. `project-files.ts` is not found.

- [ ] **Step 3: Implement project files**

`cli/src/project-files.ts`:
```ts
/** Tasks every project's deno.json exposes. Plan 2 adds the data-layer commands. */
export const PROJECT_TASKS = ["build", "test", "dev", "check", "setup"] as const;

export const PACKAGE_BASE_URI = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell";

/** deno.json for a map project; `cli` is a JSR specifier or a path to cli/src/main.ts. */
export function projectDenoJson(cli: string): string {
  const tasks = Object.fromEntries(PROJECT_TASKS.map((task) => [task, `deno run -A ${cli} ${task}`]));
  return `${JSON.stringify({ tasks }, null, 2)}\n`;
}

/** PklProject for a map project, depending on a published or a local moonwell package. */
export function projectPklProject(dependency: { version: string } | { local: string }): string {
  const line = "local" in dependency
    ? `  ["moonwell"] = import("${dependency.local}/PklProject")`
    : `  ["moonwell"] { uri = "${PACKAGE_BASE_URI}@${dependency.version}" }`;
  return `amends "pkl:Project"\n\ndependencies {\n${line}\n}\n`;
}

/** Template files init generates itself instead of copying. */
export const TEMPLATE_EXCLUDE = ["deno.json", "PklProject", "PklProject.deps.json", "moonwell.local.pkl"] as const;
```

- [ ] **Step 4: Create the template**

Copy the source map from wc3-dev-framework. It's a Lua-mode, folder-format map saved by the current World Editor, with a v39 w3i:
```bash
mkdir -p template/maps
cp -r ../wc3-dev-framework/maps/map.w3x template/maps/map.w3x
```

`template/deno.json` (exactly `projectDenoJson("../cli/src/main.ts")`):
```json
{
  "tasks": {
    "build": "deno run -A ../cli/src/main.ts build",
    "test": "deno run -A ../cli/src/main.ts test",
    "dev": "deno run -A ../cli/src/main.ts dev",
    "check": "deno run -A ../cli/src/main.ts check",
    "setup": "deno run -A ../cli/src/main.ts setup"
  }
}
```

`template/PklProject` (exactly `projectPklProject({ local: "../pkl" })`):
```pkl
amends "pkl:Project"

dependencies {
  ["moonwell"] = import("../pkl/PklProject")
}
```

`template/.gitignore`:
```
dist/
moonwell.local.pkl
```

`template/moonwell.pkl`:
```pkl
// Moonwell project manifest. Schema and documentation: @moonwell/Project.pkl
// Uncomment and edit what you need; everything else uses the defaults.
//
// Machine-specific values belong in moonwell.local.pkl (git-ignored):
//
//   amends "moonwell.pkl"
//   launch { gameExecutable = "C:\\Program Files (x86)\\Warcraft III\\_retail_\\x86_64\\Warcraft III.exe" }
amends "@moonwell/Project.pkl"

// map {
//   folder = "map.w3x"        // World Editor map folder under maps/
//   entry = "src/main.yue"    // gameplay entry point
// }

// build {
//   folder = "dist/bin"
//   minify = true
// }

// launch {
//   args { "-launch"; "-windowmode"; "windowed" }
// }

// yue {
//   version = "0.34.2"
// }
```

`template/src/main.yue`:
```yue
import "moonwell" as mw

mw.on_main ->
  print "Moonwell is running."
  unit = CreateUnit Player(0), FourCC("hfoo"), 0, 0, 270
  TimerStart CreateTimer!, 1.0, true, ->
    SetUnitColor unit, ConvertPlayerColor GetRandomInt 0, 23
```

Add the template to the workspace. In the root `deno.json`, change `"workspace": ["./cli"]` to:
```json
  "workspace": ["./cli", "./template"],
```

Resolve the template's Pkl dependencies:
```bash
cd template && pkl project resolve && cd ..
```
Expected: `template/PklProject.deps.json` is created, with `"type": "local"` and `"path": "../pkl"`.

- [ ] **Step 5: Embed the template**

Update `tools/gen.ts` to this full content:
```ts
/** Renders cli/src/embedded/*: JSR modules cannot read their own non-TS files when run remotely. */
import { encodeBase64 } from "@std/encoding/base64";
import { dirname, fromFileUrl, join } from "@std/path";
import { listFiles, writeTextIfChanged } from "../cli/src/shared/fs.ts";
import { TEMPLATE_EXCLUDE } from "../cli/src/project-files.ts";

export const REPO = join(dirname(fromFileUrl(import.meta.url)), "..");
const HEADER = "// GENERATED by `deno task gen` — do not edit.\n";

export async function renderEmbedded(repo: string = REPO): Promise<Map<string, string>> {
  const out = new Map<string, string>();
  const runtime = await Deno.readTextFile(join(repo, "cli", "runtime", "moonwell.lua"));
  out.set("cli/src/embedded/runtime.ts", `${HEADER}export const RUNTIME_LUA: string = ${JSON.stringify(runtime)};\n`);

  const templateDir = join(repo, "template");
  const excluded = new Set<string>(TEMPLATE_EXCLUDE);
  const entries: string[] = [];
  for (const path of await listFiles(templateDir)) {
    if (excluded.has(path) || path.startsWith("dist/")) continue;
    const bytes = await Deno.readFile(join(templateDir, ...path.split("/")));
    entries.push(`  { path: ${JSON.stringify(path)}, base64: ${JSON.stringify(encodeBase64(bytes))} },`);
  }
  out.set(
    "cli/src/embedded/template.ts",
    `${HEADER}export const TEMPLATE_FILES: ReadonlyArray<{ path: string; base64: string }> = [\n${entries.join("\n")}\n];\n`,
  );
  return out;
}

if (import.meta.main) {
  for (const [path, text] of await renderEmbedded()) {
    if (await writeTextIfChanged(join(REPO, ...path.split("/")), text)) console.log(`wrote ${path}`);
  }
}
```

Run: `deno task gen`
Expected: `wrote cli/src/embedded/template.ts`

- [ ] **Step 6: Write the init test (it needs real Pkl)**

`cli/tests/pkl/init.test.ts`:
```ts
import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { init } from "../../src/commands/init.ts";
import { createContext } from "../../src/context.ts";
import { loadProject } from "../../src/project/project.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";

Deno.test("init --link scaffolds a project that loads", async () => {
  const parent = await Deno.makeTempDir();
  const ctx = createContext(parent, silentLogger());
  const project = await init(join(parent, "my-map"), ctx, { link: true });
  for (const file of ["moonwell.pkl", "src/main.yue", "maps/map.w3x/war3map.lua", "PklProject.deps.json", ".gitignore"]) {
    assert(await exists(join(project, file)), file);
  }
  assertStringIncludes(await Deno.readTextFile(join(project, "deno.json")), "cli/src/main.ts build");
  assertEquals((await loadProject(project)).map.folder, "map.w3x");
});

Deno.test("init refuses a non-empty directory", async () => {
  const dir = await Deno.makeTempDir();
  await Deno.writeTextFile(join(dir, "keep.txt"), "");
  await assertRejects(() => init(dir, createContext(dir, silentLogger()), { link: true }), MoonwellError, "not empty");
});
```

Run: `deno task test:pkl`
Expected: FAIL. `init.ts` is not found.

- [ ] **Step 7: Implement init**

`cli/src/commands/init.ts`:
```ts
import { decodeBase64 } from "@std/encoding/base64";
import { exists } from "@std/fs";
import { dirname, fromFileUrl, join, relative, resolve } from "@std/path";
import type { CommandContext } from "../context.ts";
import { TEMPLATE_FILES } from "../embedded/template.ts";
import { projectDenoJson, projectPklProject } from "../project-files.ts";
import { MoonwellError } from "../shared/errors.ts";
import { toPosix } from "../shared/fs.ts";
import { VERSION } from "../version.ts";

/** Scaffolds a project into `dir` (new or empty) and resolves its Pkl dependencies. */
export async function init(dir: string, ctx: CommandContext, options: { link?: boolean } = {}): Promise<string> {
  const target = resolve(ctx.root, dir);
  if (await exists(target)) {
    for await (const _ of Deno.readDir(target)) {
      throw new MoonwellError(`${dir} is not empty.`, { hint: "Choose a new or empty directory." });
    }
  }
  const links = options.link ? localLinks(target) : undefined;

  for (const file of TEMPLATE_FILES) {
    const path = join(target, ...file.path.split("/"));
    await Deno.mkdir(dirname(path), { recursive: true });
    await Deno.writeFile(path, decodeBase64(file.base64));
  }
  await Deno.writeTextFile(join(target, "deno.json"), projectDenoJson(links?.cli ?? `jsr:@moonwell/cli@${VERSION}`));
  await Deno.writeTextFile(
    join(target, "PklProject"),
    projectPklProject(links ? { local: links.pkl } : { version: VERSION }),
  );

  const result = await ctx.run("pkl", ["project", "resolve"], {
    cwd: target,
    notFoundHint: "Install Pkl 0.32 or newer: https://pkl-lang.org/main/current/pkl-cli/index.html#installation",
  });
  if (result.code !== 0) {
    throw new MoonwellError(`pkl project resolve failed:\n${(result.stderr || result.stdout).trim()}`, {
      file: join(dir, "PklProject"),
    });
  }
  ctx.logger.info(`Created ${dir}. Next: cd ${dir} && deno task build`);
  return target;
}

/** Paths from `target` to this checkout's CLI entry and Pkl package; only valid when running from files. */
function localLinks(target: string): { cli: string; pkl: string } {
  if (!import.meta.url.startsWith("file:")) {
    throw new MoonwellError("--link only works when Moonwell runs from a local checkout.");
  }
  const repo = resolve(dirname(fromFileUrl(import.meta.url)), "..", "..", "..");
  const link = (path: string) => toPosix(relative(target, path));
  return { cli: link(join(repo, "cli", "src", "main.ts")), pkl: link(join(repo, "pkl")) };
}
```

In `cli/src/main.ts`, add the import and the command case:
```ts
import { init } from "./commands/init.ts";
```
Add this as the first case in the `switch`:
```ts
      case "init": {
        const dir = flags._[1] === undefined ? undefined : String(flags._[1]);
        if (dir === undefined) throw new MoonwellError("init needs a directory.", { hint: "moonwell init my-map" });
        await init(dir, ctx, { link: flags.link });
        break;
      }
```
Also add `import { MoonwellError } from "./shared/errors.ts";` next to the existing `formatError` import. Merge them into one line: `import { formatError, MoonwellError } from "./shared/errors.ts";`.

- [ ] **Step 8: Run all tests**

Run: `deno task gen && deno task test && deno task test:pkl && deno task check`
Expected: PASS. The embedded freshness test covers `template.ts`.

- [ ] **Step 9: Commit**

```bash
git add deno.json cli tools template
git commit -m "feat: project template and init command

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 16: `dev` watch mode

**Files:**
- Create: `cli/src/commands/dev.ts`
- Modify: `cli/src/main.ts` (add `dev`)
- Test: `cli/tests/unit/dev.test.ts` (the path filter). The watch loop is covered in the Task 17 e2e test.

**Interfaces:**
- Consumes: `check` (Task 14); `formatError`; `toPosix`; `CommandContext`.
- Produces:
  - `isRelevantChange(root: string, path: string): boolean`
  - `dev(ctx: CommandContext, options?: { signal?: AbortSignal; debounceMs?: number }): Promise<void>`

- [ ] **Step 1: Write the failing test**

`cli/tests/unit/dev.test.ts`:
```ts
import { assertEquals } from "@std/assert";
import { join } from "@std/path";
import { isRelevantChange } from "../../src/commands/dev.ts";

Deno.test("isRelevantChange watches Yue sources and project manifests only", () => {
  const root = join("C:", "proj");
  const check = (...parts: string[]) => isRelevantChange(root, join(root, ...parts));
  assertEquals(check("src", "main.yue"), true);
  assertEquals(check("src", "game", "units.yue"), true);
  assertEquals(check("src", "notes.txt"), false);
  assertEquals(check("src", "generated", "objects.yue"), false);
  assertEquals(check("moonwell.pkl"), true);
  assertEquals(check("moonwell.local.pkl"), true);
  assertEquals(check("PklProject"), true);
  assertEquals(check("dist", "stage", "lua", "main.lua"), false);
  assertEquals(check("README.md"), false);
});
```

- [ ] **Step 2: Run to verify it fails**

Run: `deno task test`
Expected: FAIL. `dev.ts` is not found.

- [ ] **Step 3: Implement**

`cli/src/commands/dev.ts`:
```ts
import { join, relative } from "@std/path";
import type { CommandContext } from "../context.ts";
import { formatError } from "../shared/errors.ts";
import { toPosix } from "../shared/fs.ts";
import { check } from "./check.ts";

/** Changes that should trigger a re-check. Generated sources are excluded to avoid feedback loops. */
export function isRelevantChange(root: string, path: string): boolean {
  const rel = toPosix(relative(root, path));
  if (rel.startsWith("src/generated/")) return false;
  if (rel.startsWith("src/")) return rel.endsWith(".yue");
  return /^moonwell(\.local)?\.pkl$/.test(rel) || rel === "PklProject" || rel === "PklProject.deps.json";
}

/** Re-runs `check` whenever sources or manifests change, until `signal` aborts. */
export async function dev(
  ctx: CommandContext,
  options: { signal?: AbortSignal; debounceMs?: number } = {},
): Promise<void> {
  const cycle = async () => {
    try {
      await check(ctx);
    } catch (error) {
      ctx.logger.error(formatError(error));
    }
  };
  await cycle();
  ctx.logger.info("Watching src/ and the project manifests. Press Ctrl+C to stop.");

  const watchers = [
    Deno.watchFs(join(ctx.root, "src"), { recursive: true }),
    Deno.watchFs(ctx.root, { recursive: false }),
  ];
  options.signal?.addEventListener("abort", () => watchers.forEach((watcher) => watcher.close()));

  let timer: number | undefined;
  let running = Promise.resolve();
  const schedule = () => {
    clearTimeout(timer);
    timer = setTimeout(() => {
      running = running.then(cycle);
    }, options.debounceMs ?? 150);
  };
  await Promise.all(watchers.map(async (watcher) => {
    for await (const event of watcher) {
      if (event.paths.some((path) => isRelevantChange(ctx.root, path))) schedule();
    }
  }));
  clearTimeout(timer);
  await running;
}
```

In `cli/src/main.ts`, add `import { dev } from "./commands/dev.ts";` and this case to the `switch`:
```ts
      case "dev":
        await dev(ctx);
        break;
```

- [ ] **Step 4: Run to verify**

Run: `deno task test && deno task check`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add cli
git commit -m "feat: dev watch mode re-checks on save

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 17: End-to-end tests and documentation

**Files:**
- Create: `cli/tests/e2e/project.test.ts`, `README.md`, `CONTRIBUTING.md`, `LICENSE`
- Test: the e2e test above

**Interfaces:**
- Consumes: the whole CLI, run as a subprocess; `openMpq` (test support).

- [ ] **Step 1: Write the e2e tests**

`cli/tests/e2e/project.test.ts`:
```ts
import { assert, assertEquals, assertStringIncludes } from "@std/assert";
import { fromFileUrl, join } from "@std/path";
import { openMpq } from "../support/mpq-reader.ts";

const REPO = fromFileUrl(new URL("../../../", import.meta.url));
const MAIN = join(REPO, "cli", "src", "main.ts");

async function deno(args: string[], cwd: string) {
  const output = await new Deno.Command(Deno.execPath(), { args, cwd, stdout: "piped", stderr: "piped" }).output();
  const decoder = new TextDecoder();
  return { code: output.code, text: decoder.decode(output.stdout) + decoder.decode(output.stderr) };
}

async function newProject(): Promise<string> {
  const project = join(await Deno.makeTempDir({ prefix: "moonwell-e2e-" }), "my-map");
  const result = await deno(["run", "-A", MAIN, "init", "--link", project], REPO);
  assertEquals(result.code, 0, result.text);
  return project;
}

Deno.test("init → build produces an archive with the injected bundle", async () => {
  const project = await newProject();
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  assertStringIncludes(built.text, "Built dist/bin/map.w3x");

  const archive = openMpq(await Deno.readFile(join(project, "dist", "bin", "map.w3x")));
  const lua = new TextDecoder().decode(await archive.read("war3map.lua"));
  assertStringIncludes(lua, "function main()");
  assertStringIncludes(lua, '__mw.define("main", function(...)');
  assertStringIncludes(lua, '__mw.boot("main")');
  assert(await archive.read("war3map.w3i"));
  assert((await archive.listfile()).includes("war3map.lua"));
});

Deno.test("check reports a YueScript syntax error with its source position", async () => {
  const project = await newProject();
  await Deno.writeTextFile(join(project, "src", "main.yue"), "import \"moonwell\" as mw\nx = \n  if then\n");
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1);
  assertStringIncludes(checked.text, "error: src/main.yue:");
});

Deno.test("dev re-checks when a source file changes", async () => {
  const project = await newProject();
  const child = new Deno.Command(Deno.execPath(), {
    args: ["run", "-A", MAIN, "dev"],
    cwd: project,
    stdout: "null",
    stderr: "piped",
  }).spawn();
  const reader = child.stderr.pipeThrough(new TextDecoderStream()).getReader();
  let seen = "";
  const waitFor = async (text: string) => {
    const deadline = Date.now() + 60_000;
    while (!seen.includes(text)) {
      if (Date.now() > deadline) throw new Error(`timed out waiting for "${text}"; output:\n${seen}`);
      const { value, done } = await reader.read();
      if (done) throw new Error(`dev exited early; output:\n${seen}`);
      seen += value;
    }
  };
  try {
    await waitFor("Watching src/");
    await Deno.writeTextFile(join(project, "src", "main.yue"), "x = \n  if then\n");
    await waitFor("error: src/main.yue:");
  } finally {
    child.kill();
    await reader.cancel();
    await child.status;
  }
});
```

- [ ] **Step 2: Run the e2e tests**

Run: `deno task test:e2e`
Expected: PASS (3 tests). The first run downloads yue if it isn't cached. If the build fails, fix the failing component and re-run its own task's tests first. Do not weaken the e2e assertions.

- [ ] **Step 3: Write the documentation**

`LICENSE`:
```
MIT License

Copyright (c) 2026 Moonwell contributors

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

---

The template map in template/maps/ derives from wc3-ts-template and
wc3-dev-framework, distributed under the MIT License:

Copyright (c) 2019 trigger

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

`README.md`:
````markdown
# Moonwell

**Build Warcraft III maps with YueScript gameplay and Pkl project data.**

- Gameplay in [YueScript](https://github.com/IppClub/YueScript), compiled to Lua 5.3 and bundled into your map.
  Only the modules you import are included, and runtime errors point at your `.yue` lines.
- Project configuration in [Pkl](https://pkl-lang.org), validated against the versioned `moonwell` schema package.
- A Deno command-line tool: no Node.js, no `package.json`, no `node_modules`.

## Quickstart

Install [Deno](https://deno.com/) 2.9+ and [Pkl](https://pkl-lang.org) 0.32+.

```powershell
deno run -A jsr:@moonwell/cli init my-map
cd my-map
deno task build
```

Then create `moonwell.local.pkl` to point at your game and play:

```pkl
amends "moonwell.pkl"
launch { gameExecutable = "C:\\Program Files (x86)\\Warcraft III\\_retail_\\x86_64\\Warcraft III.exe" }
```

```powershell
deno task test
```

## A project

| Path | What |
| --- | --- |
| `moonwell.pkl` | Project manifest (`amends "@moonwell/Project.pkl"`); commented examples show every option |
| `moonwell.local.pkl` | Your machine's overrides, git-ignored |
| `src/main.yue` | Gameplay entry |
| `maps/map.w3x/` | World Editor map (folder format, Lua script mode) |
| `dist/` | Build output |

Gameplay registers hooks with the `moonwell` module:

```yue
import "moonwell" as mw

mw.on_main ->
  print "Hello from YueScript"
```

Hooks: `before_config`, `on_config`, `before_main`, `on_main`. A failing hook prints its error with the
`.yue` file and line, and the other hooks still run. Module top-level code runs while the map script loads, so
create game objects inside hooks.

## Commands

| Command | What |
| --- | --- |
| `deno task build [--entry src/x.yue] [--minify]` | Build `dist/bin/<map>.w3x` |
| `deno task test [--entry src/x.yue]` | Stage the map and launch Warcraft III |
| `deno task dev` | Re-check on every save |
| `deno task check` | Compile and validate without building |
| `deno task setup` | Download the pinned YueScript compiler |

The compiler is downloaded once per version into `%LOCALAPPDATA%\moonwell` (or `~/.cache/moonwell`) and
verified by checksum. Set `yue { path = "..." }` in `moonwell.local.pkl` to use your own build.

## Credits

The template map derives from TriggerHappy's [wc3-ts-template](https://github.com/cipherxof/wc3-ts-template)
via wc3-dev-framework. MIT licensed; see [LICENSE](LICENSE).
````

`CONTRIBUTING.md`:
````markdown
# Contributing to Moonwell

## Layout

- `cli/`: the `@moonwell/cli` JSR package (Deno, `jsr:@std/*` only).
- `pkl/`: the `moonwell` Pkl package.
- `template/`: the project `init` scaffolds. It links to `../cli` and `../pkl`, so use it to try changes.
- `tools/gen.ts`: regenerates `cli/src/embedded/` from `cli/runtime/` and `template/`. Run it after changing
  either; a unit test fails when the embedded copies are stale.

## Checks

```powershell
deno task check        # type-check
deno task lint
deno task test         # unit tests, no external tools
deno task test:runtime # needs yue (downloaded automatically)
deno task test:pkl     # needs pkl
deno task test:e2e     # needs pkl and yue
```

Never add `package.json`, `node_modules` or `npm:` imports.

Versions must agree. `cli/deno.json` `version`, `cli/src/version.ts` and `pkl/PklProject` `package.version`
always carry the same number, and a unit test enforces it.

## Release gate (manual, before every release)

1. Run every check above from a clean checkout.
2. `cd template`, create `moonwell.local.pkl` with your `gameExecutable`, and run `deno task test`. Confirm
   "Moonwell is running." prints and the footman changes colour every second.
3. Add `error "gate"` inside the `on_main` hook, run `deno task test` again, and confirm the on-screen error
   names `src/main.yue` and the right line. Record which chunk-name form the game used.
4. Run `deno task build --minify` and play `dist/bin/map.w3x` directly.
5. Open the packed map in World Editor and confirm it loads.
6. Record the Warcraft III and World Editor versions in the changelog.

## Publishing

1. Bump the version in `cli/deno.json`, `cli/src/version.ts` and `pkl/PklProject`.
2. `pkl project package pkl/` produces the package zip and metadata. Create a GitHub release tagged
   `moonwell@<version>` in `mdlsvensson/moonwell` and attach both files.
3. `cd cli && deno publish`.
````

- [ ] **Step 4: Run every check**

Run: `deno task check && deno task lint && deno task test && deno task test:runtime && deno task test:pkl && deno task test:e2e`
Expected: all PASS. Fix any `deno lint` findings in source (not by disabling rules).

- [ ] **Step 5: Commit**

```bash
git add cli README.md CONTRIBUTING.md LICENSE
git commit -m "test: end-to-end init/build/check/dev; docs and license

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

## After this plan

- Run the manual release gate in `CONTRIBUTING.md`. It resolves the open items in spec §11 about the traceback chunk name and in-game behaviour.
- Then write **Plan 2 (data layers)** from spec §6. It will cover:
  - metadata generation;
  - the object-data reader and writer, and `Objects.pkl`/`ObjectFile.pkl`;
  - `src/generated/objects.yue`;
  - assets and map settings;
  - the full w3i reader, which also fills in the HM3W flags and players;
  - the `objects:eval`, `assets:*` and `settings:check` commands.

  It inserts its steps into `prepareStage` between staging and injection. Plan 2 also adds the repo tasks `gen:metadata`, `gen:schema` and `release` (spec §4). Until then, releases follow the manual steps in `CONTRIBUTING.md`.
