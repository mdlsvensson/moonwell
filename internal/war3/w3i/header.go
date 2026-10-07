// Package w3i reads a map's war3map.w3i and edits it in place: every byte Moonwell does not change is kept. It
// takes the file's bytes and returns values with their offsets, or new bytes. It knows nothing of the map folder
// the bytes come from, of the project, or of which values Moonwell sets.
package w3i

import (
	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/diag"
)

// Header is the start of a war3map.w3i.
type Header struct {
	Version int32
	// HasGameVersion is true for version 28 and later, which record the game version that saved the map.
	HasGameVersion bool
	Major, Minor   uint32
}

// ReadHeader reads the format version and, for version 28 and later, the major and minor version of the game that
// saved the map. A file of that version that ends before them has no game version. file is the name its error
// gives.
func ReadHeader(data []byte, file string) (Header, error) {
	r := binio.NewReader(data)
	header := Header{Version: r.I32()}
	if r.Err() != nil {
		return Header{}, errNoVersion(file)
	}
	if header.Version < 28 {
		return header, nil
	}
	r.Skip(8) // the save count and the editor version
	major, minor := r.U32(), r.U32()
	if r.Err() != nil {
		return header, nil
	}
	header.HasGameVersion, header.Major, header.Minor = true, major, minor
	return header, nil
}

// Headerless reports whether the packed map has no HM3W header: version 39 maps saved by 1.31 and later are packed
// without it.
func (h Header) Headerless() bool {
	if h.Version != 39 || !h.HasGameVersion {
		return false
	}
	return h.Major*100+h.Minor >= 131
}

// ---- errors ----

// errNoVersion says that the file ends before its format version.
func errNoVersion(file string) error {
	return &diag.Error{Msg: "war3map.w3i is truncated.", File: file, Hint: "Save the map again in World Editor."}
}
