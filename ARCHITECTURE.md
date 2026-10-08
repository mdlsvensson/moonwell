# The architecture of Moonwell

This is the way into the code for a developer who has not seen it. It says what the program does, where each part
lives, how a command runs from the first line to the last, and which file to open for a given question. It is written
from the code. A test, `documents_test.go`, fails when a path in it names a file or a folder that is not there.

Read it from the top once. After that, the table [Where do I look for X](#where-do-i-look-for-x) is the part to come
back to.

## What Moonwell does

Moonwell is one program, `moonwell`, that builds a Warcraft III map from a project folder. A project holds a map
that World Editor saved as a folder (the source map), gameplay code in YueScript and Lua, custom objects and map
settings written in Pkl, and files to import. `moonwell build` evaluates the project's manifest with Pkl (the
manifest is `moonwell.pkl`, or `moonwell.local.pkl`, which amends it, where a project has one), compiles the gameplay
to Lua with the YueScript compiler, and works out every file of the map that has to change: the object data, the map
settings, the imported files, and the map's script with the gameplay added at its end. It writes the changed map as a
folder, `dist/stage/<map.folder>`, and packs that into one `.w3x` archive. The source map is only read. The other
commands are parts of this: `check` works everything out and writes no map, `test` writes the folder and starts the
game on it, `dev` checks again on every save.

Four words of a project come back all through this document:

- The **source map** is `maps/<map.folder>`, the map as World Editor saved it. No build writes into it.
- The **stage** is `dist/stage/<map.folder>`, the built map as a folder. This document says "stage" of that folder
  only. (`dist/stage/lua`, beside it, is where the compiler's output is kept; it is called the compile's cache
  here.)
- The **entry** is the module that `map.entry` names, `src/main.yue` in a new project: where the gameplay starts.
  What goes into the map is the entry and every module it reaches through `import` and `require`.
- The **bundle** is the one block of Lua that a build adds to the end of the map's script: Moonwell's runtime and
  those modules.

## The repository

- `cmd/moonwell/`: the program's `main`. It is one line, a call of `internal/cli`.
- `internal/`: the packages the program is built from. They stand on four shelves, described below.
- `tools/gen/`: the generator, a second program. A contributor runs it after a game patch; it writes `data/` and
  `schema/generated/`. The program is not built from it.
- `tools/metadata/` and `tools/natives/`: the two files a contributor writes by hand for the generator.
- `data/`: game data the program carries: the list of file paths the game ships, the fields and standard objects of
  the object editor, and the game's script API. The generator writes all three.
- `schema/`: the Pkl package `moonwell`, which is the format of a project. `schema/generated/` is written by the
  generator, the rest by hand. `schema/tests/` holds the schema's own tests, in Pkl.
- `runtime/`: `runtime/moonwell.lua`, the Lua that goes into every map, and `runtime/macros.yue`, the macro module
  a project imports.
- `template/`: the project that `moonwell init` creates, file for file.
- `.github/workflows/`: the checks that run on every push (`.github/workflows/ci.yml`) and the release
  (`.github/workflows/release.yml`).
- `embed.go` and `version.go`: the root package, `moonwell`. It holds the files the program carries inside its
  executable (`template/`, `runtime/`, `data/`) and the version number.
- `install.ps1` and `install.sh`: the install scripts a release serves.
- The test files at the root (`layout_test.go`, `module_test.go`, `schema_test.go`, `install_test.go`,
  `documents_test.go`) are tests of the repository as a whole: what may import what, that no dependency crept in,
  that the version is the same everywhere, that the install scripts work, that this document names real files and
  quotes `Plan` as the source has it.

The module is `github.com/mdlsvensson/moonwell`. Beside the Go standard library it depends on one module,
`github.com/spf13/cobra`, which reads the command line, and on the two that `cobra` needs (`pflag`, and `mousetrap`
on Windows). `module_test.go` holds `go.mod` to exactly those three.

## The four shelves

The packages below `internal/` are sorted onto four shelves. A shelf is a level: a package imports packages from
the shelves below its own and never from a shelf above. To find where something is done, pick the shelf first.

```
                cmd/moonwell
                     |
  commands          cli             reads the command line, calls build or an area, prints, sets the exit code
                     |
  the build        build            the one package that knows the order of a build
                     |
  areas            objects  settings  assets  script  library  toolchain  editor
                     |              one job each; none imports another (editor is the exception)
                     |
  foundations      manifest  mapdir  env  fsx  diag  binio            no opinion about a build
  and formats      war3/w3i  war3/objmod  war3/imp  war3/mpq  war3/model  war3/picture  war3/lua  war3/txt
```

Three words for the packages:

- An **area** is a package with one job in a build, such as the custom objects or the imported files.
- A **foundation** is a package that knows nothing of a build: files, errors, a map folder, the manifest.
- A **door** is an exported function that other packages call. A package has few of them.

The Go code has no comments: the maintainer had them all removed on 2026-10-08, after 0.11.0, and what is left are
the lines the compiler reads (`//go:embed`, `//go:build`). What a package does and what it must not know is in the
table below and in the rest of this document; the code as it was commented is the tag `moonwell@0.11.0`.

### Commands

| Package | What it does |
| --- | --- |
| `internal/cli` | Reads the command line, holds the table of commands, has one function for each command, prints, and turns a command's outcome into an exit code. It holds no step of a build. |

### The build

| Package | What it does |
| --- | --- |
| `internal/build` | Holds the order of a build in one function, `Plan`, and the four doors that use it: `Build`, `Test`, `Check` and `Dev`. It also opens a project (`Load`, `Source`), writes the stage and the archive, keeps the build lock, watches files for `dev` and starts the game. |

### Areas

| Package | What it does |
| --- | --- |
| `internal/objects` | Checks the manifest's custom objects against the game's metadata and the objects the map has, plans the map's object files, and renders `src/generated/objects.yue` and the JSON of `objects:eval`. |
| `internal/settings` | Plans the map settings: it patches `war3map.w3i`, edits the calls World Editor wrote into `war3map.lua`, merges the two settings text files, and puts a preview picture in the minimap's place. |
| `internal/assets` | Collects the files to import and gives each its path in the map, plans the import with `war3map.imp`, writes it into the source map for `assets:sync`, and reports which files a model references for `assets:paths`. |
| `internal/script` | Turns source files into the Lua that goes into the map: it finds the modules, runs the YueScript compiler and keeps its cache, follows the requires from the entry, checks for unknown globals, renders the bundle and plans its place at the end of `war3map.lua`. |
| `internal/library` | Brings a project's libraries up to date: it downloads GitHub tag archives and copies local folders into `.moonwell/`, and keeps `moonwell.lock`. |
| `internal/toolchain` | Keeps the two programs Moonwell pins, the YueScript compiler and Pkl: it finds one in the cache or downloads it, checks its SHA-256, and decides which program runs when the user has one of their own. |
| `internal/editor` | Writes what a code editor reads: the declaration files in `.moonwell/types`, the libraries' modules as Lua in `.moonwell/lua`, and a project's editor files (`.luarc.json` and the others) where one is missing. |

