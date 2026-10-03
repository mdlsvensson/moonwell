# Moonwell Lua Modules (Plan 4a) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A project's own `lua/**/*.lua` files are modules next to `src/**/*.yue`: one namespace, bundled when
required, source-mapped at runtime, their top-level globals known to the unknown-global check, and resolved by the
editor.

**Architecture:** A new `collectModules` lists every module (YueScript in `src/`, Lua in `lua/`) and rejects dotted
names and clashes. `compileSources` compiles the YueScript ones as before; a `moduleLoader` gives the bundler graph
either a compiled YueScript module or a Lua module's own source, trying `<name>.init` too. The bundle's line table marks
minified modules per entry, so Lua modules keep line numbers in `--minify` builds. A token-based scan finds a Lua
file's top-level globals for the unknown-global check. The template and `setup` teach `.luarc.json` about `lua/`.

**Tech Stack:** Deno 2.9+, TypeScript, existing `jsr:@std/*` imports, Pkl 0.32, YueScript 0.34.2.

**Spec:** `docs/superpowers/specs/2026-09-28-moonwell-lua-libraries-design.md` (approved 2026-09-28). This plan covers
§§3, 5.1–5.3 as they apply to `lua/`, and the Plan 4a rows of §§6–11. Plan 4b (libraries) follows; 0.5.0 is released
after it.

## Global Constraints

- No Node.js: no `package.json`, `node_modules`, `npm:` or `node:` specifiers. Only `jsr:@std/*` from the existing
  import maps.
- File-system code is async. Expected failures throw `MoonwellError` (`cli/src/shared/errors.ts`) with `file` and
  `hint`; anything else is reported as an internal error, so user mistakes must never reach it.
- `deno fmt` width 120 (`template/`, `docs/`, `cli/src/embedded/` excluded); `deno fmt --check` and `deno lint` clean.
- After changing `template/` or `cli/runtime/moonwell.lua`, run `deno task gen`. Nothing stray in `template/`: every
  file there is embedded.
- Write files containing backslashes with a file-editing tool, never a shell heredoc or `sed`.
- Module names: a path under its folder with `/` as `.`; folder and file names cannot contain dots. The error text
  stays `Module file and folder names cannot contain dots.` with the hint
  ``Dots separate module names in `import`; rename the file or folder.``
- Every task: failing test first, then code, then the full gate (below), then one commit on `main`.
  Do not push; the maintainer pushes. Do not bump versions.

**Full gate (before every commit):**

```bash
deno task check && deno task lint && deno fmt --check && deno task test && deno task test:runtime && deno task test:pkl && deno task test:e2e
```

## Answers to the spec's "To verify" (§9), checked 2026-09-28

| Claim | Answer |
| --- | --- |
| V1 lua-language-server resolves `lua/` modules through `runtime.path` | Yes. With `lua/?.lua` and `lua/?/init.lua` in `runtime.path`, `require("tools")` resolved to `lua/tools.lua` and `require("kit")` to `lua/kit/init.lua` (a call with a wrong argument type against each `---@param` was flagged), and a global defined at the top of `lua/counter.lua` was not flagged while an unknown one was. Checked with sumneko.lua 3.19.1's server `--check`. |
| V2 it resolves a module under a `workspace.library` folder | Yes: `require("core.scheduler")` resolved to `.moonwell/lua/core/scheduler.lua` with `.moonwell/lua` in `workspace.library` (Plan 4b uses this). |
| V3 the game runs a bundled global-style Lua library | Moved to the release gate (Task 7): it needs this plan's bundling first. Plain Lua semantics: a global assigned inside the module's wrapper function is a real global. |
| V4 annotated and lightweight tags both give the commit in the archive comment | Yes: `IppClub/YueScript` `v0.34.2` (annotated, tag object `e346f11…`) gives commit `3c5cbf6fc9ddff95798b9853ccb74607a9a6783d`; `mdlsvensson/moonwell` `moonwell@0.4.0` (lightweight) gives `27cd6ec…` (Plan 4b). |

## Decisions this plan adds to the spec

- **A module is defined under the name it was required by.** `require "kit"` resolving to `lua/kit/init.lua` defines
  `kit`; a second `require "kit.init"` would define the same file again, as Lua's own `package.loaded` would.
- **The line table marks minified modules per entry** (`{first, last, name, path, true}`), replacing the bundle-wide
  `__mw.minified` flag, so a Lua module in a `--minify` build keeps its line numbers (spec §3.3).
- **Lua files are listed only under `lua/`** and YueScript files only under `src/`; other files there are ignored.
  `src/**/*.lua` stays the editor's output.
- **The top-level global scan** (spec §3.4) is token-based: a statement is recognised when its first token starts a
  line or follows `;`, at block depth 0 and outside brackets.
- **`setup` reports but does not fail** when `.luarc.json` is not a JSON object (for example, it has comments): it
  logs a warning naming the entries to add.

## File Structure

