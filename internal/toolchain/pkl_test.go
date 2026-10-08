package toolchain

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const installPage = "https://pkl-lang.org/main/current/pkl-cli/index.html#installation"

func TestPklProgramUsesPklOnPathWhenItIs032OrNewer(t *testing.T) {
	for _, found := range []string{"Pkl 0.32.1 (Windows 10.0, native)", "Pkl 0.33.0 (Linux)", "Pkl 1.0.0 (Linux)"} {
		for _, platform := range []string{"linux-x86_64", ""} {
			e, log, fetches := pklInstaller(t, "")
			e.Platform = platform
			var asked []string
			e.Run = func(
				ctx context.Context, program string, args []string, options env.RunOptions,
			) (env.RunResult, error) {
				asked = append([]string{program}, args...)
				return pathAndPinned(found, "")(ctx, program, args, options)
			}
			program, err := FindPkl(background, e)
			if err != nil || program != "pkl" || *fetches != 0 || len(log.Lines()) != 0 {
				t.Errorf("%s on %q: PklProgram = %q, %v, %d downloads, log %q",
					found, platform, program, err, *fetches, log.Lines())
			}
			if !slices.Equal(asked, []string{"pkl", "--version"}) {
				t.Errorf("%s on %q: it ran %q, want pkl --version", found, platform, asked)
			}
		}
	}
}

func TestPklProgramDownloadsVerifiesAndCachesThePinnedPklOnceWhenPathHasNone(t *testing.T) {
	e, log, fetches := pklInstaller(t, "")
	program, err := FindPkl(background, e)
	want := filepath.Join(e.CacheDir, "pkl", PklVersion, "pkl")
	if err != nil || program != want {
		t.Fatalf("PklProgram = %q, %v", program, err)
	}
	if data, _ := os.ReadFile(program); string(data) != "fake-pkl" {
		t.Errorf("the executable holds %q", data)
	}
	if program, err := FindPkl(background, e); err != nil || program != want || *fetches != 1 {
		t.Errorf("again: %q, %d downloads, %v", program, *fetches, err)
	}
	if lines := log.Lines(); !slices.Equal(lines, []string{"Downloading Pkl " + PklVersion + "..."}) {
		t.Errorf("log = %q", lines)
	}
	if left := holds(t, e.CacheDir); !slices.Equal(left, []string{"pkl", "pkl/" + PklVersion, "pkl/" + PklVersion + "/pkl"}) {
		t.Errorf("the cache holds %q", left)
	}
}

func TestPklProgramWarnsAboutAnOlderPklOnPathAndUsesThePinnedOne(t *testing.T) {
	e, log, _ := pklInstaller(t, "")
	e.Run = pathAndPinned("Pkl 0.31.0 (Linux)\n", "Pkl "+PklVersion+" (Linux)")
	program, err := FindPkl(background, e)
	if err != nil || program != filepath.Join(e.CacheDir, "pkl", PklVersion, "pkl") {
		t.Fatalf("PklProgram = %q, %v", program, err)
	}
	want := []string{
		"warning: pkl on PATH is older than 0.32 (Pkl 0.31.0 (Linux)), so Moonwell runs its own Pkl " + PklVersion +
			". A pkl command you type yourself, such as `pkl project resolve`, still runs the old one.",
		"Downloading Pkl " + PklVersion + "...",
	}
	if lines := log.Lines(); !slices.Equal(lines, want) {
		t.Errorf("log = %q", lines)
	}
}

func TestPklProgramTakesAPklOnPathThatNamesNoVersionForAnOlderOne(t *testing.T) {
	for output, shown := range map[string]string{"pkl: no such flag\n": "(pkl: no such flag)", " \r\n": "(unknown)"} {
		e, log, fetches := pklInstaller(t, "")
		e.Run = pathAndPinned(output, "Pkl "+PklVersion+" (Linux)")
		program, err := FindPkl(background, e)
		if err != nil || program != filepath.Join(e.CacheDir, "pkl", PklVersion, "pkl") || *fetches != 1 {
			t.Errorf("%q: PklProgram = %q, %v, %d downloads", output, program, err, *fetches)
		}
		lines := log.Lines()
		if len(lines) != 2 || !strings.HasPrefix(lines[0], "warning: pkl on PATH is older than 0.32 "+shown+", ") {
			t.Errorf("%q: log = %q", output, lines)
		}
	}
}

