package toolchain

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

func FindCompiler(ctx context.Context, e *env.Env, version string, path *string) (string, error) {
	if path == nil {
		return Ensure(ctx, e, YueScript, version)
	}
	if !fsx.Exists(*path) {
		return "", errNoYuePath(*path, manifest.UserFilePath(e))
	}
	found, err := QueryVersion(ctx, e, YueScript, *path)
	if err != nil {
		return "", blameYuePath(err, manifest.UserFilePath(e))
	}
	if found != version {
		e.Log.Warn("yue.path reports version " + versionOrUnknown(found) + ", expected " + version + ".")
	}
	return *path, nil
}

func blameYuePath(err error, userFile string) error {
	var diagErr *diag.Error
	if errors.As(err, &diagErr) {
		diagErr.File, diagErr.Hint = userFile, yuePathHint
	}
	return err
}

func FindPkl(ctx context.Context, e *env.Env) (string, error) {
	output, recent, err := findPklOnPath(ctx, e)
	onPath := !isStartFailure(err)
	switch {
	case err != nil && onPath:
		return "", err
	case recent:
		return Pkl.Name, nil
	}
	if _, downloads := Pkl.Versions[PklVersion][e.Platform]; !downloads {
		if !onPath {
			return "", err
		}
		return "", errPklTooOld(output)
	}
	if onPath {
		e.Log.Warn("pkl on PATH is older than 0.32 (" + output + "), so Moonwell runs its own Pkl " + PklVersion +
			". A pkl command you type yourself, such as `pkl project resolve`, still runs the old one.")
	}
	return Ensure(ctx, e, Pkl, PklVersion)
}

func findPklOnPath(ctx context.Context, e *env.Env) (output string, recent bool, err error) {
	result, err := Pkl.runVersionCommand(ctx, e, Pkl.Name, pklInstallHint)
	if err != nil {
		return "", false, err
	}
	return versionOrUnknown(fsx.TrimASCIISpace(result.Stdout)), isSupportedPkl(Pkl.parseVersion(result)), nil
}

func isSupportedPkl(version string) bool {
	numbers := strings.Split(version, ".")
	if len(numbers) < 2 {
		return false
	}
	major, _ := strconv.Atoi(numbers[0])
	minor, _ := strconv.Atoi(numbers[1])
	return major > 0 || minor >= 32
}

func isStartFailure(err error) bool {
	var diagErr *diag.Error
	return errors.As(err, &diagErr)
}

const pklInstallHint = "Install Pkl 0.32 or newer: " + pklPage

const yuePathHint = "Point yue.path in that file at a yue program this system can run, or remove it to use the " +
	"compiler Moonwell downloads."

func errNoYuePath(path, userFile string) error {
	return &diag.Error{Msg: "yue.path does not exist: " + path, File: userFile, Hint: yuePathHint}
}

func errPklTooOld(output string) error {
	return &diag.Error{Msg: "Moonwell needs Pkl 0.32 or newer (found: " + output + ").", Hint: pklInstallHint}
}