### Foundations

| Package | What it does |
| --- | --- |
| `internal/manifest` | A project as Go values, the mirror of `schema/`. It runs `pkl eval` on the manifest and decodes the JSON, checks that the project's Pkl package is of the program's version, and holds the `PklProject` and `moonwell.local.pkl` a new project gets. |
| `internal/mapdir` | A map saved as a folder. It scans the folder once, finds a file in any letter case, holds planned changes as a view (the folder with the changes laid over it, and nothing written yet), and writes a view to the stage or into the folder. |
| `internal/env` | The outside world as one struct, `Env`: running a program, downloading an address, starting the game, the logger, the cache folder, the platform. |
| `internal/diag` | The expected failure (`Error`, `Problems`), the one function that turns any error into what a user reads (`Format`), and "did you mean". |
| `internal/fsx` | File helpers: listing, copying and removing, a path that must stay inside its folder, a journal of writes that can be undone, and the rules for text (the byte order mark, ASCII white space). |
| `internal/binio` | A little-endian reader that keeps its first failure, and a writer. |

### Formats

A format package takes bytes and returns bytes or values. It knows nothing of projects, manifests or folders.

| Package | What it does |
| --- | --- |
| `internal/war3/w3i` | Reads `war3map.w3i`, the map's info file, and edits it in place. |
| `internal/war3/objmod` | Reads the object files of a map (`war3map.w3u` and the others) and appends objects to them. |
| `internal/war3/imp` | Reads and writes `war3map.imp`, World Editor's index of imported files. |
| `internal/war3/mpq` | Writes an MPQ archive, which is what a packed map is, and the header some maps carry before it. |
| `internal/war3/model` | Lists the files a model references, from binary MDX or text MDL. |
| `internal/war3/picture` | Checks a preview picture, and rewrites a TGA or a PNG as the TGA the game reads. |
| `internal/war3/lua` | A tokenizer for Lua 5.3 and the scanners on it: functions and their calls, top-level globals, requires, what `war3map.lua` defines. It also writes Lua literals and splices edits into a source. |
| `internal/war3/txt` | Merges keys into the section-and-key text files of a map (`war3mapMisc.txt`, `war3mapSkin.txt`). |

### For tests only

| Package | What it does |
| --- | --- |
| `internal/testkit` | What tests share: files written and read back, the fixtures World Editor saved, a stand-in `Env`, builders for the formats, damaged input, and the recordings. |
| `internal/tooltest` | Gives a test the real YueScript compiler. It is apart from `internal/testkit` because it imports `internal/toolchain`. |

### What may import what

- `cmd/moonwell` imports `internal/cli` and nothing else of the module.
- `internal/cli` may import `internal/build`, the areas, the foundations and the formats.
- `internal/build` may import the areas, the foundations and the formats. It never imports `internal/cli`.
- An area may import the foundations and the formats, and no other area. The one exception is `internal/editor`,
  which may import `internal/objects` and `internal/script`: it renders what those two found.
- A foundation imports only the foundations named for it: `internal/diag` and `internal/binio` import nothing;
  `internal/fsx` imports `diag`; `internal/env` and `internal/mapdir` import `diag` and `fsx`; `internal/manifest`
  imports `env`, `diag` and `fsx`. No foundation imports a format.
- A format may import other formats, and `diag`, `fsx` and `binio`.
- Only `internal/env` imports `os/exec` and `net/http`, the two packages of the standard library that start a
  program and reach the network. (`internal/fsx` is let off for `os/exec`: it names one error type of it and starts
  nothing.) Test files and the two test-only packages may import both: `internal/testkit/tools.go` looks for `pkl`
  with `os/exec`.
- Only `internal/cli` imports `cobra`, `pflag` and `mousetrap`, the modules that read a command line: no other
  package knows how a line is read.
- The two test-only packages are imported by test files alone. A test file follows the rule of its package, and may
  also import its own package and the two test-only packages.
- The root package is under no rule. A package below `internal/` may import it for the embedded files and the
  version: `cli`, `manifest`, `objects`, `script`, `assets` and `editor` do.
- Nothing outside `tools/` imports a package below `tools/`. The generator may import `internal/objects`,
  `internal/script`, `internal/assets`, `internal/manifest`, `internal/fsx`, the root package and its own parsers.
  A parser imports nothing of the module.

`layout_test.go` is the proof. It reads the imports of every Go file below `internal/`, `cmd/` and `tools/` and
fails for each import that breaks a rule, and for a package that is on no shelf. The rules are data at the top of
that file (`foundations`, `areas`, `belowFormats`, `testOnly`, `outsideWorld`, `generatorMay`) and one function,
`allowed`. The root package's own files, `embed.go` and `version.go`, are outside its rules too: the test does not
read them, and they import nothing of the module.

A new package is added to that file, on its shelf, and to the tables above.

## How a command runs

Follow `moonwell build --minify`:

1. `cmd/moonwell/main.go` is `os.Exit(cli.Main())`.
2. `Main` in `internal/cli/cli.go` finds the working folder, makes two writers (lines for the terminal go to
   standard error, output meant for other programs goes to standard output), turns Ctrl+C into the cancelling of a
   `context.Context`, and calls `Run`.
3. `Run` calls `runIn`, and that `carryOut`, which does these things in order:
   - `tree` makes the command line as `cobra` reads it: a `cobra` command for each row of the table `commands`. A
     row has the name, the help text, how many arguments the command takes, the flags it has, and the function
     that runs it.
   - `cobra` reads the line. It answers `--help` itself, and so the commands `help` and `completion`, which are
     its own; `moonwell --version` is answered by the function of `moonwell` in `tree`. A line `cobra` cannot
     read is refused in its words, printed as every failure of Moonwell is, and nothing runs.
   - For a line that names a row, `run` refuses an `--entry` that is no `.yue` file under `src/`, and then makes a
     logger and the outside world: `env.NewLogger` and `env.New`. In a project the logger also writes
     `dist/moonwell.log`.
   - The row's function runs, and `exitCode` turns what it returned into 0, 1, or 130 after Ctrl+C. An error is
     printed here, once, by `diag.Format`.
