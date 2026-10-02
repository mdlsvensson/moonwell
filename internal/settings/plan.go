package settings

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

const (
	resaveMap = "Open and re-save the map in World Editor in folder format with Lua as the script language."
	bom       = "\xEF\xBB\xBF"
	// The minimap World Editor writes, which the game's map list shows, and the name a TGA picture takes its place
	// under.
	minimap    = "war3mapMap.blp"
	minimapTGA = "war3mapMap.tga"
)

var settingsFiles = []string{"war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt"}

// PlanOptions say how a settings plan names things in its errors.
type PlanOptions struct {
	// ManifestFile is the evaluated manifest, which errors that do not depend on the map name.
	ManifestFile string
	// SourceLabel is the folder that map-file errors name, such as maps/map.w3x when the plan is for a staged copy:
	// the user fixes the source, not the copy. Empty: errors name the path in the planned folder.
	SourceLabel string
	// Root is the project folder that settings.info.preview starts at.
	Root string
}

// readMapFile reads a map file; nil without an error only when an optional file does not exist. Errors name file,
// the label, not the path.
func readMapFile(path, file string, optional bool) ([]byte, error) {
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		return data, nil
	case errors.Is(err, fs.ErrNotExist) && optional:
		return nil, nil
	case errors.Is(err, fs.ErrNotExist):
		return nil, &diag.Error{
			Msg: "A map file needed by the configured settings is missing.", File: file, Cause: err, Hint: resaveMap,
		}
	}
	return nil, &diag.Error{
		Msg:   "Reading a map file for map settings failed: " + fsx.Reason(err),
		File:  file,
		Cause: err,
		Hint:  "Make sure this is a readable file, not a folder, and that no other program has it locked.",
	}
}

// decodeText decodes strict UTF-8, keeping a byte order mark aside so that it survives re-encoding.
func decodeText(data []byte, file string) (mark, text string, err error) {
	if !utf8.Valid(data) {
		return "", "", &diag.Error{Msg: "This map file is not valid UTF-8 text.", File: file, Hint: resaveMap}
	}
	if rest, found := strings.CutPrefix(string(data), bom); found {
		return bom, rest, nil
	}
	return "", string(data), nil
}

