package toolchain

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func newDownloadEnv(t testing.TB, body []byte, archive, binary string) (*env.Env, Tool) {
	t.Helper()
	e, _, _ := newEnv(t, yueAddress, body)
	e.Run = fakeYueRun("9.9.9")
	return e, yueToolWith(Asset{URL: yueAddress, SHA256: fsx.SHA256Hex(body), Archive: archive, Binary: binary})
}

func TestTheProgramIsAskedForItsVersionInAStagingFolderBesideTheTargetBeforeItIsMoved(t *testing.T) {
	e, _, _, tool := newYueEnv(t, "")
	target := filepath.Join(e.CacheDir, "yue", "9.9.9")
	var asked string
	e.Run = func(_ context.Context, program string, args []string, _ env.RunOptions) (env.RunResult, error) {
		asked = program
		if fsx.Exists(target) {
			t.Errorf("%s is there before the program was asked for its version", target)
		}
		if data, _ := os.ReadFile(program); string(data) != "fake-binary" || !slices.Equal(args, []string{"-v"}) {
			t.Errorf("asked %s %q, which holds %q", program, args, data)
		}
		return env.RunResult{Stdout: "Yuescript version: 9.9.9\n"}, nil
	}
	if _, err := Ensure(background, e, tool, "9.9.9"); err != nil {
		t.Fatal(diag.Format(err))
	}
	staging := filepath.Dir(asked)
	if filepath.Dir(staging) != filepath.Dir(target) || !strings.HasPrefix(filepath.Base(staging), ".install-") ||
		filepath.Base(asked) != "yue" || fsx.Exists(staging) {
		t.Errorf("the program was asked at %s", asked)
	}
}

func TestAZipGivesUpOnlyTheProgram(t *testing.T) {
	archive := newZip(t, "../escaped", "x", "../../../escaped", "x", "/escaped", "x", "notes/readme", "x", "yue", "the compiler")
	e, tool := newDownloadEnv(t, archive, "zip", "yue")
	root := t.TempDir()
	e.CacheDir = filepath.Join(root, "user", "cache")
	program, err := Ensure(background, e, tool, "9.9.9")
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if data, _ := os.ReadFile(program); string(data) != "the compiler" {
		t.Errorf("the compiler holds %q", data)
	}
	want := []string{"user", "user/cache", "user/cache/yue", "user/cache/yue/9.9.9", "user/cache/yue/9.9.9/yue"}
	if got := listDir(t, root); !slices.Equal(got, want) {
		t.Errorf("the install wrote %q, want %q", got, want)
	}
}

func TestAProgramInAFolderOfTheArchiveKeepsThatFolderInTheCache(t *testing.T) {
	e, tool := newDownloadEnv(t, newZip(t, "yue", "at the top", "bin/yue", "in the folder"), "zip", "bin/yue")
	program, err := Ensure(background, e, tool, "9.9.9")
	if err != nil || program != filepath.Join(e.CacheDir, "yue", "9.9.9", "bin", "yue") {
		t.Fatalf("Ensure = %q, %v", program, err)
	}
	if data, _ := os.ReadFile(program); string(data) != "in the folder" {
		t.Errorf("the compiler holds %q", data)
	}
}

func TestADownloadThatIsNoZipIsRefusedAndNothingIsInstalled(t *testing.T) {
	e, tool := newDownloadEnv(t, []byte("an error page"), "zip", "yue")
	e.Run = failingRun(t)
	_, err := Ensure(background, e, tool, "9.9.9")
	if diagErr := asDiagError(t, err, "no zip"); !strings.HasPrefix(diagErr.Msg, "Invalid zip archive: ") {
		t.Errorf("error = %+v", diagErr)
	}
	if left := listDir(t, filepath.Join(e.CacheDir, "yue")); len(left) != 0 {
		t.Errorf("something was left in the cache: %q", left)
	}
}

