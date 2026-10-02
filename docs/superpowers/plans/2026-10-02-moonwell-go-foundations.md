# Moonwell in Go, Plan 5a: Foundations and Formats — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A Go module beside the TypeScript CLI with the shared packages and the binary formats: `text`, `diag`,
`names`, `fsx`, `binio`, `ordered`, `mapdir`, `luasrc`, `mpq`, `w3i`, `models`, `natives`, the embedded files and
`testkit`, each with the tests its TypeScript counterpart has, and Go checked by CI.

**Architecture:** One module at the repository root with no dependencies; packages under `internal/`. The TypeScript in
`cli/` stays in place and working: Go embeds `cli/runtime/` and `cli/data/` where they are until Plan 5e moves them.
Nothing in this plan is reachable from a command yet; Plan 5d adds `cmd/moonwell`.

**Tech Stack:** Go 1.27, standard library only. `gofmt`, `go vet`, `go test`.

**Spec:** `docs/superpowers/specs/2026-10-02-moonwell-go-toolchain-design.md`

**How to read the tasks.** This is a port with a redesign, so each task's specification is two things: the TypeScript
it names (its behaviour, messages and tests are the reference and are not repeated here) and the Go interface this
plan gives (the redesign). A task is done when every test case of its inventory row has a Go counterpart that passes.
Each task is test-first: write the Go tests from the TypeScript tests, see them fail to compile or fail, write the
package, see them pass, run `gofmt -l .` and `go vet ./...`, commit.

## Global Constraints

- Standard library only: `go.mod` has no `require` line; no file imports `"C"`. A test keeps it so (Task 1).
- Expected failures return `*diag.Error` (or `diag.Problems`) with the file and hint the TypeScript gives, word for
  word. Any other error is an internal error.
- Messages and written bytes equal the TypeScript's for the same input, except the deviations each task lists.
- Version stays `0.7.0` until Plan 5e.
- Go is not on the PATH of the agent's shell: prefix commands with
  `export PATH="$PATH:/c/Program Files/Go/bin"`.
- Checks before every commit: `gofmt -l .` prints nothing, `go vet ./...` and `go test ./...` pass; and, because the
  TypeScript still lives here, `deno fmt --check` and `deno task lint` still pass.
- Files with backslashes are written with a file-editing tool, never through a shell heredoc.

## Amendments to the spec's package table

Found while reading the code; the spec's §3.1 table gains three rows and loses nothing:

| Package | Does |
| --- | --- |
| `internal/text` | The places where JavaScript's strings and Go's differ, in one package: UTF-16 length, offsets and sort order; `JSON.stringify` quoting; JavaScript's whitespace set, `trim`, `toLowerCase`, `toUpperCase`; decoding UTF-8 the way `TextDecoder` and `Deno.readTextFile` do. |
| `internal/names` | `shared/names.ts`: edit distance, joining words, closest names. |
| `internal/testkit` | Test-only helpers and the fixtures several packages share (embedded from `internal/testkit/testdata/`). |

And one fact the spec's §4.4 did not know: JavaScript does not keep plain insertion order. Keys that are canonical
array indexes (`"0"`, `"7"`, `"10"`) come first, in numeric order; the rest follow in insertion order. `ordered.Map`
does the same, since `settings.players` and `settings.forces` are keyed that way.

## File structure

```
go.mod
embed.go            package moonwell: the embedded template, runtime and data; TemplateFiles
version.go          Version
module_test.go      the standard-library rule; the template list; .gitattributes
internal/text/      text.go, utf8.go, text_test.go
internal/diag/      diag.go, diag_test.go
internal/names/     names.go, names_test.go
internal/fsx/       fsx.go, paths.go, fsx_test.go
internal/binio/     binio.go, binio_test.go
internal/ordered/   ordered.go, ordered_test.go
internal/mapdir/    mapdir.go, mapdir_test.go
internal/luasrc/    token.go, requires.go, globals.go, functions.go, mapglobals.go, *_test.go
internal/w3i/       header.go, mapinfo.go, edits.go, *_test.go
internal/mpq/       crypto.go, writer.go, hm3w.go, pack.go, *_test.go
internal/models/    path.go, mdx.go, mdl.go, gamepaths.go, models.go, *_test.go
internal/natives/   natives.go, natives_test.go
internal/testkit/   mpq.go, mdx.go, mapinfo.go, fixtures.go, testdata/…
```

## Test inventory

