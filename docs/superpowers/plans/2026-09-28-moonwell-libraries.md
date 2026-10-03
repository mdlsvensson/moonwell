# Moonwell Libraries (Plan 4b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A `libraries` block in `moonwell.pkl` fetches YueScript and Lua modules from GitHub tags (pinned by commit in
`moonwell.lock`) or copies them from local folders into `.moonwell/libraries/`, and the pipeline, the unknown-global
check and the editor treat them like the project's own modules.

**Architecture:** Compile outputs are keyed by project-relative path, so library `.yue` files compile next to `src/`.
`syncLibraries` brings `.moonwell/libraries/<key>/` up to date at the start of every compile (and in `setup`), reading
GitHub tag archives with the existing unzip code and a new lock file. `collectModules` gets one module root per library
and kind; the resolved module graph decides which modules' globals are known names. After compiling, `.moonwell/lua/`
holds every library module as Lua for lua-language-server.

**Tech Stack:** Deno 2.9+, TypeScript, existing `jsr:@std/*` imports, Pkl 0.32, YueScript 0.34.2.

**Spec:** `docs/superpowers/specs/2026-09-28-moonwell-lua-libraries-design.md` (approved 2026-09-28). This plan covers
§4, §5, the Plan 4b rows of §§6–11, and one amendment (below). Plan 4a (Lua modules) is done; 0.5.0 is released after
this plan.

## Global Constraints

- No Node.js: no `package.json`, `node_modules`, `npm:` or `node:` specifiers. Only `jsr:@std/*` from the existing
  import maps.
- File-system code is async. Expected failures throw `MoonwellError` (`cli/src/shared/errors.ts`) with `file` and
  `hint`; anything else is reported as an internal error, so user mistakes (and network failures) must never reach it.
- `deno fmt` width 120 (`template/`, `docs/`, `cli/src/embedded/` excluded); `deno fmt --check` and `deno lint` clean.
- After changing `template/`, run `deno task gen`. Nothing stray in `template/`: every file there is embedded.
- Write files containing backslashes with a file-editing tool, never a shell heredoc or `sed`.
- Unit tests never touch the network: downloads go through an injected `fetch`. Only `cli/tests/network/` does, and only
  when `MOONWELL_NETWORK_TESTS=1`.
- Paths: `.moonwell/libraries/<key>/` (library files), `.moonwell/lua/` (editor view), `moonwell.lock` (project root,
  committed), `dist/stage/lua/.libraries/<key>/` (compiled library YueScript).
- Every task: failing test first, then code, then the full gate (below), then one commit on `main`.
  Do not push; the maintainer pushes. Do not bump versions.

**Full gate (before every commit):**

```bash
deno task check && deno task lint && deno fmt --check && deno task test && deno task test:runtime && deno task test:pkl && deno task test:e2e
```

## Facts this plan relies on (checked 2026-09-28)

