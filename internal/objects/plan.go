package objects

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/war3/objmod"
)

type Result struct {
	Changes []mapdir.Change
	Objects []Resolved
	IDs     string
}

func Plan(source *mapdir.Folder, objects manifest.Objects, metadata *Metadata) (*Result, error) {
	if objects.IsEmpty() {
		return &Result{Objects: []Resolved{}, IDs: noIDs}, nil
	}
	files, err := readObjectFiles(source)
	if err != nil {
		return nil, err
	}
	resolved, err := Resolve(metadata, objects, files.customIDs())
	if err != nil {
		return nil, err
	}
	changes, err := files.planChanges(resolved)
	if err != nil {
		return nil, err
	}
	ids, err := RenderIDs(resolved)
	if err != nil {
		return nil, err
	}
	return &Result{Changes: changes, Objects: resolved, IDs: ids}, nil
}

var extensions = []string{"w3u", "w3t", "w3h", "w3a", "w3q"}

var extensionOf = map[manifest.Category]string{
	"heroes": "w3u", "units": "w3u", "buildings": "w3u", "items": "w3t", "buffs": "w3h", "abilities": "w3a",
	"upgrades": "w3q",
}

const firstSkinVersion = 3

func mainFileName(extension string) string { return "war3map." + extension }
func skinFileName(extension string) string { return "war3mapSkin." + extension }

type objectFiles struct {
	source *mapdir.Folder
	files  map[string]objectFile
}

type objectFile struct {
	data   []byte
	parsed *objmod.File
}

func readObjectFiles(source *mapdir.Folder) (*objectFiles, error) {
	files := &objectFiles{source: source, files: map[string]objectFile{}}
	for _, name := range objectFileNames() {
		if err := files.readFile(name); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func objectFileNames() []string {
	var names []string
	for _, extension := range extensions {
		names = append(names, mainFileName(extension))
	}
	for _, extension := range extensions {
		names = append(names, skinFileName(extension))
	}
	return names
}

func (f *objectFiles) readFile(name string) error {
	data, found, err := f.source.Read(name)
	if err != nil || !found {
		return err
	}
	label := f.source.DisplayPath(name)
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
	f.files[name] = objectFile{data: data, parsed: parsed}
	return nil
}

func (f *objectFiles) customIDs() map[string]bool {
	ids := map[string]bool{}
	for _, file := range f.files {
		for _, object := range file.parsed.Custom.Objects {
			ids[object.ID.String()] = true
		}
	}
	return ids
}

func (f *objectFiles) usesSkinFile(extension string) bool {
	version := int32(objmod.NewFileVersion)
	if main, has := f.files[mainFileName(extension)]; has {
		version = main.parsed.Version
	}
	return version >= firstSkinVersion
}

func (f *objectFiles) planChanges(resolved []Resolved) ([]mapdir.Change, error) {
	var mains, skins []mapdir.Change
	for _, extension := range extensions {
		family := objectsWithExtension(resolved, extension)
		if len(family) == 0 {
			continue
		}
		main, skin, err := f.appendObjects(extension, family)
		if err != nil {
			return nil, err
		}
		mains = append(mains, main)
		skins = append(skins, skin...)
	}
	return append(mains, skins...), nil
}

func objectsWithExtension(resolved []Resolved, extension string) []Resolved {
	var family []Resolved
	for _, object := range resolved {
		if extensionOf[object.Category] == extension {
			family = append(family, object)
		}
	}
	slices.SortStableFunc(family, func(a, b Resolved) int { return strings.Compare(a.ID, b.ID) })
	return family
}

func (f *objectFiles) appendObjects(extension string, family []Resolved) (main mapdir.Change, skin []mapdir.Change, err error) {
	if !f.usesSkinFile(extension) {
		main, err = f.appendToFile(mainFileName(extension), family, func(Field) bool { return true })
		return main, nil, err
	}
	main, err = f.appendToFile(mainFileName(extension), family, func(field Field) bool { return !field.Skin })
	if err != nil {
		return main, nil, err
	}
	beside, err := f.appendToFile(skinFileName(extension), family, func(field Field) bool { return field.Skin })
	return main, []mapdir.Change{beside}, err
}

func (f *objectFiles) appendToFile(name string, family []Resolved, include func(Field) bool) (mapdir.Change, error) {
	placed, err := f.source.ResolveNewPath(name)
	if err != nil {
		return mapdir.Change{}, err
	}
	added, err := newObjects(family, include)
	if err != nil {
		return mapdir.Change{}, err
	}
	held := f.files[name]
	written, err := objmod.AppendObjects(held.parsed, held.data, objmod.KindOf(name), added)
	if err != nil {
		return mapdir.Change{}, err
	}
	return mapdir.Change{Path: placed, Data: written}, nil
}

func newObjects(family []Resolved, include func(Field) bool) ([]objmod.NewObject, error) {
	var ids idParser
	added := make([]objmod.NewObject, 0, len(family))
	for _, object := range family {
		one := objmod.NewObject{Base: ids.parse(object.Base), ID: ids.parse(object.ID), Mods: []objmod.NewMod{}}
		for _, field := range object.Fields {
			if include(field) {
				one.Mods = append(one.Mods, objmod.NewMod{
					Field: ids.parse(field.ID), Level: int32(field.Level), Column: int32(field.Column), Value: toFileValue(field.Value),
				})
			}
		}
		added = append(added, one)
	}
	return added, ids.err
}

func toFileValue(value Value) objmod.Value {
	switch value.Type {
	case objmod.Int:
		return objmod.Value{Type: objmod.Int, Int: int32(value.Number)}
	case objmod.Real, objmod.Unreal:
		return objmod.Value{Type: value.Type, Real: float32(value.Number)}
	}
	return objmod.Value{Type: value.Type, Text: value.Text}
}

type idParser struct{ err error }

func (r *idParser) parse(text string) objmod.ID {
	id, ok := objmod.ParseID(text)
	if !ok && r.err == nil {
		r.err = fmt.Errorf("Cannot write %s to an object file: a rawcode is 4 Latin-1 characters.", fsx.QuoteJSON(text))
	}
	return id
}

func errCustomObjectTwice(id objmod.ID, file string) error {
	return &diag.Error{
		Msg:  "Custom object '" + id.String() + "' appears twice in this file.",
		File: file,
		Hint: "Open and re-save this map in World Editor 3.00.",
	}
}
