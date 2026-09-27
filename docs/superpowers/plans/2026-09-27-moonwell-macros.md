# Moonwell Macros (Plan 3c) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `import "moonwell.macros" as {:$FourCC}` gives gameplay code `$FourCC "hfoo"`, which compiles to the integer
`1751543663` and fails the compile at the call for anything but a 4-character string literal.

**Architecture:** The macro module is a YueScript file in the repo (`cli/runtime/macros.yue`), embedded into the CLI by
`deno task gen`. `check`, `build`, `test`, `dev` and `setup` write it to `.moonwell/yue/moonwell/macros.yue` with the
editor declarations. Every `yue` run Moonwell makes (compile and `-g`) gets `--path <root>/.moonwell/yue/?.lua`, and
the module's hash joins both cache keys. The editor already finds the module through `yueconfig.yue`'s `include`.

**Tech Stack:** Deno 2.9+, TypeScript, existing `jsr:@std/*` imports, YueScript 0.34.2.

**Spec:** `docs/superpowers/specs/2026-09-27-moonwell-editor-dx-design.md` (approved 2026-09-27). This plan covers §6
and the Plan 3c rows of §§8, 9 and 12. Plans 3a and 3b are done; 0.4.0 is released after this plan.

## Global Constraints

- No Node.js: no `package.json`, `node_modules`, `npm:` or `node:` specifiers. Only `jsr:@std/*` from the existing
  import maps.
- File-system code is async. Expected failures throw `MoonwellError` (`cli/src/shared/errors.ts`) with `file` and
  `hint`; anything else is reported as an internal error, so user mistakes must never reach it.
- `deno fmt` width 120 (`template/`, `docs/`, `cli/src/embedded/` excluded); `deno fmt --check` and `deno lint` clean.
- After changing `template/`, `cli/runtime/moonwell.lua` or the new `cli/runtime/macros.yue`, run `deno task gen`.
  Nothing stray in `template/`: every file there is embedded.
- Write files containing backslashes with a file-editing tool, never a shell heredoc or `sed`.
- The macro's error text is exactly the spec's (§6):
  `$FourCC needs a string literal of exactly 4 characters, such as "hfoo".`
- `$FourCC("hfoo")` and `$FourCC "hfoo"` compile to `1751543663`; `$FourCC "Hpal"` to `1215324524`.
- Every task: failing test first, then code, then the full gate (below), then one commit on `main` ending with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Do not push; the maintainer pushes. Do not bump versions.

**Full gate (before every commit):**

```bash
deno task check && deno task lint && deno fmt --check && deno task test && deno task test:runtime && deno task test:pkl && deno task test:e2e
```

## Facts this plan relies on (checked 2026-09-27 with yue 0.34.2 on Windows)

- With the macro module at `<root>/.moonwell/yue/moonwell/macros.yue`, `yue --target=5.3 -r --path
  <root>\.moonwell\yue\?.lua -o out.lua src/main.yue` finds it (an absolute Windows path works). `-m` (minify) works
  the same. `--path` sits before the source file.
- The module below expands `$FourCC "hfoo"`, `$FourCC 'hfoo'` and `$FourCC("hfoo")` to `1751543663` and
  `$FourCC("Hpal")` to `1215324524`. `$FourCC!`, `$FourCC x`, `"hfo"`, `"hfooo"`, `1234`, `"h\oo"`, `"héé"` and
  `[[hfoo]]` all fail. The compiled Lua has no `require` of the macro module and, in `-r` mode, keeps line numbers.
- A failing macro prints `<line>: failed to expand macro: (macro FourCC):18: <message>` (the `18` is a line of the
  macro module), then an excerpt with a caret at the call, and exits 1.
- `yue -g --path ... src/main.yue` on a file that uses `$FourCC` lists neither `FourCC` nor anything for the macro
  import: `print $FourCC "hfoo"` lists only `print`. Without `--path`, both compiling and `-g` fail with
  `module 'moonwell.macros' not found`.
- A backslash in the macro module's own source (for example in a Lua pattern) broke loading the macro
  (`invalid escape sequence`), so the module avoids backslashes.

## Decisions this plan adds to the spec

- **The macro source is a real file**, `cli/runtime/macros.yue`, embedded by `deno task gen` as `MACROS_YUE` in
  `cli/src/embedded/macros.ts` (like `cli/runtime/moonwell.lua`), so the real compiler can test it.
- **Accepted input:** one single- or double-quoted string literal of exactly four printable ASCII characters (bytes 32
  to 126), without a backslash or its own quote character. Rawcodes are ASCII; anything else is refused with the
  spec's message.
