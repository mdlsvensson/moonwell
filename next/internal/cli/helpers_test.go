package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
	"github.com/mdlsvensson/moonwell/next/internal/tooltest"
)

var background = context.Background()

// outcome is what a command line printed and how it ended.
type outcome struct {
	code int
	// output is what went to the terminal, stdout what was printed for other programs.
	output, stdout string
}

// mark stands between the place of a failure and its message, as a line prints one: U+203A.
const mark = "\xe2\x80\xba"

// run runs a command line in root, as the program does: in the real world, whose cache is the user's. A test
// runs so only a line that touches nothing of the user's: one that is refused or answered before it has a world,
// one that starts no program, and one of a project that compiles, in a test that has asked for the compiler
// (compiling): the compiler is then in the cache, and the line downloads nothing. A line that evaluates a
// manifest and needs no compiler runs with okWithPklAlone or failsWithPklAlone, and setup in a seeded world.
func run(root string, args ...string) outcome {
	var lines, printed []string
	code := Run(background, args, root, func(line string) { lines = append(lines, line) },
		func(text string) { printed = append(printed, text) })
	return outcome{code, strings.Join(lines, "\n"), strings.Join(printed, "\n")}
}

// carried runs a command line in root as the program does, but for the outside world, which is a stand-in.
func carried(t *testing.T, ctx context.Context, root string, args ...string) outcome {
	t.Helper()
	return carriedIn(ctx, standIn(t), root, args...)
}

// carriedIn runs a command line in root as the program does, in the outside world that is given.
func carriedIn(ctx context.Context, outside world, root string, args ...string) outcome {
	var lines, printed []string
	code := runIn(ctx, outside, args, root, func(line string) { lines = append(lines, line) },
		func(text string) { printed = append(printed, text) })
	return outcome{code, strings.Join(lines, "\n"), strings.Join(printed, "\n")}
}

// standIn is a stand-in for the outside world. The log is the line's own, so what a command says is printed and
// kept as in the real world. Of all programs it answers one, pkl asked for its version, as a Pkl that is new
// enough: so a command gets as far as the manifest without a program started, and any other program fails the
// test. A program is not started with a context that is cancelled, and the context's error is returned, as
// env.Run does.
func standIn(t *testing.T) world {
	return func(root string, log *env.Logger) *env.Env {
		e, _ := testkit.Env(t, root)
		e.Log = log
		e.Run = func(ctx context.Context, program string, args []string, _ env.RunOptions) (env.RunResult, error) {
			switch {
			case ctx.Err() != nil:
				return env.RunResult{}, ctx.Err()
			case program == toolchain.Pkl.Name && slices.Equal(args, toolchain.Pkl.VersionArgs):
				return env.RunResult{Stdout: "Pkl 0.32.1 (a stand-in)\n"}, nil
			}
			// Without stopping the test: a command may run a program from a goroutine of its own.
			t.Errorf("the test has no stand-in for the program: %s %q", program, args)
			return env.RunResult{}, errors.New("no stand-in for " + program)
		}
		return e
	}
}

// ok runs a command line that must succeed.
func ok(t *testing.T, root string, args ...string) outcome {
	t.Helper()
	return endedWith(t, run(root, args...), 0, args)
}

// fails runs a command line that must exit with 1, printing every part of wanted.
func fails(t *testing.T, root string, wanted []string, args ...string) outcome {
	t.Helper()
	result := endedWith(t, run(root, args...), 1, args)
	contains(t, result.output, wanted...)
	return result
}

// endedWith is the outcome of a command line that must have ended with this exit code.
func endedWith(t *testing.T, result outcome, code int, args []string) outcome {
	t.Helper()
	if result.code != code {
		t.Fatalf("moonwell %s exited with %d:\n%s", strings.Join(args, " "), result.code, result.output)
	}
	return result
}

// seededWorld is a world for a command that asks for the project's compiler and never starts it, which is setup.
// Its pkl is the real one. Its cache is a temporary folder of the test, with a stand-in for the pinned compiler
// in the compiler's place: a file that is no program. So setup finds its compiler without a download, and keeps
// its copy for the editor in that cache, not in the user's, whose bin folder is on the PATH.
//
// No `yue` is on the PATH of this world, whatever the machine has. The stand-in, and each program a test names
// as one more, reports the pinned version when it is asked. Any other program, a download, and a start of the
// game fail the test.
type seededWorld struct {
	outside  world
	cache    string // the cache folder
	compiler string // the stand-in for the pinned compiler, in the cache
}

