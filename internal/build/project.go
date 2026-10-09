package build

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
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
	pkl, err := toolchain.FindPkl(ctx, e)
	if err != nil {
		return nil, err
	}
	return manifest.Load(ctx, e, pkl)
}

func OpenSource(project *manifest.Project) (*mapdir.Folder, error) {
	mapDir, err := sourceMapDir(project)
	if err != nil {
		return nil, err
	}
	displayPath := mapsDir + "/" + mapDir
	fullPath, err := fsx.SafeJoinNoSymlinks(project.Root, displayPath)
	if err != nil {
		return nil, err
	}
	if !fsx.Exists(fullPath) {
		return nil, errNoMap(project.ManifestName, displayPath)
	}
	source, err := mapdir.Open(fullPath, displayPath)
	if isMissing(err) {
		return nil, errNoMap(project.ManifestName, displayPath)
	}
	return source, err
}

func sourceMapDir(project *manifest.Project) (string, error) {
	parts, fault := parseDir(project.Map.Folder)
	switch fault {
	case dirEscapes, dirEmpty:
		return "", errNotInsideMaps(project.ManifestName, project.Map.Folder)
	case dirNotPortable:
		return "", errUnusableMapFolder(project.ManifestName, project.Map.Folder)
	}
	return strings.Join(parts, "/"), nil
}

func isMissing(err error) bool {
	var diagErr *diag.Error
	return errors.Is(err, fs.ErrNotExist) && !errors.As(err, &diagErr)
}

func ReadMapGlobals(source *mapdir.Folder) (*lua.MapGlobals, error) {
	data, found, err := source.Read(scriptName)
	switch {
	case err != nil:
		return nil, err
	case found:
		globals := lua.ReadMapGlobals(string(data))
		return &globals, nil
	case source.IsDir(scriptName):
		return nil, errFolderForScript(source.CanonicalPath(scriptName), source.DisplayPath(scriptName))
	}
	return nil, nil
}

func CollectAssets(project *manifest.Project, synced []library.Synced) (collected []assets.Asset, replaced []string, err error) {
	libraries, err := librariesWithAssets(project.Root, synced)
	if err != nil {
		return nil, nil, err
	}
	return assets.Collect(project.Root, project.Assets, manifest.SharedManifest, libraries)
}

func librariesWithAssets(root string, synced []library.Synced) ([]assets.Library, error) {
	var libraries []assets.Library
	for _, syncedLibrary := range synced {
		if syncedLibrary.Assets == "" {
			continue
		}
		if _, err := fsx.SafeJoinNoSymlinks(root, syncedLibrary.Assets); err != nil {
			return nil, err
		}
		relPath, _ := fsx.CleanRelPath(syncedLibrary.Assets)
		libraries = append(libraries, assets.Library{Key: syncedLibrary.Key, Dir: filepath.Join(root, filepath.FromSlash(relPath))})
	}
	return libraries, nil
}

func AssetStatePath(project *manifest.Project) (string, error) {
	mapDir, err := sourceMapDir(project)
	if err != nil {
		return "", err
	}
	return assets.StateFilePath(project.Root, mapDir)
}

func errNotInsideMaps(manifestName, mapFolder string) error {
	return &diag.Error{
		Msg:  `map.folder must name a folder inside maps/, not "` + mapFolder + `".`,
		File: manifestName,
		Hint: "Set map.folder to the name of the map folder under maps/, such as map.w3x.",
	}
}

func errUnusableMapFolder(manifestName, mapFolder string) error {
	return &diag.Error{
		Msg:  `map.folder has a name that Windows cannot hold: "` + mapFolder + `".`,
		File: manifestName,
		Hint: "Set map.folder to the name of the map folder under maps/, such as map.w3x. " + unusableNames,
	}
}

func errNoMap(manifestName, displayPath string) error {
	return &diag.Error{
		Msg:  "Source map folder " + displayPath + " not found.",
		File: manifestName,
		Hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
	}
}

func errFolderForScript(dir, displayPath string) error {
	return &diag.Error{
		Msg:  dir + " in the map is a folder, not a file.",
		File: displayPath,
		Hint: "The map has a folder where its script belongs. Remove that folder from the source map, or save the " +
			"map in World Editor with Lua as the script language.",
	}
}