func TestPklProgramAsksForAnInstallWhereItCannotDownloadPkl(t *testing.T) {
	const hint = "Install Pkl 0.32 or newer: " + installPage
	e, _, fetches := pklInstaller(t, "")
	e.Platform = ""
	_, err := FindPkl(background, e)
	if failure := asError(t, err, "no pkl"); failure.Msg != "Cannot run 'pkl': command not found." || failure.Hint != hint {
		t.Errorf("error = %+v", failure)
	}
	for found, message := range map[string]string{
		"Pkl 0.31.0 (Linux)": "Moonwell needs Pkl 0.32 or newer (found: Pkl 0.31.0 (Linux)).",
		"":                   "Moonwell needs Pkl 0.32 or newer (found: unknown).",
		" \t\r\n":            "Moonwell needs Pkl 0.32 or newer (found: unknown).",
		"pkl: no such flag":  "Moonwell needs Pkl 0.32 or newer (found: pkl: no such flag).",
	} {
		e.Run = prints(found)
		_, err := FindPkl(background, e)
		if failure := asError(t, err, found); failure.Msg != message || failure.Hint != hint || failure.File != "" {
			t.Errorf("error = %+v", failure)
		}
	}
	if *fetches != 0 {
		t.Errorf("%d downloads", *fetches)
	}
}

func TestAPklIs032OrNewerByItsFirstTwoNumbers(t *testing.T) {
	for version, want := range map[string]bool{
		"0.32.0": true, "0.32.1": true, "0.100.0": true, "1.0.0": true, "2.31.5": true,
		"0.31.9": false, "0.9.0": false, "0.0.32": false, "": false,
	} {
		if got := isSupportedPkl(version); got != want {
			t.Errorf("recentPkl(%q) = %v, want %v", version, got, want)
		}
	}
}

func TestPklProgramRejectsAChecksumMismatchAndInstallsNothing(t *testing.T) {
	e, _, _ := pklInstaller(t, strings.Repeat("0", 64))
	_, err := FindPkl(background, e)
	failure := asError(t, err, "a wrong checksum")
	if !strings.HasPrefix(failure.Msg, "Pkl download checksum mismatch (expected "+strings.Repeat("0", 64)+", got ") ||
		!strings.Contains(failure.Hint, "do not bypass the check") {
		t.Errorf("error = %+v", failure)
	}
	if left := holds(t, e.CacheDir); len(left) != 0 {
		t.Errorf("the cache holds %q", left)
	}
}

func TestPklProgramRefusesADownloadThatFailsOrIsNotThePinnedPkl(t *testing.T) {
	e, _, _ := pklInstaller(t, "")
	e.Fetch = offline
	_, err := FindPkl(background, e)
	if failure := asError(t, err, "offline"); failure.Msg != "Downloading https://example.test/pkl-linux-amd64 failed." ||
		failure.Hint != "Check your connection and retry, or install Pkl 0.32 or newer yourself: "+installPage {
		t.Errorf("error = %+v", failure)
	}
	e.Fetch = status(404)
	_, err = FindPkl(background, e)
	if failure := asError(t, err, "a server failure"); failure.Msg != "Downloading https://example.test/pkl-linux-amd64 failed with HTTP 404." ||
		failure.Hint != "Retry later, or install Pkl 0.32 or newer yourself: "+installPage {
		t.Errorf("error = %+v", failure)
	}

	e, _, _ = pklInstaller(t, "")
	e.Run = pathAndPinned("", "Pkl 0.32.0 (Linux)")
	_, err = FindPkl(background, e)
	if failure := asError(t, err, "another version"); failure.Msg != "Downloaded Pkl reports version 0.32.0, expected "+PklVersion+"." {
		t.Errorf("error = %+v", failure)
	}
	if left := holds(t, filepath.Join(e.CacheDir, "pkl")); len(left) != 0 {
		t.Errorf("something was installed: %q", left)
	}
}

func TestPklProgramPassesOnACancelledRun(t *testing.T) {
	e, _, fetches := pklInstaller(t, "")
	e.Run = interrupted
	if _, err := FindPkl(background, e); err != context.Canceled || *fetches != 0 {
		t.Errorf("PklProgram = %v, %d downloads", err, *fetches)
	}
}

