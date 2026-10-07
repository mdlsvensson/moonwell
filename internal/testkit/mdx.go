package testkit

// Builders for binary MDX models: the file with its chunks, and the entries and records of the chunks that hold
// file paths (textures, particle emitters, attachments, popcorn effects and face effects).

const (
	EmitterUsesMDL = 0x8000
	EmitterUsesTGA = 0x10000
)

// Chunk is an MDX chunk: its tag, its size and its body.
func Chunk(tag string, body []byte) []byte {
	return Concat([]byte(tag), U32(uint32(len(body))), body)
}

// MDX is a binary model of the given chunks.
func MDX(chunks ...[]byte) []byte {
	return Concat(append([][]byte{[]byte("MDLX")}, chunks...)...)
}

// Texture is one entry of a TEXS chunk.
func Texture(path string, replaceableID uint32) []byte {
	return Concat(U32(replaceableID), Fixed(path, 260), U32(0))
}

// node is a model node; extra stands in for animation tracks, so readers must step over nodes by their size.
func node(name string, flags uint32, extra []byte) []byte {
	return Concat(U32(uint32(96+len(extra))), Fixed(name, 80), U32(0), U32(0xffffffff), U32(flags), extra)
}

// record is a record whose leading uint32 counts the whole record, itself included.
func record(parts ...[]byte) []byte {
	body := Concat(parts...)
	return Concat(U32(uint32(len(body)+4)), body)
}

// Emitter is one record of a PREM chunk.
func Emitter(path string, flags uint32) []byte {
	return record(
		node("Emitter", flags, make([]byte, 8)),
		F32(1), // emission rate
		F32(0), // gravity
		F32(0), // longitude
		F32(0), // latitude
		Fixed(path, 260),
		F32(1),           // life span
		F32(1),           // speed
		make([]byte, 12), // tracks
	)
}

// Attachment is one record of an ATCH chunk.
func Attachment(path string) []byte {
	return record(node("Attachment", 0, nil), Fixed(path, 260), U32(0), make([]byte, 6))
}

// Popcorn is one record of a CORN chunk.
func Popcorn(path string) []byte {
	return record(
		node("Popcorn", 0, nil),
		F32(1), // life span
		F32(1), // emission rate
		F32(1), // speed
		F32(1), // colour r
		F32(1), // colour g
		F32(1), // colour b
		F32(1), // alpha
		U32(0), // replaceable id
		Fixed(path, 260),
		Fixed("", 260), // animation visibility guide
	)
}

// FaceEffect is one entry of a FAFX chunk.
func FaceEffect(kind, path string) []byte {
	return Concat(Fixed(kind, 80), Fixed(path, 260))
}
