package model

import (
	"bytes"

	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const (
	magic           = "MDLX"
	tagSize         = 4
	chunkHeaderSize = 8
)

const (
	pathSize           = 260
	textureSize        = 268
	faceEffectNameSize = 80
	faceEffectSize     = 340
)

const (
	recordSizeSize  = 4
	nodeFixedSize   = 96
	nodeFlagsOffset = 92
	emitterUsesMDL  = 0x8000
	emitterUsesTGA  = 0x10000
)

type nodeChunk struct {
	pathOffset int
	kind       func(nodeFlags uint32) Kind
}

var nodeChunks = map[string]nodeChunk{
	"PREM": {16, emitterKindOf},
	"ATCH": {0, func(uint32) Kind { return Attachment }},
	"CORN": {32, func(uint32) Kind { return Popcorn }},
}

func emitterKindOf(nodeFlags uint32) Kind {
	return emitterKind(nodeFlags&emitterUsesMDL != 0, nodeFlags&emitterUsesTGA != 0)
}

func IsMDX(data []byte) bool {
	return bytes.HasPrefix(data, []byte(magic))
}

func ReadMDX(data []byte, file string) ([]Path, error) {
	var paths []Path
	chunks := binio.NewReader(data[min(len(magic), len(data)):])
	for chunks.Len() > 0 {
		next, err := readChunk(chunks, file)
		if err != nil {
			return nil, err
		}
		found, err := next.paths(file)
		if err != nil {
			return nil, err
		}
		paths = append(paths, found...)
	}
	return paths, nil
}

type chunk struct {
	tag  string
	body []byte
}

func readChunk(r *binio.Reader, file string) (chunk, error) {
	if r.Len() < chunkHeaderSize {
		return chunk{}, errChunkHeaderCutOff(file)
	}
	tag, size := tagName(r.Bytes(tagSize)), r.U32()
	if uint64(size) > uint64(r.Len()) {
		return chunk{}, errChunkPastEnd(file, tag)
	}
	return chunk{tag, r.Bytes(int(size))}, nil
}

func tagName(tag []byte) string {
	name := make([]rune, len(tag))
	for i, b := range tag {
		name[i] = rune(b)
	}
	return string(name)
}

func (c chunk) paths(file string) ([]Path, error) {
	switch c.tag {
	case "TEXS":
		return texturePaths(c.body, file)
	case "FAFX":
		return faceEffectPaths(c.body, file)
	}
	if layout, ok := nodeChunks[c.tag]; ok {
		return nodePaths(c, layout, file)
	}
	return nil, nil
}

func texturePaths(body []byte, file string) ([]Path, error) {
	if len(body)%textureSize != 0 {
		return nil, errTexturesNotWhole(file)
	}
	var paths []Path
	for r := binio.NewReader(body); r.Len() > 0; {
		slot := r.U32()
		paths = append(paths, Path{Kind: Texture, Path: fieldText(r.Bytes(pathSize)), ReplaceableID: int64(slot)})
		r.Skip(4)
	}
	return paths, nil
}

func faceEffectPaths(body []byte, file string) ([]Path, error) {
	if len(body)%faceEffectSize != 0 {
		return nil, errFaceEffectsNotWhole(file)
	}
	var paths []Path
	for r := binio.NewReader(body); r.Len() > 0; {
		r.Skip(faceEffectNameSize)
		if path := fieldText(r.Bytes(pathSize)); path != "" {
			paths = append(paths, Path{Kind: FaceEffect, Path: path})
		}
	}
	return paths, nil
}

func nodePaths(c chunk, layout nodeChunk, file string) ([]Path, error) {
	var paths []Path
	for r := binio.NewReader(c.body); r.Len() > 0; {
		record, err := readRecord(r, c.tag, file)
		if err != nil {
			return nil, err
		}
		nodeFlags, path, err := recordPath(record, layout.pathOffset, c.tag, file)
		if err != nil {
			return nil, err
		}
		if path != "" {
			paths = append(paths, Path{Kind: layout.kind(nodeFlags), Path: path})
		}
	}
	return paths, nil
}

func readRecord(r *binio.Reader, tag, file string) ([]byte, error) {
	if r.Len() < recordSizeSize {
		return nil, errRecordCutOff(file, tag)
	}
	size := r.U32()
	if size < recordSizeSize+nodeFixedSize || uint64(size-recordSizeSize) > uint64(r.Len()) {
		return nil, errRecordSize(file, tag)
	}
	return r.Bytes(int(size - recordSizeSize)), nil
}

func recordPath(record []byte, pathOffset int, tag, file string) (nodeFlags uint32, path string, err error) {
	r := binio.NewReader(record)
	nodeSize := r.U32()
	if nodeSize < nodeFixedSize || uint64(nodeSize) > uint64(len(record)) {
		return 0, "", errNodeSize(file, tag)
	}
	r.Skip(nodeFlagsOffset - r.Offset())
	nodeFlags = r.U32()
	r.Skip(int(nodeSize) - nodeFixedSize + pathOffset)
	field := r.Bytes(pathSize)
	if r.Err() != nil {
		return 0, "", errRecordTooSmall(file, tag)
	}
	return nodeFlags, fieldText(field), nil
}

func fieldText(field []byte) string {
	text, _, _ := bytes.Cut(field, []byte{0})
	return fsx.DecodeText(text)
}

func errChunkHeaderCutOff(file string) error {
	return errNotReadable(file, "a chunk header is cut off")
}

func errChunkPastEnd(file, tag string) error {
	return errNotReadable(file, "the "+tag+" chunk runs past the end of the file")
}

func errTexturesNotWhole(file string) error {
	return errNotReadable(file, "the TEXS chunk is not a whole number of textures")
}

func errFaceEffectsNotWhole(file string) error {
	return errNotReadable(file, "the FAFX chunk is not a whole number of face effects")
}

func errRecordCutOff(file, tag string) error {
	return errNotReadable(file, "the "+tag+" chunk has a record that is cut off")
}

func errRecordSize(file, tag string) error {
	return errNotReadable(file, "the "+tag+" chunk has a record with an invalid size")
}

func errNodeSize(file, tag string) error {
	return errNotReadable(file, "the "+tag+" chunk has a node with an invalid size")
}

func errRecordTooSmall(file, tag string) error {
	return errNotReadable(file, "the "+tag+" chunk has a record too small for its path")
}
