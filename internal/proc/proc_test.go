package proc

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

// TestMain lets the test binary stand in for a program: with MOONWELL_PROC_HELPER set it prints and exits.
func TestMain(m *testing.M) {
	if os.Getenv("MOONWELL_PROC_HELPER") == "1" {
		fmt.Println("out")
		fmt.Fprintln(os.Stderr, "err")
		wd, _ := os.Getwd()
		fmt.Println(filepath.Base(wd))
		os.Exit(3)
	}
	os.Exit(m.Run())
}

func TestRunCapturesOutputAndExitCode(t *testing.T) {
	t.Setenv("MOONWELL_PROC_HELPER", "1")
	dir := t.TempDir()
	result, err := Run(context.Background(), os.Args[0], nil, Options{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 3 || strings.TrimSpace(result.Stderr) != "err" {
		t.Errorf("result = %+v", result)
	}
	if lines := strings.Fields(result.Stdout); len(lines) != 2 || lines[0] != "out" || lines[1] != filepath.Base(dir) {
		t.Errorf("stdout = %q", result.Stdout)
	}
}

func TestRunReportsAMissingCommandWithTheHint(t *testing.T) {
	_, err := Run(context.Background(), "definitely-not-a-command-moonwell", nil, Options{Hint: "install it"})
	var e *diag.Error
	if !errors.As(err, &e) || e.Hint != "install it" ||
		e.Msg != "Cannot run 'definitely-not-a-command-moonwell': command not found." {
		t.Errorf("error = %+v", err)
	}
}

func TestRunReportsAnySpawnFailureWithTheHint(t *testing.T) {
	dir := t.TempDir()
	// A folder, and a file that is not a program.
	junk := filepath.Join(dir, "junk.exe")
	if err := os.WriteFile(junk, []byte("not a program"), 0o666); err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{dir, junk} {
		_, err := Run(context.Background(), command, nil, Options{Hint: "fix it"})
		var e *diag.Error
		if !errors.As(err, &e) || e.Hint != "fix it" || !strings.Contains(e.Msg, command) ||
			!strings.HasPrefix(e.Msg, "Cannot run '") || !strings.HasSuffix(e.Msg, ".") || strings.HasSuffix(e.Msg, "..") {
			t.Errorf("running %s: %+v", command, err)
		}
	}
}

func TestRunStopsWhenTheContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	t.Setenv("MOONWELL_PROC_HELPER", "1")
	if _, err := Run(ctx, os.Args[0], nil, Options{}); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}
