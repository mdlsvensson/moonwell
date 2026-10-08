# Contributing to Moonwell

## Layout

One Go module at the root, with one dependency beyond the standard library: `cobra`, which reads the command line.

[`ARCHITECTURE.md`](ARCHITECTURE.md) is the way into the code. It says what every package does, how a command runs,
what a build does step by step, and which file to open for what. Read it before you change the code. In short:

- `cmd/moonwell/`: the program. It is one call of `internal/cli`.
- `internal/`: the program's packages, each with its tests beside it, on four shelves. `cli` reads the command line
  and calls `build`. `build` holds the order of a build and calls the areas: `objects`, `settings`, `assets`,
  `script`, `library`, `toolchain` and `editor`. The areas stand on the foundations (`manifest`, `mapdir`, `env`,
  `fsx`, `diag`, `binio`) and on the readers and writers of the map's file formats (`internal/war3/`). A package
  imports from the shelves below its own and never from above, and `layout_test.go` fails for an import that does
  not. `internal/testkit` and `internal/tooltest` hold what tests share.
- `embed.go`, `version.go`: package `moonwell`, the files the program carries (`template/`, `runtime/`, `data/`) and its
  version.
- `runtime/`: `moonwell.lua`, bundled into every map, and `macros.yue`.
- `data/`: the game data the program carries: the in-game path list, the object metadata and the natives.
- `schema/`: the `moonwell` Pkl package. (Not named `pkl/`: on Windows a `pkl` folder in the working directory shadows
  the `pkl` executable for tools that launch it from the repo root.)
- `template/`: the project `init` scaffolds. It is a linked project (its `PklProject` imports `../schema`), so use it to
  try changes. Nothing stray may be left in it: every file there goes into every new project, and a test pins the list.
- `tools/gen/`: the generator (below). `tools/metadata/` and `tools/natives/` hold the two files it reads that are
  written by hand.
- `install.ps1`, `install.sh`: the install scripts a release serves.

## Working on Moonwell

- `go run ./cmd/moonwell <command>` runs the program from the checkout; in `template/`, `go run ../cmd/moonwell build`.
- `moonwell init --link <dir>`, run in the checkout or below it, creates a project that imports the checkout's
  `schema/`. On Windows the folder must be on the same drive as the checkout: Pkl cannot load a local dependency from
  another drive. A linked project is built with whatever `moonwell` is run in it.
- `go build -o <a folder on your PATH> ./cmd/moonwell` gives you the checkout's program as `moonwell`.

## Checks

```
go vet ./...
gofmt -l .        # must print nothing
go test ./...
```

`go test` runs everything. On Windows, also run `GOOS=linux go vet ./...`: a few files are built for one system only.
`go test -short ./...` leaves out the slow tests, which build whole projects.

What a test needs from the machine, it asks for, and a variable says what happens when it is not there:

| Variable | What it does |
| --- | --- |
| `MOONWELL_REQUIRE_TOOLS=1` | A test that needs `pkl` or the YueScript compiler fails when the tool is missing. Without the variable such a test is skipped, which hides what it would have found: set it before a commit. |
| `MOONWELL_NETWORK_TESTS=1` | Adds the tests that download: the example library, and the pinned YueScript and Pkl. |
| `MOONWELL_TEST_YUE` | The path of a YueScript compiler for the tests, in place of the pinned one. |
| `MOONWELL_RECORD=1` | A recorded test writes its recordings anew, and fails (below). |
| `MOONWELL_GAME_SCRIPTS`, `MOONWELL_GAME_DATA`, `MOONWELL_GAME_LISTFILE` | The three exports of the game's files that the generator reads (below): the folder with the game's scripts, the folder with its object data, and the list of its file names. The generator's tests on the real files run only where they are set. |
| `MOONWELL_REQUIRE_EXPORTS=1` | A test that needs an export fails when its variable is not set. Without it such a test is skipped. |

`pkl` must be on the PATH: tests do not use the Pkl that `moonwell` downloads for itself. The compiler is downloaded
once into the user's cache. `.github/workflows/ci.yml` runs the three commands on Ubuntu and Windows with
`MOONWELL_REQUIRE_TOOLS` and `MOONWELL_NETWORK_TESTS` set, and the race detector on Ubuntu for the packages that
use goroutines. CI has no export of the game's files, so the tests on them run on a contributor's machine only.

