package picture

import (
	"bytes"
	"fmt"

	"github.com/mdlsvensson/moonwell/internal/binio"
)

// A BLP1 starts with its magic, six numbers of four bytes and two lists of sixteen such numbers: where each mipmap
// starts in the file, and how many bytes each has. The 156 bytes of them are its header.
const (
	blpMagic      = "BLP1"
	blp2Magic     = "BLP2" // the format of World of Warcraft, which Warcraft III does not read
	blpHeaderSize = 156
	blpMipmaps    = 16
)

// The content types of a BLP1.
const (
	blpJPEG    = 0
	blpPalette = 1
)

// blpHeader is what the header of a BLP1 says that decides whether the game reads the file.
type blpHeader struct {
	content       uint32
	width, height uint32
	// The first mipmap is the picture at its full size.
	firstOffset, firstSize uint32
}

// checkBLP accepts a BLP1 whose header is one the game reads: JPEG or palette content, one of the two sizes, and a
// first mipmap that lies in the file after the header.
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
	// The sum is made in 64 bits: two numbers of 32 bits can add up to more than 32 bits hold.
	offset, size := uint64(header.firstOffset), uint64(header.firstSize)
	if offset < blpHeaderSize || size == 0 || offset+size > uint64(len(data)) {
		return errBLPMipmapOutside(file)
	}
	return nil
}

// checkBLPMagic refuses a file that does not start as a BLP1 does, and names a BLP2 for what it is.
func checkBLPMagic(data []byte, file string) error {
	switch {
	case bytes.HasPrefix(data, []byte(blp2Magic)):
		return errBLP2(file)
	case !bytes.HasPrefix(data, []byte(blpMagic)):
		return errNotBLP(file)
	}
	return nil
}

// readBLPHeader reads the header of a file that has room for one.
func readBLPHeader(data []byte) blpHeader {
	r := binio.NewReader(data)
	r.Skip(len(blpMagic))
	content := r.U32()
	r.Skip(4) // the bits of alpha
	width, height := r.U32(), r.U32()
	r.Skip(8) // two numbers on the kind of alpha and on whether there are mipmaps
	firstOffset := r.U32()
	r.Skip((blpMipmaps - 1) * 4)
	firstSize := r.U32()
	return blpHeader{content, width, height, firstOffset, firstSize}
}

// ---- errors ----

// errBLP2 says that the file is a BLP of the other game's format.
func errBLP2(file string) error {
	return refused(file, "is a BLP2 file, the World of Warcraft format.", "Save it as BLP1, or export it as TGA.")
}

// errNotBLP says that the file under a .blp name is not a BLP at all.
func errNotBLP(file string) error {
	return refused(file, "is not a BLP file: it does not start with "+blpMagic+".", exportHint)
}

// errBLPHeaderCutShort says that the file ends inside its header.
func errBLPHeaderCutShort(file string) error {
	return refused(file, fmt.Sprintf("is cut short: a BLP header has %d bytes.", blpHeaderSize), exportHint)
}

// errBLPContent says that the content is neither JPEG nor a palette.
func errBLPContent(file string, content uint32) error {
	return refused(file, fmt.Sprintf("has the unknown BLP content type %d.", content), exportHint)
}

// errBLPMipmapOutside says that the header puts the picture at its full size where the file has no bytes for it.
func errBLPMipmapOutside(file string) error {
	return refused(file, "is cut short: its first mipmap lies outside the file.", exportHint)
}