func fakeTar(t testing.TB, archive []byte, unpacked ...string) env.RunFunc {
	return func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		if filepath.Base(program) != "tar.exe" {
			return fakeYueRun("9.9.9")(ctx, program, args, options)
		}
		if len(args) != 4 || args[0] != "-xf" || args[2] != "-C" || args[1] != filepath.Join(args[3], "archive.7z") {
			t.Errorf("tar was asked %q", args)
			return env.RunResult{ExitCode: 1}, nil
		}
		if data, _ := os.ReadFile(args[1]); string(data) != string(archive) {
			t.Errorf("the archive handed to tar holds %q", data)
		}
		for i := 0; i < len(unpacked); i += 2 {
			testkit.WriteFile(t, args[3], unpacked[i], []byte(unpacked[i+1]))
		}
		return env.RunResult{}, nil
	}
}

func TestA7zIsUnpackedByTheTarOfWindowsThroughRun(t *testing.T) {
	for _, systemRoot := range []string{`X:\System`, ""} {
		t.Setenv("SystemRoot", systemRoot)
		archive := []byte("a 7z archive")
		e, tool := newDownloadEnv(t, archive, "7z", "yue.exe")
		unpack := fakeTar(t, archive, "yue.exe", "the compiler", "license.txt", "MIT")
		var ran []string
		e.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
			ran = append(ran, program)
			return unpack(ctx, program, args, options)
		}
		program, err := Ensure(background, e, tool, "9.9.9")
		if err != nil || program != filepath.Join(e.CacheDir, "yue", "9.9.9", "yue.exe") {
			t.Fatalf("Ensure = %q, %v", program, err)
		}
		if systemRoot == "" {
			systemRoot = `C:\Windows`
		}
		if len(ran) != 2 || ran[0] != filepath.Join(systemRoot, "System32", "tar.exe") {
			t.Errorf("it ran %q", ran)
		}
		want := []string{"yue", "yue/9.9.9", "yue/9.9.9/license.txt", "yue/9.9.9/yue.exe"}
		if got := listDir(t, e.CacheDir); !slices.Equal(got, want) {
			t.Errorf("the cache holds %q, want %q", got, want)
		}
	}
}

func TestA7zThatCannotBeUnpackedIsRefusedAndNothingIsInstalled(t *testing.T) {
	archive := []byte("a 7z archive")
	tests := []struct {
		name    string
		tar     env.RunFunc
		message string
		hinted  bool
	}{
		{"tar fails", func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
			return env.RunResult{ExitCode: 1, Stderr: "tar: Error opening archive\r\n"}, nil
		}, "Extracting YueScript failed:\ntar: Error opening archive", false},
		{"the archive holds the program under another name", fakeTar(t, archive, "yue-x64.exe", "the compiler"),
			"The YueScript archive has no yue.exe.", false},
		{"there is no tar", runNotFound, "Cannot run '" + filepath.Join(`X:\System`, "System32", "tar.exe") + "': command not found.", true},
		{"tar is interrupted", runInterrupted, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("SystemRoot", `X:\System`)
			e, tool := newDownloadEnv(t, archive, "7z", "yue.exe")
			e.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
				if filepath.Base(program) != "tar.exe" {
					t.Errorf("%s was run", program)
				}
				return tc.tar(ctx, program, args, options)
			}
			_, err := Ensure(background, e, tool, "9.9.9")
			if tc.message == "" && err != context.Canceled {
				t.Errorf("Ensure = %v, want the cancellation as it is", err)
			}
			if tc.message != "" {
				diagErr := asDiagError(t, err, tc.name)
				if diagErr.Msg != tc.message || (diagErr.Hint != "") != tc.hinted {
					t.Errorf("error = %+v", diagErr)
				}
			}
			if left := listDir(t, filepath.Join(e.CacheDir, "yue")); len(left) != 0 {
				t.Errorf("something was left in the cache: %q", left)
			}
		})
	}
}

