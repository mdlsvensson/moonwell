package testkit

import (
	"context"
	"errors"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/env"
)

// Recorder is a logger that keeps its lines.
type Recorder struct {
	*env.Logger
	Lines []string
}

// NewRecorder returns a logger whose lines are kept in Lines instead of printed.
func NewRecorder() *Recorder {
	recorder := &Recorder{}
	recorder.Logger = env.NewLogger(func(line string) { recorder.Lines = append(recorder.Lines, line) }, "")
	return recorder
}

// Env is a test world for a project at root: its log is recorded, its cache is a temporary folder, and Run, Fetch
// and Spawn fail the test when they are called, until the test replaces them. After failing the test they return an
// error, for a TB that records the failure and goes on.
func Env(t testing.TB, root string) (*env.Env, *Recorder) {
	t.Helper()
	recorder := NewRecorder()
	return &env.Env{
		Root:     root,
		Log:      recorder.Logger,
		Run:      refusedRun(t),
		Fetch:    refusedFetch(t),
		Spawn:    refusedSpawn(t),
		CacheDir: t.TempDir(),
		Platform: env.CurrentPlatform(),
	}, recorder
}

// refusedRun is a Run that fails the test and names the program.
func refusedRun(t testing.TB) env.RunFunc {
	return func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		t.Helper()
		t.Fatalf("the test did not expect a program to run: %s %q", program, args)
		return env.RunResult{}, errors.New("env.Run was not replaced: " + program)
	}
}

// refusedFetch is a Fetch that fails the test and names the address.
func refusedFetch(t testing.TB) env.FetchFunc {
	return func(ctx context.Context, url string) (int, []byte, error) {
		t.Helper()
		t.Fatalf("the test did not expect a download: %s", url)
		return 0, nil, errors.New("env.Fetch was not replaced: " + url)
	}
}

// refusedSpawn is a Spawn that fails the test and names the program.
func refusedSpawn(t testing.TB) func(program string, args []string) error {
	return func(program string, args []string) error {
		t.Helper()
		t.Fatalf("the test did not expect a program to start: %s %q", program, args)
		return errors.New("env.Spawn was not replaced: " + program)
	}
}
