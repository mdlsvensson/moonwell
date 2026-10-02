// Package w3i reads a map's war3map.w3i (its name, loading screen, players, forces and environment) and edits it
// byte for byte: every field Moonwell does not change keeps its original bytes.
package w3i

import (
	"encoding/binary"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

// Header is the start of a war3map.w3i.
type Header struct {
	Version int32
	// HasGameVersion is true for version 28 and later, which record the game version that saved the map.
	HasGameVersion bool
	Major, Minor   uint32
}

// ReadHeader reads the format version and, for version 28 and later, the saving game's major and minor version.
func ReadHeader(b []byte) (Header, error) {
	if len(b) < 4 {
		return Header{}, &diag.Error{Msg: "war3map.w3i is truncated."}
	}
	header := Header{Version: int32(binary.LittleEndian.Uint32(b))}
	if header.Version >= 28 && len(b) >= 20 {
		header.HasGameVersion = true
		header.Major = binary.LittleEndian.Uint32(b[12:])
		header.Minor = binary.LittleEndian.Uint32(b[16:])
	}
	return header, nil
}

// Headerless reports whether the packed map has no legacy HM3W header: version 39 maps saved by 1.31 and later ship
// without it.
func (h Header) Headerless() bool {
	if h.Version != 39 || !h.HasGameVersion {
		return false
	}
	return h.Major*100+h.Minor >= 131
}
