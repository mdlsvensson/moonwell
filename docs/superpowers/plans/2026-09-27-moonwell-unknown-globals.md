# Moonwell Unknown-Global Check (Plan 3b) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or
> superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `check`, `build`, `test` and `dev` report every global a gameplay file uses that nothing defines, with its
file, line and column and the nearest known name, failing the command or only warning as `lint.unknownGlobals` says.

**Architecture:** After `compileSources`, `compileProject` asks the compiler for each file's globals (`yue -g`), cached
per file hash in `dist/stage/lua/.globals.json`, so only changed files run it again. The known names are rebuilt on
every run from the embedded `natives.json`, the source map's `war3map.lua`, the `global` lines under `src/` and the
manifest's `lint.globals`. Unknown uses become `Problem`s with positions, thrown together as one `ProblemsError` or
logged as warnings.

**Tech Stack:** Deno 2.9+, TypeScript, existing `jsr:@std/*` imports, Pkl 0.32, YueScript 0.34.2.

**Spec:** `docs/superpowers/specs/2026-09-27-moonwell-editor-dx-design.md` (approved 2026-09-27). This plan covers §5
and the Plan 3b rows of §§7–9 and 12. Plan 3a (natives data, declarations, editor setup) is done; Plan 3c (macros)
follows, and 0.4.0 is released after it.

## Global Constraints

- No Node.js: no `package.json`, `node_modules`, `npm:` or `node:` specifiers. Only `jsr:@std/*` from the existing
  import maps.
- File-system code is async. Expected failures throw `MoonwellError` (`cli/src/shared/errors.ts`) with `file` and
  `hint`; anything else is reported as an internal error, so user mistakes must never reach it.
- `deno fmt` width 120 (`template/`, `docs/`, `cli/src/embedded/` excluded); `deno fmt --check` and `deno lint` clean.
- After changing `template/`, run `deno task gen`. Nothing stray in `template/`: every file there is embedded.
- Write files containing backslashes with a file-editing tool, never a shell heredoc or `sed`.
- The message format is the spec's (§5.4), exactly:
  `error: src/main.yue:7:11 › Unknown global CreatUnit.` then
  ``hint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.``
- Every task: failing test first, then code, then the full gate (below), then one commit on `main` ending with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`. Do not push; the maintainer pushes. Do not bump versions.

**Full gate (before every commit):**

```bash
deno task check && deno task lint && deno fmt --check && deno task test && deno task test:runtime && deno task test:pkl && deno task test:e2e
```

## Facts this plan relies on (checked 2026-09-27 with yue 0.34.2 on Windows)

- `yue -g <file>` prints `NAME LINE COLUMN` per use, in source order, 1-based, with CRLF line endings on Windows and a
  blank last line; exit code 0. For `global Score = 0\nprint CreatUnit!\nx = math.floor 1.5\nprint Score, x\n` it
  prints `Score 1 8`, `print 2 1`, `CreatUnit 2 7`, `math 3 5`, `print 4 1`, `Score 4 7`.
- `import "moonwell" as mw` is listed as a use of `require`; a `class` lists `setmetatable`. Both are in
  `natives.lua.globals`.
- Local assignments (`x = 1`) are not listed. `global a, b` with no value lists nothing. `global class Boss` lists only
  `setmetatable`, so the class name is known only through the `global`-line scan. `global const K = 1` lists `K`.
- `global *` and `global ^` make later assignments global without naming them (spec §5.5: not detected).
- The template's `src/main.yue` uses only known names: `require`, `print`, `CreateUnit` (at 8:10), `Player`,
  `TimerStart`, `CreateTimer`, `SetUnitColor`, `GetPlayerColor`, `GetRandomInt`, `bj_MAX_PLAYERS`.
  `src/generated/objects.yue` uses none.

## Decisions this plan adds to the spec

- **Cache file.** The `yue -g` results live in their own file, `dist/stage/lua/.globals.json`, next to the compile
  hashes (`.hashes.json`) rather than inside it: `{ settings, files: { "<path under src/>": { hash, uses } } }`, where
  `settings` is the compiler path (as for compiling) and `uses` is a list of `[name, line, column]`. The known names
  are never cached, so a change to `war3map.lua`, `lint.globals` or Moonwell re-evaluates every file without running
  `yue` (spec §5.2).
- **Every use is reported**, not every name: `Player` used twice unknown is two problems. The error output keeps the
  existing cap of 20 problems then `and N more`; warnings use the same cap.
- **Nearest names** (spec §5.4 says "reuses the nearest-name matching from object data"): the edit distance and
  `joinWords` move to `cli/src/shared/names.ts` and are shared; the threshold is tighter for globals (at most a quarter
  of the name's length, at least 1, no substring rule). Object data's looser rule, applied to 5,000 globals, suggested
  `Cos`, `I2R` and `I2S` for `io` and `GetPlayerScore` for `Score`. Up to three names are suggested.
- **Names the game removes** (`natives.lua.removed`, such as `io` and `debug`) get the hint
  `Warcraft III's Lua does not provide io.` instead of a suggestion.
- **Position type.** `Problem` gains optional `line` and `column`; `ObjectDataError` becomes a subclass of a new
  `ProblemsError`, which the unknown-global check throws. Object data problems carry no position and print as before.
- **A manifest without a `lint` block** (unit fixtures; every 0.4 schema has one) parses to the defaults.
- **No `--path` yet.** Spec §5.2 runs `yue -g --path <project>/.moonwell/yue/?.lua`; the macro module that path finds
  arrives in Plan 3c, which adds `--path` to every `yue` run (compile and `-g`) and the macro hash to both cache keys.
  This plan runs `yue -g <file>`.
- **Order in `compileProject`:** compile, resolve the module graph, then check globals. A syntax error or a missing
  module is reported before unknown globals.

## File Structure

| File | Responsibility |
| --- | --- |
| `cli/src/shared/names.ts` (new) | `editDistance`, `joinWords`, `closestNames` |
| `cli/src/shared/errors.ts` | `Problem` positions, `ProblemsError`, `formatProblem`, `MAX_PROBLEMS` |
| `cli/src/objectdata/metadata.ts`, `resolve.ts` | import `editDistance`/`joinWords` from `shared/names.ts` |
| `schema/Project.pkl` | `LintConfig` and `lint` |
| `cli/src/project/project.ts` | `Project.lint`, parsed and validated |
| `template/moonwell.pkl` | the `lint` block with comments |
| `cli/src/yue/compile.ts` | `CompileOutput.hashes`; export `compileError` and `forEachLimited` |
| `cli/src/lint/uses.ts` (new) | `parseGlobalUses`, `listGlobalUses` (runs `yue -g`, caches) |
| `cli/src/lint/unknown-globals.ts` (new) | `declaredGlobals`, `knownGlobals`, `unknownGlobalProblems`, `checkUnknownGlobals` |
| `cli/src/editor/refresh.ts` | export `readSourceScript` |
| `cli/src/pipeline.ts` | `compileProject` runs the check |
| Tests | `cli/tests/unit/{names,errors,project,pipeline,build,lint-uses,lint-unknown-globals}.test.ts`, `cli/tests/yue/{compile,uses}.test.ts`, `schema/tests/Project.pkl`, `cli/tests/e2e/lint.test.ts` |
| Docs | `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`, the spec |

---

### Task 1: Shared name matching and positioned problems

**Files:**
- Create: `cli/src/shared/names.ts`
- Modify: `cli/src/shared/errors.ts`, `cli/src/objectdata/metadata.ts`, `cli/src/objectdata/resolve.ts`
- Test: `cli/tests/unit/names.test.ts` (new), `cli/tests/unit/errors.test.ts`

