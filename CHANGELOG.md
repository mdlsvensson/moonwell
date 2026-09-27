# Changelog

## Unreleased

- `assets:paths` with no file argument reports every model it can read and marks each unreadable one in its place,
  instead of stopping at the first; the command still fails when any model was unreadable.
- Ctrl+C during `assets:sync` undoes every change it already made to the source map and exits with code 130. A second
  Ctrl+C still exits at once.
- `assets:paths` reads large text `.mdl` models with far less memory (about a tenth for a 70 MB model).

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
