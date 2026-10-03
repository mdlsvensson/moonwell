package yue

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/logging"
	"github.com/mdlsvensson/moonwell/internal/proc"
)

// InstallBin copies the pinned compiler to <cacheRoot>/bin/, the folder users put on PATH for the editor, unless an
// identical copy is already there.
func InstallBin(binary, cacheRoot string) (path string, copied bool, err error) {
	path = filepath.Join(cacheRoot, "bin", filepath.Base(binary))
	copied, err = fsx.CopyProgram(binary, path)
	if err != nil {
		return path, false, &diag.Error{
			Msg:   "Copying YueScript to " + path + " failed: " + fsx.Reason(err),
			Cause: err,
			Hint:  "Close VS Code (its YueScript extension runs this copy of yue), then run moonwell setup again.",
		}
	}
	return path, copied, nil
}

// OnPath says what `yue` on PATH is: found is false when there is none (or it names no version), and version is
// what it reports.
func OnPath(ctx context.Context, run proc.RunFunc) (version string, found bool, err error) {
	version, err = Version(ctx, "yue", run)
	if err != nil {
		// A program that cannot be started is a user-facing error; anything else (a cancelled command) is passed on.
		var expected *diag.Error
		if errors.As(err, &expected) {
			return "", false, nil
		}
		return "", false, err
	}
	return version, version != "", nil
}

// DirAsWritten is the folder of path with the separators the path was written with: the manifest's yue.path may use
// "/" on Windows, and the PATH command shows that folder to the user. filepath.Dir would rewrite them.
func DirAsWritten(path string) string {
	root := len(filepath.VolumeName(path))
	end := len(path)
	for end > root && os.IsPathSeparator(path[end-1]) {
		end--
	}
	for end > root && !os.IsPathSeparator(path[end-1]) {
		end--
	}
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

// PathCommand is the one-time command that adds binDir to the user's PATH: PowerShell on Windows, a shell line
// elsewhere. Moonwell never runs it itself.
func PathCommand(binDir, goos string) string {
	if goos == "windows" {
		// A single-quoted PowerShell literal takes the folder as it is; only "'" needs doubling.
		entry := strings.ReplaceAll(";"+binDir, "'", "''")
		return "[Environment]::SetEnvironmentVariable('Path', [Environment]::GetEnvironmentVariable('Path', 'User') + '" +
			entry + "', 'User')"
	}
	return `echo 'export PATH="` + binDir + `:$PATH"' >> ~/.profile`
}

// ReportEditorTools logs what VS Code's YueScript extension still needs on this machine: `yue` of the pinned version
// on PATH. The extension finds lua-language-server in the Lua extension, which the template's .vscode/extensions.json
// recommends.
func ReportEditorTools(ctx context.Context, run proc.RunFunc, log *logging.Logger, version, binDir, goos string) error {
	onPath, found, err := OnPath(ctx, run)
	if err != nil || (found && onPath == version) {
		return err
	}
	problem := "is not on PATH"
	if found {
		problem = "on PATH is version " + onPath
	}
	shell := "your shell"
	if goos == "windows" {
		shell = "PowerShell"
	}
	log.Warn("yue " + problem + "; VS Code's YueScript extension needs YueScript " + version + " there. " +
		"Run this once in " + shell + ", then open a new terminal and restart VS Code:\n" +
		"  " + PathCommand(binDir, goos))
	return nil
}
