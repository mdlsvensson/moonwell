# Contributing to Moonwell

## Layout

One Go module at the root, with two dependencies beyond the standard library: `cobra`, which reads the command line,
and `viper`, which reads the settings files.

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
- `gate/`: the files of the release gate (below): copied over a new project, they make the gate project. Not part of
  the program, and not embedded.
- `tools/gen/`: the generator (below). `tools/metadata/` and `tools/natives/` hold the two files it reads that are
  written by hand.
- `install.ps1`, `install.sh`: the install scripts a release serves.
- `THIRD_PARTY_LICENSES`: the licences of the fifteen modules the program is built with, each as its authors wrote
  it. A release hands the file out beside the programs.

## Working on Moonwell

- `go run ./cmd/moonwell <command>` runs the program from the checkout; in `template/`, `go run ../cmd/moonwell build`.
- `moonwell init --link <dir>`, run in the checkout or below it, creates a project that imports the checkout's
  `schema/`. On Windows the folder must be on the same drive as the checkout: Pkl cannot load a local dependency from
  another drive. A linked project is built with whatever `moonwell` is run in it.
- `go build -o <a folder on your PATH> ./cmd/moonwell` gives you the checkout's program as `moonwell`.
- The settings of your machine, `config.toml` in `.moonwell` in your user folder, are made by the install scripts and
  by no command. If you never ran an install script, write the file yourself as the README's "Your machine" shows:
  `moonwell test` needs its `launch.gameExecutable`. `install_test.go` runs the scripts with a user folder of its own.

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

- **Two dependencies, `cobra` and `viper`.** The only third-party modules are `github.com/spf13/cobra` with the
  two it needs (`pflag`, and `mousetrap` on Windows), and `github.com/spf13/viper` with the eleven it brings.
  Only `internal/cli` imports `cobra` and `pflag`; only `internal/manifest` imports `viper`, its decoder
  (`mapstructure`) and its TOML reader (`go-toml`), and nothing imports the other nine. There is no cgo.
  `module_test.go` holds `go.mod` to that list and `layout_test.go` holds the imports. A further module needs a
  design the maintainer approves; it is then added to the list in `module_test.go`, and `layout_test.go` says which
  package may import it. `THIRD_PARTY_LICENSES` names each module at the version `go.mod` requires, with its licence
  (`module_test.go` holds the names and the versions): when a version changes, copy the licence from the module
  anew, and when a module is added, add its section.
- **Errors.** An expected failure is a `*diag.Error` (or `diag.Problems`) with the file and a hint. Any other error, and
  any panic, is reported as an internal "please report" error, so a user's mistake must never reach it. The text of
  an error is made by a named function, at the bottom of the file that raises it.
- **No comments in the Go code.** The maintainer had them all removed on 2026-10-08. The lines the compiler reads
  (`//go:embed`, `//go:build`) stay. A name says what a thing is; what a package is for is in `ARCHITECTURE.md`.
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

## Release gate (before every release)

The gate is what the tests cannot hold: that the game and World Editor read what Moonwell writes. It has three
parts. What a command can settle is settled by commands. What only the game shows is looked at in a gate run: a map
that shows one thing at a time and writes what it prints to a file. What only World Editor or a code editor shows is
looked at there, one thing at a time. A release needs only the runs and looks whose code changed.

Up to 0.12.1 the gate was sixteen numbered steps, and the changelog's sections up to that release name them
("step 13"); they are in this file as the tag `moonwell@0.12.1` has it.

### The gate project

`gate/` holds the gate's own files. A gate project is a new linked project with `gate/` copied over it:

```
moonwell init --link <a folder outside the checkout>/gate     # in the checkout
```

Then copy everything in `gate/` into that folder, replacing its `moonwell.toml`. The project keeps the template's
map, its Captain and its `src/main.yue`, and gains:

