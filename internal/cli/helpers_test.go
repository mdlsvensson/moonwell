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

type runResult struct {
	code           int
	output, stdout string
}

const mark = "\xe2\x80\xba"

func runCLI(root string, args ...string) runResult {
	var lines, printed []string
	code := Run(background, args, root, func(line string) { lines = append(lines, line) },
		func(text string) { printed = append(printed, text) })
	return runResult{code, strings.Join(lines, "\n"), strings.Join(printed, "\n")}
}

func runCLIWithContext(t *testing.T, ctx context.Context, root string, args ...string) runResult {
	t.Helper()
	return runCLIIn(ctx, fakeEnvFactory(t), root, args...)
}

func runCLIIn(ctx context.Context, outside envFactory, root string, args ...string) runResult {
	var lines, printed []string
	code := runIn(ctx, outside, args, root, func(line string) { lines = append(lines, line) },
		func(text string) { printed = append(printed, text) })
	return runResult{code, strings.Join(lines, "\n"), strings.Join(printed, "\n")}
}

func fakeEnvFactory(t *testing.T) envFactory {
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

func mustSucceed(t *testing.T, root string, args ...string) runResult {
	t.Helper()
	return checkExitCode(t, runCLI(root, args...), 0, args)
}

func mustFail(t *testing.T, root string, wanted []string, args ...string) runResult {
	t.Helper()
	result := checkExitCode(t, runCLI(root, args...), 1, args)
	checkContains(t, result.output, wanted...)
	return result
}

func checkExitCode(t *testing.T, result runResult, code int, args []string) runResult {
	t.Helper()
	if result.code != code {
		t.Fatalf("moonwell %s exited with %d:\n%s", strings.Join(args, " "), result.code, result.output)
	}
	return result
}

type fakeWorld struct {
	outside  envFactory
	cache    string
	compiler string
}

func newFakeWorld(t *testing.T, standIns ...string) fakeWorld {
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
	return fakeWorld{outside: outside, cache: cache, compiler: compiler}
}

func (w fakeWorld) mustSucceed(t *testing.T, root string, args ...string) runResult {
	t.Helper()
	return checkExitCode(t, runCLIIn(background, w.outside, root, args...), 0, args)
}

func (w fakeWorld) mustFail(t *testing.T, root string, wanted []string, args ...string) runResult {
	t.Helper()
	result := checkExitCode(t, runCLIIn(background, w.outside, root, args...), 1, args)
	checkContains(t, result.output, wanted...)
	return result
}

func (w fakeWorld) envAt(root string) (*env.Env, *testkit.LogRecorder) {
	log := testkit.NewLogRecorder()
	return w.outside(root, log.Logger), log
}

func (w fakeWorld) binDir() string { return filepath.Join(w.cache, "bin") }

func checkContains(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%q is missing from:\n%s", part, text)
		}
	}
}

func exists(root, path string) bool { return fsx.Exists(filepath.Join(root, filepath.FromSlash(path))) }

func findCommand(table []command, name string) (command, bool) {
	index := slices.IndexFunc(table, func(row command) bool { return row.name == name })
	if index < 0 {
		return command{}, false
	}
	return table[index], true
}

func mustFindCommand(t *testing.T, name string) command {
	t.Helper()
	chosen, known := findCommand(commands, name)
	if !known {
		t.Fatalf("the command table has no %s", name)
	}
	return chosen
}

func newTemplateProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	testkit.WriteFile(t, root, "moonwell.pkl", []byte("// A manifest that no test evaluates.\n"))
	return root
}