**Interfaces:**
- Produces: `editDistance(a: string, b: string): number`; `joinWords(words: string[], conjunction: "and" | "or", max?:
  number): string`; `closestNames(names: Iterable<string>, key: string, max?: number): string[]` (all in
  `cli/src/shared/names.ts`). In `cli/src/shared/errors.ts`: `Problem { file; line?; column?; message; hint? }`,
  `class ProblemsError extends MoonwellError { problems: Problem[] }`, `ObjectDataError extends ProblemsError`,
  `MAX_PROBLEMS = 20`, `formatProblem(problem: ProblemText): string` where
  `ProblemText = { file?: string; line?: number; column?: number; message: string; hint?: string }`.

- [ ] **Step 1: Write the failing tests**

Create `cli/tests/unit/names.test.ts`:

```ts
import { assertEquals } from "@std/assert";
import { closestNames, editDistance, joinWords } from "../../src/shared/names.ts";

Deno.test("editDistance counts insertions, deletions and substitutions", () => {
  assertEquals(editDistance("kitten", "sitting"), 3);
  assertEquals(editDistance("", "abc"), 3);
  assertEquals(editDistance("same", "same"), 0);
});

Deno.test("joinWords joins with commas and a conjunction, and caps the list", () => {
  assertEquals(joinWords([], "or"), "");
  assertEquals(joinWords(["a"], "or"), "a");
  assertEquals(joinWords(["a", "b"], "or"), "a or b");
  assertEquals(joinWords(["a", "b", "c"], "and"), "a, b and c");
  assertEquals(joinWords(["a", "b", "c", "d"], "or", 2), "a, b and 2 more");
});

Deno.test("closestNames finds names a few edits away, ignoring case, nearest first", () => {
  const names = ["CreateUnit", "CreateItem", "print", "Player", "GetTriggerUnit", "Cos", "I2S"];
  assertEquals(closestNames(names, "CreatUnit"), ["CreateUnit"]);
  assertEquals(closestNames(names, "createunit"), ["CreateUnit"]);
  assertEquals(closestNames(names, "prnt"), ["print"]);
  assertEquals(closestNames(names, "GetTriggerUnt"), ["GetTriggerUnit"]);
  // A short name allows one edit, so "io" matches nothing here.
  assertEquals(closestNames(names, "io"), []);
  // The name itself is never suggested.
  assertEquals(closestNames(names, "print"), []);
  // At most three by default, ties broken by name.
  assertEquals(closestNames(["ae", "ad", "ac", "ab"], "aa"), ["ab", "ac", "ad"]);
  assertEquals(closestNames(["ae", "ad", "ac", "ab"], "aa", 1), ["ab"]);
});
```

Add to `cli/tests/unit/errors.test.ts` (extend the import to
`import { formatError, formatProblem, MoonwellError, ObjectDataError, ProblemsError } from "../../src/shared/errors.ts";`):

```ts
Deno.test("formatError prints each problem with its line and column when it has them", () => {
  const error = new ProblemsError([
    {
      file: "src/main.yue",
      line: 7,
      column: 11,
      message: "Unknown global CreatUnit.",
      hint: "Did you mean CreateUnit?",
    },
    { file: "src/main.yue", line: 9, message: "second" },
    { file: "objects/a.pkl", message: "third" },
  ]);
  assertEquals(
    formatError(error),
    [
      "error: src/main.yue:7:11 › Unknown global CreatUnit.",
      "hint: Did you mean CreateUnit?",
      "error: src/main.yue:9 › second",
      "error: objects/a.pkl › third",
    ].join("\n"),
  );
  assertEquals([error instanceof MoonwellError, error.file, error.line, error.message], [
    true,
    "src/main.yue",
    7,
    "Unknown global CreatUnit.",
  ]);
});

Deno.test("formatProblem has no prefix, so warnings can use it", () => {
  assertEquals(formatProblem({ file: "src/a.yue", line: 1, column: 2, message: "m", hint: "h" }), "src/a.yue:1:2 › m\nhint: h");
  assertEquals(formatProblem({ message: "m" }), "m");
});

Deno.test("ObjectDataError is a ProblemsError", () => {
  const error = new ObjectDataError([{ file: "objects/a.pkl", message: "first" }]);
  assertEquals([error instanceof ProblemsError, error.name], [true, "ObjectDataError"]);
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/names.test.ts cli/tests/unit/errors.test.ts`
Expected: FAIL: module `cli/src/shared/names.ts` not found, and `formatProblem`/`ProblemsError` not exported.

- [ ] **Step 3: Create `cli/src/shared/names.ts`**

Move `editDistance` out of `cli/src/objectdata/metadata.ts` and `joinWords` out of `cli/src/objectdata/resolve.ts`
unchanged, and add `closestNames`:

```ts
/** Levenshtein distance between `a` and `b`. */
export function editDistance(a: string, b: string): number {
  let previous = Array.from({ length: b.length + 1 }, (_, j) => j);
  for (let i = 1; i <= a.length; i++) {
    const current = [i];
    for (let j = 1; j <= b.length; j++) {
      current[j] = Math.min(previous[j] + 1, current[j - 1] + 1, previous[j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1));
    }
    previous = current;
  }
  return previous[b.length];
}

/** `a`, `a or b`, `a, b or c`; with `max`, the rest as `and N more`. */
export function joinWords(words: string[], conjunction: "and" | "or", max = words.length): string {
  const shown = words.slice(0, max);
  if (words.length > max) return `${shown.join(", ")} and ${words.length - max} more`;
  return shown.length < 2 ? shown.join("") : `${shown.slice(0, -1).join(", ")} ${conjunction} ${shown.at(-1)}`;
}

/**
 * Up to `max` of `names` closest to `key`, ignoring letter case: at most a quarter of its length in edits (at least
 * one), nearest first, then by name. `key` itself is never returned. Tighter than object data's rule, because it
 * searches thousands of global names.
 */
export function closestNames(names: Iterable<string>, key: string, max = 3): string[] {
  const wanted = key.toLowerCase();
  const limit = Math.max(1, Math.floor(wanted.length / 4));
  const matches: Array<{ name: string; distance: number }> = [];
  for (const name of names) {
    if (name === key || Math.abs(name.length - wanted.length) > limit) continue;
    const distance = editDistance(wanted, name.toLowerCase());
    if (distance <= limit) matches.push({ name, distance });
  }
  return matches
    .sort((a, b) => a.distance - b.distance || (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))
    .slice(0, max)
    .map(({ name }) => name);
}
```

In `cli/src/objectdata/metadata.ts`, delete `editDistance` and add `import { editDistance } from "../shared/names.ts";`
(`nearestBases` uses it). In `cli/src/objectdata/resolve.ts`, delete the local `joinWords`, remove `editDistance` from
the `./metadata.ts` import, and add `import { editDistance, joinWords } from "../shared/names.ts";`. Run
`grep -rn "editDistance\|joinWords" cli tools` afterwards: every use must import from `shared/names.ts`.

- [ ] **Step 4: Update `cli/src/shared/errors.ts`**

Replace everything from `/** One problem of several` to the end of the file with:

