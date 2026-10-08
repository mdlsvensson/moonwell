package picture

import (
	"bytes"
	"fmt"

	"github.com/mdlsvensson/moonwell/internal/binio"
)

const (
	tgaHeaderSize    = 18
	tgaPlain         = 2
	tgaRunLength     = 10
	tgaRightToLeft   = 0x10
	tgaFromTop       = 0x20
	tgaPacketRepeats = 0x80
	tgaPacketLength  = 0x7f
)

const (
	writtenDepth      = 32
	writtenDescriptor = 8
	opaque            = 255
)

type pixels struct {
	width, height int
	step          int
	fromTop       bool
	stored        []byte
}

func (p pixels) rowFromBottom(row int) []byte {
	if p.fromTop {
		row = p.height - 1 - row
	}
	size := p.width * p.step
	return p.stored[row*size : (row+1)*size]
}

func opaqueTGA(p pixels) []byte {
	var w binio.Writer
	w.U8(0)
	w.U8(0)
	w.U8(tgaPlain)
	w.Zero(9)
	w.U16(uint16(p.width))
	w.U16(uint16(p.height))
	w.U8(writtenDepth)
	w.U8(writtenDescriptor)
	for row := range p.height {
		w.Write(opaqueRow(p.rowFromBottom(row), p.step))
	}
	return w.Bytes()
}

func opaqueRow(row []byte, step int) []byte {
	out := make([]byte, 0, len(row)/step*4)
	for at := 0; at < len(row); at += step {
		out = append(out, row[at], row[at+1], row[at+2], opaque)
	}
	return out
}

type tgaHeader struct {
	idLength      int
	runLength     bool
	step          int
	width, height int
	fromTop       bool
}

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

func readTGAHeader(r *binio.Reader, file string) (tgaHeader, error) {
	idLength, colourMap, kind := r.U8(), r.U8(), r.U8()
	r.Skip(9)
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

func plainPixels(r *binio.Reader, count, step int, file string) ([]byte, error) {
	stored := r.Bytes(count * step)
	if r.Err() != nil {
		return nil, errTGAPixelsCutShort(file)
	}
	return stored, nil
}

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
		if r.Err() != nil {
			return nil, errTGAPixelsCutShort(file)
		}
	}
	return stored, nil
}

func errTGAHeaderCutShort(file string) error {
	return refused(file, fmt.Sprintf("is cut short: a TGA header has %d bytes.", tgaHeaderSize), exportHint)
}

func errTGAColourMap(file string) error {
	return refused(file, "is a TGA with a colour map.", exportHint)
}

func errTGAKind(file string, kind uint8) error {
	return refused(file, fmt.Sprintf("is a TGA of image type %d, not a true-colour picture.", kind), exportHint)
}

func errTGADepth(file string, depth uint8) error {
	return refused(file, fmt.Sprintf("is a TGA with %d bits a pixel, not 24 or 32.", depth), exportHint)
}

func errTGARightToLeft(file string) error {
	return refused(file, "is a TGA whose rows run from right to left.", exportHint)
}

func errTGAPixelsCutShort(file string) error {
	return refused(file, "is cut short: its pixel data ends early.", exportHint)
}

func errTGARunOverruns(file string) error {
	return refused(file, "is damaged: a run of pixels overruns the picture.", exportHint)
}
