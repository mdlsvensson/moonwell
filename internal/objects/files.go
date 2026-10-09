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
	displayPath := f.source.DisplayPath(name)
	parsed, err := objmod.Read(data, objmod.KindOf(name), displayPath)
	if err != nil {
		return err
	}
	if err := checkCustomIDsUnique(parsed.Custom.Objects, displayPath); err != nil {
		return err
	}
	f.files[name] = objectFile{data: data, parsed: parsed}
	return nil
}

func checkCustomIDsUnique(objects []objmod.Object, displayPath string) error {
	seen := map[objmod.ID]bool{}
	for _, object := range objects {
		if seen[object.ID] {
			return errCustomObjectTwice(object.ID, displayPath)
		}
		seen[object.ID] = true
	}
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

func (f *objectFiles) planChanges(resolved []Resolved) ([]mapdir.Change, error) {
	var mainChanges, skinChanges []mapdir.Change
	for _, extension := range extensions {
		objects := objectsWithExtension(resolved, extension)
		if len(objects) == 0 {
			continue
		}
		mainChange, skinChange, err := f.appendObjects(extension, objects)
		if err != nil {
			return nil, err
		}
		mainChanges = append(mainChanges, mainChange)
		skinChanges = append(skinChanges, skinChange...)
	}
	return append(mainChanges, skinChanges...), nil
}

func objectsWithExtension(resolved []Resolved, extension string) []Resolved {
	var objects []Resolved
	for _, object := range resolved {
		if extensionOf[object.Category] == extension {
			objects = append(objects, object)
		}
	}
	slices.SortStableFunc(objects, func(a, b Resolved) int { return strings.Compare(a.ID, b.ID) })
	return objects
}

func (f *objectFiles) appendObjects(extension string, objects []Resolved) (mainChange mapdir.Change, skinChanges []mapdir.Change, err error) {
	if !f.usesSkinFile(extension) {
		mainChange, err = f.appendToFile(mainFileName(extension), objects, isAnyField)
		return mainChange, nil, err
	}
	mainChange, err = f.appendToFile(mainFileName(extension), objects, isMainField)
	if err != nil {
		return mainChange, nil, err
	}
	skinChange, err := f.appendToFile(skinFileName(extension), objects, isSkinField)
	return mainChange, []mapdir.Change{skinChange}, err
}

func (f *objectFiles) usesSkinFile(extension string) bool {
	version := int32(objmod.NewFileVersion)
	if main, has := f.files[mainFileName(extension)]; has {
		version = main.parsed.Version
	}
	return version >= firstSkinVersion
}

func isAnyField(Field) bool { return true }

func isMainField(field Field) bool { return !field.Skin }

func isSkinField(field Field) bool { return field.Skin }

func (f *objectFiles) appendToFile(name string, objects []Resolved, include func(Field) bool) (mapdir.Change, error) {
	path, err := f.source.ResolveNewPath(name)
	if err != nil {
		return mapdir.Change{}, err
	}
	newObjects, err := toNewObjects(objects, include)
	if err != nil {
		return mapdir.Change{}, err
	}
	file := f.files[name]
	data, err := objmod.AppendObjects(file.parsed, file.data, objmod.KindOf(name), newObjects)
	if err != nil {
		return mapdir.Change{}, err
	}
	return mapdir.Change{Path: path, Data: data}, nil
}

func toNewObjects(objects []Resolved, include func(Field) bool) ([]objmod.NewObject, error) {
	var ids idParser
	newObjects := make([]objmod.NewObject, 0, len(objects))
	for _, object := range objects {
		newObjects = append(newObjects, objmod.NewObject{
			Base: ids.parse(object.Base), ID: ids.parse(object.ID), Mods: toNewMods(object.Fields, include, &ids),
		})
	}
	return newObjects, ids.err
}

func toNewMods(fields []Field, include func(Field) bool, ids *idParser) []objmod.NewMod {
	mods := []objmod.NewMod{}
	for _, field := range fields {
		if include(field) {
			mods = append(mods, objmod.NewMod{
				Field: ids.parse(field.ID), Level: int32(field.Level), Column: int32(field.Column),
				Value: toFileValue(field.Value),
			})
		}
	}
	return mods
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

func (p *idParser) parse(text string) objmod.ID {
	id, ok := objmod.ParseID(text)
	if !ok && p.err == nil {
		p.err = fmt.Errorf("Cannot write %s to an object file: a rawcode is 4 Latin-1 characters.", fsx.QuoteJSON(text))
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