| File | Responsibility |
| --- | --- |
| `cli/src/bundle/modules.ts` (new) | `SourceModule`, `ModuleRoot`, `PROJECT_MODULE_ROOTS`, `collectModules`, `moduleLoader` |
| `cli/src/yue/compile.ts` | compiles the YueScript modules `collectModules` lists; `CompiledModule.kind` |
| `cli/src/bundle/emit.ts`, `cli/runtime/moonwell.lua` | per-module minified flag in the line table |
| `cli/src/bundle/graph.ts` | "Module not found" hint names `lua/` |
| `cli/src/pipeline.ts` | `compileProject` collects modules and uses `moduleLoader` |
| `cli/src/lint/lua-globals.ts` (new) | `luaTopLevelGlobals` |
| `cli/src/lint/unknown-globals.ts` | Lua modules' top-level globals are known names |
| `cli/src/editor/scaffold.ts`, `cli/src/commands/setup.ts` | `mergeLuarc` |
| `cli/src/commands/dev.ts` | watches `lua/` |
| `template/.luarc.json`, `template/lua/.gitkeep` | `lua/` in `runtime.path`; the `lua/` folder |
| Tests | `cli/tests/unit/{modules,emit,lua-globals,lint-unknown-globals,editor-scaffold,dev}.test.ts`, `cli/tests/yue/runtime.test.ts`, `cli/tests/e2e/lua.test.ts` |
| Docs | `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md` |

---

### Task 1: Collect modules from `src/` and `lua/`

**Files:**
- Create: `cli/src/bundle/modules.ts`
- Modify: `cli/src/yue/compile.ts`
- Test: `cli/tests/unit/modules.test.ts` (new)

**Interfaces:**
- Produces (in `cli/src/bundle/modules.ts`): `type ModuleKind = "yue" | "lua"`;
  `interface SourceModule { name: string; path: string; kind: ModuleKind; source?: string }` (`path` is POSIX,
  relative to the project root; `source` is set for Lua modules);
  `interface ModuleRoot { dir: string; kind: ModuleKind; required: boolean }`;
  `PROJECT_MODULE_ROOTS: readonly ModuleRoot[]`;
  `collectModules(root: string, roots?: readonly ModuleRoot[]): Promise<SourceModule[]>`;
  `moduleLoader(modules: readonly SourceModule[], loadCompiled: (name: string) => CompiledModule | undefined):
  (name: string) => CompiledModule | undefined`.
- In `cli/src/yue/compile.ts`: `CompiledModule` gains `kind?: "yue" | "lua"`; `compileSources(options)` gains
  `modules?: readonly SourceModule[]`.

- [ ] **Step 1: Write the failing tests**

Create `cli/tests/unit/modules.test.ts`:

```ts
import { assertEquals, assertRejects } from "@std/assert";
import { dirname, join } from "@std/path";
import { collectModules, moduleLoader } from "../../src/bundle/modules.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

async function project(files: Record<string, string>): Promise<string> {
  const root = await Deno.makeTempDir({ prefix: "moonwell-modules-" });
  for (const [path, text] of Object.entries(files)) {
    await Deno.mkdir(dirname(join(root, path)), { recursive: true });
    await Deno.writeTextFile(join(root, path), text);
  }
  return root;
}

Deno.test("collectModules lists YueScript in src/ and Lua in lua/, named by path", async () => {
  const root = await project({
    "src/main.yue": "x = 1\n",
    "src/game/units.yue": "x = 1\n",
    "src/main.lua": "-- the editor's output, ignored\n",
    "lua/tools/init.lua": "return {}\n",
    "lua/counter.lua": "Count = 0\n",
    "lua/README.md": "ignored\n",
  });
  try {
    assertEquals(await collectModules(root), [
      { name: "game.units", path: "src/game/units.yue", kind: "yue" },
      { name: "main", path: "src/main.yue", kind: "yue" },
      { name: "counter", path: "lua/counter.lua", kind: "lua", source: "Count = 0\n" },
      { name: "tools.init", path: "lua/tools/init.lua", kind: "lua", source: "return {}\n" },
    ]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("collectModules works without lua/ and requires src/", async () => {
  const root = await project({ "src/main.yue": "x = 1\n" });
  try {
    assertEquals((await collectModules(root)).map((module) => module.path), ["src/main.yue"]);
    await Deno.remove(join(root, "src"), { recursive: true });
    await assertRejects(() => collectModules(root), MoonwellError, "The src/ folder is missing.");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("collectModules refuses dotted names in either folder", async () => {
  for (const path of ["src/a.b.yue", "lua/x.y/z.lua"]) {
    const root = await project({ "src/main.yue": "x = 1\n", [path]: "" });
    try {
      const error = await assertRejects(() => collectModules(root), MoonwellError, "cannot contain dots");
      assertEquals(error.file, path);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  }
});

Deno.test("collectModules refuses a name two files define, naming both", async () => {
  const root = await project({ "src/main.yue": "x = 1\n", "src/tools.yue": "x = 1\n", "lua/tools.lua": "" });
  try {
    const error = await assertRejects(
      () => collectModules(root),
      MoonwellError,
      "Module tools is defined by src/tools.yue and lua/tools.lua.",
    );
    assertEquals(error.file, "lua/tools.lua");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("moduleLoader resolves a name, then <name>.init, under the name that was required", () => {
  const modules = [
    { name: "main", path: "src/main.yue", kind: "yue" as const },
    { name: "tools.init", path: "lua/tools/init.lua", kind: "lua" as const, source: "return {}" },
  ];
  const compiled = { name: "main", sourcePath: "src/main.yue", source: "local x = 1" };
  const load = moduleLoader(modules, (name) => name === "main" ? compiled : undefined);
  assertEquals(load("main"), compiled);
  assertEquals(load("tools"), { name: "tools", sourcePath: "lua/tools/init.lua", source: "return {}", kind: "lua" });
  assertEquals(load("tools.init")?.name, "tools.init");
  assertEquals(load("missing"), undefined);
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/modules.test.ts`
Expected: FAIL: `cli/src/bundle/modules.ts` not found.

