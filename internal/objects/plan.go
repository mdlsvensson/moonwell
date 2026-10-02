package objects

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Plan is what the manifest's objects change in a map.
type Plan struct {
	// Changes holds changed files only: the main files in the order w3u, w3t, w3h, w3a, w3q, then the skin files
	// in that order. Names are relative to the map folder, in the spelling the files have there.
	Changes []mapdir.Change
	// Generated is the content of src/generated/objects.yue.
	Generated string
	Objects   []Resolved
}

// PlanOptions say what a plan reads and how it names things in its errors.
type PlanOptions struct {
	// Metadata is the game metadata; the embedded copy when nil. Tests pass miniature metadata.
	Metadata *Metadata
	// Manifest is the evaluated manifest, which a missing map folder names (map.folder is what to fix).
	Manifest string
	// SourceLabel is the source map folder as map-file errors name it, such as maps/map.w3x, even when planning a
	// copy.
	SourceLabel string
}

// The modification-file family each category's objects go to.
var extensionOf = map[Category]string{
	"heroes": "w3u", "units": "w3u", "buildings": "w3u", "items": "w3t", "buffs": "w3h", "abilities": "w3a",
	"upgrades": "w3q",
}

var extensions = []string{"w3u", "w3t", "w3h", "w3a", "w3q"}

const (
	// Maps saved before skin files (versions 1 and 2) keep every field in the main file.
	firstSkinVersion = 3
	resaveObjects    = "Open and re-save this map in World Editor 3.00."
)

func mainFile(extension string) string { return "war3map." + extension }
func skinFile(extension string) string { return "war3mapSkin." + extension }

type sourceFile struct {
	name   string
	data   []byte
	parsed *ModFile
}

func readSourceFile(dir, name, file string) (*sourceFile, error) {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return nil, &diag.Error{
			Msg:   "Reading a map file for object data failed: " + fsx.Reason(err),
			File:  file,
			Cause: err,
			Hint:  "Make sure this is a readable file, not a folder, and that no other program has it locked.",
		}
	}
	parsed, err := ReadModFile(data, KindOf(name), file)
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, object := range parsed.Custom.Objects {
		if seen[object.ID] {
			return nil, &diag.Error{
				Msg: "Custom object '" + object.ID + "' appears twice in this file.", File: file, Hint: resaveObjects,
			}
		}
		seen[object.ID] = true
	}
	return &sourceFile{name: name, data: data, parsed: parsed}, nil
}

// PlanObjects computes the object files the manifest's objects change in the source map folder mapDir, without
// writing anything. With no objects it returns an empty plan without reading the map. Otherwise it reads the ten
// object files, resolves and validates the objects against them (failing with every problem), and appends each
// object, sorted by id, to its main file and, for maps with skin files, to its skin file, as World Editor writes
// both.
func PlanObjects(mapDir string, manifest Manifest, options PlanOptions) (*Plan, error) {
	if manifest.Empty() {
		return &Plan{Generated: RenderIDs(nil), Objects: []Resolved{}}, nil
	}
	metadata := options.Metadata
	if metadata == nil {
		metadata = LoadMetadata()
	}
	dir, err := filepath.Abs(mapDir)
	if err != nil {
		return nil, err
	}
	label := func(name string) string { return options.SourceLabel + "/" + name }

	listed, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &diag.Error{
			Msg:   "Source map folder " + options.SourceLabel + " not found.",
			File:  options.Manifest,
			Cause: err,
			Hint:  "Set map.folder to a folder under maps/ saved by World Editor in folder format.",
		}
	}
	if err != nil {
		return nil, &diag.Error{
			Msg:   "Reading the map folder for object data failed: " + fsx.Reason(err),
			File:  options.SourceLabel,
			Cause: err,
			Hint:  "Check that the map folder is readable.",
		}
	}
	var entries, objectFiles []string
	for _, entry := range listed {
		entries = append(entries, entry.Name())
	}
	for _, extension := range extensions {
		objectFiles = append(objectFiles, mainFile(extension))
	}
	for _, extension := range extensions {
		objectFiles = append(objectFiles, skinFile(extension))
	}
	found, err := mapdir.Names(entries, objectFiles, label)
	if err != nil {
		return nil, err
	}

	sources := map[string]*sourceFile{}
	existingIDs := map[string]bool{}
	for _, canonical := range objectFiles {
		name, ok := found[mapdir.Key(canonical)]
		if !ok {
			continue
		}
		source, err := readSourceFile(dir, name, label(name))
		if err != nil {
			return nil, err
		}
		sources[canonical] = source
		for _, object := range source.parsed.Custom.Objects {
			existingIDs[object.ID] = true
		}
	}
	resolved, err := Resolve(metadata, manifest, existingIDs)
	if err != nil {
		return nil, err
	}

	var mainChanges, skinChanges []mapdir.Change
	for _, extension := range extensions {
		var family []Resolved
		for _, object := range resolved {
			if extensionOf[object.Category] == extension {
				family = append(family, object)
			}
		}
		if len(family) == 0 {
			continue
		}
		slices.SortStableFunc(family, func(a, b Resolved) int { return text.Compare(a.ID, b.ID) })
		main, skin := sources[mainFile(extension)], sources[skinFile(extension)]
		// The main file's version decides the split, even when a skin file exists beside an old main file.
		version := int32(NewFileVersion)
		if main != nil {
			version = main.parsed.Version
		}
		split := version >= firstSkinVersion
		appendTo := func(canonical string, source *sourceFile, skinSide bool) (mapdir.Change, error) {
			objects := make([]NewObject, 0, len(family))
			for _, object := range family {
				added := NewObject{Base: object.Base, ID: object.ID, Mods: []NewMod{}}
				for _, field := range object.Fields {
					if !split || field.Skin == skinSide {
						added.Mods = append(added.Mods, NewMod{
							Field: field.ID, Level: int64(field.Level), Column: int64(field.Column), Value: field.Value,
						})
					}
				}
				objects = append(objects, added)
			}
			name, data := canonical, []byte(nil)
			if source != nil {
				name, data = source.name, source.data
			}
			written, err := AppendObjects(data, KindOf(name), objects, label(name))
			return mapdir.Change{Name: name, Bytes: written}, err
		}
		change, err := appendTo(mainFile(extension), main, false)
		if err != nil {
			return nil, err
		}
		mainChanges = append(mainChanges, change)
		if split {
			change, err := appendTo(skinFile(extension), skin, true)
			if err != nil {
				return nil, err
			}
			skinChanges = append(skinChanges, change)
		}
	}
	return &Plan{Changes: append(mainChanges, skinChanges...), Generated: RenderIDs(resolved), Objects: resolved}, nil
}

// Apply writes the planned object files into the staged map folder stagedDir. Only build and test call this.
func Apply(plan *Plan, stagedDir string) error {
	return mapdir.Apply(stagedDir, plan.Changes, "Writing staged object data failed.")
}
