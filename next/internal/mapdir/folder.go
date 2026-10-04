// Package mapdir is a Warcraft III map saved as a folder. The game matches a map's file names without regard to
// letter case, so a Folder does too. It reads the folder, holds planned changes as views, and writes them. It
// knows nothing of what the files mean.
//
// It takes the folder's path and the label errors name it by, and names of files relative to the folder. It returns
// the files' bytes and spellings, and Changes: the complete new content of a file, or its removal. A failure a user
// can act on is a *diag.Error that names the file by the label. It imports no package of Moonwell but diag and fsx.
package mapdir

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// Key is a path's identity in a map: "/" separators, lower case.
func Key(path string) string {
	return strings.ToLower(slashed(path))
}

// slashed is path with "/" separators.
func slashed(path string) string { return strings.ReplaceAll(path, `\`, "/") }

// join is name below the folder path, with "/"; a folder of "" is the top.
func join(folder, name string) string {
	if folder == "" {
		return name
	}
	return folder + "/" + name
}

// Folder is a map folder on disk, or a view of one: the same folder with planned changes laid over it. Every view
// of a folder shares its scan and its record of what was read, so a check before writing covers what any of them
// read. Because of that record, a folder and its views are for one goroutine at a time.
type Folder struct {
	dir, label string
	found      *listing          // what Open found on disk
	hashes     map[string]string // by key, the SHA-256 of each file as it was first read from disk
	// The planned changes: one a file, in the order first planned. A change to a file that is not on disk is always
	// a write.
	changes []Change
	planned map[string]int    // by key, where in changes a file's change is
	made    map[string]string // by key, the spelling of each folder the changed files are in
}

// Open scans dir, which must be an existing real folder. label is how errors name it, such as "maps/map.w3x".
// A missing dir is an error that satisfies errors.Is(err, fs.ErrNotExist); the caller words it.
func Open(dir, label string) (*Folder, error) {
	found, err := scan(dir, label)
	if err != nil {
		return nil, err
	}
	return &Folder{
		dir: dir, label: label, found: found,
		hashes: map[string]string{}, planned: map[string]int{}, made: map[string]string{},
	}, nil
}

// Dir is the folder's path on disk.
func (f *Folder) Dir() string { return f.dir }

// Label is how errors name the file: the folder's label, "/", and the spelling Name gives. Label("") is the folder
// itself.
func (f *Folder) Label(name string) string {
	if name == "" {
		return f.label
	}
	return f.label + "/" + f.Name(name)
}

// Files is every file, in its spelling: each folder's entries sorted by name, subfolders entered where they stand,
// then the files that planned changes add, in the order first planned.
func (f *Folder) Files() []string {
	var files []string
	for _, name := range f.found.files {
		if f.Has(name) {
			files = append(files, name)
		}
	}
	for _, change := range f.changes {
		if !f.found.has(change.Name) {
			files = append(files, change.Name)
		}
	}
	return files
}

// Has reports whether the folder has a file under name, in any letter case. A folder of the map is not a file.
func (f *Folder) Has(name string) bool {
	if at, ok := f.planned[Key(name)]; ok {
		return !f.changes[at].Remove
	}
	return f.found.has(name)
}

// Name is the spelling the folder has for name, else name as given. A file that a view removes keeps its spelling,
// so an error about it names the file as the map on disk spells it.
func (f *Folder) Name(name string) string {
	key := Key(name)
	if at, ok := f.planned[key]; ok {
		return f.changes[at].Name
	}
	if spelled, ok := f.found.names[key]; ok {
		return spelled
	}
	return name
}

// Read is the content of the file under name, in any letter case: what a planned change writes, else what is on
// disk. found is false, without an error, when the folder has no such file. The bytes are the folder's own: a
// caller must not change them.
func (f *Folder) Read(name string) (data []byte, found bool, err error) {
	key := Key(name)
	if at, ok := f.planned[key]; ok {
		if f.changes[at].Remove {
			return nil, false, nil
		}
		return f.changes[at].Bytes, true, nil
	}
	spelled, ok := f.found.names[key]
	if !ok {
		return nil, false, nil
	}
	data, err = os.ReadFile(filepath.Join(f.dir, filepath.FromSlash(spelled)))
	if err != nil {
		return nil, false, errUnreadable(f.Label(spelled), err)
	}
	f.remember(key, data)
	return data, true, nil
}

// remember notes what a file held when it was first read from disk, for ApplyInPlace to compare with. A later
// read that differs is not noted: what was planned from the first would then be written over a file nobody checked.
func (f *Folder) remember(key string, data []byte) {
	if _, noted := f.hashes[key]; !noted {
		f.hashes[key] = fsx.SHA256Hex(data)
	}
}

// ---- errors ----

func errUnreadable(file string, cause error) error {
	return &diag.Error{
		Msg:   "Reading a map file failed: " + fsx.Reason(cause),
		File:  file,
		Cause: cause,
		Hint:  "Make sure this is a readable file, not a folder, and that no other program has it locked.",
	}
}