```ts
/** One problem of several, each reported on its own `error:` line with its own hint. */
export interface Problem {
  file: string;
  /** 1-based source position, when the problem has one. */
  line?: number;
  column?: number;
  message: string;
  hint?: string;
}

/** Several problems found at once; `file`, `line`, `message` and `hint` are the first problem's. */
export class ProblemsError extends MoonwellError {
  readonly problems: Problem[];

  constructor(problems: Problem[]) {
    if (problems.length === 0) throw new Error("A ProblemsError needs at least one problem.");
    super(problems[0].message, { file: problems[0].file, line: problems[0].line, hint: problems[0].hint });
    this.name = "ProblemsError";
    this.problems = problems;
  }
}

/** Every problem found in the custom objects. */
export class ObjectDataError extends ProblemsError {
  constructor(problems: Problem[]) {
    super(problems);
    this.name = "ObjectDataError";
  }
}

/** How many problems of a `ProblemsError` are printed before `and N more`. */
export const MAX_PROBLEMS = 20;

/** What `formatProblem` prints: a `Problem`, or a `MoonwellError`. */
export interface ProblemText {
  file?: string;
  line?: number;
  column?: number;
  message: string;
  hint?: string;
}

/** `file:line:column › message` and a `hint:` line, without the `error:` or `warning:` prefix. */
export function formatProblem(problem: ProblemText): string {
  const column = problem.column === undefined ? "" : `:${problem.column}`;
  const position = problem.line === undefined ? "" : `:${problem.line}${column}`;
  const where = problem.file === undefined ? "" : `${problem.file}${position} › `;
  return `${where}${problem.message}${problem.hint ? `\nhint: ${problem.hint}` : ""}`;
}

/** Renders an error for the terminal and the log file. */
export function formatError(error: unknown): string {
  if (error instanceof ProblemsError) {
    const lines = error.problems.slice(0, MAX_PROBLEMS).map((problem) => `error: ${formatProblem(problem)}`);
    const more = error.problems.length - MAX_PROBLEMS;
    return [...lines, ...(more > 0 ? [`and ${more} more`] : [])].join("\n");
  }
  if (error instanceof MoonwellError) return `error: ${formatProblem(error)}`;
  const detail = error instanceof Error ? (error.stack ?? error.message) : String(error);
  return `internal error: ${detail}\nThis is a bug in Moonwell; please report it.`;
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/names.test.ts cli/tests/unit/errors.test.ts cli/tests/unit/objectdata-resolve.test.ts cli/tests/unit/metadata.test.ts`
Expected: PASS (the existing object-data and error tests are unchanged and still pass).

- [ ] **Step 6: Full gate, then commit**

```bash
git add cli/src/shared/names.ts cli/src/shared/errors.ts cli/src/objectdata/metadata.ts cli/src/objectdata/resolve.ts cli/tests/unit/names.test.ts cli/tests/unit/errors.test.ts
git commit -m "refactor: shared name matching, and problems with source positions

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 2: The `lint` block

**Files:**
- Modify: `schema/Project.pkl`, `schema/tests/Project.pkl`, `cli/src/project/project.ts`, `template/moonwell.pkl`
- Regenerate: `cli/src/embedded/template.ts` (`deno task gen`)
- Test: `cli/tests/unit/project.test.ts`, `cli/tests/unit/pipeline.test.ts`, `cli/tests/unit/build.test.ts`,
  `cli/tests/pkl/project.test.ts`

**Interfaces:**
- Produces: `Project.lint: { unknownGlobals: "error" | "warning"; globals: string[] }` in
  `cli/src/project/project.ts`, and the manifest block
  `lint { unknownGlobals = "error"; globals = List() }`.

- [ ] **Step 1: Write the failing Pkl facts**

In `schema/tests/Project.pkl`, add to `["defaults"]`:

```pkl
    Project.lint.unknownGlobals == "error"
    Project.lint.globals == List()
```

and add these facts after `["defaults"]`:

```pkl
  ["lint accepts warning and Lua names"] {
    (Project) { lint { unknownGlobals = "warning" } }.lint.unknownGlobals == "warning"
    (Project) { lint { globals = List("MyLibrary", "_private", "x2") } }.lint.globals == List("MyLibrary", "_private", "x2")
  }
  ["lint rejects other levels and names that are not Lua names"] {
    t.catch(() -> (Project) { lint { unknownGlobals = "off" } }.lint.unknownGlobals).contains("off")
    t.catch(() -> (Project) { lint { globals = List("2x") } }.lint.globals).contains("isLuaName")
    t.catch(() -> (Project) { lint { globals = List("my.lib") } }.lint.globals).contains("isLuaName")
    t.catch(() -> (Project) { lint { globals = List("") } }.lint.globals).contains("isLuaName")
    t.catch(() -> (Project) { lint { globals = List("end") } }.lint.globals).contains("isLuaName")
  }
```

Where the facts read the rendered fixture (`parsed.map.folder == "hero.w3x"` and its neighbours), add:

```pkl
    parsed.lint.unknownGlobals == "error"
```

If Pkl's message for `"off"` does not contain `off`, use a substring of the actual message that names the rejected
value or the allowed ones, and say so in the report.

- [ ] **Step 2: Write the failing TypeScript tests**

In `cli/tests/unit/project.test.ts`, the expected object of `"parseProject maps omitted nullable fields to null"` gains
`lint: { unknownGlobals: "error", globals: [] },` after `objects: emptyObjects(),` (a manifest without `lint` gets the
defaults). Add:

```ts
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
```

In `cli/tests/pkl/project.test.ts`, the real-project test gains
`assertEquals(project.lint, { unknownGlobals: "error", globals: [] });`.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/project.test.ts` and `pkl test schema/tests/Project.pkl`
Expected: FAIL: `lint` is missing from `Project`, and `Project.lint` is not a property of the schema.

- [ ] **Step 4: Add `LintConfig` to `schema/Project.pkl`**

After the `isReservedFolder` function, add:

```pkl
/// Lua 5.3's reserved words, which cannot name a global.
local const luaKeywords = Set(
  "and", "break", "do", "else", "elseif", "end", "false", "for", "function", "goto", "if", "in", "local", "nil", "not",
  "or", "repeat", "return", "then", "true", "until", "while"
)

/// A Lua name: letters, digits and `_`, not starting with a digit, and not a reserved word.
local const function isLuaName(name: String): Boolean =
  name.matches(Regex(#"[A-Za-z_][A-Za-z0-9_]*"#)) && !luaKeywords.contains(name)
```

After `class AssetsConfig { ... }`, add:

```pkl
class LintConfig {
  /// What `check`, `build`, `test` and `dev` do with a global that nothing defines. Known globals are the game's
  /// natives, Blizzard.j functions and globals, the Lua libraries the game provides, the globals and functions of the
  /// source map's `war3map.lua`, names declared with `global` in any file under `src/`, and `globals` below.
  /// `"error"` fails the command; `"warning"` reports each one and continues.
  unknownGlobals: "error"|"warning" = "error"

  /// Extra global names to allow, such as those a `global *` or `global ^` file creates: Moonwell does not detect
  /// those.
  globals: List<String(isLuaName(this))> = List()
}
```

After `assets: AssetsConfig = new {}`, add `lint: LintConfig = new {}`.

- [ ] **Step 5: Parse it in `cli/src/project/project.ts`**

Add to `interface Project`, after `assets`:

```ts
  /** The unknown-global check (spec §5.3). */
  lint: { unknownGlobals: "error" | "warning"; globals: string[] };
```

In `parseProject`, next to the other helpers, add:

```ts
  const level = (input: unknown, path: string): "error" | "warning" =>
    input === "error" || input === "warning" ? input : fail(path, '"error" or "warning"');
```

after the `assets` line, add:

```ts
  // Every 0.4 schema package has a lint block; a manifest without one (unit fixtures) gets the defaults.
  const lint = data.lint === undefined ? { unknownGlobals: "error", globals: [] } : record(data.lint, "lint");
```

and in the returned object, after `assets`:

```ts
    lint: {
      unknownGlobals: level(lint.unknownGlobals, "lint.unknownGlobals"),
      globals: strings(lint.globals, "lint.globals"),
    },
```

Every `Project` literal in the tests needs the field: add `lint: { unknownGlobals: "error", globals: [] },` after
`assets: { paths: {}, exclude: [] },` in `cli/tests/unit/pipeline.test.ts` (`stageProject`) and
`cli/tests/unit/build.test.ts`. `deno task test` type-checks them; fix any other literal it names the same way.

- [ ] **Step 6: Show the block in `template/moonwell.pkl`**

After the `assets { ... }` block, add:

```pkl
// Globals your gameplay code uses must be defined somewhere: by the game, the map's triggers, a `global` line in
// src/, or this list. check, build, test and dev report any other as a likely typo.
lint {
  unknownGlobals = "error"  // "warning" reports unknown globals without failing the command
  globals = List()          // extra global names to allow, e.g. List("MyLibrary")
}
```

