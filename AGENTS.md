# AGENTS.md: handoff for coding agents

Moonwell is a Warcraft III map development framework. Gameplay is written in YueScript and compiled to Lua 5.3; project
data is written in Pkl; the toolchain is a Deno CLI published to JSR as `@moonwell/cli`. The Pkl schemas are published
as the Pkl package `moonwell` (a GitHub release tagged `moonwell@<version>`). This file tells you what exists, the
rules, the known pitfalls, and what to do next. It was written by the previous agent (Claude) on 2026-09-25 when handing
over, and updated on 2026-09-29 after wrappers v0.2.0 was implemented and again after wrappers v0.3.0 was implemented.

## Read first

- `docs/superpowers/specs/2026-09-24-moonwell-core-design.md` is the **binding design** for the whole project. Its §6
  describes the data layers: §6.1 object data, §6.2 assets and §6.3 map settings are all implemented.
- `README.md` (user docs), `CONTRIBUTING.md` (checks, release gate, publishing), `CHANGELOG.md` (one section per
  release, each with its release gate; add `## Unreleased` above the newest for work done since).
- Later specs, all implemented: `2026-09-25-moonwell-model-paths-design.md`,
  `2026-09-25-moonwell-in-game-paths-design.md`, `2026-09-25-moonwell-map-settings-design.md` and
  `2026-09-26-moonwell-object-data-design.md`. `2026-09-27-moonwell-editor-dx-design.md` (sub-project 3) is implemented
  by Plans 3a, 3b and 3c, released together as 0.4.0. Plans for everything built so far are in
  `docs/superpowers/plans/`; follow their style when writing new plans.
- `docs/superpowers/research/2026-09-29-w3ts-comparison.md` compares w3ts with moonwell-wrappers v0.3.0: native caveats
  we lacked, inputs for wrappers release B and for 4d, and follow-ups for the maintainer to choose from.
  `2026-09-29-wrappers-advantages.md` explains our advantages with code examples, and `2026-09-29-wcsharp-comparison.md`
  does the same comparison for WCSharp (C#) and revises the w3ts conclusions (§8).

## State (2026-09-29)

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
- **Released 0.4.0** (2026-09-28, JSR `@moonwell/cli@0.4.0` and GitHub release `moonwell@0.4.0` on `a9a2e15`, checked
  with `init` and `build` from JSR): sub-project 3, Plans 3a, 3b and 3c below. The release gate passed steps 1–6, 10 and
  11; 7–9 were not re-run (their code is unchanged). Pkl's HTTP client fails in Claude's shells with "Unable to
  establish loopback connection", even outside the sandbox, so the maintainer runs the JSR `init` check.
- **Plan 3a** (2026-09-27, `docs/superpowers/plans/2026-09-27-moonwell-editor-setup.md`): editor support for VS Code's
  YueScript extension (`LiJin.yuescript`) with the Lua extension (`sumneko.lua`), whose bundled lua-language-server the
  YueScript extension uses. `deno task gen:natives <folder> <version>` (`tools/gen-natives.ts`, parser in
  `tools/natives/jass.ts`) reads `war3.w3mod/scripts/common.j` and `blizzard.j` exported with CascView into
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
- **Plan 3b** (2026-09-27, `docs/superpowers/plans/2026-09-27-moonwell-unknown-globals.md`): the unknown-global check.
  `compileProject` runs `yue -g` per changed source (cache `dist/stage/lua/.globals.json`, code in `cli/src/lint/`),
  builds the known names from `natives.json`, the source map's `war3map.lua`, `global` lines under `src/` and
  `lint.globals`, and throws a `ProblemsError` (or warns, with `lint.unknownGlobals = "warning"`). Name matching is in
  `cli/src/shared/names.ts`, shared with object data. Its gate (CONTRIBUTING step 10, re-run in full) passed 2026-09-27
  after one editor fix: `moonwell.d.lua` declares `require` (`.luarc.json` turns off LuaLS's `package` library, which
  removed it) and types hook callbacks `fun(): ...` (YueScript returns the last expression). To check editor diagnostics
  without the editor, run the Lua extension's bundled server:
  `<extensions>/sumneko.lua-<version>/server/bin/lua-language-server --check=<project> --checklevel=Hint` after
  compiling the `.yue` files with `yue -l -c --target=5.3` (what the YueScript extension writes on save).
- **Plan 3c** (2026-09-27, `docs/superpowers/plans/2026-09-27-moonwell-macros.md`): macros. `cli/runtime/macros.yue`
  (embedded as `MACROS_YUE`) exports `$FourCC`; `refreshEditorFiles` writes it to `.moonwell/yue/moonwell/macros.yue`,
  and every `yue` run gets `--path <root>/.moonwell/yue/?.lua` (`cli/src/yue/macros.ts`), with the module's hash in the
  compile and `yue -g` cache keys. The macro module must not have a backslash inside a string literal: yue 0.34.2 fails
  to load such a macro. The template's standard Footman, next to the Captain, uses `$FourCC`. Its gate (CONTRIBUTING
  step 11) passed 2026-09-27. That probe reported screen-only `print` output; the later 4c gate confirmed wrapper
  callback errors, ticks and cleanup messages were retained in F12 on 3.0.0.24268.