// Plan computes every file the settings change in the map folder mapDir, without writing anything. It returns
// changed files only, named relative to mapDir, in the order war3map.w3i, war3map.lua, war3mapMisc.txt,
// war3mapSkin.txt, and then the files of a preview picture: war3mapMinimap.blp (World Editor's minimap, kept),
// war3mapMap.blp (replaced by a BLP picture, removed for a TGA one) and war3mapMap.tga.
func Plan(mapDir string, s *Settings, options PlanOptions) ([]mapdir.Change, error) {
	if !s.Has() {
		return nil, nil
	}
	// Conflicts that do not depend on the map fail before any map file is read.
	manifestFile := options.ManifestFile
	if manifestFile == "" {
		manifestFile = "moonwell.pkl"
	}
	misc, err := GameplaySections(s, manifestFile)
	if err != nil {
		return nil, err
	}
	var picture *Picture
	if s.Preview != nil {
		if options.Root == "" {
			return nil, errors.New("settings.Plan needs the project folder to read settings.info.preview.")
		}
		if picture, err = LoadPreview(options.Root, *s.Preview, options.ManifestFile); err != nil {
			return nil, err
		}
	}
	dir, err := filepath.Abs(mapDir)
	if err != nil {
		return nil, err
	}
	label := func(name string) string {
		switch {
		case options.SourceLabel == "":
			return filepath.Join(dir, name)
		case name == "":
			return options.SourceLabel
		}
		return options.SourceLabel + "/" + name
	}

	wanted := settingsFiles
	if picture != nil {
		wanted = append(append([]string{}, settingsFiles...), minimap, minimapTGA, KeptMinimap)
	}
	var entries []string
	listed, err := os.ReadDir(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, &diag.Error{
			Msg:   "Reading the map folder for map settings failed: " + fsx.Reason(err),
			File:  label(""),
			Cause: err,
			Hint:  "Check that the map folder is readable.",
		}
	}
	for _, entry := range listed {
		entries = append(entries, entry.Name())
	}
	names, err := mapdir.Names(entries, wanted, label)
	if err != nil {
		return nil, err
	}
	// existing is the name a file has in the folder, or its usual name when the folder has none.
	existing := func(name string) string {
		if found, ok := names[mapdir.Key(name)]; ok {
			return found
		}
		return name
	}

	var changes []mapdir.Change
	extended := s.HasExtended()
	needsW3i := extended || !s.Info.empty() || !s.Loading.empty()
	needsLua := extended || s.Info.Name != nil || s.Info.Description != nil
	// A preview picture takes the minimap's place, so the map must have one, and the two names it adds must be free.
	minimapName, hasMinimap := names[mapdir.Key(minimap)]
	if picture != nil {
		if !hasMinimap {
			return nil, &diag.Error{
				Msg:  "The map has no " + minimap + ", the minimap whose place the preview picture takes.",
				File: label(minimap),
				Hint: "Open and save the map in World Editor, which writes the minimap.",
			}
		}
		for _, needed := range []string{KeptMinimap, minimapTGA} {
			if taken, ok := names[mapdir.Key(needed)]; ok {
				return nil, &diag.Error{
					Msg:  "The map already has " + taken + ", a name the preview picture needs.",
					File: label(taken),
					Hint: "Remove that file from the map: a build with settings.info.preview writes it.",
				}
			}
		}
	}

	w3iName := existing("war3map.w3i")
	w3iFile := label(w3iName)
	var patchedInfo []byte
	if needsW3i {
		info, err := readMapFile(filepath.Join(dir, w3iName), w3iFile, false)
		if err != nil {
			return nil, err
		}
		if patchedInfo, err = PatchMapInfo(info, s, w3iFile); err != nil {
			return nil, err
		}
		if !bytes.Equal(patchedInfo, info) {
			changes = append(changes, mapdir.Change{Name: w3iName, Bytes: patchedInfo})
		}
	}
	if needsLua || picture != nil {
		luaName := existing("war3map.lua")
		luaFile := label(luaName)
		data, err := readMapFile(filepath.Join(dir, luaName), luaFile, false)
		if err != nil {
			return nil, err
		}
		mark, source, err := decodeText(data, luaFile)
		if err != nil {
			return nil, err
		}
		lua := source
		if needsLua {
			if lua, err = PatchLua(lua, s, patchedInfo, luaFile, w3iFile); err != nil {
				return nil, err
			}
		}
		if picture != nil {
			if lua, err = PatchMinimapLua(lua, luaFile); err != nil {
				return nil, err
			}
		}
		if lua != source {
			changes = append(changes, mapdir.Change{Name: luaName, Bytes: []byte(mark + lua)})
		}
	}

	for _, file := range []struct {
		name     string
		sections *Sections
	}{{"war3mapMisc.txt", &misc}, {"war3mapSkin.txt", &s.GameInterface}} {
		if !hasEntries(file.sections) {
			continue
		}
		name := existing(file.name)
		data, err := readMapFile(filepath.Join(dir, name), label(name), true)
		if err != nil {
			return nil, err
		}
		mark, source := "", ""
		if data != nil {
			if mark, source, err = decodeText(data, label(name)); err != nil {
				return nil, err
			}
		}
		if merged := PatchText(source, *file.sections); data == nil || merged != source {
			changes = append(changes, mapdir.Change{Name: name, Bytes: []byte(mark + merged)})
		}
	}

	if picture != nil {
		kept, err := readMapFile(filepath.Join(dir, minimapName), label(minimapName), false)
		if err != nil {
			return nil, err
		}
		changes = append(changes, mapdir.Change{Name: KeptMinimap, Bytes: kept})
		if picture.Extension == "blp" {
			changes = append(changes, mapdir.Change{Name: minimapName, Bytes: picture.Bytes})
		} else {
			changes = append(changes,
				mapdir.Change{Name: minimapName, Remove: true},
				mapdir.Change{Name: minimapTGA, Bytes: picture.Bytes})
		}
	}
	return changes, nil
}

// Apply writes planned settings into the staged map folder. Only build and test call this, on dist/stage.
func Apply(mapDir string, changes []mapdir.Change) error {
	return mapdir.Apply(mapDir, changes, "Writing staged map settings failed.")
}

// MapDir returns the source map folder maps/<mapFolder> that settings are checked against: an existing real folder.
// It is the folder build and test stage, but checked more strictly than their staging, which only tests that the
// path exists: here map.folder must stay inside maps/ and no path segment below the project may be a symlink or
// junction. A missing folder names the manifest, as build does, since map.folder is what to fix.
func MapDir(root, mapFolder, manifestFile string) (string, error) {
	if manifestFile == "" {
		manifestFile = "moonwell.pkl"
	}
	maps, err := filepath.Abs(filepath.Join(root, "maps"))
	if err != nil {
		return "", err
	}
	inside, err := filepath.Rel(maps, fsx.Resolve(maps, mapFolder))
	label := "maps/" + mapFolder
	if err != nil || inside == "." || strings.Split(inside, string(filepath.Separator))[0] == ".." || filepath.IsAbs(inside) {
		return "", &diag.Error{
			Msg:  `map.folder must name a folder inside maps/, not "` + mapFolder + `".`,
			File: manifestFile,
			Hint: "Set map.folder to the name of the map folder under maps/, such as map.w3x.",
		}
	}
	dir, err := fsx.SafeJoin(root, label)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return "", &diag.Error{
			Msg:   "Source map folder " + label + " not found.",
			File:  manifestFile,
			Cause: err,
			Hint:  "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
		}
	}
	if err != nil {
		return "", &diag.Error{
			Msg:   "Reading the source map folder failed: " + fsx.Reason(err),
			File:  label,
			Cause: err,
			Hint:  "Check that the folder is readable.",
		}
	}
	if !info.IsDir() {
		return "", &diag.Error{
			Msg:  "Source map " + label + " is not a folder.",
			File: label,
			Hint: "Save the map in World Editor in folder format (File > Save Map As, Folder), or set map.folder to it.",
		}
	}
	return dir, nil
}
