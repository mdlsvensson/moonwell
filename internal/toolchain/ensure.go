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

type Asset struct {
	URL, SHA256 string
	Archive     string
	Binary      string
}

type Tool struct {
	Name        string
	Title       string
	Versions    map[string]map[string]Asset
	VersionArgs []string
	Reported    *regexp.Regexp
	Otherwise   string
}

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

func ReportedVersion(ctx context.Context, e *env.Env, tool Tool, program string) (string, error) {
	printed, err := tool.ask(ctx, e, program, "")
	if err != nil {
		return "", err
	}
	return tool.versionIn(printed), nil
}

func (tool Tool) ask(ctx context.Context, e *env.Env, program, hint string) (env.RunResult, error) {
	return e.Run(ctx, program, tool.VersionArgs, env.RunOptions{Hint: hint})
}

func (tool Tool) versionIn(printed env.RunResult) string {
	if match := tool.Reported.FindStringSubmatch(printed.Stdout + printed.Stderr); match != nil {
		return match[1]
	}
	return ""
}

func orUnknown(version string) string {
	if version == "" {
		return "unknown"
	}
	return version
}

func sentence(text string) string {
	if text == "" {
		return ""
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

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