Run `deno task gen` to re-embed the template.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `deno task test && deno task test:pkl`
Expected: PASS, including the embedded-template freshness test and `cli/tests/pkl/init.test.ts` (a new project
evaluates with the new block).

- [ ] **Step 8: Full gate, then commit**

```bash
git add schema/Project.pkl schema/tests/Project.pkl cli/src/project/project.ts template/moonwell.pkl cli/src/embedded/template.ts cli/tests/unit/project.test.ts cli/tests/unit/pipeline.test.ts cli/tests/unit/build.test.ts cli/tests/pkl/project.test.ts
git commit -m "feat(lint): the lint block in moonwell.pkl

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 3: List each file's globals with `yue -g`, cached

**Files:**
- Create: `cli/src/lint/uses.ts`
- Modify: `cli/src/yue/compile.ts`
- Test: `cli/tests/unit/lint-uses.test.ts` (new), `cli/tests/yue/uses.test.ts` (new), `cli/tests/yue/compile.test.ts`

**Interfaces:**
- Consumes: `compileError(file: string, output: string): MoonwellError` and
  `forEachLimited<T>(items: T[], limit: number, fn: (item: T) => Promise<void>): Promise<void>`, both now exported from
  `cli/src/yue/compile.ts`.
- Produces: `CompileOutput.hashes: Record<string, string>` (SHA-256 per compiled source, keyed by POSIX path under
  `src/`, e.g. `"heroes/captain.yue"`). In `cli/src/lint/uses.ts`: `interface GlobalUse { name: string; line: number;
  column: number }`, `USES_CACHE = "dist/stage/lua/.globals.json"`, `parseGlobalUses(output: string, file: string):
  GlobalUse[]`, `listGlobalUses(options: { yue: string; root: string; hashes: Record<string, string>; run?: Runner;
  concurrency?: number }): Promise<Record<string, GlobalUse[]>>` (keys as in `hashes`).

- [ ] **Step 1: Write the failing unit tests**

Create `cli/tests/unit/lint-uses.test.ts`:

```ts
import { assertEquals, assertRejects, assertThrows } from "@std/assert";
import { dirname, join, relative } from "@std/path";
import { listGlobalUses, parseGlobalUses, USES_CACHE } from "../../src/lint/uses.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import type { Runner, RunResult } from "../../src/shared/process.ts";

/** A stand-in compiler: `yue -g <file>` prints `outputs[<path under src/>]` and records the call. */
function stubYue(root: string, outputs: Record<string, RunResult>) {
  const calls: string[] = [];
  const run: Runner = (_command, args) => {
    assertEquals(args.length, 2);
    assertEquals(args[0], "-g");
    const file = relative(join(root, "src"), args[1]).replaceAll("\\", "/");
    calls.push(file);
    return Promise.resolve(outputs[file] ?? { code: 0, stdout: "", stderr: "" });
  };
  return { run, calls };
}

const ok = (stdout: string): RunResult => ({ code: 0, stdout, stderr: "" });

Deno.test("parseGlobalUses reads CRLF output and skips blank lines", () => {
  assertEquals(parseGlobalUses("Score 1 8\r\nCreatUnit 2 7\r\n\r\n", "src/main.yue"), [
    { name: "Score", line: 1, column: 8 },
    { name: "CreatUnit", line: 2, column: 7 },
  ]);
  assertEquals(parseGlobalUses("\n", "src/main.yue"), []);
});

Deno.test("parseGlobalUses refuses output it cannot read", () => {
  const error = assertThrows(() => parseGlobalUses("Score one 8\n", "src/main.yue"), MoonwellError, "Score one 8");
  assertEquals(error.file, "src/main.yue");
});

