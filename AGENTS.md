# AGENTS.md: handoff for coding agents

Moonwell is a Warcraft III map development framework. Gameplay is written in YueScript and compiled to Lua 5.3; project
data is written in Pkl; the toolchain is a Deno CLI published to JSR as `@moonwell/cli`. The Pkl schemas are published
as the Pkl package `moonwell` (a GitHub release tagged `moonwell@<version>`). This file tells you what exists, the
rules, the known pitfalls, and what to do next. It was written by the previous agent (Claude) on 2026-09-25 when handing
over, and updated on 2026-09-27 after Plan 3b.

## Read first

- `docs/superpowers/specs/2026-09-24-moonwell-core-design.md` is the **binding design** for the whole project. Its §6
  describes the data layers: §6.1 object data, §6.2 assets and §6.3 map settings are all implemented.
- `README.md` (user docs), `CONTRIBUTING.md` (checks, release gate, publishing), `CHANGELOG.md` (0.1.0 released,
  `## Unreleased` lists what is done since).
- Later specs, all implemented: `2026-09-25-moonwell-model-paths-design.md`,
  `2026-09-25-moonwell-in-game-paths-design.md`, `2026-09-25-moonwell-map-settings-design.md` and
  `2026-09-26-moonwell-object-data-design.md`. `2026-09-27-moonwell-editor-dx-design.md` (sub-project 3) is being
  implemented: Plans 3a and 3b are done, 3c is not. Plans for everything built so far are in `docs/superpowers/plans/`;
  follow their style when writing new plans.

## State (2026-09-27)

- **Released:** 0.1.0, on JSR (`@moonwell/cli@0.1.0`) and as a GitHub release (`moonwell@0.1.0`). It contains the
  toolchain: `init`, `setup`, `build`, `test`, `dev`, `check`; the YueScript bundler with runtime hooks; the MPQ writer;
  and the manifest in Pkl.
