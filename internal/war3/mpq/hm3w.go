package mpq

import "github.com/mdlsvensson/moonwell/internal/binio"

const (
	hm3wSize       = 512
	hm3wMagic      = "HM3W"
	hm3wUnusedSize = 4
	nulSize        = 1
	hm3wNameLimit  = hm3wSize - len(hm3wMagic) - hm3wUnusedSize - nulSize - 2*wordSize
)

func HM3WHeader(name string, flags, maxPlayers uint32) []byte {
	var w binio.Writer
	w.Write([]byte(hm3wMagic))
	w.Zero(hm3wUnusedSize)
	w.CString(name[:min(len(name), hm3wNameLimit)])
	w.U32(flags)
	w.U32(maxPlayers)
	w.Zero(hm3wSize - w.Len())
	return w.Bytes()
}