4. The row's function for `build` is `runBuild` in `internal/cli/build.go`. It is one call:
   `build.Build(ctx, e, options)`.
5. `Build` in `internal/build/build.go` does the whole command. The next section follows it.

The command table and where each command goes:

| Command | Its function | What the function calls |
| --- | --- | --- |
| `build` | `runBuild` in `internal/cli/build.go` | `build.Build` |
| `test` | `runTest` in `internal/cli/testcmd.go` | `build.Test` |
| `check` | `runCheck` in `internal/cli/check.go` | `build.Check` |
| `dev` | `runDev` in `internal/cli/dev.go` | `build.Dev` |
| `init` | `runInit` in `internal/cli/initcmd.go` | writes the template's files and runs `pkl project resolve` |
| `setup` | `runSetup` in `internal/cli/setup.go` | Pkl, the manifest, the compiler, the editor's files, the libraries |
| `assets:check`, `assets:sync` | `runAssetsCheck` and `runAssetsSync`, each one call of `importAssets` in `internal/cli/assets.go` | `build.Load`, `build.Source`, `library.Sync`, `build.PlanAssets`, and for a sync `assets.Sync` |
| `assets:paths` | `runAssetsPaths`, which calls `assetsPaths` in `internal/cli/assetspaths.go` | in a project `build.Load`, `library.Sync` and `build.Assets`; then `assets.ReportModels` |
| `settings:check` | `runSettingsCheck` in `internal/cli/settings.go` | `build.Load`, `build.Source`, `settings.Plan` |
| `objects:check`, `objects:eval` | `runObjectsCheck` and `runObjectsEval`, which both start with `planObjects` in `internal/cli/objects.go` | `build.Load`, `build.Source`, `objects.Plan` |

The four commands that plan a whole build are one call of a door of `internal/build`. The commands about one area
(`assets:check`, `assets:sync`, `settings:check`, `objects:check`, `objects:eval`) open the project the way a build
does, with `build.Load` and `build.Source`, and then call that area. `init`, `setup` and `assets:paths` have steps
of their own, as the table says.

## The build, step by step

`Build` in `internal/build/build.go` reads top to bottom:

1. `Load` finds Pkl (`toolchain.PklProgram`) and evaluates the manifest (`manifest.Load`). The result is a
   `*manifest.Project`: every setting of the project as a Go value.
2. `TakeLock` writes `dist/.lock`, so that a second build in the same project fails at once. The manifest is
   evaluated first, so that a command run outside a project makes no `dist` folder there.
3. `clearedArchive` works out where the archive goes, `<build.folder>/<map.folder>`, and removes the archive of the
   build before.
4. `Plan` works out the whole map. Nothing of the map is written yet.
5. `stage` writes the planned map to `dist/stage/<map.folder>`.
6. `packInto` packs the planned map into an archive and writes it.

`Test` is steps 1, 2, 4 and 5, and then starts the game on the stage. `Check` is steps 1, 2 and 4, and then says
what the plan holds. `Dev` is that check again and again, with the ids module written as a build writes it. So the
order of a build is written once, in `Plan`.

### `Plan`

This is the function as the source has it, with the error check after each step left out (each is
`if err != nil { return nil, err }`: the first step that fails ends the plan). A test in `documents_test.go` holds
the quote to the source, so a step added to `Plan` is a step added here:

```go
func Plan(ctx context.Context, e *env.Env, p *manifest.Project, opts Options) (*Result, error) {
	source, err := Source(p)
	globals, err := MapGlobals(source)

	objs, err := objects.Plan(source, p.Objects, objects.LoadMetadata())
	err = writeGenerated(e, source, objs, globals, opts)
	synced, err := library.Sync(ctx, e, p.Libraries, p.ManifestName)
	program, err := compile(ctx, e, p, synced, globals, opts)

	view := source.WithChanges(objs.Changes)
	set, err := settings.Plan(view, p)
	view = view.WithChanges(set)
	imported, replaced, err := PlanAssets(ctx, view, p, synced)
	view = view.WithChanges(imported.Changes)
	bundle, err := script.Inject(view, program)
	view = view.WithChanges(bundle)
	return &Result{Map: view, Objects: objs, Settings: set, Assets: imported, Replaced: replaced, Program: program}, nil
}
```

The steps, one by one:

1. **`Source(p)`** opens the source map, `maps/<map.folder>`, as a `*mapdir.Folder`. The folder is scanned whole,
   once: a link, a name Windows cannot hold, or two paths that differ only in letter case is refused here, wherever
   in the map it is. It comes first because every later step reads the map, and a map that is missing should be
   reported before a compile error.
2. **`MapGlobals(source)`** reads the map's `war3map.lua` and lists what it defines: the `gg_` and `udg_` globals
   and the functions World Editor wrote. Two later steps need the list: the check for unknown globals, and the
   editor's declarations.
3. **`objects.Plan`** checks the manifest's custom objects against the game's metadata and against the objects the
   map already has. It returns three things: the changed object files, the resolved objects, and the text of the ids
   module. It runs before the compile for two reasons: an invalid object fails before the slow steps, and the
   gameplay imports the ids module, which must be current when it is compiled.
4. **`writeGenerated`** writes what the gameplay and the editor read beside the sources: the ids module
   `src/generated/objects.yue`, the declarations in `.moonwell/types`, and the macro module in `.moonwell/yue`. It
   stands before the two steps that may need the network, so the editor has its files also when a library cannot be
   fetched. For `check` it writes no ids module and fails when the one on disk is stale (`Options.KeepGenerated`).
5. **`library.Sync`** brings the copies of the libraries below `.moonwell/` up to date and writes `moonwell.lock`.
   It returns, for each library, the folder of its modules and the folder of the files it ships. It stands before
   the compile, which compiles the libraries' modules, and before the assets, which import the libraries' files.
