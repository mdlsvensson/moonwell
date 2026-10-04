package env

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os/exec"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// RunFunc runs a program to completion and captures its output. Tests replace it.
type RunFunc func(ctx context.Context, program string, args []string, options RunOptions) (RunResult, error)

// RunOptions change how a program is run.
type RunOptions struct {
	Dir  string // the working directory; empty means the caller's
	Hint string // shown when the program cannot be started at all
}

// RunResult is what a program printed and how it ended.
type RunResult struct {
	Code           int
	Stdout, Stderr string
}

// Run runs a program to completion and captures its output, decoded as text. A program that ran and failed is a
// RunResult with its exit code; an error means that it could not be started, or that ctx was cancelled.
func Run(ctx context.Context, program string, args []string, options RunOptions) (RunResult, error) {
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir = options.Dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	result := RunResult{Stdout: fsx.DecodeText(stdout.Bytes()), Stderr: fsx.DecodeText(stderr.Bytes())}
	var exit *exec.ExitError
	switch {
	case err == nil:
		return result, nil
	case ctx.Err() != nil:
		// A program stopped by the cancellation also ends with an exit error, so the context is asked first.
		return result, ctx.Err()
	case errors.As(err, &exit):
		result.Code = exit.ExitCode()
		return result, nil
	}
	return result, SpawnError(program, err, options.Hint, "")
}

// ---- errors ----

// SpawnError is the failure a user reads when program could not be started: it is missing, a folder, or not
// something the system can run. file is the file that names the program, or "" when there is none.
func SpawnError(program string, err error, hint, file string) *diag.Error {
	reason := fsx.Reason(err)
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		reason = "command not found"
	}
	return &diag.Error{
		// Some of the system's reasons end with a full stop and some do not; the message ends with exactly one.
		Msg:   "Cannot run '" + program + "': " + strings.TrimSuffix(reason, ".") + ".",
		File:  file,
		Hint:  hint,
		Cause: err,
	}
}
