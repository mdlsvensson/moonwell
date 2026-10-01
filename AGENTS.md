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
  `.luarc.json` sets it to 10000. On Windows the extension 0.2.9 opened a console window for `yue`; our fix,
  `windowsHide: true` on its two `spawn` calls (pigpigyyy/yuescript-vscode#11), shipped in extension 0.2.10
  (2026-09-27), and the maintainer confirmed on 2026-09-30 that no window opens.
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
- **Released 0.5.1** (2026-09-30, JSR `@moonwell/cli@0.5.1` and GitHub release `moonwell@0.5.1` on `c883e4c`, checked
  with `init` and `build` from JSR): roadmap phase 0. `UnitAlive` is a known native, `assets:sync` stops at Ctrl+C while
  planning, and the README's `yue` console-window note is gone. Gate steps 1 and 2 passed; 3 to 12 were not re-run (no
  game-facing change).
- **Released 0.5.2** (2026-09-30, JSR `@moonwell/cli@0.5.2` and GitHub release `moonwell@0.5.2` on `e459d1d`): builds
  fail when yue writes no Lua for a file with code (the `//` pitfall below). Gate steps 1 and 2 passed; 3 to 12 were not
  re-run.
- **0.6.0, assets shipped by libraries** (2026-10-01; spec
  `docs/superpowers/specs/2026-10-01-moonwell-library-assets-design.md`, plan
  `docs/superpowers/plans/2026-10-01-moonwell-library-assets.md`). Roadmap phase 4, item 2.
  - A library describes itself in a `moonwell-library.json` at its root: `dir` (its module folder) and `assets` (a
    folder of files for the map). JSON, not Pkl, because it is read from a download before anything of the library is
    trusted. An unknown key fails. The manifest's `dir` still wins when it is set; no schema change.
  - Files only: the maintainer chose to keep object data in the map, so the dummy unit of `systems.dummy` stays a pasted
    Pkl block.
  - `cli/src/libraries/manifest.ts` reads the file. `sync.ts` keeps the assets in `.moonwell/library-assets/<key>/`,
    writes an `assets` hash into the lock entry, and stamps library folders with `layout: 2`, so the first run after
    upgrading downloads every GitHub library once more. A lock entry from 0.5 without `assets` is compared by commit
    alone when the download ships assets.
  - `collectProjectAssets` (`cli/src/assets/collect.ts`) adds the libraries' files to the map's own: the map's file wins
    at the same in-map path, with a line saying so, and two libraries at one path fail. `planAssets` takes the library
    keys, so builds, `check`, `assets:check`, `assets:sync` and `assets:paths` all see one list. The three assets
    commands now sync the libraries first.
  - The example library has a `v0.2.0` tag (commit `0b69cfa`) with such a file and one asset; the network test downloads
    both tags. Its tags must never be moved.
  - Gate steps 1 and 2 passed; 3 to 12 were not re-run. No in-game run, by the maintainer's choice.
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
  integration LuaLS runs do diagnose the library's copies in `.moonwell/lua/` (planted checks, 2026-09-30), and since
  roadmap item 1.1 integration also checks `src/wrappers` on its own against the native declarations. The in-game gate
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
- **Wrappers v0.4.0, classic UI, released** (2026-09-29, GitHub pre-release `v0.4.0` on `7e8ef13`; tag consumption
  passed; spec `docs/superpowers/specs/2026-09-29-moonwell-wrappers-classic-ui-design.md`, plan
  `docs/superpowers/plans/2026-09-29-moonwell-wrappers-classic-ui.md`): release B of the UI backlog item. Dialog
  (per-button callbacks, buttons owned by the dialog), Multiboard (one-based cell methods that release every cell
  handle, rows changed one at a time), Leaderboard (items keyed by player), Quest with QuestItem, DefeatCondition and
  TimerDialog. The gate map's `ui-init` probe measured w3ts's classic UI notes on 3.0.0.24268: dialogs and multiboards
  shown directly in `on_main` do not appear, creating quests, leaderboards and multiboards there works, a direct row
  count change from 0 to 5 works, and new multiboard cells show an eye icon (`../wrappers-gate/PROBE-UI-RESULTS.md`).
  The classic UI gate passed normal and minified (`deno task gate ui`, `ui-min`), and the packed map opened in World
  Editor.
