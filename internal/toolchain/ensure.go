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
	Name              string
	Title             string
	Versions          map[string]map[string]Asset
	VersionArgs       []string
	VersionPattern    *regexp.Regexp
	ManualInstallHint string
}

func Ensure(ctx context.Context, e *env.Env, tool Tool, version string) (path string, err error) {
	asset, err := tool.assetFor(version, e.Platform)
	if err != nil {
		return "", err
	}
	inst := installer{ctx: ctx, e: e, tool: tool, version: version, asset: asset}
	inst.target = filepath.Join(e.CacheDir, tool.Name, version)
	if cached := inst.programPath(inst.target); fsx.Exists(cached) {
		return cached, nil
	}
	download, err := inst.download()
	if err != nil {
		return "", err
	}
	if err := inst.verify(download); err != nil {
		return "", err
	}
	staging, err := inst.makeStagingDir()
	if err != nil {
		return "", err
	}
	defer inst.removeStagingDir(staging)
	if err := inst.unpack(download, staging); err != nil {
		return "", err
	}
	if err := inst.verifyVersion(staging); err != nil {
		return "", err
	}
	return inst.moveIntoPlace(staging)
}

func (tool Tool) assetFor(version, platform string) (Asset, error) {
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

func QueryVersion(ctx context.Context, e *env.Env, tool Tool, program string) (string, error) {
	output, err := tool.runVersionCommand(ctx, e, program, "")
	if err != nil {
		return "", err
	}
	return tool.parseVersion(output), nil
}

func (tool Tool) runVersionCommand(ctx context.Context, e *env.Env, program, hint string) (env.RunResult, error) {
	return e.Run(ctx, program, tool.VersionArgs, env.RunOptions{Hint: hint})
}

func (tool Tool) parseVersion(output env.RunResult) string {
	if match := tool.VersionPattern.FindStringSubmatch(output.Stdout + output.Stderr); match != nil {
		return match[1]
	}
	return ""
}

func versionOrUnknown(version string) string {
	if version == "" {
		return "unknown"
	}
	return version
}

func capitalize(text string) string {
	if text == "" {
		return ""
	}
	return strings.ToUpper(text[:1]) + text[1:]
}

func platformName() string {
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
		Hint: "Use a known version, or " + tool.ManualInstallHint,
	}
}

func errNoDownload(tool Tool) error {
	return &diag.Error{
		Msg:  "Moonwell cannot download " + tool.Title + " for this platform (" + platformName() + ").",
		Hint: capitalize(tool.ManualInstallHint),
	}
}
