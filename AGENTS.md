# AGENTS.md: handoff for coding agents

Moonwell is a Warcraft III map development framework. Gameplay is written in YueScript and compiled to Lua 5.3; project
data is written in Pkl; the toolchain is a Deno CLI published to JSR as `@moonwell/cli`. The Pkl schemas are published
as the Pkl package `moonwell` (a GitHub release tagged `moonwell@<version>`). This file tells you what exists, the
rules, the known pitfalls, and what to do next. It was written by the previous agent (Claude) on 2026-09-25 when handing
over.

## Read first

- `docs/superpowers/specs/2026-09-24-moonwell-core-design.md` is the **binding design** for the whole project. Its §6
  describes the data layers still to build: §6.1 object data and §6.3 map settings. §6.2 assets is done.
- `README.md` (user docs), `CONTRIBUTING.md` (checks, release gate, publishing), `CHANGELOG.md` (0.1.0 released,
  `## Unreleased` lists what is done since).
- Later specs, all implemented: `2026-09-25-moonwell-model-paths-design.md` and
  `2026-09-25-moonwell-in-game-paths-design.md`. Plans for everything built so far are in `docs/superpowers/plans/`;
  follow their style when writing new plans.

## State (2026-09-25)

- **Released:** 0.1.0, on JSR (`@moonwell/cli@0.1.0`) and as a GitHub release (`moonwell@0.1.0`). It contains the
  toolchain: `init`, `setup`, `build`, `test`, `dev`, `check`; the YueScript bundler with runtime hooks; the MPQ writer;
  and the manifest in Pkl.
- **Done on `main`, unreleased:**
  - Assets (Plan 2a): `assets/` is imported into builds. The manifest's `assets { paths {}; exclude = List() }` maps and
    excludes files. `assets:check` / `assets:sync` write the assets into the source map with ownership in
    `.asset-state/`.
  - `assets:paths [file]`: lists the files a model (`.mdx`/`.mdl`) references as `in-game path`,
    `in-game path, replaced`, `custom path, imported` or `custom path, not imported`, with `\` like World Editor's
    Import Manager. It uses an embedded list of 42,127 in-game paths generated from WC3 3.0.0.24268
    (`cli/data/game-paths.txt`).
  - `init` creates the icon folders `assets/ReplaceableTextures/{CommandButtons,CommandButtonsDisabled,PassiveButtons}`.
- **CI** (GitHub Actions, Ubuntu and Windows) is green as of commit `eea99d9`.
- **The manual release gate passed for 0.1.0** in the game. The maintainer plays on Warcraft III Reforged 3.0.0.24268
  with World Editor 3.00, on Windows.

## Next work, in order

1. **Plan 2b: map settings** (spec §6.3). Needs nothing from the maintainer.
   - **What:**
     `settings { info, loadingScreen, gameplayConstants, gameInterface, gameplay, players, forces, environment }` in the
     manifest. It patches `war3map.w3i` (versions 18, 25, 28, 31, 32, 33, 39), merges `war3mapMisc.txt` and
     `war3mapSkin.txt`, and makes coordinated `config()` edits in `war3map.lua`. It also adds a `settings:check`
     command. Settings are applied to the staged map after staging and before bundle injection.
   - **Reference implementation to port** (read only; it's TypeScript with some npm dependencies you must not copy):
     `C:\Users\mdlsvensson\Repo\wc3-dev-framework\scripts\map-settings\` (`settings.ts`, `map-info.ts`, `binary.ts`,
     `lua.ts`, `options.ts`), with its schema in `map-settings-schema.pkl` and tests in
     `scripts\tests\map-settings*.ts`. Its `lua.ts` uses `createRequire`; replace that with plain Deno code.
   - **Template rule, decided with the maintainer:** the template's `settings` block writes out every setting with its
     default value. A setting stays commented out only when it needs a World Editor step first, and its comment names
     that step. Lists are Pkl `List`, never `Listing` (amending a `Listing` default appends to it). Advanced settings
     are left out of the template but documented. See spec §3.3.
2. **Plan 2c: object data** (spec §6.1): custom units, heroes, buildings, items, abilities, buffs and upgrades in Pkl,
   written into the map's modification files (`w3u`/`w3t`/`w3h`/`w3a`/`w3q` and their `war3mapSkin.*` counterparts),
   plus the generated `src/generated/objects.yue`.
   - **Blocked on the maintainer.** It needs the game's SLKs (`UnitMetaData`, `AbilityMetaData`, `AbilityBuffMetaData`,
     `UpgradeMetaData`, and the `UnitData`/`ItemData`/`AbilityData`/`AbilityBuffData`/`UpgradeData` id lists), extracted
     with CascView. It also needs a small test map saved by World Editor 3.00 with one custom object of each kind, to
     confirm the version-3 file format and the skin split (spec §11).
   - The reference framework used the npm `war3-objectdata-th` package for this; Moonwell must write its own
     reader/writer.
   - Payoff: the template can restore the original footman-as-Captain (object data swaps its model).
3. **Release 0.2.0** after 2b (or 2c). Follow CONTRIBUTING's release gate and publishing steps.
   - Bump the version in `cli/deno.json`, `cli/src/version.ts` and `schema/PklProject`.
   - `pkl project package` may need `--skip-publish-check` in sandboxed shells.
   - For 24 hours after publishing, Deno blocks the new version unless you pass `--min-dep-age=0`.

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
- **Generated files:** after changing `template/`, `cli/runtime/moonwell.lua` or `cli/data/game-paths.txt`, run
  `deno task gen`; the embedded modules in `cli/src/embedded/` are freshness-tested. Nothing stray may be left in
  `template/`: every file there is embedded into `init`, and a stray file fails the embedded-template test.
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
- **Deno quirks:** it refuses JSR versions published less than 24 hours ago unless you pass `--min-dep-age=0`. A locked
  file on Windows surfaces as a plain `Error` with code `EBUSY`, not a `Deno.errors` class.

## Open, deliberately deferred (small)

- `assets:paths`: with no file argument, one unreadable model aborts the whole report. Very large `.mdl` files use a lot
  of memory, because the tokenizer builds an array.
- `assets:sync` on Ctrl+C has no rollback (the reference framework behaves the same). A target starting with `war3map`
  (e.g. `war3mapPreview.tga`) is rejected as reserved.
- The asset state-file error carries a slightly off hint. `assets:sync` with no assets writes an empty
  `.asset-state/<map>.json`.
