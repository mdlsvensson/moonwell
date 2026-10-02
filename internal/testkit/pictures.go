package testkit

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/png"
	"slices"
)

// Preview pictures built in code: no binary fixtures.

// Pixels is a square picture, rows from the top, four bytes a pixel (red, green, blue, alpha).
type Pixels struct {
	Size int
	RGBA []byte
}

// NewPixels returns a picture in which no two rows and no two halves are alike: the left half is one colour a row
// (long runs), the right half changes with every pixel. Its alpha is 7 everywhere, so a reader that keeps it is
// found out.
func NewPixels(size int) Pixels {
	rgba := make([]byte, size*size*4)
	for y := range size {
		for x := range size {
			color := [3]int{(x*7 + y) % 256, (x + y*3) % 256, (x ^ y) % 256}
			if x < size/2 {
				color = [3]int{y % 256, 40, 200}
			}
			at := (y*size + x) * 4
			rgba[at], rgba[at+1], rgba[at+2], rgba[at+3] = byte(color[0]), byte(color[1]), byte(color[2]), 7
		}
	}
	return Pixels{Size: size, RGBA: rgba}
}

// TGAOptions choose the layout of a TGA.
type TGAOptions struct {
	RLE     bool // run-length encoded (image type 10) instead of plain (type 2)
	Depth   int  // 24 or 32; zero means 32
	FromTop bool // rows stored from the top instead of from the bottom
	ID      int  // bytes of an ID field between the header and the pixels
	// Alpha is written for every pixel of a 32-bit file; nil keeps the source's own.
	Alpha *byte
}

// TGA is a true-colour TGA of picture.
func TGA(picture Pixels, options TGAOptions) []byte {
	size, depth := picture.Size, options.Depth
	if depth == 0 {
		depth = 32
	}
	step := depth / 8
	pixel := func(x, row int) []byte {
		y := size - 1 - row
		if options.FromTop {
			y = row
		}
		at := (y*size + x) * 4
		bgr := []byte{picture.RGBA[at+2], picture.RGBA[at+1], picture.RGBA[at]}
		if step != 4 {
			return bgr
		}
		if options.Alpha != nil {
			return append(bgr, *options.Alpha)
		}
		return append(bgr, picture.RGBA[at+3])
	}
	var data []byte
	for row := range size {
		if !options.RLE {
			for x := range size {
				data = append(data, pixel(x, row)...)
			}
			continue
		}
		for x := 0; x < size; {
			first := pixel(x, row)
			run := 1
			for x+run < size && run < 128 && slices.Equal(pixel(x+run, row), first) {
				run++
			}
			if run > 1 {
				data = append(data, byte(0x80|(run-1)))
				data = append(data, first...)
				x += run
				continue
			}
			// A raw packet: up to 128 pixels, stopping before the next run of two.
			raw := 1
			for x+raw < size && raw < 128 && !slices.Equal(pixel(x+raw, row), pixel(x+raw-1, row)) {
				raw++
			}
			data = append(data, byte(raw-1))
			for index := range raw {
				data = append(data, pixel(x+index, row)...)
			}
			x += raw
		}
	}
	out := make([]byte, 18+options.ID, 18+options.ID+len(data))
	out[0] = byte(options.ID)
	out[2] = 2
	if options.RLE {
		out[2] = 10
	}
	binary.LittleEndian.PutUint16(out[12:], uint16(size))
	binary.LittleEndian.PutUint16(out[14:], uint16(size))
	out[16] = byte(depth)
	if step == 4 {
		out[17] = 8
	}
	if options.FromTop {
		out[17] |= 0x20
	}
	for i := 18; i < 18+options.ID; i++ {
		out[i] = 0xee
	}
	return append(out, data...)
}

// BLP is a BLP1 with a palette and one mipmap whose pixels are all palette entry 0; content 0 claims JPEG content.
func BLP(size int, content uint32) []byte {
	const header, palette = 156, 1024
	out := make([]byte, header+palette+size*size)
	copy(out, "BLP1")
	le := binary.LittleEndian
	le.PutUint32(out[4:], content)
	le.PutUint32(out[12:], uint32(size))
	le.PutUint32(out[16:], uint32(size))
	le.PutUint32(out[20:], 5)
	le.PutUint32(out[28:], header+palette)
	le.PutUint32(out[92:], uint32(size*size))
	copy(out[header:], []byte{60, 170, 40, 255})
	return out
}

// GreyPixels returns a picture of greys, in which no two neighbouring rows or columns are alike, for the kinds of
// PNG without colour. Its alpha is 255.
func GreyPixels(size int) Pixels {
	rgba := make([]byte, size*size*4)
	for y := range size {
		for x := range size {
			grey := byte((x*3 + y*5) % 256)
			at := (y*size + x) * 4
			rgba[at], rgba[at+1], rgba[at+2], rgba[at+3] = grey, grey, grey, 255
		}
	}
	return Pixels{Size: size, RGBA: rgba}
}

