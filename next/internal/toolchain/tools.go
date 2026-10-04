package toolchain

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

const (
	YueVersion = "0.34.3" // what a project gets unless its manifest says otherwise
	PklVersion = "0.32.1" // what Moonwell downloads when PATH has no Pkl it can use
)

const (
	yueReleases = "https://github.com/IppClub/YueScript/releases/download"
	pklReleases = "https://github.com/apple/pkl/releases/download/" + PklVersion + "/"
	pklPage     = "https://pkl-lang.org/main/current/pkl-cli/index.html#installation"
)

// YueScript is the compiler. Its checksums were verified when each version was pinned. It also knows 0.34.2,
// which a project on an older Pkl package names.
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
	VersionArgs: []string{"-v"},
	// The version ends at the first white space, which is ASCII's.
	Reported:  regexp.MustCompile(`Yuescript version: ([^ \t\n\v\f\r]+)`),
	Otherwise: "build or install yue yourself and set yue.path in moonwell.local.pkl.",
}

// Pkl is the program that evaluates a project's manifest. Its checksums were verified when the version was
// pinned; they are GitHub's digests of the release's files, which are bare executables.
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
	VersionArgs: []string{"--version"},
	Reported:    regexp.MustCompile(`Pkl (\d+\.\d+\.\d+)`),
	Otherwise:   "install Pkl 0.32 or newer yourself: " + pklPage,
}

// Compiler returns the YueScript compiler for a project: the manifest's yue.path when it is set, used as it is,
// with a warning when it reports another version; else Ensure.
//
// A yue.path is the user's own program: it is neither verified nor copied, and it is taken on a platform and for
// a version that Moonwell has no download of. The `yue` on PATH is never the compiler.
func Compiler(ctx context.Context, e *env.Env, version string, path *string) (string, error) {
	if path == nil {
		return Ensure(ctx, e, YueScript, version)
	}
	if !fsx.Exists(*path) {
		return "", errNoYuePath(*path)
	}
	found, err := ReportedVersion(ctx, e, YueScript, *path)
	if err != nil {
		return "", err
	}
	if found != version {
		e.Log.Warn("yue.path reports version " + orUnknown(found) + ", expected " + version + ".")
	}
	return *path, nil
}

// PklProgram returns the pkl to run: "pkl" when the one on PATH is 0.32 or newer; else Ensure, with a warning
// when PATH has an older one.
//
// Where Moonwell has no download for the platform, a PATH without a usable Pkl is refused in words of its own,
// which ask for an install.
func PklProgram(ctx context.Context, e *env.Env) (string, error) {
	printed, recent, err := pklOnPath(ctx, e)
	onPath := !notStarted(err)
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
		return "", errOlderPkl(printed)
	}
	if onPath {
		e.Log.Warn("pkl on PATH is older than 0.32 (" + printed + "), so Moonwell runs its own Pkl " + PklVersion +
			". A pkl command you type yourself, such as `pkl project resolve`, still runs the old one.")
	}
	return Ensure(ctx, e, Pkl, PklVersion)
}

// pklOnPath asks the pkl on PATH for its version: what it printed, "unknown" for nothing, and whether that is
// Pkl 0.32 or newer. The error of a PATH without pkl carries the hint to install one.
func pklOnPath(ctx context.Context, e *env.Env) (printed string, recent bool, err error) {
	result, err := Pkl.ask(ctx, e, Pkl.Name, pklInstallHint)
	if err != nil {
		return "", false, err
	}
	return orUnknown(trimmed(result.Stdout)), recentPkl(Pkl.versionIn(result)), nil
}

// recentPkl reports whether a version, as Pkl reports it, is 0.32 or newer; "" is none.
func recentPkl(version string) bool {
	numbers := strings.Split(version, ".")
	if len(numbers) < 2 {
		return false
	}
	// A number too large to hold is read as the largest there is, which is newer.
	major, _ := strconv.Atoi(numbers[0])
	minor, _ := strconv.Atoi(numbers[1])
	return major > 0 || minor >= 32
}

// notStarted reports whether err is that of a program that could not be started, which Run raises as a
// *diag.Error; every other failure of a run, a cancelled context among them, comes as it is.
func notStarted(err error) bool {
	var expected *diag.Error
	return errors.As(err, &expected)
}

// ---- errors ----

// pklInstallHint says how to get Pkl where Moonwell cannot download it.
const pklInstallHint = "Install Pkl 0.32 or newer: " + pklPage

func errNoYuePath(path string) error {
	return &diag.Error{Msg: "yue.path does not exist: " + path, File: "moonwell.local.pkl"}
}

func errOlderPkl(printed string) error {
	return &diag.Error{Msg: "Moonwell needs Pkl 0.32 or newer (found: " + printed + ").", Hint: pklInstallHint}
}
