// Package toolchain keeps the external programs Moonwell pins: the YueScript compiler and Pkl.
//
// Ensure takes a tool, a version and the outside world, and returns the path of that program in the user's cache;
// the first time it downloads the program, and it puts into the cache only one whose download has the pinned
// SHA-256 and that reports the version asked for. Compiler and PklProgram are the two tools' own rules for a
// program the user provides: they take what a manifest says of the compiler, or nothing, and return the program to
// run. The functions of shell.go take such a program and keep a copy where a shell and an editor find it; they
// return the copy's path, the command that puts its folder on PATH, and log what a user still has to do.
//
// It must not know a project, a manifest or a build: a caller hands it the version and the yue.path that a
// manifest names. It runs no program and downloads nothing by itself: both are asked of the Env it is handed. Of
// the environment it reads one variable itself, SystemRoot: the tar.exe of Windows is run by its full path, since
// a tar on PATH may be another program, and the Env has no door to the environment.
//
// Of Moonwell it imports env, diag and fsx.
package toolchain

import (
	"context"
	"maps"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// Asset is one download of a tool, for one platform.
type Asset struct {
	URL, SHA256 string
	Archive     string // "" for a bare executable, "zip" or "7z"
	Binary      string // the program's file: its name in the cache, and its path inside an archive
}

// Tool is an external program Moonwell pins.
type Tool struct {
	Name        string                      // the program's name on PATH: "yue", "pkl"
	Title       string                      // how messages name it: "YueScript", "Pkl"
	Versions    map[string]map[string]Asset // by version, then by platform as env.Platform names it
	VersionArgs []string                    // what makes the program print its version
	Reported    *regexp.Regexp              // finds the version in what it prints, as the first group
	// Otherwise is what a user can do in place of the download, for hints. It ends a sentence that a hint begins
	// ("Retry later, or …"), so it starts in lower case and carries its own ending: a full stop, or an address.
	Otherwise string
}

// Ensure returns the path of the tool at this version in the user's cache, downloading it the first time: it
// checks the SHA-256, unpacks, asks the program for its version and only then moves it into place.
//
// The program is unpacked and asked in a staging folder beside its place in the cache, which is removed whatever
// happens, and named in a warning when it cannot be: a download that fails a check is never moved into place and
// never returned. A program that is in the cache is returned as it is, without a download and without being
// asked.
func Ensure(ctx context.Context, e *env.Env, tool Tool, version string) (path string, err error) {
	asset, err := tool.asset(version, e.Platform)
	if err != nil {
		return "", err
	}
	in := install{ctx: ctx, e: e, tool: tool, version: version, asset: asset}
	in.target = filepath.Join(e.CacheDir, tool.Name, version)
	if cached := in.programIn(in.target); fsx.Exists(cached) {
		return cached, nil
	}
	download, err := in.download()
	if err != nil {
		return "", err
	}
	if err := in.verify(download); err != nil {
		return "", err
	}
	staging, err := in.stage()
	if err != nil {
		return "", err
	}
	defer in.discard(staging)
	if err := in.unpack(download, staging); err != nil {
		return "", err
	}
	if err := in.askVersion(staging); err != nil {
		return "", err
	}
	return in.moveIntoPlace(staging)
}

// asset is the download of this version for this platform.
func (tool Tool) asset(version, platform string) (Asset, error) {
	platforms, known := tool.Versions[version]
	if !known {
		return Asset{}, errUnknownVersion(tool, version)
	}
	asset, supported := platforms[platform]
	if !supported {
		return Asset{}, errNoDownload(tool)
	}
	return asset, nil
}

// ReportedVersion asks a program for its version; "" when what it prints names none.
func ReportedVersion(ctx context.Context, e *env.Env, tool Tool, program string) (string, error) {
	printed, err := tool.ask(ctx, e, program, "")
	if err != nil {
		return "", err
	}
	return tool.versionIn(printed), nil
}

// ask runs a program of this tool with the arguments that make it print its version. hint is what a user reads
// when the program cannot be started.
func (tool Tool) ask(ctx context.Context, e *env.Env, program, hint string) (env.RunResult, error) {
	return e.Run(ctx, program, tool.VersionArgs, env.RunOptions{Hint: hint})
}

// versionIn is the version in what a program printed, on either stream; "" when it names none.
func (tool Tool) versionIn(printed env.RunResult) string {
	if match := tool.Reported.FindStringSubmatch(printed.Stdout + printed.Stderr); match != nil {
		return match[1]
	}
	return ""
}

// orUnknown is a version as a message shows it.
func orUnknown(version string) string {
	if version == "" {
		return "unknown"
	}
	return version
}

// sentence is text that ends a sentence, as a sentence of its own.
func sentence(text string) string {
	if text == "" {
		return ""
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

// machine names this machine as "os/architecture", with the architecture names x86_64 and aarch64.
func machine() string {
	arch := runtime.GOARCH
	switch arch {
	case "amd64":
		arch = "x86_64"
	case "arm64":
		arch = "aarch64"
	}
	return runtime.GOOS + "/" + arch
}

// ---- errors ----

// errUnknownVersion names the manifest, where a project says which compiler it wants. Only the compiler's
// version comes from a manifest: an unknown version of Pkl is a caller's bug, and is worded the same.
func errUnknownVersion(tool Tool, version string) error {
	known := slices.Sorted(maps.Keys(tool.Versions))
	return &diag.Error{
		Msg:  "Unknown " + tool.Title + " version " + version + ". Known versions: " + strings.Join(known, ", ") + ".",
		File: "moonwell.pkl",
		Hint: "Use a known version, or " + tool.Otherwise,
	}
}

func errNoDownload(tool Tool) error {
	return &diag.Error{
		Msg:  "Moonwell cannot download " + tool.Title + " for this platform (" + machine() + ").",
		Hint: sentence(tool.Otherwise),
	}
}