// seeded is a seeded world. standIns are the programs that report the pinned compiler's version beside the one
// in the cache: what a test writes as a manifest's yue.path. It needs pkl, and a platform that Moonwell pins a
// compiler for: the cache has a place for no other.
func seeded(t *testing.T, standIns ...string) seededWorld {
	t.Helper()
	testkit.NeedPkl(t)
	asset, pinned := toolchain.YueScript.Versions[toolchain.YueVersion][env.CurrentPlatform()]
	if !pinned {
		t.Skip("Moonwell pins no compiler for this platform")
	}
	cache := t.TempDir()
	place := filepath.Join(cache, toolchain.YueScript.Name, toolchain.YueVersion)
	compiler := testkit.WriteFile(t, place, asset.Binary, []byte("A stand-in for the compiler.\n"))
	reporting := append([]string{compiler}, standIns...)
	runs := func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		asked := slices.Equal(args, toolchain.YueScript.VersionArgs)
		switch {
		case program == toolchain.Pkl.Name:
			return env.Run(ctx, program, args, options)
		case asked && program == toolchain.YueScript.Name:
			return env.RunResult{}, env.SpawnError(program, exec.ErrNotFound, options.Hint, "")
		case asked && slices.Contains(reporting, program):
			return env.RunResult{Stdout: "Yuescript version: " + toolchain.YueVersion + "\n"}, nil
		}
		t.Errorf("the seeded world has no stand-in for the program: %s %q", program, args)
		return env.RunResult{}, errors.New("no stand-in for " + program)
	}
	outside := func(root string, log *env.Logger) *env.Env {
		e, _ := testkit.Env(t, root)
		e.Log, e.CacheDir, e.Run = log, cache, runs
		return e
	}
	return seededWorld{outside: outside, cache: cache, compiler: compiler}
}

// ok runs a command line that must succeed, in this world.
func (w seededWorld) ok(t *testing.T, root string, args ...string) outcome {
	t.Helper()
	return endedWith(t, carriedIn(background, w.outside, root, args...), 0, args)
}

// fails runs a command line that must exit with 1, printing every part of wanted, in this world.
func (w seededWorld) fails(t *testing.T, root string, wanted []string, args ...string) outcome {
	t.Helper()
	result := endedWith(t, carriedIn(background, w.outside, root, args...), 1, args)
	contains(t, result.output, wanted...)
	return result
}

// at is this world for root, with a log that keeps its lines: for a command that a test calls by its function.
func (w seededWorld) at(root string) (*env.Env, *testkit.Recorder) {
	log := testkit.NewRecorder()
	return w.outside(root, log.Logger), log
}

// binDir is the folder of this world's cache that setup keeps the copies for a shell and an editor in.
func (w seededWorld) binDir() string { return filepath.Join(w.cache, "bin") }

func contains(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%q is missing from:\n%s", part, text)
		}
	}
}

// exists reports whether a file or folder of root is there; path uses "/".
func exists(root, path string) bool { return fsx.Exists(filepath.Join(root, filepath.FromSlash(path))) }

// rowNamed is the row of the command table for a command that must be there.
func rowNamed(t *testing.T, name string) command {
	t.Helper()
	chosen, known := rowOf(commands, name)
	if !known {
		t.Fatalf("the command table has no %s", name)
	}
	return chosen
}

// project is a folder that is a project to the command line: it has a moonwell.pkl, and nothing else.
func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testkit.WriteFile(t, root, "moonwell.pkl", []byte("// A manifest that no test evaluates.\n"))
	return root
}

// read is the text of a file of root; path uses "/".
func read(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// write writes a file of root, with its folders; path uses "/".
func write(t *testing.T, root, path, content string) {
	t.Helper()
	testkit.WriteFile(t, root, path, []byte(content))
}

// edit replaces from, which must be there, with to in a file of root.
func edit(t *testing.T, root, path, from, to string) {
	t.Helper()
	content := read(t, root, path)
	if !strings.Contains(content, from) {
		t.Fatalf("%s does not contain %q:\n%s", path, from, content)
	}
	write(t, root, path, strings.Replace(content, from, to, 1))
}

// appendTo adds text to the end of a file of root.
func appendTo(t *testing.T, root, path, more string) {
	t.Helper()
	write(t, root, path, read(t, root, path)+more)
}

// remove removes a file or a folder of root, with all that is in it.
func remove(t *testing.T, root, path string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
}

// asError is err as the expected failure it must be.
func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}

