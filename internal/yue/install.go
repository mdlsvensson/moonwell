package yue

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/logging"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// InstallDeps is what installing a compiler needs from the outside world.
type InstallDeps struct {
	Fetch library.Fetch
	Run   proc.RunFunc
	// CacheRoot is the per-user folder compilers are kept in.
	CacheRoot string
	// Platform is this machine's, as Known names it; "" for one Moonwell cannot download a compiler for.
	Platform string
	Known    map[string]map[string]Asset
	Log      *logging.Logger
}

// DefaultCacheRoot is the per-user cache: MOONWELL_CACHE, else %LOCALAPPDATA%\moonwell on Windows, else
// $XDG_CACHE_HOME/moonwell or ~/.cache/moonwell.
func DefaultCacheRoot() string {
	if override := os.Getenv("MOONWELL_CACHE"); override != "" {
		return override
	}
	if runtime.GOOS == "windows" {
		if local := os.Getenv("LOCALAPPDATA"); local != "" {
			return filepath.Join(local, "moonwell")
		}
	}
	if xdg := os.Getenv("XDG_CACHE_HOME"); xdg != "" {
		return filepath.Join(xdg, "moonwell")
	}
	home, isSet := os.LookupEnv("HOME")
	if !isSet {
		if home, isSet = os.LookupEnv("USERPROFILE"); !isSet {
			home = "."
		}
	}
	return filepath.Join(home, ".cache", "moonwell")
}

// DefaultInstallDeps downloads over HTTP into the user's cache, for this machine.
func DefaultInstallDeps(log *logging.Logger, run proc.RunFunc) InstallDeps {
	return InstallDeps{
		Fetch:     library.HTTPFetch(http.DefaultClient),
		Run:       run,
		CacheRoot: DefaultCacheRoot(),
		Platform:  CurrentPlatform(),
		Known:     Known,
		Log:       log,
	}
}

var reportedVersion = regexp.MustCompile(`Yuescript version: ([^` + text.SpaceSet + `]+)`)

// Version asks a compiler for its version; "" when its answer names none.
func Version(ctx context.Context, binary string, run proc.RunFunc) (string, error) {
	result, err := run(ctx, binary, []string{"-v"}, proc.Options{})
	if err != nil {
		return "", err
	}
	if match := reportedVersion.FindStringSubmatch(result.Stdout + result.Stderr); match != nil {
		return match[1], nil
	}
	return "", nil
}

func orUnknown(version string) string {
	if version == "" {
		return "unknown"
	}
	return version
}