- **Released 0.5.0** (2026-09-28, JSR `@moonwell/cli@0.5.0` and GitHub release `moonwell@0.5.0` on `27d093a`, checked
  with `init` and `build` from JSR): sub-project 4, Plans 4a and 4b below. The release gate passed steps 1–6 and 12;
  7–11 were not re-run (their code is unchanged).
- **Plan 4a, released in 0.5.0** (2026-09-28, `docs/superpowers/plans/2026-09-28-moonwell-lua-modules.md`): Lua modules
  in `lua/` (spec `docs/superpowers/specs/2026-09-28-moonwell-lua-libraries-design.md`). `collectModules`
  (`cli/src/bundle/modules.ts`) lists `src/**/*.yue` and `lua/**/*.lua` as one namespace; `moduleLoader` resolves a name
  or `<name>.init`; Lua modules are bundled unchanged and keep their lines in `--minify` builds (per-entry flag in the
  line table); `luaTopLevelGlobals` (`cli/src/lint/lua-globals.ts`) makes their top-level globals known; `mergeLuarc`
  lets `setup` add `.luarc.json` entries to older projects. Its gate (CONTRIBUTING step 12, the Lua modules part) passed
  2026-09-28.
- **Plan 4b, released in 0.5.0** (2026-09-28, `docs/superpowers/plans/2026-09-28-moonwell-libraries.md`): libraries,
  from a GitHub tag or a local `path`, in the manifest's `libraries` block (spec §4). `cli/src/libraries/` reads the tag
  archive (`archive.ts`, `download.ts`), writes `moonwell.lock` (`lock.ts`) and syncs `.moonwell/libraries/<key>/`
  (`sync.ts`) at the start of every compile (`compileProject`) and in `setup`; a moved tag fails. Sync refuses keys that
  differ only by case, a `github` repository of `.`/`..`, a tag with `.`/`..` segments and a local library whose folder
  contains the project's `.moonwell/libraries`. Library modules join the one namespace (`libraryModuleRoots`). Compile
  outputs are keyed by project path (`outputPathOf` in `cli/src/yue/compile.ts`: libraries under
  `dist/stage/lua/.libraries/<key>/`). `cli/src/editor/library-view.ts` writes `.moonwell/lua/` (`setup`: the Lua
  modules only). Known names come from the resolved graph, and only the `src/` modules the map requires are checked.
  `isWithin` is shared in `cli/src/shared/fs.ts`. The example library is `mdlsvensson/moonwell-example-lib` `v0.1.0`
  (commit `13e35535c481fddd267533cc513f86b55b313b66`), which `deno task test:network` downloads. In a library, a `.lua`
  beside a `.yue` of the same stem is its compiled output; a local override keeps the lock entry. Its gate (CONTRIBUTING
  step 12, the libraries part) passed 2026-09-28.
