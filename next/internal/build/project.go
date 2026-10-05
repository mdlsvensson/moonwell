package build

import (
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
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
// as their file: map.folder is what to put right. A link on the way to the folder, a name Windows cannot hold
// and what a map folder cannot hold are refused by fsx.Inside and mapdir.Open, at the path they are found at; a
// file in the folder's place, such as a packed map, is among the last.
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
// maps/, and is refused with the manifest as its file. No ".." is resolved against the part before it.
func mapFolder(p *manifest.Project) (string, error) {
	parts := partsOf(p.Map.Folder)
	if leavesItsFolder(p.Map.Folder, parts) || len(parts) == 0 {
		return "", errNotInsideMaps(p.File, p.Map.Folder)
	}
	return strings.Join(parts, "/"), nil
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
// a library's files the map's own replace.
func Assets(p *manifest.Project, synced []library.Synced) (found []assets.Asset, replaced []string, err error) {
	shipping, err := shippingLibraries(p.Root, synced)
	if err != nil {
		return nil, nil, err
	}
	return assets.Collect(p.Root, p.Assets, p.File, shipping)
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

// StateFile is the file that records which files of the project's source map assets:sync owns. It is named by
// the map's folder as mapFolder reads it, so every way to write one folder names one file; a map.folder that
// names no folder inside maps/ is refused as Source refuses it. Nothing need be at the place.
func StateFile(p *manifest.Project) (string, error) {
	folder, err := mapFolder(p)
	if err != nil {
		return "", err
	}
	return assets.StateFile(p.Root, folder)
}

// outputAt is the way to every place Moonwell writes what it builds: relative is the place from the project
// folder, with "/", such as dist/.lock. Nothing need be at the place.
//
// The place is reached with fsx.Inside, so every folder on the way to it is a real folder of the project, the
// first too: what a build writes, and what it removes to write it, stays in the project. A link on the way, or
// at the place, is refused in words of this package's own, since what fsx.Inside says of a link is said of files
// the user keeps, and these are Moonwell's to make. fsx.Inside's other refusals are passed on: a path that leaves
// the project, a name Windows cannot hold, and a way the system cannot look at.
func outputAt(root, relative string) (string, error) {
	place, err := fsx.Inside(root, relative)
	if err == nil {
		return place, nil
	}
	if link, found := linkOnTheWay(root, relative); found {
		return "", errLinkedOutput(link, relative)
	}
	return "", err
}

// linkOnTheWay is the first step of the way to relative that is a link, as a path from the project folder with
// "/". It is found by a look at each step, the first step first, and not by what a refusal says: fsx names no
// kind of refusal. A path that fsx.RelPath does not take has no steps to look at.
func linkOnTheWay(root, relative string) (link string, found bool) {
	slashed, portable := fsx.RelPath(relative)
	if !portable {
		return "", false
	}
	for end := 1; end <= len(slashed); end++ {
		if end < len(slashed) && slashed[end] != '/' {
			continue
		}
		info, err := fsx.Lstat(filepath.Join(root, filepath.FromSlash(slashed[:end])))
		if err != nil || info == nil {
			return "", false // nothing is below a step that is not there, or that cannot be looked at
		}
		if fsx.IsLink(info) {
			return slashed[:end], true
		}
	}
	return "", false
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

// errLinkedOutput is the refusal of a link on the way to a place Moonwell writes: link is the step that is one,
// and file the place, both from the project folder.
func errLinkedOutput(link, file string) error {
	return &diag.Error{
		Msg:  link + " is a link: Moonwell writes what it builds into real files and folders.",
		File: file,
		Hint: "Remove the link (or Windows junction) at " + link + ", then try again: Moonwell makes what it needs " +
			"there.",
	}
}
