// Package imp reads and writes war3map.imp, World Editor's index of imported files.
//
// Read takes the bytes of the file and the name to give in errors, and returns the entries in the file's order.
// Write takes entries and returns the bytes of a file. An entry is a flag and a path, and its MapPath is where the
// flag puts the file inside the map.
//
// The package must not know the map folder the bytes come from, whether the files an index names are there, or
// which entries Moonwell adds, replaces or owns.
package imp

import (
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/next/internal/binio"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

// CustomPath is the flag Moonwell writes: the entry's path is the full in-map path.
const CustomPath uint8 = 13

// version is the one version of the file there is.
const version = 1

// importedFolder is the folder inside the map of every file whose entry has no custom path.
const importedFolder = `war3mapImported\`

var (
	// defaultPathFlags are the flags of an entry whose file lives in the imported folder.
	defaultPathFlags = []uint8{0, 5, 8}
	// customPathFlags are the flags of an entry whose path is the full in-map path. 29 is 13 with the bit 0x10
	// set, which no description of the format accounts for.
	customPathFlags = []uint8{10, CustomPath, 29}
)

// Entry is one imported file in the index.
type Entry struct {
	// Flag 0, 5 or 8: the file lives under war3mapImported\. Flag 10, 13 or 29: Path is the full in-map path.
	// World Editor 3.00 saves a custom-path import as 29.
	Flag uint8
	Path string
}

// MapPath is the path of the entry's file inside the map.
func (e Entry) MapPath() string {
	if slices.Contains(customPathFlags, e.Flag) {
		return e.Path
	}
	return importedFolder + e.Path
}

// Read reads a war3map.imp. file is the name its errors give. Every entry must have one of the six flags and a
// path that is not empty and is UTF-8, and the file must end with its last entry. The bytes of a path are kept as
// they are.
func Read(data []byte, file string) ([]Entry, error) {
	r := binio.NewReader(data)
	found, count := r.U32(), r.U32()
	if r.Err() != nil {
		return nil, errTruncated(file)
	}
	if found != version {
		return nil, errVersion(file, found)
	}
	// The count is not trusted with an allocation: a file may claim more entries than it has bytes.
	entries := []Entry{}
	for i := range count {
		entry, err := readEntry(r, file, i)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if r.Len() != 0 {
		return nil, errTrailing(file)
	}
	return entries, nil
}

// readEntry reads the entry at index: one byte of flag, then the path up to its NUL.
func readEntry(r *binio.Reader, file string, index uint32) (Entry, error) {
	flag := r.U8()
	if r.Err() != nil {
		return Entry{}, errTruncated(file)
	}
	if !slices.Contains(defaultPathFlags, flag) && !slices.Contains(customPathFlags, flag) {
		return Entry{}, errFlag(file, index, flag)
	}
	path := r.CString()
	switch {
	case r.Err() != nil:
		return Entry{}, errTruncated(file)
	case len(path) == 0:
		return Entry{}, errEmptyPath(file, index)
	case !utf8.Valid(path):
		return Entry{}, errNotUTF8(file, index)
	}
	return Entry{Flag: flag, Path: string(path)}, nil
}

// Write writes a war3map.imp with the entries in the order given. It checks nothing: an entry with a flag or a
// path that Read refuses is written as it is.
func Write(entries []Entry) []byte {
	var w binio.Writer
	w.U32(version)
	w.U32(uint32(len(entries)))
	for _, entry := range entries {
		w.U8(entry.Flag)
		w.CString(entry.Path)
	}
	return w.Bytes()
}

// ---- errors ----

// errUnreadable says that the file cannot be read as an index of imports, and why.
func errUnreadable(file, problem string) error {
	return &diag.Error{
		Msg:  "war3map.imp is unreadable: " + problem + ".",
		File: file,
		Hint: "Open and re-save the map in World Editor.",
	}
}

// errTruncated says that the file ends before the version, the count or an entry does.
func errTruncated(file string) error {
	return errUnreadable(file, "it is truncated")
}

func errVersion(file string, found uint32) error {
	return errUnreadable(file, fmt.Sprintf("version %d is not supported (expected %d)", found, version))
}

func errFlag(file string, index uint32, flag uint8) error {
	return errUnreadable(file, fmt.Sprintf("entry %d has unknown flag %d", index, flag))
}

func errEmptyPath(file string, index uint32) error {
	return errUnreadable(file, fmt.Sprintf("entry %d has an empty path", index))
}

func errNotUTF8(file string, index uint32) error {
	return errUnreadable(file, fmt.Sprintf("entry %d is not valid UTF-8", index))
}

// errTrailing says that bytes follow the last entry the count allows.
func errTrailing(file string) error {
	return errUnreadable(file, "it has trailing data")
}