- [ ] **Step 3: Create `cli/src/bundle/modules.ts`**

```ts
import { exists } from "@std/fs";
import { join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { listFiles } from "../shared/fs.ts";
import type { CompiledModule } from "../yue/compile.ts";

export type ModuleKind = "yue" | "lua";

/** A gameplay module on disk (spec §5.1). */
export interface SourceModule {
  /** Dotted name, e.g. "utils.timer". */
  name: string;
  /** POSIX path relative to the project root, e.g. "lua/utils/timer.lua". */
  path: string;
  kind: ModuleKind;
  /** A Lua module's text; YueScript modules are read when they are compiled. */
  source?: string;
}

/** A folder of modules, relative to the project root, and the kind of file it holds. */
export interface ModuleRoot {
  /** POSIX path relative to the project root, e.g. "src". */
  dir: string;
  kind: ModuleKind;
  /** Whether a missing folder is an error. */
  required: boolean;
}

/** The project's own modules: YueScript in src/ and Lua in lua/ (spec §3.1). src/**\/*.lua is the editor's output. */
export const PROJECT_MODULE_ROOTS: readonly ModuleRoot[] = [
  { dir: "src", kind: "yue", required: true },
  { dir: "lua", kind: "lua", required: false },
];

/**
 * Lists every module under `roots`, in root order and then by path. Fails on a dotted file or folder name (spec §3.1)
 * and on two files with one name (spec §3.2).
 */
export async function collectModules(
  root: string,
  roots: readonly ModuleRoot[] = PROJECT_MODULE_ROOTS,
): Promise<SourceModule[]> {
  const modules: SourceModule[] = [];
  const byName = new Map<string, SourceModule>();
  for (const moduleRoot of roots) {
    const dir = join(root, ...moduleRoot.dir.split("/"));
    if (!(await exists(dir, { isDirectory: true }))) {
      if (moduleRoot.required) throw new MoonwellError(`The ${moduleRoot.dir}/ folder is missing.`, { file: root });
      continue;
    }
    const extension = `.${moduleRoot.kind}`;
    for (const file of (await listFiles(dir)).filter((path) => path.endsWith(extension))) {
      const path = `${moduleRoot.dir}/${file}`;
      const stem = file.slice(0, -extension.length);
      if (stem.split("/").some((segment) => segment.includes("."))) {
        throw new MoonwellError("Module file and folder names cannot contain dots.", {
          file: path,
          hint: "Dots separate module names in `import`; rename the file or folder.",
        });
      }
      const module: SourceModule = { name: stem.split("/").join("."), path, kind: moduleRoot.kind };
      if (module.kind === "lua") module.source = await Deno.readTextFile(join(dir, ...file.split("/")));
      const clash = byName.get(module.name);
      if (clash !== undefined) {
        throw new MoonwellError(`Module ${module.name} is defined by ${clash.path} and ${path}.`, {
          file: path,
          hint: "Rename one of them: module names are shared by src/ and lua/.",
        });
      }
      byName.set(module.name, module);
      modules.push(module);
    }
  }
  return modules;
}

/**
 * Resolves `require` names for the bundler: the name, then `<name>.init`, Lua's `?/init.lua` convention (spec §3.2).
 * A Lua module is its own source; a YueScript module is its compiled output from `loadCompiled`. Either is returned
 * under the name it was required by, which is the name the bundle defines it with.
 */
export function moduleLoader(
  modules: readonly SourceModule[],
  loadCompiled: (name: string) => CompiledModule | undefined,
): (name: string) => CompiledModule | undefined {
  const byName = new Map(modules.map((module) => [module.name, module]));
  return (name) => {
    const module = byName.get(name) ?? byName.get(`${name}.init`);
    if (module === undefined) return undefined;
    if (module.kind === "lua") return { name, sourcePath: module.path, source: module.source ?? "", kind: "lua" };
    const compiled = loadCompiled(module.name);
    return compiled && { ...compiled, name };
  };
}
```

(The `\/` in the doc comment keeps `*/` from closing it.)

- [ ] **Step 4: Let `compileSources` use the collected modules (`cli/src/yue/compile.ts`)**

- Add to `CompiledModule`:

```ts
  /** "lua" for a Lua module, which the bundle never minifies; YueScript when absent. */
  kind?: "yue" | "lua";
```

- Add to `compileSources`'s options:

```ts
  /** The project's modules (`collectModules`); listed from disk when absent. Only YueScript modules under src/ compile. */
  modules?: readonly SourceModule[];
```

- Replace the block from `const srcDir = ...` through the end of the dotted-name `for` loop with:

```ts
  const srcDir = join(options.root, "src");
  const outDir = join(options.root, "dist", "stage", "lua");
  const modules = options.modules ?? await collectModules(options.root);
  const sources = modules
    .filter((module) => module.kind === "yue" && module.path.startsWith("src/"))
    .map((module) => module.path.slice("src/".length));
```

  (`collectModules` now owns the missing-`src/` and dotted-name errors, with the same messages.) Import
  `collectModules` and `type SourceModule` from `../bundle/modules.ts`; drop the `exists` import if nothing else uses
  it.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/modules.test.ts` and `deno test -A cli/tests/yue/compile.test.ts`
Expected: PASS (the compile test "rejects dots in file names" still passes through `collectModules`).

- [ ] **Step 6: Full gate, then commit**

```bash
git add cli/src/bundle/modules.ts cli/src/yue/compile.ts cli/tests/unit/modules.test.ts
git commit -m "feat(lua): collect modules from src/ and lua/, one namespace"
```

---

### Task 2: Bundle Lua modules

**Files:**
- Modify: `cli/src/pipeline.ts`, `cli/src/bundle/emit.ts`, `cli/runtime/moonwell.lua`, `cli/src/bundle/graph.ts`
- Regenerate: `cli/src/embedded/runtime.ts` (`deno task gen`)
- Test: `cli/tests/unit/emit.test.ts`, `cli/tests/yue/runtime.test.ts`

**Interfaces:**
- Consumes (Task 1): `collectModules`, `moduleLoader`, `CompiledModule.kind`, `compileSources({ ..., modules })`.
- Produces: the line table entry `{first, last, name, path}` gains a fifth element `true` for a minified module; the
  bundle no longer sets `__mw.minified`.

- [ ] **Step 1: Write the failing tests**

In `cli/tests/unit/emit.test.ts`, replace the test `"emitBundle marks minified bundles so errors name modules without
lines"` with:

