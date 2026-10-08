package picture

import (
	"fmt"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

type Picture struct {
	Extension string
	Data      []byte
}

func Read(data []byte, displayPath string) (*Picture, error) {
	switch extensionOf(displayPath) {
	case ".blp":
		if err := checkBLP(data, displayPath); err != nil {
			return nil, err
		}
		return &Picture{Extension: "blp", Data: data}, nil
	case ".tga":
		return toPicture(readTGA(data, displayPath))
	case ".png":
		return toPicture(readPNG(data, displayPath))
	}
	return nil, errExtension(displayPath)
}

func toPicture(read pixels, err error) (*Picture, error) {
	if err != nil {
		return nil, err
	}
	return &Picture{Extension: "tga", Data: encodeOpaqueTGA(read)}, nil
}

func extensionOf(displayPath string) string {
	name := displayPath[strings.LastIndexAny(displayPath, `/\`)+1:]
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return ""
	}
	return strings.ToLower(name[dot:])
}

func checkSize(displayPath string, width, height uint32) error {
	if width != height || (width != 256 && width != 512) {
		return errSize(displayPath, width, height)
	}
	return nil
}

const exportHint = "Export the picture from an image editor as a 24- or 32-bit TGA of 256x256 pixels."

func newPictureError(displayPath, problem, hint string) *diag.Error {
	return &diag.Error{Msg: "The preview picture " + problem, File: displayPath, Hint: hint}
}

func errExtension(displayPath string) error {
	return newPictureError(displayPath, "must be a .tga, a .blp or a .png file.", exportHint)
}

func errSize(displayPath string, width, height uint32) error {
	return newPictureError(displayPath,
		fmt.Sprintf("is %dx%d pixels; it must be 256x256 or 512x512.", width, height),
		"Resize the picture. 256x256 is where the game's start location markers sit right.")
}
