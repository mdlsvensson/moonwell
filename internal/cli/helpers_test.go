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

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
	"github.com/mdlsvensson/moonwell/internal/tooltest"
)

var background = context.Background()

type outcome struct {
	code           int
	output, stdout string
}

const mark = "\xe2\x80\xba"

func run(root string, args ...string) outcome {
	var lines, printed []string
	code := Run(background, args, root, func(line string) { lines = append(lines, line) },
		func(text string) { printed = append(printed, text) })
	return outcome{code, strings.Join(lines, "\n"), strings.Join(printed, "\n")}
}

func carried(t *testing.T, ctx context.Context, root string, args ...string) outcome {
	t.Helper()
	return carriedIn(ctx, standIn(t), root, args...)
}

func carriedIn(ctx context.Context, outside envFactory, root string, args ...string) outcome {
	var lines, printed []string
	code := runIn(ctx, outside, args, root, func(line string) { lines = append(lines, line) },
		func(text string) { printed = append(printed, text) })
	return outcome{code, strings.Join(lines, "\n"), strings.Join(printed, "\n")}
}

func standIn(t *testing.T) envFactory {
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
			t.Errorf("the test has no stand-in for the program: %s %q", program, args)
			return env.RunResult{}, errors.New("no stand-in for " + program)
		}
		return e
	}
}

func ok(t *testing.T, root string, args ...string) outcome {
	t.Helper()
	return endedWith(t, run(root, args...), 0, args)
}

func fails(t *testing.T, root string, wanted []string, args ...string) outcome {
	t.Helper()
	result := endedWith(t, run(root, args...), 1, args)
	contains(t, result.output, wanted...)
	return result
}

func endedWith(t *testing.T, result outcome, code int, args []string) outcome {
	t.Helper()
	if result.code != code {
		t.Fatalf("moonwell %s exited with %d:\n%s", strings.Join(args, " "), result.code, result.output)
	}
	return result
}

type seededWorld struct {
	outside  envFactory
	cache    string
	compiler string
}

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
			return env.RunResult{}, env.NewSpawnError(program, exec.ErrNotFound, options.Hint, "")
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

func (w seededWorld) ok(t *testing.T, root string, args ...string) outcome {
	t.Helper()
	return endedWith(t, carriedIn(background, w.outside, root, args...), 0, args)
}

func (w seededWorld) fails(t *testing.T, root string, wanted []string, args ...string) outcome {
	t.Helper()
	result := endedWith(t, carriedIn(background, w.outside, root, args...), 1, args)
	contains(t, result.output, wanted...)
	return result
}

func (w seededWorld) at(root string) (*env.Env, *testkit.LogRecorder) {
	log := testkit.NewLogRecorder()
	return w.outside(root, log.Logger), log
}

func (w seededWorld) binDir() string { return filepath.Join(w.cache, "bin") }

func contains(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%q is missing from:\n%s", part, text)
		}
	}
}

func exists(root, path string) bool { return fsx.Exists(filepath.Join(root, filepath.FromSlash(path))) }

func rowOf(table []command, name string) (command, bool) {
	at := slices.IndexFunc(table, func(row command) bool { return row.name == name })
	if at < 0 {
		return command{}, false
	}
	return table[at], true
}

func rowNamed(t *testing.T, name string) command {
	t.Helper()
	chosen, known := rowOf(commands, name)
	if !known {
		t.Fatalf("the command table has no %s", name)
	}
	return chosen
}

func project(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testkit.WriteFile(t, root, "moonwell.pkl", []byte("// A manifest that no test evaluates.\n"))
	return root
}

func read(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func write(t *testing.T, root, path, content string) {
	t.Helper()
	testkit.WriteFile(t, root, path, []byte(content))
}

func edit(t *testing.T, root, path, from, to string) {
	t.Helper()
	content := read(t, root, path)
	if !strings.Contains(content, from) {
		t.Fatalf("%s does not contain %q:\n%s", path, from, content)
	}
	write(t, root, path, strings.Replace(content, from, to, 1))
}

func appendTo(t *testing.T, root, path, more string) {
	t.Helper()
	write(t, root, path, read(t, root, path)+more)
}

func remove(t *testing.T, root, path string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
}

func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}