```ts
Deno.test("emitBundle marks minified YueScript modules; Lua modules keep their lines", () => {
  const lua = { name: "lib", sourcePath: "lua/lib.lua", source: "return {}", kind: "lua" as const };
  const plain = emitBundle({ runtime: "", modules: [...modules, lua], entry: "main", firstLine: 1 });
  const minified = emitBundle({ runtime: "", modules: [...modules, lua], entry: "main", firstLine: 1, minify: true });
  assertEquals(plain.includes(", true},"), false);
  assertStringIncludes(minified, '"main", "src/main.yue", true},');
  assertStringIncludes(minified, '"lib", "lua/lib.lua"},');
  assertEquals(minified.includes("__mw.minified"), false);
});
```

In `cli/tests/yue/runtime.test.ts`, let `runMap` take an optional path and kind per module:

```ts
async function runMap(
  modules: Array<{ name: string; source: string; sourcePath?: string; kind?: "yue" | "lua" }>,
  minify = false,
): Promise<{ log: string; printed: string }> {
  const compiled = modules.map((module) => ({
    ...module,
    sourcePath: module.sourcePath ?? `src/${module.name.split(".").join("/")}.yue`,
  }));
```

(the rest of `runMap` is unchanged), and add:

```ts
Deno.test("a Lua module keeps its line numbers in a minified bundle", async () => {
  const { printed } = await runMap([
    { name: "main", source: 'local lib = require("lib")\nlib.fail()' },
    {
      name: "lib",
      source: 'local M = {}\nfunction M.fail()\n  error("lua failed")\nend\nreturn M',
      sourcePath: "lua/lib.lua",
      kind: "lua",
    },
  ], true);
  assertStringIncludes(printed, "lua/lib.lua:3: lua failed");
});
```

The existing test `"minified bundles report the module file without a line number"` stays and must still pass.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/emit.test.ts cli/tests/yue/runtime.test.ts`
Expected: FAIL: the line table has no fifth element and the bundle sets `__mw.minified = true`, so the Lua module's
line is dropped.

- [ ] **Step 3: Mark minified modules per entry**

In `cli/src/bundle/emit.ts`, in the module loop, replace the `ranges.push(...)` call with:

```ts
    // A Lua module is its own source, never minified, so it keeps its lines (spec §3.3).
    const minified = input.minify === true && module.kind !== "lua";
    ranges.push(
      `{${start}, ${start + body.length - 1}, ${JSON.stringify(module.name)}, ${JSON.stringify(module.sourcePath)}${
        minified ? ", true" : ""
      }},`,
    );
```

delete `if (input.minify) out.push("__mw.minified = true");`, and change the doc comment's second sentence to
"Minified YueScript modules keep no source lines, so their errors name the file only; Lua modules keep theirs."

In `cli/runtime/moonwell.lua`, delete the `minified = false,` field and the comment above it, change the `map_line`
comment to `-- Maps an absolute war3map.lua line to "src/file.yue:<line>" ("src/file.yue" when that module is
minified), or nil outside modules.`, and change `if __mw.minified then` to `if entry[5] then`. Run `deno task gen`.

- [ ] **Step 4: Resolve Lua modules in `compileProject` (`cli/src/pipeline.ts`)**

Import `collectModules` and `moduleLoader` from `./bundle/modules.ts`. In `compileProject`, before `compileSources`,
add `const sourceModules = await collectModules(ctx.root);` (not `sources`: `output.sources` already means the
YueScript texts), pass `modules: sourceModules` to `compileSources`, and resolve with
`resolveGraph(entry, moduleLoader(sourceModules, output.load), BUILTIN_MODULES)`. Change its doc comment to
`/** Compiles src/, resolves the reachable module graph (src/ and lua/) from the entry and checks for unknown globals. */`.

In `cli/src/bundle/graph.ts`, change the "Module not found" hint to:

```ts
        hint: `Expected src/${path}.yue or lua/${path}.lua. Built-in modules: ${[...builtins].join(", ")}.`,
```

with `const path = name.split(".").join("/");` declared just before the `throw`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/emit.test.ts cli/tests/yue/runtime.test.ts cli/tests/unit/graph.test.ts cli/tests/unit/embedded.test.ts`
Expected: PASS.

- [ ] **Step 6: Full gate, then commit**

```bash
git add cli/src/pipeline.ts cli/src/bundle/emit.ts cli/src/bundle/graph.ts cli/runtime/moonwell.lua cli/src/embedded/runtime.ts cli/tests/unit/emit.test.ts cli/tests/yue/runtime.test.ts
git commit -m "feat(lua): bundle lua/ modules, keeping their lines in minified builds"
```