- `objects/gate.pkl`: a hero, an ability of four levels, a buff, an item, a building and an upgrade of two levels;
- the map settings and the preview picture in `moonwell.toml`, with `preview.png` beside it;
- `assets/war3mapImported/gate-probe.tga`, a picture to import;
- `lua/gate_greeter.lua` and `lua/gate_counter.lua`, a module that returns a table and one that defines a global;
- `src/gate/steps.yue`, the module the runs share, and the four runs: `src/gate_start.yue`, `src/gate_objects.yue`,
  `src/gate_settings.yue` and `src/gate_assets.yue`.

A run is built with `moonwell build --entry src/gate_<run>.yue` (`--minify` for a minified run), and the game is
started on `dist/bin/map.w3x`:

```
& "<the game's folder>\_retail_\x86_64\Warcraft III.exe" -launch -windowmode windowed -loadfile "<project>\dist\bin\map.w3x"
```

Every line a run prints, Moonwell's own among them, is also written to
`Documents\Warcraft III\CustomMapData\moonwell-gate-<run>.pld`, anew at every line, so a run is read from that file
afterwards and needs no screenshot. A value a native can give is printed, and the file is held against the lists
below. What has to be seen is a step: one line on screen says what to look for, and Esc goes on to the next. Each
step stands alone: it makes what it shows and takes it away again, so nothing of an earlier step is on screen. Esc
is the only key; to see a step again, start the map again. Nothing is timed. A run says when it is done.

A test, `internal/cli/gate_test.go`, makes the gate project and builds every run, plain and minified, so a gate
file that no longer compiles fails the checks.

### Which runs a release needs

Build the recorded projects (`internal/build/seeds_test.go`) with the version being released and with the release
before, and compare every staged and packed file. Where nothing differs and none of the paths below changed, the
gate is the commands and one start of the run `start`. Otherwise:

| What changed | Runs and looks |
| --- | --- |
| `runtime/`, `internal/script`, `internal/war3/lua` | `start`, and `start` minified |
| `internal/objects`, `internal/war3/objmod`, `schema/`, `data/metadata.json` | `objects`; the Object Editor |
| `internal/settings`, `internal/war3/w3i`, `internal/war3/txt`, `internal/war3/picture` | `settings`; the lobby; World Editor's dialogs |
| `internal/assets`, `internal/war3/imp`, `internal/war3/model` | `assets`; the Import Manager |
| `internal/war3/mpq`, `internal/build/pack.go`, `internal/build/archive.go` | `start`; the packed map opens in World Editor |
| `internal/editor`, `data/natives.json` | The looks in the editor |
| `internal/build/launch.go`, `internal/env/spawn_windows.go`, `internal/env/spawn_unix.go`, `internal/cli/testcmd.go` | `moonwell test` |
| A new version of the game | Every run, once |

The changelog's section of the release says which runs and looks were made, which were left out and why, and the
versions of the game and of World Editor.

### Checked by commands

1. Every check above, from a clean checkout. Then build the checkout's program into the folder the install script
   uses, so that the commands below run it as `moonwell`:
   `go build -o "$env:LOCALAPPDATA\moonwell\bin\moonwell.exe" ./cmd/moonwell` on Windows,
   `go build -o ~/.local/bin/moonwell ./cmd/moonwell` on Linux. `moonwell --version` must print the version being
   released.
2. `data/game-paths.txt` starts with `# Warcraft III <version>`, not the "Not generated yet" placeholder: with the
   placeholder every in-game path is reported as `custom path, not imported`.
3. In the gate project, `moonwell objects:check` lists the ten object files, `moonwell settings:check` lists
   `war3map.w3i`, `war3map.lua`, `war3mapMisc.txt`, `war3mapMinimap.blp`, `war3mapMap.blp (removed)` and
   `war3mapMap.tga`, and every run builds. The gate's test does the same; this is the same thing with the program
   that is released.