- **Wrappers v0.5.0, frames, released** (2026-09-29, GitHub pre-release `v0.5.0` on `b91ffd4`; tag consumption passed;
  spec `docs/superpowers/specs/2026-09-29-moonwell-wrappers-frames-design.md`, plan
  `docs/superpowers/plans/2026-09-29-moonwell-wrappers-frames.md`): release C of the UI backlog item. `wrappers.frame`
  with an owned tree (owned frames, template parts, borrowed game frames), automatic create contexts, per-frame event
  callbacks with the player and synced event data, `releaseFocusFor`, `setVisibleFor` and `Frame.loadTOC`. The gate
  map's `frame-init` probe (`../wrappers-gate/PROBE-FRAME-RESULTS.md`) found on 3.0.0.24268: origin frames exist in
  `on_main`; the game's own templates (nine tried, including `ScriptDialogButton` and `EscMenuBackdrop`) create without
  a TOC; an unknown template gives nil; destroying a frame removes its children, including a re-parented one; a clicked
  button keeps the keyboard focus until `releaseFocusFor`. The frames gate passed normal and minified
  (`deno task gate frames`, `frames-min`).
- **Wrappers v0.5.1, released** (2026-09-30, GitHub pre-release `v0.5.1` on `af9961e`; tag consumption passed with
  Moonwell 0.5.1): roadmap item 0.3. `Effect.attach` and `Effect.flashOn` take a Unit in the editor, so LuaLS flags an
  Item or a Destructable (17 expected negative diagnostics); the runtime still accepts any widget. The in-game gate was
  not re-run (annotations only).
- **Wrappers v0.6.0, the refactor after the review, released** (2026-09-30, GitHub pre-release `v0.6.0` on `933b580`;
  tag consumption passed; review `docs/superpowers/research/2026-09-30-wrappers-review.md`, spec and plan
  `2026-09-30-moonwell-wrappers-refactor`). Roadmap 2.2.
  - Wrapper errors point at the calling line. A tail call into a raising helper had dropped the position entirely, and
    `tests/blame.lua` sweeps every class.
  - A one-lookup method prologue: the fixed cost per method fell from about 120 ns to 70 ns in game. One-table
    enumeration: 20 units cost 27.0 µs, down from 29.2 µs.
  - `isAlive()` through `UnitAlive`, so the library needs Moonwell 0.5.1 or later.
  - `exists()` on Unit, Item and Destructable. A unit removed by raw code reads `false` only from the next frame.
  - Options errors are reported in sorted order, and the README has a per-module API reference.
  - The in-game gate passed `core`, `probes`, `presentation`, `ui` and `perf`. Frames and the minified runs were not
    re-run, by the maintainer's decision.
- **Wrappers v0.7.0, the port prerequisites, released** (2026-09-30, GitHub pre-release `v0.7.0` on `e9c2880`; tag
  consumption passed; spec and plan `2026-09-30-moonwell-wrappers-port-prerequisites`). Roadmap 2.3.
  - `unit:getCollisionSize()` and `unit:setPathing(flag)`.
  - `wrappers.damage`: `onDamaging`/`onDamaged` listeners with one shared event per hit; DAMAGING events set the amount
    and the three types, DAMAGED events only the amount; setters raise once the hit's listeners have run.
  - `wrappers.sync`: `Sync.send` raises over 255 bytes (the game cuts silently); `Sync.on`/`Sync.off` per prefix.
  - The gate map's new `port` run passed. It measured: `isAttack` is false for `damageTarget` even with `attack` true; a
    type change before armor works, after armor it does nothing; the outer hit's setters still work after a nested hit;
    `setPathing(false)` does not make move orders cross trees; sync prefixes of 16, 17 and 32 characters arrive whole.