- **CI** (GitHub Actions, Ubuntu and Windows, with `test:network`) is green as of commit `632c765` (Plan 4b).
- **Plan 4c, released as wrappers `v0.1.0`** (2026-09-28): annotated Lua wrappers in the separate sibling
  `../moonwell-wrappers` repository, commit `3b923d5` on main. Player, Unit, Timer, Trigger, Group and Effect use
  explicit methods, stable handle identity and explicit cleanup. Consume with `path = "../moonwell-wrappers"` and
  `dir = "src"`, or from GitHub `mdlsvensson/moonwell-wrappers` tag `v0.1.0` (commit `c1209f5`, a pre-release). Its 23
  behavior tests, Lua 5.3.6 syntax check, real Moonwell normal/minified builds, bundle runtime and LuaLS 3.19.1
  positive/negative fixtures pass. `fromHandle` is conservatively nullable in LuaLS: narrow or assert the result. All
  six wrappers are exercised by its `examples/gate.yue`. The maintainer passed normal gameplay/cleanup, intentional
  callback-error recovery, minified gameplay and World Editor opening on 2026-09-28 (game 3.0.0.24268, editor 3.00).
  First published-tag consumption passed 2026-09-28: a fresh map locked `v0.1.0` to `c1209f5`, and the lock stayed
  unchanged after removing `.moonwell/`. Moonwell CLI code is unchanged.
- **Wrappers v0.2.0, broad coverage, released** (2026-09-29, spec
  `docs/superpowers/specs/2026-09-28-moonwell-wrappers-broad-design.md`, plan
  `docs/superpowers/plans/2026-09-28-moonwell-wrappers-broad.md`): GitHub pre-release `v0.2.0` of
  `mdlsvensson/moonwell-wrappers` on commit `7baa81e`. Adds Item, Destructable, Rect, Region and Force; deeper Unit,
  Player, Trigger (predicate conditions, removable action/condition tokens) and Group (filtered enumerations, `forEach`,
  `first`); a widget layer; weak Unit/Item/Destructable caches. Wrapper arguments convert through the loaded registries,
  so a module imports another only to return its wrappers. Automated checks pass: 69 behavior tests, Lua 5.3.6 syntax,
  Moonwell normal/minified builds, LuaLS positive and 8 negative diagnostics, and a Trigger-only bundle check. The
  integration LuaLS runs do not diagnose the library's own files; check `src` with a direct LuaLS run. The in-game gate
  passed 2026-09-29 (game 3.0.0.24268): the weak cache probe printed `collected=true stale=true identity=true`, and the
  self-removing action and condition ran. Text printed while a map loads never reaches the screen or the log, so the
  gate starts from a zero-second timer. The two-player desync run is deferred to the online checks before 1.0 (Backlog).
  Tag consumption passed: a fresh map locked `v0.2.0` to `7baa81e`.
- **Wrappers v0.3.0, presentation, released** (2026-09-29, GitHub pre-release `v0.3.0` of
  `mdlsvensson/moonwell-wrappers` on commit `1277875`; spec
  `docs/superpowers/specs/2026-09-29-moonwell-wrappers-presentation-design.md`, plan
  `docs/superpowers/plans/2026-09-29-moonwell-wrappers-presentation.md`): release A of the UI and presentation backlog
  item. TextTag, Sound, Lightning, Image, Ubersplat and FogModifier (owned objects only), `TextTag.float`,
  `Sound.playOnce`, `Effect.flash`/`flashOn`, `setVisibleFor`/`playFor`, deeper Effect and `Item`/`Destructable`
  `enumInRect`. Automated checks pass (107 behavior tests). The in-game gate passed 2026-09-29, normal and minified,
  after four gate fixes found in game: trees snap to a 64-unit grid, Chain Lightning fades by itself (the gate uses
  Drain Life), `lightning:setColor` shows no visible change, and effects attached to items and destructables are not
  drawn (the last two documented in the wrappers README). Tag consumption passed: a fresh map locked `v0.3.0` to
  `1277875`. The disposable gate map is `../wrappers-gate` (`deno task gate <run>`, one command per run).
