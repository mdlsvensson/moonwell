# Moonwell in Go, Plan 5b: The Manifest and the Planners — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The Go packages that read a project and plan what a build changes in a map: `proc`, `project`, `settings`,
`objects` and `assets`, each with the tests its TypeScript counterpart has.

**Architecture:** As Plan 5a: packages under `internal/`, the TypeScript untouched. The manifest's JSON is read into
an ordered tree (`ordered.Decode`), validated with the checks the TypeScript has, in its order, and turned into typed
Go values. Each planner computes `mapdir.Change` values and writes nothing; assets keep a journal of their own, since
they write into the source map and must undo it.

**Tech Stack:** Go 1.27, standard library only.

**Spec:** `docs/superpowers/specs/2026-10-02-moonwell-go-toolchain-design.md`. Plan 5a
(`2026-10-02-moonwell-go-foundations.md`) gives the packages this plan builds on.

**How to read the tasks.** As in Plan 5a: each task's specification is the TypeScript it names (behaviour, messages,
tests) and the Go interface given here. A task is done when its inventory rows are ported and pass. Test-first.

## Global Constraints

- Standard library only. `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass before every commit.
- Messages, hints and written bytes equal the TypeScript's, except the deviations a task lists.
- A string that reaches a file or a message goes through `internal/text` where JavaScript and Go differ; a number
  through `text.Number`.
- Tests that need `pkl` ask `testkit.NeedPkl(t)`: skipped when `pkl` is missing, failed when
  `MOONWELL_REQUIRE_TOOLS=1`.
- Go is not on the PATH of the agent's shell: `export PATH="$PATH:/c/Program Files/Go/bin"`.

## Decision under the spec's §4.7

The spec left open whether the manifest decodes into structs directly or through a cursor over an ordered tree, with
the pinned messages as the criterion. The tests pin them, and they pin the **order** of the checks too: the first
unknown or invalid key in document order is the one reported. So every manifest check runs over the ordered tree
(`ordered.Decode`), as a direct port, and produces typed Go structs. `encoding/json` struct decoding is used only for
our own data files (`metadata.json`, `natives.json`).

## File structure

```
internal/text/number.go        Number: JavaScript's Number-to-string
internal/ordered/tree.go       Decode (an ordered JSON tree), Stringify (JSON.stringify of a tree)
internal/layout/layout.go      the names of the folders and files inside a project
internal/proc/proc.go          running a program and capturing its output
internal/project/              project.go (Load, Parse), version.go, files.go
internal/settings/             options.go, text.go, w3i.go, lua.go, picture.go, preview.go, plan.go
internal/objects/              metadata.go, manifest.go, resolve.go, modfile.go, ids.go, plan.go
internal/assets/               imports.go, paths.go, collect.go, plan.go, apply.go
internal/testkit/              pictures.go, objectdata.go, tools.go, logger.go
```

## Test inventory

| TypeScript test file | Cases | Go test file | Notes |
| --- | --- | --- | --- |
| `shared.test.ts` (process) | 3 | `internal/proc/proc_test.go` | the test program is the Go test binary itself, not Deno |
| `project.test.ts` | 20 | `internal/project/project_test.go` | |
| `project-files.test.ts` | 4 | `internal/project/files_test.go` | the `deno.json` cases are dropped; see Task 2 |
| `settings-options.test.ts` | 7 | `internal/settings/options_test.go` | |
| `settings-text.test.ts` | 7 | `internal/settings/text_test.go` | |
| `settings-w3i.test.ts` | 8 of 10 | `internal/settings/w3i_test.go` | 2 were ported in Plan 5a |
| `settings-lua.test.ts` | 16 | `internal/settings/lua_test.go` | |
| `settings-picture.test.ts` | 7 | `internal/settings/picture_test.go` | |
| `settings-plan.test.ts` | 25 | `internal/settings/plan_test.go` | |
| `metadata.test.ts` | 8 | `internal/objects/metadata_test.go` | |
| `objectdata-manifest.test.ts` | 5 | `internal/objects/manifest_test.go` | |
| `objectdata-resolve.test.ts` | 27 | `internal/objects/resolve_test.go` | |
| `objectdata-modfile.test.ts` | 12 | `internal/objects/modfile_test.go` | |
| `objectdata-ids.test.ts` | 7 | `internal/objects/ids_test.go` | |
| `objectdata-plan.test.ts` | 16 | `internal/objects/plan_test.go` | |
| `imports.test.ts` | 3 | `internal/assets/imports_test.go` | |
| `assets-paths.test.ts` | 5 | `internal/assets/paths_test.go` | |
| `assets-collect.test.ts` | 11 | `internal/assets/collect_test.go` | |
| `assets-plan.test.ts` | 18 | `internal/assets/plan_test.go` | |
| `pkl/project.test.ts` | 1 | `internal/project/pkl_test.go` | needs `pkl` |
| `metadata-readers.test.ts` | 6 | | the generators' SLK and text readers: Plan 5e |
| `pkl/settings`, `objects`, `assets`, `assets-paths`, `library-assets`, `init` | 27 | | they run commands: Plan 5d |

---

### Task 1: `text.Number`, the ordered tree, `layout`, `proc`

**Produces:**

```go
package text
// Number is JavaScript's Number-to-string: the shortest digits that round-trip, plain notation from 1e-6 up to
// 1e21, exponent notation outside ("1e+21", "1e-7").
func Number(v float64) string
```

```go
package ordered
// Decode parses JSON into a tree: *Map[any] for an object, []any for an array, string, float64, bool, nil.
func Decode(data []byte) (any, error)
// Stringify is JSON.stringify(value, null, indent) for such a tree; indent 0 is the compact form.
func Stringify(value any, indent int) string
```

```go
package layout
const (
    LibrariesDir     = ".moonwell/libraries"
    LibraryAssetsDir = ".moonwell/library-assets"
    ObjectIDsFile    = "src/generated/objects.yue"
)
```

```go
package proc
type Result struct {
    Code           int
    Stdout, Stderr string
}
type Options struct {
    Dir  string
    Hint string // shown when the command cannot be started at all
}
type RunFunc func(ctx context.Context, command string, args []string, options Options) (Result, error)
func Run(ctx context.Context, command string, args []string, options Options) (Result, error)
// SpawnError wraps a failure to start command ("command not found", a folder, not executable) in a *diag.Error.
func SpawnError(command string, err error, hint, file string) *diag.Error
```

- [ ] `Number` tests: `0`, `-0` (prints `0`), `128`, `-896`, `0.5`, `1/255` (`0.00392156862745098`), `float64(float32(0.1))`
      (`0.10000000149011612`), `1e21` (`1e+21`), `1e-7`, `123456789012345680000`, `0.000001`, `1.5e-7`, infinities
      and NaN.
- [ ] `Stringify` writes numbers with `text.Number`, strings with `text.Quote`, objects in `Map` order, and with an
      indent the layout of `JSON.stringify(v, null, 2)` (`{}` and `[]` for empty ones).
- [ ] `Run` decodes output with `text.Decode`. A non-zero exit is a `Result`, not an error.
- [ ] Commit: `go: Number, the ordered JSON tree, layout and proc`.

### Task 2: `project`

**Reference:** `cli/src/project/project.ts`, `cli/src/project-files.ts`.

**Produces:**

```go
type Project struct {
    Root     string
    Manifest string // moonwell.local.pkl when it exists, else moonwell.pkl
    Map      struct{ Folder, Entry string }
    Build    struct {
        Folder string
        Minify bool
    }
    Launch struct {
        GameExecutable *string
        Args           []string
    }
    Yue struct {
        Version string
        Path    *string
    }
    Assets struct {
        Paths   ordered.Map[string]
        Exclude []string
    }
    Lint struct {
        UnknownGlobals string // "error" or "warning"
        Globals        []string
    }
    Libraries ordered.Map[Library]
    Settings  *settings.Settings
    Objects   objects.Manifest
}
type Library struct {
    GitHub, Tag, Path *string
    Dir               string
}

