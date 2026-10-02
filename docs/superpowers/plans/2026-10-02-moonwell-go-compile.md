# Moonwell in Go, Plan 5c: Libraries, Compiling and the Pipeline — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Go packages that fetch libraries, drive `yue`, bundle the modules, check for unknown globals, write the
editor's files and stage a map: `logging`, `library`, `bundle`, `yue`, `lint`, `editor` and `pipeline`, each with the
tests its TypeScript counterpart has.

**Architecture:** As Plans 5a and 5b. `pipeline.Env` is what a command gets from the outside world (the root, the
logger, the process runner, the downloader, the cache folder); `pipeline` composes the planners of Plan 5b with the
packages of this plan into the three things commands share: compile the project, plan its objects, prepare the staged
map.

**Tech Stack:** Go 1.27, standard library only (`archive/zip` for tag archives, `net/http` for downloads).

**Spec:** `docs/superpowers/specs/2026-10-02-moonwell-go-toolchain-design.md`. Plans 5a and 5b give the packages this
plan builds on.

**How to read the tasks.** As before: each task's specification is the TypeScript it names and the Go interface given
here; a task is done when its inventory rows are ported and pass. Test-first.

## Global Constraints

- Standard library only. `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass before every commit.
- Messages, hints and written bytes equal the TypeScript's, except the deviations a task lists.
- Tests that need `yue` ask `testkit.NeedYue(t)`: `MOONWELL_TEST_YUE`, else the pinned compiler from the user cache
  (downloaded once); skipped when neither works, failed when `MOONWELL_REQUIRE_TOOLS=1`. Network tests ask
  `testkit.NeedNetwork(t)`.
- Go is not on the PATH of the agent's shell: `export PATH="$PATH:/c/Program Files/Go/bin"`.

## Decisions

- **One package for modules.** `bundle/modules.ts` and `yue/compile.ts` import each other's types. In Go, `bundle`
  owns `SourceModule`, `CompiledModule`, collecting, the loader, the graph and the bundle text; `yue` imports
  `bundle`.
- **`pipeline.Env`, not `cli.Env`.** The spec put the environment in `cli`; `pipeline` needs it and `cli` imports
  `pipeline`, so it lives here.
- **`lint/uses.ts` goes to `yue`** (it runs `yue -g`), as the spec's table says; `lint` keeps the check.
- **Downloads** are `func(ctx, url) (status int, body []byte, err error)`: what both callers (libraries, the compiler)
  use of a response.
- **Caches** (`.hashes.json`, `.globals.json`) keep their names and shapes, written with `ordered.Stringify`, so a
  project's existing caches stay valid and nothing recompiles after the switch.

## Amendments made while implementing

- **`internal/yuetest`, not `testkit.NeedYue`.** `testkit` is imported by the tests of packages that `yue` depends on
  (`luasrc`), so a helper that imports `yue` cannot live there. Tests call `yuetest.Need(t)`.
- **`text.LocaleCompare`.** The TypeScript sorts the failed files of a compile with `localeCompare`, which is not
  code-unit order (`_` before digits before letters, `a` before `B`). `text.LocaleCompare` reproduces it for ASCII
  (compared with Deno on 3,999 pairs); characters past ASCII sort after the letters, which is a deviation.
- **`yue.Output` has `Texts` and `Load`, no `sources`.** The TypeScript's `sources` (texts keyed by path under src/)
  had no caller; `Texts["src/" + path]` is the same text.
- **`yue.OnPath`** replaces `checkYueOnPath`'s three-way result with `(version, found, err)`.
- **`library.HTTPFetch`** is the real downloader; `pipeline.NewEnv` leaves `Spawn` for Plan 5d, which ports
  `launch.ts`, and the two `launchGame` cases of `pipeline.test.ts` move there with it.
- **Test files** are fewer than the inventory names: `library_test.go` and `sync_test.go` hold the six library files'
  cases, `bundle_test.go` the three bundle files', `editor_test.go` the five editor files', and `install_test.go`,
  `uses_test.go` and `yue_test.go` the rest. Every case of the inventory is ported.
- **Deviations:** an unreadable `.hashes.json` or `.globals.json` (wrong shapes inside valid JSON) is treated as
  absent instead of failing with an internal error; "Known versions" are listed sorted; the hint for a compiler copy
  that cannot be replaced says `moonwell setup`.

## File structure

```
internal/logging/     logging.go           the logger: a sink and dist/moonwell.log
internal/library/     manifest.go archive.go download.go lock.go sync.go
internal/bundle/      modules.go graph.go emit.go
internal/yue/         versions.go install.go bin.go macros.go compile.go uses.go
internal/lint/        unknown.go
internal/editor/      declarations.go mapdecl.go refresh.go scaffold.go libraryview.go
internal/pipeline/    env.go pipeline.go lock.go
internal/testkit/     logger.go zip.go (+ NeedYue in tools.go)
```

## Test inventory

| TypeScript test file | Cases | Go test file |
| --- | --- | --- |
| `shared.test.ts` (logger, lock) | 1 + 3 | `internal/logging/logging_test.go`, `internal/pipeline/lock_test.go` |
| `library-file.test.ts` | 5 | `internal/library/manifest_test.go` |
| `library-archive.test.ts` | 5 | `internal/library/archive_test.go` |
| `library-lock.test.ts` | 3 | `internal/library/lock_test.go` |
| `library-sync.test.ts` | 16 | `internal/library/sync_test.go` |
| `library-assets-sync.test.ts` | 9 | `internal/library/assets_test.go` |
| `unzip.test.ts` | 2 | dropped: `archive/zip` reads the archives; its two behaviours (stored and deflated entries, the comment) are covered by `archive_test.go` |
| `modules.test.ts` | 13 | `internal/bundle/modules_test.go` |
| `graph.test.ts` | 6 | `internal/bundle/graph_test.go` |
| `emit.test.ts` | 5 | `internal/bundle/emit_test.go` |
| `install.test.ts` | 6 | `internal/yue/install_test.go` |
| `yue-bin.test.ts` | 7 | `internal/yue/bin_test.go` |
| `lint-uses.test.ts` | 7 | `internal/yue/uses_test.go` |
| `yue/compile.test.ts`, `macros.test.ts`, `uses.test.ts` | 10 + 4 + 2 | `internal/yue/yue_test.go` (need `yue`) |
| `lint-unknown-globals.test.ts` | 8 | `internal/lint/unknown_test.go` |
| `editor-declarations.test.ts` | 6 | `internal/editor/declarations_test.go` |
| `editor-map-globals.test.ts` | 2 of 4 | `internal/editor/declarations_test.go` (2 were ported in Plan 5a) |
| `editor-refresh.test.ts` | 5 | `internal/editor/refresh_test.go` |
| `editor-scaffold.test.ts` | 11 | `internal/editor/scaffold_test.go` |
| `library-view.test.ts` | 3 | `internal/editor/libraryview_test.go` |
| `pipeline.test.ts` | 9 | `internal/pipeline/pipeline_test.go` |
| `network/libraries.test.ts` | 2 | `internal/library/network_test.go` (needs the network) |
| `yue/runtime.test.ts`, `objects.test.ts`, `settings.test.ts`, `e2e/*` | 13 + 34 | Plan 5d: they run commands or whole builds |

---

### Task 1: `logging` and `library`

**Reference:** `cli/src/shared/log.ts`, `cli/src/libraries/*`.

**Produces:**

```go
package logging

type Logger struct{ /* a sink and an optional log file */ }
func New(write func(line string), file string) *Logger // file "": no log file
func (l *Logger) Info(message string)
func (l *Logger) Warn(message string)  // the sink gets "warning: " + message
func (l *Logger) Error(message string)
```

```go
package library

