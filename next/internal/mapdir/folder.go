// Package mapdir is a Warcraft III map saved as a folder. The game matches a map's file names without regard to
// letter case, so a Folder does too. It reads the folder, holds planned changes as views, and writes them. It
// knows nothing of what the files mean.
//
// It takes the folder's path and the label errors name it by, and names of files relative to the folder. It returns
// the files' bytes and spellings, and Changes: the complete new content of a file, or its removal. A failure a user
// can act on is a *diag.Error that names the file by the label. Name and Label spell a file as the map does, and a
// folder of the map too, so that a planner's error about a folder where a file belongs names the folder as it is
// spelled. It imports no package of Moonwell but diag and fsx.
//
// For the author of a planner. A planner is given a Folder and returns Changes; it writes nothing. It reads every
// file it changes, removes or relies on through the folder (Read, Has, Name), in any letter case; it asks Place for
// the name of a file the map does not have, and IsFolder whether a name it finds no file under is a folder of the
// map; and it returns the changes. Its caller lays them over the folder with With, and stages the view with
// StageTo or writes it into the map with ApplyInPlace.
//
// Open refuses, wherever in the folder it is:
//
//   - a link, and a map folder that is itself a link;
//   - a map folder that is a file;
//   - an entry that is neither a file nor a folder;
//   - a name Windows cannot hold: one with a backslash, a control character or any of < > : " | ? *, one that ends
//     with a dot or a space, and a device name such as CON or NUL;
//   - two paths that differ only in letter case;
//   - a folder that cannot be listed.
//
// A map folder that is not there fails with the system's error, for the caller to word.
//
// Before their first write, StageTo and ApplyInPlace check the whole plan, and refuse it when a change has a name
// that no file can have (one fsx.RelPath does not take), when a new file is named as a folder of the map, and when
// a new file would be below a file of the map, also one the plan removes: no file of a map becomes a folder. Place
// refuses all three before that: the first with a plain error, since a planner hands it a fixed name or one it has
// checked, and the last two with an error a user can act on, which names what is in the way as its file. So a plan
// refused by this check is its planner's bug, and the error is a plain one. StageTo also refuses to stage over the
// source map.
//
// ApplyInPlace writes only where the folder is still what was seen. Before each write it checks the file: one the
// scan found must be there, with the bytes that were read if any view of the folder read it, and one the scan did
// not find must not be there. A file nobody read is checked for its presence alone, so a planner reads what it
// replaces or removes.
//
// An error of this package carries a Cause exactly when the system failed: a folder that cannot be listed, a file
// that cannot be read or written, a stage that cannot be made. It carries none when the content of the map, or a
// plan for it, is refused: a link, a name that cannot be used, two spellings of one path, an entry that is no
// regular file, a file that is no folder, a new file without a place, a stage over the source map, a file that
// changed after the plan. A link made after the scan is met by a read or a write, which fails as the system's
// with the link's refusal as its Cause. Callers tell the two kinds apart by the Cause, as the assets area does to
// say whose failure it is, so every error added to this package keeps the rule.
package mapdir

import (
	"os"
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
	// The planned changes: one per file, in the order first planned. A change to a file that is not on disk is always
	// a write.
	changes []Change
	planned map[string]int    // by key, where in changes a file's change is
	made    map[string]string // by key, the spelling of each folder the changed files are in
}

// Open scans dir, which must be an existing real folder. label is how errors name it, such as "maps/map.w3x".
// A missing dir is an error that satisfies errors.Is(err, fs.ErrNotExist); the caller words it.
//
// It refuses what a map folder cannot hold, each with a *diag.Error at the path it is found at:
//
//   - a link, anywhere below dir, and a dir that is itself a link;
//   - a dir that is a file, not a folder;
//   - an entry that is neither a file nor a folder;
//   - an entry with a name Windows cannot hold: a backslash, a control character or any of < > : " | ? * in it,
//     a dot or a space at its end, or a device name such as CON or NUL;
//   - two paths that differ only in letter case, of files or of folders;
//   - a folder that cannot be listed, which alone of these is the system's failure and has a Cause.
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

// Label is how errors name a file or a folder of the map: the map folder's label, "/", and the spelling Name
// gives, which is the map's own for a file and for a folder it has. Label("") is the map folder itself.
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

// IsFolder reports whether the map has a folder under this name, by any letter case: one the scan found, or one a
// planned change makes. A removal takes a file away and leaves its folder, so a folder the scan found stays one
// in a view that removes every file in it. A folder that only planned files make is one as long as the view
// plans a file in it: it is none once every such write is taken back. The map folder itself, under "", is none.
//
// In a view whose every new file got its name from Place, a name is a file or a folder and never both. A view
// with a change below a file has that file's name as both; the check before the first write refuses such a plan.
func (f *Folder) IsFolder(name string) bool {
	_, is := f.folder(Key(name))
	return is
}

// Name is the spelling the map has for name: that of the file under it, else that of the folder under it, one the
// scan found or one a planned change makes, else name as given. A file that a view removes keeps its spelling. So
// an error names a file or a folder as the map spells it, and what the map has neither of as the caller does.
func (f *Folder) Name(name string) string {
	key := Key(name)
	if spelled, ok := f.spelling(key); ok {
		return spelled
	}
	if spelled, ok := f.folder(key); ok {
		return spelled
	}
	return name
}

// spelling is the name of the file under key: the one its planned change has, else the one on disk.
func (f *Folder) spelling(key string) (string, bool) {
	if at, ok := f.planned[key]; ok {
		return f.changes[at].Name, true
	}
	spelled, ok := f.found.names[key]
	return spelled, ok
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
	if data, err = readBelow(f.dir, spelled); err != nil {
		return nil, false, errUnreadable(f.Label(spelled), err)
	}
	f.remember(key, data)
	return data, true, nil
}

// readBelow reads the file at path below dir without following a link: where one stands in place of the file, or
// of a folder on the way to it, the read fails. The scan refused every link, so such a one was made after it.
func readBelow(dir, path string) ([]byte, error) {
	file, err := fsx.SafeJoin(dir, path)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(file)
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