- **moonwell-systems v0.1.0, release 1 of the `wc3-lib` port, released** (2026-09-30, public repository
  `mdlsvensson/moonwell-systems`, GitHub pre-release `v0.1.0` on `1725436`; tag consumption passed with wrappers
  `v0.7.0`; spec `docs/superpowers/specs/2026-09-30-moonwell-systems-design.md`, plan
  `docs/superpowers/plans/2026-09-30-moonwell-systems-release-1.md`). Roadmap phase 3, release 1.
  - `systems.scheduler` (heap clock, `start()` on one wrappers Timer), `systems.signal`, `systems.scope` and
    `systems.time` (32-bit range, `Time.localUtc()` through `os.time()`).
  - Annotated Lua on the wrappers, with no port interfaces; failures are printed or passed to `onError`, never rethrown.
  - Its tooling has no Deno: `yue -e tests/run.lua`, `tools/check.lua` and `tools/integration.lua` (Lua, through
    `io.popen`).
  - The in-game gate (`deno task gate systems` in `../wrappers-gate`, whose local manifest now lists both libraries)
    passed: the scheduler kept time with a Warcraft timer to within 4 µs.
- **moonwell-systems v0.2.0, release 2 of the port, released** (2026-10-01, GitHub pre-release `v0.2.0` on `afabc3d`;
  tag consumption passed; spec `docs/superpowers/specs/2026-09-30-moonwell-systems-release-2-design.md`, plan
  `docs/superpowers/plans/2026-09-30-moonwell-systems-release-2.md`).
  - `systems.buffs` (Units only; refresh, replace, stack and independent stacking; the store polls its units every 0.25
    s and clears removed, disposed and dead ones), `systems.aura` (members keep the query's order, never a handle-id
    sort) and `systems.dummy` (fresh units, `sourceOf` for attribution), on `systems.internal.ordered`.
  - The README has the Pkl definition of the dummy unit type (`modelFile = ".mdl"` draws nothing; confirmed in game).
  - User callbacks are typed `fun(...): ...`: YueScript returns a callback's last expression, and LuaLS flagged that
    against `fun()` parameters (the gate example found it).
  - The in-game gate (`deno task gate systems`, now both releases, about 18 s) passed.
- **moonwell-systems v0.3.0, release 3 of the port, released** (2026-10-01, GitHub pre-release `v0.3.0` on `d67d3fc`;
  tag consumption passed; spec `docs/superpowers/specs/2026-10-01-moonwell-systems-release-3-design.md`, plan
  `docs/superpowers/plans/2026-10-01-moonwell-systems-release-3.md`).
  - `systems.damage`: `beforeArmor`, `afterArmor` and `observe` listeners on `wrappers.damage`; a Hit changes through
    setter methods that raise at the listener's line, and only the hit being handled may change; `deal` queues script
    damage so it never nests and carries `metadata`; `sourceOf` credits a hit to another Unit (`dummies\sourceOf`).
  - Only failures are reported: a missing or unpaired DAMAGED event and a rejected native call are silent.
  - The plan's code was run in a scratch copy before it was written down, and the plan was assembled from those files.
  - The in-game gate has its own run (`deno task gate systems-damage`, about 10 s) and passed. Measured on 3.0.0.24268:
    Warcraft sends DAMAGED for a hit set to 0; magic damage on a spell-immune unit fires DAMAGING but no DAMAGED; Storm
    Bolt causes two hits (0, then 100, not reduced by armor); a real attack reads `isAttack` true.
  - LuaLS's `--check` mangles a project path that contains `--`, such as Claude's scratchpad folder.
