package testkit

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
)

type LogRecorder struct {
	*env.Logger
	mu    sync.Mutex
	lines []string
}

func NewLogRecorder() *LogRecorder {
	recorder := &LogRecorder{}
	recorder.Logger = env.NewLogger(recorder.record, "")
	return recorder
}

func (r *LogRecorder) record(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines = append(r.lines, line)
}

func (r *LogRecorder) Lines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.lines)
}

func Env(t testing.TB, root string) (*env.Env, *LogRecorder) {
	t.Helper()
	recorder := NewLogRecorder()
	return &env.Env{
		Root:     root,
		Log:      recorder.Logger,
		Run:      failingRun(t),
		Fetch:    failingFetch(t),
		Spawn:    failingSpawn(t),
		CacheDir: t.TempDir(),
		Platform: env.CurrentPlatform(),
	}, recorder
}

func failingRun(t testing.TB) env.RunFunc {
	return func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		t.Helper()
		t.Errorf("the test did not expect a program to run: %s %q", program, args)
		return env.RunResult{}, errors.New("env.Run was not replaced: " + program)
	}
}

func failingFetch(t testing.TB) env.FetchFunc {
	return func(ctx context.Context, url string) (int, []byte, error) {
		t.Helper()
		t.Errorf("the test did not expect a download: %s", url)
		return 0, nil, errors.New("env.Fetch was not replaced: " + url)
	}
}

func failingSpawn(t testing.TB) func(program string, args []string) error {
	return func(program string, args []string) error {
		t.Helper()
		t.Errorf("the test did not expect a program to start: %s %q", program, args)
		return errors.New("env.Spawn was not replaced: " + program)
	}
}
