package testkit

import (
	"encoding/binary"
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
