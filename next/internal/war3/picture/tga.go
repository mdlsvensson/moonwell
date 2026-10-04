package picture

import (
	"bytes"
	"fmt"

	"github.com/mdlsvensson/moonwell/next/internal/binio"
)

// The header of a TGA has 18 bytes. After it come an ID field of any text, and then the pixels.
const (
	tgaHeaderSize = 18
	// The image types of a true-colour picture.
	tgaPlain     = 2  // pixel after pixel
	tgaRunLength = 10 // in packets
	// Bits of the descriptor, the header's last byte.
	tgaRightToLeft = 0x10
	tgaFromTop     = 0x20
	// A packet's first byte: whether its one pixel is repeated, and how many pixels it stands for, less one.
	tgaPacketRepeats = 0x80
	tgaPacketLength  = 0x7f
)

// What Read writes: 32 bits a pixel, of which the descriptor says eight are alpha, and every alpha is opaque.
const (
	writtenDepth      = 32
	writtenDescriptor = 8
	opaque            = 255
)

// pixels is a picture as a reader found it stored: row after row, and in a row pixel after pixel from the left.
type pixels struct {
	width, height int
	// step is the number of bytes of a pixel: blue, green, red, and of four bytes an alpha that is not kept.
	step    int
	fromTop bool // the first row is the picture's top row; otherwise its bottom row
	stored  []byte
}

// rowFromBottom is the stored bytes of the row that is this many rows above the picture's bottom row.
func (p pixels) rowFromBottom(row int) []byte {
	if p.fromTop {
		row = p.height - 1 - row
	}
	size := p.width * p.step
	return p.stored[row*size : (row+1)*size]
}

// opaqueTGA writes pixels in the one layout the game was seen to accept: a plain TGA of 32 bits with rows from the
// bottom and every pixel opaque.
func opaqueTGA(p pixels) []byte {
	var w binio.Writer
	w.U8(0) // no ID field
	w.U8(0) // no colour map
	w.U8(tgaPlain)
	w.Zero(9) // the fields of the colour map there is none of, and the origin
	w.U16(uint16(p.width))
	w.U16(uint16(p.height))
	w.U8(writtenDepth)
	w.U8(writtenDescriptor)
	for row := range p.height {
		w.Write(opaqueRow(p.rowFromBottom(row), p.step))
	}
	return w.Bytes()
}

// opaqueRow is a row of stored pixels as pixels of four bytes: blue, green, red and an alpha that is opaque.
func opaqueRow(row []byte, step int) []byte {
	out := make([]byte, 0, len(row)/step*4)
	for at := 0; at < len(row); at += step {
		out = append(out, row[at], row[at+1], row[at+2], opaque)
	}
	return out
}

// tgaHeader is what the header of a TGA says of the bytes after it.
type tgaHeader struct {
	idLength      int
	runLength     bool // the pixels are in packets
	step          int  // bytes a pixel
	width, height int
	fromTop       bool
}

// readTGA reads a true-colour TGA: plain or run-length encoded, 24 or 32 bits, rows from the top or the bottom.
// Bytes after the pixels, such as a footer, are not looked at.
func readTGA(data []byte, file string) (pixels, error) {
	r := binio.NewReader(data)
	header, err := readTGAHeader(r, file)
	if err != nil {
		return pixels{}, err
	}
	r.Skip(header.idLength)
	read := plainPixels
	if header.runLength {
		read = runLengthPixels
	}
	stored, err := read(r, header.width*header.height, header.step, file)
	if err != nil {
		return pixels{}, err
	}
	return pixels{header.width, header.height, header.step, header.fromTop, stored}, nil
}

