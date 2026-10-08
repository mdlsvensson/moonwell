package mapdir

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type Change struct {
	Name   string
	Bytes  []byte
	Remove bool
}

func (f *Folder) With(changes []Change) *Folder {
	view := *f
	view.changes = slices.Clone(f.changes)
	view.planned = maps.Clone(f.planned)
	view.made = maps.Clone(f.made)
	for _, change := range changes {
		view.lay(change)
	}
	view.compact()
	return &view
}

func (f *Folder) Changes() []Change { return slices.Clone(f.changes) }

func (f *Folder) lay(change Change) {
	change.Name = f.spelled(change.Name)
	key := Key(change.Name)
	at, planned := f.planned[key]
	switch {
	case change.Remove && !f.Has(change.Name):
	case change.Remove && !f.found.has(change.Name):
		delete(f.planned, key)
	case planned:
		f.changes[at] = change
	default:
		f.planned[key] = len(f.changes)
		f.changes = append(f.changes, change)
		f.makeFolders(change.Name)
	}
}

func (f *Folder) compact() {
	if len(f.changes) == len(f.planned) {
		return
	}
	stays := func(at int) bool {
		planned, ok := f.planned[Key(f.changes[at].Name)]
		return ok && planned == at
	}
	kept := make([]Change, 0, len(f.planned))
	for at, change := range f.changes {
		if stays(at) {
			kept = append(kept, change)
		}
	}
	f.changes = kept
	clear(f.planned)
	clear(f.made)
	for at, change := range f.changes {
		f.planned[Key(change.Name)] = at
		f.makeFolders(change.Name)
	}
}

func foldersOf(name string) []string {
	var folders []string
	for i := range len(name) {
		if name[i] == '/' {
			folders = append(folders, name[:i])
		}
	}
	return folders
}

func (f *Folder) makeFolders(name string) {
	for _, folder := range foldersOf(name) {
		if _, noted := f.made[Key(folder)]; !noted {
			f.made[Key(folder)] = folder
		}
	}
}

func (f *Folder) folder(key string) (string, bool) {
	if path, ok := f.found.folders[key]; ok {
		return path, true
	}
	path, ok := f.made[key]
	return path, ok
}

func (f *Folder) spelled(name string) string {
	name = slashed(name)
	if known, ok := f.spelling(Key(name)); ok {
		return known
	}
	for end := strings.LastIndexByte(name, '/'); end >= 0; end = strings.LastIndexByte(name[:end], '/') {
		if existing, ok := f.folder(Key(name[:end])); ok {
			return existing + name[end:]
		}
	}
	return name
}

func (f *Folder) Place(name string) (string, error) {
	if _, ok := fsx.RelPath(name); !ok {
		return "", errNoPlaceForSuchAName(name)
	}
	placed := f.spelled(name)
	if file, ok := f.fileOnTheWay(placed); ok {
		return "", errFileOnTheWay(f.Name(file), name, f.Label(file))
	}
	if existing, ok := f.folder(Key(placed)); ok {
		return "", errOntoAFolder(name, join(f.label, existing))
	}
	return placed, nil
}

func (f *Folder) fileOnTheWay(name string) (file string, found bool) {
	for _, folder := range foldersOf(name) {
		if f.found.has(folder) || f.Has(folder) {
			return folder, true
		}
	}
	return "", false
}

func errNoPlaceForSuchAName(name string) error {
	return fmt.Errorf("Cannot place %q: it is not a relative path that a file of a map can have.", name)
}

func errFileOnTheWay(blocking, name, file string) error {
	return &diag.Error{
		Msg:  blocking + " in the map is a file, not a folder, so " + name + " cannot go there.",
		File: file,
		Hint: "Give the new file another path, or remove " + blocking + " from the source map.",
	}
}

func errOntoAFolder(name, file string) error {
	return &diag.Error{
		Msg:  name + " would replace a folder in the map.",
		File: file,
		Hint: "Give the new file another path, or remove that folder from the source map.",
	}
}
