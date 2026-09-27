# Moonwell Editor & DX (sub-project 3) — Design

- **Date:** 2026-09-27
- **Status:** Sections 1–5 approved by the maintainer in chat on 2026-09-27; this written spec awaits review
- **Builds on:** `2026-09-24-moonwell-core-design.md` (§1 roadmap item 3, §§3–5 and 9);
  `2026-09-26-moonwell-object-data-design.md` (generator and embedding pattern, `objects.yue`, nearest-name hints)
- **Scope:** Editor support for YueScript gameplay code in VS Code, a build-time check for unknown globals, and
  compile-time macros. Three plans (3a, 3b, 3c), one release (0.4.0).

## 1. Summary

Moonwell projects are written in YueScript, and today nothing tells an author that `CreatUnit` is a typo until the
game runs the code. This sub-project gives authors three things, all driven by one data file generated from the game's
own `common.j` and `Blizzard.j`:

1. **Editor support.** In VS Code with the YueScript extension (`LiJin.yuescript`) and lua-language-server (LuaLS),
   `.yue` files get completion, hover, signature help and diagnostics for every native, Blizzard.j function and global,
   the Moonwell runtime, the project's object ids and the map's own globals.
2. **Unknown-global check.** `check`, `build`, `test` and `dev` report every global a gameplay file uses that nothing
   defines, with the nearest known name as a hint, in any editor or none.
3. **Macros.** `$FourCC("hfoo")` compiles to the integer `1751543663`, checked at compile time.

Success means: a new project opened in VS Code with the two tools installed completes and type-checks natives without
further setup beyond putting `yue` on PATH; a typo in a global fails `check` with the file, line, column and a
suggestion; `$FourCC` works in the game; and the release gate (§9) passes on Warcraft III Reforged 3.0.0.24268.

Decisions are marked **Decision:** with a one-line rationale. Claims not yet confirmed are marked **To verify** and
collected in §10; the first task of Plan 3a checks them before anything depends on them.

## 2. Verified facts this design relies on

Checked on 2026-09-27 with the pinned compiler (YueScript 0.34.2, Windows) and the extension's source
(`pigpigyyy/yuescript-vscode`, version 0.2.9 on the Marketplace):

- **Yue macros.** A module exporting `macro FourCC = (s) -> ...` is imported with `import "moonwell.macros" as
  {:$FourCC}` (the `{$FourCC}` form fails with "can't destructure value"). `$FourCC("hfoo")` compiled to
  `1751543663`. An `assert` inside the macro fails the compile with `failed to expand macro: (macro FourCC):4: <message>`
  and a caret at the call.
- **Macro search path.** With the module at `<project>/.moonwell/yue/moonwell/macros.yue`, passing
  `--path .moonwell/yue/?.lua` (relative to the working directory, or absolute as a Windows path) finds it: the
  compiler searches the `.lua` pattern with `.yue`. Without `--path` it searches only next to the source file.
- **`yue -g <file>`** prints `NAME LINE COLUMN` for every global a file reads or writes, one per line, and exits 0; a
  syntax error prints the usual compile error and exits 1. Macro calls and their expansions are not listed. A
  `global Score = 0` declaration is listed like a use (`Score 1 8`). Fields are not listed separately
  (`math.floor` lists `math`).
- **`-g` cannot share a run with compiling:** `yue -g -r -o out.lua file.yue` crashed (segmentation fault).
- **The extension** starts its language server as `spawn("yue", ["-e", ".../server.yue"])`: `yue` must be on PATH
  and there is no setting for its location. It reads `yueconfig.yue` from the workspace root. Its own global lint
  (`lint_global`) flags every global not in the config's `globals` list. It passes `include` entries to the compiler as
  `<configDir>/<include>/?.lua`. LuaLS features are fully enabled only with `build`, `reserve_line_number` and
  `reserve_comment` all true; with `build: true` it writes `<file>.lua` next to each saved `.yue`. Its settings are
  `yuescript.luaLS.executablePath` (empty = auto-detect) and `yuescript.luaLS.parameters`.
