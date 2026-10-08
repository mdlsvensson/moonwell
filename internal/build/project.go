package build

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

const (
	mapsDir    = "maps"
	sourcesDir = "src"
	scriptName = "war3map.lua"
)

func Load(ctx context.Context, e *env.Env) (*manifest.Project, error) {
	pkl, err := toolchain.PklProgram(ctx, e)
	if err != nil {
		return nil, err
	}
	return manifest.Load(ctx, e, pkl)
}

func Source(p *manifest.Project) (*mapdir.Folder, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return nil, err
	}
	label := mapsDir + "/" + folder
	dir, err := fsx.SafeJoinNoSymlinks(p.Root, label)
	if err != nil {
		return nil, err
	}
	if !fsx.Exists(dir) {
		return nil, errNoMap(p.ManifestName, label)
	}
	source, err := mapdir.Open(dir, label)
	if isMissing(err) {
		return nil, errNoMap(p.ManifestName, label)
	}
	return source, err
}

func mapFolder(p *manifest.Project) (string, error) {
	parts, fault := readFolder(p.Map.Folder)
	switch fault {
	case leavesItsFolder, namesNoFolder:
		return "", errNotInsideMaps(p.ManifestName, p.Map.Folder)
	case unusableName:
		return "", errUnusableMapFolder(p.ManifestName, p.Map.Folder)
	}
	return strings.Join(parts, "/"), nil
}

type folderFault int

const (
	noFault folderFault = iota
	leavesItsFolder
	namesNoFolder
	unusableName
)

func readFolder(written string) (parts []string, fault folderFault) {
	parts = partsOf(written)
	switch {
	case startsAtARoot(written) || slices.Contains(parts, ".."):
		return nil, leavesItsFolder
	case len(parts) == 0:
		return nil, namesNoFolder
	case !everySystemHolds(parts):
		return nil, unusableName
	}
	return parts, noFault
}

func everySystemHolds(parts []string) bool {
	_, portable := fsx.CleanRelPath(strings.Join(parts, "/"))
	return portable
}

func partsOf(written string) []string {
	var parts []string
	for part := range strings.SplitSeq(strings.ReplaceAll(written, `\`, "/"), "/") {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	return parts
}

func startsAtARoot(written string) bool {
	if strings.HasPrefix(written, "/") || strings.HasPrefix(written, `\`) {
		return true
	}
	if len(written) < 2 || written[1] != ':' {
		return false
	}
	letter := written[0] | 0x20
	return letter >= 'a' && letter <= 'z'
}

func isMissing(err error) bool {
	var expected *diag.Error
	return errors.Is(err, fs.ErrNotExist) && !errors.As(err, &expected)
}

func MapGlobals(source *mapdir.Folder) (*lua.MapGlobals, error) {
	script, found, err := source.Read(scriptName)
	switch {
	case err != nil:
		return nil, err
	case found:
		defined := lua.ReadMapGlobals(string(script))
		return &defined, nil
	case source.IsDir(scriptName):
		return nil, errFolderForScript(source.CanonicalPath(scriptName), source.DisplayPath(scriptName))
	}
	return nil, nil
}

func Assets(p *manifest.Project, synced []library.Synced) (found []assets.Asset, replaced []string, err error) {
	shipping, err := shippingLibraries(p.Root, synced)
	if err != nil {
		return nil, nil, err
	}
	return assets.Collect(p.Root, p.Assets, manifest.SharedManifest, shipping)
}

func shippingLibraries(root string, synced []library.Synced) ([]assets.Library, error) {
	var shipping []assets.Library
	for _, lib := range synced {
		if lib.Assets == "" {
			continue
		}
		if _, err := fsx.SafeJoinNoSymlinks(root, lib.Assets); err != nil {
			return nil, err
		}
		below, _ := fsx.CleanRelPath(lib.Assets)
		shipping = append(shipping, assets.Library{Key: lib.Key, Dir: filepath.Join(root, filepath.FromSlash(below))})
	}
	return shipping, nil
}

func OwnershipFile(p *manifest.Project) (string, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return "", err
	}
	return assets.StateFile(p.Root, folder)
}

func errNotInsideMaps(manifestFile, folder string) error {
	return &diag.Error{
		Msg:  `map.folder must name a folder inside maps/, not "` + folder + `".`,
		File: manifestFile,
		Hint: "Set map.folder to the name of the map folder under maps/, such as map.w3x.",
	}
}

const unusableNames = `A name cannot hold a control character or any of < > : " | ? *, end with a dot or a ` +
	"space, or be a device name such as CON or NUL."

func errUnusableMapFolder(manifestFile, folder string) error {
	return &diag.Error{
		Msg:  `map.folder has a name that Windows cannot hold: "` + folder + `".`,
		File: manifestFile,
		Hint: "Set map.folder to the name of the map folder under maps/, such as map.w3x. " + unusableNames,
	}
}

func errNoMap(manifestFile, label string) error {
	return &diag.Error{
		Msg:  "Source map folder " + label + " not found.",
		File: manifestFile,
		Hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
	}
}

func errFolderForScript(folder, file string) error {
	return &diag.Error{
		Msg:  folder + " in the map is a folder, not a file.",
		File: file,
		Hint: "The map has a folder where its script belongs. Remove that folder from the source map, or save the " +
			"map in World Editor with Lua as the script language.",
	}
}
