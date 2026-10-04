// Package objects turns the manifest's custom objects into the object files of a map: it resolves and checks
// them against the game's metadata and the objects the map already has, plans the files, and renders the module
// of their ids and the JSON that objects:eval prints.
//
// It takes the manifest's objects, the metadata and a map folder, and returns the changes to the map's object
// files, the resolved objects and the text of the ids module. It writes nothing but the generated ids module in
// the project folder, and only when asked.
//
// It must not know how the manifest was evaluated, which command asked, or anything of the other areas.
//
// Of Moonwell it imports manifest, mapdir, diag, fsx and war3/objmod, and the root package for the embedded
// metadata.
package objects

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// Result is what the manifest's objects change in a map.
type Result struct {
	// Changes holds the changed files only: the main files in the order w3u, w3t, w3h, w3a, w3q, then their
	// skin files in that order.
	Changes []mapdir.Change
	Objects []Resolved
	// IDs is the content of the generated ids module.
	IDs string
}

// Plan computes the object files the manifest's objects change in the map folder, and writes nothing. With no
// objects it reads nothing. Otherwise it reads the map's object files, resolves the objects against them, and
// appends each, sorted by id, to its main file and, for a map with skin files, to its skin file.
func Plan(folder *mapdir.Folder, objects manifest.Objects, metadata *Metadata) (*Result, error) {
	if objects.Empty() {
		return &Result{Objects: []Resolved{}, IDs: RenderIDs(nil)}, nil
	}
	files, err := readObjectFiles(folder)
	if err != nil {
		return nil, err
	}
	resolved, err := Resolve(metadata, objects, files.customIDs())
	if err != nil {
		return nil, err
	}
	changes, err := files.changes(resolved)
	if err != nil {
		return nil, err
	}
	return &Result{Changes: changes, Objects: resolved, IDs: RenderIDs(resolved)}, nil
}

// extensions is the extensions of the object files Moonwell appends to, in the order of their changes.
var extensions = []string{"w3u", "w3t", "w3h", "w3a", "w3q"}

// extensionOf is the extension of the files each category's objects go to.
var extensionOf = map[manifest.Category]string{
	"heroes": "w3u", "units": "w3u", "buildings": "w3u", "items": "w3t", "buffs": "w3h", "abilities": "w3a",
	"upgrades": "w3q",
}

// firstSkinVersion is the first version of a main file that has a skin file beside it. A map saved before it,
// with files of version 1 or 2, keeps every field in the main file.
const firstSkinVersion = 3

func mainFile(extension string) string { return "war3map." + extension }
func skinFile(extension string) string { return "war3mapSkin." + extension }

// ---- the files the map has ----

// objectFiles is the object files of a map folder that Moonwell appends to.
type objectFiles struct {
	folder *mapdir.Folder
	held   map[string]objectFile // by mainFile or skinFile of an extension; a file the map lacks is not in it
}

// objectFile is an object file as the map has it.
type objectFile struct {
	data   []byte
	parsed *objmod.File
}

// readObjectFiles reads the object files the map has: the main files, then the skin files. It fails on the
// first that does not read or holds one custom object twice.
func readObjectFiles(folder *mapdir.Folder) (*objectFiles, error) {
	files := &objectFiles{folder: folder, held: map[string]objectFile{}}
	for _, name := range objectFileNames() {
		if err := files.read(name); err != nil {
			return nil, err
		}
	}
	return files, nil
}

// objectFileNames is the ten files in the order they are read: the main files, then the skin files.
func objectFileNames() []string {
	var names []string
	for _, extension := range extensions {
		names = append(names, mainFile(extension))
	}
	for _, extension := range extensions {
		names = append(names, skinFile(extension))
	}
	return names
}

// read reads the file the map has under name, in any letter case, and holds it. A map without the file holds
// nothing under the name.
func (f *objectFiles) read(name string) error {
	data, found, err := f.folder.Read(name)
	if err != nil || !found {
		return err
	}
	label := f.folder.Label(name)
	parsed, err := objmod.Read(data, objmod.KindOf(name), label)
	if err != nil {
		return err
	}
	seen := map[objmod.ID]bool{}
	for _, object := range parsed.Custom.Objects {
		if seen[object.ID] {
			return errCustomObjectTwice(object.ID, label)
		}
		seen[object.ID] = true
	}
	f.held[name] = objectFile{data: data, parsed: parsed}
	return nil
}

// customIDs is the ids of the custom objects in any of the files.
func (f *objectFiles) customIDs() map[string]bool {
	ids := map[string]bool{}
	for _, file := range f.held {
		for _, object := range file.parsed.Custom.Objects {
			ids[object.ID.String()] = true
		}
	}
	return ids
}

