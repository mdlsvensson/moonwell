// Package build is the one place that knows in which order a map is built. It takes the outside world and a
// project, and returns the map as it will be staged; Build, Test, Check and Dev are that plan and one more step.
// It must not know how any area does its work, nor how a command line is read. It imports the areas, the
// foundations and the root package.
package build

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/assets"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/library"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
	"github.com/mdlsvensson/moonwell/next/internal/war3/lua"
)

const (
	// mapsDir is the folder of a project that holds its source maps, from the project folder.
	mapsDir = "maps"
	// scriptName is a map's script, by the name World Editor gives it. A map may spell it in another letter case.
	scriptName = "war3map.lua"
)

// Load finds Pkl and evaluates the manifest of the project in e.Root.
func Load(ctx context.Context, e *env.Env) (*manifest.Project, error) {
	pkl, err := toolchain.PklProgram(ctx, e)
	if err != nil {
		return nil, err
	}
	return loadWith(ctx, e, pkl)
}

// loadWith evaluates the manifest of the project in e.Root with the pkl program: Load for a command that has
// found Pkl already, and evaluates more than once.
func loadWith(ctx context.Context, e *env.Env, pkl string) (*manifest.Project, error) {
	return manifest.Load(ctx, e, pkl)
}

// Source opens the project's source map, maps/<map.folder>. Commands read it and never write into it, except
// assets:sync.
//
// A map.folder that names no folder inside maps/, and a folder that is not there, are refused with the manifest
// as their file: map.folder is what to put right. A link on the way to the folder, a name no folder can have and
// what a map folder cannot hold are refused by fsx.Inside and mapdir.Open, at the path they are found at; a
// file in the folder's place, such as a packed map, is among the last.
func Source(p *manifest.Project) (*mapdir.Folder, error) {
	label, err := mapLabel(p)
	if err != nil {
		return nil, err
	}
	dir, err := fsx.Inside(p.Root, label)
	if err != nil {
		return nil, err
	}
	// Nothing at the place is a map folder that is not there, and so is a file on the way to it, on every system.
	// What is at the place is mapdir.Open's to judge: a folder, or a file where the folder belongs.
	if !fsx.Exists(dir) {
		return nil, errNoMap(p.File, label)
	}
	source, err := mapdir.Open(dir, label)
	// A folder that is gone between the look and the scan is not there either.
	if isMissing(err) {
		return nil, errNoMap(p.File, label)
	}
	return source, err
}

// mapLabel is the source map's folder from the project folder, which is how errors name it: maps/ and map.folder
// as the manifest writes it. A map.folder that does not name a folder inside maps/ is refused.
func mapLabel(p *manifest.Project) (string, error) {
	if !staysBelow(p.Map.Folder) {
		return "", errNotInsideMaps(p.File, p.Map.Folder)
	}
	return mapsDir + "/" + p.Map.Folder, nil
}

// staysBelow reports whether a path, as a manifest writes one, names a place below the folder it starts from:
// not that folder itself, nothing above it and nothing beside it. The path is read as text, with "/" and "\"
// both separating, so the answer is the same on every system. A path from a root or from a drive names no place
// below any folder.
func staysBelow(written string) bool {
	slashed := strings.ReplaceAll(written, `\`, "/")
	if strings.HasPrefix(slashed, "/") || startsWithDrive(slashed) {
		return false
	}
	shortest := path.Clean(slashed)
	return shortest != "." && shortest != ".." && !strings.HasPrefix(shortest, "../")
}

// startsWithDrive reports whether a path starts with a drive of Windows: a letter and a colon.
func startsWithDrive(written string) bool {
	if len(written) < 2 || written[1] != ':' {
		return false
	}
	letter := written[0] | 0x20 // an ASCII letter in lower case
	return letter >= 'a' && letter <= 'z'
}

// isMissing reports whether err is that of mapdir.Open for a map folder that is not there: the system's error as
// it came. A folder below the map that is gone by the time it is listed is an expected failure, which names that
// folder.
func isMissing(err error) bool {
	var expected *diag.Error
	return errors.Is(err, fs.ErrNotExist) && !errors.As(err, &expected)
}

// MapGlobals is what the source map's war3map.lua defines, for the unknown-global check and the editor's
// declarations. It is nil for a map without a script.
//
// The script is read once, through the folder, so in any letter case. A folder under the script's name is no
// script, and is refused as a folder where the file belongs.
func MapGlobals(source *mapdir.Folder) (*lua.MapGlobals, error) {
	script, found, err := source.Read(scriptName)
	switch {
	case err != nil:
		return nil, err
	case found:
		defined := lua.ReadMapGlobals(string(script))
		return &defined, nil
	case source.IsFolder(scriptName):
		return nil, errFolderForScript(source.Name(scriptName), source.Label(scriptName))
	}
	return nil, nil
}

// Assets collects the map's own assets and the files the synced libraries ship, and the lines that say which of
// a library's files the map's own replace.
func Assets(p *manifest.Project, synced []library.Synced) (found []assets.Asset, replaced []string, err error) {
	shipping, err := shippingLibraries(p.Root, synced)
	if err != nil {
		return nil, nil, err
	}
	return assets.Collect(p.Root, p.Assets, p.File, shipping)
}

// shippingLibraries is the libraries that ship files for the map, each with the folder of those files on disk.
// The folder is reached from the project folder, so a link on the way to it is refused: .moonwell as a link, or
// the folder itself.
func shippingLibraries(root string, synced []library.Synced) ([]assets.Library, error) {
	var shipping []assets.Library
	for _, lib := range synced {
		if lib.Assets == "" {
			continue
		}
		dir, err := fsx.Inside(root, lib.Assets)
		if err != nil {
			return nil, err
		}
		shipping = append(shipping, assets.Library{Key: lib.Key, Dir: dir})
	}
	return shipping, nil
}

// ---- errors ----

func errNotInsideMaps(manifestFile, folder string) error {
	return &diag.Error{
		Msg:  `map.folder must name a folder inside maps/, not "` + folder + `".`,
		File: manifestFile,
		Hint: "Set map.folder to the name of the map folder under maps/, such as map.w3x.",
	}
}

func errNoMap(manifestFile, label string) error {
	return &diag.Error{
		Msg:  "Source map folder " + label + " not found.",
		File: manifestFile,
		Hint: "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
	}
}

// errFolderForScript names the folder as the map spells it.
func errFolderForScript(folder, file string) error {
	return &diag.Error{
		Msg:  folder + " in the map is a folder, not a file.",
		File: file,
		Hint: "The map has a folder where its script belongs. Remove that folder from the source map, or open and " +
			"re-save the map in World Editor.",
	}
}
