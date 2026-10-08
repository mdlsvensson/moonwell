package picture

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
)

const pngSignature = "\x89PNG\r\n\x1a\n"

func readPNG(data []byte, file string) (pixels, error) {
	if !bytes.HasPrefix(data, []byte(pngSignature)) {
		return pixels{}, errNotPNG(file)
	}
	header, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return pixels{}, errPNGUnreadable(file, err)
	}
	if err := checkSize(file, uint32(header.Width), uint32(header.Height)); err != nil {
		return pixels{}, err
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return pixels{}, errPNGUnreadable(file, err)
	}
	return storedPixels(decoded, header.Width, header.Height), nil
}

func storedPixels(decoded image.Image, width, height int) pixels {
	corner := decoded.Bounds().Min
	stored := make([]byte, 0, width*height*3)
	for y := range height {
		for x := range width {
			red, green, blue := storedColour(decoded.At(corner.X+x, corner.Y+y))
			stored = append(stored, blue, green, red)
		}
	}
	return pixels{width: width, height: height, step: 3, fromTop: true, stored: stored}
}

func storedColour(pixel color.Color) (red, green, blue byte) {
	switch c := pixel.(type) {
	case color.NRGBA:
		return c.R, c.G, c.B
	case color.NRGBA64:
		return byte(c.R >> 8), byte(c.G >> 8), byte(c.B >> 8)
	}
	c := color.NRGBAModel.Convert(pixel).(color.NRGBA)
	return c.R, c.G, c.B
}

const pngHint = "Export the picture again from an image editor as a PNG of 256x256 pixels."

func errNotPNG(file string) error {
	return refused(file, "is not a PNG file: it does not start with a PNG signature.", pngHint)
}

func errPNGUnreadable(file string, cause error) error {
	reason := strings.TrimPrefix(cause.Error(), "png: ")
	failure := refused(file, "is a PNG that could not be read: "+reason+".", pngHint)
	failure.Cause = cause
	return failure
}
