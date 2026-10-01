# Moonwell Library Assets (Moonwell 0.6.0) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A library can ship files for the map, declared in its own `moonwell-library.json`; every map that lists the
library imports them with its own assets.

**Architecture:** `libraries/manifest.ts` reads the library's file. `libraries/sync.ts` takes the module folder from it
(the manifest's `dir` still wins) and keeps the assets folder's files in `.moonwell/library-assets/<key>/`, with an
`assets` hash in `moonwell.lock`. `assets/collect.ts` adds those files to the map's own assets: the map's file wins at
the same in-map path, two libraries at one path fail. The one asset list then serves builds, `check`, `assets:check`,
`assets:sync` and `assets:paths`.

**Tech Stack:** Deno, `jsr:@std/*` only; Pkl 0.32; YueScript 0.34.2.

**Spec:** `docs/superpowers/specs/2026-10-01-moonwell-library-assets-design.md`.

**Verified in advance:** every change below was built and run on 2026-10-01 in a scratch clone of the repository
(`../moonwell-proto6`, eight commits on `a038c96`): type check, lint and format; 466 unit tests, 30 runtime, the Pkl
tests and 27 Pkl-backed tests, 33 end-to-end and 2 network tests passed. 34 mutations of the new code are each caught by
a test. The diffs are that clone's, against `a038c96`.

## Global Constraints

- **Repository:** `C:\Users\mdlsvensson\Repo\moonwell`. Commit on `main`, explicit paths only. End every commit message
  with `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **No Node.js:** no `package.json`, no `npm:` or `node:` specifiers; only `jsr:@std/*` from the import map.
- **Errors:** expected failures throw `MoonwellError` with `file` and `hint`.
- **Checks**, each its own command, all green before a commit: `deno task check`, `deno task lint`, `deno fmt --check`,
  `deno task test`; and before the release also `deno task test:runtime`, `deno task test:pkl`, `deno task test:e2e`
  and `MOONWELL_NETWORK_TESTS=1 deno task test:network`.
- **Applying a diff:** save the block to a file and run `git apply <file>`, or make the same edits by hand. Write files
  with the file tools, not shell heredocs: Git Bash and Python heredocs mangle backslashes.
- **After changing `template/`:** run `deno task gen`; the embedded copy is freshness-tested.
- **Names:** the library's file is `moonwell-library.json`; the stamp in a synced library folder is
  `.moonwell-library.json` (with a dot) and is not the same file. Assets are kept in `.moonwell/library-assets/<key>/`.

## Departures from the spec

Recorded in the spec's §12: the lock task comes before the sync task; the upgrade from a 0.5 lock compares the commit
alone when the download ships assets; the commands are tested in-process with real Pkl beside one end-to-end test;
the template's example names the example library's `v0.2.0`, which was pushed while planning (commit `0b69cfa`).

## File Structure

| File | Responsibility |
| --- | --- |
| `cli/src/libraries/manifest.ts` (new) | Reads and validates a library's `moonwell-library.json` |
| `cli/src/libraries/lock.ts` | The optional `assets` hash of a lock entry |
| `cli/src/libraries/sync.ts` | The module folder from the file, the assets set, the stamp's layout, local libraries |
| `cli/src/assets/collect.ts` | `collectProjectAssets`: the map's assets and the libraries', the override, the clash |
| `cli/src/assets/plan.ts` | `planAssets` takes the library keys; `AssetPlan.replaced` |
| `cli/src/pipeline.ts`, `cli/src/commands/{check,assets,assets-paths,dev}.ts` | Pass the libraries through, sync first, print the new lines, watch |
| `cli/tests/unit/library-file.test.ts`, `library-assets-sync.test.ts` (new) | The reader and the sync |
| `cli/tests/pkl/library-assets.test.ts`, `cli/tests/e2e/library-assets.test.ts` (new) | The commands and a real build |

---

### Task 1: Read a library's `moonwell-library.json`

**Files:** Create `cli/src/libraries/manifest.ts`, `cli/tests/unit/library-file.test.ts`.

**Interfaces:**
- Produces: `LIBRARY_FILE = "moonwell-library.json"`; `interface LibraryFile { dir: string | null; assets: string | null }`;
  `parseLibraryFile(key: string, bytes: Uint8Array | undefined, where: string): LibraryFile`, which throws a
  `MoonwellError` with `file: where`.

- [ ] **Step 1: Write the failing tests.** Create `cli/tests/unit/library-file.test.ts`:

```ts
import { assertEquals, assertThrows } from "@std/assert";
import { LIBRARY_FILE, parseLibraryFile } from "../../src/libraries/manifest.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

const bytes = (text: string) => new TextEncoder().encode(text);
const WHERE = "https://github.com/owner/lib/blob/v1/moonwell-library.json";
const REPORT = "Report it to the library's author, or use another tag of the library.";

Deno.test("a library without the file ships modules from its root and no assets", () => {
  assertEquals(LIBRARY_FILE, "moonwell-library.json");
  assertEquals(parseLibraryFile("ex", undefined, WHERE), { dir: null, assets: null });
});

Deno.test("the file names the module folder and the assets folder, each optional", () => {
  assertEquals(parseLibraryFile("ex", bytes('{ "dir": "src", "assets": "assets" }'), WHERE), {
    dir: "src",
    assets: "assets",
  });
  assertEquals(parseLibraryFile("ex", bytes('{"assets":"files/for the map"}'), WHERE), {
    dir: null,
    assets: "files/for the map",
  });
  assertEquals(parseLibraryFile("ex", bytes('{"dir":"lua/lib"}'), WHERE), { dir: "lua/lib", assets: null });
  assertEquals(parseLibraryFile("ex", bytes("{}"), WHERE), { dir: null, assets: null });
});

Deno.test("a file that is not a JSON object is refused, naming the library and the file", () => {
  for (
    const [text, problem] of [["{", "is not valid JSON."], ["[]", "is not a JSON object."], [
      "null",
      "is not a JSON object.",
    ]]
  ) {
    const error = assertThrows(() => parseLibraryFile("ex", bytes(text), WHERE), MoonwellError);
    assertEquals(error.message, `Library ex: moonwell-library.json ${problem}`);
    assertEquals(error.file, WHERE);
    assertEquals(error.hint, REPORT);
  }
  const invalid = assertThrows(
    () => parseLibraryFile("ex", new Uint8Array([...bytes('{"dir":"s'), 0xff, ...bytes('"}')]), WHERE),
    MoonwellError,
  );
  assertEquals(invalid.message, "Library ex: moonwell-library.json is not valid JSON.");
});

Deno.test("an unknown key is refused, the first in sorted order, with the hint about a newer Moonwell", () => {
  const error = assertThrows(
    () => parseLibraryFile("ex", bytes('{"objects":"objects","dir":"src","extra":1}'), WHERE),
    MoonwellError,
  );
  assertEquals(error.message, 'Library ex: moonwell-library.json has an unknown key "extra".');
  assertEquals(error.file, WHERE);
  assertEquals(error.hint, "This Moonwell knows dir and assets; the library may need a newer Moonwell.");
});

Deno.test("a folder must be a relative path of plain names", () => {
  for (const value of ["", ".", "..", "a/../b", "/abs", "a//b", "a/", "a\\b", "C:/x", 7, null, ["src"]]) {
    for (const name of ["dir", "assets"]) {
      const error = assertThrows(
        () => parseLibraryFile("ex", bytes(JSON.stringify({ [name]: value })), WHERE),
        MoonwellError,
      );
      assertEquals(
        error.message,
        `Library ex: moonwell-library.json has ${name} = ${
          JSON.stringify(value)
        }, which is not a folder inside the library.`,
      );
      assertEquals(error.hint, REPORT);
    }
  }
});
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno test -A cli/tests/unit/library-file.test.ts`
Expected: the module `../../src/libraries/manifest.ts` is not found.

- [ ] **Step 3: Implement.** Create `cli/src/libraries/manifest.ts`:

```ts
import { MoonwellError } from "../shared/errors.ts";

/** The file a library describes its own layout with, at its root (spec: library assets §2). */
export const LIBRARY_FILE = "moonwell-library.json";

/** What a library says about itself. `null` is "not given". Both are folders inside the library, with `/`. */
export interface LibraryFile {
  /** The folder module names start from. */
  dir: string | null;
  /** The folder whose files the map imports. */
  assets: string | null;
}

const KEYS = ["dir", "assets"] as const;

/** A relative folder path of plain names: no empty, `.` or `..` segment, no `\` and no `:`. */
function isFolder(value: unknown): value is string {
  return typeof value === "string" && !/[\\:]/.test(value) &&
    value.split("/").every((segment) => segment !== "" && segment !== "." && segment !== "..");
}

/**
 * Reads a library's `moonwell-library.json`. `bytes` is the file's content, or `undefined` for a library without one,
 * which ships nothing but modules from its root. `where` names the file in errors: a path, or a URL for a download.
 */
export function parseLibraryFile(key: string, bytes: Uint8Array | undefined, where: string): LibraryFile {
  if (bytes === undefined) return { dir: null, assets: null };
  const fail = (problem: string, hint: string): never => {
    throw new MoonwellError(`Library ${key}: ${LIBRARY_FILE} ${problem}`, { file: where, hint });
  };
  const report = "Report it to the library's author, or use another tag of the library.";
  let data: unknown;
  try {
    data = JSON.parse(new TextDecoder("utf-8", { fatal: true }).decode(bytes));
  } catch {
    fail("is not valid JSON.", report);
  }
  if (typeof data !== "object" || data === null || Array.isArray(data)) fail("is not a JSON object.", report);
  const fields = data as Record<string, unknown>;
  for (const name of Object.keys(fields).sort()) {
    if (!(KEYS as readonly string[]).includes(name)) {
      fail(
        `has an unknown key "${name}".`,
        `This Moonwell knows ${KEYS.join(" and ")}; the library may need a newer Moonwell.`,
      );
    }
  }
  const folder = (name: (typeof KEYS)[number]): string | null => {
    const value = fields[name];
    if (value === undefined) return null;
    if (!isFolder(value)) {
      fail(`has ${name} = ${JSON.stringify(value)}, which is not a folder inside the library.`, report);
    }
    return value as string;
  };
  return { dir: folder("dir"), assets: folder("assets") };
}
```

- [ ] **Step 4: Run the tests**

Run: `deno test -A cli/tests/unit/library-file.test.ts`
Expected: `ok | 5 passed | 0 failed`.

- [ ] **Step 5: Commit**

```bash
git add cli/src/libraries/manifest.ts cli/tests/unit/library-file.test.ts
git commit -m "feat: read a library's moonwell-library.json"
```

---

### Task 2: An `assets` hash in `moonwell.lock`

**Files:** Modify `cli/src/libraries/lock.ts`, `cli/tests/unit/library-lock.test.ts`.

**Interfaces:**
- Produces: `LockEntry.assets?: string`, read back when present, written last, refused when it is not a string.

- [ ] **Step 1: Write the failing tests.** Apply to `cli/tests/unit/library-lock.test.ts`:

```diff
diff --git a/cli/tests/unit/library-lock.test.ts b/cli/tests/unit/library-lock.test.ts
index da876d3..8ebd9c8 100644
--- a/cli/tests/unit/library-lock.test.ts
+++ b/cli/tests/unit/library-lock.test.ts
@@ -31,7 +31,8 @@ Deno.test("writeLock writes sorted JSON, readLock reads it back, and no librarie
 Deno.test("readLock refuses a lock file it cannot read", async () => {
   const root = await Deno.makeTempDir({ prefix: "moonwell-lock-" });
   try {
-    for (const text of ["not json", '{"libraries": {"a": {"github": 1}}}', "[]"]) {
+    const numbered = JSON.stringify({ libraries: { a: { ...ENTRY, assets: 5 } } });
+    for (const text of ["not json", '{"libraries": {"a": {"github": 1}}}', "[]", numbered]) {
       await Deno.writeTextFile(join(root, LOCK_FILE), text);
       const error = await assertRejects(() => readLock(root), MoonwellError);
       assertEquals(error.file, LOCK_FILE);
@@ -40,3 +41,17 @@ Deno.test("readLock refuses a lock file it cannot read", async () => {
     await Deno.remove(root, { recursive: true });
   }
 });
+
+Deno.test("an entry keeps its assets hash, written last, and an entry without one gets no such key", async () => {
+  const root = await Deno.makeTempDir({ prefix: "moonwell-lock-" });
+  try {
+    const shipping = { ...ENTRY, assets: "sha256:def" };
+    await writeLock(root, { plain: ENTRY, shipping: { assets: "sha256:def", ...ENTRY } });
+    const libraries = JSON.parse(await Deno.readTextFile(join(root, LOCK_FILE))).libraries;
+    assertEquals(Object.keys(libraries.shipping), ["github", "tag", "dir", "commit", "files", "assets"]);
+    assertEquals(Object.keys(libraries.plain), ["github", "tag", "dir", "commit", "files"]);
+    assertEquals(await readLock(root), { plain: ENTRY, shipping });
+  } finally {
+    await Deno.remove(root, { recursive: true });
+  }
+});
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno test -A cli/tests/unit/library-lock.test.ts`
Expected: a type error (`assets` is not a property of `LockEntry`); with `--no-check`, two tests fail.

- [ ] **Step 3: Implement.** Apply to `cli/src/libraries/lock.ts`:

```diff
diff --git a/cli/src/libraries/lock.ts b/cli/src/libraries/lock.ts
index 0884e01..a24eeb3 100644
--- a/cli/src/libraries/lock.ts
+++ b/cli/src/libraries/lock.ts
@@ -5,13 +5,17 @@ import { removeFileIfExists, writeTextIfChanged } from "../shared/fs.ts";
 /** The committed lock file, at the project root (spec §4.3). */
 export const LOCK_FILE = "moonwell.lock";
 
-/** What a GitHub library resolved to: the manifest's `github`, `tag` and `dir`, the tag's commit and the kept files. */
+/**
+ * What a GitHub library resolved to: the manifest's `github`, `tag` and `dir`, the tag's commit, the hash of the kept
+ * module files and, for a library that ships assets, the hash of those (spec: library assets §4.4).
+ */
 export interface LockEntry {
   github: string;
   tag: string;
   dir: string;
   commit: string;
   files: string;
+  assets?: string;
 }
 
 const HINT = "Fix it, or delete it: the next check downloads every library again and writes a new one.";
@@ -19,7 +23,8 @@ const HINT = "Fix it, or delete it: the next check downloads every library again
 function isEntry(value: unknown): value is LockEntry {
   const entry = value as Record<string, unknown>;
   return typeof value === "object" && value !== null &&
-    ["github", "tag", "dir", "commit", "files"].every((key) => typeof entry[key] === "string");
+    ["github", "tag", "dir", "commit", "files"].every((key) => typeof entry[key] === "string") &&
+    (entry.assets === undefined || typeof entry.assets === "string");
 }
 
 /** The lock's entries by library key; none when there is no lock file. */
@@ -49,8 +54,8 @@ export async function readLock(root: string): Promise<Record<string, LockEntry>>
 }
 
 /**
- * Writes the entries sorted by key, each as github, tag, dir, commit, files, with two-space indentation; only when
- * the text changes. No entries removes the file (plan decision).
+ * Writes the entries sorted by key, each as github, tag, dir, commit, files and (when it has one) assets, with
+ * two-space indentation; only when the text changes. No entries removes the file (plan decision).
  */
 export async function writeLock(root: string, libraries: Record<string, LockEntry>): Promise<void> {
   const path = join(root, LOCK_FILE);
@@ -60,8 +65,8 @@ export async function writeLock(root: string, libraries: Record<string, LockEntr
     return;
   }
   const sorted = Object.fromEntries(keys.map((key) => {
-    const { github, tag, dir, commit, files } = libraries[key];
-    return [key, { github, tag, dir, commit, files }];
+    const { github, tag, dir, commit, files, assets } = libraries[key];
+    return [key, { github, tag, dir, commit, files, ...(assets === undefined ? {} : { assets }) }];
   }));
   await writeTextIfChanged(path, `${JSON.stringify({ libraries: sorted }, null, 2)}\n`);
 }
```

- [ ] **Step 4: Run the tests**

Run: `deno test -A cli/tests/unit/library-lock.test.ts`
Expected: `ok | 3 passed | 0 failed`.

- [ ] **Step 5: Commit**

```bash
git add cli/src/libraries/lock.ts cli/tests/unit/library-lock.test.ts
git commit -m "feat: an assets hash in moonwell.lock"
```

---

### Task 3: Sync the module folder and the assets from the library's file

**Files:** Replace `cli/src/libraries/sync.ts`; create `cli/tests/unit/library-assets-sync.test.ts`.

**Interfaces:**
- Consumes: `parseLibraryFile`, `LIBRARY_FILE` (Task 1); `LockEntry.assets` (Task 2).
- Produces: `LIBRARY_ASSETS_DIR = ".moonwell/library-assets"`; `syncLibraries` (same signature) now also keeps
  `.moonwell/library-assets/<key>/` up to date and writes `assets` into the lock.

- [ ] **Step 1: Write the failing tests.** Create `cli/tests/unit/library-assets-sync.test.ts`:

```ts
import { assertEquals, assertMatch, assertRejects } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { filesHash } from "../../src/libraries/archive.ts";
import { LOCK_FILE, readLock } from "../../src/libraries/lock.ts";
import { archiveUrl, LIBRARY_ASSETS_DIR, syncLibraries } from "../../src/libraries/sync.ts";
import type { Library } from "../../src/project/project.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { listFiles } from "../../src/shared/fs.ts";
import { silentLogger } from "../support/logger.ts";
import { makeZip } from "../support/zip.ts";

const COMMIT_A = "a".repeat(40);
const COMMIT_B = "b".repeat(40);
const URL_V1 = archiveUrl("owner/lib", "v0.1.0");
const WHERE = "https://github.com/owner/lib/blob/v0.1.0/moonwell-library.json";
const text = (value: string) => new TextEncoder().encode(value);
const github = (tag = "v0.1.0", dir = ""): Library => ({ github: "owner/lib", tag, path: null, dir });
const local = (path: string, dir = ""): Library => ({ github: null, tag: null, path, dir });

function server(archives: Record<string, Uint8Array>) {
  const requests: string[] = [];
  const fetch = (url: string) => {
    requests.push(url);
    const body = archives[url];
    return Promise.resolve(body ? new Response(body.slice()) : new Response("Not Found", { status: 404 }));
  };
  return { fetch, requests };
}

async function archive(commit: string, files: Record<string, string>): Promise<Uint8Array> {
  return await makeZip(
    Object.entries(files).map(([name, data]) => ({ name: `lib-0.1.0/${name}`, data: text(data) })),
    commit,
  );
}

async function withRoot(body: (root: string) => Promise<void>) {
  const root = await Deno.makeTempDir({ prefix: "moonwell-libassets-" });
  try {
    await body(root);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
}

async function write(folder: string, files: Record<string, string>) {
  for (const [path, data] of Object.entries(files)) {
    await Deno.mkdir(join(folder, ...path.split("/").slice(0, -1)), { recursive: true });
    await Deno.writeTextFile(join(folder, ...path.split("/")), data);
  }
}

const read = (root: string, path: string) => Deno.readTextFile(join(root, ...path.split("/")));
const list = async (root: string, path: string) =>
  (await exists(join(root, ...path.split("/")))) ? await listFiles(join(root, ...path.split("/"))) : undefined;
const SHIPPING = {
  "moonwell-library.json": '{"dir":"src","assets":"assets"}',
  "README.md": "# lib",
  "src/example/greet.lua": "return {}",
  "assets/Models/Golem.mdx": "model",
  "assets/war3mapImported/lib/ui.toc": "toc",
  "assets/.hidden": "no",
  "assets/.git/config": "no",
};

Deno.test("the library's file names its module folder, and the manifest's dir wins over it", async () => {
  await withRoot(async (root) => {
    const { fetch } = server({
      [URL_V1]: await archive(COMMIT_A, {
        "moonwell-library.json": '{"dir":"src"}',
        "src/a.lua": "from src",
        "other/b.lua": "from other",
      }),
    });
    const deps = { fetch, logger: silentLogger() };
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/ex"), [".moonwell-library.json", "a.lua"]);
    assertEquals((await readLock(root)).ex.dir, "", "the lock keeps the manifest's dir");
    assertEquals((await readLock(root)).ex.assets, undefined);
    assertEquals(await exists(join(root, ".moonwell", "library-assets")), false);
    await syncLibraries(root, { ex: github("v0.1.0", "other") }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/ex"), [".moonwell-library.json", "b.lua"]);
    assertEquals((await readLock(root)).ex.dir, "other");
  });
});

Deno.test("a library's assets are kept beside its modules, without dot-names, and locked by their hash", async () => {
  await withRoot(async (root) => {
    assertEquals(LIBRARY_ASSETS_DIR, ".moonwell/library-assets");
    const { fetch, requests } = server({ [URL_V1]: await archive(COMMIT_A, SHIPPING) });
    const deps = { fetch, logger: silentLogger() };
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/ex"), [".moonwell-library.json", "example/greet.lua"]);
    assertEquals(await list(root, ".moonwell/library-assets/ex"), ["Models/Golem.mdx", "war3mapImported/lib/ui.toc"]);
    assertEquals(await read(root, ".moonwell/library-assets/ex/Models/Golem.mdx"), "model");
    const lock = (await readLock(root)).ex;
    assertEquals(
      lock.assets,
      await filesHash(new Map([["Models/Golem.mdx", text("model")], ["war3mapImported/lib/ui.toc", text("toc")]])),
    );
    assertEquals(lock.files, await filesHash(new Map([["example/greet.lua", text("return {}")]])));
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(requests.length, 1, "folders that hold the lock entry are not fetched again");
    await Deno.remove(join(root, ".moonwell", "library-assets"), { recursive: true });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(requests.length, 2, "a missing assets folder is fetched again");
    assertEquals(await read(root, ".moonwell/library-assets/ex/war3mapImported/lib/ui.toc"), "toc");
  });
});

Deno.test("an assets folder inside the module folder holds no modules", async () => {
  await withRoot(async (root) => {
    const { fetch } = server({
      [URL_V1]: await archive(COMMIT_A, {
        "moonwell-library.json": '{"assets":"assets"}',
        "a.lua": "module",
        "assets/b.lua": "asset",
        "assetsmore/c.lua": "module",
      }),
    });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch, logger: silentLogger() });
    assertEquals(await list(root, ".moonwell/libraries/ex"), [
      ".moonwell-library.json",
      "a.lua",
      "assetsmore/c.lua",
      "moonwell-library.json",
    ]);
    assertEquals(await list(root, ".moonwell/library-assets/ex"), ["b.lua"]);
  });
});

Deno.test("a folder the library's file names without files, and a bad file, are refused naming the file", async () => {
  const cases: Array<[Record<string, string>, string, string]> = [
    [
      { "moonwell-library.json": '{"assets":"art"}', "a.lua": "1" },
      "Library ex has no folder art at v0.1.0.",
      "Its moonwell-library.json names an assets folder that has no files; report it to the library's author.",
    ],
    [
      { "moonwell-library.json": '{"dir":"lua"}', "a.lua": "1" },
      "Library ex has no folder lua at v0.1.0.",
      "Its moonwell-library.json names a dir that has no files.",
    ],
    [
      { "moonwell-library.json": '{"objects":"objects"}', "a.lua": "1" },
      'Library ex: moonwell-library.json has an unknown key "objects".',
      "This Moonwell knows dir and assets; the library may need a newer Moonwell.",
    ],
  ];
  for (const [files, message, hint] of cases) {
    await withRoot(async (root) => {
      const { fetch } = server({ [URL_V1]: await archive(COMMIT_A, files) });
      const error = await assertRejects(
        () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch, logger: silentLogger() }),
        MoonwellError,
      );
      assertEquals([error.message, error.file, error.hint], [message, WHERE, hint]);
      assertEquals(await exists(join(root, ".moonwell", "libraries", "ex")), false);
      assertEquals(await exists(join(root, LOCK_FILE)), false);
    });
  }
});

Deno.test("a folder from before assets is fetched once more, and its lock is upgraded by its commit", async () => {
  await withRoot(async (root) => {
    const first = server({ [URL_V1]: await archive(COMMIT_A, SHIPPING) });
    // What Moonwell 0.5 left: a lock entry with dir "src" and no assets hash, and a stamp without a layout.
    const old = { github: "owner/lib", tag: "v0.1.0", dir: "src", commit: COMMIT_A, files: "sha256:from-0.5" };
    await Deno.writeTextFile(join(root, LOCK_FILE), JSON.stringify({ libraries: { ex: old } }));
    await write(join(root, ".moonwell", "libraries", "ex"), {
      ".moonwell-library.json": JSON.stringify(old),
      "example/greet.lua": "return {}",
    });
    const library = github("v0.1.0", "src");
    await syncLibraries(root, { ex: library }, "moonwell.pkl", { fetch: first.fetch, logger: silentLogger() });
    assertEquals(first.requests.length, 1);
    const upgraded = (await readLock(root)).ex;
    assertMatch(upgraded.assets ?? "", /^sha256:[0-9a-f]{64}$/);
    assertEquals(upgraded.commit, COMMIT_A);
    assertEquals(await list(root, ".moonwell/library-assets/ex"), ["Models/Golem.mdx", "war3mapImported/lib/ui.toc"]);
    await syncLibraries(root, { ex: library }, "moonwell.pkl", { fetch: first.fetch, logger: silentLogger() });
    assertEquals(first.requests.length, 1, "the new stamp holds the upgraded entry");

    // The same old lock against another commit is a moved tag, as before.
    await Deno.writeTextFile(join(root, LOCK_FILE), JSON.stringify({ libraries: { ex: old } }));
    await Deno.remove(join(root, ".moonwell"), { recursive: true });
    const moved = server({ [URL_V1]: await archive(COMMIT_B, SHIPPING) });
    await assertRejects(
      () => syncLibraries(root, { ex: library }, "moonwell.pkl", { fetch: moved.fetch, logger: silentLogger() }),
      MoonwellError,
      "moved",
    );
  });
});

Deno.test("with an assets hash in the lock, changed assets or modules under the same commit are a moved tag", async () => {
  await withRoot(async (root) => {
    const first = server({ [URL_V1]: await archive(COMMIT_A, SHIPPING) });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: first.fetch, logger: silentLogger() });
    await Deno.remove(join(root, ".moonwell"), { recursive: true });
    for (const changed of [{ "assets/Models/Golem.mdx": "another model" }, { "src/example/greet.lua": "return 1" }]) {
      const other = server({ [URL_V1]: await archive(COMMIT_A, { ...SHIPPING, ...changed }) });
      const error = await assertRejects(
        () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: other.fetch, logger: silentLogger() }),
        MoonwellError,
        "moved",
      );
      assertEquals(error.file, LOCK_FILE);
      assertEquals(await exists(join(root, ".moonwell", "library-assets", "ex")), false);
    }
  });
});

