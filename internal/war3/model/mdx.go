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
	kindOf     func(nodeFlags uint32) Kind
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

func ReadMDX(data []byte, displayPath string) ([]Path, error) {
	var paths []Path
	chunks := binio.NewReader(data[min(len(magic), len(data)):])
	for chunks.Len() > 0 {
		next, err := readChunk(chunks, displayPath)
		if err != nil {
			return nil, err
		}
		found, err := next.readPaths(displayPath)
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

func readChunk(r *binio.Reader, displayPath string) (chunk, error) {
	if r.Len() < chunkHeaderSize {
		return chunk{}, errChunkHeaderCutOff(displayPath)
	}
	tag, size := tagName(r.Bytes(tagSize)), r.U32()
	if uint64(size) > uint64(r.Len()) {
		return chunk{}, errChunkPastEnd(displayPath, tag)
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

func (c chunk) readPaths(displayPath string) ([]Path, error) {
	switch c.tag {
	case "TEXS":
		return readTexturePaths(c.body, displayPath)
	case "FAFX":
		return readFaceEffectPaths(c.body, displayPath)
	}
	if layout, ok := nodeChunks[c.tag]; ok {
		return readNodePaths(c, layout, displayPath)
	}
	return nil, nil
}

func readTexturePaths(body []byte, displayPath string) ([]Path, error) {
	if len(body)%textureSize != 0 {
		return nil, errTexturesNotWhole(displayPath)
	}
	var paths []Path
	for r := binio.NewReader(body); r.Len() > 0; {
		slot := r.U32()
		paths = append(paths, Path{Kind: Texture, Path: fixedFieldText(r.Bytes(pathSize)), ReplaceableID: int64(slot)})
		r.Skip(4)
	}
	return paths, nil
}

func readFaceEffectPaths(body []byte, displayPath string) ([]Path, error) {
	if len(body)%faceEffectSize != 0 {
		return nil, errFaceEffectsNotWhole(displayPath)
	}
	var paths []Path
	for r := binio.NewReader(body); r.Len() > 0; {
		r.Skip(faceEffectNameSize)
		if path := fixedFieldText(r.Bytes(pathSize)); path != "" {
			paths = append(paths, Path{Kind: FaceEffect, Path: path})
		}
	}
	return paths, nil
}

func readNodePaths(c chunk, layout nodeChunk, displayPath string) ([]Path, error) {
	var paths []Path
	for r := binio.NewReader(c.body); r.Len() > 0; {
		record, err := readRecord(r, c.tag, displayPath)
		if err != nil {
			return nil, err
		}
		nodeFlags, path, err := readRecordPath(record, layout.pathOffset, c.tag, displayPath)
		if err != nil {
			return nil, err
		}
		if path != "" {
			paths = append(paths, Path{Kind: layout.kindOf(nodeFlags), Path: path})
		}
	}
	return paths, nil
}

func readRecord(r *binio.Reader, tag, displayPath string) ([]byte, error) {
	if r.Len() < recordSizeSize {
		return nil, errRecordCutOff(displayPath, tag)
	}
	size := r.U32()
	if size < recordSizeSize+nodeFixedSize || uint64(size-recordSizeSize) > uint64(r.Len()) {
		return nil, errRecordSize(displayPath, tag)
	}
	return r.Bytes(int(size - recordSizeSize)), nil
}

func readRecordPath(record []byte, pathOffset int, tag, displayPath string) (nodeFlags uint32, path string, err error) {
	r := binio.NewReader(record)
	nodeSize := r.U32()
	if nodeSize < nodeFixedSize || uint64(nodeSize) > uint64(len(record)) {
		return 0, "", errNodeSize(displayPath, tag)
	}
	r.Skip(nodeFlagsOffset - r.Offset())
	nodeFlags = r.U32()
	r.Skip(int(nodeSize) - nodeFixedSize + pathOffset)
	field := r.Bytes(pathSize)
	if r.Err() != nil {
		return 0, "", errRecordTooSmall(displayPath, tag)
	}
	return nodeFlags, fixedFieldText(field), nil
}

func fixedFieldText(field []byte) string {
	text, _, _ := bytes.Cut(field, []byte{0})
	return fsx.DecodeText(text)
}

func errChunkHeaderCutOff(displayPath string) error {
	return errNotReadable(displayPath, "a chunk header is cut off")
}

func errChunkPastEnd(displayPath, tag string) error {
	return errNotReadable(displayPath, "the "+tag+" chunk runs past the end of the file")
}

func errTexturesNotWhole(displayPath string) error {
	return errNotReadable(displayPath, "the TEXS chunk is not a whole number of textures")
}

func errFaceEffectsNotWhole(displayPath string) error {
	return errNotReadable(displayPath, "the FAFX chunk is not a whole number of face effects")
}

func errRecordCutOff(displayPath, tag string) error {
	return errNotReadable(displayPath, "the "+tag+" chunk has a record that is cut off")
}

func errRecordSize(displayPath, tag string) error {
	return errNotReadable(displayPath, "the "+tag+" chunk has a record with an invalid size")
}

func errNodeSize(displayPath, tag string) error {
	return errNotReadable(displayPath, "the "+tag+" chunk has a node with an invalid size")
}

func errRecordTooSmall(displayPath, tag string) error {
	return errNotReadable(displayPath, "the "+tag+" chunk has a record too small for its path")
}
