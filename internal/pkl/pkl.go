// Package pkl finds the pkl program Moonwell runs: the one on PATH when it is Pkl 0.32 or newer, else a pinned Pkl
// that it downloads into the user's cache.
package pkl

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/logging"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

// Version is the Pkl Moonwell downloads when PATH has none it can use.
const Version = "0.32.1"

// OnPath is what Ensure returns for the pkl on PATH.
const OnPath = "pkl"

const installPage = "https://pkl-lang.org/main/current/pkl-cli/index.html#installation"

// InstallHint says how to get Pkl where Moonwell cannot download it.
const InstallHint = "Install Pkl 0.32 or newer: " + installPage

// Asset is a Pkl executable Moonwell can download.
type Asset struct {
	URL    string
	SHA256 string
	// Binary is the executable's name in the cache.
	Binary string
}

const releases = "https://github.com/apple/pkl/releases/download/" + Version + "/"

// Known are the executables of Version by platform, as yue.Platform names it, with checksums verified when the version
// was pinned (they match GitHub's digests of the release assets).
var Known = map[string]Asset{
	"windows-x86_64": {
		URL:    releases + "pkl-windows-amd64.exe",
		SHA256: "8550a00fcf027335e42c5e2cd553b88e98845408cb1880b3e3d1860caf46d22a",
		Binary: "pkl.exe",
	},
	"linux-x86_64": {
		URL:    releases + "pkl-linux-amd64",
		SHA256: "3180b62da95c0cad1d904e9bb6c5f4a8f9032413c21e53194bb91ff1ee5f3211",
		Binary: "pkl",
	},
}

// Deps is what finding and installing Pkl needs from the outside world.
type Deps struct {
	Fetch library.Fetch
	Run   proc.RunFunc
	// CacheRoot is the per-user folder downloaded tools are kept in.
	CacheRoot string
	// Platform is this machine's, as Known names it; "" for one Moonwell cannot download Pkl for.
	Platform string
	Known    map[string]Asset
	Log      *logging.Logger
}

var reportedVersion = regexp.MustCompile(`Pkl (\d+)\.(\d+)\.(\d+)`)

// version asks a pkl program for its version: the whole line it printed, or "unknown", and whether it is 0.32 or
// newer.
func version(ctx context.Context, program string, run proc.RunFunc) (found string, recent bool, err error) {
	result, err := run(ctx, program, []string{"--version"}, proc.Options{Hint: InstallHint})
	if err != nil {
		return "", false, err
	}
	found = text.Trim(result.Stdout)
	if found == "" {
		found = "unknown"
	}
	match := reportedVersion.FindStringSubmatch(result.Stdout)
	if match == nil {
		return found, false, nil
	}
	major, _ := strconv.Atoi(match[1])
	minor, _ := strconv.Atoi(match[2])
	return found, major > 0 || minor >= 32, nil
}

// Ensure returns the pkl program to run: OnPath when it is Pkl 0.32 or newer, else the path of the pinned Pkl, which
// it downloads into the cache the first time. An older pkl on PATH is reported with a warning.
func Ensure(ctx context.Context, deps Deps) (string, error) {
	found, recent, err := version(ctx, OnPath, deps.Run)
	if err == nil && recent {
		return OnPath, nil
	}
	var notStarted *diag.Error
	if err != nil && !errors.As(err, &notStarted) {
		return "", err
	}
	asset, supported := deps.Known[deps.Platform]
	if !supported {
		if err != nil {
			return "", err
		}
		return "", &diag.Error{Msg: "Moonwell needs Pkl 0.32 or newer (found: " + found + ").", Hint: InstallHint}
	}
	if err == nil {
		deps.Log.Warn("pkl on PATH is older than 0.32 (" + found + "), so Moonwell runs its own Pkl " + Version +
			". A pkl command you type yourself, such as `pkl project resolve`, still runs the old one.")
	}
	installDir := filepath.Join(deps.CacheRoot, "pkl", Version)
	binary := filepath.Join(installDir, asset.Binary)
	if fsx.Exists(binary) {
		return binary, nil
	}
	if err := install(ctx, asset, installDir, deps); err != nil {
		return "", err
	}
	return binary, nil
}