- **moonwell-systems v0.4.0, release 4 of the port, released** (2026-10-01, GitHub pre-release `v0.4.0` on `cfa21b6`;
  tag consumption passed; spec `docs/superpowers/specs/2026-10-01-moonwell-systems-release-4-design.md`, plan
  `docs/superpowers/plans/2026-10-01-moonwell-systems-release-4.md`).
  - `systems.geometry` (vector functions on plain numbers), `systems.terrain` (ground height, walkability, `isClear` and
    the world bounds), `systems.missile` (swept collision, heights above the ground, a filter per missile, piercing,
    range, gravity, steering, `followGround`, an effect that faces its travel) and `systems.knockback` (one per unit, by
    angle, distance and duration, with pathing policies).
  - A probe before the design (`../wrappers-gate/PROBE-PHYSICS-RESULTS.md`) measured on 3.0.0.24268: `IsTerrainPathable`
    sees only terrain, and placing an item sees trees and buildings; `SetUnitX` keeps a unit's order and
    `SetUnitPosition` clears it; a unit's absolute height is `GetLocationZ` plus `GetUnitFlyHeight`; an effect's yaw 0
    points east and a positive pitch points its nose down.
  - The first gate read 3.603 ms per step for 100 missiles among 20 footmen, over the 3 ms budget. A second probe
    (`../wrappers-gate/PROBE-MISSILE-PERF-RESULTS.md`) measured: `GroupEnumUnitsInRange` tests unit origins and clears
    its group first; `IsUnitInRangeXY` is true up to its range plus the unit's collision size; a native costs 0.3 to 0.5
    µs. The missile step now asks `IsUnitInRangeXY` first, and the gate read 1.597 ms. 100 knockbacks under the default
    `"obstacles"` pathing read 2.528 ms (`"terrain"` costs about a seventh of that).
  - The gate has two runs, `deno task gate systems-physics` and `deno task gate systems-knockback`. Both passed. The
    knockbacks got their own run, one footman at a time with each push announced first, because the maintainer could not
    follow five pushes at once; and a turn in a gate step must be large enough to see.
- **moonwell-systems v0.5.0, release 5 and the last of the port, released** (2026-10-01, GitHub pre-release `v0.5.0` on
  `11331a8`; tag consumption passed; spec `docs/superpowers/specs/2026-10-01-moonwell-systems-release-5-design.md`, plan
  `docs/superpowers/plans/2026-10-01-moonwell-systems-release-5.md`).
  - `systems.codec` (save codes packed by a versioned schema into 64 symbols: integers by their range, booleans, strings
    and lists, with a check value keyed by a map secret, a binding and migrations), `systems.sync` (`ask` one player's
    machine for a local value; the answer reaches every machine at the same moment) and `systems.savefile` (`save` and
    `load` in a local file, with the steps that run on one machine only inside the library).
  - Three probes came before the design (`../wrappers-gate/PROBE-PRELOAD-RESULTS.md`), measured on 3.0.0.24268: Lua
    cannot be run from a Preload file (`//!beginusercode` does nothing); a game cache carries text, but `InitGameCache`
    makes a new handle on every call, so a file read on one machine would make handles on that machine only; a file
    whose JASS names a global of blizzard.j crashes the game; tooltips make no handles, and only level 0 of an ability's
    tooltip and extended tooltip keep text, so a file carries one chunk per ability field; 48 of 50 standard unit
    abilities can carry text; 45 sync packets of 250 bytes sent in one burst arrive whole and in order within 0.1 s.
  - The game's integers are 32-bit and the test runner's 64-bit: the codec writes at most 16 bits at a time, masks the
    one multiplication that is meant to wrap, and its fixed codes are compared in game by the gate. A second
    implementation written from the spec's layout gave the same codes.
  - The in-game gate (`deno task gate systems-save`, one machine, nothing to watch) passed: the three fixed codes
    matched; the largest save (8189 symbols, 44 tooltips, 38 packets) took 10 ms to write and 64 ms to read and send. A
    dry run of the compiled gate on stub natives, outside the game, found a bug in the gate example before the
    maintainer ran it.
  - Two machines are untested: the online checks before 1.0 cover the sync and save systems.
  - yue 0.34.2 writes an empty file for a source with a bitwise operator, as for `//` (IppClub/YueScript#256 is fixed in
    0.34.3 for `//`; Moonwell still pins 0.34.2).
