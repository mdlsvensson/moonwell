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
	outputDepth      = 32
	outputDescriptor = 8
	opaque           = 255
)

type pixels struct {
	width, height int
	bytesPerPixel int
	fromTop       bool
	data          []byte
}

func (p pixels) rowFromBottom(row int) []byte {
	if p.fromTop {
		row = p.height - 1 - row
	}
	size := p.width * p.bytesPerPixel
	return p.data[row*size : (row+1)*size]
}

func encodeOpaqueTGA(p pixels) []byte {
	var w binio.Writer
	w.U8(0)
	w.U8(0)
	w.U8(tgaPlain)
	w.Zero(9)
	w.U16(uint16(p.width))
	w.U16(uint16(p.height))
	w.U8(outputDepth)
	w.U8(outputDescriptor)
	for row := range p.height {
		w.Write(makeRowOpaque(p.rowFromBottom(row), p.bytesPerPixel))
	}
	return w.Bytes()
}

func makeRowOpaque(row []byte, step int) []byte {
	out := make([]byte, 0, len(row)/step*4)
	for i := 0; i < len(row); i += step {
		out = append(out, row[i], row[i+1], row[i+2], opaque)
	}
	return out
}

type tgaHeader struct {
	idLength      int
	runLength     bool
	bytesPerPixel int
	width, height int
	fromTop       bool
}

func readTGA(data []byte, displayPath string) (pixels, error) {
	r := binio.NewReader(data)
	header, err := readTGAHeader(r, displayPath)
	if err != nil {
		return pixels{}, err
	}
	r.Skip(header.idLength)
	read := readPlainPixels
	if header.runLength {
		read = readRunLengthPixels
	}
	stored, err := read(r, header.width*header.height, header.bytesPerPixel, displayPath)
	if err != nil {
		return pixels{}, err
	}
	return pixels{header.width, header.height, header.bytesPerPixel, header.fromTop, stored}, nil
}

func readTGAHeader(r *binio.Reader, displayPath string) (tgaHeader, error) {
	idLength, colourMap, kind := r.U8(), r.U8(), r.U8()
	r.Skip(9)
	width, height, depth, descriptor := r.U16(), r.U16(), r.U8(), r.U8()
	switch {
	case r.Err() != nil:
		return tgaHeader{}, errTGAHeaderCutShort(displayPath)
	case colourMap != 0:
		return tgaHeader{}, errTGAColourMap(displayPath)
	case kind != tgaPlain && kind != tgaRunLength:
		return tgaHeader{}, errTGAKind(displayPath, kind)
	case depth != 24 && depth != 32:
		return tgaHeader{}, errTGADepth(displayPath, depth)
	case descriptor&tgaRightToLeft != 0:
		return tgaHeader{}, errTGARightToLeft(displayPath)
	}
	if err := checkSize(displayPath, uint32(width), uint32(height)); err != nil {
		return tgaHeader{}, err
	}
	return tgaHeader{
		idLength:      int(idLength),
		runLength:     kind == tgaRunLength,
		bytesPerPixel: int(depth) / 8,
		width:         int(width),
		height:        int(height),
		fromTop:       descriptor&tgaFromTop != 0,
	}, nil
}

func readPlainPixels(r *binio.Reader, count, step int, displayPath string) ([]byte, error) {
	stored := r.Bytes(count * step)
	if r.Err() != nil {
		return nil, errTGAPixelsCutShort(displayPath)
	}
	return stored, nil
}

func readRunLengthPixels(r *binio.Reader, count, step int, displayPath string) ([]byte, error) {
	stored := make([]byte, 0, count*step)
	for len(stored) < count*step {
		packet := r.U8()
		if r.Err() != nil {
			return nil, errTGAPixelsCutShort(displayPath)
		}
		length := int(packet&tgaPacketLength) + 1
		if len(stored)/step+length > count {
			return nil, errTGARunOverruns(displayPath)
		}
		if packet&tgaPacketRepeats != 0 {
			stored = append(stored, bytes.Repeat(r.Bytes(step), length)...)
		} else {
			stored = append(stored, r.Bytes(length*step)...)
		}
		if r.Err() != nil {
			return nil, errTGAPixelsCutShort(displayPath)
		}
	}
	return stored, nil
}

func errTGAHeaderCutShort(displayPath string) error {
	return newPictureError(displayPath, fmt.Sprintf("is cut short: a TGA header has %d bytes.", tgaHeaderSize), exportHint)
}

func errTGAColourMap(displayPath string) error {
	return newPictureError(displayPath, "is a TGA with a colour map.", exportHint)
}

func errTGAKind(displayPath string, kind uint8) error {
	return newPictureError(displayPath, fmt.Sprintf("is a TGA of image type %d, not a true-colour picture.", kind), exportHint)
}

func errTGADepth(displayPath string, depth uint8) error {
	return newPictureError(displayPath, fmt.Sprintf("is a TGA with %d bits a pixel, not 24 or 32.", depth), exportHint)
}

func errTGARightToLeft(displayPath string) error {
	return newPictureError(displayPath, "is a TGA whose rows run from right to left.", exportHint)
}

func errTGAPixelsCutShort(displayPath string) error {
	return newPictureError(displayPath, "is cut short: its pixel data ends early.", exportHint)
}

func errTGARunOverruns(displayPath string) error {
	return newPictureError(displayPath, "is damaged: a run of pixels overruns the picture.", exportHint)
}
