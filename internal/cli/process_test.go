package cli_test

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// A real executable checks what cli.Run cannot: the working directory, streams, process exit codes and interrupts.
func TestMoonwellExecutable(t *testing.T) {
	root := compiling(t)
	binary := filepath.Join(t.TempDir(), "moonwell")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./cmd/moonwell")
	build.Dir = testkit.RepoRoot(t)
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build executable: %v\n%s", err, output)
	}
	command := func(args ...string) (code int, stdout, stderr string) {
		t.Helper()
		ctx, cancel := context.WithTimeout(background, 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = root
		var out, log bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = &log
		err := cmd.Run()
		if err != nil {
			var exited *exec.ExitError
			if !errors.As(err, &exited) {
				t.Fatal(err)
			}
			code = exited.ExitCode()
		}
		return code, out.String(), log.String()
	}
	t.Run("help uses stderr", func(t *testing.T) {
		code, out, log := command("--help")
		if code != 0 || out != "" {
			t.Fatalf("%d %s %s", code, out, log)
		}
		contains(t, log, "Usage: moonwell")
	})
	t.Run("unknown command exits one", func(t *testing.T) {
		code, out, log := command("unknown")
		if code != 1 || out != "" {
			t.Fatalf("%d %s %s", code, out, log)
		}
		contains(t, log, "Unknown command")
	})
	t.Run("objects eval uses stdout and cwd", func(t *testing.T) {
		code, out, log := command("objects:eval")
		if code != 0 || log != "" {
			t.Fatalf("%d %s %s", code, out, log)
		}
		value := jsonObject(t, out)
		if value["units"].(map[string]any)["captain"] == nil {
			t.Fatal(value)
		}
	})
	t.Run("invalid objects use stderr", func(t *testing.T) {
		write(t, root, "objects/bad.pkl", objectFile(`units { ["bad"] { id = "h001"; base = "zzzz" } }`))
		defer remove(t, root, "objects/bad.pkl")
		code, out, log := command("objects:eval")
		if code != 1 || out != "" {
			t.Fatalf("%d %s %s", code, out, log)
		}
		contains(t, log, "objects/bad.pkl", "'zzzz' is not a standard unit")
	})
	t.Run("Ctrl+C stops dev and removes lock", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(background, 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "dev")
		cmd.Dir = root
		configureDevProcess(cmd)
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
		go readProcessLines(stderr, lines)
		var seen []string
		for {
			select {
			case line, open := <-lines:
				if !open {
					t.Fatalf("dev ended early: %s", strings.Join(seen, "\n"))
				}
				seen = append(seen, line)
				if strings.Contains(line, "Watching src/") {
					goto watching
				}
			case <-ctx.Done():
				t.Fatalf("dev never started: %s", strings.Join(seen, "\n"))
			}
		}
	watching:
		write(t, root, "src/main.yue", "x = \n  if then\n")
		for {
			select {
			case line, open := <-lines:
				if !open {
					t.Fatal("dev ended before checking save")
				}
				seen = append(seen, line)
				if strings.Contains(line, "error: src/main.yue:") {
					goto checked
				}
			case <-ctx.Done():
				t.Fatalf("dev never rechecked: %s", strings.Join(seen, "\n"))
			}
		}
	checked:
		if err = interruptDevProcess(cmd); err != nil {
			t.Fatal(err)
		}
		out, err := readAllProcess(stdout)
		if err != nil {
			t.Fatal(err)
		}
		if len(out) != 0 {
			t.Fatalf("dev stdout: %s", out)
		}
		err = cmd.Wait()
		var exited *exec.ExitError
		if !errors.As(err, &exited) || exited.ExitCode() != 130 {
			t.Fatalf("interrupt exit: %v", err)
		}
		if exists(root, "dist/.lock") {
			t.Fatal("interrupt left build lock")
		}
	})
}
