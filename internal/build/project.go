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

// This file holds a project as a build opens it: the manifest, the source map and how map.folder names it, what
// the map's script defines, the assets, and the file that records which files of the map assets:sync owns.

const (
	// mapsDir is the folder of a project that holds its source maps, from the project folder.
	mapsDir = "maps"
	// sourcesDir is the folder of a project's gameplay, from the project folder.
	sourcesDir = "src"
	// scriptName is a map's script, by the name World Editor gives it. A map may spell it in another letter case.
	scriptName = "war3map.lua"
	// sharedManifest is the manifest every project has, which a manifest of one machine amends. The assets block
	// is written in it, so an error about that block names it, whichever manifest was evaluated.
	sharedManifest = "moonwell.pkl"
)

// Load finds Pkl and evaluates the manifest of the project in e.Root.
func Load(ctx context.Context, e *env.Env) (*manifest.Project, error) {
	pkl, err := toolchain.PklProgram(ctx, e)
	if err != nil {
		return nil, err
	}
	return manifest.Load(ctx, e, pkl)
}

// Source opens the project's source map, maps/<map.folder>. Commands read it and never write into it, except
// assets:sync.
//
// A map.folder that names no folder inside maps/, one with a name Windows cannot hold, and a folder that is not
// there, are refused with the manifest as their file: map.folder is what to put right. A link on the way to the
// folder and what a map folder cannot hold are refused by fsx.Inside and mapdir.Open, at the path they are found
// at; a file in the folder's place, such as a packed map, is among the last.
func Source(p *manifest.Project) (*mapdir.Folder, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return nil, err
	}
	label := mapsDir + "/" + folder
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

// mapFolder is the project's map.folder as everything in this package reads it: the folder below maps/, and
// below the stage, with "/" between its parts, such as "campaign/one.w3x". Its last part is the map's name.
//
// The value is read by the rule of schema/Project.pkl (segments, isRelativeFolder), as text, so that the answer
// is the same on every system: it is split at "/" and "\", and its empty and "." parts are dropped. A value that
// starts with a separator or a drive, one with a ".." part, and one with no part left names no folder inside
// maps/, and is refused with the manifest as its file. No ".." is resolved against the part before it. So is a
// value with a name Windows cannot hold: the map, its stage and its archive are named by it on every system.
func mapFolder(p *manifest.Project) (string, error) {
	parts := partsOf(p.Map.Folder)
	switch {
	case leavesItsFolder(p.Map.Folder, parts), len(parts) == 0:
		return "", errNotInsideMaps(p.File, p.Map.Folder)
	case !everySystemHolds(parts):
		return "", errUnusableMapFolder(p.File, p.Map.Folder)
	}
	return strings.Join(parts, "/"), nil
}

// everySystemHolds reports whether a folder of these parts is one every system can make: whether each part is a
// name fsx.RelPath takes, which Windows can hold. The schema lets a folder with another name through, and
// fsx.Inside refuses the place of such a one in words for a file of the user's; a manifest's folder is refused
// by the manifest, with the value.
func everySystemHolds(parts []string) bool {
	_, portable := fsx.RelPath(strings.Join(parts, "/"))
	return portable
}

// leavesItsFolder reports whether a path, as a manifest writes one, is no path from the folder it is written
// for: one that starts at a root, and one of whose parts, which are partsOf it, is "..".
func leavesItsFolder(written string, parts []string) bool {
	return startsAtARoot(written) || slices.Contains(parts, "..")
}

// partsOf is the parts of a path as a manifest writes one: what stands between its separators, "/" and "\",
// without the parts that are empty or ".".
func partsOf(written string) []string {
	var parts []string
	for part := range strings.SplitSeq(strings.ReplaceAll(written, `\`, "/"), "/") {
		if part != "" && part != "." {
			parts = append(parts, part)
		}
	}
	return parts
}

// startsAtARoot reports whether a path, as a manifest writes one, starts with a separator or with a drive of
// Windows, which is an ASCII letter and a colon: such a path is not one from the folder it is written for.
func startsAtARoot(written string) bool {
	if strings.HasPrefix(written, "/") || strings.HasPrefix(written, `\`) {
		return true
	}
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
// a library's files the map's own replace. A mistake in the assets block names the shared manifest, as the hint
// of such a mistake does.
func Assets(p *manifest.Project, synced []library.Synced) (found []assets.Asset, replaced []string, err error) {
	shipping, err := shippingLibraries(p.Root, synced)
	if err != nil {
		return nil, nil, err
	}
	return assets.Collect(p.Root, p.Assets, sharedManifest, shipping)
}

// shippingLibraries is the libraries that ship files for the map, each with the folder of those files. The way
// to the folder is checked from the project folder, so a link on it is refused: .moonwell as a link, or the
// folder itself. The folder is then named as the project folder is, with the library's path joined to it, and not
// by the place fsx.Inside gives: assets.Collect names a library's folder from the project folder in its errors,
// which it can for a project folder of any form only when both are of that form.
func shippingLibraries(root string, synced []library.Synced) ([]assets.Library, error) {
	var shipping []assets.Library
	for _, lib := range synced {
		if lib.Assets == "" {
			continue
		}
		if _, err := fsx.Inside(root, lib.Assets); err != nil {
			return nil, err
		}
		// fsx.Inside took the path, so it is one that fsx.RelPath takes.
		below, _ := fsx.RelPath(lib.Assets)
		shipping = append(shipping, assets.Library{Key: lib.Key, Dir: filepath.Join(root, filepath.FromSlash(below))})
	}
	return shipping, nil
}

// StateFile is the file that records which files of the project's source map assets:sync owns, by its path from
// the project folder. It is named by the map's folder as mapFolder reads it, so every way to write one folder
// names one file; a map.folder that names no folder inside maps/ is refused as Source refuses it. Nothing need
// be at the place.
func StateFile(p *manifest.Project) (string, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return "", err
	}
	return assets.StateFile(p.Root, folder)
}

// ---- errors ----

func errNotInsideMaps(manifestFile, folder string) error {
	return &diag.Error{
		Msg:  `map.folder must name a folder inside maps/, not "` + folder + `".`,
		File: manifestFile,
		Hint: "Set map.folder to the name of the map folder under maps/, such as map.w3x.",
	}
}

// unusableNames ends a refusal of a folder of the manifest that has a name Windows cannot hold.
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

// errFolderForScript names the folder as the map spells it. Its words are those script has for the same folder
// when it adds the program to the map's script.
func errFolderForScript(folder, file string) error {
	return &diag.Error{
		Msg:  folder + " in the map is a folder, not a file.",
		File: file,
		Hint: "The map has a folder where its script belongs. Remove that folder from the source map, or save the " +
			"map in World Editor with Lua as the script language.",
	}
}