- **The macro's line number is dropped from the headline.** `compileError` removes the
  `failed to expand macro: (macro <name>):<n>: ` prefix from the first line, so the error reads
  `error: src/main.yue:13 › $FourCC needs a string literal of exactly 4 characters, such as "hfoo".`; the excerpt
  below it keeps the compiler's full text.
- **Cache keys.** The compile key becomes `<yue>|<minify or rewrite>|<macro hash>`; the `yue -g` key becomes
  `<yue>|<macro hash>`. Both caches are therefore rebuilt once after upgrading.
- **`macros` is optional** in `compileSources`, `listGlobalUses` and `checkUnknownGlobals`, so unit tests with stand-in
  compilers keep their argument lists; `compileProject` always passes it.
- **`refreshEditorFiles` writes the macro module** (`.moonwell/yue/moonwell/macros.yue`) with the four declaration
  files: its list becomes project-relative paths. It already runs before `compileProject` in `check`, `build`, `test`
  and `dev`, and in `setup`.
- **The template's Footman** is created at `-45, -650`, beside the Captain at `45, -650`, as the last line of
  `on_main`; the Captain's `CreateUnit` moves from line 8 to line 9 of `src/main.yue`.

## File Structure

| File | Responsibility |
| --- | --- |
| `cli/runtime/macros.yue` (new) | The macro module: `export macro FourCC` |
| `tools/gen.ts` | embeds it as `cli/src/embedded/macros.ts` (`MACROS_YUE`) |
| `cli/src/yue/macros.ts` (new) | `MACROS_FILE`, `MacroSearch`, `macroSearch(root)`, `macroPathArgs(macros)` |
| `cli/src/editor/refresh.ts` | also writes `MACROS_FILE` |
| `cli/src/yue/compile.ts` | `--path` and the macro hash; the macro prefix dropped from compile errors |
| `cli/src/lint/uses.ts`, `cli/src/lint/unknown-globals.ts` | `--path` and the macro hash for `yue -g` |
| `cli/src/pipeline.ts` | `compileProject` passes `macroSearch(ctx.root)` to both |
| `template/src/main.yue` | the Footman made with `$FourCC` |
| Tests | `cli/tests/unit/{embedded,editor-refresh,lint-uses,pipeline}.test.ts`, `cli/tests/yue/{macros,compile,uses}.test.ts`, `cli/tests/e2e/{project,lint}.test.ts` |
| Docs | `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md` |

---

### Task 1: The macro module

**Files:**
- Create: `cli/runtime/macros.yue`, `cli/src/yue/macros.ts`
- Modify: `tools/gen.ts`
- Generate: `cli/src/embedded/macros.ts` (`deno task gen`)
- Test: `cli/tests/yue/macros.test.ts` (new), `cli/tests/unit/embedded.test.ts`

**Interfaces:**
- Produces: `MACROS_YUE: string` (`cli/src/embedded/macros.ts`); in `cli/src/yue/macros.ts`:
  `MACROS_FILE = ".moonwell/yue/moonwell/macros.yue"`, `interface MacroSearch { path: string; hash: string }`,
  `macroSearch(root: string): Promise<MacroSearch>`, `macroPathArgs(macros: MacroSearch | undefined): string[]`.

- [ ] **Step 1: Write the failing tests**

Add to `cli/tests/unit/embedded.test.ts`, next to the runtime freshness test:

```ts
Deno.test("the embedded macro module is up to date (run `deno task gen`)", async () => {
  const path = "cli/src/embedded/macros.ts";
  const expected = (await renderEmbedded()).get(path);
  assert(await Deno.readTextFile(join(REPO, path)) === expected, `${path} is stale: run \`deno task gen\`.`);
});
```

Create `cli/tests/yue/macros.test.ts` (real compiler; `-r` as builds use):

```ts
import { assert, assertEquals, assertStringIncludes } from "@std/assert";
import { dirname, join } from "@std/path";
import { MACROS_YUE } from "../../src/embedded/macros.ts";
import { runProcess } from "../../src/shared/process.ts";
import { MACROS_FILE, macroPathArgs, macroSearch } from "../../src/yue/macros.ts";
import { testYue } from "../support/yue.ts";

const MESSAGE = '$FourCC needs a string literal of exactly 4 characters, such as "hfoo".';