// pngLow is the low byte of every 16-bit sample PNG writes: a reader must take the high byte, not round.
const pngLow = 0xa5

// PNG is picture as a PNG of one kind:
//
//	"rgba"        8 bits a channel, colour with the picture's own alpha
//	"rgb"         8 bits a channel, colour, no alpha
//	"rgba16"      16 bits a channel, colour with the picture's own alpha
//	"rgb16"       16 bits a channel, colour, no alpha
//	"grey"        8 bits of grey, for a picture of greys
//	"grey16"      16 bits of grey, for a picture of greys
//	"palette"     a palette of 256 greys, for a picture of greys
//	"interlaced"  as "rgba", stored in the seven passes of Adam7
func PNG(picture Pixels, kind string) []byte {
	size := picture.Size
	area := image.Rect(0, 0, size, size)
	wide := func(value byte) uint16 { return uint16(value)<<8 | pngLow }
	var source image.Image
	switch kind {
	case "rgba":
		source = &image.NRGBA{Pix: slices.Clone(picture.RGBA), Stride: size * 4, Rect: area}
	case "rgb":
		opaque := image.NewRGBA(area)
		for at := 0; at < len(picture.RGBA); at += 4 {
			copy(opaque.Pix[at:], picture.RGBA[at:at+3])
			opaque.Pix[at+3] = 255
		}
		source = opaque
	case "rgba16":
		deep := image.NewNRGBA64(area)
		for at, value := range picture.RGBA {
			binary.BigEndian.PutUint16(deep.Pix[at*2:], wide(value))
		}
		source = deep
	case "rgb16":
		deep := image.NewRGBA64(area)
		for at, value := range picture.RGBA {
			if at%4 == 3 {
				value = 255
			}
			binary.BigEndian.PutUint16(deep.Pix[at*2:], wide(value))
		}
		for at := 6; at < len(deep.Pix); at += 8 {
			deep.Pix[at], deep.Pix[at+1] = 255, 255
		}
		source = deep
	case "grey":
		grey := image.NewGray(area)
		for at := range grey.Pix {
			grey.Pix[at] = picture.RGBA[at*4]
		}
		source = grey
	case "grey16":
		grey := image.NewGray16(area)
		for at := 0; at < size*size; at++ {
			binary.BigEndian.PutUint16(grey.Pix[at*2:], wide(picture.RGBA[at*4]))
		}
		source = grey
	case "palette":
		// The palette runs from white to black, so an index is never its own grey.
		palette := make(color.Palette, 256)
		for index := range palette {
			grey := byte(255 - index)
			palette[index] = color.RGBA{R: grey, G: grey, B: grey, A: 255}
		}
		indexed := image.NewPaletted(area, palette)
		for at := range indexed.Pix {
			indexed.Pix[at] = 255 - picture.RGBA[at*4]
		}
		source = indexed
	case "interlaced":
		return interlacedPNG(picture)
	default:
		panic("testkit.PNG: unknown kind " + kind)
	}
	var out bytes.Buffer
	if err := png.Encode(&out, source); err != nil {
		panic(err)
	}
	return out.Bytes()
}

const pngSignature = "\x89PNG\r\n\x1a\n"

// pngChunk is one chunk of a PNG: its length, name, data and check value.
func pngChunk(name string, data []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	out = append(out, name...)
	out = append(out, data...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out[4:]))
}

// PNGHeader is the start of a PNG that says it is width by height pixels of 8-bit colour with alpha, and holds no
// pixel at all: for a reader that must judge the size before it decodes.
func PNGHeader(width, height uint32, interlaced bool) []byte {
	header := binary.BigEndian.AppendUint32(nil, width)
	header = binary.BigEndian.AppendUint32(header, height)
	header = append(header, 8, 6, 0, 0, 0)
	if interlaced {
		header[12] = 1
	}
	return append([]byte(pngSignature), pngChunk("IHDR", header)...)
}

// interlacedPNG writes the picture's rows in the seven passes of Adam7, each row unfiltered. Go's encoder does not
// interlace.
func interlacedPNG(picture Pixels) []byte {
	size := picture.Size
	// Each pass: the first column and row, then the steps between columns and between rows.
	passes := [7][4]int{{0, 0, 8, 8}, {4, 0, 8, 8}, {0, 4, 4, 8}, {2, 0, 4, 4}, {0, 2, 2, 4}, {1, 0, 2, 2}, {0, 1, 1, 2}}
	var rows bytes.Buffer
	for _, pass := range passes {
		for y := pass[1]; y < size; y += pass[3] {
			rows.WriteByte(0)
			for x := pass[0]; x < size; x += pass[2] {
				at := (y*size + x) * 4
				rows.Write(picture.RGBA[at : at+4])
			}
		}
	}
	var packed bytes.Buffer
	writer := zlib.NewWriter(&packed)
	writer.Write(rows.Bytes())
	writer.Close()
	out := PNGHeader(uint32(size), uint32(size), true)
	out = append(out, pngChunk("IDAT", packed.Bytes())...)
	return append(out, pngChunk("IEND", nil)...)
}
