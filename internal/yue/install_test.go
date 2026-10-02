package yue_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
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

// reports is a compiler that prints stdout whatever it is asked.
func reports(stdout string) proc.RunFunc {
	return func(context.Context, string, []string, proc.Options) (proc.Result, error) {
		return proc.Result{Stdout: stdout}, nil
	}
}

func versionRunner(version string) proc.RunFunc {
	return reports("Yuescript version: " + version + "\n")
}

// missing is a command that cannot be started.
func missing(_ context.Context, command string, _ []string, _ proc.Options) (proc.Result, error) {
	return proc.Result{}, proc.SpawnError(command, fs.ErrNotExist, "", "")
}

// installer serves a zip archive with a fake compiler of version 9.9.9 for Linux, and counts the downloads.
func installer(t *testing.T, sha string) (deps yue.InstallDeps, log *testkit.Recorder, fetches *int) {
	t.Helper()
	archive := testkit.MakeZip(t, []testkit.ZipEntry{{Name: "yue", Data: []byte("fake-binary"), Deflate: true}}, "")
	if sha == "" {
		sha = fsx.SHA256Hex(archive)
	}
	fetches = new(int)
	log = testkit.NewRecorder()
	deps = yue.InstallDeps{
		Fetch: func(_ context.Context, url string) (int, []byte, error) {
			*fetches++
			if url != "https://example.test/yue.zip" {
				t.Errorf("fetched %s", url)
			}
			return 200, slices.Clone(archive), nil
		},
		Run:       versionRunner("9.9.9"),
		CacheRoot: t.TempDir(),
		Platform:  "linux-x86_64",
		Known: map[string]map[string]yue.Asset{
			"9.9.9": {"linux-x86_64": {URL: "https://example.test/yue.zip", SHA256: sha, Archive: "zip", Binary: "yue"}},
		},
		Log: log.Logger,
	}
	return deps, log, fetches
}

func TestEnsureDownloadsVerifiesAndCachesTheCompilerOnce(t *testing.T) {
	deps, log, fetches := installer(t, "")
	binary, err := yue.Ensure(background, "9.9.9", nil, deps)
	if err != nil || binary != filepath.Join(deps.CacheRoot, "yue", "9.9.9", "yue") {
		t.Fatalf("Ensure = %q, %v", binary, err)
	}
	if data, _ := os.ReadFile(binary); string(data) != "fake-binary" {
		t.Errorf("the compiler holds %q", data)
	}
	if _, err := yue.Ensure(background, "9.9.9", nil, deps); err != nil || *fetches != 1 {
		t.Errorf("%d downloads, %v", *fetches, err)
	}
	if !slices.Equal(log.Lines, []string{"Downloading YueScript 9.9.9..."}) {
		t.Errorf("log = %q", log.Lines)
	}
	if left, _ := os.ReadDir(filepath.Join(deps.CacheRoot, "yue")); len(left) != 1 {
		t.Errorf("the staging folder was left behind: %v", left)
	}
}

func TestEnsureRejectsAChecksumMismatchAndInstallsNothing(t *testing.T) {
	deps, _, _ := installer(t, strings.Repeat("0", 64))
	_, err := yue.Ensure(background, "9.9.9", nil, deps)
	e := asError(t, err, "a wrong checksum")
	if !strings.HasPrefix(e.Msg, "YueScript download checksum mismatch (expected "+strings.Repeat("0", 64)+", got ") ||
		!strings.Contains(e.Hint, "yue.path") || fsx.Exists(filepath.Join(deps.CacheRoot, "yue", "9.9.9")) {
		t.Errorf("error = %+v", e)
	}
}

func TestEnsureListsKnownVersionsForAnUnknownOne(t *testing.T) {
	deps, _, _ := installer(t, "")
	_, err := yue.Ensure(background, "1.0.0", nil, deps)
	e := asError(t, err, "an unknown version")
	if e.Msg != "Unknown YueScript version 1.0.0. Known versions: 9.9.9." || e.File != "moonwell.pkl" ||
		e.Hint != "Use a known version, or set yue.path to a local compiler." {
		t.Errorf("error = %+v", e)
	}
}