/** Compiles `print <call>` after the macro import in a temp project; returns the Lua, or the compiler's output. */
async function compile(call: string): Promise<{ ok: boolean; text: string }> {
  const yue = await testYue();
  const root = await Deno.makeTempDir({ prefix: "moonwell-macros-" });
  try {
    await Deno.mkdir(dirname(join(root, MACROS_FILE)), { recursive: true });
    await Deno.writeTextFile(join(root, MACROS_FILE), MACROS_YUE);
    await Deno.mkdir(join(root, "src"));
    const source = join(root, "src", "main.yue");
    await Deno.writeTextFile(source, `import "moonwell.macros" as {:$FourCC}\nprint ${call}\n`);
    const output = join(root, "main.lua");
    const args = ["--target=5.3", "-r", "-o", output, ...macroPathArgs(await macroSearch(root)), source];
    const result = await runProcess(yue, args);
    if (result.code !== 0) return { ok: false, text: `${result.stdout}\n${result.stderr}` };
    return { ok: true, text: await Deno.readTextFile(output) };
  } finally {
    await Deno.remove(root, { recursive: true });
  }
}

Deno.test("$FourCC turns a 4-character string literal into the rawcode's integer", async () => {
  for (const call of ['$FourCC "hfoo"', "$FourCC 'hfoo'", '$FourCC("hfoo")']) {
    const result = await compile(call);
    assert(result.ok, result.text);
    assertStringIncludes(result.text, "1751543663");
    assertEquals(result.text.includes("moonwell.macros"), false, "the macro import leaves nothing in the Lua");
  }
  const hero = await compile('$FourCC "Hpal"');
  assertStringIncludes(hero.text, "1215324524");
});

Deno.test("$FourCC refuses anything but a 4-character string literal", async () => {
  for (const call of ["$FourCC!", "$FourCC x", '$FourCC "hfo"', '$FourCC "hfooo"', "$FourCC 1234", '$FourCC "h\\oo"', '$FourCC "héé"', "$FourCC [[hfoo]]"]) {
    const result = await compile(call);
    assertEquals(result.ok, false, call);
    assertStringIncludes(result.text, MESSAGE, call);
  }
});

Deno.test("macroSearch points yue at .moonwell/yue and hashes the module", async () => {
  const search = await macroSearch("/project");
  assertEquals(search.path, join("/project", ".moonwell", "yue", "?.lua"));
  assertEquals(search.hash.length, 64);
  assertEquals(macroPathArgs(search), ["--path", search.path]);
  assertEquals(macroPathArgs(undefined), []);
});
```

(The `'$FourCC "h\\oo"'` string in TypeScript is the YueScript source `$FourCC "h\oo"`. `deno fmt` will wrap the long
array.)

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/yue/macros.test.ts cli/tests/unit/embedded.test.ts`
Expected: FAIL: `cli/src/embedded/macros.ts` and `cli/src/yue/macros.ts` not found.

- [ ] **Step 3: Write `cli/runtime/macros.yue`**

Exactly this (it contains no backslash; keep it that way, see Facts):

```yue
-- Moonwell's compile-time macros, imported with `import "moonwell.macros" as {:$FourCC}`. Moonwell writes this file to
-- .moonwell/yue/moonwell/macros.yue, where the compiler and the editor's YueScript extension find it. It adds nothing
-- to the map.

-- $FourCC "hfoo" is a rawcode as the integer the game uses, 1751543663: its four characters' bytes, big-endian.
export macro FourCC = (code = "") ->
  quote = code\sub 1, 1
  text = code\sub 2, -2
  valid = (quote == '"' or quote == "'") and code\sub(-1) == quote and #text == 4
  if valid
    for i = 1, 4
      byte = text\byte i
      -- Printable ASCII only: a backslash starts an escape sequence, and the quote ends the string.
      valid = false if byte < 32 or byte > 126 or byte == 92 or byte == quote\byte!
  unless valid
    error '$FourCC needs a string literal of exactly 4 characters, such as "hfoo".'
  value = 0
  for i = 1, 4
    value = value * 256 + text\byte i
  tostring value
```

- [ ] **Step 4: Embed it in `tools/gen.ts`**

After the `runtime.ts` entry in `renderEmbedded`, add:

```ts
  const macros = await Deno.readTextFile(join(repo, "cli", "runtime", "macros.yue"));
  out.set("cli/src/embedded/macros.ts", `${HEADER}export const MACROS_YUE: string = ${JSON.stringify(macros)};\n`);
```

Run `deno task gen`.

- [ ] **Step 5: Create `cli/src/yue/macros.ts`**