// readTGAHeader reads the header and refuses a picture of a kind the pixel readers do not know, or of a size the
// game was not seen to show.
func readTGAHeader(r *binio.Reader, file string) (tgaHeader, error) {
	idLength, colourMap, kind := r.U8(), r.U8(), r.U8()
	r.Skip(9) // the fields of the colour map, and the origin
	width, height, depth, descriptor := r.U16(), r.U16(), r.U8(), r.U8()
	switch {
	case r.Err() != nil:
		return tgaHeader{}, errTGAHeaderCutShort(file)
	case colourMap != 0:
		return tgaHeader{}, errTGAColourMap(file)
	case kind != tgaPlain && kind != tgaRunLength:
		return tgaHeader{}, errTGAKind(file, kind)
	case depth != 24 && depth != 32:
		return tgaHeader{}, errTGADepth(file, depth)
	case descriptor&tgaRightToLeft != 0:
		return tgaHeader{}, errTGARightToLeft(file)
	}
	if err := checkSize(file, uint32(width), uint32(height)); err != nil {
		return tgaHeader{}, err
	}
	return tgaHeader{
		idLength:  int(idLength),
		runLength: kind == tgaRunLength,
		step:      int(depth) / 8,
		width:     int(width),
		height:    int(height),
		fromTop:   descriptor&tgaFromTop != 0,
	}, nil
}

// plainPixels reads count pixels of step bytes that are stored one after the other.
func plainPixels(r *binio.Reader, count, step int, file string) ([]byte, error) {
	stored := r.Bytes(count * step)
	if r.Err() != nil {
		return nil, errTGAPixelsCutShort(file)
	}
	return stored, nil
}

// runLengthPixels reads count pixels of step bytes that are stored in packets, and returns them one after the
// other. A packet is a byte and then either one pixel, which stands for as many as the byte says, or that many
// pixels as they are. A packet may run from one row into the next, and none may run past the last pixel.
func runLengthPixels(r *binio.Reader, count, step int, file string) ([]byte, error) {
	stored := make([]byte, 0, count*step)
	for len(stored) < count*step {
		packet := r.U8()
		if r.Err() != nil {
			return nil, errTGAPixelsCutShort(file)
		}
		length := int(packet&tgaPacketLength) + 1
		if len(stored)/step+length > count {
			return nil, errTGARunOverruns(file)
		}
		if packet&tgaPacketRepeats != 0 {
			stored = append(stored, bytes.Repeat(r.Bytes(step), length)...)
		} else {
			stored = append(stored, r.Bytes(length*step)...)
		}
		// A pixel that is cut off must not be filled in: the read has failed, and nothing was appended.
		if r.Err() != nil {
			return nil, errTGAPixelsCutShort(file)
		}
	}
	return stored, nil
}

// ---- errors ----

// errTGAHeaderCutShort says that the file ends inside its header.
func errTGAHeaderCutShort(file string) error {
	return refused(file, fmt.Sprintf("is cut short: a TGA header has %d bytes.", tgaHeaderSize), exportHint)
}

// errTGAColourMap says that the pixels are numbers into a list of colours.
func errTGAColourMap(file string) error {
	return refused(file, "is a TGA with a colour map.", exportHint)
}

// errTGAKind says that the image type is none of the two of a true-colour picture.
func errTGAKind(file string, kind uint8) error {
	return refused(file, fmt.Sprintf("is a TGA of image type %d, not a true-colour picture.", kind), exportHint)
}

// errTGADepth says that a pixel has another number of bits than the two the reader knows.
func errTGADepth(file string, depth uint8) error {
	return refused(file, fmt.Sprintf("is a TGA with %d bits a pixel, not 24 or 32.", depth), exportHint)
}

// errTGARightToLeft says that each row is stored from its right end.
func errTGARightToLeft(file string) error {
	return refused(file, "is a TGA whose rows run from right to left.", exportHint)
}

// errTGAPixelsCutShort says that the file ends before its last pixel does.
func errTGAPixelsCutShort(file string) error {
	return refused(file, "is cut short: its pixel data ends early.", exportHint)
}

// errTGARunOverruns says that a packet stands for more pixels than the picture has left.
func errTGARunOverruns(file string) error {
	return refused(file, "is damaged: a run of pixels overruns the picture.", exportHint)
}