func TestAnArchiveOfAKindMoonwellDoesNotUnpackIsNotAUsersMistake(t *testing.T) {
	e, tool := newDownloadEnv(t, []byte("a tarball"), "tar.gz", "yue")
	e.Run = failingRun(t)
	_, err := Ensure(background, e, tool, "9.9.9")
	var expected *diag.Error
	if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "tar.gz") {
		t.Errorf("Ensure = %v, want an error that is no diag error and names the kind", err)
	}
	if left := listDir(t, filepath.Join(e.CacheDir, "yue")); len(left) != 0 {
		t.Errorf("something was left in the cache: %q", left)
	}
}

func TestADownloadedProgramThatCannotBeStartedIsRefusedWithWhatElseToDo(t *testing.T) {
	yueWorld, _, _, yue := newYueEnv(t, "")
	pklWorld, _, _ := newPklEnv(t, "")
	tests := []struct {
		e       *env.Env
		tool    Tool
		version string
		hint    string
	}{
		{yueWorld, yue, "9.9.9", "Build or install yue yourself and set yue.path in config.toml in your Moonwell folder."},
		{pklWorld, Pkl, PklVersion,
			"Install Pkl 0.32 or newer yourself: https://pkl-lang.org/main/current/pkl-cli/index.html#installation"},
	}
	for _, tc := range tests {
		tc.e.Run = runNotFound
		_, err := Ensure(background, tc.e, tc.tool, tc.version)
		diagErr := asDiagError(t, err, "a download that is no program")
		staged := filepath.Join(tc.e.CacheDir, tc.tool.Name, ".install-")
		if !strings.HasPrefix(diagErr.Msg, "Cannot run '"+staged) || !strings.HasSuffix(diagErr.Msg, "': command not found.") ||
			diagErr.Hint != tc.hint {
			t.Errorf("error = %+v", diagErr)
		}
		if left := listDir(t, filepath.Join(tc.e.CacheDir, tc.tool.Name)); len(left) != 0 {
			t.Errorf("something was left in the cache: %q", left)
		}
	}
}

func TestAnInstallAnotherProcessFinishedFirstIsKept(t *testing.T) {
	e, _, _, tool := newYueEnv(t, "")
	e.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		testkit.WriteFile(t, e.CacheDir, "yue/9.9.9/yue", []byte("theirs"))
		return fakeYueRun("9.9.9")(ctx, program, args, options)
	}
	program, err := Ensure(background, e, tool, "9.9.9")
	if err != nil || program != filepath.Join(e.CacheDir, "yue", "9.9.9", "yue") {
		t.Fatalf("Ensure = %q, %v", program, err)
	}
	if data, _ := os.ReadFile(program); string(data) != "theirs" {
		t.Errorf("the compiler holds %q", data)
	}
	if left := listDir(t, e.CacheDir); !slices.Equal(left, []string{"yue", "yue/9.9.9", "yue/9.9.9/yue"}) {
		t.Errorf("the cache holds %q", left)
	}
}

func TestAFolderInThePlaceOfTheInstallThatHoldsNoProgramIsRefusedByItsName(t *testing.T) {
	e, _, _, tool := newYueEnv(t, "")
	testkit.WriteFile(t, e.CacheDir, "yue/9.9.9/notes.txt", []byte("mine"))
	_, err := Ensure(background, e, tool, "9.9.9")
	diagErr := asDiagError(t, err, "a folder in the way")
	target := filepath.Join(e.CacheDir, "yue", "9.9.9")
	if !strings.HasPrefix(diagErr.Msg, "Installing YueScript failed: ") || diagErr.File != target ||
		!strings.Contains(diagErr.Hint, "Remove "+target) {
		t.Errorf("error = %+v", diagErr)
	}
	if left := listDir(t, e.CacheDir); !slices.Equal(left, []string{"yue", "yue/9.9.9", "yue/9.9.9/notes.txt"}) {
		t.Errorf("the cache holds %q", left)
	}
}