// splits reports whether the objects of an extension go to a main file and a skin file. The main file's version
// decides, even when a skin file stands beside an older main file; a map without the main file gets a new one,
// which has a skin file.
func (f *objectFiles) splits(extension string) bool {
	version := int32(objmod.NewFileVersion)
	if main, has := f.held[mainFile(extension)]; has {
		version = main.parsed.Version
	}
	return version >= firstSkinVersion
}

// ---- the changes ----

// changes appends the resolved objects to their files and returns the files that changed: the main files, then
// the skin files.
func (f *objectFiles) changes(resolved []Resolved) ([]mapdir.Change, error) {
	var mains, skins []mapdir.Change
	for _, extension := range extensions {
		family := familyOf(resolved, extension)
		if len(family) == 0 {
			continue
		}
		main, skin, err := f.appendFamily(extension, family)
		if err != nil {
			return nil, err
		}
		mains = append(mains, main)
		skins = append(skins, skin...)
	}
	return append(mains, skins...), nil
}

// familyOf is the objects that go to the files of an extension, sorted by id as World Editor writes them.
func familyOf(resolved []Resolved, extension string) []Resolved {
	var family []Resolved
	for _, object := range resolved {
		if extensionOf[object.Category] == extension {
			family = append(family, object)
		}
	}
	slices.SortStableFunc(family, func(a, b Resolved) int { return strings.Compare(a.ID, b.ID) })
	return family
}

// appendFamily appends the objects to the files of an extension. Where the files split, every object goes to
// both, as World Editor writes it: with its skin fields in the skin file and the others in the main file. Else
// the main file takes every field and there is no change to a skin file.
func (f *objectFiles) appendFamily(extension string, family []Resolved) (main mapdir.Change, skin []mapdir.Change, err error) {
	if !f.splits(extension) {
		main, err = f.appended(mainFile(extension), family, func(Field) bool { return true })
		return main, nil, err
	}
	main, err = f.appended(mainFile(extension), family, func(field Field) bool { return !field.Skin })
	if err != nil {
		return main, nil, err
	}
	beside, err := f.appended(skinFile(extension), family, func(field Field) bool { return field.Skin })
	return main, []mapdir.Change{beside}, err
}

// appended is the change that adds the objects, each with the fields the file takes, to the file under name: to
// what the map has there, under the spelling the map has, or to a new file. A map that has a folder where the
// new file would be is refused.
func (f *objectFiles) appended(name string, family []Resolved, takes func(Field) bool) (mapdir.Change, error) {
	placed, err := f.folder.Place(name)
	if err != nil {
		return mapdir.Change{}, err
	}
	added, err := newObjects(family, takes)
	if err != nil {
		return mapdir.Change{}, err
	}
	written, err := objmod.Append(f.held[name].data, objmod.KindOf(name), added, f.folder.Label(name))
	if err != nil {
		return mapdir.Change{}, err
	}
	return mapdir.Change{Name: placed, Bytes: written}, nil
}

// newObjects is the objects as a file stores them, each with the fields the file takes.
func newObjects(family []Resolved, takes func(Field) bool) ([]objmod.NewObject, error) {
	var ids rawcodes
	added := make([]objmod.NewObject, 0, len(family))
	for _, object := range family {
		one := objmod.NewObject{Base: ids.of(object.Base), ID: ids.of(object.ID), Mods: []objmod.NewMod{}}
		for _, field := range object.Fields {
			if takes(field) {
				one.Mods = append(one.Mods, objmod.NewMod{
					Field: ids.of(field.ID), Level: int32(field.Level), Column: int32(field.Column), Value: fileValue(field.Value),
				})
			}
		}
		added = append(added, one)
	}
	return added, ids.err
}

// fileValue is a resolved value in the type its file stores. Resolve lets through only a number that the type
// holds.
func fileValue(value Value) objmod.Value {
	switch value.Type {
	case objmod.Int:
		return objmod.Value{Type: objmod.Int, Int: int32(value.Number)}
	case objmod.Real, objmod.Unreal:
		return objmod.Value{Type: value.Type, Real: float32(value.Number)}
	}
	return objmod.Value{Type: value.Type, Text: value.Text}
}

// rawcodes reads ids as the files store them. It keeps the first text that is no id and goes on returning the
// zero id. Resolve lets through only ids of four ASCII letters or digits and the ids of the metadata, so such a
// text is a bug, and its error is not a diag error.
type rawcodes struct{ err error }

func (r *rawcodes) of(text string) objmod.ID {
	id, ok := objmod.ParseID(text)
	if !ok && r.err == nil {
		r.err = fmt.Errorf("Cannot write %s to an object file: a rawcode is 4 Latin-1 characters.", fsx.Quoted(text))
	}
	return id
}

// ---- errors ----

func errCustomObjectTwice(id objmod.ID, file string) error {
	return &diag.Error{
		Msg:  "Custom object '" + id.String() + "' appears twice in this file.",
		File: file,
		Hint: "Open and re-save this map in World Editor 3.00.",
	}
}
