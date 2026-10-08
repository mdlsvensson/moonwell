package picture

import (
	"fmt"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

type Picture struct {
	Extension string
	Bytes     []byte
}

func Read(data []byte, file string) (*Picture, error) {
	switch extension(file) {
	case ".blp":
		if err := checkBLP(data, file); err != nil {
			return nil, err
		}
		return &Picture{Extension: "blp", Bytes: data}, nil
	case ".tga":
		return written(readTGA(data, file))
	case ".png":
		return written(readPNG(data, file))
	}
	return nil, errExtension(file)
}

func written(read pixels, err error) (*Picture, error) {
	if err != nil {
		return nil, err
	}
	return &Picture{Extension: "tga", Bytes: opaqueTGA(read)}, nil
}

func extension(file string) string {
	name := file[strings.LastIndexAny(file, `/\`)+1:]
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return ""
	}
	return strings.ToLower(name[dot:])
}

func checkSize(file string, width, height uint32) error {
	if width != height || (width != 256 && width != 512) {
		return errSize(file, width, height)
	}
	return nil
}

const exportHint = "Export the picture from an image editor as a 24- or 32-bit TGA of 256x256 pixels."

func refused(file, problem, hint string) *diag.Error {
	return &diag.Error{Msg: "The preview picture " + problem, File: file, Hint: hint}
}

func errExtension(file string) error {
	return refused(file, "must be a .tga, a .blp or a .png file.", exportHint)
}

func errSize(file string, width, height uint32) error {
	return refused(file,
		fmt.Sprintf("is %dx%d pixels; it must be 256x256 or 512x512.", width, height),
		"Resize the picture. 256x256 is where the game's start location markers sit right.")
}
