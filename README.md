<p align="center">
  <img src="mw.gif" alt="Moonwell" width="256" />
</p>
<p align="center">
  <i>Multilingual Warcraft III modding framework</i>
</p>

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
| `objects/`           | Custom units, heroes, items, abilities and more, in Pkl                                                |
| `src/generated/`     | `objects.yue`, the ids of those objects for gameplay code; written by builds, commit it                |
| `maps/map.w3x/`      | World Editor map (folder format, Lua script mode)                                                      |
| `assets/`            | Files to import into the map                                                                           |
| `.asset-state/`      | Which source-map files `assets:sync` owns; commit it                                                   |
| `yueconfig.yue`      | Settings for VS Code's YueScript extension; builds do not read it                                      |
| `.luarc.json`        | Settings for lua-language-server in the editor                                                         |
| `.vscode/`           | `extensions.json`, which recommends the YueScript and Lua extensions                                   |
| `.moonwell/`         | Declarations for the editor, written by `check`, `build`, `test`, `dev` and `setup`; git-ignored       |
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

## Editor setup

VS Code with two extensions gives `.yue` files completion, hover, signature help and type warnings for the game's API.

1. Install [VS Code](https://code.visualstudio.com/), then open the project folder itself (File > Open Folder). VS Code
   offers the two extensions the project recommends: YueScript (`LiJin.yuescript`) and Lua (`sumneko.lua`). Install
   both. The YueScript extension uses the lua-language-server that the Lua extension brings, so nothing else needs
   installing.
2. Run `deno task setup` in the project. It adds the editor files and `.gitignore` lines an older project lacks (it
   never overwrites a file) and keeps a copy of the project's pinned YueScript in the cache's `bin` folder (by default
   `%LOCALAPPDATA%\moonwell\bin` on Windows; with `yue.path` set, your own binary's folder counts instead). The
   extension runs `yue` from PATH and has no setting for its location, so when `yue` is missing there or another
   version, `setup` prints a command that adds the `bin` folder to your user PATH. Run it once (in PowerShell on
   Windows), then open a new terminal and restart VS Code. Moonwell never changes PATH itself. Projects that pin
   different YueScript versions share the one `yue` in the `bin` folder: each `setup` replaces it with that project's
   version.
3. Always open the project folder itself, as in step 1. lua-language-server reads `.luarc.json` only from the first
   folder of the workspace: opened as a parent folder, a single file or a second workspace folder, nothing is
   recognised.

You get completion, hover and signatures for every native and Blizzard.j function and global of Warcraft III
3.0.0.24268, `import "moonwell"`, `import "generated.objects"` and the map's own `gg_` and `udg_` globals, and a warning
for a `unit` passed where a `player` is expected. The declarations live in `.moonwell/types/`; `check`, `build`, `test`
and `dev` keep them current, so run `deno task check` (or save any `.yue` file while `deno task dev` runs) after saving
the map in World Editor to pick up new `gg_` and `udg_` globals.

- **`.lua` files next to your `.yue` files.** The extension writes a `.lua` file next to each saved `.yue` file. It
  needs them for lua-language-server. They are git-ignored, and builds never use them: Moonwell compiles `src/**/*.yue`
  itself.
- **A `yue` console window on Windows.** Whenever VS Code starts or reloads, the YueScript extension starts `yue` in a
  console window (with Windows Terminal as the default terminal, a terminal tab running `yue.exe`). Leave it open:
  closing it stops the extension's completion until the next reload. This is the extension's behaviour, not Moonwell's:
  version 0.2.9 does not hide the process's window. The fix is merged upstream (pigpigyyy/yuescript-vscode#11) and ships
  in the extension's next release.
- **The game's Lua.** Warcraft III 3.0.0.24268 runs Lua 5.3 without `collectgarbage`, `dofile`, `loadfile`, `debug`,
  `io` and `package`, and its `os` has only `clock`, `date`, `difftime` and `time`. `.luarc.json` turns off `io`,
  `debug` and `package` in the editor; lua-language-server cannot turn off single functions, so the editor does not flag
  the others.
- **Pkl.** Pkl files need no editor plugin: the `pkl` and `deno` command-line tools do all the work. Editors with Pkl
  support (the Pkl extension for VS Code, the IntelliJ plugin) add completion and hover docs for `moonwell.pkl` and
  `objects/`. They find the schema through `PklProject`, so run their "sync projects" command once after `init`. If the
  extension cannot find `pkl`, set its CLI path (`pkl.cli.path` in VS Code).

## Unknown globals

`check`, `build`, `test` and `dev` stop on a global that nothing defines, which is almost always a typo:

```text
error: src/main.yue:8:10 › Unknown global CreatUnit.
hint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.
```

A global is known when it is a native, any function, global or constant of common.j or Blizzard.j (such as
`PLAYER_NEUTRAL_AGGRESSIVE`), or a Lua library the game provides; a global or function of the source map's `war3map.lua`
(such as `gg_unit_Hpal_0002` or `udg_Score`; run the command again after saving the map in World Editor); a name
declared with `global` in any file under `src/` (`global Score = 0`, `global a, b`); or a name listed in `lint.globals`
in `moonwell.pkl`. Fields are not checked: `math.floor` checks only `math`.

`global *` and `global ^` make later assignments global without naming them, so Moonwell cannot see those names; list
them in `lint.globals`. To report unknown globals without failing, set `lint { unknownGlobals = "warning" }`. In the
editor, lua-language-server underlines most of the same names as you type; it does not know `lint.globals`, and does not
flag `collectgarbage`, `dofile` or `loadfile`.

The check runs the compiler's `yue -g` on each changed file and caches the result in `dist/stage/lua/`.

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

## Objects

Custom units, heroes, buildings, items, abilities, buffs and upgrades are written in Pkl under `objects/`. Builds add
them to the map's object data (`war3map.w3u` and the other modification files, and their `war3mapSkin.*` counterparts),
in the staged copy only. `moonwell.pkl` merges every file under `objects/`, in any folders:

```pkl
import "@moonwell/Objects.pkl"
objects = Objects.merge(import*("objects/**.pkl"))
```

Every file there amends `@moonwell/ObjectFile.pkl` and fills any of the mappings `heroes`, `units`, `buildings`,
`items`, `abilities`, `buffs` and `upgrades`. Put shared helpers in a module outside `objects/` and import them, since
every `.pkl` file under `objects/` is merged as an object file. The template's Captain:

```pkl
amends "@moonwell/ObjectFile.pkl"

units {
  ["captain"] {
    id = "h000"
    base = "hfoo"
    name = "Captain"
    modelFile = #"units\human\TheCaptain\TheCaptain"#
    iconGameInterface = #"ReplaceableTextures\CommandButtons\BTNTheCaptain.blp"#
  }
}
```

- **Keys** name the object in gameplay code and are unique per category across all files: letters, digits and `_`, not a
  Lua or YueScript keyword.
- **`id`** is the new object's four-letter rawcode, unique across all categories and not a standard object's. Heroes
  start with an uppercase letter; units and buildings must not.
- **`base`** is the standard object it copies. Objects based on other custom objects are not supported.
- **Fields** have friendly names made from World Editor's labels: `Hit Points Maximum (Base)` is `hitPointsMaximumBase`.
  Hover a field in an editor with Pkl support to see its label and rawcode, or read the generated `*Props.pkl` modules
  in the `moonwell` package.

Builds write `src/generated/objects.yue` with each object's id, so gameplay code does not repeat rawcodes:

```yue
import "moonwell" as mw
import "generated.objects" as objects

mw.on_main ->
  CreateUnit Player(0), objects.units.captain, 0, 0, 270
```

`build`, `test` and `dev` rewrite it when the objects change. Commit it; `check` fails when it is stale.

### Values and levels

- A value sets the field; on a per-level field it sets level 1. A `List` on a per-level field sets levels 1, 2, ..., and
  later levels keep the base's values: `cooldown = List(8, 7, 6)`. It may not have more entries than the object's level
  count (its own `levels`, else the base's).
- List fields, such as a unit's `normal` abilities, take a comma-separated `String` or a `List<String>`:
  `normal = List("Adef", "Aslo")`. On a per-level list field, a `List<List<String>>` sets levels.
- `null` or a field left out keeps the base's value. `false`, `0`, `""` and, on a list field, `List()` are real values.
- Text is written as literal text; Moonwell never creates `TRIGSTR_` references or changes `war3map.wts`.

Abilities, buffs and upgrades have typed fields only for what every object of the category shares. Fields specific to
some abilities, such as Holy Light's heal amount, go in `properties`, keyed by friendly name or rawcode:

```pkl
abilities {
  ["holy_light"] {
    id = "A000"
    base = "AHhb"
    levels = 4
    properties { ["amountHealedOrDamaged"] = List(111, 222, 333, 444) }  // or ["Hhb1"]
  }
}
```

A field set twice (typed and in `properties`, or by name and by rawcode) is an error, as is a field that does not apply
to the base.

### Errors and commands

Pkl reports type and pattern errors with file and line. Moonwell then checks the objects against the game data and the
map, and reports every problem at once, each with the file, object and field:

```text
error: objects/heroes.pkl › heroes["paladin"].base: 'Hpla' is not a standard hero.
hint: Did you mean 'Hpal' (Paladin), 'Hpb1' (Paladin) or 'Hpb2' (Paladin)?
```

`deno task objects:check` validates without building, lists the internal files a build would change, and says whether
`src/generated/objects.yue` is current. `check` and `dev` run the same checks. `deno task objects:eval` prints every
resolved object as JSON: its id, base and source file, and each field's rawcode, name, level, data column and value.

### World Editor objects

Objects made in World Editor stay as they are: modified standard objects and custom objects in the source map are copied
byte for byte, and Moonwell's objects are added next to them. An `id` already used by a World Editor custom object is an
error; change the Pkl id or delete the object in World Editor. Builds start from the source map every time, so objects
never pile up.

Not supported yet:

- Modifying standard objects from Pkl (modify them in World Editor); editing or removing objects made in World Editor.
- Doodads and destructables, which World Editor's files keep as they are.
- `TRIGSTR_` references and localized labels.
- Objects based on custom objects, placing units, and per-unit skins.
- Importing `.w3o` exports or reading packed `.w3x` maps.
- Value ranges and named values for fields such as attack types: fields take their plain `Int`, `Number` or `String`.

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
| `deno task objects:check`                        | Validate the objects and show which internal map files they would change         |
| `deno task objects:eval`                         | Print the resolved objects as JSON                                               |
| `deno task setup`                                | Create a missing `moonwell.local.pkl`, download YueScript and prepare the editor |

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

## Credits

The template map derives from TriggerHappy's [wc3-ts-template](https://github.com/cipherxof/wc3-ts-template) via
wc3-dev-framework. MIT licensed; see [LICENSE](LICENSE).