Deno.test("a library that stops shipping assets, or leaves the manifest, loses its assets folder", async () => {
  await withRoot(async (root) => {
    const { fetch } = server({
      [URL_V1]: await archive(COMMIT_A, SHIPPING),
      [archiveUrl("owner/lib", "v0.2.0")]: await archive(COMMIT_B, { "src/a.lua": "1" }),
    });
    const deps = { fetch, logger: silentLogger() };
    await syncLibraries(root, { ex: github(), other: github() }, "moonwell.pkl", deps);
    assertEquals(await exists(join(root, ".moonwell", "library-assets", "other", "Models", "Golem.mdx")), true);
    await syncLibraries(root, { ex: github("v0.2.0", "src") }, "moonwell.pkl", deps);
    assertEquals(await exists(join(root, ".moonwell", "library-assets", "ex")), false);
    assertEquals(await exists(join(root, ".moonwell", "library-assets", "other")), false);
    assertEquals((await readLock(root)).ex.assets, undefined);
    assertEquals(Object.keys(await readLock(root)), ["ex"]);
  });
});

Deno.test("a local library's file is read from its path, and its assets are mirrored", async () => {
  await withRoot(async (root) => {
    const source = join(root, "lib");
    await write(source, {
      "moonwell-library.json": '{"dir":"src","assets":"assets"}',
      "src/a.lua": "return 1",
      "other/b.lua": "return 2",
      "assets/icons/BTNGolem.blp": "icon",
      "assets/old.txt": "old",
      "assets/.DS_Store": "no",
      "assets/.cache/x.bin": "no",
    });
    const deps = { fetch: () => Promise.reject(new Error("no network in this test")), logger: silentLogger() };
    await syncLibraries(root, { mine: local("lib") }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/mine"), [".moonwell-library.json", "a.lua"]);
    assertEquals(await list(root, ".moonwell/library-assets/mine"), ["icons/BTNGolem.blp", "old.txt"]);
    assertEquals(await exists(join(root, LOCK_FILE)), false);

    await Deno.remove(join(source, "assets", "old.txt"));
    await Deno.writeTextFile(join(source, "assets", "icons", "BTNGolem.blp"), "new icon");
    await syncLibraries(root, { mine: local("lib", "other") }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/mine"), [".moonwell-library.json", "b.lua"]);
    assertEquals(await list(root, ".moonwell/library-assets/mine"), ["icons/BTNGolem.blp"]);
    assertEquals(await read(root, ".moonwell/library-assets/mine/icons/BTNGolem.blp"), "new icon");

    await Deno.writeTextFile(join(source, "moonwell-library.json"), '{"dir":"src","assets":"missing"}');
    const error = await assertRejects(
      () => syncLibraries(root, { mine: local("lib") }, "moonwell.pkl", deps),
      MoonwellError,
    );
    assertEquals(error.message, `Library mine: ${join(source, "missing")} is not a folder.`);
    assertEquals(error.file, join(source, "moonwell-library.json"));
    assertEquals(error.hint, "Create the folder, or fix assets in the library's moonwell-library.json.");

    await Deno.writeTextFile(join(source, "moonwell-library.json"), '{"dir":"src"}');
    await syncLibraries(root, { mine: local("lib") }, "moonwell.pkl", deps);
    assertEquals(await exists(join(root, ".moonwell", "library-assets", "mine")), false);

    await Deno.writeTextFile(join(source, "moonwell-library.json"), '{"dir":7}');
    const bad = await assertRejects(
      () => syncLibraries(root, { mine: local("lib") }, "moonwell.pkl", deps),
      MoonwellError,
      "Library mine: moonwell-library.json has dir = 7",
    );
    assertEquals(bad.file, join(source, "moonwell-library.json"));
  });
});

Deno.test("a local library with assets inside its module folder copies none of them as modules", async () => {
  await withRoot(async (root) => {
    const source = join(root, "lib");
    await write(source, {
      "moonwell-library.json": '{"assets":"files/assets"}',
      "a.lua": "return 1",
      "files/b.lua": "return 2",
      "files/assets/c.lua": "an asset",
      "files/assets/d.mdx": "model",
    });
    const deps = { fetch: () => Promise.reject(new Error("no network in this test")), logger: silentLogger() };
    await syncLibraries(root, { mine: local(source) }, "moonwell.pkl", deps);
    assertEquals(await list(root, ".moonwell/libraries/mine"), [".moonwell-library.json", "a.lua", "files/b.lua"]);
    assertEquals(await list(root, ".moonwell/library-assets/mine"), ["c.lua", "d.mdx"]);
  });
});
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno test -A cli/tests/unit/library-assets-sync.test.ts`
Expected: a type error: `sync.ts` has no export `LIBRARY_ASSETS_DIR`.

- [ ] **Step 3: Implement.** Replace `cli/src/libraries/sync.ts` with:

```ts
import { dirname, join, resolve } from "@std/path";
import type { Library } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { isWithin, listFiles, removeIfExists, writeTextIfChanged } from "../shared/fs.ts";
import type { Logger } from "../shared/log.ts";
import { filesHash } from "./archive.ts";
import { downloadTag, type Fetch, reasonOf } from "./download.ts";
import { LOCK_FILE, type LockEntry, readLock, writeLock } from "./lock.ts";
import { LIBRARY_FILE, parseLibraryFile } from "./manifest.ts";

/** Where libraries' modules live in a project (git-ignored with .moonwell/). */
export const LIBRARIES_DIR = ".moonwell/libraries";

/** Where the files libraries ship for the map live, by library key (spec: library assets §4.1). */
export const LIBRARY_ASSETS_DIR = ".moonwell/library-assets";

/** What a library folder holds: its lock entry, or the local path it was copied from. */
const STAMP = ".moonwell-library.json";

/** The stamp's layout: 2 since a library can ship assets. A folder with another layout is fetched again. */
const LAYOUT = 2;

export { archiveUrl } from "./download.ts";

export interface SyncDeps {
  fetch: Fetch;
  logger: Logger;
}

/** The two folders under `.moonwell/` that hold every library's modules and assets. */
interface Folders {
  modules: string;
  assets: string;
}

/**
 * Brings `.moonwell/libraries/<key>/` and `.moonwell/library-assets/<key>/` up to date for every library of the
 * manifest, and `moonwell.lock` with the GitHub ones (spec §4.2, §4.3; library assets §4). A GitHub library whose
 * folders already hold its lock entry is not downloaded again; a local library keeps the lock entry it had.
 */
export async function syncLibraries(
  root: string,
  libraries: Record<string, Library>,
  manifest: string,
  deps: SyncDeps,
): Promise<void> {
  const keys = Object.keys(libraries).sort();
  refuseCaseClashes(keys, manifest);
  const folders: Folders = {
    modules: join(root, ...LIBRARIES_DIR.split("/")),
    assets: join(root, ...LIBRARY_ASSETS_DIR.split("/")),
  };
  // Before syncing: on a case-insensitive file system, a stale folder `lib` would otherwise take key `Lib`'s files and
  // then be removed as stale.
  await removeStale(folders.modules, LIBRARIES_DIR, new Set(keys));
  await removeStale(folders.assets, LIBRARY_ASSETS_DIR, new Set(keys));
  const lock = await readLock(root);
  const next: Record<string, LockEntry> = {};
  for (const key of keys) {
    const library = libraries[key];
    if (library.path !== null) {
      await syncLocal(root, folders, key, library.path, library.dir, manifest);
      // A path usually comes from moonwell.local.pkl, which is not committed: keep the committed lock entry, so
      // switching back still checks the tag.
      if (lock[key] !== undefined) next[key] = lock[key];
    } else next[key] = await syncGitHub(folders, key, library, lock[key], manifest, deps);
  }
  try {
    await writeLock(root, next);
  } catch (cause) {
    throw new MoonwellError(`Writing ${LOCK_FILE} failed: ${reasonOf(cause)}`, {
      file: LOCK_FILE,
      cause,
      hint: "Close programs that have moonwell.lock open, and check it is not read-only.",
    });
  }
}

/** Keys such as `Lib` and `lib` would share one folder on Windows. */
function refuseCaseClashes(keys: string[], manifest: string): void {
  const seen = new Map<string, string>();
  for (const key of keys) {
    const other = seen.get(key.toLowerCase());
    if (other !== undefined) {
      throw new MoonwellError(`Libraries ${other} and ${key} differ only by case.`, {
        file: manifest,
        hint: "Rename one of them: each library gets a folder in .moonwell/libraries/.",
      });
    }
    seen.set(key.toLowerCase(), key);
  }
}

/** Removes folders of libraries no longer in the manifest, and leftover `.<key>.tmp` folders. */
async function removeStale(folder: string, label: string, keys: Set<string>): Promise<void> {
  const names: string[] = [];
  await writing(label, "", async () => {
    try {
      for await (const entry of Deno.readDir(folder)) names.push(entry.name);
    } catch (cause) {
      if (!(cause instanceof Deno.errors.NotFound)) throw cause;
    }
  });
  for (const name of names) {
    if (!keys.has(name)) await writing(label, name, () => removeIfExists(join(folder, name)));
  }
}

/** A file's bytes, or `undefined` when it cannot be read: a library without the file is the usual case. */
async function readIfFile(path: string): Promise<Uint8Array | undefined> {
  try {
    return await Deno.readFile(path);
  } catch {
    return undefined;
  }
}

/**
 * Copies a local library from `<path>`: its modules (`.yue` and `.lua` files) under the module folder and, when its
 * `moonwell-library.json` names one, every file under its assets folder; both outside dot-names such as `.git/`.
 * Only changed files are written, and every other file is removed.
 */
async function syncLocal(root: string, folders: Folders, key: string, path: string, dir: string, manifest: string) {
  const base = resolve(root, path);
  const file = join(base, LIBRARY_FILE);
  const described = parseLibraryFile(key, await readIfFile(file), file);
  const source = resolve(base, dir !== "" ? dir : described.dir ?? "");
  const info = await Deno.stat(source).catch(() => undefined);
  if (!info?.isDirectory) {
    throw new MoonwellError(`Library ${key}: ${source} is not a folder.`, {
      file: manifest,
      hint: "Set the library's path (and dir) to a folder that holds its modules.",
    });
  }
  if (isWithin(folders.modules, source)) {
    throw new MoonwellError(`Library ${key}: ${source} contains this project's ${LIBRARIES_DIR}.`, {
      file: manifest,
      hint: "Point the library's path (and dir) at the folder that holds its modules, not at the project.",
    });
  }
  const assetsSource = described.assets === null ? undefined : resolve(base, described.assets);
  if (assetsSource !== undefined && !(await Deno.stat(assetsSource).catch(() => undefined))?.isDirectory) {
    throw new MoonwellError(`Library ${key}: ${assetsSource} is not a folder.`, {
      file,
      hint: `Create the folder, or fix assets in the library's ${LIBRARY_FILE}.`,
    });
  }
  const files = new Map<string, Uint8Array>();
  const assets = new Map<string, Uint8Array>();
  try {
    for (const name of await listLocalFiles(source, /\.(yue|lua)$/, assetsSource)) {
      files.set(name, await Deno.readFile(join(source, ...name.split("/"))));
    }
    if (assetsSource !== undefined) {
      for (const name of await listLocalFiles(assetsSource, /^/)) {
        assets.set(name, await Deno.readFile(join(assetsSource, ...name.split("/"))));
      }
    }
  } catch (cause) {
    throw new MoonwellError(`Reading library ${key} from ${source} failed: ${reasonOf(cause)}`, {
      file: manifest,
      cause,
      hint: "Check the library's path and that its files can be read.",
    });
  }
  await writing(LIBRARY_ASSETS_DIR, key, async () => {
    if (assetsSource === undefined) await removeIfExists(join(folders.assets, key));
    else await mirror(join(folders.assets, key), assets);
  });
  const target = join(folders.modules, key);
  await writing(LIBRARIES_DIR, key, async () => {
    await mirror(target, files, STAMP);
    await writeTextIfChanged(join(target, STAMP), stampText({ path: source }));
  });
}

/** Makes `target` hold exactly `files` (and `keep`, when named), writing only the files that changed. */
async function mirror(target: string, files: Map<string, Uint8Array>, keep?: string): Promise<void> {
  for (const [file, data] of files) await writeBytesIfChanged(join(target, ...file.split("/")), data);
  await Deno.mkdir(target, { recursive: true });
  for (const file of await listFiles(target)) {
    if (file !== keep && !files.has(file)) await Deno.remove(join(target, ...file.split("/")));
  }
}

/**
 * The files under `dir` whose name matches, as POSIX paths. It never takes a file or looks inside a folder whose name
 * starts with `.`, and never looks inside `skip` (the assets folder, when it lies inside the module folder).
 */
async function listLocalFiles(dir: string, name: RegExp, skip?: string, prefix = ""): Promise<string[]> {
  const files: string[] = [];
  for await (const entry of Deno.readDir(dir)) {
    if (entry.name.startsWith(".")) continue;
    const path = `${prefix}${entry.name}`;
    const full = join(dir, entry.name);
    if (entry.isDirectory) {
      if (skip !== undefined && isWithin(full, skip)) continue;
      files.push(...await listLocalFiles(full, name, skip, `${path}/`));
    } else if (name.test(entry.name)) files.push(path);
  }
  return files;
}

/** Downloads a GitHub library unless its folders hold its lock entry; returns its lock entry. */
async function syncGitHub(
  folders: Folders,
  key: string,
  library: Library,
  locked: LockEntry | undefined,
  manifest: string,
  deps: SyncDeps,
): Promise<LockEntry> {
  const [github, tag, dir] = [library.github!, library.tag!, library.dir];
  const repository = github.split("/")[1];
  if (tag.split("/").some((segment) => segment === "." || segment === "..")) {
    throw new MoonwellError(`Library ${key}: ${tag} is not a tag name.`, {
      file: manifest,
      hint: `Use the tag's name as it appears at https://github.com/${github}/tags.`,
    });
  }
  if (repository === "." || repository === "..") {
    throw new MoonwellError(`Library ${key}: ${github} is not a GitHub repository.`, {
      file: manifest,
      hint: 'Write it as "owner/repo".',
    });
  }
  const sameTag = locked !== undefined && locked.github === github && locked.tag === tag && locked.dir === dir;
  if (sameTag && await holds(folders, key, locked)) return locked;
  const { commit, files } = await downloadTag(key, github, tag, manifest, deps.fetch);
  const where = `https://github.com/${github}/blob/${tag}/${LIBRARY_FILE}`;
  const described = parseLibraryFile(key, files.get(LIBRARY_FILE), where);
  const moduleDir = dir !== "" ? dir : described.dir ?? "";
  const kept = keepDir(files, moduleDir, described.assets ?? undefined);
  if (kept.size === 0) {
    throw new MoonwellError(`Library ${key} has no folder ${moduleDir} at ${tag}.`, {
      file: dir !== "" ? manifest : where,
      hint: dir !== "" ? "Fix the library's dir." : `Its ${LIBRARY_FILE} names a dir that has no files.`,
    });
  }
  const assets = described.assets === null ? undefined : keepDir(files, described.assets);
  if (assets !== undefined && assets.size === 0) {
    throw new MoonwellError(`Library ${key} has no folder ${described.assets} at ${tag}.`, {
      file: where,
      hint: `Its ${LIBRARY_FILE} names an assets folder that has no files; report it to the library's author.`,
    });
  }
  const entry: LockEntry = {
    github,
    tag,
    dir,
    commit,
    files: await filesHash(kept),
    ...(assets === undefined ? {} : { assets: await filesHash(assets) }),
  };
  // A lock from before libraries shipped assets has no assets hash, and its module hash may count files that are
  // assets now: then only the commit is compared (spec: library assets §4.4).
  const comparable = sameTag && (locked.assets === undefined) === (entry.assets === undefined);
  if (
    sameTag &&
    (entry.commit !== locked.commit || (comparable && (entry.files !== locked.files || entry.assets !== locked.assets)))
  ) {
    throw new MoonwellError(
      `Library ${key}: tag ${tag} of ${github} moved from ${locked.commit.slice(0, 12)} to ${
        entry.commit.slice(0, 12)
      } since moonwell.lock recorded it.`,
      {
        file: LOCK_FILE,
        hint: "If the move was intended, delete the library's entry from moonwell.lock and run the command again.",
      },
    );
  }
  // The assets first and the stamp last: an interrupted sync leaves folders that are fetched again.
  await writing(LIBRARY_ASSETS_DIR, key, async () => {
    if (assets === undefined) await removeIfExists(join(folders.assets, key));
    else await replaceFolder(folders.assets, key, assets);
  });
  await writing(LIBRARIES_DIR, key, () => replaceFolder(folders.modules, key, kept, entry));
  deps.logger.info(`Fetched library ${key}: ${github} ${tag} (${entry.commit.slice(0, 7)}).`);
  return entry;
}

/** Whether the library's folders hold `entry`: the stamp says so, and its assets folder is there when it has assets. */
async function holds(folders: Folders, key: string, entry: LockEntry): Promise<boolean> {
  if (!sameEntry(await readStamp(join(folders.modules, key)), entry)) return false;
  if (entry.assets === undefined) return true;
  return (await Deno.stat(join(folders.assets, key)).catch(() => undefined))?.isDirectory === true;
}

/**
 * The files under `dir` (all of them when it is empty), relative to it, except those in a folder or with a name that
 * starts with `.` (such as `.github/`, and the stamp), and except those under the folder `except`.
 */
function keepDir(files: Map<string, Uint8Array>, dir: string, except?: string): Map<string, Uint8Array> {
  const prefix = dir.split(/[\\/]/).filter((segment) => segment !== "" && segment !== ".").join("/");
  const kept = new Map<string, Uint8Array>();
  for (const [path, data] of files) {
    if (except !== undefined && path.startsWith(`${except}/`)) continue;
    const relative = prefix === "" ? path : path.startsWith(`${prefix}/`) ? path.slice(prefix.length + 1) : undefined;
    if (relative !== undefined && !relative.split("/").some((segment) => segment.startsWith("."))) {
      kept.set(relative, data);
    }
  }
  return kept;
}

/** Writes the files, and the stamp of `entry` when given, into `.<key>.tmp/`, which then replaces `<folder>/<key>`. */
async function replaceFolder(folder: string, key: string, files: Map<string, Uint8Array>, entry?: LockEntry) {
  const temp = join(folder, `.${key}.tmp`);
  await removeIfExists(temp);
  for (const [path, data] of files) {
    const file = join(temp, ...path.split("/"));
    await Deno.mkdir(dirname(file), { recursive: true });
    await Deno.writeFile(file, data);
  }
  if (entry !== undefined) await writeTextIfChanged(join(temp, STAMP), stampText({ ...entry, layout: LAYOUT }));
  const target = join(folder, key);
  await removeIfExists(target);
  await Deno.rename(temp, target);
}

/** The stamp in a library's folder; `undefined` when there is none or it is unreadable. */
async function readStamp(target: string): Promise<unknown> {
  try {
    return JSON.parse(await Deno.readTextFile(join(target, STAMP)));
  } catch {
    return undefined;
  }
}

function sameEntry(stamp: unknown, entry: LockEntry): boolean {
  if (typeof stamp !== "object" || stamp === null) return false;
  const fields = stamp as Record<string, unknown>;
  return fields.layout === LAYOUT &&
    (["github", "tag", "dir", "commit", "files", "assets"] as const).every((field) => fields[field] === entry[field]);
}

const stampText = (value: object) => `${JSON.stringify(value, null, 2)}\n`;

async function writeBytesIfChanged(path: string, bytes: Uint8Array): Promise<void> {
  try {
    const old = await Deno.readFile(path);
    if (old.length === bytes.length && old.every((byte, index) => byte === bytes[index])) return;
  } catch (cause) {
    if (!(cause instanceof Deno.errors.NotFound)) throw cause;
  }
  await Deno.mkdir(dirname(path), { recursive: true });
  await Deno.writeFile(path, bytes);
}

/** Runs `body`, reporting any failure as one writing `<label>/<name>` (the folder `label` itself for ""). */
async function writing<T>(label: string, name: string, body: () => Promise<T>): Promise<T> {
  try {
    return await body();
  } catch (cause) {
    const path = name === "" ? label : `${label}/${name}`;
    throw new MoonwellError(`Writing ${path} failed: ${reasonOf(cause)}`, {
      file: path,
      cause,
      hint: "Close programs that have files in .moonwell/ open, then retry.",
    });
  }
}
```

- [ ] **Step 4: Run the tests**

Run: `deno test -A cli/tests/unit/library-assets-sync.test.ts cli/tests/unit/library-sync.test.ts`
Expected: `ok | 25 passed | 0 failed` (9 new, and the 16 existing sync tests unchanged).

- [ ] **Step 5: Commit**

```bash
git add cli/src/libraries/sync.ts cli/tests/unit/library-assets-sync.test.ts
git commit -m "feat: libraries sync their module folder and assets from moonwell-library.json"
```

---

### Task 4: The files libraries ship join the asset list

**Files:** Modify `cli/src/assets/collect.ts`, `cli/src/assets/plan.ts`, `cli/tests/unit/assets-collect.test.ts`,
`cli/tests/unit/assets-plan.test.ts`.

**Interfaces:**
- Consumes: `LIBRARY_ASSETS_DIR` (Task 3).
- Produces: `Asset.library?: string`; `interface ProjectAssets { assets: Asset[]; replaced: string[] }`;
  `collectProjectAssets(root, config, libraries: readonly string[]): Promise<ProjectAssets>`;
  `planAssets(root, mapDir, stateFile, config, signal?, libraries: readonly string[] = [])`; `AssetPlan.replaced: string[]`.
  A replaced line reads `assets/<source> replaces library <key>'s <path>`.

- [ ] **Step 1: Write the failing tests.** Apply to `cli/tests/unit/assets-collect.test.ts`:

```diff
diff --git a/cli/tests/unit/assets-collect.test.ts b/cli/tests/unit/assets-collect.test.ts
index 2d870f8..ae724a7 100644
--- a/cli/tests/unit/assets-collect.test.ts
+++ b/cli/tests/unit/assets-collect.test.ts
@@ -1,6 +1,6 @@
 import { assertEquals, assertRejects, assertThrows } from "@std/assert";
 import { dirname, join } from "@std/path";
-import { collectAssets } from "../../src/assets/collect.ts";
+import { collectAssets, collectProjectAssets } from "../../src/assets/collect.ts";
 import { assetPath, pathKey, safeJoin, scanFiles, targetPath } from "../../src/assets/paths.ts";
 import { MoonwellError } from "../../src/shared/errors.ts";
 
@@ -126,3 +126,72 @@ Deno.test("safeJoin accepts a root that is itself a link but rejects a link belo
   await Deno.symlink(join(parent, "assets"), join(real, "inner"), { type });
   await assertRejects(() => safeJoin(linkedRoot, "inner/x.blp"), MoonwellError, "Symlinks");
 });
+
+Deno.test("library assets follow the map's own, by key, and import at their path in the library", async () => {
+  const root = await projectRoot();
+  await put(root, "assets/Models/Own.mdx", "own");
+  await put(root, ".moonwell/library-assets/zeta/Sounds/Horn.wav", "horn");
+  await put(root, ".moonwell/library-assets/alpha/war3mapImported/alpha/frames.toc", "toc");
+  await put(root, ".moonwell/library-assets/alpha/.hidden/x.txt", "hidden");
+  await put(root, ".moonwell/library-assets/other/never.txt", "not in the manifest");
+  const { assets, replaced } = await collectProjectAssets(root, defaults, ["zeta", "alpha", "none"]);
+  assertEquals(assets.map((asset) => [asset.library, asset.source, asset.target]), [
+    [undefined, "Models/Own.mdx", "Models/Own.mdx"],
+    ["zeta", "Sounds/Horn.wav", "Sounds/Horn.wav"],
+    ["alpha", "war3mapImported/alpha/frames.toc", "war3mapImported/alpha/frames.toc"],
+  ]);
+  assertEquals(new TextDecoder().decode(assets[1].bytes), "horn");
+  assertEquals(replaced, []);
+  assertEquals(await collectProjectAssets(root, defaults, []), { assets: [assets[0]], replaced: [] });
+});
+
+Deno.test("the map's own asset replaces a library's at the same in-map path, in any letter case", async () => {
+  const root = await projectRoot();
+  await put(root, "assets/custom/golem.blp", "the map's");
+  await put(root, "assets/Models/own.mdx", "the map's model");
+  await put(root, ".moonwell/library-assets/lib/Textures/Golem.blp", "the library's");
+  await put(root, ".moonwell/library-assets/lib/models/Own.mdx", "the library's model");
+  await put(root, ".moonwell/library-assets/lib/Textures/Other.blp", "kept");
+  const config = { paths: { "custom/golem.blp": "textures/golem.blp" }, exclude: [] };
+  const { assets, replaced } = await collectProjectAssets(root, config, ["lib"]);
+  assertEquals(assets.map((asset) => [asset.library, asset.target]), [
+    [undefined, "Models/own.mdx"],
+    [undefined, "textures/golem.blp"],
+    ["lib", "Textures/Other.blp"],
+  ]);
+  assertEquals(replaced, [
+    "assets/custom/golem.blp replaces library lib's Textures/Golem.blp",
+    "assets/Models/own.mdx replaces library lib's models/Own.mdx",
+  ]);
+});
+
+Deno.test("two libraries at one in-map path fail, unless the map's own file replaces both", async () => {
+  const root = await projectRoot();
+  await put(root, ".moonwell/library-assets/a/UI/Frame.fdf", "a");
+  await put(root, ".moonwell/library-assets/b/ui/frame.fdf", "b");
+  const error = await assertRejects(() => collectProjectAssets(root, defaults, ["b", "a"]), MoonwellError);
+  assertEquals(error.message, "Libraries a and b both import ui\\frame.fdf.");
+  assertEquals(error.file, "moonwell.pkl");
+  assertEquals(
+    error.hint,
+    "Drop one of the libraries, or put your own file at that path under assets/ to replace both.",
+  );
+  await put(root, "assets/UI/Frame.fdf", "the map's");
+  const { assets, replaced } = await collectProjectAssets(root, defaults, ["b", "a"]);
+  assertEquals(assets.map((asset) => asset.library), [undefined]);
+  assertEquals(replaced.length, 2);
+});
+
+Deno.test("a library file with a reserved path, or inside another asset file, is refused", async () => {
+  const reserved = await projectRoot();
+  await put(reserved, ".moonwell/library-assets/bad/war3map.lua", "script");
+  const error = await assertRejects(() => collectProjectAssets(reserved, defaults, ["bad"]), MoonwellError);
+  assertEquals(error.message, "Library bad: Reserved map path: war3map.lua");
+  assertEquals(error.file, ".moonwell/library-assets/bad");
+  assertEquals(error.hint, "Report it to the library's author, or use another version of the library.");
+
+  const nested = await projectRoot();
+  await put(nested, "assets/data", "a file");
+  await put(nested, ".moonwell/library-assets/lib/data/inner.txt", "inside it");
+  await assertRejects(() => collectProjectAssets(nested, defaults, ["lib"]), MoonwellError, "file/folder collision");
+});
```

and to `cli/tests/unit/assets-plan.test.ts`:

```diff
diff --git a/cli/tests/unit/assets-plan.test.ts b/cli/tests/unit/assets-plan.test.ts
index 8aab116..3180f21 100644
--- a/cli/tests/unit/assets-plan.test.ts
+++ b/cli/tests/unit/assets-plan.test.ts
@@ -253,6 +253,7 @@ Deno.test("an incomplete rollback names the failure and every file it could not
       { file: join(map, "gone.blp"), before: encode("missing"), after: encode("new") },
     ],
     state: { version: 1 as const, files: {} },
+    replaced: [],
   };
   const error = await assertRejects(() => applyAssetPlan(plan), MoonwellError);
   assert(error.message.includes("changed after the assets were checked"), error.message);
@@ -272,7 +273,7 @@ Deno.test("with no assets and nothing owned, a build never reads war3map.imp", a
   const { root, map, state } = await fixture();
   await Deno.writeFile(join(map, "war3map.imp"), new Uint8Array([9, 9]));
   const plan = await planAssets(root, map, state, defaults);
-  assertEquals(plan, { assets: [], changes: [], state: { version: 1, files: {} } });
+  assertEquals(plan, { assets: [], replaced: [], changes: [], state: { version: 1, files: {} } });
 });
 
 Deno.test("a failed sync restores the files it overwrote byte for byte", async () => {
@@ -296,3 +297,50 @@ Deno.test("a failed sync restores the files it overwrote byte for byte", async (
   assertEquals(await Deno.readFile(join(map, "war3map.imp")), impBefore);
   assertEquals(await Deno.readFile(state), stateBefore);
 });
+
+Deno.test("the files libraries ship are planned like the map's own, and owned by a sync", async () => {
+  const { root, map, state } = await fixture();
+  await put(root, "assets/Models/Own.mdx", "own");
+  await put(root, "assets/icons/shared.blp", "the map's");
+  await put(root, ".moonwell/library-assets/ui/war3mapImported/ui/frames.toc", "toc");
+  await put(root, ".moonwell/library-assets/ui/icons/shared.blp", "the library's");
+  await put(root, ".moonwell/library-assets/unlisted/never.txt", "not in the manifest");
+
+  const plan = await planAssets(root, map, state, defaults, undefined, ["ui"]);
+  assertEquals(plan.assets.map((asset) => [asset.library, asset.source, asset.target]), [
+    [undefined, "icons/shared.blp", "icons/shared.blp"],
+    [undefined, "Models/Own.mdx", "Models/Own.mdx"],
+    ["ui", "war3mapImported/ui/frames.toc", "war3mapImported/ui/frames.toc"],
+  ]);
+  assertEquals(plan.replaced, ["assets/icons/shared.blp replaces library ui's icons/shared.blp"]);
+  await applyAssetPlan(plan, state);
+  assertEquals(await text(map, "war3mapImported/ui/frames.toc"), "toc");
+  assertEquals(await text(map, "icons/shared.blp"), "the map's");
+  assertEquals(
+    (await entries(map)).map((entry) => entry.path).sort(),
+    ["Models\\Own.mdx", "icons\\shared.blp", "war3mapImported\\ui\\frames.toc"],
+  );
+  assertEquals((await planAssets(root, map, state, defaults, undefined, ["ui"])).changes, []);
+
+  // Without the library, the next sync deletes the file it owned.
+  const without = await planAssets(root, map, state, defaults);
+  assertEquals(without.replaced, []);
+  await applyAssetPlan(without, state);
+  assertEquals(await exists(join(map, "war3mapImported", "ui", "frames.toc")), false);
+  assertEquals((await entries(map)).map((entry) => entry.path).sort(), ["Models\\Own.mdx", "icons\\shared.blp"]);
+});
+
+Deno.test("a library file that clashes with the map's own copy names the library", async () => {
+  const { root, map, state } = await fixture();
+  await put(root, ".moonwell/library-assets/ui/Models/Golem.mdx", "library");
+  await put(map, "Models/Golem.mdx", "editor");
+  const error = await assertRejects(
+    () => planAssets(root, map, state, defaults, undefined, ["ui"]),
+    MoonwellError,
+  );
+  assertEquals(
+    error.message,
+    "Asset Models/Golem.mdx of library ui conflicts with a file or import already in the map.",
+  );
+  assertEquals(error.hint, "Remove the map's own copy in World Editor's Import Manager.");
+});
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno test -A cli/tests/unit/assets-collect.test.ts cli/tests/unit/assets-plan.test.ts`
Expected: a type error: `collect.ts` has no export `collectProjectAssets`.

- [ ] **Step 3: Implement.** Apply to `cli/src/assets/collect.ts`:

```diff
diff --git a/cli/src/assets/collect.ts b/cli/src/assets/collect.ts
index 1f0d0e3..f164fa7 100644
--- a/cli/src/assets/collect.ts
+++ b/cli/src/assets/collect.ts
@@ -1,3 +1,4 @@
+import { LIBRARY_ASSETS_DIR } from "../libraries/sync.ts";
 import type { Project } from "../project/project.ts";
 import { MoonwellError } from "../shared/errors.ts";
 import { sha256Hex } from "../shared/fs.ts";
@@ -6,10 +7,12 @@ import { assetPath, pathKey, safeJoin, scanFiles, targetPath } from "./paths.ts"
 /** The manifest's `assets` block: `paths` maps assets/ files to exact in-map paths; `exclude` leaves files out. */
 export type AssetsConfig = Project["assets"];
 
-/** A file under assets/ and the in-map path it is imported as. */
+/** A file under assets/, or one a library ships, and the in-map path it is imported as. */
 export interface Asset {
-  /** Path under assets/, with `/`. */
+  /** Path under assets/, or under the library's assets folder, with `/`. */
   source: string;
+  /** The key of the library that ships the file; `undefined` for the map's own. */
+  library?: string;
   /** In-map path, with `/`. */
   target: string;
   bytes: Uint8Array;
@@ -56,6 +59,12 @@ export async function collectAssets(root: string, config: AssetsConfig): Promise
     const bytes = await Deno.readFile(await safeJoin(folder, source));
     assets.push({ source, target, bytes, hash: await sha256Hex(bytes) });
   }
+  refuseNesting(targets);
+  return assets.sort((a, b) => compare(pathKey(a.target), pathKey(b.target)));
+}
+
+/** Fails when one target would be a folder that another target is a file in. `targets` holds path keys. */
+function refuseNesting(targets: ReadonlySet<string>): void {
   for (const target of targets) {
     const parts = target.split("/");
     while (parts.length > 1) {
@@ -67,5 +76,68 @@ export async function collectAssets(root: string, config: AssetsConfig): Promise
       }
     }
   }
-  return assets.sort((a, b) => compare(pathKey(a.target), pathKey(b.target)));
+}
+
+/** A build's assets, and a line for each library file that one of the map's own files replaces. */
+export interface ProjectAssets {
+  assets: Asset[];
+  replaced: string[];
+}
+
+/**
+ * The map's own assets and then the files its libraries ship (`.moonwell/library-assets/<key>/`, as the last library
+ * sync left them), in key order (spec: library assets §5). A library file's in-map path is its path in that folder.
+ * The map's own file wins over a library's at the same in-map path; two libraries at one path fail. Nothing is
+ * written.
+ */
+export async function collectProjectAssets(
+  root: string,
+  config: AssetsConfig,
+  libraries: readonly string[],
+): Promise<ProjectAssets> {
+  const assets = await collectAssets(root, config);
+  const taken = new Map(assets.map((asset) => [pathKey(asset.target), asset]));
+  const replaced: string[] = [];
+  for (const key of [...libraries].sort()) {
+    const label = `${LIBRARY_ASSETS_DIR}/${key}`;
+    const folder = await safeJoin(root, label);
+    const fromLibrary = (error: unknown): never => {
+      if (!(error instanceof MoonwellError)) throw error;
+      throw new MoonwellError(`Library ${key}: ${error.message}`, {
+        file: label,
+        hint: "Report it to the library's author, or use another version of the library.",
+        cause: error,
+      });
+    };
+    const files = await scanFiles(folder).catch(fromLibrary);
+    for (const [, source] of files) {
+      if (source.split("/").some((part) => part.startsWith("."))) continue;
+      let target: string;
+      try {
+        target = targetPath(source);
+      } catch (error) {
+        target = fromLibrary(error);
+      }
+      const other = taken.get(pathKey(target));
+      if (other !== undefined && other.library === undefined) {
+        replaced.push(`assets/${other.source} replaces library ${key}'s ${source}`);
+        continue;
+      }
+      if (other !== undefined) {
+        throw new MoonwellError(
+          `Libraries ${other.library} and ${key} both import ${target.replaceAll("/", "\\")}.`,
+          {
+            file: "moonwell.pkl",
+            hint: "Drop one of the libraries, or put your own file at that path under assets/ to replace both.",
+          },
+        );
+      }
+      const bytes = await Deno.readFile(await safeJoin(folder, source));
+      const asset: Asset = { source, library: key, target, bytes, hash: await sha256Hex(bytes) };
+      assets.push(asset);
+      taken.set(pathKey(target), asset);
+    }
+  }
+  refuseNesting(new Set(taken.keys()));
+  return { assets: assets.sort((a, b) => compare(pathKey(a.target), pathKey(b.target))), replaced };
 }