func realWorld(root string) (*env.Env, *testkit.LogRecorder) {
	log := testkit.NewLogRecorder()
	return env.New(root, log.Logger), log
}

func newProject(t *testing.T, name string) string {
	t.Helper()
	return projectIn(t, t.TempDir(), name)
}

func projectIn(t *testing.T, parent, name string) string {
	t.Helper()
	testkit.NeedPkl(t)
	e, _, _ := pklOnly(t, parent)
	if err := createProject(background, e, name, filepath.Join(testkit.RepoRoot(t), "schema")); err != nil {
		t.Fatal(diag.Format(err))
	}
	return filepath.Join(parent, name)
}

func compiling(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("the test compiles a project with the real compiler, which takes its time: not with -short")
	}
	tooltest.Yue(t)
	return newProject(t, "my-map")
}

func ownCache(t *testing.T) string {
	t.Helper()
	compiler := tooltest.Yue(t)
	if _, pinned := toolchain.YueScript.Versions[toolchain.YueVersion][env.CurrentPlatform()]; !pinned {
		t.Skip("Moonwell pins no compiler for this platform")
	}
	program, err := os.ReadFile(compiler)
	if err != nil {
		t.Fatal(err)
	}
	cache := t.TempDir()
	place := pinnedCompilerIn(cache)
	if err := os.MkdirAll(filepath.Dir(place), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(place, program, 0o777); err != nil {
		t.Fatal(err)
	}
	return cache
}

func pinnedCompilerIn(cache string) string {
	asset := toolchain.YueScript.Versions[toolchain.YueVersion][env.CurrentPlatform()]
	return filepath.Join(cache, toolchain.YueScript.Name, toolchain.YueVersion, filepath.FromSlash(asset.Binary))
}

func pklAlone(t *testing.T) envFactory {
	return func(root string, log *env.Logger) *env.Env {
		e, _, _ := pklOnly(t, root)
		e.Log = log
		return e
	}
}

func okWithPklAlone(t *testing.T, root string, args ...string) outcome {
	t.Helper()
	return endedWith(t, carriedIn(background, pklAlone(t), root, args...), 0, args)
}

func failsWithPklAlone(t *testing.T, root string, wanted []string, args ...string) outcome {
	t.Helper()
	result := endedWith(t, carriedIn(background, pklAlone(t), root, args...), 1, args)
	contains(t, result.output, wanted...)
	return result
}

func pklOnly(t *testing.T, root string) (e *env.Env, log *testkit.LogRecorder, ran func() []string) {
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

func commandIn(t *testing.T, ctx context.Context, e *env.Env, name string, arguments ...string) ([]string, error) {
	t.Helper()
	var printed []string
	said := commandArgs{arguments: arguments, writeStdout: func(text string) { printed = append(printed, text) }}
	err := rowNamed(t, name).run(ctx, e, said)
	return printed, err
}

func logged(t *testing.T, e *env.Env, log *testkit.LogRecorder, name string, arguments ...string) []string {
	t.Helper()
	if _, err := commandIn(t, background, e, name, arguments...); err != nil {
		t.Fatalf("moonwell %s failed:\n%s\nafter it logged %q", name, diag.Format(err), log.Lines())
	}
	return log.Lines()
}

func onlyPkl(t *testing.T, ran func() []string) {
	t.Helper()
	programs := ran()
	if len(programs) == 0 || slices.ContainsFunc(programs, func(program string) bool { return program != "pkl" }) {
		t.Errorf("the command ran %q, want pkl alone", programs)
	}
}

func writeLocal(t *testing.T, root, body string) {
	t.Helper()
	write(t, root, "moonwell.local.pkl", "amends \"moonwell.pkl\"\n"+body+"\n")
}

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

func holdBuildLock(t *testing.T, root string) {
	t.Helper()
	release, err := build.AcquireLock(root)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	t.Cleanup(release)
}

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

func useLibrary(t *testing.T, root, library string) {
	t.Helper()
	appendTo(t, root, "moonwell.local.pkl",
		"\nlibraries { [\"ex\"] { path = \""+filepath.ToSlash(library)+"\"; dir = \"src\" } }\n")
	appendTo(t, root, "src/main.yue", "\nimport \"example.loud\"\nrequire \"example.globals\"\n"+
		"print loud.shout \"Moonwell\"\nprint ExampleAdd 1, 2\n")
}
