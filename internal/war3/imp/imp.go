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

const importedPrefix = `war3mapImported\`

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
	return importedPrefix + e.Path
}

func Read(data []byte, displayPath string) ([]Entry, error) {
	r := binio.NewReader(data)
	fileVersion, count := r.U32(), r.U32()
	if r.Err() != nil {
		return nil, errTruncated(displayPath)
	}
	if fileVersion != version {
		return nil, errUnsupportedVersion(displayPath, fileVersion)
	}
	entries := []Entry{}
	for i := range count {
		entry, err := readEntry(r, displayPath, i)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if r.Len() != 0 {
		return nil, errTrailingData(displayPath)
	}
	return entries, nil
}

func readEntry(r *binio.Reader, displayPath string, index uint32) (Entry, error) {
	flag := r.U8()
	if r.Err() != nil {
		return Entry{}, errTruncated(displayPath)
	}
	if !slices.Contains(defaultPathFlags, flag) && !slices.Contains(customPathFlags, flag) {
		return Entry{}, errUnknownFlag(displayPath, index, flag)
	}
	path := r.CString()
	switch {
	case r.Err() != nil:
		return Entry{}, errTruncated(displayPath)
	case len(path) == 0:
		return Entry{}, errEmptyPath(displayPath, index)
	case !utf8.Valid(path):
		return Entry{}, errNotUTF8(displayPath, index)
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

func errUnreadable(displayPath, problem string) error {
	return &diag.Error{
		Msg:  "war3map.imp is unreadable: " + problem + ".",
		File: displayPath,
		Hint: "Open and re-save the map in World Editor.",
	}
}

func errTruncated(displayPath string) error {
	return errUnreadable(displayPath, "it is truncated")
}

func errUnsupportedVersion(displayPath string, fileVersion uint32) error {
	return errUnreadable(displayPath, fmt.Sprintf("version %d is not supported (expected %d)", fileVersion, version))
}

func errUnknownFlag(displayPath string, index uint32, flag uint8) error {
	return errUnreadable(displayPath, fmt.Sprintf("entry %d has unknown flag %d", index, flag))
}

func errEmptyPath(displayPath string, index uint32) error {
	return errUnreadable(displayPath, fmt.Sprintf("entry %d has an empty path", index))
}

func errNotUTF8(displayPath string, index uint32) error {
	return errUnreadable(displayPath, fmt.Sprintf("entry %d is not valid UTF-8", index))
}

func errTrailingData(displayPath string) error {
	return errUnreadable(displayPath, "it has trailing data")
}
