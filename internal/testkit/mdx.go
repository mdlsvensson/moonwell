package testkit

const (
	EmitterUsesMDL = 0x8000
	EmitterUsesTGA = 0x10000
)

func Chunk(tag string, body []byte) []byte {
	return Concat([]byte(tag), U32(uint32(len(body))), body)
}

func MDX(chunks ...[]byte) []byte {
	return Concat(append([][]byte{[]byte("MDLX")}, chunks...)...)
}

func Texture(path string, replaceableID uint32) []byte {
	return Concat(U32(replaceableID), FixedField(path, 260), U32(0))
}

func node(name string, flags uint32, extra []byte) []byte {
	return Concat(U32(uint32(96+len(extra))), FixedField(name, 80), U32(0), U32(0xffffffff), U32(flags), extra)
}

func record(parts ...[]byte) []byte {
	body := Concat(parts...)
	return Concat(U32(uint32(len(body)+4)), body)
}

func Emitter(path string, flags uint32) []byte {
	return record(
		node("Emitter", flags, make([]byte, 8)),
		F32(1),
		F32(0),
		F32(0),
		F32(0),
		FixedField(path, 260),
		F32(1),
		F32(1),
		make([]byte, 12),
	)
}

func Attachment(path string) []byte {
	return record(node("Attachment", 0, nil), FixedField(path, 260), U32(0), make([]byte, 6))
}

func Popcorn(path string) []byte {
	return record(
		node("Popcorn", 0, nil),
		F32(1),
		F32(1),
		F32(1),
		F32(1),
		F32(1),
		F32(1),
		F32(1),
		U32(0),
		FixedField(path, 260),
		FixedField("", 260),
	)
}

func FaceEffect(kind, path string) []byte {
	return Concat(FixedField(kind, 80), FixedField(path, 260))
}