- The test library [mdlsvensson/moonwell-example-lib](https://github.com/mdlsvensson/moonwell-example-lib), tag
  `v0.1.0` (lightweight) on commit `c07126f080c3887ba667596d08aa21df3b3a20f7`, holds `src/example/greet.lua` (a module
  table with `hello(name)`), `src/example/loud.yue` (`import "example.greet"`, `export shout`) and
  `src/example/globals.lua` (globals `ExampleVersion` and `ExampleAdd`), plus `README.md` and `LICENSE`.
- Its archive from `https://codeload.github.com/mdlsvensson/moonwell-example-lib/zip/refs/tags/v0.1.0` has the single
  top folder `moonwell-example-lib-0.1.0/` (GitHub drops the tag's leading `v`, so the name cannot be predicted: strip
  whatever single top folder there is), and its zip comment is the commit SHA. An annotated tag also gives the commit.
- `extractZip` (`cli/src/yue/unzip.ts`) returns file entries only (directories skipped); `makeZip`
  (`cli/tests/support/zip.ts`) builds test archives but writes no comment.
- `ctx.install.fetch` (`InstallDeps.fetch`, `cli/src/yue/install.ts`) is the injectable `fetch` the yue download uses.

## Spec amendment (maintainer, 2026-09-28: "do as you think is best")

**Known names come from reachable modules only.** Spec §3.4/§5.3 made every `.lua` module's top-level globals (and
every `src/` file's `global` lines) known names. Then a file nothing requires still "defines" its globals, so
`CountUp!` without `require "counter"` passes `check` and fails silently in the game. From this plan on, `global` lines
and Lua top-level globals count only for modules in the module graph resolved from the entry (project and library
modules alike). Task 2 implements it and Task 9 updates the spec, README and CHANGELOG (a behaviour change for `src/`
files that declare globals but are never imported).

## Decisions this plan adds to the spec

- **Libraries sync at the start of `compileProject`** (so `check`, `build`, `test` and `dev` all do it) and in `setup`,
  not "before planning objects" (spec §4.2): objects do not depend on libraries, and one call site is simpler.
- **Compile outputs are keyed by project-relative path** (`src/x.yue` → `dist/stage/lua/x.lua`,
  `.moonwell/libraries/<key>/x.yue` → `dist/stage/lua/.libraries/<key>/x.lua`); `CompileOutput.hashes`/`sources` stay
  keyed under `src/` for the unknown-global check, which still reads only `src/`.
- **`moonwell.lock` is deleted when no GitHub library remains**, like `.asset-state/` files.
- **A library's own compiled `.lua` next to its `.yue`** clashes like any other duplicate name; the clash hint says to
  rename one or narrow the library's `dir`.
- **Module roots per library:** each key gets a YueScript and a Lua root over `.moonwell/libraries/<key>`, after the
  project's roots, in key order.

## File Structure

| File | Responsibility |
| --- | --- |
| `cli/src/yue/compile.ts` | outputs keyed by project path: `outputPathOf`, `CompileOutput.texts`, `loadModule` |
| `cli/src/bundle/modules.ts` | `ModuleRoot.library`, `SourceModule.library`, `libraryModuleRoots`, clash hint; `moduleLoader` takes a `SourceModule` loader |
| `cli/src/lint/unknown-globals.ts`, `cli/src/pipeline.ts` | known names from the resolved graph (`declared`) |
| `schema/Project.pkl`, `cli/src/project/project.ts`, `template/moonwell.pkl` | `libraries` |
| `cli/src/yue/unzip.ts` | `zipComment` |
| `cli/src/libraries/archive.ts` (new) | `readGitHubArchive`, `filesHash` |
| `cli/src/libraries/lock.ts` (new) | `LockEntry`, `readLock`, `writeLock` |
| `cli/src/libraries/sync.ts` (new) | `LIBRARIES_DIR`, `archiveUrl`, `syncLibraries` |
| `cli/src/editor/library-view.ts` (new) | `LIBRARY_VIEW_DIR`, `refreshLibraryView` |
| `cli/src/commands/setup.ts`, `cli/src/commands/dev.ts`, `template/.luarc.json` | sync and view in setup; dev watches local libraries; `.moonwell/lua` in the editor |
| Tests | unit: `modules`, `lint-unknown-globals`, `project`, `unzip`/new `library-archive`, `library-lock`, `library-sync`, `library-view`, `editor-scaffold`; runtime: `compile`; Pkl: `schema/tests/Project.pkl`; e2e: `libraries.test.ts`, `lint.test.ts`; network: `cli/tests/network/libraries.test.ts` |
| CI and docs | `.github/workflows/ci.yml`, `deno.json`, `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`, the spec |

---

### Task 1: Compile outputs keyed by project path; library module roots

**Files:**
- Modify: `cli/src/yue/compile.ts`, `cli/src/bundle/modules.ts`, `cli/src/pipeline.ts`
- Test: `cli/tests/yue/compile.test.ts`, `cli/tests/unit/modules.test.ts`

**Interfaces:**
- Produces (compile.ts): `outputPathOf(path: string): string`; `CompileOutput` gains
  `texts: Record<string, string>` (every compiled YueScript source by project-relative path) and
  `loadModule(module: SourceModule): CompiledModule | undefined` (any compiled YueScript module; `sourcePath` is its
  project path). `hashes`, `sources` and `load(name)` keep their meaning (src/ only).
- Produces (modules.ts): `ModuleRoot.library?: string`; `SourceModule.library?: string` (the library key);
  `libraryModuleRoots(keys: readonly string[]): ModuleRoot[]`; `moduleLoader(modules, loadCompiled:
  (module: SourceModule) => CompiledModule | undefined)`. `LIBRARIES_DIR` is declared in Task 5's `sync.ts`; this task
  hard-codes nothing else: `libraryModuleRoots` builds `.moonwell/libraries/<key>`.

- [ ] **Step 1: Write the failing tests**

In `cli/tests/yue/compile.test.ts` add (the helper `project(files)` writes files under a temp root):

```ts
Deno.test("compileSources compiles library YueScript into dist/stage/lua/.libraries/<key>/", async () => {
  const yue = await testYue();
  const root = await project({
    "src/main.yue": 'import "example.loud"\n',
    ".moonwell/libraries/ex/example/loud.yue": "export shout = (name) -> name\\upper!\n",
  });
  const modules = await collectModules(root, [...PROJECT_MODULE_ROOTS, ...libraryModuleRoots(["ex"])]);
  const output = await compileSources({ yue, root, minify: false, modules });
  const library = modules.find((module) => module.name === "example.loud")!;
  assertEquals(library.library, "ex");
  const compiled = output.loadModule(library)!;
  assertEquals(compiled.sourcePath, ".moonwell/libraries/ex/example/loud.yue");
  assertStringIncludes(compiled.source, "upper");
  assert(await exists(join(root, "dist", "stage", "lua", ".libraries", "ex", "example", "loud.lua")));
  assertEquals(Object.keys(output.hashes), ["main.yue"], "the unknown-global check still sees src/ only");
  assertEquals(Object.keys(output.texts).sort(), [".moonwell/libraries/ex/example/loud.yue", "src/main.yue"]);
});
```

(import `collectModules`, `PROJECT_MODULE_ROOTS`, `libraryModuleRoots` from `../../src/bundle/modules.ts`; `assert` and
`exists` if the file lacks them; the `\\upper!` in the TS string is the YueScript `\upper!`, so write this file with the
file tool).

In `cli/tests/unit/modules.test.ts`:

- the `moduleLoader` test's stand-in becomes `(module) => module.name === "main" ? compiled : undefined` and the YueScript
  init case's stand-in keys on `module.name === "game.init"` (the loader now passes the `SourceModule`);
- add:

```ts
Deno.test("libraryModuleRoots gives each library a YueScript and a Lua root, in key order", () => {
  assertEquals(libraryModuleRoots(["b", "a"]), [
    { dir: ".moonwell/libraries/a", kind: "yue", required: false, library: "a" },
    { dir: ".moonwell/libraries/a", kind: "lua", required: false, library: "a" },
    { dir: ".moonwell/libraries/b", kind: "yue", required: false, library: "b" },
    { dir: ".moonwell/libraries/b", kind: "lua", required: false, library: "b" },
  ]);
});

Deno.test("a clash with a library module names both files and suggests narrowing dir", async () => {
  const root = await project({
    "src/main.yue": "x = 1\n",
    "lua/example/greet.lua": "return {}\n",
    ".moonwell/libraries/ex/example/greet.lua": "return {}\n",
  });
  try {
    const error = await assertRejects(
      () => collectModules(root, [...PROJECT_MODULE_ROOTS, ...libraryModuleRoots(["ex"])]),
      MoonwellError,
      "Module example.greet is defined by lua/example/greet.lua and .moonwell/libraries/ex/example/greet.lua.",
    );
    assertEquals(error.hint, "Rename one of them, or narrow the library's `dir`: module names are shared by src/, lua/ and libraries.");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

(import `libraryModuleRoots` and `PROJECT_MODULE_ROOTS`).

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/modules.test.ts cli/tests/yue/compile.test.ts`
Expected: FAIL: `libraryModuleRoots` is not exported; `compileSources` compiles only `src/`.

- [ ] **Step 3: `cli/src/bundle/modules.ts`**

- `ModuleRoot` gains `/** The library key when the folder is a library's (spec §4.4). */ library?: string;` and
  `SourceModule` gains `/** The library key for a library's module. */ library?: string;`. In `collectModules`, create the
  module with `library: moduleRoot.library` when it is set (omit the key otherwise, so project modules compare equal to
  `{ name, path, kind }`).
- The clash hint becomes: when `module.library !== undefined || clash.library !== undefined`,
  ``"Rename one of them, or narrow the library's `dir`: module names are shared by src/, lua/ and libraries."``;
  otherwise the current hint.
- Add:

```ts
/** A YueScript and a Lua root over `.moonwell/libraries/<key>` for each library, in key order (spec §4.4). */
export function libraryModuleRoots(keys: readonly string[]): ModuleRoot[] {
  return [...keys].sort().flatMap((key) =>
    (["yue", "lua"] as const).map((kind) => ({
      dir: `.moonwell/libraries/${key}`,
      kind,
      required: false,
      library: key,
    }))
  );
}
```

- `moduleLoader`'s `loadCompiled` parameter becomes `(module: SourceModule) => CompiledModule | undefined` and it calls
  `loadCompiled(module)`; update its doc comment.

- [ ] **Step 4: `cli/src/yue/compile.ts`**

- Add:

```ts
/**
 * Where a YueScript source compiles to, under dist/stage/lua: `src/x.yue` → `x.lua`,
 * `.moonwell/libraries/<key>/x.yue` → `.libraries/<key>/x.lua`.
 */
export function outputPathOf(path: string): string {
  const lua = path.replace(/\.yue$/, ".lua");
  if (lua.startsWith("src/")) return lua.slice("src/".length);
  if (lua.startsWith(".moonwell/libraries/")) return `.libraries/${lua.slice(".moonwell/libraries/".length)}`;
  throw new Error(`No compile output location for ${path}.`);
}
```

- Compile every YueScript module in `modules` (not only `src/`): iterate over their project-relative `path`s. Read and
  hash each file from `join(options.root, ...path.split("/"))`, write its output to
  `join(outDir, ...outputPathOf(path).split("/"))`, and label failures with `path`. Key the cache manifest's `files` by
  project-relative path, and add a format marker to the settings string so an old manifest (keyed under `src/`) is not
  reused: `` `paths|${options.yue}|${...}|${...}` ``. Remove outputs of manifest entries that are gone only when the
  previous manifest has the same format (its `settings` starts with `paths|`); `outputPathOf` of an old-format key would
  throw.
- Keep `hashes` and `sources` for `src/` modules keyed by the path under `src/` (as today), add `texts` keyed by
  project-relative path for every compiled YueScript module, and add:

```ts
    loadModule(module: SourceModule): CompiledModule | undefined {
      if (module.kind !== "yue" || !(module.path in texts)) return undefined;
      try {
        return {
          name: module.name,
          sourcePath: module.path,
          source: Deno.readTextFileSync(join(outDir, ...outputPathOf(module.path).split("/"))),
        };
      } catch (error) {
        if (error instanceof Deno.errors.NotFound) return undefined;
        throw error;
      }
    },
```

  (`load(name)` keeps working for `src/` modules; implement it through `loadModule` on the matching `src/` module.)
  Update the `modules` option's doc comment ("Every YueScript module compiles").

- [ ] **Step 5: `cli/src/pipeline.ts`**

`compileProject` passes `output.loadModule` to `moduleLoader`.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/modules.test.ts cli/tests/yue/compile.test.ts cli/tests/unit/pipeline.test.ts`
Expected: PASS (the existing compile tests, including the recompile counts, still pass).

- [ ] **Step 7: Full gate, then commit**

```bash
git add cli/src/yue/compile.ts cli/src/bundle/modules.ts cli/src/pipeline.ts cli/tests/yue/compile.test.ts cli/tests/unit/modules.test.ts
git commit -m "refactor(libraries): compile outputs by project path, and module roots for libraries"
```

---

### Task 2: Known names from reachable modules only

**Files:**
- Modify: `cli/src/lint/unknown-globals.ts`, `cli/src/pipeline.ts`
- Test: `cli/tests/unit/lint-unknown-globals.test.ts`, `cli/tests/e2e/lint.test.ts`, `cli/tests/e2e/lua.test.ts`

**Interfaces:**
- Consumes: Task 1's `CompileOutput.texts` and `resolveGraph`'s modules (`CompiledModule` with `sourcePath`, `kind`,
  `source`).
- Produces: `checkUnknownGlobals(ctx, project, compiled, natives?)` where `compiled` is
  `{ yue: string; hashes: Record<string, string>; macros?: MacroSearch; declared: { yue: readonly string[]; lua:
  readonly string[] } }` — `declared.yue` holds the YueScript texts whose `global` lines are known names,
  `declared.lua` the Lua texts whose top-level globals are; the `sources` and `lua` fields go away.

- [ ] **Step 1: Write the failing tests**

In `cli/tests/unit/lint-unknown-globals.test.ts`, `lintProject` returns
`compiled: { yue: "yue", hashes, declared: { yue: Object.values(sources), lua: options.lua ?? [] } }` (drop `sources` and
`lua`). The existing tests keep passing with that.

In `cli/tests/e2e/lint.test.ts`, `"check accepts declared globals, lint.globals and the map's globals"` must now import
the file that declares `Round`: write `src/state.yue` as before and add `import "state"` to the appended `main.yue` text,
before the `global Score` line. Add a new e2e test to `cli/tests/e2e/lua.test.ts`:

```ts
Deno.test("a Lua file's globals are known only when something requires it", async () => {
  const project = await newProject();
  await Deno.writeTextFile(join(project, "lua", "counter.lua"), "function CountUp() end\n");
  const main = join(project, "src", "main.yue");
  const base = await Deno.readTextFile(main);
  await Deno.writeTextFile(main, `${base}\nCountUp!\n`);
  const unrequired = await deno(["task", "check"], project);
  assertEquals(unrequired.code, 1, unrequired.text);
  assertStringIncludes(unrequired.text, "Unknown global CountUp.");
  await Deno.writeTextFile(main, `${base}\nrequire "counter"\nCountUp!\n`);
  const required = await deno(["task", "check"], project);
  assertEquals(required.code, 0, required.text);
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/lint-unknown-globals.test.ts` then `deno test -A cli/tests/e2e/lua.test.ts`
Expected: FAIL: `declared` is not accepted; the unrequired `CountUp` passes `check`.

- [ ] **Step 3: `cli/src/lint/unknown-globals.ts`**

Replace the `sources` and `lua` fields of `compiled` with

```ts
    /**
     * Texts whose globals are known names (spec §5.3, amended): the `global` lines of `yue` texts and the top-level
     * globals of `lua` texts. The pipeline passes the modules reachable from the entry.
     */
    declared: { yue: readonly string[]; lua: readonly string[] };
```

and build `declared` as `[...compiled.declared.yue.flatMap(declaredGlobals), ...compiled.declared.lua.flatMap(luaTopLevelGlobals)]`.
Update the function's doc comment.

- [ ] **Step 4: `cli/src/pipeline.ts`**

In `compileProject`, after `resolveGraph`, pass

```ts
    declared: {
      yue: modules.filter((module) => module.kind !== "lua").map((module) => output.texts[module.sourcePath] ?? ""),
      lua: modules.filter((module) => module.kind === "lua").map((module) => module.source),
    },
```

instead of `sources` and `lua`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/lint-unknown-globals.test.ts cli/tests/unit/pipeline.test.ts` and
`deno test -A cli/tests/e2e/lua.test.ts cli/tests/e2e/lint.test.ts`
Expected: PASS.

- [ ] **Step 6: Full gate, then commit**

```bash
git add cli/src/lint/unknown-globals.ts cli/src/pipeline.ts cli/tests/unit/lint-unknown-globals.test.ts cli/tests/e2e/lint.test.ts cli/tests/e2e/lua.test.ts
git commit -m "feat(lint): only modules the map requires make their globals known"
```

---

### Task 3: The `libraries` block

**Files:**
- Modify: `schema/Project.pkl`, `schema/tests/Project.pkl`, `cli/src/project/project.ts`, `template/moonwell.pkl`
- Regenerate: `cli/src/embedded/template.ts` (`deno task gen`)
- Test: `cli/tests/unit/project.test.ts`, `cli/tests/pkl/project.test.ts`; `Project` literals in tests

**Interfaces:**
- Produces: `interface Library { github: string | null; tag: string | null; path: string | null; dir: string }`
  (exported from `cli/src/project/project.ts`) and `Project.libraries: Record<string, Library>`.

- [ ] **Step 1: Write the failing tests**

`schema/tests/Project.pkl`: add to `["defaults"]` `Project.libraries.isEmpty`, and add

```pkl
  ["libraries accept GitHub tags and local paths"] {
    local project = (Project) {
      libraries {
        ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.1.0"; dir = "src" }
        ["mine"] { path = "../mine" }
      }
    }
    project.libraries.toMap()["example"].dir == "src"
    project.libraries.toMap()["mine"].dir == ""
  }
  ["libraries reject bad keys, repositories and folders"] {
    t.catch(() -> (Project) { libraries { ["no spaces"] { path = "x" } } }.libraries.toMap()).contains("matches")
    t.catch(() -> (Project) { libraries { ["x"] { github = "no-slash"; tag = "v1" } } }.libraries.toMap()["x"].github)
      .contains("matches")
    t.catch(() -> (Project) { libraries { ["x"] { path = "p"; dir = "../up" } } }.libraries.toMap()["x"].dir)
      .contains("isRelativeFolder")
  }
```

(adjust the substrings to the real messages if needed and say so; they must name the violated constraint).

`cli/tests/unit/project.test.ts`: the "omitted nullable fields" expectation gains `libraries: {}`; add

```ts
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
```

`cli/tests/pkl/project.test.ts`'s real-project test gains `assertEquals(project.libraries, {});`. Every `Project`
literal in unit tests (`pipeline.test.ts`, `build.test.ts`, `lint-unknown-globals.test.ts`, and any other
`deno task test` names) gains `libraries: {},`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/project.test.ts` and `pkl test schema/tests/Project.pkl`
Expected: FAIL: no `libraries`.

- [ ] **Step 3: `schema/Project.pkl`**

After `class LintConfig { ... }` add:

```pkl
/// A library of YueScript and Lua modules: a GitHub tag, or a local folder (spec: Lua modules and libraries).
class Library {
  /// A GitHub repository, "owner/repo". With `tag`, Moonwell downloads that tag; moonwell.lock records its commit.
  github: String(matches(Regex(#"[A-Za-z0-9-]+/[A-Za-z0-9._-]+"#)))?

  /// The tag to download.
  tag: String(!isEmpty)?

  /// A local folder instead, relative to the project or absolute. When set, `github` and `tag` are ignored: set it in
  /// moonwell.local.pkl to work on a library next to the map.
  path: String(!isEmpty)?

  /// The folder inside the library that module names start from, such as "src". Empty for the library's root.
  dir: String(isEmpty || isRelativeFolder(this)) = ""
}
```

and after `lint: LintConfig = new {}`:

```pkl
/// Libraries of modules, named by their path in the library (`import "example.loud"`). The key names the library's
/// folder in .moonwell/libraries/.
libraries: Mapping<String(matches(Regex(#"[A-Za-z0-9_-]+"#))), Library> = new {}
```

- [ ] **Step 4: `cli/src/project/project.ts`**

Export `Library` (above), add `libraries: Record<string, Library>;` to `Project` (after `lint`), and in `parseProject`:

```ts
  // Every 0.5 schema package has a libraries block; a manifest without one (unit fixtures) has none.
  const libraries: Record<string, Library> = {};
  for (const [key, value] of Object.entries(data.libraries === undefined ? {} : record(data.libraries, "libraries"))) {
    const path = `libraries["${key}"]`;
    const library = record(value, path);
    const parsed: Library = {
      github: nullableString(library.github, `${path}.github`),
      tag: nullableString(library.tag, `${path}.tag`),
      path: nullableString(library.path, `${path}.path`),
      dir: library.dir === undefined ? "" : string(library.dir, `${path}.dir`),
    };
    if (parsed.path === null && (parsed.github === null || parsed.tag === null)) {
      throw new MoonwellError(`${path} needs both github and tag, or a path.`, {
        file,
        hint: 'For example: ["example"] { github = "owner/repo"; tag = "v1.0.0" }, or path = "../my-library".',
      });
    }
    libraries[key] = parsed;
  }
```

and `libraries,` in the returned object.

- [ ] **Step 5: `template/moonwell.pkl`**

After the `lint { ... }` block add:

```pkl
// Libraries: modules from a GitHub tag or a local folder, named by their path in the library. moonwell.lock records
// each tag's commit; commit it. See "Libraries" in Moonwell's README.
libraries {
  // ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.1.0"; dir = "src" }
}
```

Run `deno task gen`.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `deno task test && deno task test:pkl`
Expected: PASS.

- [ ] **Step 7: Full gate, then commit**

```bash
git add schema/Project.pkl schema/tests/Project.pkl cli/src/project/project.ts template/moonwell.pkl cli/src/embedded/template.ts cli/tests
git commit -m "feat(libraries): the libraries block in moonwell.pkl"
```

---

### Task 4: GitHub archives and `moonwell.lock`

**Files:**
- Modify: `cli/src/yue/unzip.ts`
- Create: `cli/src/libraries/archive.ts`, `cli/src/libraries/lock.ts`
- Modify: `cli/tests/support/zip.ts` (optional comment)
- Test: `cli/tests/unit/library-archive.test.ts`, `cli/tests/unit/library-lock.test.ts` (new)

**Interfaces:**
- Produces: `zipComment(bytes: Uint8Array): string` (unzip.ts);
  `readGitHubArchive(bytes: Uint8Array): Promise<{ commit: string; files: Map<string, Uint8Array> }>` and
  `filesHash(files: ReadonlyMap<string, Uint8Array>): Promise<string>` (archive.ts);
  `LOCK_FILE = "moonwell.lock"`, `interface LockEntry { github: string; tag: string; dir: string; commit: string;
  files: string }`, `readLock(root: string): Promise<Record<string, LockEntry>>`,
  `writeLock(root: string, libraries: Record<string, LockEntry>): Promise<void>` (lock.ts).

- [ ] **Step 1: Write the failing tests**

Give `makeZip` an optional second parameter `comment = ""`, written after the end record (set the end record's comment
length at offset 20 and append the encoded comment).

`cli/tests/unit/library-archive.test.ts`:

```ts
import { assertEquals, assertRejects } from "@std/assert";
import { filesHash, readGitHubArchive } from "../../src/libraries/archive.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { zipComment } from "../../src/yue/unzip.ts";
import { makeZip } from "../support/zip.ts";

const COMMIT = "c07126f080c3887ba667596d08aa21df3b3a20f7";
const text = (value: string) => new TextEncoder().encode(value);

Deno.test("zipComment reads the end record's comment", async () => {
  assertEquals(zipComment(await makeZip([{ name: "a.txt", data: text("a") }], COMMIT)), COMMIT);
  assertEquals(zipComment(await makeZip([{ name: "a.txt", data: text("a") }])), "");
});

Deno.test("readGitHubArchive strips the single top folder and reads the commit", async () => {
  const zip = await makeZip([
    { name: "lib-0.1.0/README.md", data: text("# lib") },
    { name: "lib-0.1.0/src/example/greet.lua", data: text("return {}"), deflate: true },
  ], COMMIT);
  const archive = await readGitHubArchive(zip);
  assertEquals(archive.commit, COMMIT);
  assertEquals([...archive.files.keys()].sort(), ["README.md", "src/example/greet.lua"]);
});

Deno.test("readGitHubArchive refuses an archive without a commit or a single top folder", async () => {
  await assertRejects(
    async () => readGitHubArchive(await makeZip([{ name: "lib/a.lua", data: text("") }])),
    MoonwellError,
    "commit",
  );
  await assertRejects(
    async () =>
      readGitHubArchive(await makeZip([{ name: "a/x.lua", data: text("") }, { name: "b/y.lua", data: text("") }], COMMIT)),
    MoonwellError,
    "single top folder",
  );
});

Deno.test("filesHash depends on paths and contents, not order", async () => {
  const a = new Map([["x.lua", text("1")], ["y.lua", text("2")]]);
  const b = new Map([["y.lua", text("2")], ["x.lua", text("1")]]);
  assertEquals(await filesHash(a), await filesHash(b));
  assertEquals((await filesHash(a)).startsWith("sha256:"), true);
  assertEquals(await filesHash(a) === await filesHash(new Map([["x.lua", text("1")], ["y.lua", text("3")]])), false);
});
```

`cli/tests/unit/library-lock.test.ts`:

```ts
import { assertEquals, assertRejects } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { LOCK_FILE, readLock, writeLock } from "../../src/libraries/lock.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

const ENTRY = {
  github: "mdlsvensson/moonwell-example-lib",
  tag: "v0.1.0",
  dir: "src",
  commit: "c07126f080c3887ba667596d08aa21df3b3a20f7",
  files: "sha256:abc",
};

Deno.test("writeLock writes sorted JSON, readLock reads it back, and no libraries removes the file", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-lock-" });
  try {
    assertEquals(await readLock(root), {});
    await writeLock(root, { z: ENTRY, a: ENTRY });
    const text = await Deno.readTextFile(join(root, LOCK_FILE));
    assertEquals(Object.keys(JSON.parse(text).libraries), ["a", "z"]);
    assertEquals(text.endsWith("\n"), true);
    assertEquals(await readLock(root), { a: ENTRY, z: ENTRY });
    await writeLock(root, {});
    assertEquals(await exists(join(root, LOCK_FILE)), false);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("readLock refuses a lock file it cannot read", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-lock-" });
  try {
    for (const text of ["not json", '{"libraries": {"a": {"github": 1}}}', "[]"]) {
      await Deno.writeTextFile(join(root, LOCK_FILE), text);
      const error = await assertRejects(() => readLock(root), MoonwellError);
      assertEquals(error.file, LOCK_FILE);
    }
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/library-archive.test.ts cli/tests/unit/library-lock.test.ts`
Expected: FAIL: the modules do not exist.

- [ ] **Step 3: `zipComment` (`cli/src/yue/unzip.ts`)**

Move the end-of-central-directory search out of `extractZip` into a local `endOfCentralDirectory(bytes): number` (the
same loop and "not found" error), use it in `extractZip`, and add:

```ts
/** The archive comment of the end-of-central-directory record; GitHub's tag archives hold the commit SHA there. */
export function zipComment(bytes: Uint8Array): string {
  const end = endOfCentralDirectory(bytes);
  const view = new DataView(bytes.buffer, bytes.byteOffset, bytes.byteLength);
  const length = view.getUint16(end + 20, true);
  return new TextDecoder().decode(bytes.subarray(end + 22, end + 22 + length));
}
```

- [ ] **Step 4: `cli/src/libraries/archive.ts`**

```ts
import { MoonwellError } from "../shared/errors.ts";
import { sha256Hex } from "../shared/fs.ts";
import { extractZip, zipComment } from "../yue/unzip.ts";

/**
 * A GitHub tag archive's files, without their single top folder (whose name GitHub derives from the repository and
 * tag), and the commit SHA from the zip comment (spec §2, §4.2).
 */
export async function readGitHubArchive(bytes: Uint8Array): Promise<{ commit: string; files: Map<string, Uint8Array> }> {
  const commit = zipComment(bytes).trim();
  if (!/^[0-9a-f]{40}$/.test(commit)) throw new MoonwellError("The archive's comment is not a commit SHA.");
  const files = new Map<string, Uint8Array>();
  let top: string | undefined;
  for (const [name, data] of await extractZip(bytes)) {
    const slash = name.indexOf("/");
    const first = slash < 0 ? undefined : name.slice(0, slash);
    if (first === undefined || (top !== undefined && first !== top)) {
      throw new MoonwellError("The archive does not have a single top folder.");
    }
    top = first;
    files.set(name.slice(slash + 1), data);
  }
  return { commit, files };
}

/** `sha256:` and the SHA-256 of `<path>\n<sha256 of its bytes>\n` for each file, sorted by path (spec §4.3). */
export async function filesHash(files: ReadonlyMap<string, Uint8Array>): Promise<string> {
  let text = "";
  for (const path of [...files.keys()].sort()) text += `${path}\n${await sha256Hex(files.get(path)!)}\n`;
  return `sha256:${await sha256Hex(new TextEncoder().encode(text))}`;
}
```

(The `\n` inside the template literals are escapes: write this file with the file tool.)

- [ ] **Step 5: `cli/src/libraries/lock.ts`**

```ts
import { join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { removeFileIfExists, writeTextIfChanged } from "../shared/fs.ts";

/** The committed lock file, at the project root (spec §4.3). */
export const LOCK_FILE = "moonwell.lock";

/** What a GitHub library resolved to: the manifest's `github`, `tag` and `dir`, the tag's commit and the kept files. */
export interface LockEntry {
  github: string;
  tag: string;
  dir: string;
  commit: string;
  files: string;
}

const HINT = "Fix it, or delete it: the next check downloads every library again and writes a new one.";

function isEntry(value: unknown): value is LockEntry {
  const entry = value as Record<string, unknown>;
  return typeof value === "object" && value !== null &&
    ["github", "tag", "dir", "commit", "files"].every((key) => typeof entry[key] === "string");
}

/** The lock's entries by library key; none when there is no lock file. */
export async function readLock(root: string): Promise<Record<string, LockEntry>> {
  let text: string;
  try {
    text = await Deno.readTextFile(join(root, LOCK_FILE));
  } catch (cause) {
    if (cause instanceof Deno.errors.NotFound) return {};
    const reason = cause instanceof Error ? cause.message : String(cause);
    throw new MoonwellError(`Reading ${LOCK_FILE} failed: ${reason}`, { file: LOCK_FILE, cause, hint: HINT });
  }
  let data: unknown;
  try {
    data = JSON.parse(text);
  } catch (cause) {
    throw new MoonwellError(`${LOCK_FILE} is not valid JSON.`, { file: LOCK_FILE, cause, hint: HINT });
  }
  const libraries = (data as { libraries?: unknown } | null)?.libraries;
  if (typeof libraries !== "object" || libraries === null || Array.isArray(libraries) || !Object.values(libraries).every(isEntry)) {
    throw new MoonwellError(`${LOCK_FILE} is not a Moonwell lock file.`, { file: LOCK_FILE, hint: HINT });
  }
  return libraries as Record<string, LockEntry>;
}

/**
 * Writes the entries sorted by key, each as github, tag, dir, commit, files, with two-space indentation; only when
 * the text changes. No entries removes the file (plan decision).
 */
export async function writeLock(root: string, libraries: Record<string, LockEntry>): Promise<void> {
  const path = join(root, LOCK_FILE);
  const keys = Object.keys(libraries).sort();
  if (keys.length === 0) {
    await removeFileIfExists(path);
    return;
  }
  const sorted = Object.fromEntries(keys.map((key) => {
    const { github, tag, dir, commit, files } = libraries[key];
    return [key, { github, tag, dir, commit, files }];
  }));
  await writeTextIfChanged(path, `${JSON.stringify({ libraries: sorted }, null, 2)}\n`);
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/library-archive.test.ts cli/tests/unit/library-lock.test.ts cli/tests/unit/unzip.test.ts`
Expected: PASS.

- [ ] **Step 7: Full gate, then commit**

```bash
git add cli/src/yue/unzip.ts cli/src/libraries/archive.ts cli/src/libraries/lock.ts cli/tests/support/zip.ts cli/tests/unit/library-archive.test.ts cli/tests/unit/library-lock.test.ts
git commit -m "feat(libraries): read GitHub tag archives and moonwell.lock"
```

---

### Task 5: `syncLibraries`

**Files:**
- Create: `cli/src/libraries/sync.ts`
- Test: `cli/tests/unit/library-sync.test.ts` (new)

**Interfaces:**
- Consumes: Task 3's `Library`; Task 4's `readGitHubArchive`, `filesHash`, `LockEntry`, `readLock`, `writeLock`.
- Produces: `LIBRARIES_DIR = ".moonwell/libraries"`, `archiveUrl(github: string, tag: string): string`,
  `syncLibraries(root: string, libraries: Record<string, Library>, manifest: string, deps: { fetch: (url: string) =>
  Promise<Response>; logger: Logger }): Promise<void>`.

- [ ] **Step 1: Write the failing tests**

Create `cli/tests/unit/library-sync.test.ts`:

```ts
import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { readLock } from "../../src/libraries/lock.ts";
import { archiveUrl, syncLibraries } from "../../src/libraries/sync.ts";
import type { Library } from "../../src/project/project.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";
import { makeZip } from "../support/zip.ts";

const COMMIT_A = "a".repeat(40);
const COMMIT_B = "b".repeat(40);
const text = (value: string) => new TextEncoder().encode(value);
const github = (tag = "v0.1.0", dir = "src"): Library => ({ github: "owner/lib", tag, path: null, dir });

/** A stand-in for fetch that serves tag archives by URL and counts requests. */
function server(archives: Record<string, Uint8Array>) {
  const requests: string[] = [];
  const fetch = (url: string) => {
    requests.push(url);
    const body = archives[url];
    return Promise.resolve(body ? new Response(body) : new Response("Not Found", { status: 404 }));
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
  const root = await Deno.makeTempDir({ prefix: "moonwell-libs-" });
  try {
    await body(root);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
}

const read = (root: string, path: string) => Deno.readTextFile(join(root, ...path.split("/")));

Deno.test("archiveUrl encodes the tag", () => {
  assertEquals(archiveUrl("owner/lib", "v0.1.0"), "https://codeload.github.com/owner/lib/zip/refs/tags/v0.1.0");
  assertEquals(archiveUrl("o/r", "moonwell@0.4.0"), "https://codeload.github.com/o/r/zip/refs/tags/moonwell%400.4.0");
  assertEquals(archiveUrl("o/r", "release/1"), "https://codeload.github.com/o/r/zip/refs/tags/release/1");
});

Deno.test("a GitHub library is downloaded once, keeping dir, and locked by commit", async () => {
  await withRoot(async (root) => {
    const { fetch, requests } = server({
      [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, {
        "README.md": "# lib",
        "src/example/greet.lua": "return {}",
      }),
    });
    const deps = { fetch, logger: silentLogger() };
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(await read(root, ".moonwell/libraries/ex/example/greet.lua"), "return {}");
    assertEquals(await exists(join(root, ".moonwell", "libraries", "ex", "README.md")), false);
    const lock = await readLock(root);
    assertEquals(lock.ex.commit, COMMIT_A);
    assertEquals([lock.ex.github, lock.ex.tag, lock.ex.dir], ["owner/lib", "v0.1.0", "src"]);
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", deps);
    assertEquals(requests.length, 1, "an up-to-date library is not downloaded again");
    assert(deps.logger.lines.some((line) => line.includes("ex") && line.includes(COMMIT_A.slice(0, 7))));
  });
});

Deno.test("a moved tag fails; a changed tag updates the lock", async () => {
  await withRoot(async (root) => {
    const first = server({ [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, { "src/a.lua": "1" }) });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: first.fetch, logger: silentLogger() });
    await Deno.remove(join(root, ".moonwell"), { recursive: true });
    const moved = server({ [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_B, { "src/a.lua": "2" }) });
    const error = await assertRejects(
      () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: moved.fetch, logger: silentLogger() }),
      MoonwellError,
      "moved",
    );
    assertEquals(error.file, "moonwell.lock");
    assertStringIncludes(error.message, COMMIT_A.slice(0, 12));
    assertStringIncludes(error.message, COMMIT_B.slice(0, 12));
    const upgraded = server({ [archiveUrl("owner/lib", "v0.2.0")]: await archive(COMMIT_B, { "src/a.lua": "2" }) });
    await syncLibraries(root, { ex: github("v0.2.0") }, "moonwell.pkl", {
      fetch: upgraded.fetch,
      logger: silentLogger(),
    });
    assertEquals((await readLock(root)).ex.commit, COMMIT_B);
    assertEquals(await read(root, ".moonwell/libraries/ex/a.lua"), "2");
  });
});

Deno.test("download failures are MoonwellErrors naming the library", async () => {
  await withRoot(async (root) => {
    const missing = server({});
    const notFound = await assertRejects(
      () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: missing.fetch, logger: silentLogger() }),
      MoonwellError,
      "has no tag v0.1.0",
    );
    assertEquals(notFound.file, "moonwell.pkl");
    const offline = { fetch: () => Promise.reject(new TypeError("network down")), logger: silentLogger() };
    await assertRejects(() => syncLibraries(root, { ex: github() }, "moonwell.pkl", offline), MoonwellError, "ex");
    const noDir = server({ [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, { "lib/a.lua": "1" }) });
    await assertRejects(
      () => syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch: noDir.fetch, logger: silentLogger() }),
      MoonwellError,
      "no folder src",
    );
  });
});

Deno.test("a local library is copied, changed files only, and never locked", async () => {
  await withRoot(async (root) => {
    const source = join(root, "..", `${root.split(/[\\/]/).pop()}-lib`);
    await Deno.mkdir(join(source, "src", "example"), { recursive: true });
    try {
      await Deno.writeTextFile(join(source, "src", "example", "greet.lua"), "return 1");
      await Deno.writeTextFile(join(source, "src", "example", "old.lua"), "return 0");
      const local: Library = { github: null, tag: null, path: source, dir: "src" };
      const deps = { fetch: () => Promise.reject(new Error("no network in this test")), logger: silentLogger() };
      await syncLibraries(root, { mine: local }, "moonwell.pkl", deps);
      assertEquals(await read(root, ".moonwell/libraries/mine/example/greet.lua"), "return 1");
      await Deno.remove(join(source, "src", "example", "old.lua"));
      await Deno.writeTextFile(join(source, "src", "example", "greet.lua"), "return 2");
      await syncLibraries(root, { mine: local }, "moonwell.pkl", deps);
      assertEquals(await read(root, ".moonwell/libraries/mine/example/greet.lua"), "return 2");
      assertEquals(await exists(join(root, ".moonwell", "libraries", "mine", "example", "old.lua")), false);
      assertEquals(await exists(join(root, "moonwell.lock")), false);
      await assertRejects(
        () =>
          syncLibraries(root, { mine: { ...local, path: join(source, "missing") } }, "moonwell.pkl", deps),
        MoonwellError,
        "not a folder",
      );
    } finally {
      await Deno.remove(source, { recursive: true });
    }
  });
});

Deno.test("a library removed from the manifest leaves .moonwell/libraries and the lock", async () => {
  await withRoot(async (root) => {
    const { fetch } = server({ [archiveUrl("owner/lib", "v0.1.0")]: await archive(COMMIT_A, { "src/a.lua": "1" }) });
    await syncLibraries(root, { ex: github() }, "moonwell.pkl", { fetch, logger: silentLogger() });
    await syncLibraries(root, {}, "moonwell.pkl", { fetch, logger: silentLogger() });
    assertEquals(await exists(join(root, ".moonwell", "libraries", "ex")), false);
    assertEquals(await exists(join(root, "moonwell.lock")), false);
  });
});
```

(`dirname` may be unused; drop it if `deno lint` says so.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/library-sync.test.ts`
Expected: FAIL: `cli/src/libraries/sync.ts` does not exist.

- [ ] **Step 3: Create `cli/src/libraries/sync.ts`**

Implement with these parts (full behaviour is spec §4.2–4.3; messages below are exact):

```ts
import { exists } from "@std/fs";
import { dirname, join, resolve } from "@std/path";
import type { Library } from "../project/project.ts";
import { MoonwellError } from "../shared/errors.ts";
import { listFiles, removeIfExists, writeTextIfChanged } from "../shared/fs.ts";
import type { Logger } from "../shared/log.ts";
import { filesHash, readGitHubArchive } from "./archive.ts";
import { LOCK_FILE, type LockEntry, readLock, writeLock } from "./lock.ts";

/** Where libraries live in a project (git-ignored with .moonwell/). */
export const LIBRARIES_DIR = ".moonwell/libraries";

/** What a library folder holds: its lock entry, or the local path it was copied from. */
const STAMP = ".moonwell-library.json";

export interface SyncDeps {
  fetch: (url: string) => Promise<Response>;
  logger: Logger;
}

/** The zip archive of a GitHub tag (spec §2); a tag's `/` stays a path separator. */
export function archiveUrl(github: string, tag: string): string {
  return `https://codeload.github.com/${github}/zip/refs/tags/${encodeURIComponent(tag).replaceAll("%2F", "/")}`;
}
```

and `syncLibraries(root, libraries, manifest, deps)`:

1. `const lock = await readLock(root); const next: Record<string, LockEntry> = {};`
2. For each key in sorted order:
   - **Local** (`library.path !== null`): `source = resolve(root, library.path, library.dir)`; if it is not a directory,
     throw `MoonwellError(`Library ${key}: ${source} is not a folder.`, { file: manifest, hint: "Set the library's path
     (and dir) to a folder that holds its modules." })`. Copy every file of `listFiles(source)` into
     `<root>/.moonwell/libraries/<key>/` with `Deno.readFile`/`Deno.writeFile`, writing only files whose bytes differ,
     and remove files there that the source no longer has (except the stamp). Write the stamp as
     `{ "path": <source> }`. No lock entry.
   - **GitHub**: `wanted = { github, tag, dir }`; `locked = lock[key]`; `sameTag` when `locked` has the same three
     fields. When `sameTag` and the stamp in the folder equals `locked` (compare the five fields), keep `next[key] =
     locked` and do nothing else. Otherwise download `archiveUrl(github, tag)` through `deps.fetch`:
     - `fetch` throws → `MoonwellError(`Downloading library ${key} failed: ${reason}`, { file: manifest, cause, hint:
       `Check your connection and that https://github.com/${github} exists.` })`;
     - HTTP 404 → `MoonwellError(`Library ${key}: ${github} has no tag ${tag}.`, { file: manifest, hint: `See the
       tags at https://github.com/${github}/tags.` })`;
     - other non-OK status → `MoonwellError(`Downloading library ${key} failed: HTTP ${status}.`, { file: manifest,
       hint: "Try again later." })`;
     - `readGitHubArchive` fails → `MoonwellError(`The download of library ${key} is not a GitHub tag archive:
       ${message}`, { file: manifest, cause, hint: `Check ${url} in a browser.` })`.

     Keep the files under `dir` (all files when `dir` is `""`; otherwise those starting with `${dir}/`, with that prefix
     removed); none kept → `MoonwellError(`Library ${key} has no folder ${dir} at ${tag}.`, { file: manifest, hint:
     "Fix the library's dir." })`. `entry = { ...wanted, commit, files: await filesHash(kept) }`. When `sameTag` and
     (`entry.commit !== locked.commit` or `entry.files !== locked.files`) throw
     `MoonwellError(`Library ${key}: tag ${tag} of ${github} moved from ${locked.commit.slice(0, 12)} to
     ${entry.commit.slice(0, 12)} since moonwell.lock recorded it.`, { file: LOCK_FILE, hint: "If the move was
     intended, delete the library's entry from moonwell.lock and run the command again." })`. Otherwise write the kept
     files and the stamp (`entry`) into `<root>/.moonwell/libraries/.<key>.tmp/`, remove the old
     `.moonwell/libraries/<key>/`, rename the temporary folder into place, set `next[key] = entry`, and log
     `Fetched library ${key}: ${github} ${tag} (${entry.commit.slice(0, 7)}).`
3. Remove every entry of `.moonwell/libraries/` whose name is not a manifest key (folders of removed libraries and
   leftover `.<key>.tmp` folders).
4. `await writeLock(root, next)`.

Writing into `.moonwell/libraries/` wraps unexpected errors in `MoonwellError(`Writing ${LIBRARIES_DIR}/${key} failed:
${reason}`, { file: `${LIBRARIES_DIR}/${key}`, cause, hint: "Close programs that have files in .moonwell/ open, then
retry." })`. Keep the file under ~200 lines by splitting helpers (`syncLocal`, `download`, `keepDir`, `replaceFolder`,
`readStamp`).

- [ ] **Step 4: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/library-sync.test.ts`
Expected: PASS.

- [ ] **Step 5: Full gate, then commit**

```bash
git add cli/src/libraries/sync.ts cli/tests/unit/library-sync.test.ts
git commit -m "feat(libraries): sync libraries from GitHub tags and local folders"
```

---

### Task 6: Libraries in the pipeline, `setup` and `dev`

**Files:**
- Modify: `cli/src/pipeline.ts`, `cli/src/commands/setup.ts`, `cli/src/commands/dev.ts`
- Test: `cli/tests/e2e/libraries.test.ts` (new), `cli/tests/unit/dev.test.ts`

**Interfaces:**
- Consumes: `syncLibraries`, `LIBRARIES_DIR` (Task 5); `libraryModuleRoots`, `PROJECT_MODULE_ROOTS` (Task 1);
  `Project.libraries` (Task 3).
- Produces: `syncProjectLibraries(ctx: CommandContext, project: Project): Promise<void>` in `cli/src/pipeline.ts`;
  `localLibraryFolders(root: string, project: Project): string[]` (absolute `path`/`dir` folders of local libraries)
  in `cli/src/commands/dev.ts`.

- [ ] **Step 1: Write the failing e2e tests**

Create `cli/tests/e2e/libraries.test.ts` with the same `deno`/`newProject` helpers as `cli/tests/e2e/lua.test.ts`, and:

```ts
/** A local library like mdlsvensson/moonwell-example-lib, in a temp folder. */
async function exampleLibrary(): Promise<string> {
  const dir = await Deno.makeTempDir({ prefix: "moonwell-lib-" });
  await Deno.mkdir(join(dir, "src", "example"), { recursive: true });
  await Deno.writeTextFile(
    join(dir, "src", "example", "greet.lua"),
    'local M = {}\nfunction M.hello(name)\n  return "Hello, " .. name\nend\nreturn M\n',
  );
  await Deno.writeTextFile(
    join(dir, "src", "example", "loud.yue"),
    'import "example.greet"\n\nexport shout = (name) -> greet.hello(name)\\upper!\n',
  );
  await Deno.writeTextFile(join(dir, "src", "example", "globals.lua"), "function ExampleAdd(a, b)\n  return a + b\nend\n");
  return dir;
}

async function useLibrary(project: string, library: string): Promise<void> {
  const local = join(project, "moonwell.local.pkl");
  await Deno.writeTextFile(
    local,
    `${await Deno.readTextFile(local)}\nlibraries { ["ex"] { path = "${library.replaceAll("\\", "/")}"; dir = "src" } }\n`,
  );
  const main = join(project, "src", "main.yue");
  await Deno.writeTextFile(
    main,
    `${await Deno.readTextFile(main)}\nimport "example.loud"\nrequire "example.globals"\nprint loud.shout "Moonwell"\nprint ExampleAdd 1, 2\n`,
  );
}

Deno.test("a local library's modules build, and its editor view and folder are written", async () => {
  const project = await newProject();
  const library = await exampleLibrary();
  await useLibrary(project, library);
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  const archive = openMpq(await Deno.readFile(join(project, "dist", "bin", "map.w3x")));
  const lua = new TextDecoder().decode(await archive.read("war3map.lua"));
  assertStringIncludes(lua, '__mw.define("example.loud", function(...)');
  assertStringIncludes(lua, '"example.greet", ".moonwell/libraries/ex/example/greet.lua"}');
  assert(await exists(join(project, ".moonwell", "libraries", "ex", "example", "loud.yue")));
  assertEquals(await exists(join(project, "moonwell.lock")), false, "a local library is not locked");
});

Deno.test("a project module that clashes with a library module fails check, naming both", async () => {
  const project = await newProject();
  await useLibrary(project, await exampleLibrary());
  await Deno.mkdir(join(project, "lua", "example"), { recursive: true });
  await Deno.writeTextFile(join(project, "lua", "example", "greet.lua"), "return {}\n");
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(
    checked.text,
    "Module example.greet is defined by lua/example/greet.lua and .moonwell/libraries/ex/example/greet.lua.",
  );
});
```

(the `\\upper!` and the `replaceAll("\\", "/")` need the file tool; import `assert`, `exists`, `openMpq` as in the other
e2e files).

In `cli/tests/unit/dev.test.ts` add a unit test for `localLibraryFolders`:

```ts
Deno.test("localLibraryFolders lists the folders of local libraries only", () => {
  const root = Deno.cwd();
  const folders = localLibraryFolders(root, {
    libraries: {
      mine: { github: null, tag: null, path: "../mine", dir: "src" },
      remote: { github: "o/r", tag: "v1", path: null, dir: "" },
    },
  } as unknown as Project);
  assertEquals(folders, [resolve(root, "..", "mine", "src")]);
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/e2e/libraries.test.ts cli/tests/unit/dev.test.ts`
Expected: FAIL: libraries are not synced or collected; `localLibraryFolders` is missing.

- [ ] **Step 3: `cli/src/pipeline.ts`**

Add:

```ts
/** Brings .moonwell/libraries/ up to date with the manifest's libraries (spec §4.2). */
export function syncProjectLibraries(ctx: CommandContext, project: Project): Promise<void> {
  return syncLibraries(ctx.root, project.libraries, project.manifest, {
    fetch: ctx.install.fetch,
    logger: ctx.logger,
  });
}
```

In `compileProject`, call `await syncProjectLibraries(ctx, project);` first, and collect with
`collectModules(ctx.root, [...PROJECT_MODULE_ROOTS, ...libraryModuleRoots(Object.keys(project.libraries))])`. Update
its doc comment (libraries synced and bundled).

- [ ] **Step 4: `setup` and `dev`**

In `cli/src/commands/setup.ts`, call `await syncProjectLibraries(ctx, project);` before refreshing `.moonwell/`, and
mention it in the doc comment.

In `cli/src/commands/dev.ts`, export `localLibraryFolders` (`resolve(root, library.path, library.dir)` for each library
with a `path`, in key order). In `dev`, before creating the watchers, load the project to find them
(`loadProject(ctx.root, ctx.run)`), ignoring a failure (`catch` → no folders: the first cycle reports it); watch each
existing folder recursively, treat any event in them as relevant, and add them to the "Watching" message by their
path as the manifest writes it.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/dev.test.ts` and `deno test -A cli/tests/e2e/libraries.test.ts cli/tests/e2e/lua.test.ts`
Expected: PASS.

- [ ] **Step 6: Full gate, then commit**

```bash
git add cli/src/pipeline.ts cli/src/commands/setup.ts cli/src/commands/dev.ts cli/tests/e2e/libraries.test.ts cli/tests/unit/dev.test.ts
git commit -m "feat(libraries): sync and bundle libraries in check, build, test, dev and setup"
```

---

### Task 7: The editor view of libraries

**Files:**
- Create: `cli/src/editor/library-view.ts`
- Modify: `cli/src/pipeline.ts`, `cli/src/commands/setup.ts`, `cli/src/editor/scaffold.ts`, `template/.luarc.json`
- Regenerate: `cli/src/embedded/template.ts` (`deno task gen`)
- Test: `cli/tests/unit/library-view.test.ts` (new), `cli/tests/unit/editor-scaffold.test.ts`,
  `cli/tests/e2e/libraries.test.ts`

**Interfaces:**
- Produces: `LIBRARY_VIEW_DIR = ".moonwell/lua"`, `refreshLibraryView(root: string, modules: readonly SourceModule[],
  loadCompiled?: (module: SourceModule) => CompiledModule | undefined): Promise<string[]>` (the paths it wrote);
  `luarcTemplateEntries(files?): Record<"runtime.path" | "workspace.library", string[]>` in `scaffold.ts`.

- [ ] **Step 1: Write the failing tests**

Create `cli/tests/unit/library-view.test.ts`:

```ts
import { assertEquals } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import type { SourceModule } from "../../src/bundle/modules.ts";
import { refreshLibraryView } from "../../src/editor/library-view.ts";

Deno.test("refreshLibraryView writes library modules as Lua by module path, then only changes", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-view-" });
  try {
    const modules: SourceModule[] = [
      { name: "main", path: "src/main.yue", kind: "yue" },
      { name: "tools", path: "lua/tools.lua", kind: "lua", source: "return {}" },
      { name: "example.greet", path: ".moonwell/libraries/ex/example/greet.lua", kind: "lua", source: "return 1", library: "ex" },
      { name: "example.loud", path: ".moonwell/libraries/ex/example/loud.yue", kind: "yue", library: "ex" },
      { name: "kit.init", path: ".moonwell/libraries/ex/kit/init.lua", kind: "lua", source: "return 2", library: "ex" },
    ];
    const compiled = (module: SourceModule) =>
      module.name === "example.loud" ? { name: module.name, sourcePath: module.path, source: "return 3" } : undefined;
    assertEquals((await refreshLibraryView(root, modules, compiled)).sort(), [
      ".moonwell/lua/example/greet.lua",
      ".moonwell/lua/example/loud.lua",
      ".moonwell/lua/kit/init.lua",
    ]);
    assertEquals(await Deno.readTextFile(join(root, ".moonwell", "lua", "example", "loud.lua")), "return 3");
    assertEquals(await exists(join(root, ".moonwell", "lua", "tools.lua")), false, "project modules are not copied");
    assertEquals(await refreshLibraryView(root, modules, compiled), []);
    assertEquals(await refreshLibraryView(root, modules.slice(0, 3)), []);
    assertEquals(await exists(join(root, ".moonwell", "lua", "kit", "init.lua")), false, "gone modules are removed");
    assertEquals(await exists(join(root, ".moonwell", "lua", "example", "loud.lua")), false, "no compiled Lua: none");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

In `cli/tests/unit/editor-scaffold.test.ts`, the first `mergeLuarc` test's expected `workspace.library` becomes
`[".moonwell/types", "extra", ".moonwell/lua"]` and its expected `added` list ends with `".moonwell/lua"`. Add:

```ts
Deno.test("luarcTemplateEntries lists the template's runtime.path and workspace.library", () => {
  assertEquals(luarcTemplateEntries(), {
    "runtime.path": ["src/?.lua", "src/?/init.lua", "lua/?.lua", "lua/?/init.lua"],
    "workspace.library": [".moonwell/types", ".moonwell/lua"],
  });
});
```

In `cli/tests/e2e/libraries.test.ts`'s build test add:

```ts
  assertStringIncludes(
    await Deno.readTextFile(join(project, ".moonwell", "lua", "example", "loud.lua")),
    "shout",
  );
  assert(await exists(join(project, ".moonwell", "lua", "example", "greet.lua")));
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/library-view.test.ts cli/tests/unit/editor-scaffold.test.ts`
Expected: FAIL.

- [ ] **Step 3: `cli/src/editor/library-view.ts`**

`refreshLibraryView` writes, for every module with `library` set, `.moonwell/lua/<name with "." as "/">.lua`: a Lua
module's `source`, or `loadCompiled(module)?.source` for a YueScript module (skipped when there is none). It uses
`writeTextIfChanged`, returns the POSIX paths it wrote, and removes every other file under `.moonwell/lua/` (and empty
folders are harmless to leave). Unexpected write errors become `MoonwellError(`Writing .moonwell/lua failed: ...`,
{ file: ".moonwell/lua", cause, hint: "The editor reads .moonwell/; make sure it is a folder you can write, then
retry." })`.

- [ ] **Step 4: Wire it**

- `cli/src/pipeline.ts` `compileProject`: after `compileSources`, `await refreshLibraryView(ctx.root, sourceModules,
  output.loadModule);`.
- `cli/src/commands/setup.ts`: after syncing, `await refreshLibraryView(ctx.root, await collectModules(ctx.root,
  [...PROJECT_MODULE_ROOTS, ...libraryModuleRoots(Object.keys(project.libraries))]));` (Lua modules only: setup does not
  compile).
- `template/.luarc.json`'s `workspace.library` becomes `[".moonwell/types", ".moonwell/lua"]`; run `deno task gen`.
- `cli/src/editor/scaffold.ts`: export `luarcTemplateEntries(files = TEMPLATE_FILES)` (the template's two arrays, parsed
  from the embedded `.luarc.json`; `mergeLuarc` uses it), and `setup`'s "not plain JSON" warning lists them from it
  instead of hard-coding the `lua/` entries.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/library-view.test.ts cli/tests/unit/editor-scaffold.test.ts cli/tests/unit/embedded.test.ts` and `deno test -A cli/tests/e2e/libraries.test.ts`
Expected: PASS.

- [ ] **Step 6: Full gate, then commit**

```bash
git add cli/src/editor/library-view.ts cli/src/editor/scaffold.ts cli/src/pipeline.ts cli/src/commands/setup.ts template/.luarc.json cli/src/embedded/template.ts cli/tests/unit/library-view.test.ts cli/tests/unit/editor-scaffold.test.ts cli/tests/e2e/libraries.test.ts
git commit -m "feat(libraries): .moonwell/lua, the editor's view of library modules"
```

---

### Task 8: The network test

**Files:**
- Create: `cli/tests/network/libraries.test.ts`
- Modify: `deno.json`, `.github/workflows/ci.yml`, `AGENTS.md` ("Checks"), `CONTRIBUTING.md` ("Checks")

**Interfaces:**
- Consumes: `syncLibraries`, `archiveUrl`, `readLock` (Tasks 4–5).

- [ ] **Step 1: Write the test**

```ts
import { assert, assertEquals } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { readLock } from "../../src/libraries/lock.ts";
import { syncLibraries } from "../../src/libraries/sync.ts";
import { silentLogger } from "../support/logger.ts";

const enabled = Deno.env.get("MOONWELL_NETWORK_TESTS") === "1";
const EXAMPLE = { github: "mdlsvensson/moonwell-example-lib", tag: "v0.1.0", path: null, dir: "src" };

Deno.test({
  name: "the example library's v0.1.0 tag downloads and locks commit c07126f",
  ignore: !enabled,
  async fn() {
    const root = await Deno.makeTempDir({ prefix: "moonwell-network-" });
    try {
      const requests: string[] = [];
      const deps = { fetch: (url: string) => (requests.push(url), fetch(url)), logger: silentLogger() };
      await syncLibraries(root, { example: EXAMPLE }, "moonwell.pkl", deps);
      assertEquals((await readLock(root)).example.commit, "c07126f080c3887ba667596d08aa21df3b3a20f7");
      for (const file of ["greet.lua", "loud.yue", "globals.lua"]) {
        assert(await exists(join(root, ".moonwell", "libraries", "example", "example", file)), file);
      }
      await syncLibraries(root, { example: EXAMPLE }, "moonwell.pkl", deps);
      assertEquals(requests.length, 1);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  },
});
```

- [ ] **Step 2: Wire it**

- `deno.json` tasks: `"test:network": "deno test -A cli/tests/network"`.
- `.github/workflows/ci.yml`: after `deno task test:e2e`, add

```yaml
      - run: deno task test:network
        env:
          MOONWELL_NETWORK_TESTS: "1"
```

- `AGENTS.md` and `CONTRIBUTING.md` "Checks" lists gain
  `deno task test:network  # needs the network; runs only with MOONWELL_NETWORK_TESTS=1 (CI sets it)`.

- [ ] **Step 3: Run it**

Run: `MOONWELL_NETWORK_TESTS=1 deno task test:network` (PASS, if this machine has the network; if the sandbox blocks
it, say so in the report) and `deno task test:network` without the variable (1 ignored).

- [ ] **Step 4: Full gate, then commit**

```bash
git add cli/tests/network/libraries.test.ts deno.json .github/workflows/ci.yml AGENTS.md CONTRIBUTING.md
git commit -m "test(libraries): download the example library by tag, in CI"
```

---

### Task 9: Documentation

**Files:**
- Modify: `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`,
  `docs/superpowers/specs/2026-09-28-moonwell-lua-libraries-design.md`

- [ ] **Step 1: README**

Add `## Libraries` after `## Lua modules`:

````markdown
## Libraries

A library is a folder of YueScript and Lua modules from a GitHub tag or a local folder. List libraries in
`moonwell.pkl`:

```pkl
libraries {
  ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.1.0"; dir = "src" }
}
```

`dir` is the folder inside the library that module names start from; leave it out for the library's root. A library's
modules keep their own names (`import "example.loud"`), and share one set of names with `src/` and `lua/`: a name two
of them define fails the build.

`check`, `build`, `test`, `dev` and `setup` download a library that is missing or whose `tag` or `dir` changed into
`.moonwell/libraries/<key>/` (git-ignored), and record the tag's commit in `moonwell.lock`. Commit `moonwell.lock`: a
fresh clone then gets the same code, and if a tag is moved on GitHub, the command fails instead of using the new code.
To upgrade, change `tag`.

To work on a library next to your map, point it at a local folder in `moonwell.local.pkl`:

```pkl
libraries { ["example"] { path = "../moonwell-example-lib"; dir = "src" } }
```

`path` wins over `github`, and `dev` watches that folder. Local libraries are never locked. Library code is not checked
for unknown globals, but the globals a required library module defines count as known. The editor gets every library
module in `.moonwell/lua/`.
````

In `## Unknown globals`, the known-names sentence changes "a name declared with `global` in any file under `src/`" (and
the `lua/` globals sentence) to say: in a module the map requires (its entry, and every module reached from it through
`import`/`require`), including library modules.

- [ ] **Step 2: CHANGELOG**

Under `## Unreleased`, add:

```markdown
- Libraries: the new `libraries` block in `moonwell.pkl` brings YueScript and Lua modules from a GitHub tag
  (`github`, `tag`, `dir`) or a local folder (`path`) into `.moonwell/libraries/`, and `moonwell.lock` records each
  tag's commit; a moved tag fails the command. Library modules keep their own names and share one namespace with
  `src/` and `lua/`. The editor sees them in `.moonwell/lua/`; `setup` adds it to older projects' `.luarc.json`.
- Changed: `global` lines and the top-level globals of Lua modules count as known names only for modules the map
  requires. A global declared in a file nothing imports now fails the unknown-global check: import the file, or add the
  name to `lint.globals`.
```

- [ ] **Step 3: CONTRIBUTING step 12**

Append to step 12:

```markdown
    Then add the example library by tag in `moonwell.pkl`
    (`libraries { ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.1.0"; dir = "src" } }`), use it
    from `src/main.yue` (`import "example.loud"`, `print loud.shout "Moonwell"`), run `deno task check` and commit
    `moonwell.lock`. Confirm the editor completes `loud.shout`, and the game prints the shout. Delete `.moonwell/`, run
    `deno task check` again, and confirm `moonwell.lock` is unchanged. Finally clone the library next to the project,
    point `moonwell.local.pkl` at it (`libraries { ["example"] { path = "../moonwell-example-lib"; dir = "src" } }`),
    change `hello` there, and confirm `deno task test` runs the change.
```

- [ ] **Step 4: AGENTS.md**

Add a "Plan 4b done, unreleased" State bullet (date, plan path; `cli/src/libraries/` — archive, lock, sync;
`.moonwell/libraries/`, `moonwell.lock`, `.moonwell/lua/`; compile outputs keyed by project path; known names from the
resolved graph; the example library and the network test; gate not run yet). "Next work" item 1 becomes: run
CONTRIBUTING step 12 in full, then the whole release gate, then release 0.5.0.

- [ ] **Step 5: Spec**

In `docs/superpowers/specs/2026-09-28-moonwell-lua-libraries-design.md`: §3.4 and §5.3 say known names come from modules
reachable from the entry (with a line: "Amended 2026-09-28 during Plan 4b"); §4.2 says syncing happens at the start of
every compile and in `setup`, and that the archive's top folder is stripped whatever its name; §4.3 says the lock file
is removed when no GitHub library remains; §10's input names `mdlsvensson/moonwell-example-lib` `v0.1.0`.

- [ ] **Step 6: Check and commit**

Run: `deno fmt --check`.

```bash
git add README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md docs/superpowers/specs/2026-09-28-moonwell-lua-libraries-design.md
git commit -m "docs: libraries"
```

---

### Task 10: Maintainer gate, libraries

The maintainer runs the libraries part of CONTRIBUTING step 12 on Warcraft III Reforged 3.0.0.24268; the controller runs
what needs no game or editor window first (download, lock, re-fetch from a clean `.moonwell/`, local override) and the
editor half with the Lua extension's server. Record the result under "Release gate (so far)" in `CHANGELOG.md` and
commit.

## Completion

All ten tasks done, the full gate green, the maintainer's gate recorded. Then the 0.5.0 release: the rest of the
release gate, the version bump and publishing.
