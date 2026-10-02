# Moonwell in Go, Plan 5d: The Command Line — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.
> Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `moonwell` as a program: `internal/watch`, `internal/cli` with every command, `cmd/moonwell`, the tests that
run commands and whole builds, and the conformance suite that runs the Go and the Deno CLI on the same projects and
compares what they print and write.

**Architecture:** As Plans 5a to 5c. A command is a function `(ctx, *pipeline.Env, …)` in `internal/cli` and a row in
the command table; `cli.Run` parses the arguments, makes the logger and the environment, calls the command and turns
its error into printed lines and an exit code. `cmd/moonwell` adds only what needs a real process: the working
directory, the standard streams and the signals.

**Tech Stack:** Go 1.27, standard library only (`os/signal`, `syscall` for the detached game).

**Spec:** `docs/superpowers/specs/2026-10-02-moonwell-go-toolchain-design.md` (§2 the contract, §4.2, §4.8, §4.9, §6.2,
§8.4, §8.5). Plans 5a to 5c give the packages this plan builds on.

**How to read the tasks.** As before: each task's specification is the TypeScript it names and the Go interface given
here; a task is done when its inventory rows are ported and pass. Test-first.

## Global Constraints

- Standard library only. `gofmt -l .` prints nothing; `go vet ./...` and `go test ./...` pass before every commit.
- Messages, hints and written bytes equal the TypeScript's, except the texts that name Deno (§2): `deno task <name>`
  is `moonwell <name>`, and `init` ends with `cd <dir> && moonwell build`.
- The Go code still carries version 0.7.0, so both CLIs accept the same projects.
- Tests that run commands on a real project ask `testkit.NeedPkl(t)` and, when they compile, `yuetest.Need(t)`.
- Go is not on the PATH of the agent's shell: `export PATH="$PATH:/c/Program Files/Go/bin"`.

## Decisions

- **Signals live in `cmd/moonwell`.** `cli.Run` takes a context. The first Ctrl+C cancels it: `dev` stops watching
  and returns once its cycle has finished (its cycles run on a context the interrupt does not cancel);
  `assets:sync` rolls back; any other command stops at its next waiting point. A command that ends with a cancelled
  context exits with 130; a bare `context.Canceled` prints nothing. The second Ctrl+C removes the locks this process
  holds and exits with 130 at once.
- **Unknown flags are ignored, as today.** `@std/cli`'s parser takes any flag; an unknown one swallows the argument
  after it when that does not start with `-`. The Go parser does the same, so no command line that works today stops
  working.
- **`init --link`** finds the checkout by walking up from the working directory to a `go.mod` that names this
  module (§8.5). `InitOptions.Checkout` lets tests name it.
- **`setup`** prints one line for a `deno.json` that names `@moonwell/cli` or `cli/src/main.ts` (§8.4).
- **`dev` polls** (`internal/watch`): 250 ms between passes, 150 ms of quiet before a cycle. Both are options, so
  tests need not wait.
- **End-to-end tests call `cli.Run`** in the test process instead of starting a program per command: the same code
  path as `main`, without building an executable per test. One test builds `cmd/moonwell` and runs it, for the parts
  only a process has (exit codes, the streams, Ctrl+C on `dev`).
- **The conformance suite** (`internal/conformance`, test-only) runs with `MOONWELL_CONFORMANCE=1` when `deno`, `pkl`
  and the repository's `cli/` are present. It goes away with the TypeScript in Plan 5e.

## Amendments made while implementing

- **The conformance suite found one difference,** on the Windows runner, where `yue` is not on the PATH: with
  `yue.path` set, `setup`'s PATH command named the compiler's folder with backslashes, where the Deno CLI keeps the
  separators the manifest wrote. `yue.DirAsWritten` fixes it. Nothing else differed in 35 compared command lines
  (38 with the gate map, which only a machine with `../wrappers-gate` runs).
