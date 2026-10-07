package toolchain

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// What setup does so that an editor and a shell find the pinned programs: a copy of each in one folder of the
// cache, and the command that puts that folder on PATH. Moonwell never changes PATH itself.

// InstallBin copies a tool's program to <cache>/bin/, the folder users put on PATH, unless an identical copy is
// there. It returns the copy's path, and whether it copied.
func InstallBin(e *env.Env, tool Tool, program string) (path string, copied bool, err error) {
	path = filepath.Join(e.CacheDir, "bin", filepath.Base(program))
	copied, err = fsx.CopyProgram(program, path)
	if err != nil {
		return path, false, errNotCopied(tool, path, err)
	}
	return path, copied, nil
}

// PathCommand is the one-time command that adds binDir to the user's PATH: PowerShell on Windows, a shell line
// elsewhere. goos is the system as Go names it.
func PathCommand(binDir, goos string) string {
	if goos == "windows" {
		// A single-quoted PowerShell literal takes the folder as it is; only "'" needs doubling.
		entry := strings.ReplaceAll(";"+binDir, "'", "''")
		return "[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + '" +
			entry + "', 'User')"
	}
	return `echo 'export PATH="` + binDir + `:$PATH"' >> ~/.profile`
}

// DirAsWritten is the folder of path with the separators the path was written with: the manifest's yue.path may
// use "/" on Windows, and the PATH command shows that folder to the user. filepath.Dir would rewrite them.
func DirAsWritten(path string) string {
	root := len(filepath.VolumeName(path))
	end := lastName(path, root)
	if end == root {
		if root > 0 {
			return path[:root]
		}
		return "."
	}
	// path[:end] ends with the separators before the last name; the root keeps one of them.
	for end > root+1 && os.IsPathSeparator(path[end-1]) {
		end--
	}
	return path[:end]
}

// lastName is where the last name of path starts, after the separators before it; root is where the path's
// volume ends. Separators after the last name do not count.
func lastName(path string, root int) int {
	end := len(path)
	for end > root && os.IsPathSeparator(path[end-1]) {
		end--
	}
	for end > root && !os.IsPathSeparator(path[end-1]) {
		end--
	}
	return end
}

// shellOf names the shell in which a user of this system runs the PATH command.
func shellOf(goos string) string {
	if goos == "windows" {
		return "PowerShell"
	}
	return "your shell"
}

// ReportYueOnPath logs what VS Code's YueScript extension still needs on this machine: `yue` of this version on
// PATH. binDir is the folder with the copy of the compiler, for the PATH command. It says nothing when PATH has
// that version.
func ReportYueOnPath(ctx context.Context, e *env.Env, version, binDir, goos string) error {
	onPath, found, err := yueOnPath(ctx, e)
	if err != nil || (found && onPath == version) {
		return err
	}
	problem := "is not on PATH"
	if found {
		problem = "on PATH is version " + onPath
	}
	e.Log.Warn("yue " + problem + "; VS Code's YueScript extension needs YueScript " + version + " there. " +
		"Run this once in " + shellOf(goos) + ", then open a new terminal and restart VS Code:\n" +
		"  " + PathCommand(binDir, goos))
	return nil
}

// yueOnPath says what `yue` on PATH is: found is false when there is none, or when it names no version, and
// version is what it reports. A run that was cancelled is passed on.
func yueOnPath(ctx context.Context, e *env.Env) (version string, found bool, err error) {
	version, err = ReportedVersion(ctx, e, YueScript, YueScript.Name)
	switch {
	case notStarted(err):
		return "", false, nil
	case err != nil:
		return "", false, err
	}
	return version, version != "", nil
}

// KeepPklForShell is setup's step for the pinned Pkl. When Moonwell runs its own Pkl (program is not the bare
// name of the one on PATH), it copies it to <cache>/bin/ and warns when `pkl` typed in a shell still finds no
// Pkl, with the command that puts that folder on PATH. An older pkl on PATH is PklProgram's to report.
func KeepPklForShell(ctx context.Context, e *env.Env, program, goos string) error {
	if program == Pkl.Name {
		return nil
	}
	path, copied, err := InstallBin(e, Pkl, program)
	if err != nil {
		return err
	}
	if copied {
		e.Log.Info("Copied Pkl " + PklVersion + " to " + path + ".")
	}
	if _, err := Pkl.ask(ctx, e, Pkl.Name, ""); !notStarted(err) {
		return err
	}
	e.Log.Warn("pkl is not on PATH, so a pkl command you type, such as `pkl project resolve`, finds no Pkl. " +
		"Run this once in " + shellOf(goos) + ", then open a new terminal:\n  " + PathCommand(filepath.Dir(path), goos))
	return nil
}

// ---- errors ----

func errNotCopied(tool Tool, path string, cause error) error {
	return &diag.Error{
		Msg: "Copying " + tool.Title + " to " + path + " failed: " + fsx.Reason(cause),
		Hint: "Close the programs that run this copy of " + tool.Name + " (such as an editor's " + tool.Title +
			" extension), then run moonwell setup again.",
		Cause: cause,
	}
}
