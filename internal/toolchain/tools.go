package toolchain

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const (
	YueVersion = "0.34.3"
	PklVersion = "0.32.1"
)

const (
	yueReleases = "https://github.com/IppClub/YueScript/releases/download"
	pklReleases = "https://github.com/apple/pkl/releases/download/" + PklVersion + "/"
	pklPage     = "https://pkl-lang.org/main/current/pkl-cli/index.html#installation"
)

var YueScript = Tool{
	Name:  "yue",
	Title: "YueScript",
	Versions: map[string]map[string]Asset{
		"0.34.3": {
			"windows-x86_64": {
				URL:     yueReleases + "/v0.34.3/yue-windows-x64.7z",
				SHA256:  "548b2fe699f46080cbca6c3d5951df2bcbcbb6bbdd215744054020962e6b7075",
				Archive: "7z",
				Binary:  "yue.exe",
			},
			"linux-x86_64": {
				URL:     yueReleases + "/v0.34.3/yue-linux-x86_64.zip",
				SHA256:  "9f47c8c7d3b6aa6e439786ae4708b9e070edbb01876712e1212917decd01d916",
				Archive: "zip",
				Binary:  "yue",
			},
		},
		"0.34.2": {
			"windows-x86_64": {
				URL:     yueReleases + "/v0.34.2/yue-windows-x64.7z",
				SHA256:  "367e79f450dc60d96d248c8e1d97b4ec47729b963f64226888fb8e111fc349bf",
				Archive: "7z",
				Binary:  "yue.exe",
			},
			"linux-x86_64": {
				URL:     yueReleases + "/v0.34.2/yue-linux-x86_64.zip",
				SHA256:  "fffcaa3624bc61e0a2a40cb117fe59460d6503d08348857229ce7d2b2f2b7c2d",
				Archive: "zip",
				Binary:  "yue",
			},
		},
	},
	VersionArgs:       []string{"-v"},
	VersionPattern:    regexp.MustCompile(`Yuescript version: ([^` + fsx.ASCIISpace + `]+)`),
	ManualInstallHint: "build or install yue yourself and set yue.path in moonwell.local.pkl.",
}

var Pkl = Tool{
	Name:  "pkl",
	Title: "Pkl",
	Versions: map[string]map[string]Asset{
		PklVersion: {
			"windows-x86_64": {
				URL:    pklReleases + "pkl-windows-amd64.exe",
				SHA256: "8550a00fcf027335e42c5e2cd553b88e98845408cb1880b3e3d1860caf46d22a",
				Binary: "pkl.exe",
			},
			"linux-x86_64": {
				URL:    pklReleases + "pkl-linux-amd64",
				SHA256: "3180b62da95c0cad1d904e9bb6c5f4a8f9032413c21e53194bb91ff1ee5f3211",
				Binary: "pkl",
			},
		},
	},
	VersionArgs:       []string{"--version"},
	VersionPattern:    regexp.MustCompile(`Pkl (\d+\.\d+\.\d+)`),
	ManualInstallHint: "install Pkl 0.32 or newer yourself: " + pklPage,
}

func FindCompiler(ctx context.Context, e *env.Env, version string, path *string) (string, error) {
	if path == nil {
		return Ensure(ctx, e, YueScript, version)
	}
	if !fsx.Exists(*path) {
		return "", errNoYuePath(*path)
	}
	found, err := QueryVersion(ctx, e, YueScript, *path)
	if err != nil {
		return "", blameYuePath(err)
	}
	if found != version {
		e.Log.Warn("yue.path reports version " + versionOrUnknown(found) + ", expected " + version + ".")
	}
	return *path, nil
}

func blameYuePath(err error) error {
	var diagErr *diag.Error
	if errors.As(err, &diagErr) {
		diagErr.File, diagErr.Hint = yuePathFile, yuePathHint
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

const yuePathFile = "moonwell.local.pkl"

const yuePathHint = "Point yue.path in " + yuePathFile + " at a yue program this system can run, or remove it to " +
	"use the compiler Moonwell downloads."

func errNoYuePath(path string) error {
	return &diag.Error{Msg: "yue.path does not exist: " + path, File: yuePathFile, Hint: yuePathHint}
}

func errPklTooOld(output string) error {
	return &diag.Error{Msg: "Moonwell needs Pkl 0.32 or newer (found: " + output + ").", Hint: pklInstallHint}
}
