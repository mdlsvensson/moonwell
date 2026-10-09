package env

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

const (
	helperRole   = "MOONWELL_ENV_HELPER"
	helperMarker = "MOONWELL_ENV_MARKER"
)

func TestMain(m *testing.M) {
	switch os.Getenv(helperRole) {
	case "program":
		fmt.Println("out")
		fmt.Fprintln(os.Stderr, "err")
		wd, _ := os.Getwd()
		fmt.Println(filepath.Base(wd))
		os.Exit(3)
	case "bytes":
		os.Stdout.WriteString("\xEF\xBB\xBFa\xFFb")
		os.Stderr.WriteString("c\xC3")
		os.Exit(0)
	case "parent":
		os.Setenv(helperRole, "child")
		if err := SpawnDetached(os.Args[0], nil); err != nil {
			os.Stderr.WriteString(err.Error())
			os.Exit(1)
		}
		os.Exit(0)
	case "child":
		time.Sleep(time.Second)
		os.WriteFile(os.Getenv(helperMarker), []byte("alive"), 0o666)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func asDiagError(t *testing.T, err error) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v, want a *diag.Error", err)
	}
	return e
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestRunCapturesOutputAndExitCode(t *testing.T) {
	t.Setenv(helperRole, "program")
	dir := t.TempDir()
	result, err := Run(context.Background(), os.Args[0], nil, RunOptions{Dir: dir})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 3 || strings.TrimSpace(result.Stderr) != "err" {
		t.Errorf("result = %+v", result)
	}
	if lines := strings.Fields(result.Stdout); len(lines) != 2 || lines[0] != "out" || lines[1] != filepath.Base(dir) {
		t.Errorf("stdout = %q", result.Stdout)
	}
}

func TestRunDecodesOutputThatIsNotUTF8(t *testing.T) {
	t.Setenv(helperRole, "bytes")
	result, err := Run(context.Background(), os.Args[0], nil, RunOptions{})
	if err != nil {
		t.Fatal(err)
	}
	replacement := string(utf8.RuneError)
	if result.ExitCode != 0 || result.Stdout != "a"+replacement+"b" || result.Stderr != "c"+replacement {
		t.Errorf("stdout = %q, stderr = %q, code %d", result.Stdout, result.Stderr, result.ExitCode)
	}
}

func TestRunReportsAMissingProgramWithTheHint(t *testing.T) {
	const program = "definitely-not-a-program-moonwell"
	_, err := Run(context.Background(), program, nil, RunOptions{Hint: "install it"})
	e := asDiagError(t, err)
	if e.Hint != "install it" || e.File != "" || e.Cause == nil ||
		!strings.Contains(e.Msg, "'"+program+"'") || !strings.Contains(e.Msg, "command not found") {
		t.Errorf("error = %+v", e)
	}
}

func TestRunReportsAnySpawnFailureWithTheHint(t *testing.T) {
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk.exe")
	if err := os.WriteFile(junk, []byte("not a program"), 0o666); err != nil {
		t.Fatal(err)
	}
	for name, program := range map[string]string{"a folder": dir, "a file that is not a program": junk} {
		t.Run(name, func(t *testing.T) {
			_, err := Run(context.Background(), program, nil, RunOptions{Hint: "fix it"})
			e := asDiagError(t, err)
			if e.Hint != "fix it" || e.File != "" || !strings.HasPrefix(e.Msg, "Cannot run '"+program+"': ") ||
				!strings.HasSuffix(e.Msg, ".") || strings.HasSuffix(e.Msg, "..") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestRunStopsWhenTheContextIsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	t.Setenv(helperRole, "program")
	if _, err := Run(ctx, os.Args[0], nil, RunOptions{}); !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestSpawnErrorNamesTheProgramTheReasonTheHintAndTheFile(t *testing.T) {
	denied := &fs.PathError{Op: "fork/exec", Path: "game.exe", Err: errors.New("access is denied.")}
	cases := []struct {
		name   string
		cause  error
		reason string
	}{
		{"not on the path", &exec.Error{Name: "game.exe", Err: exec.ErrNotFound}, ": command not found."},
		{"no such file", &fs.PathError{Op: "fork/exec", Path: "game.exe", Err: fs.ErrNotExist}, ": command not found."},
		{"the system's reason, with one full stop", denied, ": access is denied."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := NewSpawnError("game.exe", c.cause, "check the path", "moonwell.local.pkl")
			if e.File != "moonwell.local.pkl" || e.Hint != "check the path" || !errors.Is(e, c.cause) ||
				!strings.HasPrefix(e.Msg, "Cannot run 'game.exe'") || !strings.HasSuffix(e.Msg, c.reason) ||
				strings.Contains(e.Msg, "fork/exec") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestSpawnDetachedKeepsTheChildRunningAfterTheProgramExits(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker.txt")
	parent := exec.Command(os.Args[0])
	parent.Env = append(os.Environ(), helperRole+"=parent", helperMarker+"="+marker)
	if output, err := parent.CombinedOutput(); err != nil {
		t.Fatalf("the parent failed: %v\n%s", err, output)
	}
	if exists(marker) {
		t.Fatal("the child finished before its parent exited, so the test shows nothing")
	}
	for deadline := time.Now().Add(10 * time.Second); !exists(marker); time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the detached child died with its parent")
		}
	}
}

func TestSpawnDetachedReportsAProgramThatCannotStart(t *testing.T) {
	if err := SpawnDetached(filepath.Join(t.TempDir(), "no-such-game.exe"), nil); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("error = %v, want one that is fs.ErrNotExist", err)
	}
}
