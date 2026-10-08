package testkit

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
)

type Recorder struct {
	*env.Logger
	guard sync.Mutex
	lines []string
}

func NewRecorder() *Recorder {
	recorder := &Recorder{}
	recorder.Logger = env.NewLogger(recorder.keep, "")
	return recorder
}

func (r *Recorder) keep(line string) {
	r.guard.Lock()
	defer r.guard.Unlock()
	r.lines = append(r.lines, line)
}

func (r *Recorder) Lines() []string {
	r.guard.Lock()
	defer r.guard.Unlock()
	return slices.Clone(r.lines)
}

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

func refusedRun(t testing.TB) env.RunFunc {
	return func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		t.Helper()
		t.Errorf("the test did not expect a program to run: %s %q", program, args)
		return env.RunResult{}, errors.New("env.Run was not replaced: " + program)
	}
}

func refusedFetch(t testing.TB) env.FetchFunc {
	return func(ctx context.Context, url string) (int, []byte, error) {
		t.Helper()
		t.Errorf("the test did not expect a download: %s", url)
		return 0, nil, errors.New("env.Fetch was not replaced: " + url)
	}
}

func refusedSpawn(t testing.TB) func(program string, args []string) error {
	return func(program string, args []string) error {
		t.Helper()
		t.Errorf("the test did not expect a program to start: %s %q", program, args)
		return errors.New("env.Spawn was not replaced: " + program)
	}
}