6. **`compile`** (in `internal/build/steps.go`) finds the compiler (`toolchain.Compiler`) and makes the program in
   two steps of `internal/script`. `script.CompileSources` finds every module and compiles the YueScript ones into
   the compile's cache, `dist/stage/lua`. `script.Link` follows the requires from the entry and checks the modules
   it reaches for unknown globals. Between the two, `editor.RefreshLibraryView` writes the libraries' modules as
   Lua into `.moonwell/lua`: the editor then has them also when `script.Link` fails, which is when a user looks.
7. **`source.WithChanges(objs.Changes)`** makes the first view. A view is the source map with planned changes laid
   over it: a read through the view gives the changed file, and nothing is on disk yet.
8. **`settings.Plan(view, p)`** plans the map settings on the map as the objects leave it. It patches
   `war3map.w3i`, edits the calls World Editor wrote into `war3map.lua`, and merges the two text files.
9. **`PlanAssets`** (in `internal/build/steps.go`) collects the project's assets and the files the libraries ship,
   reads which files of the map `assets:sync` owns (the ownership state, `.asset-state/<map>.json`), and has
   `assets.Plan` plan the imports and `war3map.imp`.
10. **`script.Inject(view, program)`** appends the bundle, which is the runtime and every module the entry reaches,
    to `war3map.lua`. It is the last step because the settings edit the same file, and the bundle carries a table
    of the line each module starts on in the finished script. Nothing may change the script after it.
11. The `Result` holds the last view as `Map`, and what the commands report: the objects, the settings' changes,
    the assets and the program.

The upper half (steps 1 to 6) makes what the gameplay is compiled against, and compiles it. The lower half (steps 7
to 10) plans the map, each step on the map as the steps above it leave it. A function that takes a map folder and
returns changes is called a **planner**: `objects.Plan`, `settings.Plan`, `assets.Plan` and `script.Inject` are the
four. A planner writes nothing. `build.Plan` is not one of the four: it calls them, and its upper half writes beside
the map.

`Plan` writes nothing into the map, but it does write beside it: the ids module, everything below `.moonwell/`,
`moonwell.lock` and the compile's cache `dist/stage/lua`. So `check` writes those too, all but the ids module,
which it only compares with what the objects render. `dev` writes the ids module as well.

### After the plan

- `stage` in `internal/build/stage.go` has `mapdir` replace `dist/stage/<map.folder>` with a copy of the source map
  and write the view's changes into the copy (`Folder.StageTo` in `internal/mapdir/apply.go`). Then it logs what
  went in ("Added … custom object(s)", "Applied map settings", "Imported … asset(s)").
- `packInto` in `internal/build/archive.go` calls `pack` in `internal/build/pack.go`, which reads every file from
  the view, in the order of `Folder.Files`, and hands them to `mpq.Write`. The archive is written beside its place
  as `<map.folder>.tmp` and renamed when it is whole.
- `launch` in `internal/build/launch.go` starts the game with `-loadfile` and the stage, through `Env.Spawn`, and
  does not wait for it.

## The outside world

Everything the program uses from outside itself, other than files, is in one struct, `Env` in
`internal/env/env.go`:

```go
type Env struct {
	Root     string // the project folder
	Log      *Logger
	Run      RunFunc
	Fetch    FetchFunc
	Spawn    func(program string, args []string) error // detached: the game
	CacheDir string
	Platform string // "windows-x86_64", "linux-x86_64" or ""
}
```

`Run` runs a program and waits for it: `pkl`, `yue`, `tar`. `Fetch` downloads an address: a library's archive, a
pinned program. `Spawn` starts a program that outlives Moonwell: the game. `env.New(root, log)` is the real world.
`run` in `internal/cli/cli.go` makes one for each command and hands it down as the parameter `e`.

Only `internal/env` starts programs and reaches the network, and the layout test holds every other package to it.
The reason is the tests. A test hands the code an `Env` of its own (`testkit.Env` in `internal/testkit/env.go`),
whose `Run`, `Fetch` and `Spawn` fail the test until the test replaces them. So a test of `library` serves an
archive from memory, a test of `build` records what the game would have been started with, and no test downloads
or starts anything by accident.