```

and to `cli/src/assets/plan.ts`:

```diff
diff --git a/cli/src/assets/plan.ts b/cli/src/assets/plan.ts
index 83be593..254ddd1 100644
--- a/cli/src/assets/plan.ts
+++ b/cli/src/assets/plan.ts
@@ -1,7 +1,7 @@
 import { dirname, join } from "@std/path";
 import { MoonwellError } from "../shared/errors.ts";
 import { removeFileIfExists, sha256Hex } from "../shared/fs.ts";
-import { type Asset, type AssetsConfig, collectAssets } from "./collect.ts";
+import { type Asset, type AssetsConfig, collectProjectAssets } from "./collect.ts";
 import { importPath, readImports, writeImports } from "./imports.ts";
 import { assetPath, lstatOrUndefined, pathKey, safeJoin, scanFiles, targetPath } from "./paths.ts";
 
@@ -20,6 +20,8 @@ export interface FileChange {
 
 export interface AssetPlan {
   assets: Asset[];
+  /** A line for each library file that one of the map's own assets replaces. */
+  replaced: string[];
   changes: FileChange[];
   /** Ownership after applying the plan. */
   state: AssetState;
@@ -78,8 +80,8 @@ async function readState(file: string): Promise<AssetState> {
 const INTERRUPTED_BEFORE_WRITING = "Interrupted; nothing was written.";
 
 /**
- * Checks every asset, import and owned file, and returns the changes. Writes nothing. Stops with an error between files
- * once `signal` aborts (Ctrl+C).
+ * Checks every asset (the map's own and those of `libraries`, by key), import and owned file, and returns the changes.
+ * Writes nothing. Stops with an error between files once `signal` aborts (Ctrl+C).
  */
 export async function planAssets(
   root: string,
@@ -87,11 +89,12 @@ export async function planAssets(
   stateFile: string,
   config: AssetsConfig,
   signal?: AbortSignal,
+  libraries: readonly string[] = [],
 ): Promise<AssetPlan> {
   const stopIfInterrupted = () => {
     if (signal?.aborted) throw new MoonwellError(INTERRUPTED_BEFORE_WRITING);
   };
-  const assets = await collectAssets(root, config);
+  const { assets, replaced } = await collectProjectAssets(root, config, libraries);
   stopIfInterrupted();
   if (!(await lstatOrUndefined(mapDir))?.isDirectory) {
     throw new MoonwellError(`The map folder ${mapDir} does not exist.`, {
@@ -103,7 +106,9 @@ export async function planAssets(
     Object.entries((await readState(stateFile)).files).map(([name, digest]) => [pathKey(name), { name, digest }]),
   );
   // Nothing to import and nothing owned: leave the map (and its war3map.imp) completely alone.
-  if (assets.length === 0 && managed.size === 0) return { assets, changes: [], state: { version: 1, files: {} } };
+  if (assets.length === 0 && managed.size === 0) {
+    return { assets, replaced, changes: [], state: { version: 1, files: {} } };
+  }
   const files = await scanFiles(mapDir);
   const impFile = await safeJoin(mapDir, files.get("war3map.imp") ?? "war3map.imp");
   const impBytes = await readIfExists(impFile);
@@ -142,8 +147,11 @@ export async function planAssets(
     stopIfInterrupted();
     const key = pathKey(asset.target);
     if (!managed.has(key) && (files.has(key) || importKeys.has(key))) {
-      throw new MoonwellError(`Asset ${asset.target} conflicts with a file or import already in the map.`, {
-        hint: "Import it under another path with assets.paths, or remove the map's own copy.",
+      const owner = asset.library === undefined ? "" : ` of library ${asset.library}`;
+      throw new MoonwellError(`Asset ${asset.target}${owner} conflicts with a file or import already in the map.`, {
+        hint: asset.library === undefined
+          ? "Import it under another path with assets.paths, or remove the map's own copy."
+          : "Remove the map's own copy in World Editor's Import Manager.",
       });
     }
     // Reuse the spelling of existing folders, and of folders planned earlier, so letter case stays consistent.
@@ -204,6 +212,7 @@ export async function planAssets(
   }
   return {
     assets,
+    replaced,
     changes,
     state: { version: 1, files: Object.fromEntries(assets.map((asset) => [asset.target, asset.hash])) },
   };
```

- [ ] **Step 4: Run the tests**

Run: `deno task test`
Expected: every unit test passes, among them 11 in `assets-collect.test.ts` and 18 in `assets-plan.test.ts`.

- [ ] **Step 5: Commit**

```bash
git add cli/src/assets/collect.ts cli/src/assets/plan.ts cli/tests/unit/assets-collect.test.ts cli/tests/unit/assets-plan.test.ts
git commit -m "feat: the files libraries ship join the asset list"
```

---

### Task 5: The commands

**Files:** Modify `cli/src/pipeline.ts`, `cli/src/commands/check.ts`, `cli/src/commands/assets.ts`,
`cli/src/commands/assets-paths.ts`, `cli/src/commands/dev.ts`, `cli/tests/unit/dev.test.ts`; create
`cli/tests/pkl/library-assets.test.ts`, `cli/tests/e2e/library-assets.test.ts`.

**Interfaces:**
- Consumes: `planAssets(..., signal, libraries)`, `collectProjectAssets` (Task 4); `syncProjectLibraries` (existing).
- Produces: `libraryKeys(project: Project): string[]` in `pipeline.ts`; `localLibraryFolders` in `dev.ts` becomes
  async and lists each local library's module folder and assets folder.

- [ ] **Step 1: Write the failing tests.** Create `cli/tests/pkl/library-assets.test.ts`:

```ts
import { assertEquals, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { readImports } from "../../src/assets/imports.ts";
import { assetsPaths } from "../../src/commands/assets-paths.ts";
import { assets } from "../../src/commands/assets.ts";
import { init } from "../../src/commands/init.ts";
import { createContext } from "../../src/context.ts";
import { parseGamePaths } from "../../src/models/game-paths.ts";
import { silentLogger } from "../support/logger.ts";
import { chunk, mdx, texture } from "../support/mdx.ts";

const text = (value: string) => new TextEncoder().encode(value);

async function put(root: string, file: string, bytes: Uint8Array): Promise<void> {
  const path = join(root, ...file.split("/"));
  await Deno.mkdir(dirname(path), { recursive: true });
  await Deno.writeFile(path, bytes);
}

/** A fresh project and, beside it, a local library that ships a model, its texture and a frame template list. */
async function projectWithLibrary(): Promise<{ root: string; library: string; manifest: string; plain: string }> {
  const parent = await Deno.makeTempDir({ prefix: "moonwell-libassets-" });
  const root = await init(join(parent, "my-map"), createContext(parent, silentLogger()), { link: true });
  const library = join(parent, "golems");
  await put(library, "moonwell-library.json", text('{"dir":"src","assets":"assets"}'));
  await put(library, "src/golems/spawn.lua", text("return {}\n"));
  await put(library, "assets/Models/Golem.mdx", mdx(chunk("TEXS", texture("Textures\\Golem.blp"))));
  await put(library, "assets/Textures/Golem.blp", new Uint8Array([1]));
  await put(library, "assets/war3mapImported/golems/frames.toc", text("war3mapImported\\golems\\frames.fdf\n"));
  const manifest = join(root, "moonwell.pkl");
  const plain = await Deno.readTextFile(manifest);
  assertStringIncludes(plain, "libraries {\n");
  await Deno.writeTextFile(
    manifest,
    plain.replace("libraries {\n", `libraries {\n  ["golems"] { path = "${library.replaceAll("\\", "/")}" }\n`),
  );
  return { root, library, manifest, plain };
}

Deno.test("assets:check lists a library's files; assets:sync writes them, and removes them with the library", async () => {
  const { root, manifest, plain } = await projectWithLibrary();
  await put(root, "assets/Textures/golem.blp", new Uint8Array([2])); // the map's own, in another letter case
  const logger = silentLogger();
  const ctx = createContext(root, logger);
  const map = join(root, "maps", "map.w3x");

  const checked = await assets(ctx, "check");
  assertEquals(checked.assets.map((asset) => [asset.library, asset.target]), [
    ["golems", "Models/Golem.mdx"],
    [undefined, "Textures/golem.blp"],
    ["golems", "war3mapImported/golems/frames.toc"],
  ]);
  assertEquals(logger.lines.filter((line) => line.includes(" -> ") || line.includes("replaces")), [
    "library golems: Models/Golem.mdx -> Models\\Golem.mdx",
    "Textures/golem.blp -> Textures\\golem.blp",
    "library golems: war3mapImported/golems/frames.toc -> war3mapImported\\golems\\frames.toc",
    "assets/Textures/golem.blp replaces library golems's Textures/Golem.blp",
  ]);
  assertEquals(await exists(join(map, "Models")), false, "check writes nothing into the map");
  assertEquals(await exists(join(root, ".moonwell", "library-assets", "golems", "Models", "Golem.mdx")), true);

  await assets(ctx, "sync");
  assertEquals(await Deno.readFile(join(map, "Textures", "golem.blp")), new Uint8Array([2]));
  assertEquals(
    await Deno.readTextFile(join(map, "war3mapImported", "golems", "frames.toc")),
    "war3mapImported\\golems\\frames.fdf\n",
  );
  const imported = () => Deno.readFile(join(map, "war3map.imp")).then((bytes) => readImports(bytes).map((e) => e.path));
  assertEquals((await imported()).sort(), [
    "Models\\Golem.mdx",
    "Textures\\golem.blp",
    "war3mapImported\\golems\\frames.toc",
  ]);

  await Deno.writeTextFile(manifest, plain);
  await assets(ctx, "sync");
  assertEquals(await exists(join(map, "Models", "Golem.mdx")), false);
  assertEquals(await exists(join(map, "war3mapImported", "golems", "frames.toc")), false);
  assertEquals(await imported(), ["Textures\\golem.blp"]);
  assertEquals(await exists(join(root, ".moonwell", "library-assets", "golems")), false);
});

Deno.test("assets:paths counts a library's files as imported and checks its models", async () => {
  const { root } = await projectWithLibrary();
  await put(root, "assets/Models/Own.mdx", mdx(chunk("TEXS", texture("textures\\golem.BLP"))));
  const logger = silentLogger();
  const ctx = { ...createContext(root, logger), logger };
  const reports = await assetsPaths(ctx, undefined, { gamePaths: parseGamePaths("# test\n") });
  assertEquals(reports.map((report) => [report.heading, report.refs.map((ref) => ref.status)]), [
    ["library golems: Models/Golem.mdx", ["custom path, imported"]],
    ["assets/Models/Own.mdx", ["custom path, imported"]],
  ]);
});
```

Create `cli/tests/e2e/library-assets.test.ts`:

```ts
import { assertEquals, assertStringIncludes } from "@std/assert";
import { fromFileUrl, join } from "@std/path";
import { readImports } from "../../src/assets/imports.ts";
import { openMpq } from "../support/mpq-reader.ts";

const REPO = fromFileUrl(new URL("../../../", import.meta.url));
const MAIN = join(REPO, "cli", "src", "main.ts");

async function deno(args: string[], cwd: string) {
  const output = await new Deno.Command(Deno.execPath(), { args, cwd, stdout: "piped", stderr: "piped" }).output();
  const decoder = new TextDecoder();
  return { code: output.code, text: decoder.decode(output.stdout) + decoder.decode(output.stderr) };
}

async function write(folder: string, files: Record<string, string>) {
  for (const [path, data] of Object.entries(files)) {
    await Deno.mkdir(join(folder, ...path.split("/").slice(0, -1)), { recursive: true });
    await Deno.writeTextFile(join(folder, ...path.split("/")), data);
  }
}

Deno.test("a library's files are imported into the built map, and the map's own file replaces one", async () => {
  const project = join(await Deno.makeTempDir({ prefix: "moonwell-e2e-" }), "my-map");
  const created = await deno(["run", "-A", MAIN, "init", "--link", project], REPO);
  assertEquals(created.code, 0, created.text);
  const library = await Deno.makeTempDir({ prefix: "moonwell-lib-" });
  await write(library, {
    "moonwell-library.json": '{ "dir": "src", "assets": "assets" }\n',
    "src/golems/names.lua": 'return { first = "Granite" }\n',
    "assets/war3mapImported/golems/frames.toc": "toc from the library",
    "assets/Textures/Golem.blp": "texture from the library",
  });
  // The library's file says where its modules and files are: the map names only the path.
  const local = join(project, "moonwell.local.pkl");
  await Deno.writeTextFile(
    local,
    `${await Deno.readTextFile(local)}\nlibraries { ["golems"] { path = "${library.replaceAll("\\", "/")}" } }\n`,
  );
  const main = join(project, "src", "main.yue");
  await Deno.writeTextFile(main, `${await Deno.readTextFile(main)}\nimport "golems.names"\nprint names.first\n`);

  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 0, checked.text);
  assertStringIncludes(checked.text, "2 asset(s).");

  await write(project, { "assets/textures/golem.blp": "texture from the map" });
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  assertStringIncludes(built.text, "assets/textures/golem.blp replaces library golems's Textures/Golem.blp");
  assertStringIncludes(built.text, "Imported 2 asset(s).");

  const staged = join(project, "dist", "stage", "map.w3x");
  assertEquals(
    await Deno.readTextFile(join(staged, "war3mapImported", "golems", "frames.toc")),
    "toc from the library",
  );
  assertEquals(await Deno.readTextFile(join(staged, "textures", "golem.blp")), "texture from the map");
  const imports = readImports(await Deno.readFile(join(staged, "war3map.imp"))).map((entry) => entry.path);
  assertEquals(imports.sort(), ["textures\\golem.blp", "war3mapImported\\golems\\frames.toc"]);

  const archive = openMpq(await Deno.readFile(join(project, "dist", "bin", "map.w3x")));
  const decoder = new TextDecoder();
  assertEquals(decoder.decode(await archive.read("war3mapImported\\golems\\frames.toc")), "toc from the library");
  assertStringIncludes(decoder.decode(await archive.read("war3map.lua")), '__mw.define("golems.names"');
  // The source map is never written by a build.
  const source = join(project, "maps", "map.w3x");
  assertEquals(await Deno.stat(join(source, "war3mapImported")).then(() => true, () => false), false);
});
```

Apply to `cli/tests/unit/dev.test.ts`:

```diff
diff --git a/cli/tests/unit/dev.test.ts b/cli/tests/unit/dev.test.ts
index 3db49d3..8b023e8 100644
--- a/cli/tests/unit/dev.test.ts
+++ b/cli/tests/unit/dev.test.ts
@@ -26,9 +26,9 @@ Deno.test("isRelevantChange watches Yue sources, Lua modules, assets, object fil
   assertEquals(check("README.md"), false);
 });
 
-Deno.test("localLibraryFolders lists the folders of local libraries only", () => {
+Deno.test("localLibraryFolders lists the folders of local libraries only", async () => {
   const root = Deno.cwd();
-  const folders = localLibraryFolders(root, {
+  const folders = await localLibraryFolders(root, {
     libraries: {
       mine: { github: null, tag: null, path: "../mine", dir: "src" },
       remote: { github: "o/r", tag: "v1", path: null, dir: "" },
@@ -37,6 +37,35 @@ Deno.test("localLibraryFolders lists the folders of local libraries only", () =>
   assertEquals(folders, [resolve(root, "..", "mine", "src")]);
 });
 
+Deno.test("localLibraryFolders takes the module and assets folders from a library's own file", async () => {
+  const root = await Deno.makeTempDir({ prefix: "moonwell-dev-" });
+  try {
+    for (const [name, file] of [["described", '{"dir":"src","assets":"art"}'], ["broken", "{"], ["rooted", "{}"]]) {
+      await Deno.mkdir(join(root, name));
+      await Deno.writeTextFile(join(root, name, "moonwell-library.json"), file);
+    }
+    const local = (path: string, dir = "") => ({ github: null, tag: null, path, dir });
+    const folders = await localLibraryFolders(root, {
+      libraries: {
+        a: local("described"),
+        b: local("described", "lua"),
+        c: local("broken"),
+        d: local("rooted"),
+      },
+    } as unknown as Project);
+    assertEquals(folders, [
+      join(root, "described", "src"),
+      join(root, "described", "art"),
+      join(root, "described", "lua"),
+      join(root, "described", "art"),
+      join(root, "broken"),
+      join(root, "rooted"),
+    ]);
+  } finally {
+    await Deno.remove(root, { recursive: true });
+  }
+});
+
 Deno.test("isLibraryChange ignores changes under a local library's dot-folders", () => {
   const folder = join("C:", "lib", "src");
   assertEquals(isLibraryChange(folder, join(folder, "example", "greet.lua")), true);
```

- [ ] **Step 2: Run them to see them fail**

Run: `deno test -A cli/tests/pkl/library-assets.test.ts cli/tests/unit/dev.test.ts`
Expected: the two Pkl-backed tests fail (the library's files are not listed), and the new `dev` test fails (the folders
come back as the manifest's `path` alone).

- [ ] **Step 3: Implement.** Apply to `cli/src/pipeline.ts`:

```diff
diff --git a/cli/src/pipeline.ts b/cli/src/pipeline.ts
index a172586..9ebc64a 100644
--- a/cli/src/pipeline.ts
+++ b/cli/src/pipeline.ts
@@ -43,7 +43,12 @@ export function entryModuleName(entryPath: string): string {
   return posix.slice("src/".length, -".yue".length).split("/").join(".");
 }
 
-/** Brings .moonwell/libraries/ up to date with the manifest's libraries (spec §4.2). */
+/** The keys of the manifest's libraries, whose shipped files join the asset import (spec: library assets §5). */
+export function libraryKeys(project: Project): string[] {
+  return Object.keys(project.libraries);
+}
+
+/** Brings .moonwell/libraries/ and .moonwell/library-assets/ up to date with the manifest's libraries. */
 export function syncProjectLibraries(ctx: CommandContext, project: Project): Promise<void> {
   return syncLibraries(ctx.root, project.libraries, project.manifest, {
     fetch: ctx.install.fetch,
@@ -154,8 +159,9 @@ export async function prepareStage(
   // A build reads the ownership state (to know which source-map files assets:sync owns) but never writes it:
   // applyAssetPlan gets no state file, so only the staged copy changes.
   const { stateFile } = await assetLocations(ctx.root, project.map.folder);
-  const assets = await planAssets(ctx.root, mapDir, stateFile, project.assets);
+  const assets = await planAssets(ctx.root, mapDir, stateFile, project.assets, undefined, libraryKeys(project));
   await applyAssetPlan(assets);
+  for (const line of assets.replaced) ctx.logger.info(line);
   if (assets.assets.length > 0) ctx.logger.info(`Imported ${assets.assets.length} asset(s).`);
 
   const scriptPath = join(mapDir, "war3map.lua");
```

to `cli/src/commands/check.ts`:

```diff
diff --git a/cli/src/commands/check.ts b/cli/src/commands/check.ts
index f4a9a83..4dc29cc 100644
--- a/cli/src/commands/check.ts
+++ b/cli/src/commands/check.ts
@@ -1,11 +1,11 @@
 import { exists } from "@std/fs";
 import { join } from "@std/path";
-import { collectAssets } from "../assets/collect.ts";
+import { collectProjectAssets } from "../assets/collect.ts";
 import { assetLocations, planAssets } from "../assets/plan.ts";
 import type { CommandContext } from "../context.ts";
 import { refreshEditorFiles } from "../editor/refresh.ts";
 import { assertObjectIdsCurrent, refreshObjectIds } from "../objectdata/ids.ts";
-import { compileProject, planProjectObjects } from "../pipeline.ts";
+import { compileProject, libraryKeys, planProjectObjects } from "../pipeline.ts";
 import { loadProject } from "../project/project.ts";
 import { hasSettings } from "../settings/options.ts";
 import { planMapSettings, settingsMapDir } from "../settings/plan.ts";
@@ -38,9 +38,11 @@ export async function check(
       await planMapSettings(settingsSource, project.settings, project.manifest, `maps/${project.map.folder}`);
     }
     const { mapDir, stateFile } = await assetLocations(ctx.root, project.map.folder);
-    const assets = (await exists(mapDir))
-      ? (await planAssets(ctx.root, mapDir, stateFile, project.assets)).assets
-      : await collectAssets(ctx.root, project.assets);
+    // After compileProject, which syncs the libraries: the files they ship are assets too.
+    const { assets, replaced } = (await exists(mapDir))
+      ? await planAssets(ctx.root, mapDir, stateFile, project.assets, undefined, libraryKeys(project))
+      : await collectProjectAssets(ctx.root, project.assets, libraryKeys(project));
+    for (const line of replaced) ctx.logger.info(line);
     ctx.logger.info(`Check passed: ${modules.length} module(s) reachable from ${entry}, ${assets.length} asset(s).`);
     return { modules: modules.length, entry, assets: assets.length };
   });
```

to `cli/src/commands/assets.ts`:

```diff
diff --git a/cli/src/commands/assets.ts b/cli/src/commands/assets.ts
index c0ac98b..f925c95 100644
--- a/cli/src/commands/assets.ts
+++ b/cli/src/commands/assets.ts
@@ -2,14 +2,16 @@ import { exists } from "@std/fs";
 import { join, relative } from "@std/path";
 import { applyAssetPlan, assetLocations, type AssetPlan, planAssets } from "../assets/plan.ts";
 import type { CommandContext } from "../context.ts";
+import { libraryKeys, syncProjectLibraries } from "../pipeline.ts";
 import { loadProject } from "../project/project.ts";
 import { MoonwellError } from "../shared/errors.ts";
 import { toPosix } from "../shared/fs.ts";
 import { withBuildLock } from "../shared/lock.ts";
 
 /**
- * assets:check shows what assets:sync would change; assets:sync writes assets/ into the source map for World Editor,
- * stopping before it writes if `signal` aborts (Ctrl+C) while planning, and undoing its writes if it aborts later.
+ * assets:check shows what assets:sync would change; assets:sync writes assets/ and the files the libraries ship into
+ * the source map for World Editor, stopping before it writes if `signal` aborts (Ctrl+C) while planning, and undoing
+ * its writes if it aborts later. Both sync the libraries first.
  */
 export async function assets(
   ctx: CommandContext,
@@ -28,8 +30,13 @@ export async function assets(
         });
       }
     }
-    const plan = await planAssets(ctx.root, mapDir, stateFile, project.assets, options.signal);
-    for (const asset of plan.assets) ctx.logger.info(`${asset.source} -> ${asset.target.replaceAll("/", "\\")}`);
+    await syncProjectLibraries(ctx, project);
+    const plan = await planAssets(ctx.root, mapDir, stateFile, project.assets, options.signal, libraryKeys(project));
+    for (const asset of plan.assets) {
+      const source = asset.library === undefined ? asset.source : `library ${asset.library}: ${asset.source}`;
+      ctx.logger.info(`${source} -> ${asset.target.replaceAll("/", "\\")}`);
+    }
+    for (const line of plan.replaced) ctx.logger.info(line);
     for (const change of plan.changes) {
       ctx.logger.info(`${change.after === undefined ? "delete" : "write"} ${toPosix(relative(ctx.root, change.file))}`);
     }
```

to `cli/src/commands/assets-paths.ts`:

```diff
diff --git a/cli/src/commands/assets-paths.ts b/cli/src/commands/assets-paths.ts
index 444bd37..dcd2d00 100644
--- a/cli/src/commands/assets-paths.ts
+++ b/cli/src/commands/assets-paths.ts
@@ -1,10 +1,11 @@
 import { exists } from "@std/fs";
 import { isAbsolute, join, relative, resolve } from "@std/path";
-import { collectAssets } from "../assets/collect.ts";
+import { type Asset, collectProjectAssets } from "../assets/collect.ts";
 import { pathKey } from "../assets/paths.ts";
 import type { CommandContext } from "../context.ts";
 import { gamePathKey, loadGamePaths } from "../models/game-paths.ts";
 import { describeModelPath, type ModelPath, modelPaths } from "../models/paths.ts";
+import { libraryKeys, syncProjectLibraries } from "../pipeline.ts";
 import { loadProject } from "../project/project.ts";
 import { MoonwellError } from "../shared/errors.ts";
 import { toPosix } from "../shared/fs.ts";
@@ -44,14 +45,22 @@ function pathStatus(path: string, gamePaths: Set<string>, targets: Set<string> |
   return imported ? "custom path, imported" : "custom path, not imported";
 }
 
-/** Lists the files a model references (one file, or every model under assets/) as in-game or custom paths. */
+/**
+ * Lists the files a model references (one file, or every model under assets/ and among the files the libraries ship)
+ * as in-game or custom paths. In a project it syncs the libraries first: the files they ship count as imported.
+ */
 export async function assetsPaths(
   ctx: CommandContext,
   file?: string,
   options: { gamePaths?: Set<string> } = {},
 ): Promise<ModelReport[]> {
   const inProject = await exists(join(ctx.root, "moonwell.pkl"));
-  const assets = inProject ? await collectAssets(ctx.root, (await loadProject(ctx.root, ctx.run)).assets) : [];
+  let assets: Asset[] = [];
+  if (inProject) {
+    const project = await loadProject(ctx.root, ctx.run);
+    await syncProjectLibraries(ctx, project);
+    assets = (await collectProjectAssets(ctx.root, project.assets, libraryKeys(project))).assets;
+  }
   const targets = inProject ? new Set(assets.map((asset) => pathKey(asset.target))) : undefined;
 
   const models: Array<{ heading: string; bytes: Uint8Array }> = [];
@@ -82,7 +91,11 @@ export async function assetsPaths(
     });
   } else {
     for (const asset of assets) {
-      if (/\.(mdx|mdl)$/i.test(asset.target)) models.push({ heading: `assets/${asset.source}`, bytes: asset.bytes });
+      if (!/\.(mdx|mdl)$/i.test(asset.target)) continue;
+      const heading = asset.library === undefined
+        ? `assets/${asset.source}`
+        : `library ${asset.library}: ${asset.source}`;
+      models.push({ heading, bytes: asset.bytes });
     }
     if (models.length === 0) {
       ctx.logger.info("No models under assets/.");
```

and to `cli/src/commands/dev.ts`:

```diff
diff --git a/cli/src/commands/dev.ts b/cli/src/commands/dev.ts
index 2264e05..f4731ff 100644
--- a/cli/src/commands/dev.ts
+++ b/cli/src/commands/dev.ts
@@ -1,6 +1,7 @@
 import { exists } from "@std/fs";
-import { join, relative, resolve } from "@std/path";
+import { basename, join, relative, resolve } from "@std/path";
 import type { CommandContext } from "../context.ts";
+import { LIBRARY_FILE, type LibraryFile, parseLibraryFile } from "../libraries/manifest.ts";
 import { loadProject, type Project } from "../project/project.ts";
 import { formatError, MoonwellError } from "../shared/errors.ts";
 import { isWithin, toPosix } from "../shared/fs.ts";
@@ -22,18 +23,43 @@ export function isLibraryChange(folder: string, path: string): boolean {
   return !toPosix(relative(folder, path)).split("/").some((segment) => segment.startsWith("."));
 }
 
-/** The folders local libraries are copied from (`path` and `dir`, resolved against the project), in key order. */
-export function localLibraryFolders(root: string, project: Project): string[] {
-  return localLibraries(root, project).map(({ folder }) => folder);
+/**
+ * The folders local libraries are copied from, resolved against the project, in key order: each library's module
+ * folder and, when its moonwell-library.json names one, its assets folder.
+ */
+export async function localLibraryFolders(root: string, project: Project): Promise<string[]> {
+  return (await localLibraries(root, project)).flatMap(({ folders }) => folders.map(({ folder }) => folder));
 }
 
-/** Each local library's folder, and its label: the folder as the manifest writes it (`path`, then `dir`). */
-function localLibraries(root: string, project: Project): { folder: string; label: string }[] {
-  return Object.keys(project.libraries).sort().flatMap((key) => {
+/**
+ * Each local library's root, and its folders with their labels: a folder as the manifest and the library's file write
+ * it (`path`, then `dir` or `assets`). A library file that cannot be read counts as none: the cycle reports it.
+ */
+async function localLibraries(
+  root: string,
+  project: Project,
+): Promise<{ base: string; folders: { folder: string; label: string }[] }[]> {
+  const libraries = [];
+  for (const key of Object.keys(project.libraries).sort()) {
     const { path, dir } = project.libraries[key];
-    if (path === null) return [];
-    return [{ folder: resolve(root, path, dir), label: toPosix(join(path, dir)).replace(/\/?$/, "/") }];
-  });
+    if (path === null) continue;
+    const base = resolve(root, path);
+    let described: LibraryFile = { dir: null, assets: null };
+    try {
+      described = parseLibraryFile(key, await Deno.readFile(join(base, LIBRARY_FILE)), LIBRARY_FILE);
+    } catch {
+      // No file, or one the next cycle's sync reports.
+    }
+    const named = [dir !== "" ? dir : described.dir ?? "", ...(described.assets === null ? [] : [described.assets])];
+    libraries.push({
+      base,
+      folders: named.map((folder) => ({
+        folder: resolve(base, folder),
+        label: toPosix(join(path, folder)).replace(/\/?$/, "/"),
+      })),
+    });
+  }
+  return libraries;
 }
 
 /**
@@ -62,7 +88,7 @@ export async function dev(
 
   // A manifest that does not load has no libraries to watch; the first cycle has already reported why.
   const project = await loadProject(ctx.root, ctx.run).catch(() => undefined);
-  const libraries = project === undefined ? [] : localLibraries(ctx.root, project);
+  const libraries = project === undefined ? [] : await localLibraries(ctx.root, project);
 
   // Start watching before announcing it, so a save made right after the message is never missed.
   const projectChange = (path: string) => isRelevantChange(ctx.root, path);
@@ -77,17 +103,24 @@ export async function dev(
     watchers.push({ watcher: Deno.watchFs(join(ctx.root, folder), { recursive: true }), relevant: projectChange });
     watched.push(`${folder}/`);
   }
-  // A local library's modules are copied into .moonwell/libraries/ by each cycle, so a change in its folder counts,
-  // except under dot-folders such as .git/, which sync skips. A folder that holds this project's .moonwell/ is skipped:
-  // sync refuses it, and watching it would loop.
-  for (const { folder, label } of libraries) {
-    if (isWithin(join(ctx.root, ".moonwell"), folder)) continue;
-    if (!(await exists(folder, { isDirectory: true }))) continue;
+  // A local library's modules and assets are copied into .moonwell/ by each cycle, so a change in its folders counts,
+  // except under dot-folders such as .git/, which sync skips; and so does a change of its moonwell-library.json. A
+  // folder that holds this project's .moonwell/ is skipped: sync refuses it, and watching it would loop.
+  for (const { base, folders } of libraries) {
+    for (const { folder, label } of folders) {
+      if (isWithin(join(ctx.root, ".moonwell"), folder)) continue;
+      if (!(await exists(folder, { isDirectory: true }))) continue;
+      watchers.push({
+        watcher: Deno.watchFs(folder, { recursive: true }),
+        relevant: (path) => isLibraryChange(folder, path),
+      });
+      watched.push(label);
+    }
+    if (isWithin(join(ctx.root, ".moonwell"), base) || !(await exists(base, { isDirectory: true }))) continue;
     watchers.push({
-      watcher: Deno.watchFs(folder, { recursive: true }),
-      relevant: (path) => isLibraryChange(folder, path),
+      watcher: Deno.watchFs(base, { recursive: false }),
+      relevant: (path) => basename(path) === LIBRARY_FILE,
     });
-    watched.push(label);
   }
   const closeWatchers = () => {
     for (const { watcher } of watchers) {
```

- [ ] **Step 4: Run the tests**

Run, each as its own command: `deno task check`, `deno task lint`, `deno fmt --check`, `deno task test`,
`deno test -A cli/tests/pkl/library-assets.test.ts`, `deno test -A cli/tests/e2e/library-assets.test.ts`.
Expected: all pass; `ok | 466 passed | 0 failed` for the unit tests, 2 passed and 1 passed for the two new files.

- [ ] **Step 5: Commit**

```bash
git add cli/src/pipeline.ts cli/src/commands/check.ts cli/src/commands/assets.ts cli/src/commands/assets-paths.ts cli/src/commands/dev.ts cli/tests/unit/dev.test.ts cli/tests/pkl/library-assets.test.ts cli/tests/e2e/library-assets.test.ts
git commit -m "feat: commands import, list and watch the files libraries ship"
```

---

### Task 6: The schema's words, the template's example and the network test

**Files:** Modify `schema/Project.pkl`, `template/moonwell.pkl`, `cli/src/embedded/template.ts` (generated); replace
`cli/tests/network/libraries.test.ts`.

The example library's `v0.2.0` tag is already on GitHub (`mdlsvensson/moonwell-example-lib`, commit
`0b69cfadeac0ca69d249df68411b5edb82f4f2a8`): a `moonwell-library.json` with `dir` `src` and `assets` `assets`, and one
file, `assets/war3mapImported/example/hello.txt`. Its tags are never moved.

- [ ] **Step 1: The network test.** Replace `cli/tests/network/libraries.test.ts` with:

```ts
import { assert, assertEquals } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { readLock } from "../../src/libraries/lock.ts";
import { syncLibraries } from "../../src/libraries/sync.ts";
import { silentLogger } from "../support/logger.ts";

const enabled = Deno.env.get("MOONWELL_NETWORK_TESTS") === "1";
const EXAMPLE = { github: "mdlsvensson/moonwell-example-lib", tag: "v0.1.0", path: null, dir: "src" };
/** Since v0.2.0 the library has a moonwell-library.json: it names its module folder and ships one file. */
const SHIPPING = { github: "mdlsvensson/moonwell-example-lib", tag: "v0.2.0", path: null, dir: "" };
/** The hash of the three modules under src/, which both tags have. */
const MODULES = "sha256:b2a02000abc725476fcc6a72806632851fff48bc26179d2169b27c1ecc3b88c3";

Deno.test({
  name: "the example library's v0.1.0 tag downloads and locks commit 13e3553, as it always has",
  ignore: !enabled,
  async fn() {
    const root = await Deno.makeTempDir({ prefix: "moonwell-network-" });
    try {
      const requests: string[] = [];
      const deps = { fetch: (url: string) => (requests.push(url), fetch(url)), logger: silentLogger() };
      await syncLibraries(root, { example: EXAMPLE }, "moonwell.pkl", deps);
      assertEquals((await readLock(root)).example, {
        github: "mdlsvensson/moonwell-example-lib",
        tag: "v0.1.0",
        dir: "src",
        commit: "13e35535c481fddd267533cc513f86b55b313b66",
        files: MODULES,
      });
      for (const file of ["greet.lua", "loud.yue", "globals.lua"]) {
        assert(await exists(join(root, ".moonwell", "libraries", "example", "example", file)), file);
      }
      assertEquals(await exists(join(root, ".moonwell", "library-assets", "example")), false);
      await syncLibraries(root, { example: EXAMPLE }, "moonwell.pkl", deps);
      assertEquals(requests.length, 1);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  },
});

Deno.test({
  name: "the example library's v0.2.0 tag names its own module folder and ships a file, locked by its hash",
  ignore: !enabled,
  async fn() {
    const root = await Deno.makeTempDir({ prefix: "moonwell-network-" });
    try {
      const requests: string[] = [];
      const deps = { fetch: (url: string) => (requests.push(url), fetch(url)), logger: silentLogger() };
      await syncLibraries(root, { example: SHIPPING }, "moonwell.pkl", deps);
      assertEquals((await readLock(root)).example, {
        github: "mdlsvensson/moonwell-example-lib",
        tag: "v0.2.0",
        dir: "",
        commit: "0b69cfadeac0ca69d249df68411b5edb82f4f2a8",
        files: MODULES,
        assets: "sha256:d40d3370a1e0e14f411273c8a5051158371a1e798f58b23e6b424fbb1f27eadb",
      });
      for (const file of ["greet.lua", "loud.yue", "globals.lua"]) {
        assert(await exists(join(root, ".moonwell", "libraries", "example", "example", file)), file);
      }
      assertEquals(
        await Deno.readTextFile(
          join(root, ".moonwell", "library-assets", "example", "war3mapImported", "example", "hello.txt"),
        ),
        "Hello from moonwell-example-lib.\n",
      );
      await syncLibraries(root, { example: SHIPPING }, "moonwell.pkl", deps);
      assertEquals(requests.length, 1);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  },
});
```

- [ ] **Step 2: Run it**

Run: `MOONWELL_NETWORK_TESTS=1 deno task test:network`
Expected: `ok | 2 passed | 0 failed`. (The code is already in place from Tasks 1 to 3; this test is the check that a
real download behaves as the unit tests' archives do, and that `v0.1.0` locks to the entry it always had.)

- [ ] **Step 3: The schema's documentation.** Apply to `schema/Project.pkl`:

```diff
diff --git a/schema/Project.pkl b/schema/Project.pkl
index f6025c9..8b25186 100644
--- a/schema/Project.pkl
+++ b/schema/Project.pkl
@@ -98,7 +98,9 @@ class LintConfig {
   globals: List<String(isLuaName(this))> = List()
 }
 
-/// A library of YueScript and Lua modules: a GitHub tag, or a local folder (spec: Lua modules and libraries).
+/// A library of YueScript and Lua modules, and of the files it ships for the map: a GitHub tag, or a local folder
+/// (specs: Lua modules and libraries; library assets). A library may describe its layout in a moonwell-library.json
+/// at its root: `dir`, the folder its module names start from, and `assets`, the folder whose files the map imports.
 class Library {
   /// A GitHub repository, "owner/repo". With `tag`, Moonwell downloads that tag; moonwell.lock records its commit.
   github: String(matches(Regex(#"[A-Za-z0-9-]+/[A-Za-z0-9._-]+"#)))?
@@ -106,11 +108,12 @@ class Library {
   /// The tag to download.
   tag: String(!isEmpty)?
 
-  /// A local folder instead, relative to the project or absolute. When set, `github` and `tag` are ignored: set it in
-  /// moonwell.local.pkl to work on a library next to the map.
+  /// A local folder instead, the library's root, relative to the project or absolute. When set, `github` and `tag` are
+  /// ignored: set it in moonwell.local.pkl to work on a library next to the map.
   path: String(!isEmpty)?
 
-  /// The folder inside the library that module names start from, such as "src". Empty for the library's root.
+  /// The folder inside the library that module names start from, such as "src". Leave it empty for a library that
+  /// names its own in moonwell-library.json; without that file, empty is the library's root.
   dir: String(isEmpty || isRelativeFolder(this)) = ""
 }
 
```

- [ ] **Step 4: The template.** Apply to `template/moonwell.pkl`:

```diff
diff --git a/template/moonwell.pkl b/template/moonwell.pkl
index 1024839..7cefdb0 100644
--- a/template/moonwell.pkl
+++ b/template/moonwell.pkl
@@ -35,10 +35,10 @@ lint {
   globals = List()          // extra global names to allow, e.g. List("MyLibrary")
 }
 
-// Libraries: modules from a GitHub tag or a local folder, named by their path in the library. moonwell.lock records
-// each tag's commit; commit it. See "Libraries" in Moonwell's README.
+// Libraries: modules, and files for the map, from a GitHub tag or a local folder. Modules are named by their path in
+// the library. moonwell.lock records each tag's commit; commit it. See "Libraries" in Moonwell's README.
 libraries {
-  // ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.1.0"; dir = "src" }
+  // ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.2.0" }
 }
 
 // Map settings, applied to the built map only; the World Editor map is never changed. `deno task settings:check`
```

Then run `deno task gen` (it rewrites `cli/src/embedded/template.ts`).

- [ ] **Step 5: Run the checks that read them**

Run: `deno task test`, `deno task test:pkl`.
Expected: both pass (the embedded template is fresh; the schema still evaluates).

- [ ] **Step 6: Commit**

```bash
git add schema/Project.pkl template/moonwell.pkl cli/src/embedded/template.ts cli/tests/network/libraries.test.ts
git commit -m "test: the example library's v0.2.0 over the network; the Library schema's words"
```

---

### Task 7: Documentation

**Files:** Modify `README.md`, `CHANGELOG.md`.

- [ ] **Step 1: The README.** Apply:

````diff
diff --git a/README.md b/README.md
index 2943445..bdb56b5 100644
--- a/README.md
+++ b/README.md
@@ -186,38 +186,52 @@ through `.luarc.json`'s `runtime.path`; `deno task setup` adds the entries to a
 
 ## Libraries
 
-A library is a folder of YueScript and Lua modules from a GitHub tag or a local folder. List libraries in
-`moonwell.pkl`:
+A library is a folder of YueScript and Lua modules, and of files for the map, from a GitHub tag or a local folder. List
+libraries in `moonwell.pkl`:
 
 ```pkl
 libraries {
-  ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.1.0"; dir = "src" }
+  ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.2.0" }
 }
 ```
 
-The key names the library's folder in `.moonwell/libraries/`, so keys must differ by more than case. `dir` is the folder
-inside the library that module names start from; leave it out for the library's root. A library's modules keep their own
-names (`import "example.loud"`), and share one set of names with `src/` and `lua/`: a name two of them define fails the
-build. A `.lua` file next to a `.yue` file of the same name in a library is its compiled output, and is skipped.
+The key names the library's folder in `.moonwell/libraries/`, so keys must differ by more than case. A library's modules
+keep their own names (`import "example.loud"`), and share one set of names with `src/` and `lua/`: a name two of them
+define fails the build. A `.lua` file next to a `.yue` file of the same name in a library is its compiled output, and is
+skipped.
+
+A library describes its own layout in a `moonwell-library.json` at its root:
+
+```json
+{ "dir": "src", "assets": "assets" }
+```
+
+- `dir` is the folder module names start from. A library without the file needs it in the manifest instead
+  (`["old"] { github = "owner/repo"; tag = "v1.0.0"; dir = "src" }`), and a `dir` in the manifest always wins. With
+  neither, module names start at the library's root.
+- `assets` is a folder of files the map imports, each at its path in that folder: see [Assets](#assets).
+
+Both are optional. Any other key fails, naming the library: it may be written for a newer Moonwell.
 
 `check`, `build`, `test`, `dev` and `setup` download a library that is missing or whose `github`, `tag` or `dir` changed
-into `.moonwell/libraries/<key>/` (git-ignored), and record the tag's commit in `moonwell.lock`. Commit `moonwell.lock`:
-a fresh clone then gets the same code, and if a tag is moved on GitHub, the command fails instead of using the new code.
-To upgrade, change `tag`.
+into `.moonwell/libraries/<key>/`, and its files for the map into `.moonwell/library-assets/<key>/` (both git-ignored),
+and record the tag's commit in `moonwell.lock`. Commit `moonwell.lock`: a fresh clone then gets the same code, and if a
+tag is moved on GitHub, the command fails instead of using the new code. To upgrade, change `tag`.
 
-To work on a library next to your map, point it at a local folder in `moonwell.local.pkl`:
+To work on a library next to your map, point it at a local folder, the library's root, in `moonwell.local.pkl`:
 
 ```pkl
-libraries { ["example"] { path = "../moonwell-example-lib"; dir = "src" } }
+libraries { ["example"] { path = "../moonwell-example-lib" } }
 ```
 
-`path` wins over `github`. Its `.yue` and `.lua` files are copied into `.moonwell/libraries/<key>/` (folders whose name
-starts with `.`, such as `.git/`, are skipped), so errors in them name the copy there, not your checkout. `dev` watches
-the folder, but picks the folders to watch when it starts: restart it after adding a local library. A local library
-keeps its entry in `moonwell.lock`, so switching back to the tag still checks it. Library code is not checked for
-unknown globals, but the globals a required library module defines count as known. `check`, `build`, `test` and `dev`
-write every library module to `.moonwell/lua/` as Lua (a YueScript module compiled), where the editor finds it; `setup`,
-which does not compile, writes the Lua modules and leaves the YueScript ones as the last compile wrote them.
+`path` wins over `github`. Its `.yue` and `.lua` files, and the files of its assets folder, are copied into `.moonwell/`
+(folders whose name starts with `.`, such as `.git/`, are skipped), so errors in them name the copy there, not your
+checkout. `dev` watches those folders, but picks the folders to watch when it starts: restart it after adding a local
+library. A local library keeps its entry in `moonwell.lock`, so switching back to the tag still checks it. Library code
+is not checked for unknown globals, but the globals a required library module defines count as known. `check`, `build`,
+`test` and `dev` write every library module to `.moonwell/lua/` as Lua (a YueScript module compiled), where the editor
+finds it; `setup`, which does not compile, writes the Lua modules and leaves the YueScript ones as the last compile
+wrote them.
 
 ## Assets
 
@@ -232,6 +246,17 @@ assets {
 }
 ```
 
+The files a library ships (the `assets` folder of its `moonwell-library.json`, see [Libraries](#libraries)) are imported
+too, each at its path in that folder; `paths` and `exclude` apply to your own files only. When one of your files and a
+library's have the same in-map path, yours is imported and the command says so:
+`assets/Textures/Golem.blp replaces library golems's Textures/Golem.blp`. That is how you swap a library's icon or
+model. Two libraries with a file at the same path fail the build. `assets:check` lists a library's files as
+`library <key>: <file>`, `assets:sync` writes them into the source map with your own, and `assets:paths` counts them as
+imported and checks a library's models too.
+
+If you write a library, keep its files under a folder of its own, such as `assets/war3mapImported/<library>/`, so they
+clash with no map's and no other library's.
+
 Builds import assets into the staged copy only. To see them in World Editor, close the map there and run
 `deno task assets:sync`. It writes the files and `war3map.imp` into `maps/<folder>`, and records what it owns in
 `.asset-state/`. It never overwrites or deletes a file it does not own, and it refuses to touch an owned file you edited
````

- [ ] **Step 2: The changelog.** Apply:

```diff
diff --git a/CHANGELOG.md b/CHANGELOG.md
index 6849b9f..1b1deac 100644
--- a/CHANGELOG.md
+++ b/CHANGELOG.md
@@ -2,6 +2,18 @@
 
 ## Unreleased
 
+- **Libraries can ship files for the map.** A library names a folder of them in a `moonwell-library.json` at its root
+  (`{ "dir": "src", "assets": "assets" }`), and every map that lists the library imports those files, each at its path
+  in that folder, with the map's own assets: in builds, `check`, `assets:check`, `assets:sync` and `assets:paths`. The
+  map's own file wins over a library's at the same in-map path, and the command says so; two libraries at one path fail.
+  The same file can name the library's module folder (`dir`), so a map may leave `dir` out of its manifest; a `dir` in
+  the manifest still wins.
+- `moonwell.lock` records an `assets` hash for a library that ships files. Existing lock files stay valid. The first run
+  after upgrading downloads each GitHub library once more.
+- `assets:check`, `assets:sync` and `assets:paths` sync the libraries first, as `check` and `setup` do.
+- A local library's `path` is the library's root: its `moonwell-library.json` is read from there, and `dev` watches its
+  assets folder and that file.
+- The template's commented library example names `mdlsvensson/moonwell-example-lib` `v0.2.0`, which has such a file.
 - Documentation: the `moonwell-wrappers` v0.5.1 review, the performance and port-needs research notes with their probe
   results, and the designs and implementation plans for `moonwell-wrappers` v0.6.0 (the refactor after the review) and
   v0.7.0 (the port prerequisites: damage events, sync, collision size and pathing), both released 2026-09-30.
```

- [ ] **Step 3: Format and check**

Run: `deno fmt --check`
Expected: clean. (`deno fmt` reflows Markdown: if it changes the README, read the result before committing.)

- [ ] **Step 4: Commit**

```bash
git add README.md CHANGELOG.md
git commit -m "docs: libraries can ship files for the map"
```

---

### Task 8: Release 0.6.0

**Files:** Modify `cli/deno.json`, `cli/src/version.ts`, `schema/PklProject`, `template/PklProject.deps.json`,
`cli/src/embedded/template.ts` (generated), `CHANGELOG.md`, `AGENTS.md`, the roadmap.

- [ ] **Step 1: Every check**, each as its own command: `deno task check`, `deno task lint`, `deno fmt --check`,
`deno task test`, `deno task test:runtime`, `deno task test:pkl`, `deno task test:e2e`,
`MOONWELL_NETWORK_TESTS=1 deno task test:network`. Expected: all pass (466 unit, 30 runtime, the Pkl tests and 27
Pkl-backed, 33 end-to-end, 2 network).

- [ ] **Step 2: The version.** Set `0.6.0` in `cli/deno.json` (`version`), `cli/src/version.ts` (`VERSION`) and
`schema/PklProject` (`package.version`). In `template/`, run `pkl project resolve`, then from the root `deno task gen`.
Run `deno task test` (the versions-consistent and embedded-template tests) and `deno task test:pkl`.

- [ ] **Step 3: The records.** In `CHANGELOG.md`, rename `## Unreleased` to `## 0.6.0 (<date>)`, keep the documentation
bullets about the wrappers and systems libraries under it, and add a `### Release gate` section: the checks of Step 1
with their counts, the 34 mutations, that gate steps 1 and 2 passed and steps 3 to 12 were not re-run (the change decides
which files enter an import path those steps already covered; the end-to-end test checks the staged map's bytes), and
the example library's `v0.2.0`. In `AGENTS.md`, add a state bullet for 0.6.0 (what it adds, `moonwell-library.json`,
`.moonwell/library-assets/`, the lock's `assets`, the stamp's layout forcing one download, the example library's tag)
and update "Next work": phase 4 item 2 is done, item 3 (the custom map preview) is next; note that the wrappers and
systems libraries can add `{"dir": "src"}` in their next releases. In the roadmap, mark phase 4 item 2 released.

- [ ] **Step 4: Commit and push**

```bash
git add cli/deno.json cli/src/version.ts schema/PklProject template/PklProject.deps.json cli/src/embedded/template.ts CHANGELOG.md AGENTS.md docs/superpowers/plans/2026-09-30-moonwell-roadmap-to-wc3-lib.md
git commit -m "release: 0.6.0"
git push origin main
```

Then check CI: `"C:\Program Files\GitHub CLI\gh.exe" run list --limit 1`, and wait for it to pass on Ubuntu and Windows.

- [ ] **Step 5: The Pkl package and the GitHub release** (CONTRIBUTING, Publishing, steps 2 and 3):
`pkl project package schema/` (with `--skip-publish-check` if the publish check crashes in this shell), then create the
GitHub release `moonwell@0.6.0` on the pushed commit with `moonwell@0.6.0.zip` and the metadata file `moonwell@0.6.0`
attached, and confirm the tag names the release commit.

- [ ] **Step 6: The maintainer publishes to JSR** and checks it. Give exactly this:

> In `C:\Users\mdlsvensson\Repo\moonwell\cli`: `deno publish`. Then, outside the repository:
> `deno run -A --min-dep-age=0 jsr:@moonwell/cli@0.6.0 init my-map`, and in `my-map`
> `deno run -A --min-dep-age=0 jsr:@moonwell/cli@0.6.0 build`. Say whether both worked.

- [ ] **Step 7: Record the published check** in `CHANGELOG.md` and `AGENTS.md` ("checked with `init` and `build` from
JSR"), commit `docs: record the 0.6.0 publication`, push, and check CI.
