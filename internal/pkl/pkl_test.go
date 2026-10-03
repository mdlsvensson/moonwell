package pkl_test

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/pkl"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

var background = context.Background()

func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return e
}

// machine answers `pkl --version` with onPath ("" for no pkl on PATH) and any other program, the downloaded one, with
// downloaded.
func machine(onPath, downloaded string) proc.RunFunc {
	return func(_ context.Context, command string, args []string, _ proc.Options) (proc.Result, error) {
		if command != pkl.OnPath {
			return proc.Result{Stdout: downloaded}, nil
		}
		if onPath == "" {
			return proc.Result{}, proc.SpawnError(command, fs.ErrNotExist, pkl.InstallHint, "")
		}
		return proc.Result{Stdout: onPath}, nil
	}
}

// installer serves a fake Pkl for Linux, finds no pkl on PATH, and counts the downloads.
func installer(t *testing.T, sha string) (deps pkl.Deps, log *testkit.Recorder, fetches *int) {
	t.Helper()
	executable := []byte("fake-pkl")
	if sha == "" {
		sha = fsx.SHA256Hex(executable)
	}
	fetches = new(int)
	log = testkit.NewRecorder()
	deps = pkl.Deps{
		Fetch: func(_ context.Context, url string) (int, []byte, error) {
			*fetches++
			if url != "https://example.test/pkl-linux-amd64" {
				t.Errorf("fetched %s", url)
			}
			return 200, slices.Clone(executable), nil
		},
		Run:       machine("", "Pkl "+pkl.Version+" (Linux 6.8, native)\n"),
		CacheRoot: t.TempDir(),
		Platform:  "linux-x86_64",
		Known:     map[string]pkl.Asset{"linux-x86_64": {URL: "https://example.test/pkl-linux-amd64", SHA256: sha, Binary: "pkl"}},
		Log:       log.Logger,
	}
	return deps, log, fetches
}

func TestEnsureUsesPklOnPathWhenItIs032OrNewer(t *testing.T) {
	for _, found := range []string{"Pkl 0.32.1 (Windows 10.0, native)", "Pkl 0.33.0 (Linux)", "Pkl 1.0.0 (Linux)"} {
		deps, log, fetches := installer(t, "")
		deps.Run = machine(found, "")
		if program, err := pkl.Ensure(background, deps); err != nil || program != "pkl" || *fetches != 0 || len(log.Lines) != 0 {
			t.Errorf("%s: Ensure = %q, %v, %d downloads, log %q", found, program, err, *fetches, log.Lines)
		}
	}
}

func TestEnsureDownloadsVerifiesAndCachesThePinnedPklOnceWhenPathHasNone(t *testing.T) {
	deps, log, fetches := installer(t, "")
	program, err := pkl.Ensure(background, deps)
	want := filepath.Join(deps.CacheRoot, "pkl", pkl.Version, "pkl")
	if err != nil || program != want {
		t.Fatalf("Ensure = %q, %v", program, err)
	}
	if data, _ := os.ReadFile(program); string(data) != "fake-pkl" {
		t.Errorf("the executable holds %q", data)
	}
	if program, err := pkl.Ensure(background, deps); err != nil || program != want || *fetches != 1 {
		t.Errorf("again: %q, %d downloads, %v", program, *fetches, err)
	}
	if !slices.Equal(log.Lines, []string{"Downloading Pkl " + pkl.Version + "..."}) {
		t.Errorf("log = %q", log.Lines)
	}
	if left, _ := os.ReadDir(filepath.Join(deps.CacheRoot, "pkl")); len(left) != 1 {
		t.Errorf("the staging folder was left behind: %v", left)
	}
}

func TestEnsureWarnsAboutAnOlderPklOnPathAndUsesThePinnedOne(t *testing.T) {
	deps, log, _ := installer(t, "")
	deps.Run = machine("Pkl 0.31.0 (Linux)\n", "Pkl "+pkl.Version+" (Linux)")
	program, err := pkl.Ensure(background, deps)
	if err != nil || program != filepath.Join(deps.CacheRoot, "pkl", pkl.Version, "pkl") {
		t.Fatalf("Ensure = %q, %v", program, err)
	}
	want := []string{
		"warning: pkl on PATH is older than 0.32 (Pkl 0.31.0 (Linux)), so Moonwell runs its own Pkl " + pkl.Version +
			". A pkl command you type yourself, such as `pkl project resolve`, still runs the old one.",
		"Downloading Pkl " + pkl.Version + "...",
	}
	if !slices.Equal(log.Lines, want) {
		t.Errorf("log = %q", log.Lines)
	}
}