// Ensure returns the path of a verified compiler of this version, installing it into the cache when needed. path is
// the manifest's yue.path: a compiler the user provides, which is used as it is.
func Ensure(ctx context.Context, version string, path *string, deps InstallDeps) (string, error) {
	if path != nil {
		if !fsx.Exists(*path) {
			return "", &diag.Error{Msg: "yue.path does not exist: " + *path, File: "moonwell.local.pkl"}
		}
		found, err := Version(ctx, *path, deps.Run)
		if err != nil {
			return "", err
		}
		if found != version {
			deps.Log.Warn("yue.path reports version " + orUnknown(found) + ", expected " + version + ".")
		}
		return *path, nil
	}

	platforms, known := deps.Known[version]
	if !known {
		versions := make([]string, 0, len(deps.Known))
		for name := range deps.Known {
			versions = append(versions, name)
		}
		text.Sort(versions)
		return "", &diag.Error{
			Msg:  "Unknown YueScript version " + version + ". Known versions: " + strings.Join(versions, ", ") + ".",
			File: "moonwell.pkl",
			Hint: "Use a known version, or set yue.path to a local compiler.",
		}
	}
	asset, supported := platforms[deps.Platform]
	if !supported {
		return "", &diag.Error{
			Msg:  "Moonwell cannot download YueScript for this platform (" + machine() + ").",
			Hint: "Build or install yue yourself and set yue.path in moonwell.local.pkl.",
		}
	}

	installDir := filepath.Join(deps.CacheRoot, "yue", version)
	binary := filepath.Join(installDir, filepath.FromSlash(asset.Binary))
	if fsx.Exists(binary) {
		return binary, nil
	}

	deps.Log.Info("Downloading YueScript " + version + "...")
	status, archive, err := deps.Fetch(ctx, asset.URL)
	if err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", &diag.Error{
			Msg: "Downloading " + asset.URL + " failed.", Cause: err, Hint: "Check your connection and retry, or set yue.path.",
		}
	}
	if status < 200 || status > 299 {
		return "", &diag.Error{
			Msg:  "Downloading " + asset.URL + " failed with HTTP " + strconv.Itoa(status) + ".",
			Hint: "Retry later, or set yue.path in moonwell.local.pkl.",
		}
	}
	if actual := fsx.SHA256Hex(archive); actual != asset.SHA256 {
		return "", &diag.Error{
			Msg: "YueScript download checksum mismatch (expected " + asset.SHA256 + ", got " + actual + ").",
			Hint: "Retry the download. If it keeps failing, report it, or build yue yourself and set yue.path in " +
				"moonwell.local.pkl; do not bypass the check.",
		}
	}

	if err := os.MkdirAll(filepath.Dir(installDir), 0o777); err != nil {
		return "", err
	}
	staging, err := os.MkdirTemp(filepath.Dir(installDir), ".install-")
	if err != nil {
		return "", err
	}
	defer fsx.RemoveAll(staging)
	stagedBinary := filepath.Join(staging, filepath.FromSlash(asset.Binary))
	if err := unpack(ctx, archive, asset, staging, deps.Run); err != nil {
		return "", err
	}
	if !fsx.Exists(stagedBinary) {
		return "", &diag.Error{Msg: "The YueScript archive has no " + asset.Binary + "."}
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(stagedBinary, 0o755); err != nil {
			return "", err
		}
	}
	found, err := Version(ctx, stagedBinary, deps.Run)
	if err != nil {
		return "", err
	}
	if found != version {
		return "", &diag.Error{Msg: "Downloaded compiler reports version " + orUnknown(found) + ", expected " + version + "."}
	}
	// A failed rename with the compiler in place means another process finished the same install first: keep theirs.
	if err := os.Rename(staging, installDir); err != nil && !fsx.Exists(binary) {
		return "", err
	}
	return binary, nil
}

// unpack puts the compiler of a downloaded archive into staging.
func unpack(ctx context.Context, archive []byte, asset Asset, staging string, run proc.RunFunc) error {
	if asset.Archive == "zip" {
		// Only the compiler is extracted, so the archive's entry names can never write outside staging.
		data, err := zipEntry(archive, asset.Binary)
		if err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(staging, filepath.FromSlash(asset.Binary)), data, 0o666)
	}
	// Windows has no 7-Zip reader of its own but its tar.exe (libarchive) reads the format.
	archivePath := filepath.Join(staging, "archive.7z")
	if err := os.WriteFile(archivePath, archive, 0o666); err != nil {
		return err
	}
	systemRoot := os.Getenv("SystemRoot")
	if systemRoot == "" {
		systemRoot = `C:\Windows`
	}
	tar := filepath.Join(systemRoot, "System32", "tar.exe")
	result, err := run(ctx, tar, []string{"-xf", archivePath, "-C", staging}, proc.Options{})
	if err != nil {
		return err
	}
	if result.Code != 0 {
		return &diag.Error{Msg: "Extracting YueScript failed:\n" + text.Trim(result.Stderr)}
	}
	return os.Remove(archivePath)
}

// zipEntry reads one file of a zip archive.
func zipEntry(archive []byte, name string) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil && reader == nil {
		return nil, &diag.Error{Msg: "Invalid zip archive: " + strings.TrimPrefix(err.Error(), "zip: ") + "."}
	}
	for _, entry := range reader.File {
		if entry.Name != name {
			continue
		}
		file, err := entry.Open()
		if err != nil {
			return nil, &diag.Error{Msg: "Invalid zip archive: " + name + " cannot be read: " + err.Error() + "."}
		}
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, &diag.Error{Msg: "Invalid zip archive: " + name + " cannot be read: " + err.Error() + "."}
		}
		return data, nil
	}
	return nil, &diag.Error{Msg: "The YueScript archive has no " + name + "."}
}
