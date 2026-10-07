package model

import (
	"bytes"

	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// A binary model is the magic and then chunks. A chunk is a tag of four bytes, the size of its body and the body.
const (
	magic           = "MDLX"
	tagSize         = 4
	chunkHeaderSize = 8
)

// The entries of a TEXS and of a FAFX chunk have a fixed size.
const (
	pathSize           = 260
	textureSize        = 268 // uint32 replaceableId, char[260] path, uint32 flags
	faceEffectNameSize = 80
	faceEffectSize     = 340 // char[80] name, char[260] path
)

// The body of a PREM, an ATCH and a CORN chunk is records. A record is its own size, which counts those four
// bytes too, a node, and fields of which one is a path. A node is its own size, counted likewise, a fixed part
// and animation tracks of any length.
const (
	recordSizeSize  = 4
	nodeFixedSize   = 96 // uint32 size, char[80] name, int32 objectId, int32 parentId, uint32 flags
	nodeFlagsOffset = 92
	emitterUsesMDL  = 0x8000
	emitterUsesTGA  = 0x10000
)

// nodeChunk says where the records of one kind of chunk hold their path, and what the path is for.
type nodeChunk struct {
	pathOffset int // from the end of the node to the path
	kind       func(nodeFlags uint32) Kind
}

var nodeChunks = map[string]nodeChunk{
	"PREM": {16, emitterKindOf}, // after the emission rate, the gravity, the longitude and the latitude
	"ATCH": {0, func(uint32) Kind { return Attachment }},
	"CORN": {32, func(uint32) Kind { return Popcorn }}, // after seven floats and the replaceable id
}

// emitterKindOf is what a particle emitter with these node flags emits.
func emitterKindOf(nodeFlags uint32) Kind {
	return emitterKind(nodeFlags&emitterUsesMDL != 0, nodeFlags&emitterUsesTGA != 0)
}

// IsMDX reports whether data starts with the MDLX magic of a binary model.
func IsMDX(data []byte) bool {
	return bytes.HasPrefix(data, []byte(magic))
}

// ReadMDX returns every file a binary MDX model references, in file order. file is the name its errors give.
// Chunks without paths are passed over by their size. The first four bytes are not looked at: IsMDX says whether
// they are the magic.
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

// chunk is one chunk of a binary model.
type chunk struct {
	tag  string
	body []byte
}

// readChunk reads the next chunk, which must be whole.
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

// tagName is the four bytes of a tag as text, each byte one character, so that a tag that is not ASCII can be
// named in an error.
func tagName(tag []byte) string {
	name := make([]rune, len(tag))
	for i, b := range tag {
		name[i] = rune(b)
	}
	return string(name)
}

// paths returns the files the chunk references. A chunk of any other tag than the five with paths has none.
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

// texturePaths reads a TEXS chunk. Every texture is listed, with a path or with only its replaceable slot.
func texturePaths(body []byte, file string) ([]Path, error) {
	if len(body)%textureSize != 0 {
		return nil, errTexturesNotWhole(file)
	}
	var paths []Path
	for r := binio.NewReader(body); r.Len() > 0; {
		slot := r.U32()
		paths = append(paths, Path{Kind: Texture, Path: fieldText(r.Bytes(pathSize)), ReplaceableID: int64(slot)})
		r.Skip(4) // the flags
	}
	return paths, nil
}

// faceEffectPaths reads a FAFX chunk. A face effect without a path references nothing.
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

// nodePaths reads a chunk of records. A record without a path references nothing.
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

// readRecord reads the next record of the chunk with this tag and returns what follows its size. The record must
// be whole and have room for the fixed part of a node.
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

// recordPath returns the flags of a record's node and the record's path, which stands pathOffset bytes after the
// node.
func recordPath(record []byte, pathOffset int, tag, file string) (nodeFlags uint32, path string, err error) {
	r := binio.NewReader(record)
	nodeSize := r.U32()
	if nodeSize < nodeFixedSize || uint64(nodeSize) > uint64(len(record)) {
		return 0, "", errNodeSize(file, tag)
	}
	r.Skip(nodeFlagsOffset - r.Offset()) // the name, the object id and the parent id
	nodeFlags = r.U32()
	r.Skip(int(nodeSize) - nodeFixedSize + pathOffset) // the tracks of the node, then the fields before the path
	field := r.Bytes(pathSize)
	if r.Err() != nil {
		return 0, "", errRecordTooSmall(file, tag)
	}
	return nodeFlags, fieldText(field), nil
}

// fieldText is the text of a field of fixed size: its bytes before the first NUL, read as UTF-8.
func fieldText(field []byte) string {
	text, _, _ := bytes.Cut(field, []byte{0})
	return fsx.DecodeText(text)
}

// ---- errors ----

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
