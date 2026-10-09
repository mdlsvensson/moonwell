package toolchain

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestEnsureDownloadsVerifiesAndCachesTheCompilerOnce(t *testing.T) {
	e, log, fetches, tool := newYueEnv(t, "")
	program, err := Ensure(background, e, tool, "9.9.9")
	if err != nil || program != filepath.Join(e.CacheDir, "yue", "9.9.9", "yue") {
		t.Fatalf("Ensure = %q, %v", program, err)
	}
	if data, _ := os.ReadFile(program); string(data) != "fake-binary" {
		t.Errorf("the compiler holds %q", data)
	}
	if again, err := Ensure(background, e, tool, "9.9.9"); err != nil || again != program || *fetches != 1 {
		t.Errorf("again: %q, %d downloads, %v", again, *fetches, err)
	}
	if lines := log.Lines(); !slices.Equal(lines, []string{"Downloading YueScript 9.9.9..."}) {
		t.Errorf("log = %q", lines)
	}
	if left := listDir(t, e.CacheDir); !slices.Equal(left, []string{"yue", "yue/9.9.9", "yue/9.9.9/yue"}) {
		t.Errorf("the cache holds %q", left)
	}
}

func TestEnsureReturnsTheCachedProgramWithoutAskingTheWorld(t *testing.T) {
	e, log := testkit.Env(t, t.TempDir())
	e.Platform = "linux-x86_64"
	cached := testkit.WriteFile(t, e.CacheDir, "yue/9.9.9/yue", []byte("kept"))
	program, err := Ensure(background, e, yueToolWith(Asset{URL: yueAddress, Archive: "zip", Binary: "yue"}), "9.9.9")
	if err != nil || program != cached || len(log.Lines()) != 0 {
		t.Errorf("Ensure = %q, %v, log %q", program, err, log.Lines())
	}
}

func TestEnsureRejectsAChecksumMismatchAndInstallsNothing(t *testing.T) {
	e, _, _, tool := newYueEnv(t, strings.Repeat("0", 64))
	e.Run = failingRun(t)
	_, err := Ensure(background, e, tool, "9.9.9")
	diagErr := asDiagError(t, err, "a wrong checksum")
	if !strings.HasPrefix(diagErr.Msg, "YueScript download checksum mismatch (expected "+strings.Repeat("0", 64)+", got ") ||
		!strings.Contains(diagErr.Hint, "do not bypass the check") || !strings.Contains(diagErr.Hint, "yue.path") {
		t.Errorf("error = %+v", diagErr)
	}
	if left := listDir(t, e.CacheDir); len(left) != 0 {
		t.Errorf("the cache holds %q", left)
	}
}

func TestEnsureListsKnownVersionsForAnUnknownOne(t *testing.T) {
	e, _, fetches, tool := newYueEnv(t, "")
	_, err := Ensure(background, e, tool, "1.0.0")
	diagErr := asDiagError(t, err, "an unknown version")
	if diagErr.Msg != "Unknown YueScript version 1.0.0. Known versions: 9.9.9." || diagErr.File != "moonwell.pkl" ||
		!strings.HasPrefix(diagErr.Hint, "Use a known version, or ") || *fetches != 0 {
		t.Errorf("error = %+v, %d downloads", diagErr, *fetches)
	}
}