func TestAMoveIntoPlaceThatFailsWithNothingInItsWayIsRefusedWithTheSystemsReason(t *testing.T) {
	e, _, _, tool := newYueEnv(t, "")
	e.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		if err := os.RemoveAll(filepath.Dir(program)); err != nil {
			t.Error(err)
		}
		return fakeYueRun("9.9.9")(ctx, program, args, options)
	}
	_, err := Ensure(background, e, tool, "9.9.9")
	diagErr := asDiagError(t, err, "a move that fails")
	target := filepath.Join(e.CacheDir, "yue", "9.9.9")
	if !strings.HasPrefix(diagErr.Msg, "Installing YueScript failed: ") || diagErr.File != target ||
		diagErr.Hint == "" || strings.Contains(diagErr.Hint, "Remove ") || diagErr.Cause == nil {
		t.Errorf("error = %+v, at %q, with the hint %q", diagErr, diagErr.File, diagErr.Hint)
	}
	if left := listDir(t, filepath.Join(e.CacheDir, "yue")); len(left) != 0 {
		t.Errorf("something was left in the cache: %q", left)
	}
}

func TestACacheFolderThatCannotBeMadeIsRefusedByItsName(t *testing.T) {
	e, _, _, tool := newYueEnv(t, "")
	e.Run = failingRun(t)
	e.CacheDir = testkit.WriteFile(t, t.TempDir(), "cache", []byte("a file where the cache folder goes"))
	_, err := Ensure(background, e, tool, "9.9.9")
	diagErr := asDiagError(t, err, "a file in place of the cache")
	if !strings.HasPrefix(diagErr.Msg, "Installing YueScript failed: ") || diagErr.File != filepath.Join(e.CacheDir, "yue") ||
		diagErr.Hint == "" || diagErr.Cause == nil {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestAStagingFolderThatCannotBeRemovedIsNamedInAWarning(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows refuses to remove a file that a program has open")
	}
	e, log, _, tool := newYueEnv(t, "")
	e.Run = func(_ context.Context, program string, _ []string, _ env.RunOptions) (env.RunResult, error) {
		held, err := os.Open(program)
		if err != nil {
			t.Error(err)
		}
		t.Cleanup(func() { held.Close() })
		return env.RunResult{Stdout: "Yuescript version: 9.9.8\n"}, nil
	}
	_, err := Ensure(background, e, tool, "9.9.9")
	if diagErr := asDiagError(t, err, "another version"); !strings.Contains(diagErr.Msg, "reports version 9.9.8") {
		t.Errorf("error = %+v", diagErr)
	}
	lines := log.Lines()
	staging := filepath.Join(e.CacheDir, "yue", ".install-")
	if len(lines) != 2 || !strings.HasPrefix(lines[1], "warning: ") || !strings.Contains(lines[1], staging) ||
		!strings.Contains(lines[1], "remove the folder yourself") {
		t.Errorf("log = %q", lines)
	}
	if fsx.Exists(filepath.Join(e.CacheDir, "yue", "9.9.9")) {
		t.Errorf("the program that failed the check is in place")
	}
}

func TestTheInstalledProgramMayBeRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows keeps no permission to run a file")
	}
	for name, download := range map[string]func() (*env.Env, Tool){
		"from a zip": func() (*env.Env, Tool) { return newDownloadEnv(t, newZip(t, "yue", "x"), "zip", "yue") },
		"as it is":   func() (*env.Env, Tool) { return newDownloadEnv(t, []byte("x"), "", "yue") },
	} {
		e, tool := download()
		program, err := Ensure(background, e, tool, "9.9.9")
		if err != nil {
			t.Fatalf("%s: %v", name, diag.Format(err))
		}
		if info, err := os.Stat(program); err != nil || info.Mode().Perm() != 0o755 {
			t.Errorf("%s: the program has the mode %v, %v", name, info.Mode(), err)
		}
	}
}
