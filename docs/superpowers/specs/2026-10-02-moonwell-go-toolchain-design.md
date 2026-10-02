# Moonwell in Go: Replacing Deno (Moonwell 0.8.0) — Design

- **Date:** 2026-10-02
- **Status:** Approved by the maintainer on 2026-10-02. Plan 5a
  (`docs/superpowers/plans/2026-10-02-moonwell-go-foundations.md`) amends the package table of §3.1 and the key order
  of §4.4.
- **Builds on:** `docs/superpowers/research/2026-10-01-replacing-deno.md` (the reasons, what Deno does for the project,
  the paths). This spec takes its Path 6 and replaces its §9 lean.
- **Roadmap:** phase 5, item 1, and the backlog entry "Replace Deno".
- **Target:** Moonwell 0.8.0: one `moonwell` executable and the Pkl package `moonwell@0.8.0`, in one GitHub release.
  Nothing is published to JSR again.

## 1. Summary and the decisions

The CLI is rewritten in Go and shipped as one executable. Deno, TypeScript, JSR and every `deno.json` leave the
project: the CLI, its generators, its tests, the template, CI, and the tools of the three sibling repositories.

A map author installs `moonwell` with one line and runs `moonwell build` where they ran `deno task build`. Nothing else
changes for them: the manifest, the Pkl package, the map formats, the libraries and the editor support are untouched.

The maintainer chose (2026-10-02):

| Question | Choice |
| --- | --- |
| Language | Go. The tooling need not be in a gameplay language, since map authors only run it; and Go is easier for the maintainer to read than Rust. |
| Distribution | One `moonwell` on the PATH. The project's Pkl package version is the pin; a mismatch is refused. |
| Installing | An install script per shell, served from the GitHub release. |
| Tests | Every test comes along, as Go tests. |
| Dependencies | The Go standard library only. No third-party modules. |
| Releases | Built by CI from the tag. |
| Shape of the port | A redesign in idiomatic Go, not a line-by-line translation. |

The last choice sets the method. A translation could be checked function by function against its original. A redesign
cannot, so this spec fixes a contract the redesign may not cross (§2) and checks it from outside (§6).

## 2. The contract

What stays the same between `@moonwell/cli@0.7.0` and `moonwell` 0.8.0, for the same project:

- **The command line.** The same commands (`init`, `setup`, `build`, `test`, `dev`, `check`, `assets:check`,
  `assets:sync`, `assets:paths`, `settings:check`, `objects:eval`, `objects:check`), the same flags (`--entry`,
  `--minify`, `--link`, `-h`/`--help`, `-v`/`--version`), accepted anywhere on the line as today, and the same exit
  codes (0, 1, and 130 after Ctrl+C).
- **Every file a command writes, byte for byte:** the staged map folder under `dist/stage/`, what `assets:sync` writes
  into the source map and `.asset-state/`, `moonwell.lock`, everything under `.moonwell/`, `src/generated/objects.yue`,
  and the files `init` and `setup` create.
- **Every printed line, word for word,** including error messages and hints, except where the text names Deno
  (`deno task setup` becomes `moonwell setup`, and so on) and the version line.
- **Where output goes:** logs to stderr and `dist/moonwell.log`, the JSON of `objects:eval` to stdout.

Three things are outside the contract, on purpose:

- **The packed `.w3x` is not byte-identical.** Its sectors are zlib streams, and Go's compressor writes different,
  equally valid bytes than Deno's. The archive's header, its tables and the content, order and names of its files are
  the same. That is what is compared (§6.2), and the packed map is played in the game before release (§11).
- **Caches under `dist/`** (`dist/stage/lua/.hashes.json`, `.globals.json`) may change format. A cache of the old
  format is treated as absent.
- **The generated project** no longer has a `deno.json` (§8.4).

## 3. Layout

One Go module at the repository root. `cli/` disappears.