| TypeScript test file | Cases | Go test file | Ported here | Later or dropped |
| --- | --- | --- | --- | --- |
| `errors.test.ts` | 9 | `internal/diag/diag_test.go` | 8 | 1 dropped: "ObjectDataError is a ProblemsError" (no subclass in Go) |
| `names.test.ts` | 3 | `internal/names/names_test.go` | 3 | |
| `shared.test.ts` | 17 | `internal/fsx/fsx_test.go` | 9 (files, hashing, the locked file) | logger 1, process 3, lock 3: Plan 5b/5d; compression 1 dropped |
| `mpq.test.ts` | 5 | `internal/mpq/mpq_test.go` | 5 | |
| `pack.test.ts` | 7 | `internal/mpq/pack_test.go`, `internal/w3i/header_test.go` | 7 | |
| `settings-w3i.test.ts` | 10 | `internal/w3i/mapinfo_test.go` | 2, and the offsets of a third | 8 with `patchMapInfo`: Plan 5b |
| `settings-lua-structure.test.ts` | 11 | `internal/luasrc/functions_test.go` | 11 | |
| `lexer.test.ts` | 8 | `internal/luasrc/requires_test.go` | 8 | |
| `lua-globals.test.ts` | 3 | `internal/luasrc/globals_test.go` | 3 | |
| `editor-map-globals.test.ts` | 4 | `internal/luasrc/mapglobals_test.go` | 2 | 2 (rendering `map.d.lua`): Plan 5c |
| `model-mdl.test.ts` | 5 | `internal/models/mdl_test.go` | 5 | |
| `model-mdx.test.ts` | 5 | `internal/models/mdx_test.go` | 5 | |
| `game-paths.test.ts` | 5 | `internal/models/gamepaths_test.go` | 4 | 1 dropped: gzip round trip |
| `embedded.test.ts` | 9 | `module_test.go` | 3 (template list, generated files skipped, `.gitattributes`) | 5 freshness tests dropped; `schema/generated`: Plan 5e |

New tests with no TypeScript counterpart: `text`, `binio`, `ordered`, `mapdir`, the module rule.

---

### Task 1: The module, the embedded files, CI

**Files:** Create `go.mod`, `embed.go`, `version.go`, `module_test.go`. Modify `.gitattributes`, `deno.json`,
`.github/workflows/ci.yml`.

**Produces:**

```go
package moonwell

const Version = "0.7.0"

var RuntimeLua string // cli/runtime/moonwell.lua
var MacrosYue string  // cli/runtime/macros.yue
var GamePaths string  // cli/data/game-paths.txt
var Metadata []byte   // cli/data/metadata.json
var Natives []byte    // cli/data/natives.json

type TemplateFile struct {
    Path string // with "/"
    Data []byte
}

// TemplateFiles is every file init copies, sorted by path.
func TemplateFiles() ([]TemplateFile, error)

// TemplateExclude names the files init writes itself or never writes.
var TemplateExclude = []string{"deno.json", "PklProject", "PklProject.deps.json", "moonwell.local.pkl"}
```

- [x] `go.mod`: `module github.com/mdlsvensson/moonwell` and `go 1.27`.
- [x] `embed.go`: `//go:embed all:template` and the five single-file embeds. `TemplateFiles` walks the embedded
      tree, skips `TemplateExclude` and what a project generates (`dist/`, `.moonwell/`, `src/**/*.lua`), and sorts
      with `text.Less`.
- [x] `module_test.go`: `go.mod` has no `require` and no Go file under the module imports `"C"`; `TemplateFiles`
      equals a list written out in the test (the 30 files `git ls-files template` gives, less the excluded ones);
      `skipsGenerated` is tested on its own with the four paths of the TypeScript test; `.gitattributes` in the
      template has `maps/** binary` and `assets/** -text`; `Version` equals the `version` in `schema/PklProject`.
- [x] `.gitattributes`: add `**/testdata/** binary`.
- [x] `deno.json`: add `"internal"` to `fmt.exclude` (fixture copies must not be reformatted).
- [x] `ci.yml`: after `setup-deno`, `actions/setup-go@v5` with `go-version-file: go.mod`; after the Deno steps,
      `go vet ./...`, a step that fails when `gofmt -l .` prints anything, and `go test ./...` with
      `MOONWELL_REQUIRE_TOOLS: "1"` and `MOONWELL_NETWORK_TESTS: "1"`.
- [x] Commit: `go: the module, embedded files and CI`.

### Task 2: `text`

**Files:** Create `internal/text/text.go`, `utf8.go`, `text_test.go`.