func readFile(t *testing.T, root, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func writeFile(t *testing.T, root, path, content string) {
	t.Helper()
	testkit.WriteFile(t, root, path, []byte(content))
}

func replaceInFile(t *testing.T, root, path, from, to string) {
	t.Helper()
	content := readFile(t, root, path)
	if !strings.Contains(content, from) {
		t.Fatalf("%s does not contain %q:\n%s", path, from, content)
	}
	writeFile(t, root, path, strings.Replace(content, from, to, 1))
}

func appendToFile(t *testing.T, root, path, more string) {
	t.Helper()
	writeFile(t, root, path, readFile(t, root, path)+more)
}

func removeFile(t *testing.T, root, path string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
}

func asDiagError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return diagErr
}

func newRealEnv(root string) (*env.Env, *testkit.LogRecorder) {
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
	e, _, _ := newPklOnlyEnv(t, parent)
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

func newCacheDir(t *testing.T) string {
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

func pklOnlyEnvFactory(t *testing.T) envFactory {
	return func(root string, log *env.Logger) *env.Env {
		e, _, _ := newPklOnlyEnv(t, root)
		e.Log = log
		return e
	}
}

func mustSucceedWithPklOnly(t *testing.T, root string, args ...string) runResult {
	t.Helper()
	return checkExitCode(t, runCLIIn(background, pklOnlyEnvFactory(t), root, args...), 0, args)
}

func mustFailWithPklOnly(t *testing.T, root string, wanted []string, args ...string) runResult {
	t.Helper()
	result := checkExitCode(t, runCLIIn(background, pklOnlyEnvFactory(t), root, args...), 1, args)
	checkContains(t, result.output, wanted...)
	return result
}

func newPklOnlyEnv(t *testing.T, root string) (e *env.Env, log *testkit.LogRecorder, ran func() []string) {
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

func runCommandIn(t *testing.T, ctx context.Context, e *env.Env, name string, arguments ...string) ([]string, error) {
	t.Helper()
	var printed []string
	said := commandArgs{arguments: arguments, writeStdout: func(text string) { printed = append(printed, text) }}
	err := mustFindCommand(t, name).run(ctx, e, said)
	return printed, err
}

func mustRunCommand(t *testing.T, e *env.Env, log *testkit.LogRecorder, name string, arguments ...string) []string {
	t.Helper()
	if _, err := runCommandIn(t, background, e, name, arguments...); err != nil {
		t.Fatalf("moonwell %s failed:\n%s\nafter it logged %q", name, diag.Format(err), log.Lines())
	}
	return log.Lines()
}

func checkOnlyPklRan(t *testing.T, ran func() []string) {
	t.Helper()
	programs := ran()
	if len(programs) == 0 || slices.ContainsFunc(programs, func(program string) bool { return program != "pkl" }) {
		t.Errorf("the command ran %q, want pkl alone", programs)
	}
}

func writeLocalManifest(t *testing.T, root, body string) {
	t.Helper()
	writeFile(t, root, "moonwell.local.pkl", "amends \"moonwell.pkl\"\n"+body+"\n")
}

func checkSameFiles(t *testing.T, before, after map[string][]byte, what string) {
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
	writeFile(t, dir, "src/example/greet.lua",
		"local M = {}\nfunction M.hello(name)\n return \"Hello, \" .. name\nend\nreturn M\n")
	writeFile(t, dir, "src/example/loud.yue",
		"import \"example.greet\"\n\nexport shout = (name) -> greet.hello(name)\\upper!\n")
	writeFile(t, dir, "src/example/loud.lua", "return { shout = function() return \"stale\" end }\n")
	writeFile(t, dir, "src/example/globals.lua", "function ExampleAdd(a, b)\n return a + b\nend\n")
	return dir
}

func useLibrary(t *testing.T, root, library string) {
	t.Helper()
	appendToFile(t, root, "moonwell.local.pkl",
		"\nlibraries { [\"ex\"] { path = \""+filepath.ToSlash(library)+"\"; dir = \"src\" } }\n")
	appendToFile(t, root, "src/main.yue", "\nimport \"example.loud\"\nrequire \"example.globals\"\n"+
		"print loud.shout \"Moonwell\"\nprint ExampleAdd 1, 2\n")
}