```
go.mod                 module github.com/mdlsvensson/moonwell; no `require` lines
embed.go  version.go   package moonwell: the embedded files, and Version
cmd/moonwell/          package main: calls internal/cli and exits
internal/…             every package below
tools/gen/             package main: the generators (§9)
runtime/               moonwell.lua, macros.yue (was cli/runtime)
data/                  game-paths.txt, metadata.json, natives.json (was cli/data)
template/  schema/  docs/        unchanged in place
install.ps1  install.sh          the install scripts (§8.2)
```

`go.mod`'s `go` line names the Go release in use when Plan 5a starts; CI reads the version from it.

### 3.1 Packages

| Package | Does | Takes over from |
| --- | --- | --- |
| `internal/cli` | the command table, argument parsing, usage, signals, exit codes, the logger | `main.ts`, `context.ts`, `shared/log.ts`, `commands/*` |
| `internal/diag` | user-facing errors and their rendering (§4.1) | `shared/errors.ts` |
| `internal/fsx` | file helpers: list, replace a folder, write if changed, remove if exists, "in use" errors, `IsWithin`, `SafeJoin`, SHA-256 | `shared/fs.ts`, parts of `assets/paths.ts` |
| `internal/binio` | a little-endian reader and writer (§4.3) | three reader classes and the `DataView` code in seven files |
| `internal/ordered` | a JSON object that keeps its key order (§4.4) | JavaScript's object key order |
| `internal/mapdir` | a map folder: file names without regard to case, a change set, applying it (§4.5) | parts of `settings/plan.ts`, `objectdata/plan.ts`, `assets/plan.ts`, `assets/paths.ts` |
| `internal/luasrc` | reading Lua source: one tokenizer and the scanners on it (§4.6) | `bundle/lexer.ts`, `settings/lua-structure.ts`, `lint/lua-globals.ts`, `editor/map-globals.ts` |
| `internal/mpq` | the MPQ writer, its encryption, the HM3W header, packing a map folder | `mpq/*`, `map/*` |
| `internal/w3i` | reading `war3map.w3i` and editing it byte for byte | `w3i/*` |
| `internal/models` | paths in `.mdx` and `.mdl`, the in-game path list | `models/*` |
| `internal/natives` | the natives data | `natives/*` |
| `internal/project` | evaluating the manifest with `pkl`, the typed project, the version check, the files `init` writes | `project/*`, `project-files.ts` |
| `internal/settings` | validating map settings; patching `w3i`, `war3map.lua` and the text files; the preview picture; the plan | `settings/*`, `w3i/patch.ts` |
| `internal/objects` | metadata, the manifest's objects, resolving, modification files, object ids, the plan | `objectdata/*` |
| `internal/assets` | collecting assets, `war3map.imp`, the plan, syncing with rollback | `assets/*` |
| `internal/library` | `moonwell-library.json`, tag archives, the lock, syncing | `libraries/*` |
| `internal/yue` | pinned versions, installing, the editor copy, compiling, macros, global uses | `yue/*`, `lint/uses.ts` |
| `internal/bundle` | collecting modules, the graph, emitting and injecting the bundle | `bundle/modules.ts`, `graph.ts`, `emit.ts` |
| `internal/lint` | the unknown-global check | `lint/unknown-globals.ts` |
| `internal/editor` | declaration files, scaffolding, the library view | `editor/*` |
| `internal/pipeline` | what `build`, `test`, `check` and `dev` share: compile, stage, pack | `pipeline.ts` |
| `internal/watch` | a polling file watcher (§4.9) | `Deno.watchFs` |

Packages depend downward only: `cli` on `pipeline` and the areas; the areas on the shared packages; `diag`, `fsx`,
`binio` and `ordered` on nothing of ours.

### 3.2 What is gone, with nothing in its place

- `cli/src/embedded/*`, the part of `tools/gen.ts` that wrote it, and its freshness test: Go embeds files itself.
- `yue/unzip.ts`: the standard library reads zip archives and their comments.
- `shared/compression.ts`: the standard library has zlib, raw deflate and gzip.
- `projectDenoJson` and `PROJECT_TASKS`.
- `cli/deno.json`, the root `deno.json` and `deno.lock`, `template/deno.json`.

