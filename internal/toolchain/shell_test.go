package toolchain

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestInstallBinCopiesTheCompilerOnceAndAgainWhenItChanges(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	source := testkit.WriteFile(t, t.TempDir(), "yue.exe", []byte("v1"))
	want := filepath.Join(e.CacheDir, "bin", "yue.exe")
	if path, copied, err := InstallBin(e, YueScript, source); err != nil || path != want || !copied {
		t.Fatalf("InstallBin = %q, %v, %v", path, copied, err)
	}
	if path, copied, err := InstallBin(e, YueScript, source); err != nil || path != want || copied {
		t.Errorf("InstallBin again = %q, %v, %v", path, copied, err)
	}
	testkit.WriteFile(t, filepath.Dir(source), "yue.exe", []byte("v2"))
	if _, copied, err := InstallBin(e, YueScript, source); err != nil || !copied {
		t.Errorf("InstallBin of a changed compiler = %v, %v", copied, err)
	}
	if data, _ := os.ReadFile(want); string(data) != "v2" {
		t.Errorf("the copy holds %q", data)
	}
}

func TestInstallBinReportsACopyItCannotReplace(t *testing.T) {
	tests := []struct {
		tool  Tool
		title string
		hint  string
	}{
		{YueScript, "YueScript",
			"Close the programs that run this copy of yue (such as an editor's YueScript extension), then run moonwell setup again."},
		{Pkl, "Pkl",
			"Close the programs that run this copy of pkl (such as an editor's Pkl extension), then run moonwell setup again."},
	}
	for _, tc := range tests {
		e, _ := testkit.Env(t, t.TempDir())
		source := testkit.WriteFile(t, t.TempDir(), tc.tool.Name, []byte("v1"))
		// A folder where the copy goes.
		blocked := filepath.Join(e.CacheDir, "bin", tc.tool.Name)
		if err := os.MkdirAll(blocked, 0o777); err != nil {
			t.Fatal(err)
		}
		path, copied, err := InstallBin(e, tc.tool, source)
		failure := asError(t, err, "a folder in the way")
		if !strings.HasPrefix(failure.Msg, "Copying "+tc.title+" to "+blocked+" failed: ") || failure.Hint != tc.hint ||
			failure.Cause == nil || path != blocked || copied {
			t.Errorf("InstallBin = %q, %v, %+v", path, copied, failure)
		}
	}
}

func TestYueOnPathTellsThePinnedVersionAnotherVersionAndAMissingYueApart(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	var asked []string
	e.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		asked = append([]string{program}, args...)
		return yueOf("0.30.0")(ctx, program, args, options)
	}
	if version, found, err := yueOnPath(background, e); version != "0.30.0" || !found || err != nil {
		t.Errorf("yueOnPath = %q, %v, %v", version, found, err)
	}
	if !slices.Equal(asked, []string{"yue", "-v"}) {
		t.Errorf("it ran %q", asked)
	}
	e.Run = missing
	if version, found, err := yueOnPath(background, e); version != "" || found || err != nil {
		t.Errorf("yueOnPath without yue = %q, %v, %v", version, found, err)
	}
	e.Run = prints("not a compiler")
	if _, found, err := yueOnPath(background, e); found || err != nil {
		t.Errorf("yueOnPath of another program = %v, %v", found, err)
	}
	e.Run = interrupted
	if _, _, err := yueOnPath(background, e); err != context.Canceled {
		t.Errorf("yueOnPath of a cancelled run = %v", err)
	}
}

func TestPathCommandGivesAPowerShellCommandOnWindowsAndAProfileLineElsewhere(t *testing.T) {
	windows := PathCommand(`C:\Users\me\AppData\Local\moonwell\bin`, "windows")
	want := `[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + ` +
		`';C:\Users\me\AppData\Local\moonwell\bin', 'User')`
	if windows != want {
		t.Errorf("PathCommand = %s", windows)
	}
	linux := PathCommand("/home/me/.cache/moonwell/bin", "linux")
	if linux != `echo 'export PATH="/home/me/.cache/moonwell/bin:$PATH"' >> ~/.profile` {
		t.Errorf("PathCommand = %s", linux)
	}
}