```ts
import { join } from "@std/path";
import { MACROS_YUE } from "../embedded/macros.ts";
import { sha256Hex } from "../shared/fs.ts";

/** Where `check`, `build`, `test`, `dev` and `setup` write the macro module, relative to the project root (spec §6). */
export const MACROS_FILE = ".moonwell/yue/moonwell/macros.yue";

/** How a `yue` run finds `import "moonwell.macros"`. */
export interface MacroSearch {
  /** The compiler's `--path`: yue tries each `.lua` pattern with `.yue`, so this finds `moonwell/macros.yue`. */
  path: string;
  /** SHA-256 of the macro module. It joins the compile and `yue -g` cache keys, so a changed macro redoes every file. */
  hash: string;
}

export async function macroSearch(root: string): Promise<MacroSearch> {
  return {
    path: join(root, ".moonwell", "yue", "?.lua"),
    hash: await sha256Hex(new TextEncoder().encode(MACROS_YUE)),
  };
}

/** The `--path` arguments of a `yue` run; none without `macros`. They go before the source file. */
export function macroPathArgs(macros: MacroSearch | undefined): string[] {
  return macros === undefined ? [] : ["--path", macros.path];
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `deno test -A cli/tests/yue/macros.test.ts cli/tests/unit/embedded.test.ts`
Expected: PASS.

- [ ] **Step 7: Full gate, then commit**

```bash
git add cli/runtime/macros.yue tools/gen.ts cli/src/embedded/macros.ts cli/src/yue/macros.ts cli/tests/yue/macros.test.ts cli/tests/unit/embedded.test.ts
git commit -m "feat(macros): the moonwell.macros module with \$FourCC

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: Every `yue` run finds the macros

**Files:**
- Modify: `cli/src/editor/refresh.ts`, `cli/src/yue/compile.ts`, `cli/src/lint/uses.ts`,
  `cli/src/lint/unknown-globals.ts`, `cli/src/pipeline.ts`
- Test: `cli/tests/unit/editor-refresh.test.ts`, `cli/tests/unit/lint-uses.test.ts`,
  `cli/tests/unit/pipeline.test.ts`, `cli/tests/yue/compile.test.ts`, `cli/tests/yue/uses.test.ts`

**Interfaces:**
- Consumes (Task 1): `MACROS_YUE`; `MACROS_FILE`, `MacroSearch`, `macroSearch(root)`, `macroPathArgs(macros)`.
- Produces: `compileSources(options)` gains `macros?: MacroSearch`; `listGlobalUses(options)` gains
  `macros?: MacroSearch`; `checkUnknownGlobals(ctx, project, compiled, natives?)`'s `compiled` gains
  `macros?: MacroSearch`; `refreshEditorFiles` also writes and returns `MACROS_FILE`.

- [ ] **Step 1: Write the failing tests**

In `cli/tests/unit/editor-refresh.test.ts`, import `MACROS_YUE` from `../../src/embedded/macros.ts`, rename the first
test to `"refreshEditorFiles writes the declarations and the macro module, then only what changed"`, and make its first
expectation:

```ts
  assertEquals(await refreshEditorFiles(root, inputs), [
    ".moonwell/types/natives.d.lua",
    ".moonwell/types/moonwell.d.lua",
    ".moonwell/types/objects.d.lua",
    ".moonwell/types/map.d.lua",
    ".moonwell/yue/moonwell/macros.yue",
  ]);
  assertEquals(await Deno.readTextFile(join(root, ".moonwell", "yue", "moonwell", "macros.yue")), MACROS_YUE);
```

(the rest of the test is unchanged).

Add to `cli/tests/unit/lint-uses.test.ts`:

