package models

import (
	"bytes"
	"encoding/binary"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/text"
)

const (
	pathSize        = 260
	textureSize     = 268 // uint32 replaceableId, char[260] path, uint32 flags
	faceEffectSize  = 340 // char[80] type, char[260] path
	nodeFixedSize   = 96  // uint32 size, char[80] name, int32 objectId, int32 parentId, uint32 flags
	nodeFlagsOffset = 92
	emitterUsesMDL  = 0x8000
	emitterUsesTGA  = 0x10000
)

// IsMDX reports whether data starts with the MDLX magic of a binary model.
func IsMDX(data []byte) bool {
	return bytes.HasPrefix(data, []byte("MDLX"))
}

// ReadMDX returns every file a binary MDX model references, in file order. Chunks without paths are skipped by
// their size.
func ReadMDX(data []byte, file string) ([]Path, error) {
	u32 := func(offset int) int { return int(binary.LittleEndian.Uint32(data[offset:])) }
	field := func(offset, size int) string {
		raw := data[offset:min(offset+size, len(data))]
		if end := bytes.IndexByte(raw, 0); end >= 0 {
			raw = raw[:end]
		}
		return text.Decode(raw)
	}

	// nodeRecords reads the path of each size-prefixed node record in [start, end); the path sits pathOffset bytes
	// after the node.
	nodeRecords := func(tag string, start, end, pathOffset int, add func(flags int, path string)) error {
		for offset := start; offset < end; {
			if offset+4 > end {
				return modelError(file, "the "+tag+" chunk has a record that is cut off")
			}
			recordEnd := offset + u32(offset)
			if recordEnd < offset+4+nodeFixedSize || recordEnd > end {
				return modelError(file, "the "+tag+" chunk has a record with an invalid size")
			}
			nodeSize := u32(offset + 4)
			if nodeSize < nodeFixedSize || offset+4+nodeSize > recordEnd {
				return modelError(file, "the "+tag+" chunk has a node with an invalid size")
			}
			pathStart := offset + 4 + nodeSize + pathOffset
			if pathStart+pathSize > recordEnd {
				return modelError(file, "the "+tag+" chunk has a record too small for its path")
			}
			add(u32(offset+4+nodeFlagsOffset), field(pathStart, pathSize))
			offset = recordEnd
		}
		return nil
	}

	var paths []Path
	for offset := 4; offset < len(data); { // after the MDLX magic
		if offset+8 > len(data) {
			return nil, modelError(file, "a chunk header is cut off")
		}
		var tag strings.Builder
		for _, b := range data[offset : offset+4] {
			tag.WriteRune(rune(b))
		}
		start := offset + 8
		end := start + u32(offset+4)
		if end > len(data) {
			return nil, modelError(file, "the "+tag.String()+" chunk runs past the end of the file")
		}
		var err error
		switch tag.String() {
		case "TEXS":
			if (end-start)%textureSize != 0 {
				return nil, modelError(file, "the TEXS chunk is not a whole number of textures")
			}
			for at := start; at < end; at += textureSize {
				paths = append(paths, Path{Kind: Texture, Path: field(at+4, pathSize), ReplaceableID: int64(u32(at))})
			}
		case "FAFX":
			if (end-start)%faceEffectSize != 0 {
				return nil, modelError(file, "the FAFX chunk is not a whole number of face effects")
			}
			for at := start; at < end; at += faceEffectSize {
				if path := field(at+80, pathSize); path != "" {
					paths = append(paths, Path{Kind: FaceEffect, Path: path})
				}
			}
		case "PREM":
			err = nodeRecords("PREM", start, end, 16, func(flags int, path string) {
				if path == "" {
					return
				}
				kind := ParticleModel
				if flags&emitterUsesTGA != 0 && flags&emitterUsesMDL == 0 {
					kind = ParticleTexture
				}
				paths = append(paths, Path{Kind: kind, Path: path})
			})
		case "ATCH":
			err = nodeRecords("ATCH", start, end, 0, func(_ int, path string) {
				if path != "" {
					paths = append(paths, Path{Kind: Attachment, Path: path})
				}
			})
		case "CORN":
			err = nodeRecords("CORN", start, end, 32, func(_ int, path string) {
				if path != "" {
					paths = append(paths, Path{Kind: Popcorn, Path: path})
				}
			})
		}
		if err != nil {
			return nil, err
		}
		offset = end
	}
	return paths, nil
}
