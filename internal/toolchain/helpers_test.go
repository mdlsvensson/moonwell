package toolchain

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

var background = context.Background()

func asDiagError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return diagErr
}

func fakeRunPrinting(stdout string) env.RunFunc {
	return func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{Stdout: stdout}, nil
	}
}

func fakeYueRun(version string) env.RunFunc {
	return fakeRunPrinting("Yuescript version: " + version + "\n")
}

func runNotFound(_ context.Context, program string, _ []string, options env.RunOptions) (env.RunResult, error) {
	return env.RunResult{}, env.NewSpawnError(program, fs.ErrNotExist, options.Hint, "")
}

func runInterrupted(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
	return env.RunResult{}, context.Canceled
}

func failingRun(t testing.TB) env.RunFunc {
	t.Helper()
	untouched, _ := testkit.Env(t, "")
	return untouched.Run
}

func newZip(t testing.TB, files ...string) []byte {
	t.Helper()
	var entries []testkit.ZipEntry
	for i := 0; i < len(files); i += 2 {
		entries = append(entries, testkit.ZipEntry{Name: files[i], Data: []byte(files[i+1]), Deflate: true})
	}
	return testkit.Zip(t, "", entries...)
}

func newEnv(t testing.TB, address string, body []byte) (e *env.Env, log *testkit.LogRecorder, fetches *int) {
	t.Helper()
	e, log = testkit.Env(t, t.TempDir())
	e.Platform = "linux-x86_64"
	fetches = new(int)
	e.Fetch = func(_ context.Context, url string) (int, []byte, error) {
		*fetches++
		if url != address {
			t.Errorf("fetched %s, want %s", url, address)
		}
		return 200, slices.Clone(body), nil
	}
	return e, log, fetches
}

func fetchOffline(context.Context, string) (int, []byte, error) { return 0, nil, errors.New("offline") }

func fetchStatus(code int) env.FetchFunc {
	return func(context.Context, string) (int, []byte, error) { return code, nil, nil }
}

const (
	yueAddress = "https://example.test/yue.zip"
	pklAddress = "https://example.test/pkl-linux-amd64"
)

func yueToolWith(asset Asset) Tool {
	tool := YueScript
	tool.Versions = map[string]map[string]Asset{"9.9.9": {"linux-x86_64": asset}}
	return tool
}

func newYueEnv(t testing.TB, sha string) (e *env.Env, log *testkit.LogRecorder, fetches *int, tool Tool) {
	t.Helper()
	archive := newZip(t, "yue", "fake-binary")
	if sha == "" {
		sha = fsx.SHA256Hex(archive)
	}
	e, log, fetches = newEnv(t, yueAddress, archive)
	e.Run = fakeYueRun("9.9.9")
	return e, log, fetches, yueToolWith(Asset{URL: yueAddress, SHA256: sha, Archive: "zip", Binary: "yue"})
}

func pinTool(t testing.TB, own *Tool, tool Tool) {
	t.Helper()
	kept := *own
	*own = tool
	t.Cleanup(func() { *own = kept })
}

func fakePklRun(onPath, downloaded string) env.RunFunc {
	return func(_ context.Context, program string, _ []string, options env.RunOptions) (env.RunResult, error) {
		switch {
		case program != "pkl":
			return env.RunResult{Stdout: downloaded}, nil
		case onPath == "":
			return env.RunResult{}, env.NewSpawnError(program, fs.ErrNotExist, options.Hint, "")
		}
		return env.RunResult{Stdout: onPath}, nil
	}
}

func newPklEnv(t testing.TB, sha string) (e *env.Env, log *testkit.LogRecorder, fetches *int) {
	t.Helper()
	executable := []byte("fake-pkl")
	if sha == "" {
		sha = fsx.SHA256Hex(executable)
	}
	e, log, fetches = newEnv(t, pklAddress, executable)
	e.Run = fakePklRun("", "Pkl "+PklVersion+" (Linux 6.8, native)\n")
	tool := Pkl
	tool.Versions = map[string]map[string]Asset{
		PklVersion: {"linux-x86_64": {URL: pklAddress, SHA256: sha, Binary: "pkl"}},
	}
	pinTool(t, &Pkl, tool)
	return e, log, fetches
}

func listDir(t testing.TB, dir string) []string {
	t.Helper()
	if !fsx.Exists(dir) {
		return nil
	}
	return slices.Sorted(maps.Keys(testkit.Snapshot(t, dir)))
}

const installPage = "https://pkl-lang.org/main/current/pkl-cli/index.html#installation"
