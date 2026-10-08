package picture

import (
	"bytes"
	"fmt"

	"github.com/mdlsvensson/moonwell/internal/binio"
)

const (
	blpMagic      = "BLP1"
	blp2Magic     = "BLP2"
	blpHeaderSize = 156
	blpMipmaps    = 16
)

const (
	blpJPEG    = 0
	blpPalette = 1
)

type blpHeader struct {
	content                uint32
	width, height          uint32
	firstOffset, firstSize uint32
}

func checkBLP(data []byte, file string) error {
	if err := checkBLPMagic(data, file); err != nil {
		return err
	}
	if len(data) < blpHeaderSize {
		return errBLPHeaderCutShort(file)
	}
	header := readBLPHeader(data)
	if header.content != blpJPEG && header.content != blpPalette {
		return errBLPContent(file, header.content)
	}
	if err := checkSize(file, header.width, header.height); err != nil {
		return err
	}
	offset, size := uint64(header.firstOffset), uint64(header.firstSize)
	if offset < blpHeaderSize || size == 0 || offset+size > uint64(len(data)) {
		return errBLPMipmapOutside(file)
	}
	return nil
}

func checkBLPMagic(data []byte, file string) error {
	switch {
	case bytes.HasPrefix(data, []byte(blp2Magic)):
		return errBLP2(file)
	case !bytes.HasPrefix(data, []byte(blpMagic)):
		return errNotBLP(file)
	}
	return nil
}

func readBLPHeader(data []byte) blpHeader {
	r := binio.NewReader(data)
	r.Skip(len(blpMagic))
	content := r.U32()
	r.Skip(4)
	width, height := r.U32(), r.U32()
	r.Skip(8)
	firstOffset := r.U32()
	r.Skip((blpMipmaps - 1) * 4)
	firstSize := r.U32()
	return blpHeader{content, width, height, firstOffset, firstSize}
}

func errBLP2(file string) error {
	return refused(file, "is a BLP2 file, the World of Warcraft format.", "Save it as BLP1, or export it as TGA.")
}

func errNotBLP(file string) error {
	return refused(file, "is not a BLP file: it does not start with "+blpMagic+".", exportHint)
}

func errBLPHeaderCutShort(file string) error {
	return refused(file, fmt.Sprintf("is cut short: a BLP header has %d bytes.", blpHeaderSize), exportHint)
}

func errBLPContent(file string, content uint32) error {
	return refused(file, fmt.Sprintf("has the unknown BLP content type %d.", content), exportHint)
}

func errBLPMipmapOutside(file string) error {
	return refused(file, "is cut short: its first mipmap lies outside the file.", exportHint)
}