const File = "moonwell-library.json"
type Described struct{ Dir, Assets *string }
func ParseFile(key string, data []byte, present bool, where string) (Described, error)

func ReadGitHubArchive(data []byte) (commit string, files *Files, err error)
func FilesHash(files *Files) string

type Fetch func(ctx context.Context, url string) (status int, body []byte, err error)
func ArchiveURL(github, tag string) string
func DownloadTag(ctx context.Context, key, github, tag, manifest string, fetch Fetch) (commit string, files *Files, err error)

const LockFile = "moonwell.lock"
type LockEntry struct {
    GitHub, Tag, Dir, Commit, Files string
    Assets *string
}
func ReadLock(root string) (map[string]LockEntry, error)
func WriteLock(root string, libraries map[string]LockEntry) error

type Deps struct {
    Fetch Fetch
    Log   *logging.Logger
}
func Sync(ctx context.Context, root string, libraries *ordered.Map[project.Library], manifest string, deps Deps) error
```

`Files` is a set of files by POSIX path that keeps the order they were added in (a JavaScript `Map`).

- [x] The lock file's text is byte for byte the TypeScript's: keys sorted, each entry's fields in the order github,
      tag, dir, commit, files, assets, two-space indentation, a final newline.
- [x] The stamp `.moonwell-library.json` keeps its fields and `layout: 2`, so a project synced by 0.7.0 is not
      downloaded again.
- [x] Commit: `go: logging and library`.

### Task 2: `bundle`

**Reference:** `cli/src/bundle/modules.ts`, `graph.ts`, `emit.ts`.

**Produces:**

```go
type Kind string // "yue" or "lua"
type SourceModule struct {
    Name, Path string
    Kind       Kind
    Source     string // a Lua module's text
    Library    string // the library key; "" for the project's own
}
type CompiledModule struct {
    Name, SourcePath, Source string
    Kind                     Kind // "lua" for a Lua module, which the bundle never minifies
}
type ModuleRoot struct {
    Dir      string
    Kind     Kind
    Required bool
    Library  string
}
var Builtins = []string{"moonwell"}
var ProjectRoots = []ModuleRoot{{"src", "yue", true, ""}, {"lua", "lua", false, ""}}
func LibraryRoots(keys []string) []ModuleRoot
func CollectModules(root string, roots []ModuleRoot) ([]SourceModule, error)
func Loader(modules []SourceModule, loadCompiled func(SourceModule) (*CompiledModule, error)) func(name string) (*CompiledModule, error)
func ResolveGraph(entry string, load func(name string) (*CompiledModule, error), builtins []string) ([]CompiledModule, error)