## 4. The shared designs

### 4.1 Errors

Go functions return errors; nothing is thrown.

```go
package diag

// Error is an expected, user-facing failure.
type Error struct {
    Msg    string
    File   string // "" when the failure has no file
    Line   int    // 1-based; 0 when unknown
    Column int
    Hint   string
    Cause  error
}

// Problems is several failures found at once, each printed on its own line.
type Problems []Problem

func Format(err error) string // what the terminal and the log show
```

- `Format` prints an `*Error` or `Problems` exactly as `formatError` does today, including `and N more` after 20.
- Any other error is an internal error: `internal error: <detail>` and the "please report" line. `cli` also recovers a
  panic and prints it the same way, with the stack.
- So the rule of today holds: a user's mistake must reach the terminal as a `*diag.Error`, never as a bare error.
  Operating system errors are wrapped where they happen, with the file and a hint, as today.
- `ObjectDataError` was a subclass with no behaviour of its own; object problems are `Problems`.

### 4.2 The environment and cancellation

```go
// Env is everything a command needs from the outside world; tests substitute parts of it.
type Env struct {
    Root     string
    Log      *Logger
    Run      func(ctx context.Context, command string, args []string, opts RunOptions) (RunResult, error)
    Fetch    func(ctx context.Context, url string) (status int, body []byte, err error)
    Spawn    func(command string, args []string) error // detached: the game
    CacheDir string
    Platform string // "windows-x86_64", "linux-x86_64" or ""
}
```

- Every function that waits (a process, a download, the watcher, a loop over many files) takes a `context.Context`
  first. Ctrl+C cancels it. `AbortSignal` and the global set of held locks are gone.
- The build lock is `Acquire(dir) (release func(), err error)` with a deferred release, so a cancelled command still
  removes its lock on the way out.
- Ctrl+C behaves as today: `dev` stops watching and returns once its running check has finished; `assets:sync` rolls
  back; other commands stop at their next waiting point. A second Ctrl+C exits at once with 130, after removing
  `dist/.lock` if it holds this process's id.
- The game is started detached, so it outlives the CLI. That needs per-OS code (`spawn_windows.go`, `spawn_unix.go`)
  with `syscall` from the standard library.

### 4.3 Binary data

One reader and one writer for the little-endian formats (`w3i`, the modification files, `war3map.imp`, MDX, BLP and TGA
headers, MPQ tables).

- The reader carries a **sticky error**: after a read past the end, every later read returns zero and `Err()` reports
  the first failure with its offset. A parser reads straight through and checks once, where today each parser checks
  each read.
- Each format turns `Err()` into its own `*diag.Error` with today's message.
- 32-bit values are `uint32`, `int32` and `float32`: the `>>> 0` and `Math.fround` care of today has no counterpart.

### 4.4 Key order

JavaScript keeps the key order of a parsed JSON object. Go's maps do not, and the manifest's order reaches the output
in more than twenty places: the players and forces of `w3i` and `war3map.lua`, the sections and keys of
`war3mapMisc.txt`, an object's fields, the `assets.paths` mapping.

- `ordered.Map[V]` decodes a JSON object into its keys in document order and their values. It ranges in that order.
- Every manifest field that is a Pkl `Mapping` or `Dynamic` decodes into one. Plain Go maps are used only where the
  code sorts the keys itself.

### 4.5 A map folder

Three planners (settings, objects, assets) each compute new content for files of a map folder, and each has its own
change type and its own apply function today. Two of them carry the same lookup of file names without regard to letter
case.

- `mapdir.Names(dir, wanted)` returns the spelling each wanted file has in the folder, and fails for two spellings of
  one file with today's message.
- `mapdir.Change` is a file name, its new bytes, or a removal. `mapdir.Apply` writes a list of them into a folder and
  wraps a failure in the caller's message.
- `mapdir.Key` is today's `pathKey`.
- `assets:sync` writes into the source map and must undo its writes on failure or Ctrl+C. That stays with `assets`,
  as a journal of what it changed, built on the same `Change`.

