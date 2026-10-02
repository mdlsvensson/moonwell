package cli_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/cli"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/yuetest"
)

var background = context.Background()

// TestMain lets the test program stand in for two other programs of the detached-spawn test: a parent that spawns
// and exits, and its child that outlives it.
func TestMain(m *testing.M) {
	switch os.Getenv("MOONWELL_TEST_ROLE") {
	case "interrupt":
		processInterruptHelper()
		os.Exit(0)
	case "parent":
		os.Setenv("MOONWELL_TEST_ROLE", "child")
		if err := cli.SpawnDetached(os.Args[0], nil); err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	case "child":
		time.Sleep(time.Second)
		os.WriteFile(os.Getenv("MOONWELL_TEST_MARKER"), []byte("alive"), 0o666)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return e
}

// newEnv is the real world for root, with a logger that records its lines.
func newEnv(root string) (*pipeline.Env, *testkit.Recorder) {
	log := testkit.NewRecorder()
	return pipeline.NewEnv(root, log.Logger), log
}

// outcome is what a command line printed and how it ended.
type outcome struct {
	code int
	// output is what went to the terminal (stderr), stdout what was printed for other programs.
	output, stdout string
}

// run runs a command line in root, as the program does.
func run(root string, args ...string) outcome {
	var lines, printed []string
	code := cli.Run(background, args, root, func(line string) { lines = append(lines, line) }, func(text string) { printed = append(printed, text) })
	return outcome{code, strings.Join(lines, "\n"), strings.Join(printed, "\n")}
}

// ok runs a command line that must succeed.
func ok(t *testing.T, root string, args ...string) outcome {
	t.Helper()
	result := run(root, args...)
	if result.code != 0 {
		t.Fatalf("moonwell %s exited with %d:\n%s", strings.Join(args, " "), result.code, result.output)
	}
	return result
}

// fails runs a command line that must exit with 1, printing every part of wanted.
func fails(t *testing.T, root string, wanted []string, args ...string) outcome {
	t.Helper()
	result := run(root, args...)
	if result.code != 1 {
		t.Fatalf("moonwell %s exited with %d:\n%s", strings.Join(args, " "), result.code, result.output)
	}
	contains(t, result.output, wanted...)
	return result
}

func contains(t *testing.T, text string, parts ...string) {
	t.Helper()
	for _, part := range parts {
		if !strings.Contains(text, part) {
			t.Errorf("%q is missing from:\n%s", part, text)
		}
	}
}

// newProject creates a project with `init --link` on this checkout, in a folder named name below a new temporary
// folder. It needs pkl.
func newProject(t *testing.T, name string) string {
	t.Helper()
	testkit.NeedPkl(t)
	parent := t.TempDir()
	env, _ := newEnv(parent)
	root, err := cli.Init(background, env, filepath.Join(parent, name), cli.InitOptions{Link: true, Checkout: testkit.RepoRoot(t)})
	if err != nil {
		t.Fatal(err)
	}
	return root
}

// compiling is a project that commands compile: it needs pkl and the YueScript compiler.
func compiling(t *testing.T) string {
	t.Helper()
	yuetest.Need(t)
	return newProject(t, "my-map")
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

// edit replaces from, which must be there, with to in a file.
func edit(t *testing.T, root, path, from, to string) {
	t.Helper()
	content := read(t, root, path)
	if !strings.Contains(content, from) {
		t.Fatalf("%s does not contain %q:\n%s", path, from, content)
	}
	write(t, root, path, strings.Replace(content, from, to, 1))
}

// appendTo adds text to the end of a file.
func appendTo(t *testing.T, root, path, more string) {
	t.Helper()
	write(t, root, path, read(t, root, path)+more)
}

func exists(root, path string) bool { return fsx.Exists(filepath.Join(root, filepath.FromSlash(path))) }

func remove(t *testing.T, root, path string) {
	t.Helper()
	if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(path))); err != nil {
		t.Fatal(err)
	}
}

// writeLocal replaces moonwell.local.pkl with one that amends the shared manifest by body.
func writeLocal(t *testing.T, root, body string) {
	t.Helper()
	write(t, root, "moonwell.local.pkl", "amends \"moonwell.pkl\"\n"+body+"\n")
}

// pklOnly is an environment whose runner only lets pkl through, proving that a command needs no compiler and
// downloads nothing. It records the commands that ran.
func pklOnly(t *testing.T, root string) (env *pipeline.Env, log *testkit.Recorder, commands *[]string) {
	t.Helper()
	env, log = newEnv(root)
	commands = new([]string)
	env.Run = func(ctx context.Context, command string, args []string, options proc.Options) (proc.Result, error) {
		*commands = append(*commands, command)
		if command != "pkl" {
			return proc.Result{}, &diag.Error{Msg: "tried to run " + command}
		}
		return proc.Run(ctx, command, args, options)
	}
	env.Install.Fetch = func(context.Context, string) (int, []byte, error) {
		return 0, nil, &diag.Error{Msg: "tried to download yue"}
	}
	env.Install.Run = func(context.Context, string, []string, proc.Options) (proc.Result, error) {
		return proc.Result{}, &diag.Error{Msg: "tried to run yue"}
	}
	return env, log, commands
}

// archive opens the map a build packed.
func archive(t *testing.T, root string) *testkit.MPQ {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(root, "dist", "bin", "map.w3x"))
	if err != nil {
		t.Fatal(err)
	}
	opened, err := testkit.OpenMPQ(data)
	if err != nil {
		t.Fatal(err)
	}
	return opened
}

// packed reads a file of the packed map; found is false when the map has none of that name.
func packed(t *testing.T, opened *testkit.MPQ, name string) (data []byte, found bool) {
	t.Helper()
	data, found, err := opened.Read(name)
	if err != nil {
		t.Fatal(err)
	}
	return data, found
}

// sameFiles compares two snapshots of a folder.
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