func TestPklPinsVersion0321ForWindowsAndLinux(t *testing.T) {
	const releases = "https://github.com/apple/pkl/releases/download/0.32.1/"
	want := map[string]Asset{
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
	if PklVersion != "0.32.1" || Pkl.Name != "pkl" || len(Pkl.Versions) != 1 || len(Pkl.Versions[PklVersion]) != len(want) {
		t.Fatalf("PklVersion = %s, Pkl = %+v", PklVersion, Pkl)
	}
	for platform, asset := range want {
		if Pkl.Versions[PklVersion][platform] != asset {
			t.Errorf("the download for %s = %+v", platform, Pkl.Versions[PklVersion][platform])
		}
	}
}

func TestInstallBinCopiesPklOnceAndReportsACopyItCannotReplace(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	source := testkit.WriteFile(t, t.TempDir(), "pkl.exe", []byte("v1"))
	want := filepath.Join(e.CacheDir, "bin", "pkl.exe")
	if path, copied, err := CopyToBinDir(e, Pkl, source); err != nil || path != want || !copied {
		t.Fatalf("InstallBin = %q, %v, %v", path, copied, err)
	}
	if path, copied, err := CopyToBinDir(e, Pkl, source); err != nil || path != want || copied {
		t.Errorf("InstallBin again = %q, %v, %v", path, copied, err)
	}

	blocked, _ := testkit.Env(t, t.TempDir())
	if err := os.MkdirAll(filepath.Join(blocked.CacheDir, "bin", "pkl.exe"), 0o777); err != nil {
		t.Fatal(err)
	}
	_, _, err := CopyToBinDir(blocked, Pkl, source)
	failure := asError(t, err, "a folder in the way")
	if !strings.HasPrefix(failure.Msg, "Copying Pkl to "+filepath.Join(blocked.CacheDir, "bin", "pkl.exe")+" failed: ") ||
		!strings.HasSuffix(failure.Hint, "then run moonwell setup again.") {
		t.Errorf("error = %+v", failure)
	}
}

func TestKeepPklForShellCopiesThePinnedPklAndSaysWhenPathStillHasNone(t *testing.T) {
	e, log, _ := pklInstaller(t, "")
	pinned := testkit.WriteFile(t, e.CacheDir, "pkl/"+PklVersion+"/pkl", []byte("fake-pkl"))
	bin := filepath.Join(e.CacheDir, "bin")
	if err := CopyPklToBinDir(background, e, pinned, "windows"); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Copied Pkl " + PklVersion + " to " + filepath.Join(bin, "pkl") + ".",
		"warning: pkl is not on PATH, so a pkl command you type, such as `pkl project resolve`, finds no Pkl. " +
			"Run this once in PowerShell, then open a new terminal:\n  " + AddToPathCommand(bin, "windows"),
	}
	if lines := log.Lines(); !slices.Equal(lines, want) {
		t.Errorf("log = %q", lines)
	}
	if data, _ := os.ReadFile(filepath.Join(bin, "pkl")); string(data) != "fake-pkl" {
		t.Errorf("the copy holds %q", data)
	}

	e.Run = pathAndPinned("Pkl "+PklVersion+" (Linux)", "")
	if err := CopyPklToBinDir(background, e, pinned, "linux"); err != nil || len(log.Lines()) != len(want) {
		t.Errorf("log = %q, %v", log.Lines(), err)
	}
	untouched, silence := testkit.Env(t, t.TempDir())
	if err := CopyPklToBinDir(background, untouched, "pkl", "linux"); err != nil || len(silence.Lines()) != 0 ||
		len(holds(t, untouched.CacheDir)) != 0 {
		t.Errorf("KeepPklForShell of the pkl on PATH = %v, log %q", err, silence.Lines())
	}
}

func TestKeepPklForShellPassesOnACancellationAndACopyThatFails(t *testing.T) {
	e, log, _ := pklInstaller(t, "")
	pinned := testkit.WriteFile(t, e.CacheDir, "pkl/"+PklVersion+"/pkl", []byte("fake-pkl"))
	e.Run = interrupted
	if err := CopyPklToBinDir(background, e, pinned, "linux"); err != context.Canceled {
		t.Errorf("KeepPklForShell = %v, want the cancellation as it is", err)
	}
	if lines := log.Lines(); len(lines) != 1 || !strings.HasPrefix(lines[0], "Copied Pkl ") {
		t.Errorf("log = %q", lines)
	}
	e.Run = noProgram(t)
	gone := filepath.Join(e.CacheDir, "pkl", "0.0.0", "pkl")
	err := CopyPklToBinDir(background, e, gone, "linux")
	if failure := asError(t, err, "no pinned Pkl"); !strings.HasPrefix(failure.Msg, "Copying Pkl to ") {
		t.Errorf("error = %+v", failure)
	}
}
