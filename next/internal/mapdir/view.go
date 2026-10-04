package mapdir

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// Change is the complete new content of one file, or its removal.
type Change struct {
	Name   string // relative to the folder, with "/"
	Bytes  []byte
	Remove bool
}

// With is a view of the folder with changes laid over it. The receiver is not changed. A change to a file the
// folder has is renamed to the spelling it has there; removing a file it does not have does nothing. A new file
// keeps its own name, below folders spelled as the map spells them, or as the first change to name them did. A
// name that cannot be written is kept as given, and StageTo and ApplyInPlace refuse it. The view keeps each
// change's Bytes; it does not copy them.
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
// name, with "/", below the deepest folder on its way that the view knows, which is spelled as the view has it.
// Nothing else of a name is tidied: one that cannot be written (a leading slash, an empty folder name, "..")
// would become the name of another file, so it stays what it is, and StageTo and ApplyInPlace refuse the plan
// before they write anything.
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

// Place is the spelling a new file at name is written under: folders the map already has, or that an earlier
// change planned, keep their spelling. It fails when a folder on the way is a file, or name is a folder, with an
// error that names what is in the way as its file. A file the map has on disk is on the way in a view that
// removes it too. It is the name With gives a change to name.
//
// A name that fsx.RelPath does not take fails with a plain error. A planner asks for the place of a fixed name or
// of one it has checked, so such a name is its bug, and it shows here, at the planner, before the plan is written.
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

// fileOnTheWay is the first folder a file at name would be in that is a file: one the map has on disk, or one the
// view writes. A file on disk stays in the way when the view removes it, because no write may turn a file of the
// map into a folder: the journal that undoes a failed apply puts files back, and it cannot put one back where a
// folder was made.
func (f *Folder) fileOnTheWay(name string) (file string, found bool) {
	for _, folder := range foldersOf(name) {
		if f.found.has(folder) || f.Has(folder) {
			return folder, true
		}
	}
	return "", false
}

// ---- errors ----

// errNoPlaceForSuchAName is not a diag error: no planner asks for the place of a name it has not checked, so it
// must be reported as Moonwell's own fault.
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