- **Wrappers v0.3.1, native caveats, released** (2026-09-29, GitHub pre-release `v0.3.1` on `94d650f`; tag consumption
  passed): from the w3ts and WCSharp comparisons (`docs/superpowers/research/`) and an in-game probe run
  (`../wrappers-gate`, `deno task gate probe`; results in the WCSharp note §9). README notes measured by the probe and a
  labelled "Reported native caveats" section, `Image.create` raising on a wrong path (Warcraft returns an image with
  handle id -1, not nil), and setter checks that flip booleans. The in-game gate was not re-run by the maintainer's
  decision. Moonwell now knows `UnitAlive` (added to `tools/natives/lua-extras.json`, `natives.json` regenerated from
  the same `common.j` export with no other change); the wrappers' `unit:isAlive()` is unchanged, so the library still
  works with Moonwell 0.5.0.
- **Wrappers v0.4.0, classic UI, implemented; in-game gate pending** (spec
  `docs/superpowers/specs/2026-09-29-moonwell-wrappers-classic-ui-design.md`, plan
  `docs/superpowers/plans/2026-09-29-moonwell-wrappers-classic-ui.md`): release B of the UI backlog item. Dialog
  (per-button callbacks, buttons owned by the dialog), Multiboard (one-based cell methods that release every cell
  handle, rows changed one at a time), Leaderboard (items keyed by player), Quest with QuestItem, DefeatCondition and
  TimerDialog. Automated checks pass; the maintainer runs the gate map's `ui-init` probe, then CONTRIBUTING's gate with
  `ui = true` (`deno task gate ui`, then `ui-min`).
- **The manual release gate passed for 0.1.0** in the game. The maintainer plays on Warcraft III Reforged 3.0.0.24268
  with World Editor 3.00, on Windows.

## Next work, in order

1. **Finish wrappers v0.4.0:** the `ui-init` probe and the in-game gate (the wrappers repo's CONTRIBUTING step 8, normal
   and minified); record the probe's answers in README and move the measured w3ts notes out of the backlog item below;
   then release it like v0.3.0 (tag `v0.4.0`, tag consumption gate).
2. **Then choose the next sub-project with the maintainer:** wrappers release C (frames), the editor error for effects
   attached to items and destructables, the YueScript port of `wc3-lib` (4d) or the Reforged map preview. Each needs a
   short design or a spec first. 4d has design inputs in the w3ts comparison §2.1 and the WCSharp comparison §2.1, whose
   systems are the closest prior art.

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
- **Generated files:** after changing `template/`, `cli/runtime/moonwell.lua`, `cli/runtime/macros.yue`,
  `cli/data/game-paths.txt`, `cli/data/metadata.json` or `cli/data/natives.json`, run `deno task gen`; the embedded
  modules in `cli/src/embedded/` and `schema/generated/` are freshness-tested. Nothing stray may be left in `template/`:
  every file there is embedded into `init`, and a stray file fails the embedded-template test.
  `template/src/generated/objects.yue` must match `template/objects/` (e2e).
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
deno task test:network  # needs the network; runs only with MOONWELL_NETWORK_TESTS=1 (CI sets it)
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
- **TypeScript support.** Gameplay in TypeScript, compiled to Lua (as TypeScriptToLua does in wc3-dev-framework), next
  to YueScript. Added 2026-09-28; it builds on sub-project 4a's Lua modules. TypeScriptToLua is an npm package, so the
  "No Node.js" rule shapes the design.
- **C# support.** Gameplay in C#, compiled to Lua (for example with CSharp.lua). Added 2026-09-28; it builds on
  sub-project 4a's Lua modules.
- **Teal support.** Gameplay in Teal (typed Lua, compiled by `tl`), next to YueScript. Added 2026-09-28; it builds on
  sub-project 4a's Lua modules. The `tl` compiler is itself written in Lua, so it can run without Node.js; its type
  declarations (`.d.tl`) could be rendered from `cli/data/natives.json` like the editor's `natives.d.lua`.