### Recorded tests

Six packages have a test that compares something whole with a file in the repository, a recording, under the
package's `testdata/recorded/`: `internal/war3/lua`, `internal/settings`, `internal/assets`, `internal/objects`,
`internal/build` and `internal/cli`. The last two build whole projects and run whole command lines, and record every
line printed and every file left. `ARCHITECTURE.md` says what each recording holds.

When you change what the program does on purpose, such a test fails and shows the first line that differs. Write the
recording anew with the one test and its package named:

```
MOONWELL_RECORD=1 go test -run TestTheBuildsOfTheSeedsAreAsRecorded ./internal/build
```

This run writes the recordings and fails by design; a run without the variable then passes. Read the diff of the
recordings before you commit: it is the whole change in what the program does. A recording changes only in the
commit that changes the behaviour, and the commit message says which recordings changed and why.

- A command that fails is recorded by its exit code, the file its error names and what it left, not by its words.
  Test the words of an error in the tests of the package that raises it.
- `internal/build/testdata/leftovers/` is a fixture: what a build by an earlier Moonwell left in a project. No
  program in the repository can make it again, so do not edit or delete it.
- `internal/testkit/testdata/` holds files saved by World Editor. They are never edited by hand.

## Rules

- **One dependency, `cobra`.** The only third-party modules are `github.com/spf13/cobra` and the two it needs
  (`pflag`, and `mousetrap` on Windows). Only `internal/cli` imports them, and there is no cgo. `module_test.go`
  holds `go.mod` to that list and `layout_test.go` holds the import. A further module needs a design the
  maintainer approves; it is then added to the list in `module_test.go`, and `layout_test.go` says which package
  may import it.
- **Errors.** An expected failure is a `*diag.Error` (or `diag.Problems`) with the file and a hint. Any other error, and
  any panic, is reported as an internal "please report" error, so a user's mistake must never reach it. The text of
  an error is made by a named function below a `// ---- errors ----` line, at the bottom of the file that raises it.
- **Layout.** A new package goes on a shelf: add it to `layout_test.go` and to `ARCHITECTURE.md`. A test fails when
  either document names a file that is not there, and another when a step of a build changes and the quote of `Plan`
  in `ARCHITECTURE.md` does not: change the quote and the numbered steps below it in the same commit.
- **Style.** `gofmt` and `go vet` are clean. Markdown is wrapped at 120 by hand.
- **Versions must agree.** `version.go`, `schema/PklProject`, both install scripts and the README's examples carry the
  same number, and tests check it.
- **Generated files.** After changing `data/metadata.json`, run `go run ./tools/gen`; a test fails when
  `schema/generated/` is stale. `template/`, `runtime/` and `data/` need no step: they are embedded when the program is
  built.

## The generator

`tools/gen` is a second program. It writes the data files the program carries (`data/`) and the part of the Pkl
schema that follows the game's data (`schema/generated/`). `ARCHITECTURE.md` says how it is built; this section is
for running it.

**The generator writes into the checkout it finds.** It looks for the `go.mod` of this module in the working folder
and then in each folder above it, and writes below that folder. A path on the command line is read from the working
folder. So run it in the checkout only when you mean to change `data/` or `schema/generated/`; its tests run it in
a checkout of their own.

### After a game patch

Run the four modes in this order:

| # | Command | Writes | Reads |
| --- | --- | --- | --- |
| 1 | `go run ./tools/gen natives <folder> <version>` | `data/natives.json` | the game's two scripts, `common.j` and `blizzard.j`; `tools/natives/lua-extras.json` |
| 2 | `go run ./tools/gen metadata <folder> <version>` | `data/metadata.json` | the game's tables of object fields and of standard objects, and its strings; `tools/metadata/overrides.json`; the `data/metadata.json` that is there, for the names already released |
| 3 | `go run ./tools/gen` | `schema/generated/*.pkl` | `data/metadata.json` |
| 4 | `go run ./tools/gen game-paths <listfile> <version>` | `data/game-paths.txt` | a list of the game's file names |