4. Tests hold what earlier gates checked by hand, and need no step of their own: a misspelt native and its hint, and
   the warning mode (`TestE2ELintMisspeltNativePositionAndHint`, `TestE2ELintWarningBuildSucceeds`); Lua modules and
   their globals (`TestE2ELuaModulesRequiredAndUnused`, `TestE2ELuaGlobalsKnownOnlyWhenRequired`); a library from a
   local folder and a library by tag (`TestE2ELocalLibraryBuildAndEditorView`, the network tests of
   `internal/library`); an archive that is written whole or not at all
   (`TestWriteArchiveWritesBesideThePlaceAndMovesTheWholeArchiveThere`); Ctrl+C and the build lock
   (`TestMoonwellExecutable`, `TestLeavingAtOnceGivesBackTheBuildLockAndThenExitsWith130`).
5. The published Pkl package is checked by the release workflow's last job, which installs the release and builds a
   project made by a plain `moonwell init` (Publishing, step 4).

### The runs

**`start`** shows the runtime, the script and the modules. Its file holds, in this order:

```text
Moonwell is running.
Gate start: missing collectgarbage dofile loadfile debug io package
Gate start: os clock date difftime time
Gate start: fourcc 1751543663
Gate start: captain 1747988528
Gate start: lua module hello gate
Gate start: lua global 1
Gate start: the next two lines are errors raised on purpose
[moonwell] on_main failed: src/gate_start.yue:41: gate error in a hook
[moonwell] on_main failed: lua/gate_greeter.lua:8: gate error in a Lua module
```

The two `[moonwell]` lines carry the game's colour codes, and the numbers are the lines the two `error` calls stand
on. In a minified build the first names `src/gate_start.yue` without a line and the second keeps its line. The
`missing` names are the `removed` list of `tools/natives/lua-extras.json`. Then two steps to look at:

1. A Captain stands in view, with a captain's model, and its own icon when it is selected.
2. The Captain changes colour every second (with ally colour mode off: Alt+A toggles it).

**`objects`** shows custom objects of every kind. Its file holds what natives say of `objects/gate.pkl`: the hero's
name (`Gate Paladin`), its proper name (`Gatekeeper`) and its strength (33); the ability's level (4), and for levels 1
to 4 what it heals (111, 222, 333, 444) and its cooldown (1, 2, 3, 4); the item's name (`Gate Claws`) and icon path;
the buff's icon path; the upgrade's name; the building's name (`Gate Smith`). Then four steps to look at:

1. The selected hero is Gatekeeper, a Gate Paladin.
2. The selected hero carries one item with the holy bolt icon, called Gate Claws.
3. The research button of the selected building reads "Research Gate Edge One".
4. With the first level researched, it reads "Research Gate Edge Two".

**`settings`** shows the map settings the template's map allows. Its file holds: player 0 is an orc (`true`); the
food ceiling (42); the level a hero stops at (7); and that World Editor's minimap is in the map under the name a
build gives it (`true`). Then two steps to look at:

1. The ground far away fades into red fog.
2. The minimap shows the terrain, not the preview picture (red with a white square).

**`assets`** shows that an imported file reaches the game. Its file holds `true` for the imported picture and
`false` for a name that is not in the map (`BlzChangeMinimapTerrainTex` answers so). It has no step to look at.

A value a run prints that differs from these lists is the gate failing, or the game answering a native in a way
this file did not expect: the lists were written from the gate's own files, and the section of the first release
that played each run records what the game printed.

### The looks outside a run

- **The lobby.** Copy `dist/bin/map.w3x` into the game's `Maps` folder and select it in the single-player custom
  game screen: the list shows the preview picture, the map is called "Moonwell Gate", and the first slot is named
  "Gate Player". A picture the game cannot read closes the game when the map is selected, so a crash there is this
  look failing.
