package testkit

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
)

// Recorder is a logger that keeps its lines. It takes lines from several goroutines at once, and Lines may be
// asked while they log.
type Recorder struct {
	*env.Logger
	guard sync.Mutex
	lines []string
}

// NewRecorder returns a logger whose lines are kept instead of printed.
func NewRecorder() *Recorder {
	recorder := &Recorder{}
	recorder.Logger = env.NewLogger(recorder.keep, "")
	return recorder
}

// keep is the logger's sink: it adds one line to the lines kept.
func (r *Recorder) keep(line string) {
	r.guard.Lock()
	defer r.guard.Unlock()
	r.lines = append(r.lines, line)
}

// Lines is the lines logged so far, in the order they came. The list is the caller's own.
func (r *Recorder) Lines() []string {
	r.guard.Lock()
	defer r.guard.Unlock()
	return slices.Clone(r.lines)
}

// Env is a test world for a project at root: its log is recorded, its cache is a temporary folder, and Run, Fetch
// and Spawn fail the test when they are called, until the test replaces them. They fail it without stopping it,
// and return an error: the code under test may call them from a goroutine of its own, and stopping a test from
// there ends that goroutine alone and leaves the test waiting for it.
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
		t.Errorf("the test did not expect a program to run: %s %q", program, args)
		return env.RunResult{}, errors.New("env.Run was not replaced: " + program)
	}
}

// refusedFetch is a Fetch that fails the test and names the address.
func refusedFetch(t testing.TB) env.FetchFunc {
	return func(ctx context.Context, url string) (int, []byte, error) {
		t.Helper()
		t.Errorf("the test did not expect a download: %s", url)
		return 0, nil, errors.New("env.Fetch was not replaced: " + url)
	}
}

// refusedSpawn is a Spawn that fails the test and names the program.
func refusedSpawn(t testing.TB) func(program string, args []string) error {
	return func(program string, args []string) error {
		t.Helper()
		t.Errorf("the test did not expect a program to start: %s %q", program, args)
		return errors.New("env.Spawn was not replaced: " + program)
	}
}
