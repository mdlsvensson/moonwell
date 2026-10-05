package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// The test of this file runs the program itself, built from next/cmd/moonwell: it holds what Run cannot show,
// which is the working folder, the two streams, the exit code of a process, and Ctrl+C. It builds the program
// and compiles a project, which takes some seconds, and is skipped with -short. The program runs with a cache of
// the test's own, so it downloads nothing and writes nothing into the user's.

// The test program stands in for one other program, by the role its environment names.
const (
	testRole = "MOONWELL_CLI_TEST_ROLE"
	// interruptingRole is the sender of an interrupt, where a system needs a process of its own for one.
	interruptingRole = "interrupt"
	// interruptedPid names the process the sender interrupts.
	interruptedPid = "MOONWELL_CLI_TEST_PID"
)

// TestMain lets the test program be the sender of an interrupt, where the system needs a process of its own for
// that.
func TestMain(m *testing.M) {
	if os.Getenv(testRole) == interruptingRole {
		sendInterrupt()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// theProgram builds next/cmd/moonwell into a folder of the test, and returns the file.
func theProgram(t *testing.T) string {
	t.Helper()
	program := filepath.Join(t.TempDir(), "moonwell")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	build := exec.Command("go", "build", "-o", program, "./next/cmd/moonwell")
	build.Dir = testkit.RepoRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the program does not build: %v\n%s", err, output)
	}
	return program
}

// executable is the program as a test runs it: in a project folder, with a cache of the test's own.
type executable struct {
	program, root, cache string
}

// command is the program with these arguments, not yet started: it runs in the project folder, and ends with
// ctx at the latest.
func (s executable) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, s.program, args...)
	cmd.Dir = s.root
	cmd.Env = append(os.Environ(), "MOONWELL_CACHE="+s.cache)
	return cmd
}

// run runs the program to its end, and returns its exit code and what it wrote to each stream.
func (s executable) run(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(background, 2*time.Minute)
	defer cancel()
	cmd := s.command(ctx, args...)
	var out, log bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &log
	if err := cmd.Run(); err != nil {
		var exited *exec.ExitError
		if !errors.As(err, &exited) {
			t.Fatal(err)
		}
		code = exited.ExitCode()
	}
	return code, out.String(), log.String()
}

func TestMoonwellExecutable(t *testing.T) {
	root := compiling(t)
	s := executable{program: theProgram(t), root: root, cache: ownCache(t)}

	t.Run("the help goes to the terminal's stream", func(t *testing.T) {
		code, out, log := s.run(t, "--help")
		if code != 0 || out != "" {
			t.Fatalf("exit %d; stdout %q; stderr %q", code, out, log)
		}
		contains(t, log, "Usage: moonwell")
	})
	t.Run("an unknown command ends with 1", func(t *testing.T) {
		code, out, log := s.run(t, "unknown")
		if code != 1 || out != "" {
			t.Fatalf("exit %d; stdout %q; stderr %q", code, out, log)
		}
		contains(t, log, "Unknown command")
	})
	t.Run("objects:eval prints to the stream for programs, in the working folder", func(t *testing.T) {
		code, out, log := s.run(t, "objects:eval")
		if code != 0 || log != "" {
			t.Fatalf("exit %d; stdout %q; stderr %q", code, out, log)
		}
		units, _ := jsonObject(t, out)["units"].(map[string]any)
		if units["captain"] == nil || !strings.HasSuffix(out, "}\n") {
			t.Fatalf("stdout %q", out)
		}
	})
	t.Run("objects that are not valid go to the terminal's stream", func(t *testing.T) {
		write(t, root, "objects/bad.pkl", objectFile(`units { ["bad"] { id = "h001"; base = "zzzz" } }`))
		defer remove(t, root, "objects/bad.pkl")
		code, out, log := s.run(t, "objects:eval")
		if code != 1 || out != "" {
			t.Fatalf("exit %d; stdout %q; stderr %q", code, out, log)
		}
		contains(t, log, "objects/bad.pkl", "'zzzz' is not a standard unit")
	})
	t.Run("a build compiles with the compiler of the cache it is given", func(t *testing.T) {
		code, out, log := s.run(t, "build")
		if code != 0 || out != "" || strings.Contains(log, "Downloading") {
			t.Fatalf("exit %d; stdout %q; stderr %q", code, out, log)
		}
		contains(t, log, "Built dist/bin/map.w3x")
		if exists(root, "dist/.lock") {
			t.Fatal("a build left its lock behind")
		}
	})
	t.Run("one Ctrl+C ends dev with 130 and leaves no lock", func(t *testing.T) { interruptedDev(t, s) })
}

// interruptedDev starts dev, waits until it watches, saves a source that does not compile, waits for the check
// of it, and then sends the program one interrupt: dev must end with 130, with nothing on the stream for
// programs and no build lock left.
func interruptedDev(t *testing.T, s executable) {
	ctx, cancel := context.WithTimeout(background, 2*time.Minute)
	defer cancel()
	cmd := s.command(ctx, "dev")
	apartFromTheTest(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if cmd.ProcessState == nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	})
	lines := make(chan string, 256)
	go sendLines(stderr, lines)
	var seen []string
	await := func(wanted string) {
		t.Helper()
		for {
			select {
			case line, open := <-lines:
				if !open {
					t.Fatalf("dev ended before it said %q; it said:\n%s", wanted, strings.Join(seen, "\n"))
				}
				seen = append(seen, line)
				if strings.Contains(line, wanted) {
					return
				}
			case <-ctx.Done():
				t.Fatalf("dev never said %q; it said:\n%s", wanted, strings.Join(seen, "\n"))
			}
		}
	}
	await("Watching src/")
	write(t, s.root, "src/main.yue", "x = \n  if then\n")
	await("error: src/main.yue:")
	if err = interruptDev(cmd); err != nil {
		t.Fatal(err)
	}
	out, err := readAll(stdout)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("dev wrote to the stream for programs: %s", out)
	}
	err = cmd.Wait()
	var exited *exec.ExitError
	if !errors.As(err, &exited) || exited.ExitCode() != 130 {
		t.Fatalf("dev ended with %v after an interrupt, want exit code 130", err)
	}
	if exists(s.root, "dist/.lock") {
		t.Fatal("an interrupted dev left the build lock behind")
	}
}