func TestEnsureAsksForYuePathOnUnsupportedPlatforms(t *testing.T) {
	deps, _, _ := installer(t, "")
	deps.Platform = ""
	_, err := yue.Ensure(background, "9.9.9", nil, deps)
	e := asError(t, err, "no platform")
	if !strings.HasPrefix(e.Msg, "Moonwell cannot download YueScript for this platform (") ||
		e.Hint != "Build or install yue yourself and set yue.path in moonwell.local.pkl." {
		t.Errorf("error = %+v", e)
	}
}

func TestEnsureUsesYuePathAndWarnsOnAVersionMismatch(t *testing.T) {
	deps, log, fetches := installer(t, "")
	local := testkit.WriteFile(t, t.TempDir(), "yue", nil)
	deps.Run = versionRunner("0.1.0")
	binary, err := yue.Ensure(background, "9.9.9", &local, deps)
	if err != nil || binary != local || *fetches != 0 {
		t.Fatalf("Ensure = %q, %v", binary, err)
	}
	if want := []string{"warning: yue.path reports version 0.1.0, expected 9.9.9."}; !slices.Equal(log.Lines, want) {
		t.Errorf("log = %q", log.Lines)
	}
	gone := filepath.Join(t.TempDir(), "yue")
	_, err = yue.Ensure(background, "9.9.9", &gone, deps)
	if e := asError(t, err, "a missing yue.path"); e.Msg != "yue.path does not exist: "+gone || e.File != "moonwell.local.pkl" {
		t.Errorf("error = %+v", e)
	}
}

func TestEnsureRefusesADownloadThatFailsOrIsNotTheCompiler(t *testing.T) {
	deps, _, _ := installer(t, "")
	deps.Fetch = func(context.Context, string) (int, []byte, error) { return 0, nil, errors.New("offline") }
	_, err := yue.Ensure(background, "9.9.9", nil, deps)
	if e := asError(t, err, "offline"); e.Msg != "Downloading https://example.test/yue.zip failed." ||
		e.Hint != "Check your connection and retry, or set yue.path." {
		t.Errorf("error = %+v", e)
	}
	deps.Fetch = func(context.Context, string) (int, []byte, error) { return 500, nil, nil }
	_, err = yue.Ensure(background, "9.9.9", nil, deps)
	if e := asError(t, err, "a server failure"); e.Msg != "Downloading https://example.test/yue.zip failed with HTTP 500." ||
		e.Hint != "Retry later, or set yue.path in moonwell.local.pkl." {
		t.Errorf("error = %+v", e)
	}

	deps, _, _ = installer(t, "")
	deps.Run = versionRunner("9.9.8")
	_, err = yue.Ensure(background, "9.9.9", nil, deps)
	if e := asError(t, err, "another version"); e.Msg != "Downloaded compiler reports version 9.9.8, expected 9.9.9." {
		t.Errorf("error = %+v", e)
	}
	if left, _ := os.ReadDir(filepath.Join(deps.CacheRoot, "yue")); len(left) != 0 {
		t.Errorf("something was installed: %v", left)
	}

	other := testkit.MakeZip(t, []testkit.ZipEntry{{Name: "README", Data: []byte("no compiler")}}, "")
	deps, _, _ = installer(t, fsx.SHA256Hex(other))
	deps.Fetch = func(context.Context, string) (int, []byte, error) { return 200, other, nil }
	_, err = yue.Ensure(background, "9.9.9", nil, deps)
	if e := asError(t, err, "no compiler in the archive"); e.Msg != "The YueScript archive has no yue." {
		t.Errorf("error = %+v", e)
	}
}

