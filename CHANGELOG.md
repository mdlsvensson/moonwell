# Changelog

## Unreleased

- Editor support for VS Code's YueScript extension: new projects get `yueconfig.yue`, `.luarc.json` and a recommendation
  of the YueScript and Lua (`sumneko.lua`) extensions, and `check`, `build`, `test` and `dev` write LuaLS declarations
  to `.moonwell/types/` for every native, Blizzard.j function and global of Warcraft III 3.0.0.24268, the Moonwell
  runtime, the project's object ids and the map's own globals. `.gitignore` gains `.moonwell/` and `src/**/*.lua` (the
  `.lua` files the extension writes on save). See "Editor setup" in the README.
- The editor declarations include `require`, so `import` lines are not flagged as an undefined global (the game's Lua
  has no `package` library, which `.luarc.json` turns off, but the Moonwell runtime defines `require`). Hook callbacks
  may return a value, so a YueScript callback, which returns its last expression, is not flagged as returning too many
  values.
- `setup` adds the editor files and `.gitignore` lines older projects lack, keeps a copy of the pinned YueScript in the
  cache's `bin` folder, and prints the command that puts it on PATH when `yue` is missing there or another version.
- `setup` now also plans the project's objects to write the editor declarations, so objects that do not resolve, or a
  missing or unreadable source map when there are objects, make `setup` fail as they make `check` fail.
- `check`, `build`, `test` and `dev` report every global a gameplay file uses that nothing defines, with its file, line
  and column and the nearest known name (`Did you mean CreateUnit?`). Known globals are the game's natives, every
  common.j and Blizzard.j function, global and constant, the game's Lua libraries, the source map's `war3map.lua`
  globals, names declared with `global` under `src/` and the new `lint.globals` list. `lint.unknownGlobals = "warning"`
  reports them without failing. New projects show the `lint` block in `moonwell.pkl`.
- Macros: `import "moonwell.macros" as {:$FourCC}` gives gameplay code `$FourCC "hfoo"`, which compiles to the rawcode's
  integer (`1751543663`) and fails the compile on anything but a 4-character string literal. The template creates a
  standard Footman with it next to the Captain.
- After upgrading, run `deno task setup` once: it adds the editor files and the new `.gitignore` lines. Until then,
  `check` and `build` leave `.moonwell/` untracked in a 0.3 project. `check` and `build` now fail on unknown globals by
  default, so a project that relies on `global *`, `global ^` or globals defined elsewhere should list them in
  `lint.globals`, or set `lint.unknownGlobals = "warning"` for a while.

### Release gate (so far)

Editor and unknown globals (CONTRIBUTING step 10) passed 2026-09-27, tested by the maintainer in Antigravity IDE (a VS
Code fork) with the YueScript extension 0.2.9 and the Lua extension 3.19.1, and re-run in full after Plan 3b:

- In the editor: completion and hover for natives, `mw.on_main`, `objects.units.captain`, a second project module, and
  the `gg_unit_` global of a unit placed in World Editor and used by a trigger. `CreatUnit` is underlined.
- `deno task check` fails with `src/main.yue:9:10 › Unknown global CreatUnit.` and `Did you mean CreateUnit?`; with
  `lint.unknownGlobals = "warning"`, `deno task build` prints the same lines as warnings and succeeds. `git status`
  shows no generated files.
- The re-run found two editor warnings, now fixed: `import` lines were flagged as using an undefined `require`, and a
  hook callback whose last line has a value was flagged as returning too many values. The fix was checked with the Lua
  extension's lua-language-server run on the project from the command line (`--check`): no warnings remain for the
  template's code, and `CreatUnit` is still flagged.

Macros (CONTRIBUTING step 11), in part, 2026-09-27, on Warcraft III Reforged 3.0.0.24268: in a new project,
`deno task
test` shows the standard Footman made with `$FourCC("hfoo")` beside the Captain, tested by the maintainer.
The built map contains `CreateUnit(Player(0), 1751543663, -45, -650, 270)`, and the Lua extension's lua-language-server,
run from the command line on the `.lua` the YueScript extension writes, finds no problems in the project. Still to run:
the Lua probe, and the macro import and `$FourCC` in the editor itself.

## 0.3.1 (2026-09-27)

- Fixed: after World Editor 3.00 saved a map with synced assets, every command that reads `war3map.imp` (`build`,
  `assets:check`, `assets:sync`) failed with "unknown flag 29". World Editor saves custom-path imports with flag 29,
  which is now read as a custom path. `assets:sync` keeps the flag an owned import already has, so saving in World
  Editor leaves nothing to sync.
- `assets:paths` with no file argument reports every model it can read and marks each unreadable one in its place,
  instead of stopping at the first; the command still fails when any model was unreadable.
- Ctrl+C during `assets:sync` undoes every change it already made to the source map and exits with code 130. A second
  Ctrl+C still exits at once.
- `assets:sync` writes no `.asset-state/<map>.json` when it owns no files, and removes the file once the last asset is
  gone. An invalid path in the state file now carries the state file's hint.
- `assets:paths` reads large text `.mdl` models with far less memory (about a tenth for a 70 MB model).

### Release gate