func TestTheKnownVersionsAreListedInByteOrder(t *testing.T) {
	e, _, _, tool := newYueEnv(t, "")
	for _, version := range []string{"10.0.0", "0.34.3", "9.10.0"} {
		tool.Versions[version] = nil
	}
	_, err := Ensure(background, e, tool, "1.0.0")
	if diagErr := asDiagError(t, err, "an unknown version"); !strings.HasSuffix(diagErr.Msg, "Known versions: 0.34.3, 10.0.0, 9.10.0, 9.9.9.") {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestEnsureAsksForYuePathOnUnsupportedPlatforms(t *testing.T) {
	e, _, fetches, tool := newYueEnv(t, "")
	e.Platform = ""
	_, err := Ensure(background, e, tool, "9.9.9")
	diagErr := asDiagError(t, err, "no platform")
	if !strings.HasPrefix(diagErr.Msg, "Moonwell cannot download YueScript for this platform (") ||
		diagErr.Hint != "Build or install yue yourself and set yue.path in moonwell.local.pkl." || *fetches != 0 {
		t.Errorf("error = %+v, %d downloads", diagErr, *fetches)
	}
}

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
		diagErr.File != "moonwell.local.pkl" || !strings.Contains(diagErr.Hint, "yue.path") {
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
		diagErr.File != "moonwell.local.pkl" || !strings.Contains(diagErr.Hint, "yue.path") || diagErr.Cause == nil {
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

func TestEnsureRefusesADownloadThatFailsOrIsNotTheCompiler(t *testing.T) {
	other := newZip(t, "README", "no compiler", "bin/yue", "in a folder", "yue.exe", "another name")
	tests := []struct {
		name    string
		fetch   env.FetchFunc
		run     env.RunFunc
		sha     string
		message string
		hint    string
	}{
		{name: "offline", fetch: fetchOffline,
			message: "Downloading https://example.test/yue.zip failed.", hint: "Check your connection and retry, or "},
		{name: "a server failure", fetch: fetchStatus(500),
			message: "Downloading https://example.test/yue.zip failed with HTTP 500.", hint: "Retry later, or "},
		{name: "a redirect that was not followed", fetch: fetchStatus(304),
			message: "Downloading https://example.test/yue.zip failed with HTTP 304.", hint: "Retry later, or "},
		{name: "another version", run: fakeYueRun("9.9.8"),
			message: "Downloaded YueScript reports version 9.9.8, expected 9.9.9."},
		{name: "a program that names no version", run: fakeRunPrinting("hello"),
			message: "Downloaded YueScript reports version unknown, expected 9.9.9."},
		{name: "no compiler in the archive", sha: fsx.SHA256Hex(other),
			fetch:   func(context.Context, string) (int, []byte, error) { return 200, other, nil },
			message: "The YueScript archive has no yue."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, _, _, tool := newYueEnv(t, tc.sha)
			e.Run = failingRun(t)
			if tc.run != nil {
				e.Run = tc.run
			}
			if tc.fetch != nil {
				e.Fetch = tc.fetch
			}
			_, err := Ensure(background, e, tool, "9.9.9")
			diagErr := asDiagError(t, err, tc.name)
			if diagErr.Msg != tc.message || diagErr.File != "" || !strings.HasPrefix(diagErr.Hint, tc.hint) ||
				(tc.hint == "") != (diagErr.Hint == "") {
				t.Errorf("error = %+v", diagErr)
			}
			if left := listDir(t, filepath.Join(e.CacheDir, "yue")); len(left) != 0 {
				t.Errorf("something was left in the cache: %q", left)
			}
		})
	}
}

func TestEnsurePassesOnACancellationAsItIs(t *testing.T) {
	t.Run("of the download", func(t *testing.T) {
		e, _, _, tool := newYueEnv(t, "")
		ctx, cancel := context.WithCancel(background)
		e.Fetch = func(ctx context.Context, _ string) (int, []byte, error) {
			cancel()
			return 0, nil, &url.Error{Op: "Get", URL: yueAddress, Err: ctx.Err()}
		}
		if _, err := Ensure(ctx, e, tool, "9.9.9"); err != context.Canceled {
			t.Errorf("Ensure = %v, want the cancellation as it is", err)
		}
	})
	t.Run("of the downloaded program", func(t *testing.T) {
		e, _, _, tool := newYueEnv(t, "")
		e.Run = runInterrupted
		if _, err := Ensure(background, e, tool, "9.9.9"); err != context.Canceled {
			t.Errorf("Ensure = %v, want the cancellation as it is", err)
		}
		if left := listDir(t, filepath.Join(e.CacheDir, "yue")); len(left) != 0 {
			t.Errorf("something was left in the cache: %q", left)
		}
	})
}

func TestReportedVersionFindsTheVersionInWhatAProgramPrints(t *testing.T) {
	tests := []struct {
		name           string
		tool           Tool
		stdout, stderr string
		want           string
	}{
		{"the compiler", YueScript, "Yuescript version: 0.34.3\n", "", "0.34.3"},
		{"the compiler, on stderr", YueScript, "", "Yuescript version: 0.34.3\n", "0.34.3"},
		{"the compiler, with a carriage return", YueScript, "Yuescript version: 0.34.3\r\n", "", "0.34.3"},
		{"a vertical tab ends the version", YueScript, "Yuescript version: 0.34.3\vx", "", "0.34.3"},
		{"a no-break space does not", YueScript, "Yuescript version: 0.34.3\xc2\xa0x y", "", "0.34.3\xc2\xa0x"},
		{"another program", YueScript, "not a compiler", "", ""},
		{"nothing", YueScript, "", "", ""},
		{"Pkl", Pkl, "Pkl 0.32.1 (Windows 10.0, native)\n", "", "0.32.1"},
		{"Pkl, its version on standard error", Pkl, "", "Pkl 0.32.1 (Linux 6.8, native)\n", "0.32.1"},
		{"Pkl without a third number", Pkl, "Pkl 0.32", "", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, _ := testkit.Env(t, t.TempDir())
			var asked []string
			e.Run = func(_ context.Context, program string, args []string, _ env.RunOptions) (env.RunResult, error) {
				asked = append([]string{program}, args...)
				return env.RunResult{Stdout: tc.stdout, Stderr: tc.stderr}, nil
			}
			got, err := QueryVersion(background, e, tc.tool, "some/program")
			if err != nil || got != tc.want {
				t.Errorf("ReportedVersion = %q, %v, want %q", got, err, tc.want)
			}
			if want := append([]string{"some/program"}, tc.tool.VersionArgs...); !slices.Equal(asked, want) {
				t.Errorf("it ran %q, want %q", asked, want)
			}
		})
	}
}

func TestReportedVersionPassesOnAProgramThatCannotBeStartedOrIsInterrupted(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	e.Run = runNotFound
	_, err := QueryVersion(background, e, YueScript, "yue")
	if diagErr := asDiagError(t, err, "no yue"); diagErr.Msg != "Cannot run 'yue': command not found." {
		t.Errorf("error = %+v", diagErr)
	}
	e.Run = runInterrupted
	if _, err := QueryVersion(background, e, YueScript, "yue"); err != context.Canceled {
		t.Errorf("ReportedVersion = %v, want the cancellation as it is", err)
	}
}

func TestKnownVersionsPinTheDefaultForWindowsAndLinux(t *testing.T) {
	const releases = "https://github.com/IppClub/YueScript/releases/download"
	want := map[string]map[string]Asset{
		"0.34.3": {
			"windows-x86_64": {releases + "/v0.34.3/yue-windows-x64.7z",
				"548b2fe699f46080cbca6c3d5951df2bcbcbb6bbdd215744054020962e6b7075", "7z", "yue.exe"},
			"linux-x86_64": {releases + "/v0.34.3/yue-linux-x86_64.zip",
				"9f47c8c7d3b6aa6e439786ae4708b9e070edbb01876712e1212917decd01d916", "zip", "yue"},
		},
		"0.34.2": {
			"windows-x86_64": {releases + "/v0.34.2/yue-windows-x64.7z",
				"367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf", "7z", "yue.exe"},
			"linux-x86_64": {releases + "/v0.34.2/yue-linux-x86_64.zip",
				"fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d", "zip", "yue"},
		},
	}
	if YueVersion != "0.34.3" || YueScript.Name != "yue" || len(YueScript.Versions) != len(want) {
		t.Fatalf("YueVersion = %s, YueScript = %+v", YueVersion, YueScript)
	}
	for version, platforms := range want {
		if len(YueScript.Versions[version]) != len(platforms) {
			t.Errorf("version %s has the downloads %+v", version, YueScript.Versions[version])
		}
		for platform, asset := range platforms {
			if YueScript.Versions[version][platform] != asset {
				t.Errorf("%s for %s = %+v", version, platform, YueScript.Versions[version][platform])
			}
		}
	}
	for _, system := range []string{"windows", "linux"} {
		if _, found := YueScript.Versions[YueVersion][env.PlatformOf(system, "amd64")]; !found {
			t.Errorf("no download of %s for %s", YueVersion, system)
		}
	}
}

func TestTheRefusalsOfBothToolsAreWordedAlike(t *testing.T) {
	const page = "https://pkl-lang.org/main/current/pkl-cli/index.html#installation"
	yueElse := "build or install yue yourself and set yue.path in moonwell.local.pkl."
	pklElse := "install Pkl 0.32 or newer yourself: " + page
	tests := []struct {
		name    string
		refusal error
		message string
		hint    string
	}{
		{"an unknown compiler", errUnknownVersion(YueScript, "1.0.0"),
			"Unknown YueScript version 1.0.0. Known versions: 0.34.2, 0.34.3.", "Use a known version, or " + yueElse},
		{"an unknown Pkl", errUnknownVersion(Pkl, "1.0.0"),
			"Unknown Pkl version 1.0.0. Known versions: 0.32.1.", "Use a known version, or " + pklElse},
		{"no compiler for the platform", errNoDownload(YueScript),
			"Moonwell cannot download YueScript for this platform (" + platformName() + ").",
			"Build or install yue yourself and set yue.path in moonwell.local.pkl."},
		{"no Pkl for the platform", errNoDownload(Pkl),
			"Moonwell cannot download Pkl for this platform (" + platformName() + ").",
			"Install Pkl 0.32 or newer yourself: " + page},
		{"the compiler, offline", errDownloadFailed(YueScript, "https://a.test/yue", os.ErrDeadlineExceeded),
			"Downloading https://a.test/yue failed.", "Check your connection and retry, or " + yueElse},
		{"Pkl, offline", errDownloadFailed(Pkl, "https://a.test/pkl", os.ErrDeadlineExceeded),
			"Downloading https://a.test/pkl failed.", "Check your connection and retry, or " + pklElse},
		{"the compiler, a failing status", errDownloadStatus(YueScript, "https://a.test/yue", 503),
			"Downloading https://a.test/yue failed with HTTP 503.", "Retry later, or " + yueElse},
		{"Pkl, a failing status", errDownloadStatus(Pkl, "https://a.test/pkl", 404),
			"Downloading https://a.test/pkl failed with HTTP 404.", "Retry later, or " + pklElse},
		{"the compiler, another checksum", errChecksum(YueScript, "aa", "bb"),
			"YueScript download checksum mismatch (expected aa, got bb).",
			"Retry the download, and do not bypass the check. If it keeps failing, report it, or " + yueElse},
		{"Pkl, another checksum", errChecksum(Pkl, "aa", "bb"),
			"Pkl download checksum mismatch (expected aa, got bb).",
			"Retry the download, and do not bypass the check. If it keeps failing, report it, or " + pklElse},
		{"the compiler, another version", errVersionMismatch(YueScript, "0.34.2", "0.34.3"),
			"Downloaded YueScript reports version 0.34.2, expected 0.34.3.", ""},
		{"Pkl, another version", errVersionMismatch(Pkl, "0.32.0", "0.32.1"),
			"Downloaded Pkl reports version 0.32.0, expected 0.32.1.", ""},
		{"Pkl, no version", errVersionMismatch(Pkl, "", "0.32.1"),
			"Downloaded Pkl reports version unknown, expected 0.32.1.", ""},
		{"an archive without the compiler", errProgramMissing(YueScript, "yue.exe"),
			"The YueScript archive has no yue.exe.", ""},
		{"an archive that cannot be unpacked", errNotExtracted(YueScript, "\r\n tar: no such file \v\n"),
			"Extracting YueScript failed:\ntar: no such file", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			diagErr := asDiagError(t, tc.refusal, tc.name)
			if diagErr.Msg != tc.message || diagErr.Hint != tc.hint {
				t.Errorf("message %q\n   hint %q\nwant    %q\n   hint %q", diagErr.Msg, diagErr.Hint, tc.message, tc.hint)
			}
		})
	}
}
