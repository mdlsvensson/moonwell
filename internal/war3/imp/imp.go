package imp

import (
	"fmt"
	"slices"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/diag"
)

const CustomPath uint8 = 13

const version = 1

const importedFolder = `war3mapImported\`

var (
	defaultPathFlags = []uint8{0, 5, 8}
	customPathFlags  = []uint8{10, CustomPath, 29}
)

type Entry struct {
	Flag uint8
	Path string
}

func (e Entry) MapPath() string {
	if slices.Contains(customPathFlags, e.Flag) {
		return e.Path
	}
	return importedFolder + e.Path
}

func Read(data []byte, file string) ([]Entry, error) {
	r := binio.NewReader(data)
	found, count := r.U32(), r.U32()
	if r.Err() != nil {
		return nil, errTruncated(file)
	}
	if found != version {
		return nil, errVersion(file, found)
	}
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

func errUnreadable(file, problem string) error {
	return &diag.Error{
		Msg:  "war3map.imp is unreadable: " + problem + ".",
		File: file,
		Hint: "Open and re-save the map in World Editor.",
	}
}

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

func errTrailing(file string) error {
	return errUnreadable(file, "it has trailing data")
}