- Step 3 must follow step 2, which ends with "Now run `go run ./tools/gen`". Steps 1 and 4 stand alone.
- Step 3 prints `wrote <path>` for each file that changed. It also removes whatever else lies in
  `schema/generated/`, a file or a folder, and prints `removed <path>` for each: the folder holds nothing but the
  schema.
- Step 2 prints `renamed (N):` and a line for each field whose friendly name is not the name of its label:
  `<list> <id> "<name of the label>" -> "<name>" (<reason>)`. The reason is `override` for a pinned name, or
  `category prefix`, `rawcode`, or `category prefix and rawcode` for a field that was renamed because its name
  clashed with another's. Read these lines before you commit: they are the names authors will write.
- Then run the checks with the three export variables set and `MOONWELL_REQUIRE_EXPORTS=1`: those tests write the
  committed files again from the game's files and compare. They are the proof after a patch.
- Commit `data/` and `schema/generated/` together.

`<version>` is the game's, such as `3.0.0.24268`. `<folder>` is an export made with CascView, which keeps the game's
relative paths: the folder that holds `war3.w3mod`. Every file of an export is found without regard to letter case.
Nothing of the game's files is committed except the names, types and signatures the data files hold.

- **Natives.** The editor declarations and the unknown-global check come from `data/natives.json`: the names, types
  and signatures of `common.j` and `blizzard.j`, plus what only the game's Lua has, from
  `tools/natives/lua-extras.json`. Export `war3.w3mod/scripts/common.j` and `war3.w3mod/scripts/blizzard.j`.
  Comments in the `.j` files are Blizzard's text and are not copied.
- **Object metadata.** `data/metadata.json` holds every object field (its rawcode, friendly name, type and where it
  applies) and every standard object. Export `war3.w3mod/units/` and `war3.w3mod/_locales/enus.w3mod/`. The
  generator reads the tables in `war3.w3mod/units/`, the labels in
  `war3.w3mod/_locales/enus.w3mod/ui/worldeditstrings.txt`, and the names of the standard objects in every file of
  `war3.w3mod/_locales/enus.w3mod/units/` whose name ends in `strings.txt`. Those files are read in the order of
  their names, and where two give the same key, the later one has it.
- **In-game path list.** `assets:paths` knows which paths the game ships from `data/game-paths.txt`. Export the
  file names of the game's CASC storage to a text file, one on a line.

### The two files written by hand

Both are JSON. Write every key in small letters, and an id as the game writes it. A byte order mark at the start of
a file is accepted. A key that stands twice in one object is refused. An empty object, and `null` as the whole file,
read as a file that sets nothing.

`tools/natives/lua-extras.json` is what the game's Lua has beside the two scripts. It was written from a probe run
in the game (the release gate has the probe).

| Key | Holds |
| --- | --- |
| `functions` | A list of functions that only the game's Lua has. Each is an object with all three of `name` (a text), `params` (a list of `{"name": …, "type": …}`; `[]` for a function that takes nothing) and `returns` (a text: a type of JASS or of Lua). |
| `globals` | A list of names: the globals of Lua's standard library that the game provides. |
| `removed` | A list of names: those the game removes. |

Each of the three may be left out. No entry may be `null` or an empty text. Any other key is refused, in the file
and in a function or a parameter.

`tools/metadata/overrides.json` holds the decisions about friendly names.

| Key | Holds |
| --- | --- |
| `names` | Pins. Below it one of the five lists (`units`, `items`, `abilities`, `buffs`, `upgrades`), below that a field's id, and its value is the friendly name. |
| `removed` | The fields whose leaving the game is acknowledged. Below it one of the five lists, whose value is a list of ids. |

- A pin must name a field of its list, by the id in its exact letters. A pin that names none is refused, and so is
  a list that is none of the five.
- The fields of units and of items are one table of the game. A pin under `units` is of the items too, and one under
  `items` of the units. `removed` is not shared so: a field that was of both is listed under both.