type EmitInput struct {
    Runtime   string
    Modules   []CompiledModule
    Entry     string
    FirstLine int
    Minify    bool
}
func Emit(input EmitInput) string
func Inject(script string, bundle func(firstLine int) string, file string) (string, error)
```

- [x] Names and paths in the bundle are quoted with `text.Quote` (`JSON.stringify`).
- [x] Commit: `go: bundle`.

### Task 3: `yue`

**Reference:** `cli/src/yue/*`, `cli/src/lint/uses.ts`.

**Produces:**

```go
const DefaultVersion = "0.34.2"
type Asset struct{ URL, SHA256, Archive, Binary string }
var Known map[string]map[string]Asset // version → platform → asset
func CurrentPlatform() string         // "windows-x86_64", "linux-x86_64" or ""

type InstallDeps struct {
    Fetch     library.Fetch
    Run       proc.RunFunc
    CacheRoot string
    Platform  string
    Known     map[string]map[string]Asset
    Log       *logging.Logger
}
func DefaultCacheRoot() string
func Version(ctx context.Context, binary string, run proc.RunFunc) (string, error)
func Ensure(ctx context.Context, version string, path *string, deps InstallDeps) (string, error)

func InstallBin(binary, cacheRoot string) (path string, copied bool, err error)
func PathCommand(binDir, goos string) string
func ReportEditorTools(ctx context.Context, run proc.RunFunc, log *logging.Logger, version, binDir, goos string) error

const MacrosFile = ".moonwell/yue/moonwell/macros.yue"
type MacroSearch struct{ Path, Hash string }
func Macros(root string) (*MacroSearch, error)

type CompileOptions struct {
    Yue, Root   string
    Minify      bool
    Macros      *MacroSearch
    Modules     []bundle.SourceModule
    Run         proc.RunFunc
    Concurrency int
}
type Output struct {
    OutDir string
    Hashes map[string]string // by path under src/
    Texts  map[string]string // by project path
    // …
}
func (o *Output) LoadModule(module bundle.SourceModule) (*bundle.CompiledModule, error)
func Compile(ctx context.Context, options CompileOptions) (*Output, error)
func CompileError(file, output string) *diag.Error

type GlobalUse struct {
    Name         string
    Line, Column int
}
const UsesCache = "dist/stage/lua/.globals.json"
func ParseGlobalUses(output, file string) ([]GlobalUse, error)
func ListGlobalUses(ctx context.Context, options UsesOptions) (map[string][]GlobalUse, error)
```

- [x] `yue` runs in parallel, eight at a time by default, with a buffered channel as the limit.
- [x] The Windows archive is 7-Zip and is unpacked by Windows' own `tar.exe`, as today.
- [x] Commit: `go: yue`.

### Task 4: `lint` and `editor`

**Reference:** `cli/src/lint/unknown-globals.ts`, `cli/src/editor/*`.

**Produces:**

```go
package lint

const UnknownGlobalHint = "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl."
func DeclaredGlobals(source string) []string
func KnownGlobals(natives *natives.Natives, mapGlobals *luasrc.MapGlobals, declared, extra []string) map[string]bool
func UnknownGlobalProblems(uses map[string][]yue.GlobalUse, known map[string]bool, removed []string) diag.Problems
type Compiled struct {
    Yue         string
    Hashes      map[string]string
    Macros      *yue.MacroSearch
    DeclaredYue []string
    DeclaredLua []string
}
func Check(ctx context.Context, root string, run proc.RunFunc, log *logging.Logger, p *project.Project, compiled Compiled, n *natives.Natives) (diag.Problems, error)
```

```go
package editor

const TypesDir = ".moonwell/types"
const LibraryViewDir = ".moonwell/lua"
func LuaType(jassType string) string
func RenderNatives(n *natives.Natives) string
const RuntimeDeclarations = "…"
func RenderObjects(objects []objects.Resolved) string
func RenderMap(globals *luasrc.MapGlobals, source string) string
func ReadSourceScript(path, label string) (script string, exists bool, err error)
type Inputs struct {
    Objects   []objects.Resolved
    MapFolder string
    Natives   *natives.Natives
}
func Refresh(root string, inputs Inputs) ([]string, error)
var Files = []string{"yueconfig.yue", ".luarc.json", ".vscode/extensions.json"}
var Ignores = []string{".moonwell/", "src/**/*.lua"}
func AddFiles(root string, template []moonwell.TemplateFile) ([]string, error)
func LuarcTemplateEntries(template []moonwell.TemplateFile) (map[string][]string, error)
func MergeLuarc(root string, template []moonwell.TemplateFile) (added []string, merged bool, err error)
func RefreshLibraryView(root string, modules []bundle.SourceModule, loadCompiled func(bundle.SourceModule) (*bundle.CompiledModule, error)) ([]string, error)
```

- [x] `.luarc.json` is rewritten with `ordered.Stringify(…, 2)` and a final newline, keeping the file's key order.
- [x] Commit: `go: lint and editor`.

### Task 5: `pipeline`

**Reference:** `cli/src/pipeline.ts`, `cli/src/context.ts`, `cli/src/shared/lock.ts`.

**Produces:**

```go
type Env struct {
    Root    string
    Log     *logging.Logger
    Run     proc.RunFunc
    Install yue.InstallDeps
    Spawn   func(command string, args []string) error
}
func NewEnv(root string, log *logging.Logger) *Env

type StageOptions struct {
    Entry  string // overrides map.entry when not empty
    Minify *bool  // overrides build.minify when set
}
func EntryModuleName(entryPath string) (string, error)
func SyncLibraries(ctx context.Context, env *Env, p *project.Project) error
func CompileProject(ctx context.Context, env *Env, p *project.Project, options StageOptions) (modules []bundle.CompiledModule, entry string, err error)
func PlanObjects(env *Env, p *project.Project) (*objects.Plan, error)
func PrepareStage(ctx context.Context, env *Env, p *project.Project, options StageOptions) (mapDir string, modules []bundle.CompiledModule, err error)

// AcquireLock takes <distDir>/.lock, which records this process's id; a second build fails fast.
func AcquireLock(distDir string) (release func(), err error)
```

- [x] Commit: `go: pipeline`.

### Task 6: Close the plan

- [x] Every check from a clean tree; `AGENTS.md` and `CHANGELOG.md` say Plan 5c is implemented, with its deviations.
- [x] Commit: `docs: Plan 5c of the Go toolchain is implemented`.