func TestKnownVersionsPinTheDefaultForWindowsAndLinux(t *testing.T) {
	if yue.DefaultVersion != "0.34.2" ||
		yue.Known["0.34.2"]["windows-x86_64"].SHA256 != "367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf" ||
		yue.Known["0.34.2"]["linux-x86_64"].SHA256 != "fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d" {
		t.Errorf("Known = %+v", yue.Known)
	}
	for _, c := range [][3]string{
		{"windows", "amd64", "windows-x86_64"}, {"linux", "amd64", "linux-x86_64"}, {"darwin", "arm64", ""}, {"linux", "arm64", ""},
	} {
		if got := yue.Platform(c[0], c[1]); got != c[2] {
			t.Errorf("Platform(%s, %s) = %q", c[0], c[1], got)
		}
	}
}

func TestInstallBinCopiesTheCompilerOnceAndAgainWhenItChanges(t *testing.T) {
	cache := t.TempDir()
	source := testkit.WriteFile(t, t.TempDir(), "yue.exe", []byte("v1"))
	want := filepath.Join(cache, "bin", "yue.exe")
	if path, copied, err := yue.InstallBin(source, cache); err != nil || path != want || !copied {
		t.Fatalf("InstallBin = %q, %v, %v", path, copied, err)
	}
	if path, copied, err := yue.InstallBin(source, cache); err != nil || path != want || copied {
		t.Errorf("InstallBin again = %q, %v, %v", path, copied, err)
	}
	testkit.WriteFile(t, filepath.Dir(source), "yue.exe", []byte("v2"))
	if _, copied, err := yue.InstallBin(source, cache); err != nil || !copied {
		t.Errorf("InstallBin of a changed compiler = %v, %v", copied, err)
	}
	if data, _ := os.ReadFile(want); string(data) != "v2" {
		t.Errorf("the copy holds %q", data)
	}
}

func TestInstallBinReportsACopyItCannotReplace(t *testing.T) {
	cache := t.TempDir()
	source := testkit.WriteFile(t, t.TempDir(), "yue", []byte("v1"))
	// A folder where the copy goes.
	if err := os.MkdirAll(filepath.Join(cache, "bin", "yue"), 0o777); err != nil {
		t.Fatal(err)
	}
	_, _, err := yue.InstallBin(source, cache)
	e := asError(t, err, "a folder in the way")
	if !strings.HasPrefix(e.Msg, "Copying YueScript to "+filepath.Join(cache, "bin", "yue")+" failed: ") ||
		e.Hint != "Close VS Code (its YueScript extension runs this copy of yue), then run moonwell setup again." {
		t.Errorf("error = %+v", e)
	}
}

func TestOnPathTellsThePinnedVersionAnotherVersionAndAMissingYueApart(t *testing.T) {
	if version, found, err := yue.OnPath(background, reports("Yuescript version: 0.30.0\n")); version != "0.30.0" || !found || err != nil {
		t.Errorf("OnPath = %q, %v, %v", version, found, err)
	}
	if version, found, err := yue.OnPath(background, missing); version != "" || found || err != nil {
		t.Errorf("OnPath without yue = %q, %v, %v", version, found, err)
	}
	if _, found, err := yue.OnPath(background, reports("not a compiler")); found || err != nil {
		t.Errorf("OnPath of another program = %v, %v", found, err)
	}
	cancelled := func(context.Context, string, []string, proc.Options) (proc.Result, error) {
		return proc.Result{}, context.Canceled
	}
	if _, _, err := yue.OnPath(background, cancelled); err != context.Canceled {
		t.Errorf("OnPath of a cancelled run = %v", err)
	}
}

func TestPathCommandGivesAPowerShellCommandOnWindowsAndAProfileLineElsewhere(t *testing.T) {
	windows := yue.PathCommand(`C:\Users\me\AppData\Local\moonwell\bin`, "windows")
	want := `[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ` +
		`';C:\Users\me\AppData\Local\moonwell\bin', 'User')`
	if windows != want {
		t.Errorf("PathCommand = %s", windows)
	}
	linux := yue.PathCommand("/home/me/.cache/moonwell/bin", "linux")
	if linux != `echo 'export PATH="/home/me/.cache/moonwell/bin:$PATH"' >> ~/.profile` {
		t.Errorf("PathCommand = %s", linux)
	}
}

