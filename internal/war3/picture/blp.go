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

func checkBLP(data []byte, displayPath string) error {
	if err := checkBLPMagic(data, displayPath); err != nil {
		return err
	}
	if len(data) < blpHeaderSize {
		return errBLPHeaderCutShort(displayPath)
	}
	header := readBLPHeader(data)
	if header.content != blpJPEG && header.content != blpPalette {
		return errBLPContentType(displayPath, header.content)
	}
	if err := checkSize(displayPath, header.width, header.height); err != nil {
		return err
	}
	offset, size := uint64(header.firstOffset), uint64(header.firstSize)
	if offset < blpHeaderSize || size == 0 || offset+size > uint64(len(data)) {
		return errBLPMipmapOutside(displayPath)
	}
	return nil
}

func checkBLPMagic(data []byte, displayPath string) error {
	switch {
	case bytes.HasPrefix(data, []byte(blp2Magic)):
		return errBLP2(displayPath)
	case !bytes.HasPrefix(data, []byte(blpMagic)):
		return errNotBLP(displayPath)
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

func errBLP2(displayPath string) error {
	return newPictureError(displayPath, "is a BLP2 file, the World of Warcraft format.", "Save it as BLP1, or export it as TGA.")
}

func errNotBLP(displayPath string) error {
	return newPictureError(displayPath, "is not a BLP file: it does not start with "+blpMagic+".", exportHint)
}

func errBLPHeaderCutShort(displayPath string) error {
	return newPictureError(displayPath, fmt.Sprintf("is cut short: a BLP header has %d bytes.", blpHeaderSize), exportHint)
}

func errBLPContentType(displayPath string, content uint32) error {
	return newPictureError(displayPath, fmt.Sprintf("has the unknown BLP content type %d.", content), exportHint)
}

func errBLPMipmapOutside(displayPath string) error {
	return newPictureError(displayPath, "is cut short: its first mipmap lies outside the file.", exportHint)
}
