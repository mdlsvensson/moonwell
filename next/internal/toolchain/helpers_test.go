package toolchain

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io/fs"
	"maps"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

var background = context.Background()

// asError is err as the expected failure it must be.
func asError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}

// prints is a machine on which every program prints stdout, whatever it is asked.
func prints(stdout string) env.RunFunc {
	return func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{Stdout: stdout}, nil
	}
}

// yueOf is a machine on which every program is a compiler of this version.
func yueOf(version string) env.RunFunc { return prints("Yuescript version: " + version + "\n") }

// missing is a machine on which no program can be started.
func missing(_ context.Context, program string, _ []string, options env.RunOptions) (env.RunResult, error) {
	return env.RunResult{}, env.SpawnError(program, fs.ErrNotExist, options.Hint, "")
}

// interrupted is a machine on which every program is stopped by a cancelled context.
func interrupted(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
	return env.RunResult{}, context.Canceled
}

// noProgram is a Run that fails the test when a program is run.
func noProgram(t testing.TB) env.RunFunc {
	t.Helper()
	untouched, _ := testkit.Env(t, "")
	return untouched.Run
}

// zipOf is a zip archive of entries, each a name and what the file holds, in the order given.
func zipOf(t testing.TB, entries ...string) []byte {
	t.Helper()
	var out bytes.Buffer
	writer := zip.NewWriter(&out)
	for i := 0; i < len(entries); i += 2 {
		file, err := writer.CreateHeader(&zip.FileHeader{Name: entries[i], Method: zip.Deflate})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := file.Write([]byte(entries[i+1])); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}

// world is a test world on Linux whose one download is body, at address. It counts the downloads, and no program
// can be run until the test replaces Run.
func world(t testing.TB, address string, body []byte) (e *env.Env, log *testkit.Recorder, fetches *int) {
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

// offline is a Fetch that gets no response.
func offline(context.Context, string) (int, []byte, error) { return 0, nil, errors.New("offline") }

// status is a Fetch whose response has a failing status.
func status(code int) env.FetchFunc {
	return func(context.Context, string) (int, []byte, error) { return code, nil, nil }
}

const (
	yueAddress = "https://example.test/yue.zip"
	pklAddress = "https://example.test/pkl-linux-amd64"
)

// yueWith is YueScript with one version, 9.9.9, whose one download is for Linux.
func yueWith(asset Asset) Tool {
	tool := YueScript
	tool.Versions = map[string]map[string]Asset{"9.9.9": {"linux-x86_64": asset}}
	return tool
}

// yueInstaller is a world that serves a zip with a compiler of version 9.9.9 for Linux, and that compiler as a
// tool. sha is the checksum the tool expects, "" for the archive's own.
func yueInstaller(t testing.TB, sha string) (e *env.Env, log *testkit.Recorder, fetches *int, tool Tool) {
	t.Helper()
	archive := zipOf(t, "yue", "fake-binary")
	if sha == "" {
		sha = fsx.SHA256Hex(archive)
	}
	e, log, fetches = world(t, yueAddress, archive)
	e.Run = yueOf("9.9.9")
	return e, log, fetches, yueWith(Asset{URL: yueAddress, SHA256: sha, Archive: "zip", Binary: "yue"})
}

// pin makes tool the package's own for the rest of the test: Compiler and PklProgram take no tool.
func pin(t testing.TB, own *Tool, tool Tool) {
	t.Helper()
	kept := *own
	*own = tool
	t.Cleanup(func() { *own = kept })
}

// pathAndPinned is a machine on which `pkl` on PATH prints onPath ("" for a PATH without pkl) and every other
// program, the downloaded one, prints downloaded.
func pathAndPinned(onPath, downloaded string) env.RunFunc {
	return func(_ context.Context, program string, _ []string, options env.RunOptions) (env.RunResult, error) {
		switch {
		case program != "pkl":
			return env.RunResult{Stdout: downloaded}, nil
		case onPath == "":
			return env.RunResult{}, env.SpawnError(program, fs.ErrNotExist, options.Hint, "")
		}
		return env.RunResult{Stdout: onPath}, nil
	}
}

// pklInstaller is a world that serves a Pkl for Linux and has no pkl on PATH; that Pkl is the package's own for
// the rest of the test. sha is the checksum expected, "" for the executable's own.
func pklInstaller(t testing.TB, sha string) (e *env.Env, log *testkit.Recorder, fetches *int) {
	t.Helper()
	executable := []byte("fake-pkl")
	if sha == "" {
		sha = fsx.SHA256Hex(executable)
	}
	e, log, fetches = world(t, pklAddress, executable)
	e.Run = pathAndPinned("", "Pkl "+PklVersion+" (Linux 6.8, native)\n")
	tool := Pkl
	tool.Versions = map[string]map[string]Asset{
		PklVersion: {"linux-x86_64": {URL: pklAddress, SHA256: sha, Binary: "pkl"}},
	}
	pin(t, &Pkl, tool)
	return e, log, fetches
}

// holds is every file and folder below folder, as paths from it with "/", sorted; none for a folder that is not
// there.
func holds(t testing.TB, folder string) []string {
	t.Helper()
	if !fsx.Exists(folder) {
		return nil
	}
	return slices.Sorted(maps.Keys(testkit.Snapshot(t, folder)))
}
