package picture

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
)

// pngSignature is the eight bytes every PNG starts with.
const pngSignature = "\x89PNG\r\n\x1a\n"

// readPNG decodes a PNG of any kind. The size is judged from the header, before a pixel is decoded, so a file
// that claims a huge picture costs nothing.
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

// storedPixels is the decoded picture's pixels from the top, each as three bytes: blue, green, red.
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

// storedColour is the colour a decoded pixel holds, at 8 bits a channel, whatever its alpha: the colour is read
// as the file stores it, not through the alpha, and of a 16-bit sample the high byte counts.
func storedColour(pixel color.Color) (red, green, blue byte) {
	// The decoder gives a pixel with transparency as one of these two kinds. The general conversion below would
	// read one through its alpha and lose colour; every other kind is opaque, and comes out as it is.
	switch c := pixel.(type) {
	case color.NRGBA:
		return c.R, c.G, c.B
	case color.NRGBA64:
		return byte(c.R >> 8), byte(c.G >> 8), byte(c.B >> 8)
	}
	c := color.NRGBAModel.Convert(pixel).(color.NRGBA)
	return c.R, c.G, c.B
}

// ---- errors ----

// pngHint says how to make a PNG that Read takes.
const pngHint = "Export the picture again from an image editor as a PNG of 256x256 pixels."

// errNotPNG says that the file under a .png name is not a PNG at all.
func errNotPNG(file string) error {
	return refused(file, "is not a PNG file: it does not start with a PNG signature.", pngHint)
}

// errPNGUnreadable says that the decoder gave the file up, in the decoder's own words.
func errPNGUnreadable(file string, cause error) error {
	reason := strings.TrimPrefix(cause.Error(), "png: ")
	failure := refused(file, "is a PNG that could not be read: "+reason+".", pngHint)
	failure.Cause = cause
	return failure
}
