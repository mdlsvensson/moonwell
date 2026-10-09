package toolchain

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestCompilerUsesYuePathAndWarnsOnAVersionMismatch(t *testing.T) {
	e, log, fetches, _ := newYueEnv(t, "")
	local := testkit.WriteFile(t, t.TempDir(), "yue", nil)
	e.Run = fakeYueRun("0.1.0")
	program, err := FindCompiler(background, e, "9.9.9", &local)
	if err != nil || program != local || *fetches != 0 {
		t.Fatalf("Compiler = %q, %v, %d downloads", program, err, *fetches)
	}
	want := []string{"warning: yue.path reports version 0.1.0, expected 9.9.9."}
	if lines := log.Lines(); !slices.Equal(lines, want) {
		t.Errorf("log = %q", lines)
	}
	gone := filepath.Join(t.TempDir(), "yue")
	_, err = FindCompiler(background, e, "9.9.9", &gone)
	if diagErr := asDiagError(t, err, "a missing yue.path"); diagErr.Msg != "yue.path does not exist: "+gone ||
		diagErr.File != manifest.UserFilePath(e) || !strings.Contains(diagErr.Hint, "yue.path") {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestCompilerTakesYuePathAsItIs(t *testing.T) {
	local := testkit.WriteFile(t, t.TempDir(), "yue", nil)
	tests := []struct {
		name    string
		run     env.RunFunc
		version string
		logs    []string
	}{
		{"the version asked for", fakeYueRun("9.9.9"), "9.9.9", nil},
		{"a version with no download", fakeYueRun("0.1.0"), "0.1.0", nil},
		{"a program that names no version", fakeRunPrinting("hello"), "9.9.9",
			[]string{"warning: yue.path reports version unknown, expected 9.9.9."}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, log := testkit.Env(t, t.TempDir())
			e.Platform = ""
			e.Run = tc.run
			program, err := FindCompiler(background, e, tc.version, &local)
			if err != nil || program != local || !slices.Equal(log.Lines(), tc.logs) {
				t.Errorf("Compiler = %q, %v, log %q", program, err, log.Lines())
			}
			if left := listDir(t, e.CacheDir); len(left) != 0 {
				t.Errorf("the cache holds %q", left)
			}
		})
	}
}

func TestCompilerPassesOnAYuePathThatCannotBeStartedOrIsInterrupted(t *testing.T) {
	local := testkit.WriteFile(t, t.TempDir(), "yue", nil)
	e, _ := testkit.Env(t, t.TempDir())
	e.Run = runNotFound
	_, err := FindCompiler(background, e, "9.9.9", &local)
	if diagErr := asDiagError(t, err, "a yue.path that is no program"); !strings.Contains(diagErr.Msg, "Cannot run '"+local+"'") ||
		diagErr.File != manifest.UserFilePath(e) || !strings.Contains(diagErr.Hint, "yue.path") || diagErr.Cause == nil {
		t.Errorf("error = %+v", diagErr)
	}
	e.Run = runInterrupted
	if _, err := FindCompiler(background, e, "9.9.9", &local); err != context.Canceled {
		t.Errorf("Compiler = %v, want the cancellation as it is", err)
	}
}

func TestCompilerWithoutYuePathEnsuresThePinnedCompiler(t *testing.T) {
	e, _, fetches, tool := newYueEnv(t, "")
	pinTool(t, &YueScript, tool)
	e.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		if program == "yue" {
			t.Errorf("yue on PATH was run")
		}
		return fakeYueRun("9.9.9")(ctx, program, args, options)
	}
	program, err := FindCompiler(background, e, "9.9.9", nil)
	if err != nil || program != filepath.Join(e.CacheDir, "yue", "9.9.9", "yue") || *fetches != 1 {
		t.Errorf("Compiler = %q, %v, %d downloads", program, err, *fetches)
	}
}