The planners keep their rule: plan everything, write nothing, then apply.

### 4.6 Lua source

The CLI reads Lua in four places with two tokenizers: to find `require` calls, to find a Lua module's top-level
globals, to find the globals World Editor declares, and to read the structure of `war3map.lua` (its functions and the
calls in them) for the settings patch.

- `luasrc.Tokenize` is the one tokenizer: Lua 5.3's full lexical grammar (long strings and comments, every number
  form). It never fails; a malformed token is a token of its own kind, and the scanner that cares reports it.
- The scanners are functions over tokens: `Requires`, `TopLevelGlobals`, `Functions`, `MapGlobals`.
- The two number patterns that use lookahead today (`1..2` is not `1.` followed by `.2`) are written by hand: Go's
  regular expressions have no lookahead.
- If the two tokenizers of today disagree on some input in a way their tests pin, the tokenizer takes an option for
  it, and Plan 5a says which.

### 4.7 The manifest

`pkl eval --format json` prints the manifest; today about 400 lines of hand-written checks turn that JSON into typed
values.

- The JSON decodes into Go structs. Optional fields are pointers. Unknown fields are ignored, as today.
- A type mismatch is reported as today: `<path> must be a string.` with the schema hint, the path written as today
  (`libraries["example"].tag`).
- Checks on values (ranges, names of races and controllers, conflicts between typed and raw gameplay constants) stay
  as explicit code.
- **A criterion for Plan 5b:** the ported tests pin these messages. If struct decoding cannot produce a pinned message
  without contortions, that part is written as a small cursor over an ordered JSON tree (`v.Field("map").String()`),
  which tracks its own path. The plan decides per type and says so.

### 4.8 Embedded files and commands

- `embed.go` embeds `template/`, `runtime/` and `data/` with `go:embed` (`all:template`, so that dot-files come
  along). The gzip-and-base64 step is gone; the data adds about 4 MB to the executable.
- `init` skips what it skips today: `PklProject`, `PklProject.deps.json`, `moonwell.local.pkl`, and what commands
  write inside a project (`dist/`, `.moonwell/`, `src/**/*.lua`). A test pins the list of files `init` writes, so a
  stray file in `template/` still fails a test.
- A command is an entry in a table: its name, its argument summary, its help line, and its function. The usage text
  is rendered from the table and equals today's, with the first line's version.
- Arguments are parsed by a small parser of our own (flags anywhere, `--entry f` and `--entry=f`): the standard `flag`
  package stops at the first positional argument, and `moonwell init my-map --link` must keep working.

### 4.9 Concurrency and watching

- The code is sequential. There are two exceptions: `yue` runs in parallel, at most eight at a time, as today; and
  `dev` has a watcher beside its check cycle.
- `watch` polls. Every 250 ms it lists the watched folders and compares each file's name, size and modification time
  with the last pass; a difference that the root's filter accepts is a change. `dev` debounces changes by 150 ms as
  today.
- `dev` watches what it watches today, decided when it starts: `src/`, the project folder itself, `assets/`,
  `objects/` and `lua/` when they exist, each local library's folders and its `moonwell-library.json`, and the
  preview picture's folder for that one file. `.moonwell/` is never watched.

## 5. Where JavaScript and Go differ silently

Checked at every review. Each is a way to pass a quick test and still write different bytes.

- **Key order** (§4.4). Also for JSON that is written: `moonwell.lock`, `.luarc.json`, `.asset-state/`, the output of
  `objects:eval`. A Go struct marshals in field order and a Go map in sorted order; each written file keeps today's
  order.
- **`JSON.stringify` and `json.Marshal`.** Go escapes `<`, `>` and `&` unless told not to, and escapes U+2028 and
  U+2029. Numbers render differently at the edges (`1e21`, negative zero). The bundle uses `JSON.stringify` to quote
  Lua strings (module names and paths); that becomes one function with the same output.