- **Its last green run** on Ubuntu and Windows is commit `e786070` (CI run 37007905168).
- **What it compares beyond the spec's list:** the source map and `.asset-state/` after every command, the files
  `init` and `setup` write (`.gitignore`, `.luarc.json`, `.vscode/`, `yueconfig.yue`, the manifests, `PklProject`
  and its resolved dependencies), and the packed map's hash table, block sizes and flags, and the fixed fields of
  its header. Each project pins `yue.path` to one compiler, so neither CLI installs one.
- **The interrupt test on Windows** sends Ctrl+Break to the program's own process group, from a second copy of the
  test program that joins the program's console. Ctrl+C is switched off below a process started in a new process
  group, as test runners are; and joining a console removes the sender's own handlers, so the event must not reach
  it. Go reports both keys as `os.Interrupt`.
- **Test files** differ from the inventory's names: `unit_test.go` holds the cases of `build`, `dev`, `init`,
  `assets-paths` and `launch`; `cli_test.go` those of `main` and the argument parser; `process_test.go` the test of
  the real program. `pkl/objects.test.ts` has 11 cases, not 12. Every case of the inventory is ported.
- **Who wrote what:** the tests of Task 4 from the settings script onward, and the conformance suite, were written
  by another agent (Codex, GPT) while Claude was out of usage, and reviewed, run and fixed by Claude afterwards.

## File structure

```
cmd/moonwell/main.go
internal/watch/       watch.go
internal/cli/         cli.go args.go launch.go spawn_windows.go spawn_unix.go
                      build.go check.go dev.go initcmd.go setup.go testcmd.go
                      assets.go assetspaths.go settingscheck.go objects.go
internal/conformance/ conformance_test.go
```

## Test inventory

| TypeScript test file | Cases | Go test file |
| --- | --- | --- |
| `main.test.ts` | 7 | `internal/cli/cli_test.go` |
| `build.test.ts` | 4 | `internal/cli/build_test.go` |
| `dev.test.ts` | 6 | `internal/cli/dev_test.go` |
| `init.test.ts` | 6 | `internal/cli/init_test.go` |
| `assets-paths.test.ts` | 5 | `internal/cli/assetspaths_test.go` |
| `launch.test.ts`, `pipeline.test.ts` (launchGame) | 3 + 2 | `internal/cli/launch_test.go` |
| `yue/runtime.test.ts` | 7 | `internal/bundle/runtime_test.go` (needs `yue`) |
| `yue/objects.test.ts` | 2 | `internal/objects/yue_test.go` (needs `yue`) |
| `yue/settings.test.ts` | 5 | `internal/settings/yue_test.go` (needs `yue`) |
| `pkl/init.test.ts` | 2 | `internal/cli/project_test.go` (needs `pkl`) |
| `pkl/assets.test.ts`, `library-assets.test.ts`, `assets-paths.test.ts` | 2 + 2 + 2 | `internal/cli/assets_pkl_test.go` |
| `pkl/settings.test.ts` | 8 | `internal/cli/settings_pkl_test.go` |
| `pkl/objects.test.ts` | 12 | `internal/cli/objects_pkl_test.go` |
| `e2e/project.test.ts` | 12 | `internal/cli/e2e_test.go` (needs `pkl` and `yue`) |
| `e2e/lint`, `lua`, `libraries`, `library-assets`, `objects` | 4 + 4 + 4 + 1 + 1 | `internal/cli/e2e_test.go` |
| `e2e/settings.test.ts` | 8 | `internal/cli/e2e_settings_test.go` |
| `versions-consistent.test.ts` | 1 | `module_test.go` (ported in Plan 5a; the `cli/deno.json` half goes in 5e) |
| (new) the watcher | | `internal/watch/watch_test.go` |
| (new) the argument parser | | `internal/cli/args_test.go` |

---

### Task 1: `watch`

**Produces:**

```go
// Root is one watched folder.
type Root struct {
    Dir       string
    Recursive bool
    // Relevant says whether a change of this path (absolute) counts.
    Relevant func(path string) bool
}
// Watcher remembers what the roots held at the last pass.
type Watcher struct{ /* … */ }
func New(roots []Root) *Watcher // takes the first pass at once
func (w *Watcher) Poll() bool   // another pass: whether a relevant file appeared, went or changed size or time
```

