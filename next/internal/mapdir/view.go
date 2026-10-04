package mapdir

import (
	"maps"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

// Change is the complete new content of one file, or its removal.
type Change struct {
	Name   string // relative to the folder, with "/"
	Bytes  []byte
	Remove bool
}

// With is a view of the folder with changes laid over it. The receiver is not changed. A change to a file the
// folder has is renamed to the spelling it has there; removing a file it does not have does nothing.
func (f *Folder) With(changes []Change) *Folder {
	view := *f
	view.changes = slices.Clone(f.changes)
	view.planned = maps.Clone(f.planned)
	view.made = maps.Clone(f.made)
	for _, change := range changes {
		view.lay(change)
	}
	return &view
}

// Changes is what the view changes in the folder on disk: one Change a file, in the order first planned.
func (f *Folder) Changes() []Change { return slices.Clone(f.changes) }

// lay puts one change over the view. A later change to a file takes the place of the earlier one.
func (f *Folder) lay(change Change) {
	change.Name = f.Name(slashed(change.Name))
	key := Key(change.Name)
	at, planned := f.planned[key]
	switch {
	case change.Remove && !f.Has(change.Name):
		// Nothing to remove, or the file is removed already.
	case change.Remove && !f.found.has(change.Name):
		f.unplan(at)
	case planned:
		f.changes[at] = change
	default:
		f.planned[key] = len(f.changes)
		f.changes = append(f.changes, change)
		f.makeFolders(change.Name)
	}
}

// unplan drops the change at a position: the planned write of a file that is not on disk, when a later change
// removes the file again. What follows moves up, and the folders the planned writes are in are found anew.
func (f *Folder) unplan(at int) {
	f.changes = slices.Delete(f.changes, at, at+1)
	clear(f.planned)
	clear(f.made)
	for i, change := range f.changes {
		f.planned[Key(change.Name)] = i
		f.makeFolders(change.Name)
	}
}

// makeFolders notes the folders a planned file is in. The first spelling planned for a folder stays.
func (f *Folder) makeFolders(name string) {
	parts := strings.Split(name, "/")
	folder := ""
	for _, part := range parts[:len(parts)-1] {
		folder = join(folder, part)
		if _, noted := f.made[Key(folder)]; !noted {
			f.made[Key(folder)] = folder
		}
	}
}

// folder is the spelling of the folder under key: the one on disk, else the one a planned write is in.
func (f *Folder) folder(key string) (string, bool) {
	if path, ok := f.found.folders[key]; ok {
		return path, true
	}
	path, ok := f.made[key]
	return path, ok
}

// Place is the spelling a new file at name is written under: folders the map already has, or that an earlier
// change planned, keep their spelling. It fails when a folder on the way is a file, or name is a folder.
func (f *Folder) Place(name string) (string, error) {
	parts := strings.Split(slashed(name), "/")
	placed := ""
	for _, part := range parts[:len(parts)-1] {
		placed = join(placed, part)
		if f.Has(placed) {
			return "", errFileOnTheWay(f.Name(placed), name, f.Label(placed))
		}
		if existing, ok := f.folder(Key(placed)); ok {
			placed = existing
		}
	}
	placed = join(placed, parts[len(parts)-1])
	if existing, ok := f.folder(Key(placed)); ok {
		return "", errOntoAFolder(name, join(f.label, existing))
	}
	return f.Name(placed), nil
}

// ---- errors ----

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