func TestEnsureAsksForAnInstallWhereItCannotDownloadPkl(t *testing.T) {
	deps, _, fetches := installer(t, "")
	deps.Platform = ""
	_, err := pkl.Ensure(background, deps)
	if e := asError(t, err, "no pkl"); e.Msg != "Cannot run 'pkl': command not found." || e.Hint != pkl.InstallHint {
		t.Errorf("error = %+v", e)
	}
	for found, message := range map[string]string{
		"Pkl 0.31.0 (Linux)": "Moonwell needs Pkl 0.32 or newer (found: Pkl 0.31.0 (Linux)).",
		"":                   "Moonwell needs Pkl 0.32 or newer (found: unknown).",
	} {
		deps.Run = func(context.Context, string, []string, proc.Options) (proc.Result, error) {
			return proc.Result{Stdout: found}, nil
		}
		_, err := pkl.Ensure(background, deps)
		if e := asError(t, err, found); e.Msg != message || e.Hint != pkl.InstallHint {
			t.Errorf("error = %+v", e)
		}
	}
	if *fetches != 0 {
		t.Errorf("%d downloads", *fetches)
	}
}

func TestEnsureRejectsAChecksumMismatchAndInstallsNothing(t *testing.T) {
	deps, _, _ := installer(t, strings.Repeat("0", 64))
	_, err := pkl.Ensure(background, deps)
	e := asError(t, err, "a wrong checksum")
	if !strings.HasPrefix(e.Msg, "Pkl download checksum mismatch (expected "+strings.Repeat("0", 64)+", got ") ||
		!strings.Contains(e.Hint, "do not bypass the check") || fsx.Exists(filepath.Join(deps.CacheRoot, "pkl", pkl.Version)) {
		t.Errorf("error = %+v", e)
	}
}

func TestEnsureRefusesADownloadThatFailsOrIsNotThePinnedPkl(t *testing.T) {
	deps, _, _ := installer(t, "")
	deps.Fetch = func(context.Context, string) (int, []byte, error) { return 0, nil, errors.New("offline") }
	_, err := pkl.Ensure(background, deps)
	if e := asError(t, err, "offline"); e.Msg != "Downloading https://example.test/pkl-linux-amd64 failed." ||
		!strings.HasPrefix(e.Hint, "Check your connection and retry, or install Pkl 0.32 or newer yourself: https://") {
		t.Errorf("error = %+v", e)
	}
	deps.Fetch = func(context.Context, string) (int, []byte, error) { return 404, nil, nil }
	_, err = pkl.Ensure(background, deps)
	if e := asError(t, err, "a server failure"); e.Msg != "Downloading https://example.test/pkl-linux-amd64 failed with HTTP 404." ||
		!strings.HasPrefix(e.Hint, "Retry later, or install Pkl 0.32 or newer yourself: https://") {
		t.Errorf("error = %+v", e)
	}

	deps, _, _ = installer(t, "")
	deps.Run = machine("", "Pkl 0.32.0 (Linux)")
	_, err = pkl.Ensure(background, deps)
	if e := asError(t, err, "another version"); e.Msg != "Downloaded Pkl reports version Pkl 0.32.0 (Linux), expected "+pkl.Version+"." {
		t.Errorf("error = %+v", e)
	}
	if left, _ := os.ReadDir(filepath.Join(deps.CacheRoot, "pkl")); len(left) != 0 {
		t.Errorf("something was installed: %v", left)
	}
}

func TestEnsurePassesOnACancelledRun(t *testing.T) {
	deps, _, fetches := installer(t, "")
	deps.Run = func(context.Context, string, []string, proc.Options) (proc.Result, error) {
		return proc.Result{}, context.Canceled
	}
	if _, err := pkl.Ensure(background, deps); err != context.Canceled || *fetches != 0 {
		t.Errorf("Ensure = %v, %d downloads", err, *fetches)
	}
}

func TestKnownPinsPkl0321ForWindowsAndLinux(t *testing.T) {
	const releases = "https://github.com/apple/pkl/releases/download/0.32.1/"
	want := map[string]pkl.Asset{
		"windows-x86_64": {
			URL:    releases + "pkl-windows-amd64.exe",
			SHA256: "8550a00fcf027335e42c5e2cd553b88e98845408cb1880b3e3d1860caf46d22a",
			Binary: "pkl.exe",
		},
		"linux-x86_64": {
			URL:    releases + "pkl-linux-amd64",
			SHA256: "3180b62da95c0cad1d904e9bb6c5f4a8f9032413c21e53194bb91ff1ee5f3211",
			Binary: "pkl",
		},
	}
	if pkl.Version != "0.32.1" || len(pkl.Known) != len(want) {
		t.Fatalf("Version = %s, Known = %+v", pkl.Version, pkl.Known)
	}
	for platform, asset := range want {
		if pkl.Known[platform] != asset {
			t.Errorf("Known[%s] = %+v", platform, pkl.Known[platform])
		}
	}
}

