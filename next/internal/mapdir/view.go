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
// folder has is renamed to the spelling it has there; removing a file it does not have does nothing. A new file
// keeps its own name, below folders spelled as the map spells them, or as the first change to name them did. The
// view keeps each change's Bytes; it does not copy them.
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

// Changes is what the view changes in the folder on disk: one Change per file, in the order first planned.
func (f *Folder) Changes() []Change { return slices.Clone(f.changes) }

// lay puts one change over the view. A later change to a file takes the place of the earlier one.
func (f *Folder) lay(change Change) {
	change.Name = f.spelled(change.Name)
	key := Key(change.Name)
	at, planned := f.planned[key]
	switch {
	case change.Remove && !f.Has(change.Name):
		// Nothing to remove, or the file is removed already.
	case change.Remove && !f.found.has(change.Name):
		// A planned new file: its write is taken back, and compact takes it out of the list.
		delete(f.planned, key)
	case planned:
		f.changes[at] = change
	default:
		f.planned[key] = len(f.changes)
		f.changes = append(f.changes, change)
		f.makeFolders(change.Name)
	}
}

// compact takes out of the list the writes that lay left unplanned, and finds the places of the rest, and the
// folders they are in, anew. Without such writes it does nothing.
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

// foldersOf is each folder a file is in, the top one first: "a" and "a/b" for "a/b/c.txt".
func foldersOf(name string) []string {
	var folders []string
	for i := range len(name) {
		if name[i] == '/' {
			folders = append(folders, name[:i])
		}
	}
	return folders
}

// makeFolders notes the folders a changed file is in. The first spelling planned for a folder stays.
func (f *Folder) makeFolders(name string) {
	for _, folder := range foldersOf(name) {
		if _, noted := f.made[Key(folder)]; !noted {
			f.made[Key(folder)] = folder
		}
	}
}

// folder is the spelling of the folder under key: the one on disk, else the one a changed file is in.
func (f *Folder) folder(key string) (string, bool) {
	if path, ok := f.found.folders[key]; ok {
		return path, true
	}
	path, ok := f.made[key]
	return path, ok
}

// spelled is the name a file is planned under. A file the view knows keeps its spelling. A new file keeps its own
// name, with "/", and each folder on its way is spelled as the folder has it.
func (f *Folder) spelled(name string) string {
	name = slashed(name)
	if known, ok := f.spelling(Key(name)); ok {
		return known
	}
	parts := strings.Split(name, "/")
	placed := ""
	for _, part := range parts[:len(parts)-1] {
		placed = join(placed, part)
		if existing, ok := f.folder(Key(placed)); ok {
			placed = existing
		}
	}
	return join(placed, parts[len(parts)-1])
}

// Place is the spelling a new file at name is written under: folders the map already has, or that an earlier
// change planned, keep their spelling. It fails when a folder on the way is a file, or name is a folder. It is
// the name With gives a change to name.
func (f *Folder) Place(name string) (string, error) {
	placed := f.spelled(name)
	for _, folder := range foldersOf(placed) {
		if f.Has(folder) {
			return "", errFileOnTheWay(f.Name(folder), name, f.Label(folder))
		}
	}
	if existing, ok := f.folder(Key(placed)); ok {
		return "", errOntoAFolder(name, join(f.label, existing))
	}
	return placed, nil
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