- **Strings.** JavaScript strings are UTF-16: `length`, slicing and the default sort order count 16-bit units, Go
  counts bytes. They agree for ASCII and differ above it. `toUpperCase` and `toLowerCase` apply full Unicode case
  mapping (`ß` becomes `SS`); Go's do not. Each use is ported with its meaning, and a non-ASCII test where a path or
  name can be non-ASCII.
- **`localeCompare`** orders by the machine's language. Its one use (the order of compile failures) becomes byte
  order, which is what it gave for file paths.
- **Text decoding.** `TextDecoder` replaces bad bytes or, when fatal, throws; a Go string holds any bytes. Where
  today's code demands valid UTF-8, the Go code checks it. A byte order mark is kept or dropped where today's is.
- **Regular expressions.** No lookahead or lookbehind in Go. `\s` matches Unicode spaces in JavaScript and ASCII
  spaces in Go; `\d` and `\w` are ASCII in both. `$` and `^` need the `m` flag in both, but `.` and `\r` need a look.
- **Numbers.** Every JavaScript number is a 64-bit float. Number-to-text (`String(0.1)`, large and small magnitudes)
  is written to match where it reaches a file. Integer checks (`Number.isInteger`) become checks on a decoded
  `float64`, since the manifest's JSON does not say which numbers are integers.
- **Arguments.** §4.8.
- **Files.** "Not found" and "already exists" are `errors.Is` checks. "In use by another program" is `EBUSY` in Deno
  and a sharing violation in Go on Windows. Creating a file only if it is new is `O_EXCL`. The order of a folder
  listing is sorted wherever it reaches output.
- **Processes.** Go refuses to run a program found in the current folder rather than on the PATH; `pkl` and `yue` are
  found on the PATH or by an absolute path, so this only needs a clear error. A missing program reports
  `Cannot run '<command>': command not found.` as today.
- **Compression.** §2: the bytes differ, the content does not.

## 6. Verification

### 6.1 Tests

Today: about 580 test cases in 81 files. Most are unit tests that need no tool; about 30 need `yue`, 28 need `pkl`, 34
run end to end and 2 use the network.

- **Every case gets a Go counterpart.** Each plan has an inventory table: TypeScript test file, the Go test file that
  takes its cases, and the count on each side. A plan is complete when its table is.
- **A case is dropped only when its subject is gone:** the embedded-file freshness tests (`embedded.test.ts`), the
  zip reader's own tests (`unzip.test.ts`; its use, reading a tag archive and its comment, is tested in `library`),
  the compression wrappers in `shared.test.ts`, the `deno.json` rendering in `project-files.test.ts`, and the
  `cli/deno.json` half of `versions-consistent.test.ts`. Each plan names the cases it drops.
- **Form:** table-driven tests beside the code, fixtures in each package's `testdata/`. The test helpers (the MPQ
  reader, the synthetic `w3i`, the MDX builder, the picture and zip builders, the recording logger) become a test-only
  package, `internal/testkit`.
- **One command:** `go test ./...`. A test that needs `yue` or `pkl` asks `testkit` for it and is skipped when it is
  missing; with `MOONWELL_REQUIRE_TOOLS=1`, which CI sets, a missing tool fails the test. Network tests run with
  `MOONWELL_NETWORK_TESTS=1`, as today. `MOONWELL_TEST_YUE` still names a compiler to use.
- **A test for the rule:** `go.mod` has no `require`, and no file imports `"C"`.

### 6.2 The conformance suite

While both CLIs are in the repository, a Go test runs each of them on its own copy of the same project and compares:

- the exit code and the printed lines, after replacing the temporary folder's path and the texts §2 exempts (a
  `deno task <name>` in a message is `moonwell <name>`);
- every file under `dist/stage/<map>`, `.moonwell/`, `src/generated/`, `moonwell.lock`, and for `assets:sync` the
  source map and `.asset-state/`;
- the packed map, unpacked: the list of files, their order and their contents, and the header before the archive.

The projects:

| Project | Commands |
| --- | --- |
| a fresh `init --link` (the template; the `deno.json` Deno's `init` writes is left out of the comparison) | `init`, `setup`, `check`, `build`, `build --minify`, `objects:eval`, `objects:check`, `settings:check`, `assets:check`, `assets:paths` |
| the gate map `../wrappers-gate` with both libraries by local path, when present | `check`, `build`, `build --minify` |
| a project on the `map-settings-v39` fixture with every setting set | `settings:check`, `build` |
| a project on the `objects-v3-names` fixture with objects of every category | `objects:check`, `objects:eval`, `build` |
| a project with assets on the `imports-we3` fixture | `assets:check`, `assets:sync`, `build` |
| a project with a preview picture, once as TGA and once as BLP | `settings:check`, `build` |
| a project with `lua/` modules and the example library by tag (network) | `check`, `build` |
| failing projects: a syntax error, an unknown global, a stale `objects.yue`, a missing source map, a wrong Pkl package version | `check` or `build` |

`test` (it starts the game) and `dev` (it never ends) are compared through their parts: `test` stages what `build`
stages, and `dev`'s cycle is `check`.

The suite runs when `MOONWELL_CONFORMANCE=1` and Deno is installed, locally and in CI. It is deleted with the
TypeScript in Plan 5e. Its last green run, with the commit, goes into the changelog entry of 0.8.0.

## 7. Continuous integration

- **During Plans 5a to 5d,** `ci.yml` runs the Go checks beside the Deno ones, on Ubuntu and Windows: `go vet ./...`,
  `gofmt -l .` (must print nothing), `go test ./...` with `MOONWELL_REQUIRE_TOOLS=1` and `MOONWELL_NETWORK_TESTS=1`,
  and from Plan 5d the conformance suite.
- **From Plan 5e,** only the Go checks.
- The Windows runner still points `TMP` and `TEMP` at the checkout's drive: tests still create linked projects.

## 8. Distribution

### 8.1 A release

The tag stays `moonwell@<version>`: the Pkl package's download address is built from it. A release holds:

- `moonwell-windows-amd64.exe` and `moonwell-linux-amd64`: plain files, built with `CGO_ENABLED=0`, `-trimpath` and
  stripped. These are the two platforms `yue` is pinned for and CI tests. macOS is not built until `yue` is pinned
  there.
- `checksums.txt`: the SHA-256 of both.
- `install.ps1` and `install.sh`, with the release's version written into them.
- `moonwell@<version>.zip` and the metadata file `moonwell@<version>`: the Pkl package, as today.

`.github/workflows/release.yml` runs on a pushed tag `moonwell@*`:

1. Fail unless the tag's version equals `Version` and `schema/PklProject`'s version.
2. Run every check of §7.
3. Build the two executables, package the schema with `pkl project package`, write the checksums and the two scripts.
4. Create the GitHub release with those files.
5. In a second job, on Windows and Ubuntu: run the install line for this version, then `moonwell init my-map` and
   `moonwell build` in it. This is today's "check from JSR" step, and it runs where Pkl's downloads work.

So a release is: bump the version, re-resolve the template's Pkl dependencies, commit, push, push the tag, and read
the workflow's result. Step 4 of today's publishing (`deno publish` and its browser sign-in) is gone.

### 8.2 The install scripts

```powershell
irm https://github.com/mdlsvensson/moonwell/releases/latest/download/install.ps1 | iex
```

```sh
curl -fsSL https://github.com/mdlsvensson/moonwell/releases/latest/download/install.sh | sh
```

- `releases/download/moonwell@<version>/install.ps1` installs that version: each release's script installs its own.
- The script downloads the executable for the machine and `checksums.txt`, compares the SHA-256, and refuses a
  mismatch.
- Windows: it writes `%LOCALAPPDATA%\moonwell\bin\moonwell.exe` (`MOONWELL_CACHE\bin` when that is set), the folder
  `setup` already puts `yue` in, and adds the folder to the user's PATH when it is missing. One PATH entry serves
  both programs.
- Linux: it writes `~/.local/bin/moonwell` and says so when that folder is not on the PATH.
- Running the line again upgrades. There is no self-update command.
- **To check in Plan 5e, on the maintainer's machine:** that an executable downloaded this way starts without a
  SmartScreen warning (a file fetched by PowerShell carries no browser mark, as far as known). If it does warn, the
  README says what the warning is and how to proceed, and signing becomes a backlog entry.

### 8.3 The version check

Unchanged in substance: the `moonwell` package resolved in `PklProject.deps.json` must have the executable's major and
minor version. The hint names both ways out:

```
error: PklProject › Pkl package moonwell@0.8.2 does not match Moonwell CLI 0.9.0.
hint: Install Moonwell 0.8.2 (irm https://github.com/mdlsvensson/moonwell/releases/download/moonwell@0.8.2/install.ps1 | iex), or use moonwell@0.9.x in PklProject and run `pkl project resolve`.
```

The message is today's; the hint is new, since today's names `deno.json`. It stays one line, like every hint. The
install line is the one for the machine's shell. For a package older than 0.8.0, which has no executable, the hint
gives only the second way.

### 8.4 Projects

- `init` writes no `deno.json`, and ends with `cd <dir> && moonwell build`.
- `moonwell.local.pkl`'s first comment names `moonwell setup`.
- **Upgrading from 0.7,** a README section: install `moonwell`; set `moonwell@0.8.0` in `PklProject`; run
  `pkl project resolve`; delete `deno.json` and `deno.lock`.
- `setup` prints one line when the project has a `deno.json` that names `@moonwell/cli` or `cli/src/main.ts`: the file
  is no longer used and can be deleted. It never deletes it.

### 8.5 Working on Moonwell

- `go run ./cmd/moonwell <command>` runs the CLI from the checkout; in `template/`, `go run ../cmd/moonwell build`.
- `template/` stays a linked project: its `PklProject` imports `../schema`.
- `init --link <dir>` stays a flag. It finds the checkout by walking up from the working directory to the folder
  whose `go.mod` names this module, and fails elsewhere with today's message. The project it creates imports the
  checkout's `schema/`; it is built with whatever `moonwell` is run in it.
- Tests pass the checkout's folder directly.

## 9. The generators

`tools/gen` is one program with four subcommands, replacing the four TypeScript generators:

| Command | Writes | From |
| --- | --- | --- |
| `go run ./tools/gen` | `schema/generated/*.pkl`, removing files it does not write | `data/metadata.json` |
| `go run ./tools/gen natives <folder> <version>` | `data/natives.json` | `common.j`, `blizzard.j`, `tools/natives/lua-extras.json` |
| `go run ./tools/gen metadata <folder>` | `data/metadata.json` | the game's SLK and text files, `tools/metadata/overrides.json` |
| `go run ./tools/gen game-paths <file> <version>` | `data/game-paths.txt` | a list of the game's file names |

- Their output is byte-identical to today's. For `schema/generated/` that is checked by a test (the folder is fresh)
  and by `git diff` after the first run. For the three data files it is checked by regenerating from the maintainer's
  game exports, when they are at hand, and seeing no change.
- The JASS, SLK and text parsers move to packages under `tools/gen/`, with their tests.

## 10. The other repositories

Deno is in three more places. None of these changes library code, so none needs a tag.

- **moonwell-wrappers:** `tools/test.ts`, `run.ts`, `check-lua.ts` and `integration.ts` (357 lines) become Lua run by
  `yue -e`, modelled on moonwell-systems' `tests/run.lua`, `tools/check.lua` and `tools/integration.lua`. `deno.json`
  and `deno.lock` go. Its `AGENTS.md` and `CONTRIBUTING.md` name the new commands.
- **moonwell-systems:** `tools/integration.lua` runs `deno run -A ../moonwell/cli/src/main.ts`; it runs `moonwell`,
  or the program `MOONWELL` names. Its documents follow.
- **wrappers-gate:** `gate.ts` and `preview-probe.ts` (391 lines) become Lua the same way; `deno task gate <run>`
  becomes `yue -e gate.lua <run>`. Its `deno.json` and `deno.lock` go.

The integration tools find `moonwell` on the PATH, or use `MOONWELL` (for example `go run` from a checkout).

## 11. The rules and the documents

**Hard rules, in `AGENTS.md` and `CONTRIBUTING.md`, replacing the Node.js and style rules:**

- **Standard library only.** No third-party module and no cgo: `go.mod` has no `require`, and a test keeps it so.
- **Errors.** Expected failures return a `*diag.Error` with a file and a hint. Any other error, and any panic, is
  reported as an internal "please report" error, so a user's mistake must never reach it.
- **Style.** `gofmt` and `go vet` are clean. Markdown is wrapped at 120 by hand, as `docs/` is today.
- **Generated files.** After changing `data/metadata.json`, run `go run ./tools/gen`; a test fails when
  `schema/generated/` is stale. `template/`, `runtime/` and `data/` need no step: they are embedded at build time.
  Nothing stray may be left in `template/`.

**Checks:**

```
go vet ./...
gofmt -l .        # prints nothing
go test ./...     # yue- and pkl-backed tests run when the tools are present; MOONWELL_NETWORK_TESTS=1 adds the network
```

**Documents rewritten in Plan 5e:** `README.md` (installing, every `deno task` line, the upgrade section),
`CONTRIBUTING.md` (layout, checks, the release gate's commands, publishing), `AGENTS.md` (state, rules, pitfalls: the
Deno quirks go, the list of §5 stays as a pitfall for later changes), `CHANGELOG.md`, and the roadmap's item 5.1.

**The release gate of 0.8.0.** A staged map folder equal to 0.7.0's is a folder that passed its gates. What is new to
the game and the machine is run by hand:

- Step 1 (every check), 2 (the path list), 3 (`setup`, then `test`: the game starts, stays open, the Captain changes
  colour), 4 (the error position), 5 (`build --minify`, play the packed map), 6 (the packed map opens in World
  Editor).
- A new step: on a PATH without `moonwell`, run the install line from the release, then `moonwell init` and
  `moonwell build`.
- Steps 7 to 13 are recorded as covered by the conformance suite, with its commit.

## 12. The plans

One spec, six plans. Each is written when the one before it is implemented, since it builds on real code. The
TypeScript stays in place, working and tested, until Plan 5e.

| Plan | Contents | Done when |
| --- | --- | --- |
| 5a | The module and layout; Go in CI beside Deno; `diag`, `fsx`, `binio`, `ordered`, `mapdir`, `luasrc`; `mpq`, `w3i`, `models`, `natives`; embedding; `testkit` | their inventory is complete and CI is green |
| 5b | `project`, `settings`, `objects`, `assets` | as above |
| 5c | `library`, `yue`, `bundle`, `lint`, `editor`, `pipeline` | as above |
| 5d | `cli` and every command, `watch` and `dev`, the end-to-end tests, the conformance suite | the conformance suite is green on both systems |
| 5e | The generators; the TypeScript, `cli/` and every `deno.json` deleted; template and documents; install scripts; the release workflow; 0.8.0 and its gate | 0.8.0 is released and installed from its release |
| 5f | moonwell-wrappers, moonwell-systems, wrappers-gate | no Deno in any of them; their checks pass with `moonwell` 0.8.0 |

Until Plan 5e the Go code carries version 0.7.0, like the TypeScript beside it, so both CLIs accept the same projects
and resolve the same Pkl package. Plan 5e sets 0.8.0.

Before Plan 5a: Go is installed on the maintainer's machine.

## 13. Not in this work

- **Teal and Fennel** (roadmap 5.2). With the standard library only, their compilers (`tl.lua`, `fennel.lua`) run on
  the Lua 5.4 inside the pinned `yue` (`yue -e`), not on a Lua embedded in the CLI.
- **PNG previews** (roadmap 5.5) get smaller: the standard library decodes PNG.
- **macOS builds,** a self-update command, package managers (Scoop, winget, Homebrew), and signing the Windows
  executable.
- **Downloading a pinned `pkl`.** Pkl stays a program the user installs.
- **Features.** The CLI gains none in 0.8.0.