func TestInstallBinCopiesPklOnceAndReportsACopyItCannotReplace(t *testing.T) {
	cache := t.TempDir()
	source := testkit.WriteFile(t, t.TempDir(), "pkl.exe", []byte("v1"))
	want := filepath.Join(cache, "bin", "pkl.exe")
	if path, copied, err := pkl.InstallBin(source, cache); err != nil || path != want || !copied {
		t.Fatalf("InstallBin = %q, %v, %v", path, copied, err)
	}
	if path, copied, err := pkl.InstallBin(source, cache); err != nil || path != want || copied {
		t.Errorf("InstallBin again = %q, %v, %v", path, copied, err)
	}

	blocked := t.TempDir()
	if err := os.MkdirAll(filepath.Join(blocked, "bin", "pkl.exe"), 0o777); err != nil {
		t.Fatal(err)
	}
	_, _, err := pkl.InstallBin(source, blocked)
	e := asError(t, err, "a folder in the way")
	if !strings.HasPrefix(e.Msg, "Copying Pkl to "+filepath.Join(blocked, "bin", "pkl.exe")+" failed: ") ||
		!strings.HasSuffix(e.Hint, "then run moonwell setup again.") {
		t.Errorf("error = %+v", e)
	}
}

func TestKeepForShellCopiesThePinnedPklAndSaysWhenPathStillHasNone(t *testing.T) {
	deps, log, _ := installer(t, "")
	pinned := testkit.WriteFile(t, filepath.Join(deps.CacheRoot, "pkl", pkl.Version), "pkl", []byte("fake-pkl"))
	bin := filepath.Join(deps.CacheRoot, "bin")
	if err := pkl.KeepForShell(background, pinned, deps, "windows"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Copied Pkl " + pkl.Version + " to " + filepath.Join(bin, "pkl") + ".",
		"warning: pkl is not on PATH, so a pkl command you type, such as `pkl project resolve`, finds no Pkl. " +
			"Run this once in PowerShell, then open a new terminal:\n  " + yue.PathCommand(bin, "windows"),
	}
	if !slices.Equal(log.Lines, want) {
		t.Errorf("log = %q", log.Lines)
	}

	// Once the folder is on PATH: no copy, no warning.
	quiet := testkit.NewRecorder()
	deps.Log = quiet.Logger
	deps.Run = machine("Pkl "+pkl.Version+" (Linux)", "")
	if err := pkl.KeepForShell(background, pinned, deps, "linux"); err != nil || len(quiet.Lines) != 0 {
		t.Errorf("log = %q, %v", quiet.Lines, err)
	}
	// Pkl on PATH is what Moonwell runs: nothing to keep.
	if err := pkl.KeepForShell(background, pkl.OnPath, pkl.Deps{}, "linux"); err != nil {
		t.Error(err)
	}
}

// The real download: the pinned executable of this machine, into the user's cache (where a later run finds it), run
// once to read its version.
func TestEnsureDownloadsTheRealPinnedPkl(t *testing.T) {
	testkit.NeedNetwork(t)
	if _, supported := pkl.Known[yue.CurrentPlatform()]; !supported {
		t.Skip("Moonwell downloads no Pkl for this platform")
	}
	noPkl := func(ctx context.Context, command string, args []string, options proc.Options) (proc.Result, error) {
		if command == pkl.OnPath {
			return proc.Result{}, proc.SpawnError(command, fs.ErrNotExist, "", "")
		}
		return proc.Run(ctx, command, args, options)
	}
	program, err := pkl.Ensure(background, pkl.Deps{
		Fetch:     library.HTTPFetch(http.DefaultClient),
		Run:       noPkl,
		CacheRoot: yue.DefaultCacheRoot(),
		Platform:  yue.CurrentPlatform(),
		Known:     pkl.Known,
		Log:       testkit.NewRecorder().Logger,
	})
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	result, err := proc.Run(background, program, []string{"--version"}, proc.Options{})
	if err != nil || !strings.HasPrefix(result.Stdout, "Pkl "+pkl.Version+" ") {
		t.Errorf("%s --version = %q, %v", program, result.Stdout, err)
	}
}