// install downloads the pinned Pkl, checks it, and moves it into installDir.
func install(ctx context.Context, asset Asset, installDir string, deps Deps) error {
	deps.Log.Info("Downloading Pkl " + Version + "...")
	status, data, err := deps.Fetch(ctx, asset.URL)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return &diag.Error{
			Msg: "Downloading " + asset.URL + " failed.", Cause: err,
			Hint: "Check your connection and retry, or install Pkl 0.32 or newer yourself: " + installPage,
		}
	}
	if status < 200 || status > 299 {
		return &diag.Error{
			Msg:  "Downloading " + asset.URL + " failed with HTTP " + strconv.Itoa(status) + ".",
			Hint: "Retry later, or install Pkl 0.32 or newer yourself: " + installPage,
		}
	}
	if actual := fsx.SHA256Hex(data); actual != asset.SHA256 {
		return &diag.Error{
			Msg: "Pkl download checksum mismatch (expected " + asset.SHA256 + ", got " + actual + ").",
			Hint: "Retry the download. If it keeps failing, report it, or install Pkl 0.32 or newer yourself; do not " +
				"bypass the check.",
		}
	}

	if err := os.MkdirAll(filepath.Dir(installDir), 0o777); err != nil {
		return err
	}
	staging, err := os.MkdirTemp(filepath.Dir(installDir), ".install-")
	if err != nil {
		return err
	}
	defer fsx.RemoveAll(staging)
	staged := filepath.Join(staging, asset.Binary)
	if err := os.WriteFile(staged, data, 0o777); err != nil {
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(staged, 0o755); err != nil {
			return err
		}
	}
	found, _, err := version(ctx, staged, deps.Run)
	if err != nil {
		return err
	}
	if match := reportedVersion.FindString(found); match != "Pkl "+Version {
		return &diag.Error{Msg: "Downloaded Pkl reports version " + found + ", expected " + Version + "."}
	}
	// A failed rename with Pkl in place means another process finished the same install first: keep theirs.
	if err := os.Rename(staging, installDir); err != nil && !fsx.Exists(filepath.Join(installDir, asset.Binary)) {
		return err
	}
	return nil
}

// KeepForShell is setup's step for the pinned Pkl: when Moonwell runs it (program is not OnPath), it copies it to
// <cacheRoot>/bin/ (InstallBin) and warns when `pkl` typed in a shell still finds no Pkl, with the command that puts
// that folder on PATH. An older pkl on PATH was already reported by Ensure.
func KeepForShell(ctx context.Context, program string, deps Deps, goos string) error {
	if program == OnPath {
		return nil
	}
	path, copied, err := InstallBin(program, deps.CacheRoot)
	if err != nil {
		return err
	}
	if copied {
		deps.Log.Info("Copied Pkl " + Version + " to " + path + ".")
	}
	_, err = deps.Run(ctx, OnPath, []string{"--version"}, proc.Options{})
	var notStarted *diag.Error
	if !errors.As(err, &notStarted) {
		return err
	}
	shell := "your shell"
	if goos == "windows" {
		shell = "PowerShell"
	}
	deps.Log.Warn("pkl is not on PATH, so a pkl command you type, such as `pkl project resolve`, finds no Pkl. " +
		"Run this once in " + shell + ", then open a new terminal:\n  " + yue.PathCommand(filepath.Dir(path), goos))
	return nil
}

// InstallBin copies the pinned Pkl to <cacheRoot>/bin/, the folder that also holds yue for the editor, so that a pkl
// command typed in a shell finds it once that folder is on PATH. It copies nothing when an identical copy is there.
func InstallBin(binary, cacheRoot string) (path string, copied bool, err error) {
	path = filepath.Join(cacheRoot, "bin", filepath.Base(binary))
	copied, err = fsx.CopyProgram(binary, path)
	if err != nil {
		return path, false, &diag.Error{
			Msg:   "Copying Pkl to " + path + " failed: " + fsx.Reason(err),
			Cause: err,
			Hint:  "Close the programs that run this copy of pkl (such as an editor's Pkl extension), then run moonwell setup again.",
		}
	}
	return path, copied, nil
}