Assets (CONTRIBUTING step 7) passed 2026-09-27 on Warcraft III Reforged 3.0.0.24268 and World Editor 3.00, tested by the
maintainer, including a World Editor save after `assets:sync`: both custom imports came back with flag 29,
`assets:check` reported no changes and `build` succeeded. The other steps were not re-run; this release changes only
asset code.

## 0.3.0 (2026-09-26)

- Object data: custom units, heroes, buildings, items, abilities, buffs and upgrades in Pkl files under `objects/`,
  merged by `moonwell.pkl` and typed by the `moonwell` package's `ObjectFile.pkl`, with friendly field names from World
  Editor's labels, per-level values as a `List`, and `properties` for ability-specific fields by name or rawcode.
- Builds add the objects to the staged map's modification files and their `war3mapSkin.*` counterparts. World Editor's
  own objects, including modified standard objects, are kept byte for byte; the source map is never changed.
- Builds write `src/generated/objects.yue` with every object's id for gameplay code; `check` fails when it is stale.
- Every object problem is reported at once, with its file, object and field, and the nearest standard ids or field names
  as hints.
- `objects:check` validates the objects and lists the files a build would change; `objects:eval` prints the resolved
  objects as JSON. `check` and `dev` validate objects too, and `dev` watches `objects/`.
- The template's footman is now the Captain, a custom unit in `objects/units.pkl`.
- New projects get a `.gitattributes` that checks text out with LF line endings and keeps World Editor's map files and
  `assets/` byte for byte.

### Release gate

Object data (CONTRIBUTING step 9) passed 2026-09-26 on Warcraft III Reforged 3.0.0.24268 and World Editor 3.00, tested
by the maintainer.

- In game, the Captain shows its model, name and icon. A hero based on the Paladin shows its custom name and strength
  and learns a custom Holy Light to level 4, which heals the four per-level amounts. A custom item shows its name and
  icon. A custom Blacksmith offers only its custom upgrade, with each level's tooltip.
- A Footman modified in World Editor's source map keeps its change next to the Moonwell objects.
- The packed map opens in World Editor, and the Object Editor lists the custom objects with their values.

## 0.2.0 (2026-09-26)

- Assets: files under `assets/` are imported into builds. The manifest's `assets` block maps them to exact paths and
  excludes files. The `assets:check` and `assets:sync` commands write them into the source map for World Editor, with
  ownership tracking and rollback.
- `assets:paths` lists the files a model (`.mdx` or `.mdl`) references as in-game or custom paths, and whether the
  project imports them.
- `init` creates World Editor's icon folders under `assets/ReplaceableTextures/`.
- Map settings: the manifest's `settings` block sets the map's name, author, description and loading screen, existing
  player slots and forces, sound environment, water colour and fog, and hero level and food limits. Advanced raw
  `gameplayConstants` and `gameInterface` mappings set any key in `war3mapMisc.txt` and `war3mapSkin.txt`. Null settings
  keep the map's values; the new project template writes every everyday setting out at `null`.
- Builds apply settings to the staged map: `war3map.w3i` is patched byte for byte (versions 18, 25, 28, 31, 32, 33 and
  39), the matching World Editor calls in `war3map.lua` are edited to agree, and the text files are merged key by key.
  The source map in `maps/` is never changed.
- `settings:check` lists the internal files the settings would change, without building or writing the map. `check` and
  `dev` validate settings too.

### Release gate

Assets (CONTRIBUTING step 7) passed 2026-09-26, tested by the maintainer.

Map settings (CONTRIBUTING step 8) passed 2026-09-26 on Warcraft III Reforged 3.0.0.24268 and World Editor 3.00, after
three fixes the gate found: a player name no longer adds `SetPlayerName` (it crashed the game on lobby creation), the
hero level constant is `MaxHeroLevel`, and map-info colours are written blue, green, red, alpha.

- The lobby shows the map name, author, description, player name and race, and the force name.
- In game, the fog and water are red, the food ceiling is 50 and a hero stops at the maximum level.
- The packed map opens in World Editor with the configured description, loading screen, player and force properties,
  fog, water tint, sound environment and gameplay constants.

## 0.1.0 (2026-09-25)

First release: the toolchain.

- `moonwell init [--link]` scaffolds a map project: `deno.json` tasks, `PklProject`, a `moonwell.pkl` manifest with the
  everyday settings written out, and a git-ignored `moonwell.local.pkl` for machine settings.
- `build`, `test`, `dev`, `check` and `setup` commands. `setup` installs the pinned, checksum-verified YueScript
  compiler and recreates a missing `moonwell.local.pkl`.
- A YueScript module bundler with a Lua runtime: `before_config`, `on_config`, `before_main` and `on_main` hooks, and
  runtime errors reported at `.yue` file and line.
- An MPQ archive writer that packs the map into `<build.folder>/<map>.w3x`.
- The `moonwell` Pkl package: the project manifest schema.

### Release gate

Passed 2026-09-25 on Warcraft III Reforged 3.0.0.24268 and World Editor 3.00:

- `deno task test` launches the staged map. "Moonwell is running." prints and the footman changes colour every second
  (ally colour mode off).
- An `error` in `on_main` is reported on screen as `src/main.yue:<line>`. The exact chunk-name form was not recorded,
  but the runtime rewrote it correctly.
- The `--minify` build plays from the game's own Maps folder as a regular custom game.
- The packed map opens and test-runs in World Editor.
