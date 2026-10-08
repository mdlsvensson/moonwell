package mpq

import "github.com/mdlsvensson/moonwell/internal/binio"

const (
	hm3wSize      = 512
	hm3wNameLimit = hm3wSize - 4 - 4 - 1 - 4 - 4
)

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