func TestPklProgramUsesPklOnPathWhenItIs032OrNewer(t *testing.T) {
	for _, found := range []string{"Pkl 0.32.1 (Windows 10.0, native)", "Pkl 0.33.0 (Linux)", "Pkl 1.0.0 (Linux)"} {
		for _, platform := range []string{"linux-x86_64", ""} {
			e, log, fetches := newPklEnv(t, "")
			e.Platform = platform
			var asked []string
			e.Run = func(
				ctx context.Context, program string, args []string, options env.RunOptions,
			) (env.RunResult, error) {
				asked = append([]string{program}, args...)
				return fakePklRun(found, "")(ctx, program, args, options)
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
	e, log, fetches := newPklEnv(t, "")
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
	if left := listDir(t, e.CacheDir); !slices.Equal(left, []string{"pkl", "pkl/" + PklVersion, "pkl/" + PklVersion + "/pkl"}) {
		t.Errorf("the cache holds %q", left)
	}
}

func TestPklProgramWarnsAboutAnOlderPklOnPathAndUsesThePinnedOne(t *testing.T) {
	e, log, _ := newPklEnv(t, "")
	e.Run = fakePklRun("Pkl 0.31.0 (Linux)\n", "Pkl "+PklVersion+" (Linux)")
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
		e, log, fetches := newPklEnv(t, "")
		e.Run = fakePklRun(output, "Pkl "+PklVersion+" (Linux)")
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
	e, _, fetches := newPklEnv(t, "")
	e.Platform = ""
	_, err := FindPkl(background, e)
	if diagErr := asDiagError(t, err, "no pkl"); diagErr.Msg != "Cannot run 'pkl': command not found." || diagErr.Hint != hint {
		t.Errorf("error = %+v", diagErr)
	}
	for found, message := range map[string]string{
		"Pkl 0.31.0 (Linux)": "Moonwell needs Pkl 0.32 or newer (found: Pkl 0.31.0 (Linux)).",
		"":                   "Moonwell needs Pkl 0.32 or newer (found: unknown).",
		" \t\r\n":            "Moonwell needs Pkl 0.32 or newer (found: unknown).",
		"pkl: no such flag":  "Moonwell needs Pkl 0.32 or newer (found: pkl: no such flag).",
	} {
		e.Run = fakeRunPrinting(found)
		_, err := FindPkl(background, e)
		if diagErr := asDiagError(t, err, found); diagErr.Msg != message || diagErr.Hint != hint || diagErr.File != "" {
			t.Errorf("error = %+v", diagErr)
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
	e, _, _ := newPklEnv(t, strings.Repeat("0", 64))
	_, err := FindPkl(background, e)
	diagErr := asDiagError(t, err, "a wrong checksum")
	if !strings.HasPrefix(diagErr.Msg, "Pkl download checksum mismatch (expected "+strings.Repeat("0", 64)+", got ") ||
		!strings.Contains(diagErr.Hint, "do not bypass the check") {
		t.Errorf("error = %+v", diagErr)
	}
	if left := listDir(t, e.CacheDir); len(left) != 0 {
		t.Errorf("the cache holds %q", left)
	}
}

func TestPklProgramRefusesADownloadThatFailsOrIsNotThePinnedPkl(t *testing.T) {
	e, _, _ := newPklEnv(t, "")
	e.Fetch = fetchOffline
	_, err := FindPkl(background, e)
	if diagErr := asDiagError(t, err, "offline"); diagErr.Msg != "Downloading https://example.test/pkl-linux-amd64 failed." ||
		diagErr.Hint != "Check your connection and retry, or install Pkl 0.32 or newer yourself: "+installPage {
		t.Errorf("error = %+v", diagErr)
	}
	e.Fetch = fetchStatus(404)
	_, err = FindPkl(background, e)
	if diagErr := asDiagError(t, err, "a server failure"); diagErr.Msg != "Downloading https://example.test/pkl-linux-amd64 failed with HTTP 404." ||
		diagErr.Hint != "Retry later, or install Pkl 0.32 or newer yourself: "+installPage {
		t.Errorf("error = %+v", diagErr)
	}

	e, _, _ = newPklEnv(t, "")
	e.Run = fakePklRun("", "Pkl 0.32.0 (Linux)")
	_, err = FindPkl(background, e)
	if diagErr := asDiagError(t, err, "another version"); diagErr.Msg != "Downloaded Pkl reports version 0.32.0, expected "+PklVersion+"." {
		t.Errorf("error = %+v", diagErr)
	}
	if left := listDir(t, filepath.Join(e.CacheDir, "pkl")); len(left) != 0 {
		t.Errorf("something was installed: %q", left)
	}
}

func TestPklProgramPassesOnACancelledRun(t *testing.T) {
	e, _, fetches := newPklEnv(t, "")
	e.Run = runInterrupted
	if _, err := FindPkl(background, e); err != context.Canceled || *fetches != 0 {
		t.Errorf("PklProgram = %v, %d downloads", err, *fetches)
	}
}
