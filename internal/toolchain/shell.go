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

func InstallBin(e *env.Env, tool Tool, program string) (path string, copied bool, err error) {
	path = filepath.Join(e.CacheDir, "bin", filepath.Base(program))
	copied, err = fsx.CopyProgram(program, path)
	if err != nil {
		return path, false, errNotCopied(tool, path, err)
	}
	return path, copied, nil
}

func PathCommand(binDir, goos string) string {
	if goos == "windows" {
		entry := strings.ReplaceAll(";"+binDir, "'", "''")
		return "[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + '" +
			entry + "', 'User')"
	}
	return `echo 'export PATH="` + binDir + `:$PATH"' >> ~/.profile`
}

func DirAsWritten(path string) string {
	root := len(filepath.VolumeName(path))
	end := lastName(path, root)
	if end == root {
		if root > 0 {
			return path[:root]
		}
		return "."
	}
	for end > root+1 && os.IsPathSeparator(path[end-1]) {
		end--
	}
	return path[:end]
}

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

func shellOf(goos string) string {
	if goos == "windows" {
		return "PowerShell"
	}
	return "your shell"
}

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

func errNotCopied(tool Tool, path string, cause error) error {
	return &diag.Error{
		Msg: "Copying " + tool.Title + " to " + path + " failed: " + fsx.Reason(cause),
		Hint: "Close the programs that run this copy of " + tool.Name + " (such as an editor's " + tool.Title +
			" extension), then run moonwell setup again.",
		Cause: cause,
	}
}