- A pinned name is a small letter and then letters and digits of ASCII. It is no keyword of Pkl, and none of `id`,
  `base`, `source`, `properties` and `output`, which every object has.
- An id of three letters is written as its three letters, such as `Crs`.
- Either key may be left out. Any other key at the top of the file is refused.

### What a refusal asks for

The generator writes nothing when it refuses. It prints `error: ` and the refusal; a refusal that lists several
things is several lines.

Any mode:

| The refusal | What it asks for |
| --- | --- |
| `run gen in a Moonwell checkout` | Run it in the checkout, or below it. |
| `unknown mode '<x>'. The modes are …`, or a line that starts `Usage:` | The command line as the table above has it. |
| `<path>: <the system's reason>` | The file or folder that could not be read or written. A file of the checkout is named from the checkout, any other as it was opened. |

Either hand-written file (`<file>` is its path from the checkout):

| The refusal | What it asks for |
| --- | --- |
| `<file>: unknown field "<key>"` | Take the key out, or spell it as the tables above do. |
| `<file>: the key "<key>" stands twice in one object: write it once` | Keep one of the two. |
| `<file>: <place> is of the wrong kind (<kind>)` | The value at that place is a text where a list belongs, or the like. The place is the keys on the way to it joined by dots, with an entry of a list as its number, counted from 0: `functions.0.params.1.type`, `names.units.ucls`. |
| `<file>: the file is empty`, `unexpected EOF`, `invalid character …`, `something follows the JSON value` | Sound JSON, and one object. |
| `…: functions.<n> has no "name"` (or `"params"`, or `"returns"`) | Give the function the key, with a value. The place is written as in the row above: `functions.1` is the second function. |
| `…: functions.<n>.params.<m> lacks its "name" or its "type".` | Give the parameter both keys. |
| `…: globals.<n> is empty or null.` (or `removed.<n>`) | Write the name, or take the entry out. |

The mode `natives`:

| The refusal | What it asks for |
| --- | --- |
| `war3.w3mod/scripts/common.j is missing from <folder>` (or `blizzard.j`) | Give the folder of the export: the one that holds `war3.w3mod`. |
| `common.j:<line>: cannot read "<the line>"` | The script has a construct the parser does not know: teach it to `tools/gen/jass/`. |
| `<script>:<line>: the function never reaches endfunction`, `<script>: the globals block never reaches endglobals` | A whole script: the export is cut. |
| `<name> is declared twice (<place> and <place>).` | A place is `common.j`, `blizzard.j`, `lua` (a function of the extras), `type`, `lua.globals` or `lua.removed`. Usually the game's scripts now declare what `lua-extras.json` lists: take the entry out there. |

The mode `metadata`:

