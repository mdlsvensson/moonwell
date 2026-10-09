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

type Pixels struct {
	Size int
	RGBA []byte
}

func NewPixels(size int) Pixels {
	rgba := make([]byte, size*size*4)
	for y := range size {
		for x := range size {
			color := [3]int{(x*7 + y) % 256, (x + y*3) % 256, (x ^ y) % 256}
			if x < size/2 {
				color = [3]int{y % 256, 40, 200}
			}
			offset := (y*size + x) * 4
			rgba[offset], rgba[offset+1], rgba[offset+2], rgba[offset+3] = byte(color[0]), byte(color[1]), byte(color[2]), 7
		}
	}
	return Pixels{Size: size, RGBA: rgba}
}

type TGAOptions struct {
	RLE     bool
	Depth   int
	FromTop bool
	ID      int
	Alpha   *byte
}

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
		offset := (y*size + x) * 4
		bgr := []byte{picture.RGBA[offset+2], picture.RGBA[offset+1], picture.RGBA[offset]}
		if step != 4 {
			return bgr
		}
		if options.Alpha != nil {
			return append(bgr, *options.Alpha)
		}
		return append(bgr, picture.RGBA[offset+3])
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

const pngLow = 0xa5

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

func pngChunk(name string, data []byte) []byte {
	out := binary.BigEndian.AppendUint32(nil, uint32(len(data)))
	out = append(out, name...)
	out = append(out, data...)
	return binary.BigEndian.AppendUint32(out, crc32.ChecksumIEEE(out[4:]))
}

func PNGHeader(width, height uint32, interlaced bool) []byte {
	header := binary.BigEndian.AppendUint32(nil, width)
	header = binary.BigEndian.AppendUint32(header, height)
	header = append(header, 8, 6, 0, 0, 0)
	if interlaced {
		header[12] = 1
	}
	return append([]byte(pngSignature), pngChunk("IHDR", header)...)
}

func interlacedPNG(picture Pixels) []byte {
	size := picture.Size
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