- **Wrappers v0.8.0, the additions the port did not need, released** (2026-10-01, GitHub pre-release `v0.8.0` on
  `d823b1b`; tag consumption passed; spec `docs/superpowers/specs/2026-10-01-moonwell-wrappers-additions-design.md`,
  plan `docs/superpowers/plans/2026-10-01-moonwell-wrappers-additions.md`). Roadmap phase 4, item 1.
  - `wrappers.input` (key and mouse listeners for one player, with the event's data and a token to remove them),
    `wrappers.weathereffect`, `Effect.abilityArt` (the model path or lightning code an ability's data names),
    `registerPlayerStateEvent`, `registerPlayerAllianceChange`, `registerGameStateEvent` and `registerTimerExpireEvent`
    on Trigger, and `fromEvent()` on Unit, Player, Item, Destructable, Timer and Region.
  - One change to existing behavior: the four Effect constructors raise for a model that is not a string.
  - Two probes came before the design (`../wrappers-gate/PROBE-EXTRAS-RESULTS.md`), measured on 3.0.0.24268: a held key
    repeats about 30 times a second; the game matches modifier keys exactly, so the module registers all 16
    combinations; typing in chat fires no key event; mouse move fires 150 to 190 times a second; an unknown weather id
    gives a handle with id -1, not nil; `GetAbilityEffectById` reads `""` for a missing entry and the last entry for
    every index past the end; a timer-expiry trigger fires before the timer's callback; a player state event fires at
    every change to a value that satisfies the comparison; an alliance change event carries no player.
  - `onKeyDown` runs once per press by default: the module keeps whether each listened key is held. A release the game
    never sends would cost one press; that is untested.
  - The in-game gate (`deno task gate additions`) passed: its printed lines are read from a file, the maintainer watched
    rain and one effect, and six input steps were advanced with Esc. All 21 weather ids of the README were created.
  - A `wrappers.event` module with every event response was rejected: it would bundle every widget class.
- **The manual release gate passed for 0.1.0** in the game. The maintainer plays on Warcraft III Reforged 3.0.0.24268
  with World Editor 3.00, on Windows.

## Next work, in order

1. **Follow the roadmap** `docs/superpowers/plans/2026-09-30-moonwell-roadmap-to-wc3-lib.md` (approved 2026-09-30). It
   orders everything toward the YueScript port of `wc3-lib` (4d): phase 0 free wins, phase 1 groundwork (static
   native-call check, performance measurement, port-needs inventory, probe batch), phase 2 the wrappers review and
   refactor, phase 3 the port, phase 4 the rest of the backlog. Its last table maps every backlog item to its place.
   Phase 0 is done (2026-09-30: Moonwell 0.5.1 and wrappers v0.5.1), and so are 1.1 (wrappers `227e142`) and 1.2
   (`docs/superpowers/research/2026-09-30-wrappers-performance.md`) and 1.3
   (`docs/superpowers/research/2026-09-30-wc3-lib-port-needs.md`). D1 is decided (option (c), a mix; damage event data
   goes in the wrappers), and 1.4 ran (port-needs note §6). Phase 1 is complete, and so is 2.1
   (`docs/superpowers/research/2026-09-30-wrappers-review.md`; every finding chosen). 2.2 is released as wrappers v0.6.0
   and 2.3 as wrappers v0.7.0, so phase 2 is complete. Phase 3 is under way: moonwell-systems v0.1.0 (release 1:
   scheduler, signal, scope, time), v0.2.0 (release 2: buffs, aura, dummy), v0.3.0 (release 3: damage), v0.4.0 (release
   4: geometry, terrain, missile, knockback) and v0.5.0 (release 5: codec, sync, savefile) are released, so phase 3 is
   complete: the port of `wc3-lib` is done. Phase 4 is under way, in the roadmap's suggested order: item 1 (the wrappers
   additions) is released as wrappers v0.8.0, and item 2 (assets shipped by libraries) as Moonwell 0.6.0. Next is item
   3, the custom map preview for Reforged; every item needs a short design first. Two small changes are waiting too: pin
   YueScript 0.34.3, which fixes the `//` bug upstream, and check whether it fixes bitwise operators; and add a
   `moonwell-library.json` with `{"dir": "src"}` to the wrappers and systems libraries in their next releases, so maps
   can leave `dir` out.

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

- **In-game error positions are bundle lines:** Moonwell bundles every module into one `war3map.lua` chunk. An error
  message caught with `pcall` (not a hook error, which Moonwell maps back to the source) reads `war3map.lua:<line>`.
  Look the line up in the built `dist/stage/map.w3x/war3map.lua`, where each module starts at a `__mw.define` line.

- **YueScript `//` empties a file:** yue 0.34.2 with `-r` or `-m` (both of Moonwell's build modes) writes a 0-byte Lua
  file for any source that uses floor division, and exits 0; without those flags it compiles correctly. Found 2026-09-30
  when a gate-map probe built as one module instead of eleven. `compileSources` now fails on an empty output for a file
  with code (`emptyOutputError` in `cli/src/yue/compile.ts`). Write `math.floor(a / b)`. Reported upstream as
  IppClub/YueScript#256; once a fixed yue is pinned, the floor-division test in `cli/tests/yue/compile.test.ts` fails
  and should become a test that such a file compiles.

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
  wrappers v0.3.0. B is wrappers v0.4.0, released 2026-09-29 (spec
  `docs/superpowers/specs/2026-09-29-moonwell-wrappers-classic-ui-design.md`). C is wrappers v0.5.0, released 2026-09-29
  (spec `docs/superpowers/specs/2026-09-29-moonwell-wrappers-frames-design.md`). B's `ui-init` probe (2026-09-29)
  measured w3ts's classic UI notes; the results are in the wrappers README and the w3ts comparison.
- **Assets shipped by libraries.** Done in Moonwell 0.6.0 (2026-10-01), for files; object data stays in the map. The
  entry as it was: a library could ship files such as a frame template `.toc` and its `.fdf` files for the map to import
  (added 2026-09-29 with wrappers v0.5.0, whose README shows the manual recipe). Needs a design for where they live in a
  library and how they join the map's `assets/` import.
- **Wrappers candidate additions** (from the w3ts comparison §6, chosen 2026-09-29 as backlog candidates, each needing a
  short design): `WeatherEffect` (`AddWeatherEffect`, enable, remove); spell effects from ability data
  (`AddSpellEffectById`, for example `Effect.flashSpell`); more Trigger registrations (player state, key, mouse, sync,
  alliance change, game state, timer expire); event helpers such as `Unit.fromEvent()`. Done: wrappers v0.8.0
  (2026-10-01), with keys and the mouse as `wrappers.input` and spell effects as `Effect.abilityArt`.
- **Automatic disposal of Unit wrappers on removal** (WCSharp comparison §2.1 and §7.5; backlogged 2026-09-29, the
  maintainer wants to know more before deciding). Today a unit the game removes by itself (decay, removal by other code)
  keeps a live-looking wrapper whose handle is dead. WCSharp detects removal as a unit leaving a region that covers the
  world bounds. **The probe run (2026-09-29) found that this does not work on 3.0.0.24268:** the leave event fired for
  none of `RemoveUnit`, an exploded death, a summoned timed-life death or a normal death left 120 s to decay (enter did
  fire at creation). **The 1.4 probe (2026-09-30) found the undefend-order trick does work:** a unit with a Defend copy
  gets order 852056 twice at death (Defend level 1) and twice at removal (level 0), for all four cases, with no false
  positives (port-needs note §6.3). It needs a custom ability in every map's object data; polling type ids, as `wc3-lib`
  does, needs none.
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
  `sound:getDuration()` agrees across machines, the wrappers v0.8.0 input listeners and `weather:enableFor`, and any
  later feature with multiplayer effects. Until then, release gates record these as deferred, not passed.
- **YueScript port of `wc3-lib`** (sub-project 4d): `@mdlsvensson/wc3-lib` (TypeScript on JSR, about 6,000 lines:
  scheduler, buffs, dummies, damage, missiles and knockback, save codes) ported to YueScript as a Moonwell library.
  Moved here 2026-09-28; it depends on 4b's library sync. The 1.4 probe found `wc3-lib`'s Preload local store broken in
  game for codes over one chunk (lines cut at 259 characters, which can crash the game on load; appending inside the
  file has no effect): port-needs note §6.5. Under way as `mdlsvensson/moonwell-systems` (annotated Lua, not YueScript,
  by the maintainer's choice); complete: v0.1.0 to v0.5.0 are released (2026-10-01).
- **Replace Deno in Moonwell's toolchain.** The maintainer's plan (2026-09-30): the project has nothing to do with
  TypeScript, so Deno should go eventually, from the CLI and from the wrappers' tools. moonwell-systems already needs
  none (its tools are Lua run with `yue -e`). Needs a design: what replaces the CLI's runtime, JSR publishing and the
  test suites.