**Produces:**

```go
func UTF16Len(s string) int
func UTF16Offset(s string, byteOffset int) int // the UTF-16 index of a byte offset
func Less(a, b string) bool                    // JavaScript's default sort order
func Compare(a, b string) int
func Sort(s []string)
func Quote(s string) string     // JSON.stringify of a string
func IsSpace(r rune) bool       // JavaScript's \s
func Trim(s string) string      // String.prototype.trim
func Lower(s string) string     // toLowerCase
func Upper(s string) string     // toUpperCase
func Lossy(b []byte) string     // Deno.readTextFile: bad bytes become U+FFFD, a BOM stays
func Decode(b []byte) string    // new TextDecoder().decode: Lossy, then a leading BOM removed
func Strict(b []byte) (string, bool) // fatal TextDecoder: false for bad bytes; a leading BOM removed
```

- [x] Tests: `UTF16Len("a🌙")` is 3; `Less` puts `""` after `"🌙"` (JavaScript's order) where byte order has
      it before; `Quote` of a string with `"`, `\`, a newline, U+0007, `<`, `é`, U+2028 gives `JSON.stringify`'s
      output (`"\""`, `"\\"`, `"\n"`, `"\u0007"`, `<` and `é` and U+2028 unescaped); `IsSpace` accepts U+00A0 and
      U+FEFF and rejects U+200B; `Lossy` of `FF FF` is two U+FFFD and of `E2 82 41` is one U+FFFD then `A`
      (one replacement per maximal invalid prefix); `Upper("straße")` is `STRASSE`; `Lower("İ")` is `i̇`.
- [x] `Lossy` follows the Encoding Standard's UTF-8 decoder: a lead byte with too few valid continuation bytes is
      one replacement, and the byte that broke the sequence is read again.
- [x] `Upper` and `Lower` use `strings.ToUpper`/`ToLower` plus the special cases that occur in file names: `ß` to
      `SS`, the ligatures U+FB00 to U+FB06, `İ` to `i` and U+0307. **Deviation, recorded:** other entries of
      Unicode's SpecialCasing table and the final sigma rule are not reproduced.
- [x] Commit: `go: text, where JavaScript and Go strings differ`.

### Task 3: `diag` and `names`

**Reference:** `cli/src/shared/errors.ts`, `names.ts`; `errors.test.ts`, `names.test.ts`.

**Produces:**

```go
package diag

type Error struct {
    Msg    string
    File   string
    Line   int
    Column int
    Hint   string
    Cause  error
}
func (e *Error) Error() string
func (e *Error) Unwrap() error

type Problem struct {
    File   string
    Line   int
    Column int
    Msg    string
    Hint   string
}
type Problems []Problem
func (p Problems) Error() string // the first problem's message

const MaxProblems = 20
func FormatProblem(p Problem) string
func Format(err error) string
// First is err as one Problem: an *Error's fields, or the first of Problems. False for any other error.
func First(err error) (Problem, bool)
```

```go
package names

func EditDistance(a, b string) int
func JoinWords(words []string, conjunction string, max int) string // max < 0: all
func Closest(names []string, key string, max int) []string
```

- [x] `Format` of any other error: `internal error: <err.Error()>` and `This is a bug in Moonwell; please report it.`
- [x] `EditDistance` counts UTF-16 units, as the TypeScript does.
- [x] Commit: `go: diag and names`.

### Task 4: `fsx`

**Reference:** `cli/src/shared/fs.ts`; `assetPath`, `lstatOrUndefined` and `safeJoin` of `cli/src/assets/paths.ts`;
the nine file cases of `shared.test.ts`.

**Produces:**

```go
func ToPosix(path string) string
func IsWithin(path, folder string) bool
func ListFiles(dir string) ([]string, error)          // sorted with text.Less
func RemoveAll(path string) error                     // removeIfExists
func RemoveFile(path string) error                    // removeFileIfExists
func ReplaceDir(source, destination string) error
func WriteIfChanged(path, content string) (bool, error)
func ReadSource(path, label string) (string, error)
func SHA256Hex(b []byte) string
func Exists(path string) bool
func IsDir(path string) bool
func RelPath(value string) (string, error)            // assetPath: "Invalid asset path: …"
func SafeJoin(root, relative string) (string, error)
```

- [x] "In use by another program" is `syscall.EBUSY`, or on Windows the sharing violation (`syscall.Errno(32)`); the
      Windows-only test holds a file open through PowerShell as the TypeScript test does.
- [x] `ReadSource` decodes with `text.Lossy`, as `Deno.readTextFile` does.
- [x] `RelPath` rejects control characters by UTF-16 unit, `<>:"|?*`, a trailing dot or space, and the device names.
- [x] Commit: `go: fsx`.

### Task 5: `binio`, `ordered`, `mapdir`

**Reference:** none for `binio` and `ordered` (new); `pathKey` of `assets/paths.ts` and the two `…FileNames` and
`apply…Plan` functions of `settings/plan.ts` and `objectdata/plan.ts` for `mapdir`.

**Produces:**

```go
package binio

type Reader struct{ /* … */ }
func NewReader(b []byte) *Reader
func (r *Reader) Offset() int
func (r *Reader) Len() int
func (r *Reader) Skip(n int)
func (r *Reader) U8() uint8
func (r *Reader) U16() uint16
func (r *Reader) U32() uint32
func (r *Reader) I32() int32
func (r *Reader) F32() float32
func (r *Reader) Bytes(n int) []byte
func (r *Reader) CString() []byte // without the NUL
func (r *Reader) Err() *Error     // the first failure; later reads return zero

type Error struct {
    Offset       int
    Unterminated bool // a string without its NUL; otherwise a read past the end
}

type Writer struct{ /* … */ }
func (w *Writer) U8(v uint8)
func (w *Writer) U16(v uint16)
func (w *Writer) U32(v uint32)
func (w *Writer) I32(v int32)
func (w *Writer) F32(v float32)
func (w *Writer) Write(b []byte)
func (w *Writer) CString(s string)
func (w *Writer) Zero(n int)
func (w *Writer) Len() int
func (w *Writer) Bytes() []byte
```

```go
package ordered

type Map[V any] struct{ /* … */ }
func (m *Map[V]) UnmarshalJSON(data []byte) error
func (m Map[V]) MarshalJSON() ([]byte, error)
func (m *Map[V]) Set(key string, value V)
func (m *Map[V]) Get(key string) (V, bool)
func (m *Map[V]) Has(key string) bool
func (m *Map[V]) Delete(key string)
func (m *Map[V]) Len() int
func (m *Map[V]) Keys() []string           // JavaScript's order
func (m *Map[V]) All() iter.Seq2[string, V]
```

```go
package mapdir

func Key(path string) string
// Names maps the key of each wanted file to the spelling it has among entries. Two spellings of one file fail,
// naming label(spelling).
func Names(entries, wanted []string, label func(name string) string) (map[string]string, error)

type Change struct {
    Name   string // relative to the map folder, in the spelling to write
    Bytes  []byte
    Remove bool
}
// Apply writes changes into dir. A failure is a *diag.Error with message failure and the "close the game" hint.
func Apply(dir string, changes []Change, failure string) error
```

- [x] `ordered` tests: document order kept; `{"b":1,"10":2,"2":3,"a":4}` ranges `2, 10, b, a`; `"01"` and `"-1"`
      are not indexes; a repeated key keeps its first place and its last value; `null` gives an empty map; marshal
      writes the same order.
- [x] `mapdir.Names` sorts entries with `text.Sort` before matching, as both TypeScript functions do, so the error
      for two spellings names the same pair in the same order.
- [x] Commit: `go: binio, ordered and mapdir`.

### Task 6: `luasrc`

**Reference:** `cli/src/bundle/lexer.ts`, `settings/lua-structure.ts`, `lint/lua-globals.ts`, `readMapGlobals` of
`editor/map-globals.ts`; their four test files.

**Produces:**

```go
type Kind uint8
const (
    Name Kind = iota
    Number
    String
    Symbol
)

type Token struct {
    Kind    Kind
    Raw     string // the source text of the token
    Text    string // a string's content: between the quotes with its escapes, or a long string without its first newline
    Start   int    // byte offsets into the source
    End     int
    Line    int // 1-based
    Escaped bool // a quoted string with a backslash
}

type Fault struct {
    Msg    string
    Offset int
}

// Tokenize never fails: the first malformed token is reported as the fault and lexing goes on.
func Tokenize(source string) ([]Token, *Fault)

type Require struct {
    Line    int
    Name    string
    Literal bool // false: the argument is not one plain string literal
}
func Requires(source string) []Require
func TopLevelGlobals(source string) []string

type Call struct {
    Name       string
    Args       [][]Token
    Start, End int
}
type Function struct {
    Name                 string
    Start, End, EndStart int
    Calls                []Call
}
func Functions(source, file string) ([]Function, error)
func LiteralNumber(tokens []Token) (float64, bool)
func PlayerID(tokens []Token) (int, bool)

type Global struct{ Name, Type string }
type MapGlobals struct {
    Globals   []Global
    Functions []string
}
func ReadMapGlobals(script string) MapGlobals
```

**The one tokenizer.** It lexes as `lua-structure.ts` does (Lua's grammar: `\z`, a backslash before a line break,
multi-character symbols, the two numeral forms written without lookahead), and where that tokenizer throws it records
the fault and recovers as `lexer.ts` does: a quoted string ends at a bare line break, an unterminated long bracket
runs to the end, a bad numeral takes `[0-9A-Za-z_.]` and exponent signs, an unknown character is a symbol of its own.

- `Functions` fails on a fault with the TypeScript's message; `at character N` counts UTF-16 units
  (`text.UTF16Offset`).
- `Requires` and `TopLevelGlobals` ignore faults. They read a view of the tokens that is the old lenient stream:
  numbers left out and every symbol except a run of dots split into single characters. So their code is a direct
  port.
- [x] **Deviations, recorded.** For Lua the game itself would reject, or for two escapes the lenient lexer got wrong,
      `Requires` and `TopLevelGlobals` can differ from 0.7.0: a quoted string with `\z` before a line break or a
      backslash before CRLF now continues (it ended at the break); a line break that ends an unterminated string now
      counts as a line; `1..2` is three tokens (it was one dropped number). Whitespace is JavaScript's `\s` in both
      scanners (the lenient lexer knew ASCII only).
- [x] `ReadMapGlobals` stays line-based with the TypeScript's four patterns.
- [x] The fixtures `map-settings-v39/war3map.lua` and `map-globals-we3/war3map.lua` come from `testkit` (Task 9
      creates it; this task adds `internal/testkit/fixtures.go` and the copied `testdata/` first).
- [x] Commit: `go: luasrc, one Lua tokenizer and its scanners`.

### Task 7: `w3i`

**Reference:** `cli/src/map/w3i.ts`, `cli/src/w3i/map-info.ts`; `pack.test.ts` (two header cases),
`settings-w3i.test.ts` (reader cases).

**Produces:**

```go
type Header struct {
    Version        int32
    HasGameVersion bool
    Major, Minor   uint32
}
func ReadHeader(b []byte) (Header, error)
func (h Header) Headerless() bool

type Field[T any] struct {
    Start, End int
    Value      T
}
type Color [4]Field[uint8] // red, green, blue, alpha; stored blue first

type Info struct {
    Version            int32
    Name               Field[string]
    Author             Field[string]
    Description        Field[string]
    RecommendedPlayers Field[string]
    Flags              Field[int32]
    Loading            Loading
    Details            *Details // nil unless read extended
}
type Loading struct {
    Background Field[int32]
    Model      *Field[string] // nil before version 25
    Text       Field[string]
    Title      Field[string]
    Subtitle   Field[string]
}
type Details struct {
    Fog              Fog
    SoundEnvironment Field[string]
    WaterColor       Color
    Players          []Player
    Forces           []Force
}
type Fog struct {
    Style               Field[int32]
    Start, End, Density Field[float32]
    Color               Color
}
type Player struct {
    ID, Controller, Race, FixedStart Field[int32]
    Name                             Field[string]
    X, Y                             Field[float32]
}
type Force struct {
    Flags, Players Field[int32]
    Name           Field[string]
}
func Read(b []byte, extended bool, file string) (*Info, error)

type Edit struct {
    Start, End int
    Bytes      []byte
}
func TextEdit(f Field[string], value string) Edit
func IntEdit(f Field[int32], value int32) Edit
func FloatEdit(f Field[float32], value float32) Edit
func ByteEdit(f Field[uint8], value uint8) Edit
func ApplyEdits(source []byte, edits []Edit) ([]byte, error)
```

- [x] The reader keeps the first failure and reads nothing after it, so the error for a cut-off file is the
      TypeScript's (`truncated war3map.w3i`, `unterminated string in war3map.w3i`, invalid UTF-8) and never a later
      check's.
- [x] `testkit.SyntheticMapInfo(version)` is `syntheticMapInfo` of `tests/support/map-settings.ts`.
- [x] Commit: `go: w3i reader and byte edits`.

### Task 8: `mpq`

**Reference:** `cli/src/mpq/*`, `cli/src/map/pack.ts`; `mpq.test.ts`, `pack.test.ts`; `tests/support/mpq-reader.ts`.

**Produces:**

```go
type HashType uint32
const (
    TableOffset HashType = iota
    NameA
    NameB
    FileKey
)
func HashString(s string, t HashType) uint32
var HashTableKey, BlockTableKey uint32
func EncryptBlock(words []uint32, key uint32)
func DecryptBlock(words []uint32, key uint32)

type File struct {
    Name string // with backslashes
    Data []byte
}
type Options struct {
    Prefix          []byte
    SectorSizeShift int // 0 means 3
}
func Write(files []File, opts Options) ([]byte, error)
func HM3WHeader(name string, flags, maxPlayers uint32) []byte
func PackMap(mapDir, mapName string) ([]byte, error)
```

```go
package testkit

type MPQ struct{ HeaderOffset int /* … */ }
func OpenMPQ(b []byte) (*MPQ, error)
func (m *MPQ) Read(name string) ([]byte, bool, error)
func (m *MPQ) Listfile() ([]string, error)
```

- [x] Sectors are compressed with `compress/zlib` at the default level; a sector is stored raw unless the compressed
      form plus its mask byte is smaller, as today. **Outside the contract (spec §2):** the compressed bytes differ
      from Deno's.
- [x] `testkit` must not import `mpq` (it would be a cycle for `mpq`'s own tests): `testkit.OpenMPQ` takes the hash
      and decrypt functions it needs from a tiny copy of the crypt table, or the MPQ tests live in package
      `mpq_test`. Use the second: `testkit` imports `mpq`, and `mpq`'s tests are an external test package.
- [x] Commit: `go: the MPQ writer and map packing`.

### Task 9: `models`, `natives`, `testkit`

**Reference:** `cli/src/models/*`, `cli/src/natives/natives.ts`; `model-mdl.test.ts`, `model-mdx.test.ts`,
`game-paths.test.ts`; `tests/support/mdx.ts`.

**Produces:**

```go
package models

type Kind string
const (
    Texture         Kind = "texture"
    ParticleModel   Kind = "particle model"
    ParticleTexture Kind = "particle texture"
    Attachment      Kind = "attachment"
    Popcorn         Kind = "popcorn"
    FaceEffect      Kind = "face effect"
)
type Path struct {
    Kind          Kind
    Path          string // "" for a replaceable texture, which has only its slot
    ReplaceableID uint32
}
func Describe(p Path) string
func IsMDX(b []byte) bool
func ReadMDX(b []byte, file string) ([]Path, error)
func ReadMDL(text, file string) ([]Path, error)
func Paths(b []byte, file string) ([]Path, error)

var TextureExtensions = []string{"blp", "dds", "tga", "tif", "tiff", "png", "jpg"}
func NormalizeGamePath(line string) (string, bool)
func RenderGamePaths(list, version string) string
func GamePathKey(path string) string
func ParseGamePaths(text string) map[string]bool
func LoadGamePaths() map[string]bool // the embedded list, parsed once
```

```go
package natives

type Param struct{ Name, Type string }
type Function struct {
    Name     string
    Source   string
    Constant bool
    Params   []Param
    Returns  string
}
type Global struct {
    Name     string
    Source   string
    Type     string
    Constant bool
    Array    bool
}
type Type struct{ Name, Extends string }
type Natives struct {
    GameVersion string
    Types       []Type
    Functions   []Function
    Globals     []Global
    Lua         struct{ Globals, Removed []string }
}
func Load() *Natives // the embedded natives.json, parsed once
```

- [x] `testkit` gains the MDX builders of `tests/support/mdx.ts` (`MDX`, `Chunk`, `Texture`, `Emitter`,
      `Attachment`, `Popcorn`, `FaceEffect`, `Concat`, `U32`, `SetU32`).
- [x] A `natives` test: `Load` finds `CreateUnit` with its six parameters and `UnitAlive`, and `collectgarbage`
      among the removed Lua globals.
- [x] Commit: `go: models, natives and the test kit`.

### Task 10: Close the plan

- [x] Run every check of Global Constraints from a clean tree.
- [x] Fill the inventory table's numbers with what was ported; a difference from the plan is explained in the table.
- [x] `AGENTS.md`: under State, a paragraph for Plan 5a (what exists, the deviations of Tasks 2 and 6, how to run
      Go here). `CHANGELOG.md`: under `## Unreleased`, one line.
- [x] Commit: `docs: Plan 5a of the Go toolchain is implemented`.
