package picture

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"strings"
)

const pngSignature = "\x89PNG\r\n\x1a\n"

func readPNG(data []byte, displayPath string) (pixels, error) {
	if !bytes.HasPrefix(data, []byte(pngSignature)) {
		return pixels{}, errNotPNG(displayPath)
	}
	header, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return pixels{}, errPNGUnreadable(displayPath, err)
	}
	if err := checkSize(displayPath, uint32(header.Width), uint32(header.Height)); err != nil {
		return pixels{}, err
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return pixels{}, errPNGUnreadable(displayPath, err)
	}
	return toPixels(decoded, header.Width, header.Height), nil
}

func toPixels(decoded image.Image, width, height int) pixels {
	corner := decoded.Bounds().Min
	stored := make([]byte, 0, width*height*3)
	for y := range height {
		for x := range width {
			red, green, blue := toRGB(decoded.At(corner.X+x, corner.Y+y))
			stored = append(stored, blue, green, red)
		}
	}
	return pixels{width: width, height: height, bytesPerPixel: 3, fromTop: true, data: stored}
}

func toRGB(pixel color.Color) (red, green, blue byte) {
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

func errNotPNG(displayPath string) error {
	return newPictureError(displayPath, "is not a PNG file: it does not start with a PNG signature.", pngHint)
}

func errPNGUnreadable(displayPath string, cause error) error {
	reason := strings.TrimPrefix(cause.Error(), "png: ")
	diagErr := newPictureError(displayPath, "is a PNG that could not be read: "+reason+".", pngHint)
	diagErr.Cause = cause
	return diagErr
}
