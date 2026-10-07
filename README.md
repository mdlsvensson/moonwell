<p align="center">
  <img src="mw.gif" alt="Moonwell" width="256" />
</p>
<p align="center">
  <i>Multilingual modding framework for Warcraft III: Reforged</i>
</p>
<p align="center">
  <a href="https://www.hiveworkshop.com/">
    <img
      src="https://cdn.hiveworkshop.com/data/styles/7/styles/vindit-reforged/hive-logo.png"
      alt="Hive Workshop"
      style="height: 36px; width: auto; vertical-align: middle;"
    />
  </a>
</p>

- Gameplay with [YueScript](https://yuescript.org/), [Teal](https://teal-language.org/),
  [Fennel](https://fennel-lang.org/).
- [Pkl](https://pkl-lang.org) for object data, assets and configuration.
- Annotated lua wrappers for Warcraft III natives.
- Full systems suite including damage engine, physics, missiles, save/load, and more.
- Powerful cli written in Go.
- Asset import pipeline: Just drop em in, sync and voilà.
- Print mdx texture paths to the console.
- Map settings management in pkl.

## Quickstart

Install Moonwell. On Windows, in PowerShell:

```powershell
irm https://github.com/mdlsvensson/moonwell/releases/latest/download/install.ps1 | iex
```

On Linux:

```sh
curl -fsSL https://github.com/mdlsvensson/moonwell/releases/latest/download/install.sh | sh
```

Then create a project and build it:

```powershell
moonwell init my-map
cd my-map
moonwell build
```

`init` writes `moonwell.local.pkl`, which points `launch.gameExecutable` at the default Battle.net install. If your game
is elsewhere, fix the path there, then run:

```powershell
moonwell test
```

## Installing and upgrading

The install line downloads one program, `moonwell`, from the GitHub release, checks it against the release's checksums
and puts it on your PATH:

- **Windows:** `%LOCALAPPDATA%\moonwell\bin\moonwell.exe` (`MOONWELL_CACHE\bin` when that variable is set). The script
  adds the folder to your user PATH; open a new terminal afterwards. It is the folder `moonwell setup` keeps `yue` in
  for the editor, so one PATH entry serves both.
- **Linux:** `~/.local/bin/moonwell`. The script says so when that folder is not on your PATH.

Moonwell is built for Windows and Linux on x86-64. Run the line again to upgrade: there is no update command.

Moonwell evaluates projects with [Pkl](https://pkl-lang.org). It uses the `pkl` on your PATH when that is Pkl 0.32 or
newer. Otherwise it downloads Pkl 0.32.1 (about 100 MB, once, checked against a pinned checksum) into its cache and runs
that, with a warning when the `pkl` on your PATH is older. `moonwell setup` then copies it next to `yue` in the `bin`
folder of its cache, so a `pkl` command you type, such as `pkl project resolve`, finds it too. On Windows that is the
folder above. On Linux it is `~/.cache/moonwell/bin`, and `setup` prints the command that puts it on your PATH.

A project names the Moonwell it is written for in its `PklProject`, as the version of the `moonwell` Pkl package. The
program and the package must have the same major and minor version; `moonwell` refuses another project and says which
of the two to change. To install one version, use its own script:

```powershell
irm https://github.com/mdlsvensson/moonwell/releases/download/moonwell@0.9.1/install.ps1 | iex
```

To move a project to a newer Moonwell, install that version, change the package's version in the project's
`PklProject` (for example `moonwell@0.8.1` to `moonwell@0.9.0`) and run `pkl project resolve`.

### Upgrading a project to 0.10

A project that builds with Moonwell 0.9 needs three steps to build with 0.10:

1. Install Moonwell 0.10: run the install line again.
2. In the project's `PklProject`, set the package's version to `moonwell@0.10.0`.
3. Run `pkl project resolve` in the project folder. If you have no `pkl` command, run `moonwell setup` there first:
   it copies Moonwell's own Pkl into its cache's `bin` folder and prints the command that puts that folder on your
   PATH. It then stops at the project's package version, which this step puts right.

In nearly every project nothing in `moonwell.pkl`, the map, `moonwell.lock` or `.asset-state/` has to change. The
first command afterwards builds the caches under `dist/` and `.moonwell/` anew. The [changelog](CHANGELOG.md) lists
what 0.10 does differently, and what it refuses that 0.9 let through; the two changes a project is most likely to
meet are that a mistyped flag is now an error, and that `dist` must be a real folder.

### Upgrading a project from 0.7

Moonwell 0.7 and earlier ran on Deno. A project made with one of them needs four steps:

1. Install `moonwell`, as above.
2. In `PklProject`, change the package's version: `moonwell@0.7.0` becomes `moonwell@0.9.1`.
3. Run `pkl project resolve`.
4. Delete `deno.json` and `deno.lock`. Where you ran `deno task build`, run `moonwell build`; the same goes for every
   other command.

Nothing else changes in the project: the manifest, the map and the libraries stay as they are. The caches under
`dist/` and `.moonwell/` are built anew by the first command.

## A project

| Path                 | What                                                                                                   |
| -------------------- | ------------------------------------------------------------------------------------------------------ |
| `moonwell.pkl`       | Project manifest (`amends "@moonwell/Project.pkl"`), shared by the team; everyday settings written out |
| `moonwell.local.pkl` | This machine's settings, such as the game path; git-ignored, and `moonwell setup` recreates it         |
| `src/main.yue`       | Gameplay entry                                                                                         |
| `lua/`               | Plain Lua modules, bundled when gameplay code requires them (see "Lua modules")                        |
| `objects/`           | Custom units, heroes, items, abilities and more, in Pkl                                                |
| `src/generated/`     | `objects.yue`, the ids of those objects for gameplay code; written by builds, commit it                |
| `maps/map.w3x/`      | World Editor map (folder format, Lua script mode)                                                      |
| `assets/`            | Files to import into the map                                                                           |
| `.asset-state/`      | Which source-map files `assets:sync` owns; commit it                                                   |
| `yueconfig.yue`      | Settings for VS Code's YueScript extension; builds do not read it                                      |
| `.luarc.json`        | Settings for lua-language-server in the editor                                                         |
| `.vscode/`           | `extensions.json`, which recommends the YueScript and Lua extensions                                   |
| `moonwell.lock`      | The commit of each library's GitHub tag (see "Libraries"); commit it                                   |
| `.moonwell/`         | Libraries and editor declarations, written by `check`, `build`, `test`, `dev`, `setup` and the three `assets:` commands; git-ignored |
| `dist/`              | Build output                                                                                           |

`moonwell.local.pkl` amends `moonwell.pkl`, so any setting can be overridden there for your machine only. Lists such as
`launch.args` are replaced, not extended: `args = List("-launch", "-windowmode", "fullscreen")`.

`dist/` and the folder the map is built into (`build.folder`, by default `dist/bin`) are real folders. Moonwell
removes and replaces what it wrote there, so it refuses a link or a Windows junction on the way to anything it writes
below them, and names the link. A link at `.moonwell/`, `.asset-state/`, `maps/`, `src/` or `lua/` is refused too.

Build only a project you trust. `build`, `test`, `check`, `dev` and `setup` run code the project brings: the program
its `yue.path` names, and its macros, which the compiler runs while it compiles. Refusing links keeps Moonwell's own
writes inside the project folder; it is no barrier against a project that means harm.

Gameplay registers hooks with the `moonwell` module:

```yue
import "moonwell" as mw

mw.on_main ->
  print "Hello from YueScript"
```

Hooks: `before_config`, `on_config`, `before_main`, `on_main`. A failing hook prints its error with the `.yue` or `.lua`
file and line, and the other hooks still run. Module top-level code runs while the map script loads, so create game
objects inside hooks.

Bitwise operators (`&`, `|`, `~`, `<<`, `>>`) do not build from YueScript: the compiler writes them, but its step that
prepares the Lua for the map does not read them, and Moonwell stops the build with an error at the line. Put that code
in a Lua file under `lua/` (see "Lua modules"), where the operators work as the game's Lua 5.3 has them.

Floor division (`//`) works with YueScript 0.34.3, the default. YueScript 0.34.2, which `yue.version` can still name,
compiles a file that uses it to no Lua at all; Moonwell stops that build too.

## Editor setup

VS Code with two extensions gives `.yue` files completion, hover, signature help and type warnings for the game's API.

1. Install [VS Code](https://code.visualstudio.com/), then open the project folder itself (File > Open Folder). VS Code
   offers the two extensions the project recommends: YueScript (`LiJin.yuescript`) and Lua (`sumneko.lua`). Install
   both. The YueScript extension uses the lua-language-server that the Lua extension brings, so nothing else needs
   installing.
2. Run `moonwell setup` in the project. It adds the editor files and `.gitignore` lines an older project lacks (it
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
and `dev` keep them current, so run `moonwell check` (or save any `.yue` file while `moonwell dev` runs) after saving
the map in World Editor to pick up new `gg_` and `udg_` globals.

- **`.lua` files next to your `.yue` files.** The extension writes a `.lua` file next to each saved `.yue` file. It
  needs them for lua-language-server. They are git-ignored, and builds never use them: Moonwell compiles `src/**/*.yue`
  itself.
- **The game's Lua.** Warcraft III 3.0.0.24268 runs Lua 5.3 without `collectgarbage`, `dofile`, `loadfile`, `debug`,
  `io` and `package`, and its `os` has only `clock`, `date`, `difftime` and `time`. `.luarc.json` turns off `io`,
  `debug` and `package` in the editor; lua-language-server cannot turn off single functions, so the editor does not flag
  the others.
- **Pkl.** Pkl files need no editor plugin: the `pkl` and `moonwell` programs do all the work. Editors with Pkl
  support (the Pkl extension for VS Code, the IntelliJ plugin) add completion and hover docs for `moonwell.pkl` and
  `objects/`. They find the schema through `PklProject`, so run their "sync projects" command once after `init`. If the
  extension cannot find `pkl`, set its CLI path (`pkl.cli.path` in VS Code).

## Unknown globals

`check`, `build`, `test` and `dev` stop on a global that nothing defines, which is almost always a typo:

```text
error: src/main.yue:9:10 › Unknown global CreatUnit.
hint: Did you mean CreateUnit? Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.
```

A global is known when it is a native, any function, global or constant of common.j or Blizzard.j (such as
`PLAYER_NEUTRAL_AGGRESSIVE`), or a Lua library the game provides; a global or function of the source map's `war3map.lua`
(such as `gg_unit_Hpal_0002` or `udg_Score`; run the command again after saving the map in World Editor); a name
declared with `global` (`global Score = 0`, `global a, b`), or defined at the top level of a Lua file (see "Lua
modules"), in a module the map requires: its entry (`map.entry`, or `--entry`), and every module reached from it through
`import`/`require`, including library modules; or a name listed in `lint.globals` in `moonwell.pkl`. Fields are not
checked: `math.floor` checks only `math`.

Only the files under `src/` that the map requires are checked. A file nothing imports is not checked, and its `global`
lines do not count; library modules are not checked either.

`global *` and `global ^` make later assignments global without naming them, so Moonwell cannot see those names; list
them in `lint.globals`. To report unknown globals without failing, set `lint { unknownGlobals = "warning" }`. In the
editor, lua-language-server underlines most of the same names as you type; it does not know `lint.globals`, and does not
flag `collectgarbage`, `dofile` or `loadfile`.

The check runs the compiler's `yue -g` on each changed file and caches the result in `dist/stage/lua/`.

## Macros

Macros run while the code compiles. Import them from `moonwell.macros`:

```yue
import "moonwell.macros" as {:$FourCC}

footman = CreateUnit Player(0), $FourCC("hfoo"), 0, 0, 270
```

`$FourCC "hfoo"` is the rawcode `'hfoo'` as the integer the game's functions take (`1751543663`), written into the
compiled code, so the game never converts it. It takes one string literal of exactly four printable ASCII characters,
with no escapes or `#{}` interpolation; anything else, such as a variable or `"hfo"`, fails the compile at that line.
For your own objects, use the ids in `generated.objects` instead.

The macro module is written to `.moonwell/yue/moonwell/macros.yue` by `setup`, `check`, `build`, `test` and `dev`, where
the compiler and the editor's YueScript extension find it. It adds nothing to the map.

## Lua modules

Plain Lua goes in `lua/`, next to `src/`. A file there is a module named by its path, like a `.yue` file in `src/`:
`lua/utils/timer.lua` is `utils.timer`, and `lua/tools/init.lua` answers to `tools`. `src/` and `lua/` share one set of
names; a name both define fails the build.

```yue
import "utils.timer" as timer   -- lua/utils/timer.lua
require "counter"               -- a Lua file that defines globals
CountUp!
```

Lua modules are bundled as they are, only when something requires them, and runtime errors in them name the `.lua` file
and line, also in `--minify` builds (Lua modules are not minified). The unknown-global check does not read `.lua` files,
but the globals a required Lua file defines at its top level (`function CountUp(`, `Count = 0`) count as known in your
YueScript. The scan recognises top-level `function Name(` definitions and `Name = …` or `A, B = …` statements that start
a line (or follow `;`), leaving out any name the file declares `local` at its top level (`local Timer` before
`Timer = {}`); globals assigned inside functions, after a label on the same line, or through `_G` are not seen and
belong in `lint.globals`. As in standalone Lua, a leading byte order mark and a first line starting with `#` are
skipped. The name `moonwell` is the built-in module's, so `lua/moonwell.lua` fails the build. The editor resolves `lua/`
through `.luarc.json`'s `runtime.path`; `moonwell setup` adds the entries to a project made by an older Moonwell.

`src/**/*.lua` stays reserved for the `.lua` files the YueScript extension writes on save, so keep your own Lua in
`lua/`.

## Libraries

A library is a folder of YueScript and Lua modules, and of files for the map, from a GitHub tag or a local folder. List
libraries in `moonwell.pkl`:

```pkl
libraries {
  ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.2.0" }
}
```

The key names the library's folder in `.moonwell/libraries/`, so keys must differ by more than case. A library's modules
keep their own names (`import "example.loud"`), and share one set of names with `src/` and `lua/`: a name two of them
define fails the build. A `.lua` file next to a `.yue` file of the same name in a library is its compiled output, and is
skipped.

A library describes its own layout in a `moonwell-library.json` at its root:

```json
{ "dir": "src", "assets": "assets" }
```

- `dir` is the folder module names start from. A library without the file needs it in the manifest instead
  (`["old"] { github = "owner/repo"; tag = "v1.0.0"; dir = "src" }`), and a `dir` in the manifest always wins. With
  neither, module names start at the library's root.
- `assets` is a folder of files the map imports, each at its path in that folder: see [Assets](#assets).

Both are optional. Any other key fails, naming the library: it may be written for a newer Moonwell.

`check`, `build`, `test`, `dev` and `setup` download a library that is missing or whose `github`, `tag` or `dir` changed
into `.moonwell/libraries/<key>/`, and its files for the map into `.moonwell/library-assets/<key>/` (both git-ignored),
and record the tag's commit in `moonwell.lock`. Commit `moonwell.lock`: a fresh clone then gets the same code, and if a
tag is moved on GitHub, the command fails instead of using the new code. To upgrade, change `tag`.

To work on a library next to your map, point it at a local folder, the library's root, in `moonwell.local.pkl`:

```pkl
libraries { ["example"] { path = "../moonwell-example-lib" } }
```

`path` wins over `github`. Its `.yue` and `.lua` files, and the files of its assets folder, are copied into `.moonwell/`
(folders whose name starts with `.`, such as `.git/`, are skipped), so errors in them name the copy there, not your
checkout. `dev` watches those folders, but picks the folders to watch when it starts: restart it after adding a local
library. A local library keeps its entry in `moonwell.lock`, so switching back to the tag still checks it. Library code
is not checked for unknown globals, but the globals a required library module defines count as known. `check`, `build`,
`test` and `dev` write every library module to `.moonwell/lua/` as Lua (a YueScript module compiled), where the editor
finds it; `setup`, which does not compile, writes the Lua modules and leaves the YueScript ones as the last compile
wrote them.

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

An asset cannot take the name of one of the map's own files, such as `war3map.lua`. That includes `war3mapPreview.tga`,
which Reforged ignores, and `war3mapMap.blp`: for a picture of your own in the game's map list, see
[A picture in the map list](#a-picture-in-the-map-list).

The files a library ships (the `assets` folder of its `moonwell-library.json`, see [Libraries](#libraries)) are imported
too, each at its path in that folder; `paths` and `exclude` apply to your own files only. When one of your files and a
library's have the same in-map path, yours is imported and the command says so:
`assets/Textures/Golem.blp replaces library golems's Textures/Golem.blp`. That is how you swap a library's icon or
model. Two libraries with a file at the same path fail the build. `assets:check` lists a library's files as
`library <key>: <file>`, `assets:sync` writes them into the source map with your own, and `assets:paths` counts them as
imported and checks a library's models too.

If you write a library, keep its files under a folder of its own, such as `assets/war3mapImported/<library>/`, so they
clash with no map's and no other library's.

Builds import assets into the staged copy only. To see them in World Editor, close the map there and run
`moonwell assets:sync`. It writes the files and `war3map.imp` into `maps/<folder>`, and records what it owns in
`.asset-state/`. It never overwrites or deletes a file it does not own, and it refuses to touch an owned file you edited
in the map.

To check that a model's textures are imported, run `moonwell assets:paths assets/Models/Knight.mdx`. It lists every
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
- **Staged copy only.** Builds and `moonwell test` write settings into the staged copy in `dist/stage/`, never into
  `maps/<folder>`, so World Editor keeps showing the map's own values. Open the built map to see them. Settings are
  applied after the objects and before the assets and the gameplay bundle, and never appear in `war3map.imp`.
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

### A picture in the map list

`settings.info.preview` names a picture that the game's map list shows for the map, instead of its minimap:

```pkl
settings {
  info { preview = "preview.png" }
}
```

- **The file.** A `.png`, a `.tga` or a `.blp` of 256×256 or 512×512 pixels, at a path from the project folder. Keep
  it beside `moonwell.pkl`, not under `assets/`.
  - A PNG of any kind is read: 8 or 16 bits, colour, grey or palette, interlaced or not. A TGA must be true colour,
    24 or 32 bits, with or without RLE compression.
  - Moonwell writes a PNG or a TGA into the map as a TGA in the one layout the game is known to read, fully opaque:
    transparency is dropped, and each pixel keeps the colour the file stores for it.
  - A BLP must be a Warcraft III BLP (BLP1) and is used as it is.
- **Strict on purpose.** A picture the game cannot read closes the game the moment the map is selected in the list, for
  everyone who has the map. So a file of another size, format or extension fails the build and `check`.
- **What a build does.** Reforged's map list shows the map's minimap file, `war3mapMap.blp`, and ignores the
  `war3mapPreview.tga` of older versions. A build puts your picture in the minimap's place, keeps World Editor's minimap
  in the map as `war3mapMinimap.blp`, and adds one call at the end of `main()` in `war3map.lua`,
  `BlzChangeMinimapTerrainTex("war3mapMinimap.blp")`, so the game itself shows the normal minimap. As with every
  setting, only the staged copy changes.
- **Start locations.** The game draws its start location markers over the picture, placed for a 256×256 one. On a
  512×512 picture they sit smaller and toward the top left.
- **A minimap of your own.** Gameplay code that calls `BlzChangeMinimapTerrainTex` in an `on_main` hook, or later, runs
  after the build's call and wins. A World Editor trigger that sets the minimap at map initialization runs before it and
  is overridden: set the minimap from gameplay code instead.
- **The source map** must have its `war3mapMap.blp`, which World Editor writes at every save, and no file named
  `war3mapMinimap.blp` or `war3mapMap.tga`.

This was measured on Warcraft III Reforged 3.0.0.24268, in the single-player map list.

### Checking settings

`moonwell settings:check` checks the settings against the source map without building and lists the internal files a
build would change:

```text
  war3map.w3i
  war3map.lua
  war3mapMisc.txt
Map settings valid: 3 internal file(s) would change during build.
```

A file a build removes is listed as `war3mapMap.blp (removed)`; that happens for a TGA picture.

`moonwell check` (and so `dev`) checks settings the same way. Mistakes in the manifest name the manifest that was
evaluated (`moonwell.local.pkl` when it exists, else `moonwell.pkl`). Problems with the map name the file under
`maps/<folder>/`, such as `maps/map.w3x/war3map.w3i`. Map files are matched ignoring letter case, as Warcraft III
does: a map saved with `war3mapskin.txt` is patched under that name.

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

`moonwell objects:check` validates without building, lists the internal files a build would change, and says whether
`src/generated/objects.yue` is current. `check` and `dev` run the same checks. `moonwell objects:eval` prints every
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

| Command                                         | What                                                                             |
| ----------------------------------------------- | -------------------------------------------------------------------------------- |
| `moonwell init <dir>`                           | Create a project in a new folder                                                 |
| `moonwell build [--entry src/x.yue] [--minify]` | Build `dist/bin/<map>.w3x`                                                       |
| `moonwell test [--entry src/x.yue] [--minify]`  | Stage the map and launch Warcraft III                                            |
| `moonwell dev`                                  | Re-check on every save                                                           |
| `moonwell check`                                | Compile and validate without building                                            |
| `moonwell assets:check`                         | Show what `assets:sync` would change in the source map                           |
| `moonwell assets:sync`                          | Write `assets/` into the source map for World Editor (close the map first)       |
| `moonwell assets:paths [file]`                  | List the files a model references, as in-game or custom paths                    |
| `moonwell settings:check`                       | Show which internal map files the settings would change, without building        |
| `moonwell objects:check`                        | Validate the objects and show which internal map files they would change         |
| `moonwell objects:eval`                         | Print the resolved objects as JSON                                               |
| `moonwell setup`                                | Create a missing `moonwell.local.pkl`, download YueScript and prepare the editor |

A command line is read strictly: Moonwell runs a line it understands whole, or nothing. Each of these is an error
with a hint, such as `Did you mean --minify?`:

- a flag Moonwell does not have (`moonwell build --minfy`), and a command it does not have;
- a flag the command does not have (`moonwell check --minify`), and a flag without a command (`moonwell --minify`);
- an argument the command does not take (`moonwell build extra`);
- a value for a flag that takes none (`--minify=false`), and `--entry` without a `.yue` file under `src/`;
- a flag given twice with two values (`--entry src/a.yue --entry src/b.yue`);
- several short flags in one (`-hv`): write `-h -v`.

A flag may stand before or after the command. `--entry` takes its file after a space or after `=`. `--` ends the
flags. `moonwell --help` (`-h`) prints the commands and `moonwell --version` (`-v`) the version. A command ends with
the exit code 0, with 1 when it fails, and with 130 after Ctrl+C.

The compiler, and Pkl when Moonwell needs its own, are downloaded once per version and verified by checksum. They are
cached in `MOONWELL_CACHE` when that is set, else in `%LOCALAPPDATA%\moonwell` on Windows, else in
`$XDG_CACHE_HOME/moonwell` or `~/.cache/moonwell`. `setup` also copies Moonwell's own Pkl to that folder's `bin`.

## Advanced settings

These are not in the generated files and keep their defaults unless you add them. The schema, `Project.pkl` in the
`moonwell` Pkl package, documents every setting.

| Setting       | Default  | What                                                                                        |
| ------------- | -------- | ------------------------------------------------------------------------------------------- |
| `yue.version` | `0.34.3` | YueScript compiler version. Only versions this CLI release pins a checksum for are accepted |
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
