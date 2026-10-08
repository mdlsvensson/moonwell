package env

import (
	"bytes"
	"context"
	"errors"
	"io/fs"
	"os/exec"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type RunFunc func(ctx context.Context, program string, args []string, options RunOptions) (RunResult, error)

type RunOptions struct {
	Dir  string
	Hint string
}

type RunResult struct {
	ExitCode       int
	Stdout, Stderr string
}

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
		return result, ctx.Err()
	case errors.As(err, &exit):
		result.ExitCode = exit.ExitCode()
		return result, nil
	}
	return result, NewSpawnError(program, err, options.Hint, "")
}

func NewSpawnError(program string, err error, hint, file string) *diag.Error {
	reason := fsx.Reason(err)
	if errors.Is(err, exec.ErrNotFound) || errors.Is(err, fs.ErrNotExist) {
		reason = "command not found"
	}
	return &diag.Error{
		Msg:   "Cannot run '" + program + "': " + strings.TrimSuffix(reason, ".") + ".",
		File:  file,
		Hint:  hint,
		Cause: err,
	}
}