// realWorld is the real world for root, with a log that keeps its lines and prints none.
func realWorld(root string) (*env.Env, *testkit.Recorder) {
	log := testkit.NewRecorder()
	return env.New(root, log.Logger), log
}

// newProject is a project that this package's init makes, linked to the schema of this checkout, in a folder
// named name below a new temporary folder. It runs the real pkl, and needs it; its world lets pkl alone run, so
// it downloads nothing.
func newProject(t *testing.T, name string) string {
	t.Helper()
	testkit.NeedPkl(t)
	parent := t.TempDir()
	e, _, _ := pklOnly(t, parent)
	if err := createProject(background, e, name, filepath.Join(testkit.RepoRoot(t), "schema")); err != nil {
		t.Fatal(diag.Format(err))
	}
	return filepath.Join(parent, name)
}

// compiling is a project that init made, for a test that compiles it with the real compiler: it needs pkl and
// the compiler, which tooltest.Yue leaves in the user's cache, where a line in the real world finds it. Such a
// test takes a second or more, and is skipped with -short.
func compiling(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("the test compiles a project with the real compiler, which takes its time: not with -short")
	}
	tooltest.Yue(t)
	return newProject(t, "my-map")
}

// ownCache is a cache folder of the test's own that holds the pinned compiler at the place Moonwell looks for it,
// copied from where tooltest.Yue finds one. A program whose MOONWELL_CACHE names the folder downloads no
// compiler, and keeps what it writes for a shell and an editor out of the user's cache, whose bin folder is on
// the PATH. It needs the compiler, and a platform that Moonwell pins one for: the cache has a place for no other.
func ownCache(t *testing.T) string {
	t.Helper()
	compiler := tooltest.Yue(t)
	asset, pinned := toolchain.YueScript.Versions[toolchain.YueVersion][env.CurrentPlatform()]
	if !pinned {
		t.Skip("Moonwell pins no compiler for this platform")
	}
	program, err := os.ReadFile(compiler)
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	place := filepath.Join(cache, toolchain.YueScript.Name, toolchain.YueVersion, filepath.FromSlash(asset.Binary))
	if err := os.MkdirAll(filepath.Dir(place), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(place, program, 0o777); err != nil {
		t.Fatal(err)
	}
	return cache
}

// pklAlone is the world of pklOnly for a whole command line: the log is the line's own, so what a command says
// is printed and kept as in the real world.
func pklAlone(t *testing.T) world {
	return func(root string, log *env.Logger) *env.Env {
		e, _, _ := pklOnly(t, root)
		e.Log = log
		return e
	}
}

// okWithPklAlone runs a command line that must succeed, in a world that lets pkl alone run: a line that
// evaluates a manifest and needs no compiler. Whatever pkl the machine has, the line downloads nothing.
func okWithPklAlone(t *testing.T, root string, args ...string) outcome {
	t.Helper()
	return endedWith(t, carriedIn(background, pklAlone(t), root, args...), 0, args)
}

// failsWithPklAlone runs a command line that must exit with 1, printing every part of wanted, in a world that
// lets pkl alone run.
func failsWithPklAlone(t *testing.T, root string, wanted []string, args ...string) outcome {
	t.Helper()
	result := endedWith(t, carriedIn(background, pklAlone(t), root, args...), 1, args)
	contains(t, result.output, wanted...)
	return result
}

// pklOnly is a world for root in which pkl is the one program that runs: every other program, every download
// and every start of the game is refused with a failure that says "tried to". Its cache is a new folder, so no
// compiler is there. So a command that ends well in it needs no compiler and downloads nothing; one that needs
// the compiler fails at its download, which toolchain words, with the refusal as the failure's cause. ran lists
// the programs that were asked for, the refused among them.
func pklOnly(t *testing.T, root string) (e *env.Env, log *testkit.Recorder, ran func() []string) {
	t.Helper()
	e, log = testkit.Env(t, root)
	var guard sync.Mutex
	var programs []string
	e.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		guard.Lock()
		programs = append(programs, program)
		guard.Unlock()
		if program != toolchain.Pkl.Name {
			return env.RunResult{}, &diag.Error{Msg: "tried to run " + program}
		}
		return env.Run(ctx, program, args, options)
	}
	e.Fetch = func(_ context.Context, url string) (int, []byte, error) {
		return 0, nil, &diag.Error{Msg: "tried to download " + url}
	}
	e.Spawn = func(program string, _ []string) error {
		return &diag.Error{Msg: "tried to start " + program}
	}
	return e, log, func() []string {
		guard.Lock()
		defer guard.Unlock()
		return slices.Clone(programs)
	}
}

