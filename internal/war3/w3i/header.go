package w3i

import (
	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/diag"
)

type Header struct {
	Version        int32
	HasGameVersion bool
	Major, Minor   uint32
}

func ReadHeader(data []byte, displayPath string) (Header, error) {
	r := binio.NewReader(data)
	header := Header{Version: r.I32()}
	if r.Err() != nil {
		return Header{}, errHeaderTruncated(displayPath)
	}
	if header.Version < 28 {
		return header, nil
	}
	r.Skip(saveCountAndEditorVersionSize)
	major, minor := r.U32(), r.U32()
	if r.Err() != nil {
		return header, nil
	}
	header.HasGameVersion, header.Major, header.Minor = true, major, minor
	return header, nil
}

func (h Header) IsHeaderless() bool {
	if h.Version != 39 || !h.HasGameVersion {
		return false
	}
	return h.Major*100+h.Minor >= 131
}

func errHeaderTruncated(displayPath string) error {
	return &diag.Error{Msg: "war3map.w3i is truncated.", File: displayPath, Hint: "Save the map again in World Editor."}
}
