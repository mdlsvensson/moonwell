package testkit

import "github.com/mdlsvensson/moonwell/next/internal/binio"

// SyntheticMapInfo builds a war3map.w3i of the given format version with one player and one force. Only version 39
// has a file recorded from World Editor (the map-settings-v39 fixture); the older layouts are written out here.
func SyntheticMapInfo(version int32) []byte {
	var w binio.Writer
	texts := func(values ...string) {
		for _, value := range values {
			w.CString(value)
		}
	}
	w.I32(version)
	if version >= 28 {
		w.Zero(24)
	} else {
		w.Zero(8)
	}
	texts("TRIGSTR_001", "Author", "Description", "1-2")
	w.Zero(56)
	w.I32(0x40)
	w.U8(65)
	w.I32(0)
	if version == 39 {
		w.I32(64)
	}
	if version >= 25 {
		texts("")
	}
	texts("Loading", "Title", "Subtitle")
	if version >= 28 {
		w.I32(0)
		texts("", "", "", "")
		w.I32(0)
		w.F32(1000)
		w.F32(5000)
		w.F32(0.5)
		w.Write([]byte{1, 2, 3, 255})
		w.I32(0)
		if version == 39 {
			w.Zero(24)
		}
		texts("Default")
		w.Write([]byte{65, 255, 255, 255, 255})
		w.I32(1)
		if version >= 31 {
			w.Zero(8)
		}
		if version >= 32 {
			w.Zero(8)
		}
		if version >= 33 {
			w.Zero(4)
		}
		if version == 39 {
			w.Zero(40)
		}
		w.I32(1)
		w.I32(0)
		w.I32(1)
		w.I32(1)
		if version == 39 {
			w.I32(64)
		}
		w.I32(1)
		texts("Player 1")
		w.F32(128)
		w.F32(-896)
		if version >= 31 {
			w.Zero(16)
		} else {
			w.Zero(8)
		}
		w.I32(1)
		w.I32(3)
		w.I32(1)
		texts("Force 1")
	}
	w.Write([]byte{0xde, 0xad, 0xbe, 0xef})
	return w.Bytes()
}