- **World Editor, on the packed map.** It opens. The Import Manager lists `war3mapImported\gate-probe.tga`. The
  Object Editor lists every custom object under Custom with the values of `objects/gate.pkl`: the heal amounts and
  cooldowns per level, the upgrade's names per level, the item's price (123), the icons. Map Description, Loading
  Screen ("Moonwell Gate Loading"), Player Properties, Map Options (red fog, red water, the Dungeon sound
  environment) and Gameplay Constants show the configured values. The water's colour and the sound environment are
  looked at here and not in a run: the template's map has no water, and an echo is nothing a step can show.
- **World Editor, on the source map.** Change the standard Footman's hit points in the project's `maps/map.w3x` and
  save. A build must keep the change beside the gate's objects: the packed map's Object Editor shows both. For the
  import index, run `moonwell assets:sync`, open the source map and confirm the Import Manager lists the picture;
  save the map in World Editor and close it, then `moonwell assets:check` must report no changes (World Editor 3.00
  saves the import with flag 29).
- **Forces.** When the code for forces changed: enable custom forces in World Editor (Scenario > Force Properties),
  save the source map, and add a `[[settings.forces]]` entry with `index = 0` and a `name`, `allied` and
  `sharedVision`. The lobby shows the team, and World Editor's Force Properties show the values.
- **The editor.** Run `moonwell setup` in the gate project and open the project folder itself in VS Code (File >
  Open Folder; opened any other way, lua-language-server finds no `.luarc.json`), with the YueScript and Lua
  extensions it recommends. In `src/gate_start.yue`: hovering `CreateUnit` shows its parameter names and types;
  `mw.on_main` and `objects.units.captain` complete; `greeter.greet` completes; no error stands on the
  `moonwell.macros` import or on `$FourCC("hfoo")`.
- **`moonwell test`.** In the gate project, with `launch.gameExecutable` in your `config.toml`: the game starts on
  the staged map, its window stays open after `moonwell` has ended, and the game's log
  (`Documents\Warcraft III\Logs\War3Log.txt`) names `dist\stage\map.w3x` as the map it opened. With `[test]` and
  `archive = true` in that `config.toml`, the last line names `dist/test/map.w3x` and the log names that file.


## Publishing

A release is built by `.github/workflows/release.yml` from a tag `moonwell@<version>`. The tag's name is fixed: the Pkl
package's download address is built from it.

1. Set the version in `version.go`, `schema/PklProject`, `install.ps1`, `install.sh` and the README's three examples (a
   test names any that was missed), re-resolve the template's Pkl
   dependencies (`cd template && pkl project resolve`), and write the changelog's section, headed
   `## <version> (<date>)`: the release notes are its entries up to the first `###` heading. Commit, push, and wait for
   CI.
2. Run the release gate above.
3. Tag the pushed commit and push the tag: `git tag moonwell@<version>`, `git push origin moonwell@<version>`. The
   workflow fails at once unless the tag, `version.go` and `schema/PklProject` agree. It then runs the checks, builds
   `moonwell-windows-amd64.exe` and `moonwell-linux-amd64` with their `checksums.txt`, packages the schema
   (`moonwell@<version>.zip` and the metadata file `moonwell@<version>`, which `package://pkg.pkl-lang.org/...` URIs
   redirect to), and creates the GitHub release with those files, the two install scripts and
   `THIRD_PARTY_LICENSES`.
4. Read the workflow's result (`gh run watch`). Its last job is the check from outside the repository: on Ubuntu and
   Windows it runs the install line of the new version, then `moonwell init my-map` and `moonwell build`.
5. On your own machine, run the install line once. A release that fails after the tag is pushed is fixed with a new
   version, not by moving the tag: projects and the Pkl package index remember what a tag held.

A Moonwell release is a full release, never a pre-release. The README's install line downloads from
`releases/latest/download/`, and GitHub's "latest" skips pre-releases: marking the newest release as a pre-release
makes that line answer 404, which Windows PowerShell reports as "The connection was closed unexpectedly". The workflow
creates the release as the latest one, and its install job runs the README's line, so it fails if that ever breaks.
