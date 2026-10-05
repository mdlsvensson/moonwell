package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
)

var background = context.Background()

// outcome is what a command line printed and how it ended.
type outcome struct {
	code int
	// output is what went to the terminal, stdout what was printed for other programs.
	output, stdout string
}

// run runs a command line in root, as the program does: in the real world. A test runs so only a line that
// starts no program and downloads nothing.
func run(root string, args ...string) outcome {
	return runWith(background, root, args...)
}

// runWith is run with the context given.
func runWith(ctx context.Context, root string, args ...string) outcome {
	var lines, printed []string
	code := Run(ctx, args, root, func(line string) { lines = append(lines, line) },
		func(text string) { printed = append(printed, text) })
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

// exists reports whether a file or folder of root is there; path uses "/".
func exists(root, path string) bool { return fsx.Exists(filepath.Join(root, filepath.FromSlash(path))) }

// world is a stand-in world for the folder root. Its log is recorded, and of all programs it answers one: pkl
// asked for its version, as a Pkl that is new enough. So a command gets as far as the manifest without a
// program started, and any other program fails the test.
func world(t *testing.T, root string) (*env.Env, *testkit.Recorder) {
	t.Helper()
	e, log := testkit.Env(t, root)
	e.Run = func(_ context.Context, program string, args []string, _ env.RunOptions) (env.RunResult, error) {
		if program == toolchain.Pkl.Name && slices.Equal(args, toolchain.Pkl.VersionArgs) {
			return env.RunResult{Stdout: "Pkl 0.32.1 (a stand-in)\n"}, nil
		}
		// Without stopping the test: a command may run a program from a goroutine of its own.
		t.Errorf("the test has no stand-in for the program: %s %q", program, args)
		return env.RunResult{}, errors.New("no stand-in for " + program)
	}
	return e, log
}

// carried carries out a command in a stand-in world for root, as Run does in the real one once it has read the
// line, and returns what the command logged and how it ended.
func carried(t *testing.T, ctx context.Context, root string, chosen command, said line) outcome {
	t.Helper()
	e, log := world(t, root)
	var printed []string
	code := carryOut(ctx, chosen, e, call{said: said, print: func(text string) { printed = append(printed, text) }})
	return outcome{code, strings.Join(log.Lines(), "\n"), strings.Join(printed, "\n")}
}

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