- **The bundler** (`cli/src/yue/compile.ts`) compiles only `src/**/*.yue`; `.lua` files in `src/` are ignored. So the
  extension's output does not affect builds. (Importing existing Lua code is sub-project 4.)

## 3. Game data: `natives.json`

### 3.1 Inputs and generator

The maintainer exports two files from the game's CASC storage with CascView, keeping the game's relative paths:
`war3.w3mod/scripts/common.j` and `war3.w3mod/scripts/blizzard.j` (**To verify:** exact paths and letter case in
3.0.0.24268). The repo-only task `deno task gen:natives <folder> <game version>` (`tools/gen-natives.ts`) parses them
and writes `cli/data/natives.json`, committed. Like `gen:metadata`, it is run once per game patch.

The parser handles the subset of JASS these files use: `type X extends Y`, `native`/`constant native` declarations,
`function ... takes ... returns ...` headers in `blizzard.j` (bodies are skipped), and the `globals ... endglobals`
blocks with `constant`, `array` and initial values. Comments are not kept: they are Blizzard's text, and the repo
never copies it.

### 3.2 Lua extras

The game's Lua mode defines names outside both files and removes parts of the Lua standard library. A hand-written,
reviewed list, `tools/natives/lua-extras.json`, records both:

- `functions`: names the game adds, with a signature, e.g. `FourCC(id: string): integer`.
- `stdlib`: the standard-library globals the game provides, with the fields that exist (e.g. `os` with only the
  functions the game keeps), and the ones it removes (e.g. `io`).

**To verify:** the exact list, in the game (release gate step 4, §9). The first version comes from a small Lua probe
map the plan provides, not from memory.

### 3.3 Shape

```json
{
  "gameVersion": "3.0.0.24268",
  "types": [{ "name": "unit", "extends": "widget" }],
  "functions": [
    {
      "name": "CreateUnit", "source": "common.j", "constant": false,
      "params": [{ "name": "id", "type": "player" }, { "name": "unitid", "type": "integer" }],
      "returns": "unit"
    }
  ],
  "globals": [
    { "name": "bj_MAX_PLAYERS", "source": "blizzard.j", "type": "integer", "constant": true, "array": false }
  ],
  "lua": { "functions": [], "stdlib": { "provided": {}, "removed": [] } }
}
```

Every list is sorted by name so regeneration produces stable diffs.

### 3.4 Type mapping

`integer` → `integer`, `real` → `number`, `boolean` → `boolean`, `string` → `string`, `code` → `function`,
`nothing` → no return. Handle types stay as named classes with their `extends` chain, so LuaLS reports a `unit` passed
where a `player` is expected. Arrays become `T[]`.

### 3.5 Embedding

`deno task gen` embeds `natives.json` into `cli/src/embedded/natives.ts`; the existing freshness test covers it. Unit
tests of the generator use small `.j` fixtures, so the real export is needed only to regenerate the data file.

**Decision:** one data file, derived views. The LuaLS declarations, the known-globals set and any future fennel-ls
docset are all rendered from `natives.json`, so they cannot disagree.

## 4. Editor setup and project files

### 4.1 Committed files

`init` creates these; `setup` adds any that are missing to existing projects and never overwrites one that exists.

- **`yueconfig.yue`:** `build: true`, `reserve_line_number: true`, `reserve_comment: true` (the three the extension
  needs for LuaLS), `lint_global: false`, `include: [".moonwell/yue"]`, `options: { target: "5.3" }`, with a short
  comment per option. **Decision:** `lint_global: false`, so editor diagnostics for unknown globals come from LuaLS and
  the declarations, and this file stays small and hand-editable instead of listing thousands of names.