- [x] A root that is missing or unreadable holds nothing; it is not an error.
- [x] Commit: `go: watch`.

### Task 2: `cli`: arguments, the table, launching

**Reference:** `cli/src/main.ts`, `launch.ts`.

**Produces:**

```go
type Flags struct {
    Positional []string
    Entry      string
    Minify, Link, Help, Version bool
}
func ParseArgs(args []string) Flags
const Usage = "…" // rendered from the table; the first line has the version
// Run runs one command line. write gets the log lines (stderr), print the output meant for other programs.
func Run(ctx context.Context, args []string, root string, write, print func(string)) int
func LaunchGame(launch project.Launch, mapPath string, spawn func(string, []string) error) error
func SpawnDetached(command string, args []string) error
```

- [x] `Run` recovers a panic and prints it as an internal error with the stack.
- [x] Commit: `go: cli, arguments and launching`.

### Task 3: the commands

**Reference:** `cli/src/commands/*`.

**Produces:**

```go
func Build(ctx context.Context, env *pipeline.Env, options pipeline.StageOptions) (string, error)
func ArchivePath(root string, p *project.Project) (string, error)
func Test(ctx context.Context, env *pipeline.Env, options pipeline.StageOptions) error
type CheckResult struct{ Modules int; Entry string; Assets int }
func Check(ctx context.Context, env *pipeline.Env, refreshObjectIDs bool) (CheckResult, error)
type InitOptions struct{ Link bool; Checkout string }
func Init(ctx context.Context, env *pipeline.Env, dir string, options InitOptions) (string, error)
func LinkPath(target, path string) (string, error)
func Setup(ctx context.Context, env *pipeline.Env) (string, error)
func Assets(ctx context.Context, env *pipeline.Env, sync bool) (*assets.Plan, error)
type ModelReport struct{ Heading string; Refs []ModelRef }
func AssetsPaths(ctx context.Context, env *pipeline.Env, file string, gamePaths map[string]bool) ([]ModelReport, error)
func SettingsCheck(ctx context.Context, env *pipeline.Env) ([]mapdir.Change, error)
func ObjectsEval(ctx context.Context, env *pipeline.Env, print func(string)) (*ordered.Map[any], error)
func ObjectsCheck(ctx context.Context, env *pipeline.Env) (*objects.Plan, error)
type DevOptions struct{ Debounce, Interval time.Duration }
func Dev(ctx context.Context, env *pipeline.Env, options DevOptions) error
func IsRelevantChange(root, path string) bool
func IsLibraryChange(folder, path string) bool
func LocalLibraryFolders(root string, p *project.Project) []string
```

- [x] `objects:eval` prints with `ordered.Stringify(…, 2)`: categories in their fixed order, objects in manifest
      order, each field's keys in today's order.
- [x] Commit: `go: the commands`.

### Task 4: `cmd/moonwell` and the tests on real projects

- [x] `cmd/moonwell/main.go`: the working directory, stderr and stdout, the two-stage Ctrl+C.
- [x] The `yue`-backed tests of `bundle`, `objects` and `settings`.
- [x] The `pkl`-backed and end-to-end tests of the inventory.
- [x] Commit: `go: cmd/moonwell and the end-to-end tests`.

### Task 5: the conformance suite

**Produces:** `internal/conformance/conformance_test.go`, which for each project of the spec's §6.2 table makes two
copies, runs the Deno CLI (`deno run -A <repo>/cli/src/main.ts`) in one and `cli.Run` in the other, and compares the
exit code, the printed lines (after replacing each copy's path, and the texts §2 exempts), the written files, and the
packed map unpacked (`testkit`'s MPQ reader).

- [x] CI sets `MOONWELL_CONFORMANCE=1` on both systems.
- [x] Every difference found is either fixed in the Go code or recorded here as a deviation with its reason.
- [x] Commit: `go: the conformance suite`.

### Task 6: Close the plan

- [x] Every check from a clean tree; `AGENTS.md` and `CHANGELOG.md` say Plan 5d is implemented, with its deviations
      and the conformance suite's green run.
- [x] Commit: `docs: Plan 5d of the Go toolchain is implemented`.
