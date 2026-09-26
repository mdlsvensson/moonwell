# Moonwell

**Build Warcraft III maps with YueScript gameplay and Pkl project data.**

- Gameplay in [YueScript](https://github.com/IppClub/YueScript), compiled to Lua 5.3 and bundled into your map. Only the
  modules you import are included, and runtime errors point at your `.yue` lines (in `--minify` builds, at the `.yue`
  file only).
- Project configuration in [Pkl](https://pkl-lang.org), validated against the versioned `moonwell` schema package.
- A Deno command-line tool: no Node.js, no `package.json`, no `node_modules`.

## Quickstart

Install [Deno](https://deno.com/) 2.9+ and [Pkl](https://pkl-lang.org) 0.32+.

```powershell
deno run -A jsr:@moonwell/cli init my-map
cd my-map
deno task build
```

`init` also writes `moonwell.local.pkl`, which points `launch.gameExecutable` at the default Battle.net install. If your
game is elsewhere, fix the path there, then play:

```powershell
deno task test
```

## A project

| Path                 | What                                                                                                   |
| -------------------- | ------------------------------------------------------------------------------------------------------ |
| `moonwell.pkl`       | Project manifest (`amends "@moonwell/Project.pkl"`), shared by the team; everyday settings written out |
| `moonwell.local.pkl` | This machine's settings, such as the game path; git-ignored, and `deno task setup` recreates it        |
| `src/main.yue`       | Gameplay entry                                                                                         |
| `maps/map.w3x/`      | World Editor map (folder format, Lua script mode)                                                      |
| `assets/`            | Files to import into the map                                                                           |
| `.asset-state/`      | Which source-map files `assets:sync` owns; commit it                                                   |
| `dist/`              | Build output                                                                                           |

`moonwell.local.pkl` amends `moonwell.pkl`, so any setting can be overridden there for your machine only. Lists such as
`launch.args` are replaced, not extended: `args = List("-launch", "-windowmode", "fullscreen")`.

Gameplay registers hooks with the `moonwell` module:

```yue
import "moonwell" as mw

mw.on_main ->
  print "Hello from YueScript"
```

Hooks: `before_config`, `on_config`, `before_main`, `on_main`. A failing hook prints its error with the `.yue` file and
line, and the other hooks still run. Module top-level code runs while the map script loads, so create game objects
inside hooks.

## Assets

Every file under `assets/` is imported into the built map at its relative path: `assets/Models/unit.mdx` becomes
`Models\unit.mdx`. Names starting with `.` are skipped. The `assets` block in `moonwell.pkl` maps files to exact in-map
paths and leaves files out:

```pkl
assets {
  paths { ["icons/BTNSword.blp"] = #"ReplaceableTextures\CommandButtons\BTNSword.blp"# }
  exclude = List("credits/")
}
```

Builds import assets into the staged copy only. To see them in World Editor, close the map there and run
`deno task assets:sync`. It writes the files and `war3map.imp` into `maps/<folder>`, and records what it owns in
`.asset-state/`. It never overwrites or deletes a file it does not own, and it refuses to touch an owned file you edited
in the map.

To check that a model's textures are imported, run `deno task assets:paths assets/Models/Knight.mdx`. It lists every
file the model references (textures, particle models, attachments), shown the way World Editor's Import Manager shows
paths, and says what each one is: an `in-game path` the game ships (or `in-game path, replaced` when you import a file
over it), a `custom path, imported`, or a `custom path, not imported`, which the model will be missing. Run it without a
file to check every model under `assets/`, or on a model outside a project to see which custom paths it needs you to
import.

## Icons

`init` creates World Editor's icon folders under `assets/`. Icons named the game's way import at the paths the object
editor expects:

| Folder                                        | File name                                 | For                                  |
| --------------------------------------------- | ----------------------------------------- | ------------------------------------ |
| `ReplaceableTextures/CommandButtons/`         | `BTN<Name>.blp`                           | Abilities, units, items and upgrades |
| `ReplaceableTextures/CommandButtonsDisabled/` | `DISBTN<Name>.blp`, `DISPASBTN<Name>.blp` | Greyed-out versions                  |
| `ReplaceableTextures/PassiveButtons/`         | `PASBTN<Name>.blp`                        | Passive abilities                    |

Give every `BTN<Name>` a matching `DISBTN<Name>`, and every `PASBTN<Name>` a matching `DISPASBTN<Name>`: the game shows
a placeholder where a disabled icon is missing.

## Map settings

The `settings` block in `moonwell.pkl` overrides the map's own settings: its name and loading screen, player slots,
forces, environment and gameplay constants. `init` writes every everyday setting out at `null`, so a new project keeps
everything the map has. Set only what you want to change:

```pkl
settings {
  info { name = "My Map"; author = "" }
  gameplay { heroMaxLevel = 25; foodLimit = 200 }
  environment { waterColor = List(20, 40, 80, 255) }
}
```

- **Inheritance and clearing.** A `null` or omitted setting keeps the map's value. `false`, `0` and `""` are real
  values: `author = ""` clears the author. Text you set is written as literal text; text you leave alone keeps its
  `TRIGSTR_*` reference into `war3map.wts`, which Moonwell never rewrites.
- **Colors** are `List(red, green, blue, alpha)`, each 0 to 255, and replace the map's color whole. Setting `waterColor`
  turns on the map's custom water tint. `fog.enabled` switches fog on or off; the other fog fields do not switch it on.
  After inheriting any value you leave out, fog `start` must not exceed `end`.
- **Staged copy only.** Builds and `deno task test` write settings into the staged copy in `dist/stage/`, never into
  `maps/<folder>`, so World Editor keeps showing the map's own values. Open the built map to see them. Settings are
  applied after staging and before assets and the gameplay bundle, and never appear in `war3map.imp`.
- **Map versions.** The map info file (`war3map.w3i`) must be version 18, 25, 28, 31, 32, 33 or 39; World Editor 3.00
  saves version 39. A loading-screen `model` needs version 25 or later. `players`, `forces` and `environment` need
  version 28 or later and Lua as the script language. If a map is refused, open it in World Editor and save it again in
  folder format with Lua as the script language.
- **Existing players and forces only.** `players["3"]` is the slot World Editor shows as Player 4 (IDs are zero-based, 0
  to 23), and `forces["0"]` is the first force. Settings change existing slots and forces; they never add or remove one
  or change which players are on a team. Create slots in World Editor's Scenario > Player Properties and save first.
- **Custom forces.** Force settings need custom forces: in World Editor, open Scenario > Force Properties, turn on Use
  Custom Forces, set up the teams, and save the map. The template's commented `forces` example says the same.
- **Editor Lua calls.** World Editor writes some settings into `war3map.lua` too, and Moonwell edits those calls to
  match: `SetMapName` and `SetMapDescription` in `config()`, the player calls in `InitCustomPlayerSlots()`, the team
  calls in `InitCustomTeams()`, and the sound, water and fog calls in `main()` (before `CreateAllUnits()` or
  `InitBlizzard()`). It needs each call exactly where World Editor puts it and refuses, naming `war3map.lua`, when the
  script does not look like World Editor's, for example after hand edits to those functions. Re-saving the map in World
  Editor restores them. Your gameplay code is not affected.

`deno task settings:check` checks the settings against the source map without building and lists the internal files a
build would change:

```text
  war3map.w3i
  war3map.lua
  war3mapMisc.txt
Map settings valid: 3 internal file(s) would change during build.
```

`deno task check` (and so `dev`) checks settings the same way; with no settings set it does not need the source map.
Mistakes in the manifest name the manifest that was evaluated (`moonwell.local.pkl` when it exists, else
`moonwell.pkl`). Problems with the map name the file under `maps/<folder>/`, such as `maps/map.w3x/war3map.w3i`. Map
files are matched ignoring letter case, as Warcraft III does: a map saved with `war3mapskin.txt` is patched under that
name.

## Commands

| Command                                          | What                                                                             |
| ------------------------------------------------ | -------------------------------------------------------------------------------- |
| `deno task build [--entry src/x.yue] [--minify]` | Build `dist/bin/<map>.w3x`                                                       |
| `deno task test [--entry src/x.yue]`             | Stage the map and launch Warcraft III                                            |
| `deno task dev`                                  | Re-check on every save                                                           |
| `deno task check`                                | Compile and validate without building                                            |
| `deno task assets:check`                         | Show what `assets:sync` would change in the source map                           |
| `deno task assets:sync`                          | Write `assets/` into the source map for World Editor (close the map first)       |
| `deno task assets:paths [file]`                  | List the files a model references, as in-game or custom paths                    |
| `deno task settings:check`                       | Show which internal map files the settings would change, without building        |
| `deno task setup`                                | Create a missing `moonwell.local.pkl` and download the pinned YueScript compiler |

The compiler is downloaded once per version and verified by checksum. It is cached in `MOONWELL_CACHE` when that is set,
else in `%LOCALAPPDATA%\moonwell` on Windows, else in `$XDG_CACHE_HOME/moonwell` or `~/.cache/moonwell`.

## Advanced settings

These are not in the generated files and keep their defaults unless you add them. The schema, `Project.pkl` in the
`moonwell` Pkl package, documents every setting.

| Setting       | Default  | What                                                                                        |
| ------------- | -------- | ------------------------------------------------------------------------------------------- |
| `yue.version` | `0.34.2` | YueScript compiler version. Only versions this CLI release pins a checksum for are accepted |
| `yue.path`    | none     | Your own `yue` binary instead of the downloaded one. Set it in `moonwell.local.pkl`         |

```pkl
yue {
  path = "C:\\tools\\yue.exe"
}
```

### Raw gameplay constants and game interface

`settings.gameplayConstants` and `settings.gameInterface` set any section and key in `war3mapMisc.txt` (World Editor's
Gameplay Constants) and `war3mapSkin.txt` (Game Interface). They are left out of the template; their schema is in
`MapSettings.pkl`, which `Project.pkl` imports.

```pkl
settings {
  gameplayConstants { ["Misc"] { ["MaxHeroLevel"] = "25" } }
  gameInterface { ["CustomSkin"] { ["Test"] = "value" } }
}
```

`CustomSkin`/`Test` only shows the syntax: it is an invented key, and nothing says Warcraft III reads it.

- Values are strings, written as they are: `"25"`, not `25`. A value is one line, and `""` writes an empty `Key=`.
- Section and key names are letters, digits and `_`, and match the file's names ignoring letter case. The file keeps its
  own spelling. Two names in one mapping that differ only in case are an error.
- Every matching key is replaced, a missing key is added to its section, and a missing section or file is created. Other
  sections, keys and comments stay as they are.
- `gameplay.heroMaxLevel` and `gameplay.foodLimit` write `[Misc] MaxHeroLevel` and `[Misc] FoodCeiling`. If you set the
  same key raw as well, the two must agree exactly: `heroMaxLevel = 25` with `["MaxHeroLevel"] = "25"` is accepted, with
  `"025"` it is an error.

## Editor support

Nothing needs an editor plugin: the `pkl` and `deno` command-line tools do all the work. Editors with Pkl support (the
Pkl extension for VS Code, the IntelliJ plugin) add completion and hover docs for `moonwell.pkl`. They find the schema
through `PklProject`, so run their "sync projects" command once after `init`. If the extension cannot find `pkl`, set
its CLI path (`pkl.cli.path` in VS Code).

## Credits

The template map derives from TriggerHappy's [wc3-ts-template](https://github.com/cipherxof/wc3-ts-template) via
wc3-dev-framework. MIT licensed; see [LICENSE](LICENSE).