const PklInstallHint = "Install Pkl 0.32 or newer: https://pkl-lang.org/main/current/pkl-cli/index.html#installation"

func Load(ctx context.Context, root string, run proc.RunFunc) (*Project, error)
func Parse(root string, value any, file string) (*Project, error)
func CheckPkl(ctx context.Context, run proc.RunFunc) error
func ReadPackageVersion(depsJSON string) (string, error)
func CheckPackageVersion(packageVersion, cliVersion string) error
func EnsureLocalManifest(root string) (bool, error)

const PackageBaseURI = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell"
const DefaultGameExecutable = `C:\Program Files (x86)\Warcraft III\_retail_\x86_64\Warcraft III.exe`
func PklProject(version, local string) string // local != "": a project linked to a checkout's schema/
func LocalPkl() string
```

- [ ] **Texts that named Deno, changed now** (spec §2 exempts them): `LocalPkl`'s first comment says
      `moonwell setup`; `CheckPackageVersion`'s hint is the spec's §8.3 hint, with the install line of the machine's
      shell; `projectDenoJson` and `PROJECT_TASKS` are not ported.
- [ ] Commit: `go: project, the manifest evaluated by pkl`.

### Task 3: `settings`

**Reference:** `cli/src/settings/*`, `cli/src/w3i/patch.ts`.

**Produces:**

```go
type Sections = ordered.Map[*ordered.Map[string]]

type Settings struct {
    Info              Info
    Loading           LoadingScreen
    Players           ordered.Map[Player]
    Forces            ordered.Map[Force]
    Environment       Environment
    Gameplay          Gameplay
    GameplayConstants Sections
    GameInterface     Sections
    Preview           *string
}
type Info struct{ Name, Author, Description, RecommendedPlayers *string }
type LoadingScreen struct {
    Background                   *int32
    Model, Text, Title, Subtitle *string
}
type Player struct {
    Name, Controller, Race *string
    FixedStart             *bool
    X, Y                   *float64
}
type Force struct {
    Name                                                                *string
    Allied, AlliedVictory, SharedVision, SharedControl, SharedAdvancedControl *bool
}
type Environment struct {
    SoundEnvironment *string
    WaterColor       *[4]uint8
    Fog              *Fog
}
type Fog struct {
    Enabled             *bool
    Style               *int32
    Start, End, Density *float64
    Color               *[4]uint8
}
type Gameplay struct{ HeroMaxLevel, FoodLimit *int }

var Controllers = []string{"", "user", "computer", "neutral", "rescuable"}
var Races = []string{"selectable", "human", "orc", "undead", "nightelf"}

func Validate(value any, file string) (*Settings, error) // value: the manifest's settings tree; nil is empty
func (s *Settings) HasExtended() bool
func (s *Settings) Has() bool

func PatchText(source string, sections Sections) string
func GameplaySections(s *Settings, file string) (Sections, error)
func PatchMapInfo(source []byte, s *Settings, file string) ([]byte, error)
func LuaString(value string) string
func PatchLua(source string, s *Settings, patchedW3i []byte, file, w3iFile string) (string, error)
const KeptMinimap = "war3mapMinimap.blp"
func PatchMinimapLua(source, file string) (string, error)

type Picture struct {
    Extension string // "blp" or "tga"
    Bytes     []byte
}
func ReadPicture(data []byte, file string) (*Picture, error)
func LoadPreview(root, preview, manifestFile string) (*Picture, error)

// Plan computes every file the settings change in the map folder, without writing. Names are relative to mapDir.
func Plan(mapDir string, s *Settings, manifestFile, sourceLabel, root string) ([]mapdir.Change, error)
func Apply(mapDir string, changes []mapdir.Change) error
func MapDir(root, mapFolder, manifestFile string) (string, error)
```

- [ ] The Lua patch works in byte offsets where the TypeScript works in UTF-16 offsets; both only slice.
- [ ] Numbers written into Lua (`DefineStartLocation`, `SetTerrainFogEx`) go through `text.Number` of the float32
      widened to a float64, as JavaScript reads a float32.
- [ ] `testkit` gains the picture builders of `tests/support/pictures.ts`.
- [ ] Commit: `go: settings`.

### Task 4: `objects`

**Reference:** `cli/src/objectdata/*`.

**Produces:**

```go
type Category string // heroes, units, buildings, items, abilities, buffs, upgrades
var Categories = []Category{"heroes", "units", "buildings", "items", "abilities", "buffs", "upgrades"}
var FieldCategories = []string{"units", "items", "abilities", "buffs", "upgrades"}

type FieldMeta struct {
    ID, Name, Label, Category, Type, Storage string
    List, PerLevel                          bool
    Column                                  int
    Skin                                    bool
    Use, Specific, NotSpecific              []string
}
type BaseMeta struct {
    Name   string
    Levels *int
}
type Metadata struct {
    Format int
    Game   string
    Fields map[string][]FieldMeta
    Bases  map[Category]map[string]BaseMeta
}
func LoadMetadata() *Metadata
func (m *Metadata) AppliesTo(field *FieldMeta, category Category, base string) bool
func (m *Metadata) FieldsFor(category Category, base string) []*FieldMeta
func (m *Metadata) FieldByRawcode(category Category, id string) *FieldMeta
func (m *Metadata) FieldByName(category Category, base, name string) *FieldMeta
func (m *Metadata) BaseOf(id string) (Category, BaseMeta, bool)
func (m *Metadata) NearestBases(category Category, id string, n int) []NamedBase

const SchemaHint = "Is the moonwell Pkl package the version this CLI expects?"
type ManifestObject struct {
    ID, Base, Source string
    Typed            ordered.Map[any] // a scalar, or a list of scalars and lists of strings
    Properties       ordered.Map[any]
}
type Manifest map[Category]*ordered.Map[ManifestObject]
func EmptyManifest() Manifest
func ParseManifest(value any, present bool, file string) (Manifest, error)
func (m Manifest) Empty() bool

type ModValue struct {
    Type string // int, real, unreal, string
    Int  int32
    Real float64
    Text string
}
type ResolvedField struct {
    ID, Name      string
    Level, Column int
    Skin          bool
    Value         ModValue
}
type Resolved struct {
    Category              Category
    Key, ID, Base, Source string
    Fields                []ResolvedField
}
func Resolve(metadata *Metadata, manifest Manifest, existingIDs map[string]bool) ([]Resolved, error)

type TableKind int // Simple, Leveled
func KindOf(fileName string) TableKind
type ModFile struct { /* version, the original and custom tables with offsets */ }
func ReadModFile(data []byte, kind TableKind, file string) (*ModFile, error)
type NewObject struct {
    Base, ID string
    Mods     []NewMod
}
func AppendObjects(source []byte, kind TableKind, objects []NewObject, file string) ([]byte, error)

