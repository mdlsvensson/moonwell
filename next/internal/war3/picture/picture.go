// Package picture prepares a preview picture for the game's map list. It takes a file's bytes and name and
// returns the bytes that go into the map, or refuses a picture the game could not read: such a file closes the
// game the moment the map is selected.
//
// Nothing doubtful gets in. A BLP1 is checked and goes in as the file it is. A TGA or a PNG is decoded and written
// again in the one layout the game was seen to accept, so a picture of either format, stored in any way Read
// knows, gives the same bytes. Only the two sizes the game was seen to show are taken.
//
// The package must not know where the file lies, which setting names it or what it is called inside the map: it
// reads no file and knows nothing of a project or a map folder.
package picture

import (
	"fmt"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

// Picture is a preview picture as it goes into the map, as war3mapMap.<extension>.
type Picture struct {
	Extension string // "blp" or "tga"
	Bytes     []byte
}

// Read checks the picture by its extension. A BLP1 is used as it is; a TGA or a PNG is written as a plain 32-bit
// TGA with rows from the bottom and every pixel opaque. file is the name errors give.
//
// The bytes of a BLP's Picture are data itself, not a copy of it. Read changes no byte of data.
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

// written is the picture that the pixels a reader returned go into the map as, or the reader's refusal.
func written(read pixels, err error) (*Picture, error) {
	if err != nil {
		return nil, err
	}
	return &Picture{Extension: "tga", Bytes: opaqueTGA(read)}, nil
}

// extension is the file's extension with its dot, in lower case: what follows the last dot of the name after the
// last "/" or "\". A name that only starts with a dot has none.
func extension(file string) string {
	name := file[strings.LastIndexAny(file, `/\`)+1:]
	dot := strings.LastIndex(name, ".")
	if dot <= 0 {
		return ""
	}
	return strings.ToLower(name[dot:])
}

// checkSize refuses a picture of any size but the two that Warcraft III 3.0.0.24268 was seen to show in its map
// list: 256 and 512 pixels a side.
func checkSize(file string, width, height uint32) error {
	if width != height || (width != 256 && width != 512) {
		return errSize(file, width, height)
	}
	return nil
}

// ---- errors ----

// exportHint says how to make a picture that Read takes.
const exportHint = "Export the picture from an image editor as a 24- or 32-bit TGA of 256x256 pixels."

// refused says that the picture does not go into a map. problem is the rest of a sentence that starts "The
// preview picture", and hint says how to get a picture that does go in.
func refused(file, problem, hint string) *diag.Error {
	return &diag.Error{Msg: "The preview picture " + problem, File: file, Hint: hint}
}

// errExtension says that the file's name does not end as the name of a picture of the three formats does.
func errExtension(file string) error {
	return refused(file, "must be a .tga, a .blp or a .png file.", exportHint)
}

// errSize says that the picture is of a size the game was not seen to show.
func errSize(file string, width, height uint32) error {
	return refused(file,
		fmt.Sprintf("is %dx%d pixels; it must be 256x256 or 512x512.", width, height),
		"Resize the picture. 256x256 is where the game's start location markers sit right.")
}