func TestPathCommandKeepsSpacesAndDoublesSingleQuotesInTheWindowsFolder(t *testing.T) {
	for folder, want := range map[string]string{
		`C:\Users\Jane Doe\AppData\Local\moonwell\bin`: `+ ';C:\Users\Jane Doe\AppData\Local\moonwell\bin', 'User')`,
		`C:\Users\O'Brien\AppData\Local\moonwell\bin`:  `+ ';C:\Users\O''Brien\AppData\Local\moonwell\bin', 'User')`,
	} {
		if got := yue.PathCommand(folder, "windows"); !strings.Contains(got, want) {
			t.Errorf("PathCommand(%s) = %s", folder, got)
		}
	}
}

func TestReportEditorToolsWarnsOnceAboutYueOnPathAndSaysNothingWhenItIsRight(t *testing.T) {
	const binDir = "/home/me/.cache/moonwell/bin"
	log := testkit.NewRecorder()
	if err := yue.ReportEditorTools(background, reports("Yuescript version: 0.30.0\n"), log.Logger, "0.34.2", binDir, "linux"); err != nil {
		t.Fatal(err)
	}
	want := "warning: yue on PATH is version 0.30.0; VS Code's YueScript extension needs YueScript 0.34.2 there. " +
		"Run this once in your shell, then open a new terminal and restart VS Code:\n  " + yue.PathCommand(binDir, "linux")
	if !slices.Equal(log.Lines, []string{want}) {
		t.Errorf("log = %q", log.Lines)
	}
	quiet := testkit.NewRecorder()
	if err := yue.ReportEditorTools(background, reports("Yuescript version: 0.34.2\n"), quiet.Logger, "0.34.2", binDir, "linux"); err != nil || len(quiet.Lines) != 0 {
		t.Errorf("log = %q, %v", quiet.Lines, err)
	}
}

func TestReportEditorToolsTellsWindowsUsersToRunTheCommandInPowerShell(t *testing.T) {
	const binDir = `C:\Users\me\AppData\Local\moonwell\bin`
	log := testkit.NewRecorder()
	if err := yue.ReportEditorTools(background, missing, log.Logger, "0.34.2", binDir, "windows"); err != nil {
		t.Fatal(err)
	}
	if len(log.Lines) != 1 || !strings.HasPrefix(log.Lines[0], "warning: yue is not on PATH; ") ||
		!strings.Contains(log.Lines[0], "Run this once in PowerShell, then open a new terminal and restart VS Code") ||
		!strings.HasSuffix(log.Lines[0], yue.PathCommand(binDir, "windows")) {
		t.Errorf("log = %q", log.Lines)
	}
}

func TestDirAsWrittenKeepsTheSeparatorsOfThePath(t *testing.T) {
	cases := map[string]string{
		"/opt/yue/0.34.2/yue": "/opt/yue/0.34.2",
		"/opt/yue//yue":       "/opt/yue",
		"/opt/yue/bin/":       "/opt/yue",
		"/yue":                "/",
		"yue":                 ".",
		"tools/yue":           "tools",
	}
	if runtime.GOOS == "windows" {
		cases["C:/Users/me/yue/0.34.2/yue.exe"] = "C:/Users/me/yue/0.34.2"
		cases[`C:\Users\me\yue\yue.exe`] = `C:\Users\me\yue`
		cases[`C:\Users/me\yue.exe`] = `C:\Users/me`
		cases["C:/yue.exe"] = "C:/"
		cases[`C:\yue.exe`] = `C:\`
	}
	for path, want := range cases {
		if got := yue.DirAsWritten(path); got != want {
			t.Errorf("DirAsWritten(%q) = %q, want %q", path, got, want)
		}
	}
}