- **`.luarc.json`:** `runtime.version: "Lua 5.3"`; `runtime.path: ["src/?.lua", "src/?/init.lua"]`;
  `workspace.library: [".moonwell/types"]`; `workspace.ignoreDir: ["dist", "maps"]` (so LuaLS does not index World
  Editor's `war3map.lua`); `runtime.builtin` disabling the libraries the game removes (§3.2).
- **`.vscode/extensions.json`:** recommends `LiJin.yuescript` and `sumneko.lua`.
- **`.gitignore`** gains `.moonwell/` and `src/**/*.lua` (the extension's output on save).

These are template files, so they are embedded and covered by the embedded-template test like the rest of
`template/`. `setup` writes the same embedded content.

### 4.2 Generated files (`.moonwell/`, git-ignored)

| File                                | Content                                                                                             |
| ----------------------------------- | --------------------------------------------------------------------------------------------------- |
| `.moonwell/types/natives.d.lua`     | `---@meta`; every handle type as `---@class unit: widget`; every function with `---@param`/`---@return`; globals with `---@type` |
| `.moonwell/types/moonwell.d.lua`    | `---@meta moonwell`: the runtime module (`on_config`, `on_main`, `before_config`, `before_main`, ...) |
| `.moonwell/types/objects.d.lua`     | `---@meta generated.objects`: the object ids by category, from the same data as `objects.yue`       |
| `.moonwell/types/map.d.lua`         | the source map's `war3map.lua` globals, e.g. `gg_unit_hfoo_0001: unit`, `udg_Score: integer[]`     |
| `.moonwell/yue/moonwell/macros.yue` | the macro module (§6)                                                                               |

`map.d.lua` comes from the `globals` block of `maps/<folder>/war3map.lua` and the functions World Editor defines
there (`InitCustomTriggers`, `CreateAllUnits`, ...). **To verify:** the exact shape of that block in a World Editor
3.00 Lua save, with typed `udg_` variables and preplaced units; the plan captures a fixture first.

**When:** `setup`, `check`, `dev` and `build` (and `test`, which builds) write `.moonwell/` at the point where
`objects.yue` is refreshed today. Each file is written only when its content differs, so the editor does not reload
for nothing. Nothing in `.moonwell/` goes into the built map. A failure to write `.moonwell/` fails the command: it is
what the editor relies on.

### 4.3 `yue` on PATH

`setup` copies the pinned compiler to a stable location, `%LOCALAPPDATA%\moonwell\bin\yue.exe` on Windows and
`<cache>/moonwell/bin/yue` on Linux, replacing it when the pinned version changes. It then runs `yue -v` from PATH:
if `yue` is missing or reports another version, `setup` prints the one-time command that adds the `bin` folder to the
user PATH (PowerShell on Windows, a shell profile line on Linux). **Decision:** Moonwell never changes PATH itself
(maintainer's choice, option A). A copy locked by a running editor on Windows (`EBUSY`) is a `MoonwellError` telling
the user to close VS Code and retry.

`setup` does not look for lua-language-server: the YueScript extension uses the one bundled with the Lua extension
(`sumneko.lua`), which `.vscode/extensions.json` recommends (V3).

### 4.4 Documentation

README gains an "Editor setup" section: install VS Code, the YueScript extension and lua-language-server; run
`deno task setup`; add `yue` to PATH with the printed command; reload VS Code.

## 5. Unknown-global check

### 5.1 Known names

A global is known when it is any of:

- a function, global or constant in `natives.json` (common.j, blizzard.j, Lua extras);
- a standard-library global the game provides (§3.2);
- a global or function of the source map's `war3map.lua` (§4.2);
- declared by the project: any name on a source line whose first token is `global` (`global Score = 0`,
  `global a, b`), in any file under `src/`;
- listed in `lint.globals` in `moonwell.pkl`.

Only root names are checked (`math.floor` checks `math`). The runtime module is imported, not global, so it needs no
entry.

### 5.2 Running

After compiling, Moonwell runs `yue -g --path <project>/.moonwell/yue/?.lua <file>` for each source file, eight at a
time, like compiling. Results are cached per file hash next to the compile hashes (`dist/stage/lua/.hashes.json`), so
only changed files run `yue -g` again. A change to `war3map.lua`, to `lint.globals` or to the Moonwell version
re-evaluates every file's cached names without re-running `yue`.

It runs in `check`, `build`, `test` and `dev`, after compiling, so a syntax error is still reported as a compile
error first.

### 5.3 Manifest

```pkl
lint {
  unknownGlobals = "error"   // "error" fails the command; "warning" reports and continues
  globals = List()           // extra allowed global names
}
```

New class `LintConfig` in `schema/Project.pkl`; `unknownGlobals` is `"error" | "warning"`, default `"error"`
(maintainer's choice, option C); `globals` entries must be valid Lua names. The template's `moonwell.pkl` shows the
block with comments.

### 5.4 Messages

Every unknown global is reported at once, sorted by file, line and column, in the existing error format:

```
error: src/main.yue:7:11 › Unknown global CreatUnit.
hint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.
```

The suggestion reuses the nearest-name matching from object data. With `"warning"`, the same lines are printed as
warnings and the command continues.

### 5.5 Not covered

`global *` and `global ^` (every assignment in a file becomes global) are not detected; names used that way need
`lint.globals`, and the README says so. No type checking (LuaLS does that in the editor), no unused-variable or style
lints.

## 6. Macros

`moonwell.macros` is written to `.moonwell/yue/moonwell/macros.yue` and exports one macro:

```yue
import "moonwell.macros" as {:$FourCC}

unit = CreateUnit Player(0), $FourCC("hfoo"), 0, 0, 270   -- compiles to 1751543663
```

`$FourCC` takes one string literal of exactly four characters and expands to the big-endian integer of its bytes.
Anything else (a variable, a number, another length, a character above 255) fails the compile at the call with
`$FourCC needs a string literal of exactly 4 characters, such as "hfoo".`, shown through the existing compile-error
format with file and line.

Every `yue` run Moonwell makes (compile and `-g`) gets `--path <project>/.moonwell/yue/?.lua`; the editor gets the
same path through `include`. The macro module's hash joins the compile cache key, so a Moonwell upgrade that changes
a macro recompiles every file. The module exists only at compile time: it adds no `require` and nothing to the map.

**Decision:** `$FourCC` only. Further macros get their own short design.

The template's `src/main.yue` uses `$FourCC` once: it creates a standard Footman (`$FourCC "hfoo"`) next to the
Captain, so e2e tests and the release gate exercise a macro in the game.

## 7. Error handling

Expected failures throw `MoonwellError` with `file` and `hint`, as everywhere else:

- `gen:natives` input that does not parse names the `.j` file and line (repo-only tool, but same style).
- A `war3map.lua` whose `globals` block cannot be read names `maps/<folder>/war3map.lua`, with a hint to re-save the
  map in World Editor.
- A malformed `lint` block is a Pkl error on `moonwell.pkl`, like the other blocks.
- `setup` failing to copy `yue` (`EBUSY` on Windows) or to write `.moonwell/` says what to close or check.

## 8. Testing

- **Unit** (`deno task test`, need neither Pkl nor yue): the JASS parser on fixture `.j` files; `natives.json` shape
  and sort order; rendering `natives.d.lua`, `moonwell.d.lua`, `objects.d.lua` and `map.d.lua` from small inputs;
  parsing `yue -g` output and detecting `global` lines; the known-name decision and messages with a stub compiler;
  writing only changed files; `setup` adding missing editor files without overwriting; the PATH check with a stub
  runner.
- **Freshness:** `natives.json` embedded; template files embedded.
- **Pkl** (`deno task test:pkl`): `lint` defaults, the two allowed values, invalid names rejected.
- **E2E** (`deno task test:e2e`, real compiler): `init` → `check` passes on the template; a `CreatUnit` typo fails
  `check` with the file, line, column and `CreateUnit` hint; `"warning"` lets `build` succeed; a `lint.globals` name
  and a `global` declaration are accepted; `$FourCC("hfoo")` compiles to `1751543663`; `$FourCC("hfo")` fails at the
  right file and line; `template/src/generated/objects.yue` still matches `template/objects/`.

## 9. Release gate

A new CONTRIBUTING step, in a throwaway project from `init --link`:

1. Open the project in VS Code with the YueScript extension and lua-language-server installed, after `deno task
   setup` and the PATH step. Confirm completion and hover for `CreateUnit` (parameter names and types),
   `mw.on_main` and `objects.units.captain`; save the map in World Editor with a preplaced unit and confirm its
   `gg_unit_...` global completes after `deno task check`.
2. Type `CreatUnit`: the editor underlines it, and `deno task check` fails naming it and suggesting `CreateUnit`. Set
   `lint.unknownGlobals = "warning"` and confirm `deno task build` succeeds with the warning.
3. `deno task test`: the Footman created with `$FourCC` appears next to the Captain.
4. In the game, run the Lua probe from Plan 3a and confirm the standard-library globals `lua-extras.json` lists as
   provided exist and the removed ones (such as `io`) do not.

## 10. To verify (Plan 3a's first task)

1. The extension resolves `import "moonwell.macros" as {:$FourCC}` through `include: [".moonwell/yue"]`.
2. LuaLS resolves `import "moonwell"` and `import "generated.objects"` to the `---@meta moonwell` and
   `---@meta generated.objects` declarations in `workspace.library`.
3. The extension finds a lua-language-server installed by the official LuaLS VS Code extension (`sumneko.lua`), or
   needs `yuescript.luaLS.executablePath`. If it needs the setting, `init` writes it to `.vscode/settings.json` and
   `setup` fills in the detected path.
4. The exact CASC paths of `common.j` and `blizzard.j` in 3.0.0.24268, and whether they carry doc comments (§3.1).
5. The shape of World Editor 3.00's Lua `globals` block with typed `udg_` variables and preplaced units (§4.2).
6. The standard library the game's Lua mode provides (§3.2), with the Lua probe map.

## 11. Inputs needed from the maintainer

- A CascView export of `war3.w3mod/scripts/common.j` and `war3.w3mod/scripts/blizzard.j` from 3.0.0.24268.
- A World Editor 3.00 Lua map saved with at least one typed global variable (`udg_`), one array variable and one
  preplaced unit, for the `map.d.lua` fixture.
- One run of the Lua probe map in the game (§10.6), and the release gate (§9).

## 12. Implementation order

1. **Plan 3a — natives data, declarations and editor setup.** Starts with the §10 verification; then `gen:natives`
   and `natives.json`; the declaration renderers; `.moonwell/` writing; `yueconfig.yue`, `.luarc.json`,
   `.vscode/extensions.json` and `.gitignore` in the template and `setup`; the `bin/yue` copy and PATH check; README.
   The maintainer runs the Lua probe map during this plan, so `lua-extras.json` is confirmed before Plan 3b relies on
   it. Ends with gate step 1.
2. **Plan 3b — unknown-global check.** `yue -g` runs and caching, known names, `LintConfig`, messages. Ends with gate
   step 2.
3. **Plan 3c — macros.** The macro module, `--path` on every `yue` run, the cache key, the template Footman. Ends with
   gate steps 3 and 4.

**Decision:** one release, 0.4.0, after Plan 3c, because 3b depends on 3a's data and the gate covers all three.

## 13. Out of scope

- Type checking in `check`; unused-variable, style or formatting rules.
- Editors other than VS Code (the files are standard, but only VS Code is documented and gated).
- Importing existing Lua libraries (sub-project 4).
- Fennel support (backlog: a fennel-ls docset rendered from `natives.json`, plus a compile step).
- Detecting `global *` / `global ^` (§5.5).