| The refusal | What it asks for |
| --- | --- |
| `<path> is missing from <folder>` | The folder of the export, or a whole export. |
| `<table>:<line>: …`, or `<table>: duplicate column '<name>' in the header row` | A sound table. If the table is as the game ships it, the parser `tools/gen/slk/` must learn its form. |
| `<table> has no column "<column>", which the generator reads. …` | The game renamed or dropped a column. Give it its new name in `tools/gen/export.go` (`columnsRead`, or the table's key) and where the generator reads it. |
| `<table>: <id>: the <column> cell '<cell>' is not a number` (or `is not a whole number`, or `is no count of levels`), or `the row has no <column> cell` | Look at the row. If the game means it, the generator must learn the form. |
| `cannot derive friendly names:` and a line that ends `no property can have this name (…); pin another under "names", "<list>", "<id>" in tools/metadata/overrides.json` | A pin, at the place the line gives. |
| the same, with `the pin is refused: no property can have this name (…)` | Another name in the pin that is there. |
| the same, with `<other id> (<label>) has this name too, and one object can have both: pin another name for one of the two …` | A pin for one of the two fields. |
| the same, with `the pin of "<id>" under names.<list> names no field: …` | The pin pins nothing: no field of the list has that id. Correct the id, in its exact letters, or take the pin out. An id under `removed` is not held so: its field is gone. |
| the same, with `names.<list> is none of the lists of fields (…)` (or `removed.<list>`) | Correct the list's name: one of the five, in small letters. |
| the same, with `no label for <key> in …` or `the row has no … cell, which names the label` | The export lacks the label: a whole export, or a change to the generator. |
| the same, with `no kind of object uses it: …` | A change to the generator. No pin helps. |
| `<table>: <id>: <other table> has no row for it` | A whole export, or a change to the generator. |
| `standard units break the rule for heroes: …` | A change to `categoryOfUnit` in `tools/gen/bases.go`, as the refusal says. Nothing can be pinned here. |
| `released friendly names would change. …` and a line `<list> <id> "<released name>" would become "<name>"` | Under `names`, pin the released name to keep it. Or pin the new name to change it on purpose: authors' manifests then change with it. |
| the same, with a line `<list> <id> "<released name>" would disappear` | List the id under `removed`, under the list the line names. |
| `data/metadata.json: <reason>` | The released file is no readable JSON: restore it from git. |

The mode without a name:

| The refusal | What it asks for |
| --- | --- |
| `data/metadata.json: <reason>` | Run the mode `metadata` first, or restore the file. |
| `cannot render the Pkl schema; fix the names in tools/metadata/overrides.json:` and a line for each field whose name is reserved, is no Pkl identifier, or is the name of another field of its module | A pin; then the mode `metadata` again, then this mode. |

The mode `game-paths`:

| The refusal | What it asks for |
| --- | --- |
| `no model or texture paths were recognized in the listfile; …` | A list with one file name on a line, in UTF-8. The path list was not changed. |

## Release gate (manual, before every release)

1. Run every check above from a clean checkout. Then build the checkout's program into the folder the install script
   uses, so that the steps below run it as `moonwell`:
   `go build -o "$env:LOCALAPPDATA\moonwell\bin\moonwell.exe" ./cmd/moonwell` on Windows,
   `go build -o ~/.local/bin/moonwell ./cmd/moonwell` on Linux. `moonwell --version` must print the version being
   released.
2. Confirm `data/game-paths.txt` starts with `# Warcraft III <version>`, not the "Not generated yet" placeholder:
   with the placeholder every in-game path is reported as `custom path, not imported`.
3. `cd template`, run `moonwell setup` (it creates `moonwell.local.pkl` if missing; check its `gameExecutable`), then
   `moonwell test`. Confirm "Moonwell is running." prints and the Captain north of the heroes changes colour every
   second (with ally colour mode off: Alt+A toggles it, and while it is on every unit shows blue, teal or red). Confirm
   the Warcraft III window is visible and stays open after `moonwell` exits.
4. Add `error "gate"` inside the `on_main` hook, run `moonwell test` again, and confirm the on-screen error names
   `src/main.yue` and the right line. Record which chunk-name form the game used.
5. Run `moonwell build --minify` and play `dist/bin/map.w3x` directly.
6. Open the packed map in World Editor and confirm it loads.
7. Assets, in a throwaway project so `template/` stays clean (a stray file there fails the template test):
   `moonwell init --link <temp dir>/assets-check` in the checkout, then in that project put a `.blp` icon at
   `assets/ReplaceableTextures/CommandButtons/BTNMoonwell.blp` and run `moonwell test`; the map must load. Close World
   Editor, run `moonwell assets:sync`, open `maps/map.w3x` and confirm the Import Manager lists
   `ReplaceableTextures\CommandButtons\BTNMoonwell.blp`. Save the map in World Editor and close it, then confirm
   `moonwell assets:check` reports no changes and `moonwell build` succeeds (World Editor 3.00 saves the import with
   flag 29). Delete the icon, sync again and confirm it is gone.
8. Map settings, in another throwaway project from `init --link`. In its `moonwell.pkl`, set `info.name` and
   `loadingScreen.title`; a `players` entry for a slot the map has (such as `["0"]` with a `name`, `race` and
   `fixedStart`); `environment.soundEnvironment`, `environment.waterColor` and fog (`enabled = true`, `start`, `end`,
   `color`); and `gameplay.heroMaxLevel` and `gameplay.foodLimit`. For team settings, first enable custom forces in
   World Editor (Scenario > Force Properties), save the map, and set `forces["0"]`, such as `name`, `allied` and
   `sharedVision`. Run `moonwell settings:check` and confirm it lists `war3map.w3i`, `war3map.lua` and
   `war3mapMisc.txt`. Run `moonwell test`, then `moonwell build --minify` and play `dist/bin/map.w3x` from the game's
   Maps folder. Confirm the lobby shows the map name, the slot and the team, and in the game the fog, water colour and
   ambient sound, the food ceiling, and that a hero cannot level past the set maximum. Open the packed map in World
   Editor and confirm Map Description, Loading Screen, Player Properties, Force Properties, Map Options (fog, water) and
   Gameplay Constants show the configured values. Confirm `maps/map.w3x` is unchanged (`git status`).
9. Object data, in another throwaway project: `moonwell init --link <temp dir>/objects-check` in the checkout. Commit
   it to a new git repository (`git init`) so sub-step 5 can use `git status`. This proves what the tests cannot: that
   the game and World Editor read the files Moonwell writes, including per-level values past level 1.
   1. Run `moonwell test`. Confirm the unit north of the heroes is the Captain (its model, name and icon) and still
      changes colour.
   2. Add `objects/gate.pkl` with: a hero based on the Paladin (`Hpal`) with a custom `name` and `startingStrength`,
      whose `hero` abilities are a custom ability based on Holy Light (`AHhb`) with `levels = 4`,
      `cooldown = List(1, 2, 3, 4)` and `properties { ["amountHealedOrDamaged"] = List(111, 222, 333, 444) }` (set
      `heroSkin` to the same list, as the game data does for every hero); a custom buff for that ability's `buffs`, with
      a new `icon`; a custom item with a new `name`, `goldCost` and `interfaceIcon`; and a custom building based on the
      Blacksmith (`hbla`) whose `researchesAvailable` is a custom upgrade with `levels = 2` and per-level names and
      tooltips (`name = List("...", "...")`, `tooltip = List("...", "...")`). Run `moonwell objects:check`: it lists
      the ten files and reports `src/generated/objects.yue` stale until the next build. In `main.yue`, create the hero,
      the building and the item for player 0 (from `objects.heroes`, `objects.buildings` and `objects.items`), raise the
      hero to level 7 (`SetHeroLevel hero, 7, false`; a hero ability's level 4 needs hero level 7), and give player 0
      gold and lumber for the research (`SetPlayerState Player(0), PLAYER_STATE_RESOURCE_GOLD, 5000`, and the same for
      lumber).
   3. Run `moonwell test`. The hero shows its name and strength. Learning the ability shows its level 1 to 4 tooltips,
      and healing a wounded unit heals 111, 222, 333 and 444 at the four levels, with the four cooldowns. The item shows
      its name and icon. The building offers only the custom upgrade; its button shows the level 1 tooltip, and after
      researching level 1 the level 2 tooltip (the research button shows the tooltip, not the name; level 2, like the
      Blacksmith's, needs a Keep, so it shows greyed out).
   4. In World Editor, change the standard Footman's hit points in the project's `maps/map.w3x` and save. Run
      `moonwell build --minify`, play `dist/bin/map.w3x`, and confirm the Footman change survived next to the Moonwell
      objects.
   5. Open the packed map in World Editor. The Object Editor lists every custom object under Custom with the configured
      values: the per-level heal amounts and upgrade names, the item's price, the skin fields (models, icons), and the
      buff's icon (Holy Light applies no buff in game, so the buff is only checked here). Confirm `maps/map.w3x` is
      unchanged apart from sub-step 4's edit (`git status`), and that a second build leaves `src/generated/objects.yue`
      unchanged.
10. Editor, in another throwaway project: `moonwell init --link <temp dir>/editor-check` in the checkout, committed
    to a new git repository (`git init`). Run `moonwell setup` there and, if it prints a PATH command, run it once in
    PowerShell, then open a new terminal. Open the project folder itself in VS Code (File > Open Folder; opened any
    other way, lua-language-server finds no `.luarc.json`) and install the YueScript and Lua extensions it recommends.
    In `src/main.yue`, confirm that hovering or completing `CreateUnit` shows its parameter names and types, and that
    `mw.on_main` and `objects.units.captain` complete. In World Editor, place a unit in `maps/map.w3x`, reference it in
    a trigger (World Editor writes a `gg_unit_...` global only for a unit a trigger uses) and save; after
    `moonwell check`, confirm typing `gg_unit_` offers every placed unit a trigger uses, including the template's
    `gg_unit_Hblm_0003` and `gg_unit_Hpal_0002`. Create `src/heroes/captain.yue` with
    `export default { greet: -> print "Hello" }`, add `import "heroes.captain"` to `src/main.yue`, save both, and
    confirm `captain.greet` completes after the import (lua-language-server indexes the git-ignored `.lua` files). Type
    `CreatUnit` for `CreateUnit` in `src/main.yue`: the editor underlines it, and `moonwell check` fails with
    `src/main.yue:<line>:<column> › Unknown global CreatUnit.` and `Did you mean CreateUnit?`. Set
    `lint { unknownGlobals = "warning" }` in `moonwell.pkl` and confirm `moonwell build` succeeds and prints the same
    lines as warnings; then undo both changes. Confirm `git status` shows no `.moonwell/` and no `src/**/*.lua` files.
11. Macros and the game's Lua, in the step 10 project: run `moonwell test` and confirm a standard Footman stands beside
    the Captain (the template makes it with `$FourCC("hfoo")`). In the editor (the project opened as step 10 says),
    confirm `src/main.yue` shows no error on the `moonwell.macros` import or on `$FourCC("hfoo")`, and that hovering
    `$FourCC("hfoo")` is quiet (the YueScript extension finds the module through `yueconfig.yue`'s `include`). Then save
    the Lua probe below as `src/probe.yue`, run `moonwell test --entry src/probe.yue`, and confirm the game shows
    `missing: collectgarbage dofile loadfile debug io package` (the `removed` list in `tools/natives/lua-extras.json`)
    and `os: clock date difftime time` (as README's "The game's Lua" says). Printed lines show on screen for a few
    seconds only; the message log (F12) does not keep them. Delete `src/probe.yue` afterwards.

    ```yue
    import "moonwell" as mw

    names = {
      "assert", "collectgarbage", "dofile", "error", "getmetatable", "ipairs", "load", "loadfile", "next", "pairs",
      "pcall", "print", "rawequal", "rawget", "rawlen", "rawset", "require", "select", "setmetatable", "tonumber",
      "tostring", "type", "xpcall", "_VERSION", "_G", "coroutine", "debug", "io", "math", "os", "package", "string",
      "table", "utf8", "FourCC", "__jarray"
    }

    mw.on_main ->
      missing = [name for name in *names when _G[name] == nil]
      print "missing: " .. table.concat missing, " "
      for lib in *{"os", "debug", "package"}
        t = _G[lib]
        if type(t) == "table"
          keys = [key for key in pairs t]
          table.sort keys
          print lib .. ": " .. table.concat keys, " "
    ```
12. Lua modules and libraries, in another throwaway project from `init --link`: add `lua/greeter.lua` that returns a
    table with a function that prints, and a global-style `lua/counter.lua` that defines a global function
    (`function CountUp() print("counted") end`). Use both from `src/main.yue` (`import "greeter"`, `require "counter"`,
    `CountUp!`). Confirm the editor completes the module's function and the global, `moonwell check` passes, and in the
    game (`moonwell test`) both print. Then add `error "lua gate"` inside the module's function, run
    `moonwell test --minify`, and confirm the game's error names `lua/greeter.lua` and the right line (Lua modules keep
    their lines in minified builds). Remove the error afterwards.

    Then add the example library by tag in `moonwell.pkl`
    (`libraries { ["example"] { github = "mdlsvensson/moonwell-example-lib"; tag = "v0.1.0"; dir = "src" } }`), use it
    from `src/main.yue` (`import "example.loud"`, `print loud.shout "Moonwell"`), run `moonwell check` and commit
    `moonwell.lock`. Confirm the editor completes `loud.shout`, and the game prints the shout. Check with the Lua
    extension's bundled lua-language-server, in the editor or from the command line
    (`<extensions>/sumneko.lua-<version>/server/bin/lua-language-server --check=<project> --checklevel=Hint`), that the
    library's modules give no duplicate-definition diagnostics between `.moonwell/libraries/` and `.moonwell/lua/`.
    Delete `.moonwell/`, run `moonwell check` again, and confirm `moonwell.lock` is unchanged. Finally clone the
    library next to the project, point `moonwell.local.pkl` at it
    (`libraries { ["example"] { path = "../moonwell-example-lib"; dir = "src" } }`), change `hello` in its
    `src/example/greet.lua`, and confirm `moonwell test` runs the change and `moonwell.lock` is unchanged.
13. Map preview, in another throwaway project from `init --link`: put a 256×256 `.tga` or `.png` beside `moonwell.pkl`
    and set `settings.info.preview` to its name (both go into the map as the same `.tga`). Confirm
    `moonwell settings:check` lists `war3map.lua`, `war3mapMinimap.blp`, `war3mapMap.blp (removed)` and
    `war3mapMap.tga`. Run `moonwell build` and copy `dist/bin/map.w3x` into the game's `Maps` folder. Open the
    single-player custom game screen and select the map: the list must show the picture, not the minimap. Start the
    game: the minimap must show the terrain, not the picture. A picture the game cannot read closes the game when the
    map is selected, so a crash there is this step failing.
14. The build lock, in any of the throwaway projects: run `moonwell build --minify` and, while it runs, press Ctrl+C
    twice. Confirm that the command ends at once, that `dist/.lock` is gone, and that the next `moonwell build` does
    not say "Another Moonwell build is running". Confirm that `dist/bin/map.w3x` is not there, or is a whole map:
    never a cut one. Note whether `dist/bin/map.w3x.tmp` is left: a build that is ended while it writes the archive
    can leave it, and the next build that packs replaces it.
15. Record the Warcraft III and World Editor versions in the changelog.
16. The published Pkl package. This step needs the release, so it is the first check after the tag is pushed
    (Publishing, step 5): in a folder outside the checkout, run `moonwell init my-map` without `--link`, then
    `cd my-map` and `moonwell build`. Every test and every step above uses a project linked to the checkout's
    `schema/`; only this one evaluates a manifest against the package that users get.


## Publishing

A release is built by `.github/workflows/release.yml` from a tag `moonwell@<version>`. The tag's name is fixed: the Pkl
package's download address is built from it.

1. Set the version in `version.go`, `schema/PklProject`, `install.ps1`, `install.sh` and the README's two examples (a
   test names any that was missed), re-resolve the template's Pkl
   dependencies (`cd template && pkl project resolve`), and write the changelog's section, headed
   `## <version> (<date>)`: the release notes are its entries up to the first `###` heading. Commit, push, and wait for
   CI.
2. Run the release gate above.
3. Tag the pushed commit and push the tag: `git tag moonwell@<version>`, `git push origin moonwell@<version>`. The
   workflow fails at once unless the tag, `version.go` and `schema/PklProject` agree. It then runs the checks, builds
   `moonwell-windows-amd64.exe` and `moonwell-linux-amd64` with their `checksums.txt`, packages the schema
   (`moonwell@<version>.zip` and the metadata file `moonwell@<version>`, which `package://pkg.pkl-lang.org/...` URIs
   redirect to), and creates the GitHub release with those files and the two install scripts.
4. Read the workflow's result (`gh run watch`). Its last job is the check from outside the repository: on Ubuntu and
   Windows it runs the install line of the new version, then `moonwell init my-map` and `moonwell build`.
5. On your own machine, run the install line once, then step 16 of the release gate: a plain `moonwell init` and a
   build against the published package. A release that fails after the tag is pushed is fixed with a new version,
   not by moving the tag: projects and the Pkl package index remember what a tag held.

A Moonwell release is a full release, never a pre-release. The README's install line downloads from
`releases/latest/download/`, and GitHub's "latest" skips pre-releases: marking the newest release as a pre-release
makes that line answer 404, which Windows PowerShell reports as "The connection was closed unexpectedly". The workflow
creates the release as the latest one, and its install job runs the README's line, so it fails if that ever breaks.
