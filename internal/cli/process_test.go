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

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const (
	testRole         = "MOONWELL_CLI_TEST_ROLE"
	interruptingRole = "interrupt"
	interruptedPid   = "MOONWELL_CLI_TEST_PID"
)

func TestMain(m *testing.M) {
	if os.Getenv(testRole) == interruptingRole {
		sendInterrupt()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func buildProgram(t *testing.T) string {
	t.Helper()
	program := filepath.Join(t.TempDir(), "moonwell")
	if runtime.GOOS == "windows" {
		program += ".exe"
	}
	build := exec.Command("go", "build", "-o", program, "./cmd/moonwell")
	build.Dir = testkit.RepoRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("the program does not build: %v\n%s", err, output)
	}
	return program
}

type executable struct {
	program, root, cache string
}

func (s executable) command(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, s.program, args...)
	cmd.Dir = s.root
	cmd.Env = append(os.Environ(), "MOONWELL_CACHE="+s.cache)
	return cmd
}

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
	s := executable{program: buildProgram(t), root: root, cache: newCacheDir(t)}

	t.Run("the help is printed for other programs", func(t *testing.T) {
		code, out, log := s.run(t, "--help")
		if code != 0 || log != "" {
			t.Fatalf("exit %d; stdout %q; stderr %q", code, out, log)
		}
		checkContains(t, out, "Moonwell ", "assets:check")
	})
	t.Run("an unknown command ends with 1", func(t *testing.T) {
		code, out, log := s.run(t, "unknown")
		if code != 1 || out != "" {
			t.Fatalf("exit %d; stdout %q; stderr %q", code, out, log)
		}
		checkContains(t, log, "error: ", "unknown")
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
		writeFile(t, root, "objects/bad.pkl", objectFile(`units { ["bad"] { id = "h001"; base = "zzzz" } }`))
		defer removeFile(t, root, "objects/bad.pkl")
		code, out, log := s.run(t, "objects:eval")
		if code != 1 || out != "" {
			t.Fatalf("exit %d; stdout %q; stderr %q", code, out, log)
		}
		checkContains(t, log, "objects/bad.pkl", "'zzzz' is not a standard unit")
	})
	t.Run("a build compiles with the compiler of the cache it is given", func(t *testing.T) {
		code, out, log := s.run(t, "build")
		if code != 0 || out != "" || strings.Contains(log, "Downloading") {
			t.Fatalf("exit %d; stdout %q; stderr %q", code, out, log)
		}
		checkContains(t, log, "Built dist/bin/map.w3x")
		if exists(root, "dist/.lock") {
			t.Fatal("a build left its lock behind")
		}
	})
	t.Run("one Ctrl+C ends dev with 130 and leaves no lock", func(t *testing.T) { interruptedDev(t, s) })
}

func interruptedDev(t *testing.T, s executable) {
	ctx, cancel := context.WithTimeout(background, 2*time.Minute)
	defer cancel()
	cmd := s.command(ctx, "dev")
	detachFromTest(cmd)
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
	writeFile(t, s.root, "src/main.yue", "x = \n  if then\n")
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