Deno.test("listGlobalUses runs yue -g once per changed file and caches by hash", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    const outputs = { "main.yue": ok("print 1 1\n"), "heroes/captain.yue": ok("CreatUnit 3 5\n") };
    const hashes = { "main.yue": "h1", "heroes/captain.yue": "h2" };
    const expected = {
      "heroes/captain.yue": [{ name: "CreatUnit", line: 3, column: 5 }],
      "main.yue": [{ name: "print", line: 1, column: 1 }],
    };

    const first = stubYue(root, outputs);
    assertEquals(await listGlobalUses({ yue: "yue", root, hashes, run: first.run }), expected);
    assertEquals(first.calls.sort(), ["heroes/captain.yue", "main.yue"]);

    const unchanged = stubYue(root, outputs);
    assertEquals(await listGlobalUses({ yue: "yue", root, hashes, run: unchanged.run }), expected);
    assertEquals(unchanged.calls, []);

    const edited = stubYue(root, { ...outputs, "main.yue": ok("") });
    const afterEdit = await listGlobalUses({ yue: "yue", root, hashes: { ...hashes, "main.yue": "h3" }, run: edited.run });
    assertEquals(edited.calls, ["main.yue"]);
    assertEquals(afterEdit["main.yue"], []);

    const otherCompiler = stubYue(root, outputs);
    await listGlobalUses({ yue: "other-yue", root, hashes, run: otherCompiler.run });
    assertEquals(otherCompiler.calls.sort(), ["heroes/captain.yue", "main.yue"]);

    // A deleted file leaves the cache.
    await listGlobalUses({ yue: "other-yue", root, hashes: { "main.yue": "h1" }, run: stubYue(root, outputs).run });
    const cache = JSON.parse(await Deno.readTextFile(join(root, USES_CACHE)));
    assertEquals(Object.keys(cache.files), ["main.yue"]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("listGlobalUses runs every file again when the cache is unreadable", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    await Deno.mkdir(dirname(join(root, USES_CACHE)), { recursive: true });
    await Deno.writeTextFile(join(root, USES_CACHE), '{"settings": "yue", "files": {"main.yue": {"hash": "h1"}}}');
    const stub = stubYue(root, { "main.yue": ok("print 1 1\n") });
    assertEquals(await listGlobalUses({ yue: "yue", root, hashes: { "main.yue": "h1" }, run: stub.run }), {
      "main.yue": [{ name: "print", line: 1, column: 1 }],
    });
    assertEquals(stub.calls, ["main.yue"]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("listGlobalUses reports a failed yue -g like a compile error", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    const failed = { code: 1, stdout: "Failed to compile: main.yue\n2: unexpected expression\n", stderr: "" };
    const stub = stubYue(root, { "main.yue": failed });
    const error = await assertRejects(
      () => listGlobalUses({ yue: "yue", root, hashes: { "main.yue": "h1" }, run: stub.run }),
      MoonwellError,
      "unexpected expression",
    );
    assertEquals([error.file, error.line], ["src/main.yue", 2]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

Create `cli/tests/yue/uses.test.ts` (real compiler):

```ts
import { assertEquals } from "@std/assert";
import { join } from "@std/path";
import { listGlobalUses } from "../../src/lint/uses.ts";
import { testYue } from "../support/yue.ts";

Deno.test("listGlobalUses reads the real compiler's yue -g output", async () => {
  const yue = await testYue();
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    await Deno.mkdir(join(root, "src"));
    await Deno.writeTextFile(
      join(root, "src", "main.yue"),
      "global Score = 0\nprint CreatUnit!\nx = math.floor 1.5\nprint Score, x\n",
    );
    assertEquals(await listGlobalUses({ yue, root, hashes: { "main.yue": "h" } }), {
      "main.yue": [
        { name: "Score", line: 1, column: 8 },
        { name: "print", line: 2, column: 1 },
        { name: "CreatUnit", line: 2, column: 7 },
        { name: "math", line: 3, column: 5 },
        { name: "print", line: 4, column: 1 },
        { name: "Score", line: 4, column: 7 },
      ],
    });
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

In `cli/tests/yue/compile.test.ts`, the first test (`"compileSources compiles modules and loads them by dotted
name"`) gains, after `const output = ...`:

```ts
  assertEquals(Object.keys(output.hashes).sort(), ["main.yue", "util/math.yue"]);
  assertEquals(output.hashes["main.yue"].length, 64);
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/lint-uses.test.ts cli/tests/yue/uses.test.ts cli/tests/yue/compile.test.ts`
Expected: FAIL: `cli/src/lint/uses.ts` not found; `output.hashes` is undefined.

- [ ] **Step 3: Expose the hashes and helpers in `cli/src/yue/compile.ts`**

Add to `interface CompileOutput`:

```ts
  /** SHA-256 of each compiled source, keyed by its POSIX path under src/, e.g. "heroes/captain.yue". */
  hashes: Record<string, string>;
```

return it (`return { outDir, hashes, load(name) { ... } }`), and add `export` to `function compileError` and
`async function forEachLimited`.

- [ ] **Step 4: Create `cli/src/lint/uses.ts`**

```ts
import { dirname, join } from "@std/path";
import { MoonwellError } from "../shared/errors.ts";
import { type Runner, runProcess } from "../shared/process.ts";
import { compileError, forEachLimited } from "../yue/compile.ts";

/** One global a source file reads or writes, at its 1-based position. */
export interface GlobalUse {
  name: string;
  line: number;
  column: number;
}

/** The `yue -g` results, next to the compile hashes (`.hashes.json`). */
export const USES_CACHE = "dist/stage/lua/.globals.json";

interface CacheEntry {
  hash: string;
  uses: Array<[string, number, number]>;
}

interface Cache {
  /** The compiler path; another compiler lists again. */
  settings: string;
  files: Record<string, CacheEntry>;
}

/** `yue -g` output: one `NAME LINE COLUMN` per line (spec §2). `file` names the source in errors. */
export function parseGlobalUses(output: string, file: string): GlobalUse[] {
  const uses: GlobalUse[] = [];
  for (const raw of output.split(/\r?\n/)) {
    const line = raw.trim();
    if (line === "") continue;
    const match = /^(\S+) (\d+) (\d+)$/.exec(line);
    if (!match) {
      throw new MoonwellError(`yue -g printed a line Moonwell cannot read: ${line}`, {
        file,
        hint: "Use a YueScript version Moonwell supports: remove yue.version and yue.path from the manifests.",
      });
    }
    uses.push({ name: match[1], line: Number(match[2]), column: Number(match[3]) });
  }
  return uses;
}

function isEntry(value: unknown): value is CacheEntry {
  const entry = value as CacheEntry;
  return typeof entry === "object" && entry !== null && typeof entry.hash === "string" && Array.isArray(entry.uses);
}

async function readCache(path: string): Promise<Cache | undefined> {
  try {
    const cache = JSON.parse(await Deno.readTextFile(path)) as Cache;
    if (typeof cache?.settings !== "string" || typeof cache.files !== "object" || cache.files === null) return undefined;
    return Object.values(cache.files).every(isEntry) ? cache : undefined;
  } catch {
    return undefined;
  }
}

/**
 * The globals each source uses, keyed like `hashes` (paths under src/). Runs `yue -g` only for files whose hash or
 * compiler changed since the last run (spec §5.2); `yue -g` cannot share a run with compiling, so it runs separately.
 */
export async function listGlobalUses(options: {
  yue: string;
  root: string;
  hashes: Record<string, string>;
  run?: Runner;
  concurrency?: number;
}): Promise<Record<string, GlobalUse[]>> {
  const run = options.run ?? runProcess;
  const cachePath = join(options.root, ...USES_CACHE.split("/"));
  const previous = await readCache(cachePath);
  const files: Record<string, CacheEntry> = {};
  const pending: string[] = [];
  for (const [file, hash] of Object.entries(options.hashes)) {
    const cached = previous?.settings === options.yue ? previous.files[file] : undefined;
    if (cached?.hash === hash) files[file] = cached;
    else pending.push(file);
  }

  const failures: MoonwellError[] = [];
  await forEachLimited(pending, options.concurrency ?? 8, async (file) => {
    const label = `src/${file}`;
    const result = await run(options.yue, ["-g", join(options.root, "src", ...file.split("/"))]);
    if (result.code !== 0) {
      failures.push(compileError(label, `${result.stdout}\n${result.stderr}`));
      return;
    }
    const uses = parseGlobalUses(result.stdout, label);
    files[file] = { hash: options.hashes[file], uses: uses.map((use) => [use.name, use.line, use.column]) };
  });

  const sorted = Object.fromEntries(Object.entries(files).sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0)));
  await Deno.mkdir(dirname(cachePath), { recursive: true });
  await Deno.writeTextFile(cachePath, JSON.stringify({ settings: options.yue, files: sorted }, null, 2));
  if (failures.length > 0) {
    failures.sort((a, b) => (a.file ?? "").localeCompare(b.file ?? ""));
    throw failures[0];
  }
  return Object.fromEntries(
    Object.entries(sorted).map(([file, entry]) => [
      file,
      entry.uses.map(([name, line, column]) => ({ name, line, column })),
    ]),
  );
}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/lint-uses.test.ts cli/tests/yue/uses.test.ts cli/tests/yue/compile.test.ts`
Expected: PASS.

- [ ] **Step 6: Full gate, then commit**

```bash
git add cli/src/lint/uses.ts cli/src/yue/compile.ts cli/tests/unit/lint-uses.test.ts cli/tests/yue/uses.test.ts cli/tests/yue/compile.test.ts
git commit -m "feat(lint): list each source's globals with yue -g, cached per file hash

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 4: Report unknown globals in `check`, `build`, `test` and `dev`

**Files:**
- Create: `cli/src/lint/unknown-globals.ts`
- Modify: `cli/src/editor/refresh.ts` (export `readSourceScript`), `cli/src/pipeline.ts` (`compileProject`)
- Test: `cli/tests/unit/lint-unknown-globals.test.ts` (new), `cli/tests/unit/pipeline.test.ts`,
  `cli/tests/e2e/lint.test.ts` (new)

**Interfaces:**
- Consumes: `listGlobalUses`, `GlobalUse` (Task 3); `CompileOutput.hashes` (Task 3); `Project.lint` (Task 2);
  `closestNames`, `joinWords` (Task 1); `Problem`, `ProblemsError`, `formatProblem`, `MAX_PROBLEMS` (Task 1);
  `readMapGlobals(script: string): MapGlobals` (`cli/src/editor/map-globals.ts`); `loadNatives(): Promise<Natives>`
  (`cli/src/natives/natives.ts`).
- Produces (in `cli/src/lint/unknown-globals.ts`): `UNKNOWN_GLOBAL_HINT`; `declaredGlobals(source: string): string[]`;
  `knownGlobals(inputs: { natives: Natives; map?: MapGlobals; declared: Iterable<string>; extra: readonly string[] }):
  Set<string>`; `unknownGlobalProblems(uses: Record<string, GlobalUse[]>, known: ReadonlySet<string>, removed:
  readonly string[]): Problem[]`; `checkUnknownGlobals(ctx: CommandContext, project: Project, compiled: { yue: string;
  hashes: Record<string, string> }, natives?: Natives): Promise<Problem[]>`. In `cli/src/editor/refresh.ts`:
  `readSourceScript(path: string, label: string): Promise<string | undefined>` (now exported, unchanged).

- [ ] **Step 1: Write the failing unit tests**

Create `cli/tests/unit/lint-unknown-globals.test.ts`:

```ts
import { assertEquals, assertRejects } from "@std/assert";
import { join, relative } from "@std/path";
import { type CommandContext, createContext } from "../../src/context.ts";
import {
  checkUnknownGlobals,
  declaredGlobals,
  knownGlobals,
  UNKNOWN_GLOBAL_HINT,
  unknownGlobalProblems,
} from "../../src/lint/unknown-globals.ts";
import type { Natives } from "../../src/natives/natives.ts";
import { emptyObjects } from "../../src/objectdata/manifest.ts";
import type { Project } from "../../src/project/project.ts";
import { validateMapSettings } from "../../src/settings/options.ts";
import { formatError, ProblemsError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";

const NATIVES: Natives = {
  gameVersion: "9.9.9",
  types: [],
  functions: [
    { name: "CreateUnit", source: "common.j", constant: false, params: [], returns: "unit" },
    { name: "FourCC", source: "lua", constant: false, params: [], returns: "integer" },
  ],
  globals: [{ name: "bj_MAX_PLAYERS", source: "blizzard.j", type: "integer", constant: true, array: false }],
  lua: { globals: ["print", "math"], removed: ["io"] },
};

Deno.test("declaredGlobals reads the names on global lines", () => {
  const source = [
    "global Score = 0",
    "global a, b",
    "  global x, y = 1, 2 -- indented, with a comment",
    "global const K = 1",
    "global class Boss extends Base",
    "global f = (n) -> n",
    "global *",
    "global ^",
    "globalScore = 1",
    "print global",
  ].join("\r\n");
  assertEquals(declaredGlobals(source), ["Score", "a", "b", "x", "y", "K", "Boss", "f"]);
});

Deno.test("knownGlobals joins the natives, the map, declared names and lint.globals", () => {
  const known = knownGlobals({
    natives: NATIVES,
    map: { globals: [{ name: "udg_Score", type: "integer" }], functions: ["InitCustomTriggers"] },
    declared: ["Round"],
    extra: ["MyLibrary"],
  });
  assertEquals([...known].sort(), [
    "CreateUnit",
    "FourCC",
    "InitCustomTriggers",
    "MyLibrary",
    "Round",
    "bj_MAX_PLAYERS",
    "math",
    "print",
    "udg_Score",
  ]);
  assertEquals(knownGlobals({ natives: NATIVES, declared: [], extra: [] }).has("udg_Score"), false);
});

Deno.test("unknownGlobalProblems reports every unknown use in file, line and column order", () => {
  const known = knownGlobals({ natives: NATIVES, declared: [], extra: [] });
  const problems = unknownGlobalProblems(
    {
      "main.yue": [
        { name: "print", line: 1, column: 1 },
        { name: "CreatUnit", line: 7, column: 11 },
        { name: "Zzz", line: 2, column: 3 },
        { name: "io", line: 2, column: 1 },
      ],
      "a.yue": [{ name: "Zzz", line: 9, column: 1 }],
    },
    known,
    NATIVES.lua.removed,
  );
  assertEquals(problems, [
    { file: "src/a.yue", line: 9, column: 1, message: "Unknown global Zzz.", hint: UNKNOWN_GLOBAL_HINT },
    {
      file: "src/main.yue",
      line: 2,
      column: 1,
      message: "Unknown global io.",
      hint: "Warcraft III's Lua does not provide io.",
    },
    { file: "src/main.yue", line: 2, column: 3, message: "Unknown global Zzz.", hint: UNKNOWN_GLOBAL_HINT },
    {
      file: "src/main.yue",
      line: 7,
      column: 11,
      message: "Unknown global CreatUnit.",
      hint: `Did you mean CreateUnit? ${UNKNOWN_GLOBAL_HINT}`,
    },
  ]);
  assertEquals(
    UNKNOWN_GLOBAL_HINT,
    "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.",
  );
});

/** A project whose compiler is a stand-in printing `outputs[<path under src/>]` for `yue -g`. */
async function lintProject(
  sources: Record<string, string>,
  outputs: Record<string, string>,
  options: { lint?: Project["lint"]; mapScript?: string } = {},
) {
  const root = await Deno.makeTempDir({ prefix: "moonwell-lint-" });
  for (const [file, text] of Object.entries(sources)) {
    await Deno.mkdir(join(root, "src", ...file.split("/").slice(0, -1)), { recursive: true });
    await Deno.writeTextFile(join(root, "src", ...file.split("/")), text);
  }
  if (options.mapScript !== undefined) {
    await Deno.mkdir(join(root, "maps", "map.w3x"), { recursive: true });
    await Deno.writeTextFile(join(root, "maps", "map.w3x", "war3map.lua"), options.mapScript);
  }
  const logger = silentLogger();
  const ctx: CommandContext = {
    ...createContext(root, logger),
    run: (_command, args) => {
      const file = relative(join(root, "src"), args[1]).replaceAll("\\", "/");
      return Promise.resolve({ code: 0, stdout: outputs[file] ?? "", stderr: "" });
    },
  };
  const project: Project = {
    root,
    manifest: "moonwell.pkl",
    map: { folder: "map.w3x", entry: "src/main.yue" },
    build: { folder: "dist/bin", minify: false },
    launch: { gameExecutable: null, args: [] },
    yue: { version: "0.34.2", path: null },
    assets: { paths: {}, exclude: [] },
    lint: options.lint ?? { unknownGlobals: "error", globals: [] },
    settings: validateMapSettings({}),
    objects: emptyObjects(),
  };
  const hashes = Object.fromEntries(Object.keys(sources).map((file, i) => [file, `h${i}`]));
  return { root, ctx, logger, project, compiled: { yue: "yue", hashes } };
}

Deno.test("checkUnknownGlobals accepts map, declared and lint.globals names", async () => {
  const { root, ctx, project, compiled } = await lintProject(
    { "main.yue": "print udg_Score, Round, MyLibrary\n", "state.yue": "global Round = 1\n" },
    { "main.yue": "print 1 1\nudg_Score 1 7\nRound 1 18\nMyLibrary 1 25\n", "state.yue": "Round 1 8\n" },
    { lint: { unknownGlobals: "error", globals: ["MyLibrary"] }, mapScript: "udg_Score = 0\nfunction main()\nend\n" },
  );
  try {
    assertEquals(await checkUnknownGlobals(ctx, project, compiled, NATIVES), []);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("checkUnknownGlobals throws every unknown use at once", async () => {
  const { root, ctx, project, compiled } = await lintProject(
    { "main.yue": "x = CreatUnit!\nprint udg_Score\n" },
    { "main.yue": "CreatUnit 1 5\nprint 2 1\nudg_Score 2 7\n" },
  );
  try {
    const error = await assertRejects(() => checkUnknownGlobals(ctx, project, compiled, NATIVES), ProblemsError);
    assertEquals(
      formatError(error),
      [
        "error: src/main.yue:1:5 › Unknown global CreatUnit.",
        `hint: Did you mean CreateUnit? ${UNKNOWN_GLOBAL_HINT}`,
        "error: src/main.yue:2:7 › Unknown global udg_Score.",
        `hint: ${UNKNOWN_GLOBAL_HINT}`,
      ].join("\n"),
    );
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("checkUnknownGlobals only warns with unknownGlobals = warning", async () => {
  const { root, ctx, logger, project, compiled } = await lintProject(
    { "main.yue": "x = CreatUnit!\n" },
    { "main.yue": "CreatUnit 1 5\n" },
    { lint: { unknownGlobals: "warning", globals: [] } },
  );
  try {
    const problems = await checkUnknownGlobals(ctx, project, compiled, NATIVES);
    assertEquals(problems.length, 1);
    assertEquals(logger.lines, [
      `warning: src/main.yue:1:5 › Unknown global CreatUnit.\nhint: Did you mean CreateUnit? ${UNKNOWN_GLOBAL_HINT}`,
    ]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("checkUnknownGlobals warns about at most 20 uses, then how many more", async () => {
  const uses = Array.from({ length: 23 }, (_, i) => `Zzz ${i + 1} 1`).join("\n");
  const { root, ctx, logger, project, compiled } = await lintProject(
    { "main.yue": "" },
    { "main.yue": uses },
    { lint: { unknownGlobals: "warning", globals: [] } },
  );
  try {
    await checkUnknownGlobals(ctx, project, compiled, NATIVES);
    assertEquals(logger.lines.length, 21);
    assertEquals(logger.lines[20], "warning: and 3 more unknown global(s)");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

In `cli/tests/unit/pipeline.test.ts`, give `stageProject` a third parameter `globals = ""` (the stand-in's `yue -g`
output for every file), and make the stand-in answer `-g` first, without recording an event:

```ts
    run: async (_command, args) => {
      if (args[0] === "-g") return { code: 0, stdout: globals, stderr: "" };
```

Extend the errors import to `import { formatError, MoonwellError, ObjectDataError, ProblemsError } from
"../../src/shared/errors.ts";` and add:

```ts
Deno.test("prepareStage fails on an unknown global after compiling, before staging the map", async () => {
  const { root, ctx, project } = await stageProject(emptyObjects(), {}, "CreatUnit 1 1\n");
  try {
    const error = await assertRejects(() => prepareStage(ctx, project, {}), ProblemsError);
    assertEquals(
      formatError(error),
      "error: src/main.yue:1:1 › Unknown global CreatUnit.\n" +
        "hint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.",
    );
    assertEquals(await exists(join(root, "dist", "stage", "map.w3x")), false);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
```

(This test uses the embedded natives, where `CreateUnit` is the only name within two edits of `CreatUnit`.)

- [ ] **Step 2: Write the failing e2e tests**

Create `cli/tests/e2e/lint.test.ts`:

```ts
import { assertEquals, assertStringIncludes } from "@std/assert";
import { fromFileUrl, join } from "@std/path";

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

async function edit(path: string, from: string, to: string): Promise<void> {
  const text = await Deno.readTextFile(path);
  if (!text.includes(from)) throw new Error(`${path} does not contain ${from}`);
  await Deno.writeTextFile(path, text.replace(from, to));
}

const TYPO = "error: src/main.yue:8:10 › Unknown global CreatUnit.\n" +
  "hint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.";

Deno.test("check fails on a misspelt native, naming its position and the nearest name", async () => {
  const project = await newProject();
  await edit(join(project, "src", "main.yue"), "CreateUnit Player(0)", "CreatUnit Player(0)");
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(checked.text, TYPO);
});

Deno.test("with unknownGlobals = warning, build reports the typo and succeeds", async () => {
  const project = await newProject();
  await edit(join(project, "src", "main.yue"), "CreateUnit Player(0)", "CreatUnit Player(0)");
  await edit(join(project, "moonwell.pkl"), 'unknownGlobals = "error"', 'unknownGlobals = "warning"');
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  assertStringIncludes(built.text, TYPO.replace("error: ", "warning: "));
  assertStringIncludes(built.text, "Built dist/bin/map.w3x");
});

Deno.test("check accepts declared globals, lint.globals and the map's globals", async () => {
  const project = await newProject();
  await Deno.writeTextFile(join(project, "src", "state.yue"), "global Round = 1\n");
  const main = join(project, "src", "main.yue");
  await Deno.writeTextFile(
    main,
    `${await Deno.readTextFile(main)}\nglobal Score = 0\nprint Score, Round, MyLibrary, gg_unit_Hblm_0003\n`,
  );
  await edit(join(project, "moonwell.pkl"), "globals = List()", 'globals = List("MyLibrary")');
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 0, checked.text);
});
```

- [ ] **Step 3: Run the tests to verify they fail**

Run: `deno test -A cli/tests/unit/lint-unknown-globals.test.ts cli/tests/unit/pipeline.test.ts`
Expected: FAIL: `cli/src/lint/unknown-globals.ts` not found. (`cli/tests/e2e/lint.test.ts` fails too: the typo
passes `check`.)

- [ ] **Step 4: Export `readSourceScript` from `cli/src/editor/refresh.ts`**

Change `async function readSourceScript(` to `export async function readSourceScript(`. Nothing else changes.

- [ ] **Step 5: Create `cli/src/lint/unknown-globals.ts`**

```ts
import { join } from "@std/path";
import type { CommandContext } from "../context.ts";
import { type MapGlobals, readMapGlobals } from "../editor/map-globals.ts";
import { readSourceScript } from "../editor/refresh.ts";
import { loadNatives, type Natives } from "../natives/natives.ts";
import type { Project } from "../project/project.ts";
import { formatProblem, MAX_PROBLEMS, type Problem, ProblemsError } from "../shared/errors.ts";
import { closestNames, joinWords } from "../shared/names.ts";
import { type GlobalUse, listGlobalUses } from "./uses.ts";

export const UNKNOWN_GLOBAL_HINT =
  "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.";

const NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;

/**
 * The names a source's `global` lines declare (spec §5.1): `global Score = 0`, `global a, b`, `global const K = 1`,
 * `global class Boss`. `global *` and `global ^` name nothing (spec §5.5).
 */
export function declaredGlobals(source: string): string[] {
  const names: string[] = [];
  for (const line of source.split(/\r?\n/)) {
    const match = /^\s*global\s+(.*)$/.exec(line);
    if (!match) continue;
    let rest = match[1].replace(/--.*$/, "").trim();
    const keyword = /^(const|class)\s+(.*)$/.exec(rest);
    if (keyword?.[1] === "class") {
      const name = /^[A-Za-z_][A-Za-z0-9_]*/.exec(keyword[2]);
      if (name) names.push(name[0]);
      continue;
    }
    if (keyword) rest = keyword[2];
    for (const part of rest.split("=")[0].split(",")) {
      const name = part.trim();
      if (NAME.test(name)) names.push(name);
    }
  }
  return names;
}

/** Every name a gameplay file may use as a global (spec §5.1). */
export function knownGlobals(inputs: {
  natives: Natives;
  /** The source map's war3map.lua; absent when there is none. */
  map?: MapGlobals;
  declared: Iterable<string>;
  /** lint.globals from the manifest. */
  extra: readonly string[];
}): Set<string> {
  const known = new Set<string>();
  for (const fn of inputs.natives.functions) known.add(fn.name);
  for (const global of inputs.natives.globals) known.add(global.name);
  for (const name of inputs.natives.lua.globals) known.add(name);
  for (const global of inputs.map?.globals ?? []) known.add(global.name);
  for (const fn of inputs.map?.functions ?? []) known.add(fn);
  for (const name of inputs.declared) known.add(name);
  for (const name of inputs.extra) known.add(name);
  return known;
}

/**
 * One problem per use of a name not in `known`, sorted by file, line and column (spec §5.4). `uses` is keyed by path
 * under src/. `removed` lists the standard-library globals the game takes away.
 */
export function unknownGlobalProblems(
  uses: Record<string, GlobalUse[]>,
  known: ReadonlySet<string>,
  removed: readonly string[],
): Problem[] {
  const hints = new Map<string, string>();
  const hintFor = (name: string): string => {
    let hint = hints.get(name);
    if (hint === undefined) {
      const closest = closestNames(known, name);
      hint = removed.includes(name)
        ? `Warcraft III's Lua does not provide ${name}.`
        : closest.length > 0
        ? `Did you mean ${joinWords(closest, "or")}? ${UNKNOWN_GLOBAL_HINT}`
        : UNKNOWN_GLOBAL_HINT;
      hints.set(name, hint);
    }
    return hint;
  };
  const problems: Problem[] = [];
  for (const [file, fileUses] of Object.entries(uses)) {
    for (const use of fileUses) {
      if (known.has(use.name)) continue;
      problems.push({
        file: `src/${file}`,
        line: use.line,
        column: use.column,
        message: `Unknown global ${use.name}.`,
        hint: hintFor(use.name),
      });
    }
  }
  return problems.sort((a, b) =>
    (a.file < b.file ? -1 : a.file > b.file ? 1 : 0) || a.line! - b.line! || a.column! - b.column!
  );
}

/**
 * Checks every compiled source for unknown globals (spec §5). With `lint.unknownGlobals = "error"` any unknown use
 * throws a `ProblemsError` listing all of them; with `"warning"` they are logged and returned.
 */
export async function checkUnknownGlobals(
  ctx: CommandContext,
  project: Project,
  compiled: { yue: string; hashes: Record<string, string> },
  natives?: Natives,
): Promise<Problem[]> {
  const uses = await listGlobalUses({ yue: compiled.yue, root: ctx.root, hashes: compiled.hashes, run: ctx.run });
  const declared: string[] = [];
  for (const file of Object.keys(compiled.hashes)) {
    declared.push(...declaredGlobals(await Deno.readTextFile(join(ctx.root, "src", ...file.split("/")))));
  }
  const label = `maps/${project.map.folder}/war3map.lua`;
  const script = await readSourceScript(join(ctx.root, ...label.split("/")), label);
  const data = natives ?? await loadNatives();
  const known = knownGlobals({
    natives: data,
    map: script === undefined ? undefined : readMapGlobals(script),
    declared,
    extra: project.lint.globals,
  });
  const problems = unknownGlobalProblems(uses, known, data.lua.removed);
  if (problems.length === 0) return problems;
  if (project.lint.unknownGlobals === "error") throw new ProblemsError(problems);
  for (const problem of problems.slice(0, MAX_PROBLEMS)) ctx.logger.warn(formatProblem(problem));
  const more = problems.length - MAX_PROBLEMS;
  if (more > 0) ctx.logger.warn(`and ${more} more unknown global(s)`);
  return problems;
}
```

- [ ] **Step 6: Run the check in `compileProject` (`cli/src/pipeline.ts`)**

Import `checkUnknownGlobals` from `./lint/unknown-globals.ts` and replace the end of `compileProject`:

```ts
  const entry = entryModuleName(options.entry ?? project.map.entry);
  const modules = resolveGraph(entry, output.load, BUILTIN_MODULES);
  // After compiling and resolving, so syntax errors and missing modules are reported first (spec §5.2).
  await checkUnknownGlobals(ctx, project, { yue, hashes: output.hashes });
  return { modules, entry };
```

Update the doc comment of `compileProject` to
`/** Compiles src/, resolves the reachable module graph from the entry and checks for unknown globals. */`.
`check`, `build`, `test` and `dev` all reach it through `compileProject`; nothing else changes.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `deno test -A cli/tests/unit/lint-unknown-globals.test.ts cli/tests/unit/pipeline.test.ts` then
`deno test -A cli/tests/e2e/lint.test.ts`
Expected: PASS. The existing e2e, runtime and Pkl tests still pass: the template's gameplay uses only known globals.

- [ ] **Step 8: Full gate, then commit**

```bash
git add cli/src/lint/unknown-globals.ts cli/src/editor/refresh.ts cli/src/pipeline.ts cli/tests/unit/lint-unknown-globals.test.ts cli/tests/unit/pipeline.test.ts cli/tests/e2e/lint.test.ts
git commit -m "feat(lint): report unknown globals in check, build, test and dev

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 5: Documentation

**Files:**
- Modify: `README.md`, `CHANGELOG.md`, `CONTRIBUTING.md`, `AGENTS.md`,
  `docs/superpowers/specs/2026-09-27-moonwell-editor-dx-design.md`

- [ ] **Step 1: README**

Add this section right after `## Editor setup` (before `## Assets`):

````markdown
## Unknown globals

`check`, `build`, `test` and `dev` stop on a global that nothing defines, which is almost always a typo:

```text
error: src/main.yue:8:10 › Unknown global CreatUnit.
hint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.
```

A global is known when it is a native, a Blizzard.j function or global, or a Lua library the game provides; a global
or function of the source map's `war3map.lua` (such as `gg_unit_Hpal_0002` or `udg_Score`; run the command again
after saving the map in World Editor); a name declared with `global` in any file under `src/` (`global Score = 0`,
`global a, b`); or a name listed in `lint.globals` in `moonwell.pkl`. Fields are not checked: `math.floor` checks only
`math`.

`global *` and `global ^` make later assignments global without naming them, so Moonwell cannot see those names; list
them in `lint.globals`. To report unknown globals without failing, set `lint { unknownGlobals = "warning" }`. In the
editor, lua-language-server underlines the same names as you type.

The check runs the compiler's `yue -g` on each changed file and caches the result in `dist/stage/lua/`.
````

- [ ] **Step 2: CHANGELOG**

Under `## Unreleased`, after the `setup` bullets and before the "After upgrading" bullet, add:

```markdown
- `check`, `build`, `test` and `dev` report every global a gameplay file uses that nothing defines, with its file,
  line and column and the nearest known name (`Did you mean CreateUnit?`). Known globals are the game's natives,
  Blizzard.j functions and globals and Lua libraries, the source map's `war3map.lua` globals, names declared with
  `global` under `src/` and the new `lint.globals` list. `lint.unknownGlobals = "warning"` reports them without
  failing. New projects show the `lint` block in `moonwell.pkl`.
```

- [ ] **Step 3: CONTRIBUTING step 10**

Append to step 10, before the final `git status` sentence:

```markdown
    Type `CreatUnit` for `CreateUnit` in `src/main.yue`: the editor underlines it, and `deno task check` fails with
    `src/main.yue:<line>:<column> › Unknown global CreatUnit.` and `Did you mean CreateUnit?`. Set
    `lint { unknownGlobals = "warning" }` in `moonwell.pkl` and confirm `deno task build` succeeds and prints the same
    lines as warnings; then undo both changes.
```

- [ ] **Step 4: AGENTS.md**

In "State", add after the Plan 3a bullet:

```markdown
- **Plan 3b done, unreleased** (`docs/superpowers/plans/2026-09-27-moonwell-unknown-globals.md`): the unknown-global
  check. `compileProject` runs `yue -g` per changed source (cache `dist/stage/lua/.globals.json`, code in
  `cli/src/lint/`), builds the known names from `natives.json`, the source map's `war3map.lua`, `global` lines under
  `src/` and `lint.globals`, and throws a `ProblemsError` (or warns, with `lint.unknownGlobals = "warning"`). Name
  matching is in `cli/src/shared/names.ts`, shared with object data.
```

In "Next work", change item 1 to Plan 3c (macros, spec §6), then the 0.4.0 release with the full gate.

- [ ] **Step 5: Spec**

In `docs/superpowers/specs/2026-09-27-moonwell-editor-dx-design.md` §5.2, replace "Results are cached per file hash
next to the compile hashes (`dist/stage/lua/.hashes.json`)" with "Results are cached per file hash in
`dist/stage/lua/.globals.json`, next to the compile hashes". In §5.4, after "The suggestion reuses the nearest-name
matching from object data." add: "with a tighter threshold (at most a quarter of the name's length in edits), since it
searches thousands of names; a standard-library name the game removes (`io`) gets `Warcraft III's Lua does not provide
io.` instead."

- [ ] **Step 6: Check and commit**

Run: `deno fmt --check` (README, CHANGELOG, CONTRIBUTING and AGENTS.md are formatted; `docs/` is excluded).

```bash
git add README.md CHANGELOG.md CONTRIBUTING.md AGENTS.md docs/superpowers/specs/2026-09-27-moonwell-editor-dx-design.md
git commit -m "docs: the unknown-global check

Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>"
```

---

### Task 6: Maintainer gate, unknown globals

The maintainer runs the new part of CONTRIBUTING step 10 (spec §9 step 2) in a throwaway `init --link` project on
Windows with VS Code (or Antigravity), the YueScript extension and the Lua extension: the editor underlines
`CreatUnit`; `deno task check` fails naming its position and suggesting `CreateUnit`; with
`lint.unknownGlobals = "warning"`, `deno task build` succeeds and prints the warning. Record the result under
"Release gate (so far)" in `CHANGELOG.md` and commit.

## Completion

All six tasks committed, the full gate green, the maintainer's gate recorded. Then Plan 3c (macros).