func PackID(id string) (uint32, error)
func RenderIDs(objects []Resolved) string
func RefreshIDs(root, expected string) (bool, error)
func AssertIDsCurrent(root, expected string) error

type Plan struct {
    Changes   []mapdir.Change
    Generated string
    Objects   []Resolved
}
type PlanOptions struct {
    Metadata    *Metadata // the embedded metadata when nil
    Manifest    string
    SourceLabel string
}
func PlanObjects(mapDir string, manifest Manifest, options PlanOptions) (*Plan, error)
func Apply(plan *Plan, stagedDir string) error
```

- [ ] Problems are `diag.Problems`, in the TypeScript's order.
- [ ] **Deviation, recorded:** "the string contains an unpaired surrogate" cannot occur: Go's JSON decoder turns an
      unpaired surrogate into U+FFFD before the check. Its test case is dropped.
- [ ] `AssertIDsCurrent`'s hint names `moonwell build` (spec §2 exemption).
- [ ] `testkit` gains the miniature metadata and modification-file builders of `tests/support/objectdata.ts`.
- [ ] Commit: `go: objects`.

### Task 5: `assets`

**Reference:** `cli/src/assets/*`.

**Produces:**

```go
type Import struct {
    Flag uint8
    Path string
}
func ImportPath(entry Import) string
func ReadImports(data []byte, file string) ([]Import, error)
func WriteImports(entries []Import) []byte

func TargetPath(value string) (string, error)
func ScanFiles(root string) (*ordered.Map[string], error) // key → path, in scan order

type Config struct {
    Paths   ordered.Map[string]
    Exclude []string
}
type Asset struct {
    Source, Library, Target string
    Bytes                   []byte
    Hash                    string
}
func Collect(root string, config Config) ([]*Asset, error)
func CollectProject(root string, config Config, libraries []string) (assets []*Asset, replaced []string, err error)

type State struct {
    Version int
    Files   ordered.Map[string]
}
type FileChange struct {
    File          string // absolute
    Before, After []byte // nil: the file is absent before, or removed after
}
type Plan struct {
    Assets   []*Asset
    Replaced []string
    Changes  []FileChange
    State    State
}
func Locations(root, mapFolder string) (mapDir, stateFile string, err error)
func PlanAssets(ctx context.Context, root, mapDir, stateFile string, config Config, libraries []string) (*Plan, error)
// ApplyPlan writes the plan and undoes every change already made if one fails or ctx is cancelled. stateFile ""
// writes no state (a build, which only changes the staged copy).
func ApplyPlan(ctx context.Context, plan *Plan, stateFile string) error
```

- [ ] `project.Project.Assets` is an `assets.Config`... **no**: `assets` must not import `project` and `project`
      need not import `assets`; `project` keeps its own two fields and `pipeline` (Plan 5c) builds the `Config`.
- [ ] Commit: `go: assets`.

### Task 6: Close the plan

- [ ] Every check from a clean tree; the inventory's numbers filled in.
- [ ] `AGENTS.md` (State) and `CHANGELOG.md` (Unreleased) say Plan 5b is implemented, with its deviations.
- [ ] Commit: `docs: Plan 5b of the Go toolchain is implemented`.