func TestPathCommandKeepsSpacesAndDoublesSingleQuotesInTheWindowsFolder(t *testing.T) {
	for folder, want := range map[string]string{
		`C:\Users\Jane Doe\AppData\Local\moonwell\bin`: `+ ';C:\Users\Jane Doe\AppData\Local\moonwell\bin', 'User')`,
		`C:\Users\O'Brien\AppData\Local\moonwell\bin`:  `+ ';C:\Users\O''Brien\AppData\Local\moonwell\bin', 'User')`,
	} {
		if got := PathCommand(folder, "windows"); !strings.Contains(got, want) {
			t.Errorf("PathCommand(%s) = %s", folder, got)
		}
	}
}

func TestReportYueOnPathWarnsOnceAboutYueOnPathAndSaysNothingWhenItIsRight(t *testing.T) {
	const binDir = "/home/me/.cache/moonwell/bin"
	e, log := testkit.Env(t, t.TempDir())
	e.Run = yueOf("0.30.0")
	if err := ReportYueOnPath(background, e, "0.34.2", binDir, "linux"); err != nil {
		t.Fatal(err)
	}
	want := "warning: yue on PATH is version 0.30.0; VS Code's YueScript extension needs YueScript 0.34.2 there. " +
		"Run this once in your shell, then open a new terminal and restart VS Code:\n  " + PathCommand(binDir, "linux")
	if lines := log.Lines(); !slices.Equal(lines, []string{want}) {
		t.Errorf("log = %q", lines)
	}
	quiet, silence := testkit.Env(t, t.TempDir())
	quiet.Run = yueOf("0.34.2")
	if err := ReportYueOnPath(background, quiet, "0.34.2", binDir, "linux"); err != nil || len(silence.Lines()) != 0 {
		t.Errorf("log = %q, %v", silence.Lines(), err)
	}
}

func TestReportYueOnPathTellsWindowsUsersToRunTheCommandInPowerShell(t *testing.T) {
	const binDir = `C:\Users\me\AppData\Local\moonwell\bin`
	e, log := testkit.Env(t, t.TempDir())
	e.Run = missing
	if err := ReportYueOnPath(background, e, "0.34.2", binDir, "windows"); err != nil {
		t.Fatal(err)
	}
	lines := log.Lines()
	if len(lines) != 1 || !strings.HasPrefix(lines[0], "warning: yue is not on PATH; ") ||
		!strings.Contains(lines[0], "Run this once in PowerShell, then open a new terminal and restart VS Code") ||
		!strings.HasSuffix(lines[0], PathCommand(binDir, "windows")) {
		t.Errorf("log = %q", lines)
	}
}

func TestReportYueOnPathPassesOnACancellationAndLogsNothing(t *testing.T) {
	e, log := testkit.Env(t, t.TempDir())
	e.Run = interrupted
	if err := ReportYueOnPath(background, e, "0.34.2", "/bin", "linux"); err != context.Canceled || len(log.Lines()) != 0 {
		t.Errorf("ReportYueOnPath = %v, log %q", err, log.Lines())
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
		// Only Windows reads a backslash as a separator and a drive letter as a root.
		cases["C:/Users/me/yue/0.34.2/yue.exe"] = "C:/Users/me/yue/0.34.2"
		cases[`C:\Users\me\yue\yue.exe`] = `C:\Users\me\yue`
		cases[`C:\Users/me\yue.exe`] = `C:\Users/me`
		cases["C:/yue.exe"] = "C:/"
		cases[`C:\yue.exe`] = `C:\`
	}
	for path, want := range cases {
		if got := DirAsWritten(path); got != want {
			t.Errorf("DirAsWritten(%q) = %q, want %q", path, got, want)
		}
	}
}