// commandIn runs the command of this name, by its row of the table, in the world e, with the arguments of its
// own that are given. It returns what the command printed for other programs, and the failure it ended with. A
// command the table does not have ends the test.
func commandIn(t *testing.T, ctx context.Context, e *env.Env, name string, arguments ...string) ([]string, error) {
	t.Helper()
	var printed []string
	said := line{words: append([]string{name}, arguments...)}
	err := rowNamed(t, name).run(ctx, e, call{said: said, print: func(text string) { printed = append(printed, text) }})
	return printed, err
}

// logged runs the command of this name in the world e, which must end well, and returns the lines log holds
// then: every line the world has logged.
func logged(t *testing.T, e *env.Env, log *testkit.Recorder, name string, arguments ...string) []string {
	t.Helper()
	if _, err := commandIn(t, background, e, name, arguments...); err != nil {
		t.Fatalf("moonwell %s failed:\n%s\nafter it logged %q", name, diag.Format(err), log.Lines())
	}
	return log.Lines()
}

// onlyPkl is what a pklOnly world ran, which must be pkl, at least once, and no other program.
func onlyPkl(t *testing.T, ran func() []string) {
	t.Helper()
	programs := ran()
	if len(programs) == 0 || slices.ContainsFunc(programs, func(program string) bool { return program != "pkl" }) {
		t.Errorf("the command ran %q, want pkl alone", programs)
	}
}

// writeLocal replaces moonwell.local.pkl of root with one that amends the shared manifest by body.
func writeLocal(t *testing.T, root, body string) {
	t.Helper()
	write(t, root, "moonwell.local.pkl", "amends \"moonwell.pkl\"\n"+body+"\n")
}

// sameFiles compares two snapshots of a folder, for what: every file that is new, changed or gone fails the test.
func sameFiles(t *testing.T, before, after map[string][]byte, what string) {
	t.Helper()
	for name, data := range after {
		previous, was := before[name]
		if !was {
			t.Errorf("%s: %s is new", what, name)
		} else if string(previous) != string(data) {
			t.Errorf("%s: %s changed", what, name)
		}
	}
	for name := range before {
		if _, still := after[name]; !still {
			t.Errorf("%s: %s is gone", what, name)
		}
	}
}

// holdBuildLock takes the build lock of root for the rest of the test, as a build that runs beside it does.
func holdBuildLock(t *testing.T, root string) {
	t.Helper()
	release, err := build.Acquire(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	t.Cleanup(release)
}

// exampleLibrary is a folder that holds a library, outside any project: a Lua module, a YueScript module that
// imports it, a stale Lua file beside that one, and a Lua module that defines a global.
func exampleLibrary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "src/example/greet.lua",
		"local M = {}\nfunction M.hello(name)\n return \"Hello, \" .. name\nend\nreturn M\n")
	write(t, dir, "src/example/loud.yue",
		"import \"example.greet\"\n\nexport shout = (name) -> greet.hello(name)\\upper!\n")
	write(t, dir, "src/example/loud.lua", "return { shout = function() return \"stale\" end }\n")
	write(t, dir, "src/example/globals.lua", "function ExampleAdd(a, b)\n return a + b\nend\n")
	return dir
}

// useLibrary has the project at root take the library in a folder as its library "ex", on this machine alone,
// and has its entry use the library's modules.
func useLibrary(t *testing.T, root, library string) {
	t.Helper()
	appendTo(t, root, "moonwell.local.pkl",
		"\nlibraries { [\"ex\"] { path = \""+filepath.ToSlash(library)+"\"; dir = \"src\" } }\n")
	appendTo(t, root, "src/main.yue", "\nimport \"example.loud\"\nrequire \"example.globals\"\n"+
		"print loud.shout \"Moonwell\"\nprint ExampleAdd 1, 2\n")
}