```ts
Deno.test("listGlobalUses gives yue -g the macro path and keys its cache on the macro module", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    const calls: string[][] = [];
    const run: Runner = (_command, args) => {
      calls.push(args);
      return Promise.resolve(ok("print 1 1\n"));
    };
    const hashes = { "main.yue": "h1" };
    const macros = { path: join(root, ".moonwell", "yue", "?.lua"), hash: "m1" };
    await listGlobalUses({ yue: "yue", root, hashes, run, macros });
    assertEquals(calls, [["-g", "--path", macros.path, join(root, "src", "main.yue")]]);
    await listGlobalUses({ yue: "yue", root, hashes, run, macros });
    assertEquals(calls.length, 1, "unchanged: cached");
    await listGlobalUses({ yue: "yue", root, hashes, run, macros: { ...macros, hash: "m2" } });
    assertEquals(calls.length, 2, "a changed macro module lists every file again");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

In `cli/tests/unit/pipeline.test.ts`, import `MACROS_FILE` from `../../src/yue/macros.ts`. In `stageProject`, add
`const calls: Array<{ args: string[]; macros: boolean }> = [];`, make the stand-in's first line
`calls.push({ args, macros: await exists(join(root, ...MACROS_FILE.split("/"))) });` (before its `-g` branch), and
return `calls` with the other values. Add:

```ts
Deno.test("prepareStage writes the macro module before any yue run and gives every run its path", async () => {
  const { root, ctx, project, calls } = await stageProject(emptyObjects());
  try {
    await prepareStage(ctx, project, {});
    const path = join(root, ".moonwell", "yue", "?.lua");
    assertEquals(calls.map((call) => call.args[0] === "-g" ? "-g" : "compile"), ["compile", "-g"]);
    for (const call of calls) {
      assert(call.macros, "the macro module exists before yue runs");
      const at = call.args.indexOf("--path");
      assertEquals(call.args[at + 1], path);
      assertEquals(at + 2, call.args.length - 1, "--path comes right before the source file");
    }
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

In `cli/tests/yue/compile.test.ts`, import `MACROS_YUE` (`../../src/embedded/macros.ts`) and `MACROS_FILE`,
`macroSearch` (`../../src/yue/macros.ts`), and add:

```ts
Deno.test("compileSources expands $FourCC through the macro module", async () => {
  const yue = await testYue();
  const root = await project({
    [MACROS_FILE]: MACROS_YUE,
    "src/main.yue": 'import "moonwell.macros" as {:$FourCC}\nexport footman = $FourCC "hfoo"\n',
  });
  const output = await compileSources({ yue, root, minify: false, macros: await macroSearch(root) });
  assertStringIncludes(output.load("main")!.source, "1751543663");
});

Deno.test("a changed macro module recompiles every file", async () => {
  const yue = await testYue();
  const root = await project({ [MACROS_FILE]: MACROS_YUE, "src/a.yue": "export x = 1\n", "src/b.yue": "export y = 2\n" });
  const macros = await macroSearch(root);
  await compileSources({ yue, root, minify: false, macros });
  const unchanged = countingRunner();
  await compileSources({ yue, root, minify: false, macros, run: unchanged.run });
  assertEquals(unchanged.compiled.length, 0);
  const changed = countingRunner();
  await compileSources({ yue, root, minify: false, macros: { ...macros, hash: "another" }, run: changed.run });
  assertEquals(changed.compiled.length, 2);
});

Deno.test("a failed macro names the file and line, with the macro's own message", async () => {
  const yue = await testYue();
  const root = await project({
    [MACROS_FILE]: MACROS_YUE,
    "src/main.yue": 'import "moonwell.macros" as {:$FourCC}\nx = 1\ny = $FourCC "hfo"\n',
  });
  const error = await assertRejects(
    () => compileSources({ yue, root, minify: false, macros: undefined }).then(() => undefined),
    MoonwellError,
  );
  assertStringIncludes(error.message, "moonwell.macros", "without --path the module is not found");
  const failed = await assertRejects(
    async () => compileSources({ yue, root, minify: false, macros: await macroSearch(root) }),
    MoonwellError,
  );
  assertEquals([failed.file, failed.line], ["src/main.yue", 3]);
  assert(
    failed.message.startsWith('$FourCC needs a string literal of exactly 4 characters, such as "hfoo".\n'),
    failed.message,
  );
});
```

In `cli/tests/yue/uses.test.ts`, import `MACROS_YUE`, `MACROS_FILE` and `macroSearch` as above, and add:

```ts
Deno.test("yue -g with the macro path lists no global for a $FourCC call", async () => {
  const yue = await testYue();
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    await Deno.mkdir(join(root, ".moonwell", "yue", "moonwell"), { recursive: true });
    await Deno.writeTextFile(join(root, ...MACROS_FILE.split("/")), MACROS_YUE);
    await Deno.mkdir(join(root, "src"));
    await Deno.writeTextFile(join(root, "src", "main.yue"), 'import "moonwell.macros" as {:$FourCC}\nprint $FourCC "hfoo"\n');
    const uses = await listGlobalUses({ yue, root, hashes: { "main.yue": "h" }, macros: await macroSearch(root) });
    assertEquals(uses, { "main.yue": [{ name: "print", line: 2, column: 1 }] });
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/editor-refresh.test.ts cli/tests/unit/lint-uses.test.ts cli/tests/unit/pipeline.test.ts cli/tests/yue/compile.test.ts cli/tests/yue/uses.test.ts`
Expected: FAIL: the macro module is not written, `macros` is not an option, no `--path` is passed, and the macro
error's message starts with `failed to expand macro:`.

- [ ] **Step 3: Write the macro module in `refreshEditorFiles` (`cli/src/editor/refresh.ts`)**

Import `MACROS_YUE` from `../embedded/macros.ts` and `MACROS_FILE` from `../yue/macros.ts`. Make the file list
project-relative and add the module:

```ts
  const files: Array<[string, string]> = [
    [`${EDITOR_TYPES_DIR}/natives.d.lua`, renderNativesDeclarations(inputs.natives ?? await loadNatives())],
    [`${EDITOR_TYPES_DIR}/moonwell.d.lua`, RUNTIME_DECLARATIONS],
    [`${EDITOR_TYPES_DIR}/objects.d.lua`, renderObjectDeclarations(inputs.objects)],
    [`${EDITOR_TYPES_DIR}/map.d.lua`, renderMapDeclarations(script === undefined ? undefined : readMapGlobals(script), source)],
    [MACROS_FILE, MACROS_YUE],
  ];
  const written: string[] = [];
  for (const [path, text] of files) {
```

(the loop body is unchanged apart from dropping its `const path = ...` line). Update the doc comment to: "Brings
`.moonwell/` under `root` up to date: the editor declarations in `.moonwell/types/` and the macro module (spec §§4.2,
6). Each file is written only when its content differs. Returns the POSIX paths it wrote."

- [ ] **Step 4: `--path` and the macro hash in `cli/src/yue/compile.ts`**

Import `type MacroSearch, macroPathArgs` from `./macros.ts`. Add to the options of `compileSources`:

```ts
  /** Where `import "moonwell.macros"` is found; every yue run gets its `--path` (spec §6). */
  macros?: MacroSearch;
```

make the settings key `` `${options.yue}|${options.minify ? "minify" : "rewrite"}|${options.macros?.hash ?? "no macros"}` ``,
and the compile arguments
`["--target=5.3", mode, "-o", output, ...macroPathArgs(options.macros), join(srcDir, file)]`.

Replace `compileError` with:

```ts
/**
 * yue prints "Failed to compile: <file>", then "<line>: <message>" and a source excerpt. A failing macro's message
 * starts with "failed to expand macro: (macro <name>):<line>: ", a line of the macro module rather than of the file, so
 * the first line of the error drops it; the excerpt keeps the compiler's full text.
 */
export function compileError(file: string, output: string): MoonwellError {
  const detail = output.split(/\r?\n/).filter((line) => !line.startsWith("Failed to compile")).join("\n").trim();
  const match = /^(\d+): (.+)$/m.exec(output);
  const headline = match?.[2].replace(/^failed to expand macro: \(macro [^)]*\):\d+: /, "");
  return new MoonwellError(match ? `${headline}\n${detail}` : detail || "YueScript compilation failed.", {
    file,
    line: match ? Number(match[1]) : undefined,
  });
}
```

- [ ] **Step 5: `--path` and the macro hash for `yue -g`**

In `cli/src/lint/uses.ts`, import `type MacroSearch, macroPathArgs` from `../yue/macros.ts`; add
`macros?: MacroSearch;` to `listGlobalUses`'s options; compute
`const settings = options.macros === undefined ? options.yue : `${options.yue}|${options.macros.hash}`;` and use it
where `options.yue` is compared with and written as the cache's `settings`; run
`["-g", ...macroPathArgs(options.macros), join(options.root, "src", ...file.split("/"))]`. Update the `settings` doc
comment in `Cache` to "The compiler path and the macro module's hash; either changing lists every file again."

In `cli/src/lint/unknown-globals.ts`, `checkUnknownGlobals`'s `compiled` parameter gains `macros?: MacroSearch`
(import the type from `../yue/macros.ts`), passed on as `macros: compiled.macros` to `listGlobalUses`.

- [ ] **Step 6: Pass it from `compileProject` (`cli/src/pipeline.ts`)**

Import `macroSearch` from `./yue/macros.ts`. After `const yue = await ensureYue(...)`, add
`const macros = await macroSearch(ctx.root);`, pass `macros` to `compileSources`, and pass
`{ yue, hashes: output.hashes, sources: output.sources, macros }` to `checkUnknownGlobals`.

- [ ] **Step 7: Run the tests to verify they pass**

Run the Step 2 command. Expected: PASS. Then the full gate: the e2e tests still pass (the template does not use a
macro yet, and `check` writes the module before compiling).

- [ ] **Step 8: Commit**

```bash
git add cli/src/editor/refresh.ts cli/src/yue/compile.ts cli/src/lint/uses.ts cli/src/lint/unknown-globals.ts cli/src/pipeline.ts cli/tests/unit/editor-refresh.test.ts cli/tests/unit/lint-uses.test.ts cli/tests/unit/pipeline.test.ts cli/tests/yue/compile.test.ts cli/tests/yue/uses.test.ts
git commit -m "feat(macros): every yue run finds moonwell.macros, and a changed macro redoes the caches

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: The template's Footman

**Files:**
- Modify: `template/src/main.yue`, `cli/tests/e2e/project.test.ts`, `cli/tests/e2e/lint.test.ts`
- Regenerate: `cli/src/embedded/template.ts` (`deno task gen`)

**Interfaces:**
- Consumes: Tasks 1–2 through the CLI (`check`, `build`).

- [ ] **Step 1: Write the failing e2e tests**

In `cli/tests/e2e/project.test.ts`:

- `"init → build produces an archive with the injected bundle"` gains
  `assertStringIncludes(lua, "1751543663"); // the template's $FourCC "hfoo"`.
- `"check writes the editor declarations for the template project"` gains
  `assertStringIncludes(await Deno.readTextFile(join(project, ".moonwell", "yue", "moonwell", "macros.yue")), "export macro FourCC");`.
- Add:

```ts
Deno.test("check reports a $FourCC that is not a 4-character literal at its file and line", async () => {
  const project = await newProject();
  const main = join(project, "src", "main.yue");
  const text = await Deno.readTextFile(main);
  assert(text.includes('$FourCC("hfoo")'), text);
  await Deno.writeTextFile(main, text.replace('$FourCC("hfoo")', '$FourCC("hfo")'));
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(
    checked.text,
    'error: src/main.yue:13 › $FourCC needs a string literal of exactly 4 characters, such as "hfoo".',
  );
});
```

In `cli/tests/e2e/lint.test.ts`, the Captain's `CreateUnit` moves to line 9: change `TYPO`'s position from `8:10` to
`9:10`.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/e2e/project.test.ts cli/tests/e2e/lint.test.ts`
Expected: FAIL: the archive has no `1751543663`, the template has no `$FourCC("hfoo")`, and the typo is still at
`8:10`.

- [ ] **Step 3: Use `$FourCC` in `template/src/main.yue`**

Replace the file with (lines 1–11 keep the Captain; the macro import is line 2; the Footman is line 13):

```yue
import "moonwell" as mw
import "moonwell.macros" as {:$FourCC}
import "generated.objects" as objects

mw.on_main ->
  print "Moonwell is running."
  -- The Captain (objects/units.pkl) just north of the preplaced heroes, changing player colour every second.
  -- Ally colour mode (Alt+A) overrides player colours; turn it off to see the change.
  unit = CreateUnit Player(0), objects.units.captain, 45, -650, 270
  TimerStart CreateTimer!, 1.0, true, ->
    SetUnitColor unit, GetPlayerColor Player GetRandomInt 0, bj_MAX_PLAYERS - 1
  -- A standard Footman beside it: $FourCC turns the rawcode into the game's integer when the code compiles.
  CreateUnit Player(0), $FourCC("hfoo"), -45, -650, 270
```

Run `deno task gen`.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `deno test -A cli/tests/e2e/project.test.ts cli/tests/e2e/lint.test.ts`, then the full gate (the embedded-template
test, the objects and settings e2e tests and the Pkl tests all use the template).
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add template/src/main.yue cli/src/embedded/template.ts cli/tests/e2e/project.test.ts cli/tests/e2e/lint.test.ts
git commit -m "feat(template): a standard Footman created with \$FourCC

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Documentation

**Files:**
- Modify: `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`

- [ ] **Step 1: README**

Add this section right after `## Unknown globals` (before `## Assets`):

````markdown
## Macros

Macros run while the code compiles. Import them from `moonwell.macros`:

```yue
import "moonwell.macros" as {:$FourCC}

footman = CreateUnit Player(0), $FourCC("hfoo"), 0, 0, 270
```

`$FourCC "hfoo"` is the rawcode `'hfoo'` as the integer the game's functions take (`1751543663`), written into the
compiled code, so the game never converts it. It takes one string literal of exactly four characters; anything else,
such as a variable or `"hfo"`, fails the compile at that line. For your own objects, use the ids in `generated.objects`
instead.

The macro module is written to `.moonwell/yue/moonwell/macros.yue` by `setup`, `check`, `build`, `test` and `dev`,
where the compiler and the editor's YueScript extension find it. It adds nothing to the map.
````

In the `## Unknown globals` section, change the example's position `src/main.yue:8:10` to `src/main.yue:9:10` (the
template's Captain line moved).

- [ ] **Step 2: CHANGELOG**

Under `## Unreleased`, before the "After upgrading" bullet, add:

```markdown
- Macros: `import "moonwell.macros" as {:$FourCC}` gives gameplay code `$FourCC "hfoo"`, which compiles to the rawcode's
  integer (`1751543663`) and fails the compile on anything but a 4-character string literal. The template creates a
  standard Footman with it next to the Captain.
```

- [ ] **Step 3: CONTRIBUTING**

Insert a new step after step 10 and renumber the old step 11 ("Record the Warcraft III and World Editor versions") to
12:

```markdown
11. Macros and the game's Lua, in the step 10 project: run `deno task test` and confirm a standard Footman stands
    beside the Captain (the template makes it with `$FourCC("hfoo")`). Then replace `src/main.yue` with the Lua probe
    below, run `deno task test`, and confirm the message log (F12) shows
    `missing: collectgarbage dofile loadfile debug io package` and `os: clock date difftime time`, the lists
    `tools/natives/lua-extras.json` records; restore `src/main.yue` afterwards.

    ```yue
    import "moonwell" as mw

    names = {
      "assert", "collectgarbage", "dofile", "error", "getmetatable", "ipairs", "load", "loadfile", "next", "pairs",
      "pcall", "print", "rawequal", "rawget", "rawlen", "rawset", "require", "select", "setmetatable", "tonumber",
      "tostring", "type", "xpcall", "_VERSION", "_G", "coroutine", "debug", "io", "math", "os", "package", "string",
      "table", "utf8", "FourCC", "__jarray"
    }

    mw.on_main ->
      missing = [name for name in *names when _G[name] == nil]
      print "missing: " .. table.concat missing, " "
      for lib in *{"os", "debug", "package"}
        t = _G[lib]
        if type(t) == "table"
          keys = [key for key in pairs t]
          table.sort keys
          print lib .. ": " .. table.concat keys, " "
    ```
```

(Check the list against `tools/natives/lua-extras.json`'s `removed`: every name there that is a global of the probe
list must appear after `missing:`. If `deno fmt` changes the nested code block's indentation, keep what it writes.)

- [ ] **Step 4: AGENTS.md**

- In "Hard rules" → "Generated files", add `cli/runtime/macros.yue` to the list of files after which `deno task gen`
  must run.
- In "State", after the Plan 3b bullet, add:

```markdown
- **Plan 3c done, unreleased** (2026-09-27, `docs/superpowers/plans/2026-09-27-moonwell-macros.md`): macros.
  `cli/runtime/macros.yue` (embedded as `MACROS_YUE`) exports `$FourCC`; `refreshEditorFiles` writes it to
  `.moonwell/yue/moonwell/macros.yue`, and every `yue` run gets `--path <root>/.moonwell/yue/?.lua`
  (`cli/src/yue/macros.ts`), with the module's hash in the compile and `yue -g` cache keys. The macro module must not
  contain a backslash: yue 0.34.2 fails to load such a macro. The template's Footman uses `$FourCC`. Its gate
  (CONTRIBUTING step 11) has not been run yet.
```

- In "Next work, in order", make item 1 the 0.4.0 release: run the whole release gate (CONTRIBUTING), including step
  11, then publish as CONTRIBUTING's "Publishing" section says. Remove the Plan 3c item.

- [ ] **Step 5: Check and commit**

Run: `deno fmt --check`.

```bash
git add README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md
git commit -m "docs: macros

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Maintainer gate, macros

The maintainer runs CONTRIBUTING step 11 on Warcraft III Reforged 3.0.0.24268: the Footman made with `$FourCC` stands
beside the Captain, and the Lua probe's lists match `tools/natives/lua-extras.json`. In the editor, hovering
`$FourCC("hfoo")` in `src/main.yue` shows no error (the extension finds the module through `yueconfig.yue`'s
`include`). Record the result under "Release gate (so far)" in `CHANGELOG.md` and commit.

## Completion

All five tasks committed, the full gate green, the maintainer's gate recorded. Then the 0.4.0 release.
