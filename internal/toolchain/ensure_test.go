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

func TestTheRefusalsOfBothToolsAreWordedAlike(t *testing.T) {
	const page = "https://pkl-lang.org/main/current/pkl-cli/index.html#installation"
	yueElse := "build or install yue yourself and set yue.path in moonwell.local.pkl."
	pklElse := "install Pkl 0.32 or newer yourself: " + page
	tests := []struct {
		name    string
		gotErr  error
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
			diagErr := asDiagError(t, tc.gotErr, tc.name)
			if diagErr.Msg != tc.message || diagErr.Hint != tc.hint {
				t.Errorf("message %q\n   hint %q\nwant    %q\n   hint %q", diagErr.Msg, diagErr.Hint, tc.message, tc.hint)
			}
		})
	}
}