---

### Task 3: Lua top-level globals in the unknown-global check

**Files:**
- Create: `cli/src/lint/lua-globals.ts`
- Modify: `cli/src/lint/unknown-globals.ts`, `cli/src/pipeline.ts`
- Test: `cli/tests/unit/lua-globals.test.ts` (new), `cli/tests/unit/lint-unknown-globals.test.ts`

**Interfaces:**
- Consumes: `tokenize(source): Token[]` (`cli/src/bundle/lexer.ts`; tokens are `name`, `string` or `punct` with a
  `line`, numbers and comments dropped, keywords are `name` tokens, `==` is two `=` tokens); `SourceModule` (Task 1).
- Produces: `luaTopLevelGlobals(source: string): string[]`; `checkUnknownGlobals`'s `compiled` parameter gains
  `lua?: readonly { source?: string }[]`.

- [ ] **Step 1: Write the failing tests**

Create `cli/tests/unit/lua-globals.test.ts`:

```ts
import { assertEquals } from "@std/assert";
import { luaTopLevelGlobals } from "../../src/lint/lua-globals.ts";

Deno.test("luaTopLevelGlobals finds top-level global functions and assignments only", () => {
  const source = [
    "function OnInit(fn) end",
    "Timer = {}",
    "A, B = 1, 2",
    "local hidden = 1",
    "local function helper() end",
    "function Timer.start() end",
    "function Timer:stop() end",
    "Config.value = 3",
    "if ready then Inner = 1 end",
    "local t = {",
    "  Field = 1,",
    "}",
    "do Scoped = 2 end",
    "for i = 1, 3 do Looped = i end",
    "x = 1; Y = 2",
    'print("Z = 1") -- W = 2',
    "--[[ V = 3 ]]",
    "Last = function() Nested = 1 end",
    "if a == b then end",
    "Timer = nil",
  ].join("\n");
  assertEquals(luaTopLevelGlobals(source), ["OnInit", "Timer", "A", "B", "x", "Y", "Last"]);
});

Deno.test("luaTopLevelGlobals finds nothing in a module that returns a table", () => {
  assertEquals(luaTopLevelGlobals("local M = {}\nfunction M.greet() end\nreturn M\n"), []);
});
```

In `cli/tests/unit/lint-unknown-globals.test.ts`, extend `lintProject`'s options with `lua?: string[]` and return
`compiled: { yue: "yue", hashes, sources, lua: (options.lua ?? []).map((source) => ({ source })) }` (keep whatever
`compiled` already holds and add `lua`), then add:

```ts
Deno.test("checkUnknownGlobals knows the globals a Lua module defines at its top level", async () => {
  const { root, ctx, project, compiled } = await lintProject(
    { "main.yue": "CountUp!\n" },
    { "main.yue": "CountUp 1 1\n" },
    { lua: ["Count = 0\nfunction CountUp()\n  Count = Count + 1\nend\n"] },
  );
  try {
    assertEquals(await checkUnknownGlobals(ctx, project, compiled, NATIVES), []);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/lua-globals.test.ts cli/tests/unit/lint-unknown-globals.test.ts`
Expected: FAIL: `cli/src/lint/lua-globals.ts` not found; `CountUp` is unknown.

- [ ] **Step 3: Create `cli/src/lint/lua-globals.ts`**