- **UI wrappers, releases B and C.** Split 2026-09-29 from "UI and presentation wrappers"; release A (presentation) is
  wrappers v0.3.0. B, v0.4.0: implemented (spec
  `docs/superpowers/specs/2026-09-29-moonwell-wrappers-classic-ui-design.md`), gate pending. C, v0.5.0: the `BlzFrame`
  API, with its own ownership design (TOC/FDF loading, parent trees, local frames). B's `ui-init` probe (2026-09-29)
  measured w3ts's classic UI notes; the results are in the wrappers README and the w3ts comparison.
- **Wrappers candidate additions** (from the w3ts comparison §6, chosen 2026-09-29 as backlog candidates, each needing a
  short design): `WeatherEffect` (`AddWeatherEffect`, enable, remove); spell effects from ability data
  (`AddSpellEffectById`, for example `Effect.flashSpell`); more Trigger registrations (player state, key, mouse, sync,
  alliance change, game state, timer expire); event helpers such as `Unit.fromEvent()`.
- **Static native-call check for the wrappers** (WCSharp comparison §7.3; backlogged 2026-09-29, the maintainer wants to
  know more before deciding). A check in the wrappers' `deno task test` that reads every native call in `src/wrappers/`
  and compares it with `cli/data/natives.json`: the native exists, and the argument count matches. It would catch a
  wrong native name or a missing argument without game or doubles, as WCSharp's Roslyn API checker does for its
  templates. Open questions: how to find calls reliably in Lua (a tokenizer, not a regex), and how the wrappers
  repository gets `natives.json` (copy, or read from `../moonwell`).
- **Automatic disposal of Unit wrappers on removal** (WCSharp comparison §2.1 and §7.5; backlogged 2026-09-29, the
  maintainer wants to know more before deciding). Today a unit the game removes by itself (decay, removal by other code)
  keeps a live-looking wrapper whose handle is dead. WCSharp detects removal as a unit leaving a region that covers the
  world bounds. **The probe run (2026-09-29) found that this does not work on 3.0.0.24268:** the leave event fired for
  none of `RemoveUnit`, an exploded death, a summoned timed-life death or a normal death left 120 s to decay (enter did
  fire at creation). Another detection method would be needed (for example the undefend-order trick unit indexers use);
  unexplored.
- **Editor error for effects attached to items and destructables.** The v0.3.0 gate (2026-09-29, game 3.0.0.24268)
  showed that Warcraft drew no effect attached to an item (Claws of Attack, two effect models) or a destructable (a
  summer tree). `Effect.attach` and `Effect.flashOn` accept any Widget, like the native, and the README documents the
  limit. The maintainer wants the editor (LuaLS) to flag an Item argument there (decided for items; destructables follow
  the same evidence), for example with a Unit parameter type, while runtime behavior may stay permissive for custom
  models.
- **Online multiplayer and desync checks: the very last step before 1.0.** The maintainer decided (2026-09-29) that
  every online and desync check waits until then: Reforged's latest patch removed LAN, and it needs a second player on
  Battle.net. Covers at least: the wrappers weak-cache gate (`examples/gate.yue`, two players past its 50-second probe,
  no desync, each machine's probe line recorded), `Player:isLocal()`, `Group:enumSelected`, map settings (players,
  forces, alliances) in a real lobby, map transfer of packed normal and minified builds, the wrappers v0.3.0 local
  visibility (`setVisibleFor`, `playFor`, the `player` options of `TextTag.float` and `Sound.playOnce`) and whether
  `sound:getDuration()` agrees across machines, and any later feature with multiplayer effects. Until then, release
  gates record these as deferred, not passed.
- **YueScript port of `wc3-lib`** (sub-project 4d): `@mdlsvensson/wc3-lib` (TypeScript on JSR, about 6,000 lines:
  scheduler, buffs, dummies, damage, missiles and knockback, save codes) ported to YueScript as a Moonwell library.
  Moved here 2026-09-28; it depends on 4b's library sync.
