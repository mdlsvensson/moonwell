// Package proc runs other programs: pkl, yue, tar.
package proc

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os/exec"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Result is what a program printed and how it ended.
type Result struct {
	Code   int
	Stdout string
	Stderr string
}

// Options change how a program is run.
type Options struct {
	// Dir is the working directory; empty means the caller's.
	Dir string
	// Hint is shown when the command cannot be started at all.
	Hint string
}

// RunFunc runs a command to completion and captures its output. Tests substitute it.
type RunFunc func(ctx context.Context, command string, args []string, options Options) (Result, error)

// SpawnError wraps a failure to start command (missing, a folder, not executable) in a user-facing error.
func SpawnError(command string, err error, hint, file string) *diag.Error {
	reason := fsx.Reason(err)
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		reason = "command not found"
	}
	return &diag.Error{
		Msg:   "Cannot run '" + command + "': " + strings.TrimSuffix(reason, ".") + ".",
		File:  file,
		Hint:  hint,
		Cause: err,
	}
}

// Run runs a command to completion and captures its output. A program that ran and failed is a Result with its
// exit code; an error means it could not be started, or ctx was cancelled.
func Run(ctx context.Context, command string, args []string, options Options) (Result, error) {
	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = options.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := Result{Stdout: text.Decode(stdout.Bytes()), Stderr: text.Decode(stderr.Bytes())}
	var exit *exec.ExitError
	switch {
	case err == nil:
	case ctx.Err() != nil:
		return result, ctx.Err()
	case errors.As(err, &exit):
		result.Code = exit.ExitCode()
	default:
		return result, SpawnError(command, err, options.Hint, "")
	}
	return result, nil
}