```ts
import { type Token, tokenize } from "../bundle/lexer.ts";

const KEYWORDS = new Set([
  "and", "break", "do", "else", "elseif", "end", "false", "for", "function", "goto", "if", "in", "local", "nil", "not",
  "or", "repeat", "return", "then", "true", "until", "while",
]);
const BLOCK_OPEN = new Set(["function", "do", "if", "repeat"]);
const BLOCK_CLOSE = new Set(["end", "until"]);
const BRACKET_OPEN = new Set(["(", "{", "["]);
const BRACKET_CLOSE = new Set([")", "}", "]"]);

const isPunct = (token: Token | undefined, value: string) => token?.kind === "punct" && token.value === value;
const isName = (token: Token | undefined) => token?.kind === "name" && !KEYWORDS.has(token.value);

/** Whether `tokens[i]` begins a statement: it starts a line or follows `;` (plan decision). */
function startsStatement(tokens: Token[], i: number): boolean {
  const previous = tokens[i - 1];
  return previous === undefined || previous.line < tokens[i].line || isPunct(previous, ";");
}

/** `Name {, Name} =` (not `==`) starting at `i`: the names; none for anything else. */
function assignedNames(tokens: Token[], i: number): string[] {
  const names: string[] = [];
  for (let j = i; isName(tokens[j]); j += 2) {
    names.push(tokens[j].value);
    if (isPunct(tokens[j + 1], ",")) continue;
    return isPunct(tokens[j + 1], "=") && !isPunct(tokens[j + 2], "=") ? names : [];
  }
  return [];
}

/**
 * The globals a Lua module defines at its top level (spec §3.4): `function Name(` and `Name = ...` or
 * `Name, Other = ...` without `local`, outside every function, block and bracket. In order, without duplicates.
 */
export function luaTopLevelGlobals(source: string): string[] {
  const tokens = tokenize(source);
  const names: string[] = [];
  let blocks = 0;
  let brackets = 0;
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i];
    const topLevel = blocks === 0 && brackets === 0;
    if (token.kind === "name") {
      if (topLevel && token.value === "function" && tokens[i - 1]?.value !== "local") {
        if (isName(tokens[i + 1]) && isPunct(tokens[i + 2], "(")) names.push(tokens[i + 1].value);
      } else if (topLevel && isName(token) && startsStatement(tokens, i)) {
        names.push(...assignedNames(tokens, i));
      }
      if (BLOCK_OPEN.has(token.value)) blocks++;
      else if (BLOCK_CLOSE.has(token.value)) blocks = Math.max(0, blocks - 1);
    } else if (token.kind === "punct") {
      if (BRACKET_OPEN.has(token.value)) brackets++;
      else if (BRACKET_CLOSE.has(token.value)) brackets = Math.max(0, brackets - 1);
    }
  }
  return [...new Set(names)];
}
```

(`deno fmt` will lay out the keyword list its own way.)

- [ ] **Step 4: Use them in `checkUnknownGlobals` and pass them from `compileProject`**

In `cli/src/lint/unknown-globals.ts`, import `luaTopLevelGlobals` from `./lua-globals.ts`, add to the `compiled`
parameter type `lua?: readonly { source?: string }[];` with the doc line "Lua modules: their top-level globals are
known names (spec §3.4); they are not checked themselves.", and after the `declared` line add:

```ts
  for (const module of compiled.lua ?? []) declared.push(...luaTopLevelGlobals(module.source ?? ""));
```

In `cli/src/pipeline.ts`, pass `lua: sourceModules.filter((module) => module.kind === "lua")` in the `checkUnknownGlobals`
call's `compiled` object.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/lua-globals.test.ts cli/tests/unit/lint-unknown-globals.test.ts cli/tests/unit/pipeline.test.ts`
Expected: PASS.

- [ ] **Step 6: Full gate, then commit**

```bash
git add cli/src/lint/lua-globals.ts cli/src/lint/unknown-globals.ts cli/src/pipeline.ts cli/tests/unit/lua-globals.test.ts cli/tests/unit/lint-unknown-globals.test.ts
git commit -m "feat(lua): globals a Lua module defines at its top level are known"
```

---

### Task 4: Editor files, `setup` and `dev`

**Files:**
- Create: `template/lua/.gitkeep` (empty)
- Modify: `template/.luarc.json`, `cli/src/editor/scaffold.ts`, `cli/src/commands/setup.ts`, `cli/src/commands/dev.ts`
- Regenerate: `cli/src/embedded/template.ts` (`deno task gen`)
- Test: `cli/tests/unit/editor-scaffold.test.ts`, `cli/tests/unit/dev.test.ts`

**Interfaces:**
- Produces: `mergeLuarc(root: string, files?: ReadonlyArray<{ path: string; base64: string }>):
  Promise<string[] | undefined>` in `cli/src/editor/scaffold.ts` (the entries it added; `undefined` when
  `.luarc.json` is not a JSON object). Plan 4b adds `.moonwell/lua` to the template's `workspace.library` and relies
  on this merging it into older projects.

- [ ] **Step 1: Write the failing tests**

Add to `cli/tests/unit/editor-scaffold.test.ts` (import `mergeLuarc` next to `addEditorFiles`):

```ts
Deno.test("mergeLuarc adds the template's missing runtime.path and workspace.library entries, keeping the rest", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-luarc-" });
  try {
    await Deno.writeTextFile(
      join(root, ".luarc.json"),
      JSON.stringify({
        "runtime.path": ["src/?.lua"],
        "workspace.library": [".moonwell/types", "extra"],
        "diagnostics.globals": ["X"],
      }),
    );
    assertEquals(await mergeLuarc(root), ["src/?/init.lua", "lua/?.lua", "lua/?/init.lua"]);
    const config = JSON.parse(await Deno.readTextFile(join(root, ".luarc.json")));
    assertEquals(config["runtime.path"], ["src/?.lua", "src/?/init.lua", "lua/?.lua", "lua/?/init.lua"]);
    assertEquals(config["workspace.library"], [".moonwell/types", "extra"]);
    assertEquals(config["diagnostics.globals"], ["X"]);
    assertEquals(await mergeLuarc(root), []);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("mergeLuarc leaves a .luarc.json that is not a JSON object alone", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-luarc-" });
  try {
    const text = '// a comment\n{ "runtime.path": [] }\n';
    await Deno.writeTextFile(join(root, ".luarc.json"), text);
    assertEquals(await mergeLuarc(root), undefined);
    assertEquals(await Deno.readTextFile(join(root, ".luarc.json")), text);
    await Deno.remove(join(root, ".luarc.json"));
    assertEquals(await mergeLuarc(root), [], "no file: nothing to merge");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

(If `editor-scaffold.test.ts` does not import `join`, import it from `@std/path`.)

In `cli/tests/unit/dev.test.ts`, the `isRelevantChange` test gains:

```ts
  assertEquals(check("lua", "tools", "init.lua"), true);
  assertEquals(check("lua", "notes.txt"), false);
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/editor-scaffold.test.ts cli/tests/unit/dev.test.ts`
Expected: FAIL: `mergeLuarc` is not exported; `lua/` changes are not relevant.

- [ ] **Step 3: The template**

`template/.luarc.json`'s `runtime.path` becomes
`["src/?.lua", "src/?/init.lua", "lua/?.lua", "lua/?/init.lua"]` (nothing else changes). Create the empty file
`template/lua/.gitkeep`. Run `deno task gen`.

- [ ] **Step 4: `mergeLuarc` (`cli/src/editor/scaffold.ts`)**

```ts
/** The .luarc.json arrays setup keeps up to date in projects made by an older Moonwell (spec §3.5). */
const LUARC_ARRAYS = ["runtime.path", "workspace.library"] as const;

/**
 * Adds the template's `runtime.path` and `workspace.library` entries that the project's .luarc.json lacks, keeping every
 * other key and value, and rewrites it as formatted JSON when it adds any (spec §3.5). Returns the entries it added, or
 * `undefined` when the file is not a JSON object (it is then left alone). A missing file adds nothing.
 */
export async function mergeLuarc(
  root: string,
  files: ReadonlyArray<{ path: string; base64: string }> = TEMPLATE_FILES,
): Promise<string[] | undefined> {
  const path = join(root, ".luarc.json");
  if (!(await exists(path))) return [];
  const file = files.find((entry) => entry.path === ".luarc.json");
  if (!file) throw new Error("The embedded template has no .luarc.json.");
  const template = JSON.parse(new TextDecoder().decode(decodeBase64(file.base64))) as Record<string, string[]>;
  let config: unknown;
  try {
    config = JSON.parse(await Deno.readTextFile(path));
  } catch {
    return undefined;
  }
  if (config === null || typeof config !== "object" || Array.isArray(config)) return undefined;
  const settings = config as Record<string, unknown>;
  const added: string[] = [];
  for (const key of LUARC_ARRAYS) {
    const current = settings[key];
    if (current === undefined) {
      settings[key] = [...template[key]];
      added.push(...template[key]);
    } else if (Array.isArray(current)) {
      for (const entry of template[key]) {
        if (current.includes(entry)) continue;
        current.push(entry);
        added.push(entry);
      }
    }
  }
  if (added.length > 0) await Deno.writeTextFile(path, `${JSON.stringify(settings, null, 2)}\n`);
  return added;
}
```

- [ ] **Step 5: Call it from `setup` (`cli/src/commands/setup.ts`)**

After the `addEditorFiles` loop, add:

```ts
  const merged = await mergeLuarc(ctx.root);
  if (merged === undefined) {
    ctx.logger.warn(
      ".luarc.json is not plain JSON, so setup left it alone. Add Moonwell's runtime.path entries (lua/?.lua, " +
        "lua/?/init.lua) to it yourself.",
    );
  } else if (merged.length > 0) {
    ctx.logger.info(`Added ${merged.join(", ")} to .luarc.json.`);
  }
```

and mention it in the function's doc comment ("adds the `.luarc.json` entries an older project lacks").

- [ ] **Step 6: `dev` watches `lua/` (`cli/src/commands/dev.ts`)**

In `isRelevantChange`, after the `src/` line, add `if (rel.startsWith("lua/")) return rel.endsWith(".lua");`. In
`dev`, the optional folders become `["assets", "objects", "lua"]`, and the doc comment of `isRelevantChange` and the
"Watching" message follow automatically from `watched`.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/editor-scaffold.test.ts cli/tests/unit/dev.test.ts cli/tests/unit/embedded.test.ts`
Expected: PASS.

- [ ] **Step 8: Full gate, then commit**

```bash
git add template/.luarc.json template/lua/.gitkeep cli/src/embedded/template.ts cli/src/editor/scaffold.ts cli/src/commands/setup.ts cli/src/commands/dev.ts cli/tests/unit/editor-scaffold.test.ts cli/tests/unit/dev.test.ts
git commit -m "feat(lua): lua/ in new projects, .luarc.json and dev; setup adds the paths to older projects"
```

---

### Task 5: End-to-end tests

**Files:**
- Create: `cli/tests/e2e/lua.test.ts`

**Interfaces:**
- Consumes: Tasks 1–4 through the CLI (`init --link`, `check`, `build`).

- [ ] **Step 1: Write the tests**

```ts
import { assert, assertEquals, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
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

async function builtScript(project: string): Promise<string> {
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  const archive = openMpq(await Deno.readFile(join(project, "dist", "bin", "map.w3x")));
  return new TextDecoder().decode(await archive.read("war3map.lua"));
}

Deno.test("a new project has lua/, and build bundles the Lua modules main.yue requires", async () => {
  const project = await newProject();
  assert(await exists(join(project, "lua", ".gitkeep")));
  await Deno.mkdir(join(project, "lua", "tools"), { recursive: true });
  await Deno.writeTextFile(
    join(project, "lua", "tools", "init.lua"),
    'local M = {}\nfunction M.greet(name)\n  return "Hello, " .. name\nend\nreturn M\n',
  );
  await Deno.writeTextFile(join(project, "lua", "counter.lua"), "Count = 0\nfunction CountUp()\n  Count = Count + 1\nend\n");
  await Deno.writeTextFile(join(project, "lua", "unused.lua"), "Unused = true\n");
  const main = join(project, "src", "main.yue");
  await Deno.writeTextFile(
    main,
    `${await Deno.readTextFile(main)}\nimport "tools"\nrequire "counter"\nCountUp!\nprint tools.greet "Moonwell"\n`,
  );
  const lua = await builtScript(project);
  assertStringIncludes(lua, '__mw.define("tools", function(...)');
  assertStringIncludes(lua, '__mw.define("counter", function(...)');
  assertStringIncludes(lua, '"tools", "lua/tools/init.lua"}');
  assertEquals(lua.includes("Unused = true"), false, "a Lua module nothing requires is not bundled");
});

Deno.test("check refuses a module name that src/ and lua/ both define", async () => {
  const project = await newProject();
  await Deno.writeTextFile(join(project, "lua", "main.lua"), "return {}\n");
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(checked.text, "error: lua/main.lua › Module main is defined by src/main.yue and lua/main.lua.");
});

Deno.test("check reports an unknown global the Lua file does not define", async () => {
  const project = await newProject();
  await Deno.writeTextFile(join(project, "lua", "counter.lua"), "local function hidden() end\n");
  const main = join(project, "src", "main.yue");
  await Deno.writeTextFile(main, `${await Deno.readTextFile(main)}\nrequire "counter"\nhidden!\n`);
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(checked.text, "Unknown global hidden.");
});
```

- [ ] **Step 2: Run them**

Run: `deno test -A cli/tests/e2e/lua.test.ts`
Expected: PASS (Tasks 1–4 are in). If one fails, the fault is in an earlier task's code: fix it there, in this task's
commit, and say so in the report.

- [ ] **Step 3: Full gate, then commit**

```bash
git add cli/tests/e2e/lua.test.ts
git commit -m "test(lua): end-to-end Lua modules"
```

---

### Task 6: Documentation

**Files:**
- Modify: `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`

- [ ] **Step 1: README**

Add this section after `## Macros` (before `## Assets`):

````markdown
## Lua modules

Plain Lua goes in `lua/`, next to `src/`. A file there is a module named by its path, like a `.yue` file in `src/`:
`lua/utils/timer.lua` is `utils.timer`, and `lua/tools/init.lua` answers to `tools`. `src/` and `lua/` share one set of
names; a name both define fails the build.

```yue
import "utils.timer" as timer   -- lua/utils/timer.lua
require "counter"               -- a Lua file that defines globals
CountUp!
```

Lua modules are bundled as they are, only when something requires them, and runtime errors in them name the `.lua`
file and line, also in `--minify` builds (Lua modules are not minified). The unknown-global check does not read `.lua`
files, but the globals a Lua file defines at its top level (`function CountUp(`, `Count = 0`) count as known in your
YueScript; globals it only assigns inside functions need `lint.globals`. The editor resolves `lua/` through
`.luarc.json`'s `runtime.path`; `deno task setup` adds the entries to a project made by an older Moonwell.

`src/**/*.lua` stays reserved for the `.lua` files the YueScript extension writes on save, so keep your own Lua in
`lua/`.
````

- [ ] **Step 2: CHANGELOG**

Add at the top, above `## 0.4.0 (2026-09-28)`:

```markdown
## Unreleased

- Lua modules: `.lua` files in `lua/` are modules named by their path, sharing one namespace with `src/` (a name both
  define fails the build), and `x/init.lua` answers to `x`. They are bundled unchanged when required, and runtime
  errors in them name the `.lua` file and line, also in `--minify` builds. Their top-level globals are known to the
  unknown-global check. New projects have a `lua/` folder, and `.luarc.json` resolves it in the editor; `setup` adds
  the entries to older projects' `.luarc.json`.
```

- [ ] **Step 3: CONTRIBUTING**

Insert a new step 12 after step 11 and renumber the old step 12 ("Record the Warcraft III and World Editor versions")
to 13:

```markdown
12. Lua modules and libraries, in another throwaway project from `init --link`: add `lua/greeter.lua` that returns a
    table with a function that prints, and a global-style `lua/counter.lua` that defines a global function
    (`function CountUp() print("counted") end`). Use both from `src/main.yue` (`import "greeter"`, `require "counter"`,
    `CountUp!`). Confirm the editor completes the module's function and the global, `deno task check` passes, and in
    the game (`deno task test`) both print. Then add `error "lua gate"` inside the module's function, run
    `deno task test --minify`, and confirm the game's error names `lua/greeter.lua` and the right line (Lua modules
    keep their lines in minified builds). Remove the error afterwards.
```

- [ ] **Step 4: AGENTS.md**

In "State", after the `Released 0.4.0` bullet's plan bullets, add:

```markdown
- **Plan 4a done, unreleased** (2026-09-28, `docs/superpowers/plans/2026-09-28-moonwell-lua-modules.md`): Lua modules
  in `lua/` (spec `docs/superpowers/specs/2026-09-28-moonwell-lua-libraries-design.md`). `collectModules`
  (`cli/src/bundle/modules.ts`) lists `src/**/*.yue` and `lua/**/*.lua` as one namespace; `moduleLoader` resolves a
  name or `<name>.init`; Lua modules are bundled unchanged and keep their lines in `--minify` builds (per-entry flag in
  the line table); `luaTopLevelGlobals` (`cli/src/lint/lua-globals.ts`) makes their top-level globals known;
  `mergeLuarc` lets `setup` add `.luarc.json` entries to older projects. Its gate (CONTRIBUTING step 12, the Lua
  modules part) has not been run yet.
```

In "Next work", item 1 becomes: "**Plan 4b** (libraries, spec §4): write it from the spec, then run CONTRIBUTING step
12 in full and release 0.5.0."

- [ ] **Step 5: Check and commit**

Run: `deno fmt --check` (fix with `deno fmt <file>` if it reflows).

```bash
git add README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md
git commit -m "docs: Lua modules"
```

---

### Task 7: Maintainer gate, Lua modules

The maintainer runs the Lua modules part of CONTRIBUTING step 12 on Warcraft III Reforged 3.0.0.24268 (this also
answers V3: a global-style Lua file defines its globals when required). The controller can check the editor half first
with the Lua extension's server (`lua-language-server --check`, as AGENTS.md describes). Record the result under a
"Release gate (so far)" heading in `CHANGELOG.md`'s `## Unreleased` section and commit.

## Completion

All seven tasks done, the full gate green, the maintainer's gate recorded. Then Plan 4b.