- **Released 0.2.0** (2026-09-26, JSR `@moonwell/cli@0.2.0` and GitHub release `moonwell@0.2.0`). It contains:
  - Assets (Plan 2a): `assets/` is imported into builds. The manifest's `assets { paths {}; exclude = List() }` maps and
    excludes files. `assets:check` / `assets:sync` write the assets into the source map with ownership in
    `.asset-state/`.
  - `assets:paths [file]`: lists the files a model (`.mdx`/`.mdl`) references as `in-game path`,
    `in-game path, replaced`, `custom path, imported` or `custom path, not imported`, with `\` like World Editor's
    Import Manager. It uses an embedded list of 42,127 in-game paths generated from WC3 3.0.0.24268
    (`cli/data/game-paths.txt`).
  - `init` creates the icon folders `assets/ReplaceableTextures/{CommandButtons,CommandButtonsDisabled,PassiveButtons}`.
  - Map settings (Plan 2b, `docs/superpowers/plans/2026-09-25-moonwell-map-settings.md`): the manifest's
    `settings { info, loadingScreen, gameplay, players, forces, environment, gameplayConstants, gameInterface }` (schema
    in `schema/MapSettings.pkl`). Builds patch the staged `war3map.w3i` (versions 18, 25, 28, 31, 32, 33, 39) byte for
    byte, make the matching World Editor call edits in `war3map.lua`, and merge `war3mapMisc.txt` and `war3mapSkin.txt`,
    after staging and before assets and bundle injection. The source map is never written. `settings:check` reports the
    files that would change; `check` and `dev` run the same planner. Code is in `cli/src/settings/` and `cli/src/w3i/`.
    The in-game release gate (CONTRIBUTING step 8) passed 2026-09-26 after three fixes: no `SetPlayerName` (it crashed
    lobby creation), the hero level key is `MaxHeroLevel`, and w3i colours are stored blue, green, red, alpha (fixture
    `cli/tests/fixtures/map-settings-v39/war3map-colors.w3i`). Sound environments use World Editor's internal names,
    such as `Default` or `Dungeon`.
- **Released 0.3.0** (2026-09-26, JSR `@moonwell/cli@0.3.0` and GitHub release `moonwell@0.3.0`). It adds object data
  (Plan 2c, `docs/superpowers/plans/2026-09-26-moonwell-object-data.md`): custom objects in Pkl under `objects/` (schema
  `schema/ObjectFile.pkl`, `Objects.pkl`, `objects/`, and `generated/*Props.pkl` rendered by `deno task gen` from
  `cli/data/metadata.json`, which `deno task gen:metadata` builds from the game's SLKs). Builds append them to the
  staged map's modification files and their `war3mapSkin.*` counterparts, keeping World Editor's bytes, and write
  `src/generated/objects.yue`; `check` fails when it is stale. `objects:check` and `objects:eval` are new. Code is in
  `cli/src/objectdata/`; the World Editor save the reader and writer are tested against is
  `cli/tests/fixtures/objects-v3-names/`. The in-game gate (CONTRIBUTING step 9) passed 2026-09-26, including per-level
  ability values and upgrade tooltips. The research button shows an upgrade's `tooltip`, not its `name`. The template's
  footman is the Captain (`template/objects/units.pkl`).
- **Released 0.3.1** (2026-09-27, JSR `@moonwell/cli@0.3.1` and GitHub release `moonwell@0.3.1`, checked with `init` and
  `build` from JSR): World Editor 3.00's import flag 29 is read, and kept on sync. Also the small assets items that were
  deferred: `assets:paths` reports every readable model before failing, and tokenizes `.mdl` lazily. Ctrl+C during
  `assets:sync` rolls back. There is no empty state file, and the state-file hint is fixed. The assets release gate
  (CONTRIBUTING step 7, with the World Editor save) passed 2026-09-27.
- **Plan 3a done, unreleased** (2026-09-27, `docs/superpowers/plans/2026-09-27-moonwell-editor-setup.md`): editor
  support for VS Code's YueScript extension (`LiJin.yuescript`) with the Lua extension (`sumneko.lua`), whose bundled
  lua-language-server the YueScript extension uses. `deno task gen:natives <folder> <version>` (`tools/gen-natives.ts`,
  parser in `tools/natives/jass.ts`) reads `war3.w3mod/scripts/common.j` and `blizzard.j` exported with CascView into
  `cli/data/natives.json` (names, types and signatures only; comments are Blizzard's text and are not copied), plus the
  hand-written `tools/natives/lua-extras.json`. `check`, `build`, `test`, `dev` and `setup` write
  `.moonwell/types/{natives,moonwell,objects,map}.d.lua` (git-ignored; code in `cli/src/editor/`); `map.d.lua` comes
  from the source map's `war3map.lua` (fixture `cli/tests/fixtures/map-globals-we3/`). The template has `yueconfig.yue`,
  `.luarc.json` and `.vscode/extensions.json`, and `.gitignore` gains `.moonwell/` and `src/**/*.lua`. `setup` adds
  those to older projects without overwriting, copies the pinned yue to `<cache>/bin` and prints the PATH command when
  `yue` on PATH is missing or another version (`cli/src/yue/bin.ts`). Found by an in-game probe: the game's Lua lacks
  `collectgarbage`, `dofile`, `loadfile`, `debug`, `io` and `package`, and `os` has only `clock`, `date`, `difftime` and
  `time`. The editor must be opened on the project folder itself: lua-language-server reads `.luarc.json` only from the
  first workspace folder. The editor gate (CONTRIBUTING step 10, plan Task 10) passed 2026-09-27 in Antigravity IDE with
  LiJin.yuescript 0.2.9 and sumneko.lua 3.19.1, after one fix: lua-language-server suggests at most
  `completion.maxSuggestCount` globals (default 100), and the extension completes at a placeholder word, so
  `.luarc.json` sets it to 10000. On Windows the extension 0.2.9 opens a console window for `yue` (it does not hide it).
  Our fix, `windowsHide: true` on its two `spawn` calls (pigpigyyy/yuescript-vscode#11), was merged 2026-09-27 and
  awaits an extension release; then drop the README note.
- **Plan 3b done, unreleased** (2026-09-27, `docs/superpowers/plans/2026-09-27-moonwell-unknown-globals.md`): the
  unknown-global check. `compileProject` runs `yue -g` per changed source (cache `dist/stage/lua/.globals.json`, code in
  `cli/src/lint/`), builds the known names from `natives.json`, the source map's `war3map.lua`, `global` lines under
  `src/` and `lint.globals`, and throws a `ProblemsError` (or warns, with `lint.unknownGlobals = "warning"`). Name
  matching is in `cli/src/shared/names.ts`, shared with object data. Its gate (the unknown-global part of CONTRIBUTING
  step 10, plan Task 6) has not been run yet.
- **CI** (GitHub Actions, Ubuntu and Windows) is green as of commit `e312e4a` (release 0.3.1).
- **The manual release gate passed for 0.1.0** in the game. The maintainer plays on Warcraft III Reforged 3.0.0.24268
  with World Editor 3.00, on Windows.

## Next work, in order

1. **Plan 3c, macros:** write it from spec §6 of `docs/superpowers/specs/2026-09-27-moonwell-editor-dx-design.md`.
2. **Release 0.4.0:** 3a, 3b and 3c are released together, after the full release gate in CONTRIBUTING.

Release tags must be `moonwell@<version>`: the Pkl package's `packageZipUrl` downloads from that tag.

## How work was done here, and should continue

- **Process:** design (spec in `docs/superpowers/specs/`), then a plan (`docs/superpowers/plans/`) of small TDD tasks,
  each with exact code and tests. Then implement task by task, test-first, with a review after each task and a final
  review. The maintainer approves each spec before it is implemented.
- **Small bounded changes** get a short design in chat and the maintainer's approval first.
- Commit directly on `main`. The maintainer pushes; then check CI (`gh run list`, `gh run view <id> --log-failed`; `gh`
  is at `C:\Program Files\GitHub CLI`).

## Hard rules

- **No Node.js:** no `package.json`, no `node_modules`, no `npm:` or `node:` specifiers anywhere. Only `jsr:@std/*`,
  from the existing import maps.
- **Errors:** expected failures throw `MoonwellError` (`cli/src/shared/errors.ts`) with `file` and `hint`. Any other
  error is reported as an internal "please report" error, so user mistakes must never reach it.
- **Style:** file system code is async. `deno fmt` uses width 120; `template/`, `docs/` and `cli/src/embedded/` are
  excluded. `deno fmt --check` and `deno lint` must be clean.
- **Generated files:** after changing `template/`, `cli/runtime/moonwell.lua`, `cli/data/game-paths.txt`,
  `cli/data/metadata.json` or `cli/data/natives.json`, run `deno task gen`; the embedded modules in `cli/src/embedded/`
  and `schema/generated/` are freshness-tested. Nothing stray may be left in `template/`: every file there is embedded
  into `init`, and a stray file fails the embedded-template test. `template/src/generated/objects.yue` must match
  `template/objects/` (e2e).
- **Pkl:** `pkl` 0.32 is required. A module property can't be named `output`, because it clashes with Pkl's built-in.
  Pkl `Mapping` values are type-checked lazily: tests that expect a constraint error must force the values (`.toMap()`).

## Checks (all must pass before a commit)

```
deno task check
deno task lint
deno fmt --check
deno task test          # unit tests; need neither pkl nor yue
deno task test:runtime  # needs yue (installed by `deno task setup` in template/)
deno task test:pkl      # needs pkl
deno task test:e2e      # needs pkl and yue
```

## Pitfalls already paid for

- **Backslashes in shell-written files:** Git Bash heredocs and `sed` turn `\\` into `\`. Write files that contain
  backslashes (Windows paths, regexes, Pkl raw strings) with a file-editing tool, not the shell, and check them.
- **Drives:** on the Windows CI runner the checkout is on `D:` and temp folders are on `C:`. Pkl cannot load a local
  project dependency from another drive, so `init --link` refuses that case, and CI points `TMP`/`TEMP` at
  `RUNNER_TEMP`. Reproduce cross-drive problems locally with `subst X: <folder>` (remove it with `subst X: /D`).
- **Unit colour in game:** ally colour mode (Alt+A) in the maintainer's game hides `SetUnitColor`. Errors inside Lua
  timer and trigger callbacks are silent in the game; wrap diagnostics in `pcall`.
- **Model files:** in text `.mdl`, a particle emitter's `Path` sits inside a nested `Particle { }` block. Reforged
  stores particle effects as `.pkb`, and references `.tif` textures that the game stores as `.dds`.
- **World Editor rewrites what it saves:** WE 3.00 saves a custom-path import in `war3map.imp` as flag 29, not the
  documented 13 (fixture `cli/tests/fixtures/imports-we3/`). Any gate step that writes into the source map must also
  save the map in World Editor and then run the commands again.
- **Deno quirks:** it refuses JSR versions published less than 24 hours ago unless you pass `--min-dep-age=0`. A locked
  file on Windows surfaces as a plain `Error` with code `EBUSY`, not a `Deno.errors` class.

## Open, deliberately deferred (small)

- `assets:sync` checks for Ctrl+C only while writing; one pressed during planning takes effect when writing starts.

## Backlog (features for later, each needs a short design first)

- **Custom map preview for Reforged.** Reforged ignores `war3mapPreview.tga`/`.blp` (a long-standing game bug): the map
  list and lobby show `war3mapMap.blp`, the minimap. So `war3mapPreview.tga` stays a reserved asset path (tested). A
  manifest setting such as `settings.info.preview = "preview.blp"` could do the known workaround in the staged map:
  import the image as `war3mapMap.blp`, keep World Editor's minimap under another name, and call
  `BlzChangeMinimapTerrainTex("<that name>")` at game start (in `war3map.lua`, like the other settings edits). See
  github.com/inwc3/ReforgedMapPreviewReplacer. Needs an in-game check of the map list and of the in-game minimap.
- **Fennel support.** The maintainer chose YueScript (2026-09-27) for its familiar syntax and VS Code support, with
  Fennel as a later option: a fennel-ls docset rendered from `cli/data/natives.json` (sub-project 3), plus a Fennel
  compile step next to the YueScript one.
- **Importing existing Lua code** (a Lua library or `.lua` modules in `src/`) belongs to sub-project 4; the bundler
  compiles only `.yue` today.
