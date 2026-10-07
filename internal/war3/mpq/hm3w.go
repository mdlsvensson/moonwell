package mpq

import "github.com/mdlsvensson/moonwell/internal/binio"

const (
	// hm3wSize is the length of the header, which is also what an archive's prefix is a multiple of.
	hm3wSize = 512
	// hm3wNameLimit is the longest name in bytes: the header less its magic, four unused bytes, the name's NUL,
	// the flags and the player count.
	hm3wNameLimit = hm3wSize - 4 - 4 - 1 - 4 - 4
)

// HM3WHeader is the 512-byte header that older maps carry before the archive: "HM3W", four unused bytes, the
// map's name up to a NUL, the map's flags and its number of players, and zeros to the end. A name that does not
// fit is cut at the last byte that does, which may be inside a letter of several bytes.
func HM3WHeader(name string, flags, maxPlayers uint32) []byte {
	var w binio.Writer
	w.Write([]byte("HM3W"))
	w.Zero(4)
	w.CString(name[:min(len(name), hm3wNameLimit)])
	w.U32(flags)
	w.U32(maxPlayers)
	w.Zero(hm3wSize - w.Len())
	return w.Bytes()
}