Files are not behind `Env`. Every package reads and writes them directly, with `os` and `internal/fsx`, and a test
gives it a temporary folder. The program reads environment variables in two places: `internal/env/cache.go`, for the
cache folder (`MOONWELL_CACHE`, else the system's own variables), and `internal/toolchain/install.go`, for
`SystemRoot`, to find `tar.exe` on Windows. `cobra` reads a few of its own while a shell asks it for completions.

## Errors

There are two kinds of failure, and the difference is the type of the error.

An **expected failure** is anything a user can cause or meet: a mistake in the manifest, a missing file, a compile
error, a file another program holds open. It is a `*diag.Error` (`internal/diag/diag.go`): a message, the file to
look at (with a line and a column when known), a hint that says what to do, and the system's error as its cause when
there is one. When several are found at once, such as every invalid object or every unknown global, they are a
`diag.Problems`.

An **internal error** is any other error, and any panic. It means a bug in Moonwell. `diag.Format` prints it as
`internal error: …` with a line that asks for a report. So a user's mistake must never reach the user as a plain
error: an error from the operating system is wrapped into a `*diag.Error` in the function where it happens.

Three habits follow, and the code keeps them everywhere:

- A user-facing error is made by a named function, such as `errNoMap` or `errHeld`. These functions stand at the
  bottom of the file that raises them. To change the words of an error, search for a few of its words; to see
  every way a file can fail, scroll to its end.
- A plain error (`errors.New`, `fmt.Errorf`) appears only where the caller, not the user, made the mistake.
- A test of an error checks its file, its hint and the words that tell it apart, not the whole sentence. The tests
  of the packages that raise such errors each have a helper, `asError`, that fails the test for an error that is no
  `*diag.Error`.

`exitCode` in `internal/cli/cli.go` prints the error a command ends with, once. `carryOut` prints a line that is
refused, and catches a panic, which it prints as an internal error with its stack. `dev` goes on after a check that
fails, so `cycle` in `internal/build/dev.go` prints that failure itself.

The generator (`tools/gen/`) is a tool for contributors and does not use `internal/diag`: its failures are plain
errors, printed after `error: `.

## The generator

`tools/gen/` is a program of its own, with its own `main`. It writes the files that the program carries and the
part of the Pkl schema that follows the game's data. It has four modes; `modes` in `tools/gen/main.go` is the table.

| Command line | Writes | From | Door |
| --- | --- | --- | --- |
| `go run ./tools/gen natives <folder> <version>` | `data/natives.json` | the game's two scripts, and `tools/natives/lua-extras.json` | `writeNatives` in `tools/gen/natives.go` |
| `go run ./tools/gen metadata <folder> <version>` | `data/metadata.json` | the game's tables of object data, and `tools/metadata/overrides.json` | `writeMetadata` in `tools/gen/metadata.go` |
| `go run ./tools/gen` | the files of `schema/generated/` | `data/metadata.json` | `writeSchema` in `tools/gen/schema.go` |
| `go run ./tools/gen game-paths <listfile> <version>` | `data/game-paths.txt` | a list of the game's file names | `writeGamePaths` in `tools/gen/gamepaths.go` |

Each door reads top to bottom, like `Plan`. The rest of the generator:

- `tools/gen/export.go` finds and reads the files of an export: a folder of the game's own files, taken out of the
  game's storage with a tool for that. `CONTRIBUTING.md` says which files, and with which tool.
- `tools/gen/fields.go`, `tools/gen/names.go` and `tools/gen/bases.go` make the object metadata: a record for each
  field, its friendly name, and the standard objects.
- `tools/gen/handwritten.go` reads the two hand-written JSON files. The keys of `tools/natives/lua-extras.json` are
  in `tools/gen/extras.go`, those of `tools/metadata/overrides.json` at the top of `tools/gen/names.go`.
- `tools/gen/jass/`, `tools/gen/slk/` and `tools/gen/ini/` are three small parsers for the game's file formats.
  They import nothing of the module.

**The generator writes into the checkout it finds.** It looks for the `go.mod` that names this module, in the
working folder and then in each folder above it (`findCheckout`), and writes below that folder. A path on the
command line is read from the working folder. So it is never run from the checkout to try something out: a run
rewrites `data/` and `schema/generated/`. Its tests run it in a checkout of their own, in a temporary folder.

`CONTRIBUTING.md` has the order of the four modes after a game patch, the format of the two hand-written files, and
what each refusal asks for.

## The tests

Tests stand beside the code they test, in Go's test files, with fixtures in a `testdata` folder of the package.
`go test ./...` runs everything. A test prints nothing when it passes and writes only into a temporary folder.

The kinds of test:

- **Table tests of one function.** Most tests. A slice of cases, a loop.
- **Tests on damaged input.** Each reader of a format (`internal/war3/…`, and the generator's parsers) is given
  valid input with seeded changes: bytes swapped, text cut. It must return a value or an expected failure, and
  never panic. `internal/war3/w3i/damaged_test.go` is one; each reader that can refuse its input has such a file.
  `testkit.Changed`, `testkit.Swept` and `testkit.Panic` make the input and catch the panic.
- **Whole command lines.** `internal/cli/e2e_test.go` runs command lines in a project that `init` made, with the
  real Pkl and the real compiler. `internal/cli/process_test.go` builds the program and runs it as a process, for
  what only a process shows: the two streams, the exit code, Ctrl+C.
- **Recorded tests.** Described below.
- **Tests of the repository,** in the root package: the layout, the single version, the install scripts, the
  template's list of files, this document.
- **The schema's own tests,** `schema/tests/`, written in Pkl and run by `schema_test.go`.

### Recorded tests

A recorded test builds something whole and compares the result with a file kept in the repository, a recording,
below the package's `testdata/recorded/`. Six packages have one:

| Package | The test | What is recorded |
| --- | --- | --- |
| `internal/build` | `internal/build/recorded_test.go` | Whole projects (the seeds, in `internal/build/seeds_test.go`), built with the real Pkl and compiler: the lines logged, every staged file, the archive unpacked, every generated file. |
| `internal/cli` | `internal/cli/recorded_test.go` | Command lines as a user types them (in `internal/cli/seeds_test.go`): the exit code, both streams, and every file the line left. |
| `internal/objects` | `internal/objects/recorded_test.go` | The object files and the JSON for objects of every kind. |
| `internal/settings` | `internal/settings/recorded_test.go` | The patched script and info file for every setting, on scripts of many layouts. |
| `internal/assets` | `internal/assets/stories_test.go` | Runs of `assets:sync` and of a build, one after the other in one project. |
| `internal/war3/lua` | `internal/war3/lua/corners_test.go` | What the tokenizer and the scanners make of sources at the edges of the language. |

What a recording is:

- Text a person reads in a diff is recorded whole: printed lines, small text files. A large or binary file is
  recorded as its SHA-256 and its length.
- A recording is the same on every machine. The project folder is written `<root>`, and words that are the operating
  system's own are written `<reason>`.
- A command that fails is recorded by its exit code, the file its error names and what it left behind, not by the
  words of its message. The tests of the step that fails hold the words.

To make a recording anew, run the one test with `MOONWELL_RECORD=1`:

```
MOONWELL_RECORD=1 go test -run TestTheBuildsOfTheSeedsAreAsRecorded ./internal/build
```

That run writes the recordings and **fails by design**, so that no run passes by recording. Run the test again
without the variable, and it passes. Then read the diff of the recordings: it is the change in what the program
does. A recording changes only in the commit that changes the behaviour, and the commit message says why. Only
`Recorded` in `internal/testkit/recorded.go` writes a recording.

### The variables

| Variable | What it does |
| --- | --- |
| `MOONWELL_REQUIRE_TOOLS=1` | A test that needs `pkl` or the YueScript compiler fails when the tool is missing. Without it such a test is skipped, which hides what it would have found. |
| `MOONWELL_NETWORK_TESTS=1` | Runs the tests that download: the pinned programs and the example library. |
| `MOONWELL_TEST_YUE` | The path of a YueScript compiler for the tests to use, in place of the pinned one. |
| `MOONWELL_RECORD=1` | Recorded tests write their recordings, and fail. |
| `MOONWELL_GAME_SCRIPTS`, `MOONWELL_GAME_DATA`, `MOONWELL_GAME_LISTFILE` | Each names an export of the game's files on the machine. The generator's tests on the real files run only with them. |
| `MOONWELL_REQUIRE_EXPORTS=1` | A test that needs an export fails when its variable is not set, in place of being skipped. |

A test asks for what it needs by a helper: `testkit.NeedPkl`, `tooltest.Yue`, `testkit.NeedNetwork`,
`testkit.NeedExport`. The helper skips or fails the test as the variables say. Tests find `pkl` on the PATH; the
compiler is downloaded once into the user's cache. The slow tests are skipped with `go test -short`.

### The fixtures

- `internal/testkit/testdata/` holds files that World Editor saved: a map's info file and script, object files, an
  index of imports. They are the proof that the readers read what the editor writes. They are never edited by hand.
- `internal/build/testdata/leftovers/` holds what a build by an earlier Moonwell left in a project folder. The
  recorded builds run over it once, to show that a new build stands on nothing an older one left. No program in the
  repository can make it again.
- `internal/build/testdata/recorded/` and the other `testdata/recorded` folders hold the recordings.

## Where do I look for X

Each row names the file to open and, in most rows, the function to read first.

### The command line

| I want to | Open |
| --- | --- |
| see what `moonwell build` does, in order | `internal/build/build.go`: `Build`, then `Plan`; the longer steps in `internal/build/steps.go` |
| add a command | `internal/cli/cli.go`: a row in the table `commands`, and a function in a file of the package that the row names |
| add a flag, or give a command a flag | `internal/cli/cli.go`: an `option` beside `entryOption`, in the `flags` of each row of `commands` that has it, a field of `call` for what the flag says, and a line in `run` that reads it. The flag is also written by hand in the `usage` text of those rows. A flag of `build` and `test` then goes on through `options` in `internal/cli/build.go` to `Options` in `internal/build/build.go` |
| know why a command line is refused | `cobra` refuses it, in its own words: an unknown flag or command, a flag without its value, a wrong number of arguments (the `args` of the row in `commands`). `refusal` in `internal/cli/cli.go` prints it. Only an `--entry` that is no entry is refused by Moonwell, in `run` |
| change the help text | `internal/cli/cli.go`: the `usage` and `help` of each row of `commands`, the `help` of an `option`, and the `Long` text in `tree`. The layout is `cobra`'s |
| know how an outcome becomes an exit code, and where a panic goes | `internal/cli/cli.go`: `carryOut`, `exitCode` |
| change what Ctrl+C does | `internal/cli/cli.go`: `Main`, `heed`, `leaveAtOnce`; `internal/build/lock.go`: `ReleaseHeld` |
| know where the log file is written | `internal/cli/cli.go`: `logFile`; `internal/env/log.go` |
| change what `init` writes | `template/` for the files; `internal/cli/initcmd.go`: `createProject`; `internal/manifest/files.go`: `PklProjectText`, `LocalManifestText` |
| change what `setup` does | `internal/cli/setup.go`: `runSetup` |

### The build

| I want to | Open |
| --- | --- |
| add a step to a build, or move one | `internal/build/build.go`: `Plan` |
| know how `map.folder` is read and the source map opened | `internal/build/project.go`: `Source`, `mapFolder`, `readFolder` |
| know where "Another Moonwell build is running" comes from | `internal/build/lock.go`: `TakeLock`, `errHeld` |
| know why a link at `dist` is refused | `internal/build/output.go`: `outputAt` |
| change where the map is staged, or what is logged after | `internal/build/stage.go`: `stagePlace`, `sayStaged` |
| change where the archive goes, or how `build.folder` is read | `internal/build/archive.go`: `archiveOf`, `buildFolder`, `writeArchive` |
| know which files go into the archive, and in which order | `internal/build/pack.go`: `pack`, `archiveFiles` |
| know how large a map may be, or change the archive's format | `internal/war3/mpq/room.go`: `CheckFits`; `internal/war3/mpq/write.go`: `Write` |
| change what `dev` watches, or when it checks again | `internal/build/dev_watched.go`: `ownFolders`, `countsInProject`, for what is watched; `internal/build/dev.go`: `Dev`, `due`, for when; the watcher in `internal/build/watch.go` |
| change how the game is started | `internal/build/launch.go`: `launch`; `internal/env/spawn_windows.go` and `internal/env/spawn_unix.go` |

### The manifest and the tools

| I want to | Open |
| --- | --- |
| add a setting to the manifest | `schema/Project.pkl` or `schema/MapSettings.pkl`, then the struct in `internal/manifest/project.go` or `internal/manifest/settings.go`; `internal/manifest/load_test.go` fails for a field no manifest sets |
| know how the manifest is evaluated, and which of the two files | `internal/manifest/load.go`: `Load`, `manifestFile`, `Decode` |
| change the check of the program's version against the project's | `internal/manifest/version.go`: `checkPackageVersion` |
| know which `pkl` and which `yue` is run | `internal/toolchain/tools.go`: `PklProgram`, `Compiler` |
| pin a new version of YueScript or Pkl | `internal/toolchain/tools.go`: `YueScript`, `Pkl`; the default `yue.version` in `schema/Project.pkl` |
| change how a pinned program is downloaded and checked | `internal/toolchain/ensure.go`: `Ensure`; its steps in `internal/toolchain/install.go` |
| change the version number | `version.go`, `schema/PklProject`, `install.ps1`, `install.sh`, and the two examples in `README.md`; `module_test.go` and `install_test.go` name one that was missed |

### The gameplay code

| I want to | Open |
| --- | --- |
| know how a file becomes a module, and what its name is | `internal/script/modules.go`: `Collect` |
| change how the compiler is run, or what is compiled again | `internal/script/yue.go`: `compileAll`, and `compile` for the one call of the compiler; the cache in `internal/script/cache.go`. The check for unknown globals runs the compiler too: `list` in `internal/script/unknown.go` |
| change the words of a compile error | `internal/script/printed.go`: `compileError`, `rewriteError` |
| know which modules go into the map, and where "module not found" comes from | `internal/script/graph.go`: `reached`, `errNoModule` |
| change the check for unknown globals | `internal/script/unknown.go`: `unknownGlobals`, `knownGlobals` |
| know what the bundle looks like, and how an error in the game finds its source line | `internal/script/bundle.go`: `bundle`; `runtime/moonwell.lua` |
| know how the bundle gets into `war3map.lua` | `internal/script/inject.go`: `Inject` |
| add a macro | `runtime/macros.yue`; it is written into a project by `RefreshMacros` in `internal/script/macros.go` |
| change what Moonwell reads out of a Lua source | `internal/war3/lua/token.go`: `Tokenize`; the scanners in `internal/war3/lua/functions.go`, `internal/war3/lua/globals.go`, `internal/war3/lua/requires.go` |

### Objects, settings, assets, libraries, the editor

| I want to | Open |
| --- | --- |
| follow an object from Pkl to the bytes of `war3map.w3u` | `internal/objects/plan.go`: `Plan`; `internal/objects/resolve.go`: `Resolve`; `internal/war3/objmod/append.go`: `AppendObjects` |
| make a field of an object settable from Pkl | Nothing in Go: the fields are data. `go run ./tools/gen metadata <folder> <version>` writes `data/metadata.json` from the game's tables, and `go run ./tools/gen` then writes `schema/generated/`; `CONTRIBUTING.md` has the steps. The program reads the file in `internal/objects/metadata.go`: `LoadMetadata` |
| change the words of an error about an object | the errors at the bottom of `internal/objects/resolve.go`, `internal/objects/fields.go` and `internal/objects/values.go` |
| change `src/generated/objects.yue` | `internal/objects/ids.go`: `RenderIDs`, `RefreshIDs`, `AssertIDsCurrent` |
| change the JSON of `objects:eval` | `internal/objects/eval.go`: `EvalJSON` |
| follow a setting into `war3map.w3i` | `internal/settings/plan.go`: `Plan`; `internal/settings/info.go`: `patchInfo`; `internal/war3/w3i/edit.go` |
| change how World Editor's calls in `war3map.lua` are edited | `internal/settings/lua.go`: `patchLua`; `internal/settings/lua_patcher.go`; one file for each group of calls beside it |
| change how gameplay constants are written | `internal/settings/constants.go`: `textSections`; `internal/war3/txt/txt.go`: `Merge` |
| change which preview pictures are taken | `internal/settings/preview.go`, `internal/settings/plan_preview.go`; `internal/war3/picture/picture.go`: `Read` |
| know which files are assets, and what path each gets in the map | `internal/assets/collect.go`: `Collect`; `internal/assets/target.go`: `targetPath` |
| know how an import is planned | `internal/build/steps.go`: `PlanAssets`, which calls `Plan` in `internal/assets/plan.go`; `internal/war3/imp/imp.go` |
| change what `assets:sync` writes, and how it undoes a failed write | `internal/assets/sync.go`: `Sync`; `internal/mapdir/apply.go`: `ApplyInPlace`; `internal/fsx/journal.go` |
| know which files of the source map `assets:sync` may replace or remove | `internal/assets/plan.go`: `Plan`, `ownedUnchanged`, `removals`; the state it is given is read in `internal/assets/state.go`: `ReadState` |
| read or change `.asset-state/<map>.json`, the ownership state | `internal/assets/state.go`: `ReadState`, and `StateFile`, which names the file for a map folder and refuses a link on the way to it; `OwnershipFile` in `internal/build/project.go` is `StateFile` for a project, with `map.folder` read as a build reads it |
| change the report of `assets:paths` | `internal/cli/assetspaths.go`: `assetsPaths`; `internal/assets/report.go`: `ReportModels`, `RenderReports` |
| read a new kind of reference out of a model | `internal/war3/model/mdx.go`: `ReadMDX`; `internal/war3/model/mdl.go`: `ReadMDL` |
| know how a library is downloaded and locked | `internal/library/sync.go`: `Sync`; `internal/library/github.go`: `syncGitHub`; `internal/library/lock.go` |
| know how a local library is copied | `internal/library/local.go`: `syncLocal` |
| add a key to `moonwell-library.json` | `internal/library/described.go`: `parseFile` |
| change the editor's declarations | `internal/editor/refresh.go`: `RefreshTypes`; the text of each file in `internal/editor/declarations.go` |
| change what `setup` adds to `.luarc.json` and `.gitignore` | `internal/editor/scaffold.go`: `AddFiles`, `MergeLuarc` |

### The foundations

| I want to | Open |
| --- | --- |
| change how an error is printed | `internal/diag/diag.go`: `Format`, `FormatProblem` |
| change a "did you mean" hint | `internal/diag/suggest.go`: `ClosestNames`, `EditDistance` |
| know why a link or a file name is refused | `internal/fsx/paths.go`: `SafeJoinNoSymlinks`, `CleanRelPath` |
| know how a map folder is scanned, and how a file is found in any letter case | `internal/mapdir/folder.go`: `Open`, `Key`, `Read`; `internal/mapdir/scan.go` |
| know how planned changes are laid over a map | `internal/mapdir/view.go`: `WithChanges`, `ResolveNewPath` |
| change where downloads are cached | `internal/env/cache.go`: `DefaultCacheDir` |
| know what may import what, or add a package | `layout_test.go`: the lists at the top, and `allowed` |

### The generator and the tests

| I want to | Open |
| --- | --- |
| add a mode to the generator | `tools/gen/main.go`: the table `modes` |
| change which files of an export are read, or a column's name | `tools/gen/export.go`: `readExport`, `columnsRead` |
| change how a field gets its friendly name | `tools/gen/names.go`: `assignNames`, `camelCase` |
| pin a name, or acknowledge a field the game dropped | `tools/metadata/overrides.json`; it is read by `readOverrides` in `tools/gen/names.go` |
| add a function or a global that only the game's Lua has | `tools/natives/lua-extras.json`; it is read by `readExtras` in `tools/gen/extras.go` |
| change how a standard object gets its category | `tools/gen/bases.go`: `categoryOfUnit` |
| add a project to the recorded builds | `internal/build/seeds_test.go`: `seeds` |
| add a recorded command line | `internal/cli/seeds_test.go`: `recordedRuns` |
| know how a recording is compared and written | `internal/testkit/recorded.go`: `Recorded` |
| give a test an outside world of its own | `internal/testkit/env.go`: `Env` |
| change how a release is built | `.github/workflows/release.yml` |

## Decisions a reader needs

Each of these is a choice the code depends on. Where the choice is made in one place of the code, the file is
named.

**The order of a build is written once.** `Plan` is the only function that calls the areas in order, and the four
build commands are `Plan` and one more step. So `check` cannot pass a project that `build` then refuses in a planning
step, and one function answers what a build does.

**A planner writes nothing.** It reads a map folder and returns changes; `internal/build` lays them over the map as
a view and writes the view at the end. So a build writes `dist/stage/<map.folder>` only after every step has
succeeded: a refused setting leaves no half-written map. (The compile's cache, `dist/stage/lua`, is written during
the plan.) It is also why `check` is cheap to keep true: it is the same plan, not written.

**A library has a type in each area that takes it.** `manifest.Library` is the library as the manifest writes it,
`library.Synced` where it lies after a sync, `script.Library` its module folder, `assets.Library` the files it ships.
No area imports another, so `internal/build` turns one into the next: `ModuleFolders` in `internal/build/steps.go`,
`shippingLibraries` in `internal/build/project.go`.

**The archive is packed from the view, not read back from the stage.** Both then hold the same bytes without a
second read from disk that could fail. The stage is still written by `build`, though the archive does not need it: a
user reads error positions in `dist/stage/<map.folder>/war3map.lua`. (`pack` in `internal/build/pack.go`.)

**The archive is written beside its place and then renamed.** The place holds a whole archive or what it held
before; a build that is ended midway leaves no cut archive under the map's name. It can leave the file beside the
place, `<map.folder>.tmp`, which the next build that packs replaces. (`writeArchive` in `internal/build/archive.go`.)

**What Moonwell writes, it writes into real folders.** A link, or a Windows junction, on the way to `dist`, the
stage, the lock, the archive, `.moonwell` or `moonwell.lock` is refused. A build removes and replaces what is at
those places, and through a link that would happen outside the project. (`outputAt` in `internal/build/output.go`,
`Inside` in `internal/fsx/paths.go`.)

**The source map is opened in one way, and scanned whole.** Every command that reads the map calls `build.Source`,
and `mapdir.Open` refuses a link, a name Windows cannot hold and two paths that differ only in letter case anywhere
in it. One rule in one place means a command cannot be more lenient than a build. The game matches a map's file
names without regard to letter case, so a `Folder` does too. (`internal/mapdir/folder.go`.)

**Pkl is the one judge of a manifest's shape.** The schema holds every type, range, pattern and default, and Pkl's
error points at the line of the user's file. The Go code decodes what Pkl printed and checks only what Pkl cannot
see: what a setting needs of the map, and two blocks that disagree. A field the structs do not have is passed over,
so that a later patch release of the package may add one. (`Decode` in `internal/manifest/load.go`.)

**`internal/build` keeps one piece of state between calls.** `internal/build/lock.go` keeps the list of build locks
this process holds. A second Ctrl+C ends the program from outside the command that is running. The function that
handles it, `leaveAtOnce` in `internal/cli/cli.go`, knows no project and has no release function to call, and it
must still leave no lock behind: the next build would take a lock that stays for a build that runs. The
only other values a package keeps are the three files of `data/`, each parsed once when it is first asked for
(`objects.LoadMetadata`, `script.LoadNatives`, `assets.LoadGamePaths`).

**Ctrl+C asks first, and leaves at the second.** The first cancels the command's context: `dev` finishes the check
that is under way, since the check holds the lock. The second gives back the locks and exits with 130 at once.
(`Main` and `leaveAtOnce` in `internal/cli/cli.go`.)

**A pinned program is checked before it is kept or run.** `Ensure` checks the SHA-256 of a download, unpacks it in a
folder beside its place, asks it for its version, and only then moves it into the cache. The `yue` on the PATH is
never the compiler: a project names its version, or its own program with `yue.path`. The `pkl` on the PATH is used
when it is 0.32 or newer. An older one is passed over for the pinned Pkl, with a warning, since a `pkl` command the
user types still runs the old one. (`internal/toolchain/ensure.go`, `internal/toolchain/tools.go`.)

**The compile is two steps, and the editor's files are written early.** The declarations and the macro module are
written before a library is fetched or a compiler downloaded, and the libraries' Lua between compiling and linking.
The editor is most needed exactly when a build fails, so its files must not wait for a build that succeeds.
(`writeGenerated` and `compile` in `internal/build/steps.go`.)

**`dev` looks again instead of waiting for events.** Four times a second it lists the watched folders and compares
each file's size and time with the look before. That needs nothing beyond the standard library and behaves the same
on every system. (`internal/build/watch.go`.)

**Bytes stay bytes.** A build never decodes `war3map.lua`, a Lua module or a compiled module: a byte that is not
UTF-8 is kept as it is. White space is the six characters of ASCII, as Lua and the game's text files have it, and
names sort by their bytes. A byte order mark at the start of a text that is read is dropped; a path in
`war3map.imp` keeps one, because it is written back as it was read. (`internal/fsx/text.go`.)

**One parser leaves its functions by a panic.** `parser` in `internal/war3/lua/functions.go` reads Lua's statements
by recursive descent, where every step would otherwise return an error and every call check one. So `fail` panics
with the error at a token the grammar does not allow, and `ParseFunctions`, which alone makes a `parser`, recovers
that panic and returns the error. It is the only place where a panic is a way out of a function: any other panic is a
bug in Moonwell, and is printed as an internal error.

**An error of `mapdir` has a cause exactly when the system failed.** A folder that cannot be listed has one; a map
whose content is refused has none. `internal/assets` reads the difference to say whether a failure is a library's
fault or the machine's, so every error added to `internal/mapdir` keeps the rule. (`internal/mapdir/folder.go`.)

**An error about the `assets` block names `moonwell.pkl`.** The block is written there. The manifest that is
evaluated is `moonwell.local.pkl` in nearly every project, which does not hold it, and nothing Pkl prints says which
of the two files wrote a value. (`Assets` in `internal/build/project.go`.)

**A preview picture is only what the game was seen to read.** A picture the game cannot read closes the game when
the map is selected in the list. So a TGA or a PNG is decoded and written again in one layout, and only two sizes
are taken. (`internal/war3/picture/picture.go`.)

**`assets:sync` writes only where the map is still as it was read.** It is the one command that writes into the
source map. It replaces and removes only files it wrote before (the ownership state), checks each file again just
before the write, and writes through a journal that puts every file back when a write fails or Ctrl+C arrives.
(`Sync` in `internal/assets/sync.go`.)

**`testkit` imports no area.** The tests of the foundations import `internal/testkit`, and every area imports a
foundation, so an area in `testkit` would be an import cycle. That is why the compiler for tests has a package of its
own, `internal/tooltest`. Both are test-only by the layout test alone: nothing in Go keeps a file of the program
from importing them.

**A recording holds no words of an error.** The recorded tests of `internal/build` and `internal/cli` record what a
failed command left, and which file it named. The words are held by the tests of the step that fails. So a
message can be reworded in one place, with one test.

**The generator is not part of the program.** Nothing below `internal/` or `cmd/` imports it, and its parsers
import nothing, so that a parser can be read alone. It needs the game's files, which are not in the repository; what
it wrote is committed, and embedded when the program is built.

## Before this layout

The source tree was written anew for Moonwell 0.10; the last commit that holds the tree before it is `7486397`.
