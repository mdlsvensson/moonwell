package settings

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

// Picture is a preview picture as it goes into the map, as war3mapMap.<extension>.
type Picture struct {
	Extension string // "blp" or "tga"
	Bytes     []byte
}

const exportHint = "Export the picture from an image editor as a 24- or 32-bit TGA of 256x256 pixels."

// refuse builds the error for a picture Moonwell will not put into a map. A picture the game cannot read closes it
// the moment the map is selected, so nothing doubtful gets in.
func refuse(file, problem string) error {
	return &diag.Error{Msg: "The preview picture " + problem, File: file, Hint: exportHint}
}

func refuseWith(file, problem, hint string) error {
	return &diag.Error{Msg: "The preview picture " + problem, File: file, Hint: hint}
}

// The sizes Warcraft III 3.0.0.24268 was seen to show in its map list.
func checkSize(file string, width, height uint32) error {
	if width != height || (width != 256 && width != 512) {
		return refuseWith(file,
			fmt.Sprintf("is %dx%d pixels; it must be 256x256 or 512x512.", width, height),
			"Resize the picture. 256x256 is where the game's start location markers sit right.")
	}
	return nil
}

const blpHeader = 156

// checkBLP accepts a BLP1 as it is, once its header is one the game reads: JPEG or palette content and a first
// mipmap.
func checkBLP(data []byte, file string) error {
	magic := string(data[:min(4, len(data))])
	if magic == "BLP2" {
		return refuseWith(file, "is a BLP2 file, the World of Warcraft format.", "Save it as BLP1, or export it as TGA.")
	}
	if magic != "BLP1" {
		return refuse(file, "is not a BLP file: it does not start with BLP1.")
	}
	if len(data) < blpHeader {
		return refuse(file, fmt.Sprintf("is cut short: a BLP header has %d bytes.", blpHeader))
	}
	u32 := func(offset int) uint32 { return binary.LittleEndian.Uint32(data[offset:]) }
	if content := u32(4); content != 0 && content != 1 {
		return refuse(file, fmt.Sprintf("has the unknown BLP content type %d.", content))
	}
	if err := checkSize(file, u32(12), u32(16)); err != nil {
		return err
	}
	offset, size := uint64(u32(28)), uint64(u32(92))
	if offset < blpHeader || size == 0 || offset+size > uint64(len(data)) {
		return refuse(file, "is cut short: its first mipmap lies outside the file.")
	}
	return nil
}

const tgaHeader = 18

// rewriteTGA reads a true-colour TGA (plain or run-length encoded, 24 or 32 bits, rows from the top or the bottom)
// and writes it again as the one layout the game was seen to accept: plain, 32 bits, rows from the bottom, every
// pixel opaque.
func rewriteTGA(data []byte, file string) ([]byte, error) {
	if len(data) < tgaHeader {
		return nil, refuse(file, fmt.Sprintf("is cut short: a TGA header has %d bytes.", tgaHeader))
	}
	idLength, colorMap, kind, depth, descriptor := int(data[0]), data[1], data[2], int(data[16]), data[17]
	width := int(binary.LittleEndian.Uint16(data[12:]))
	height := int(binary.LittleEndian.Uint16(data[14:]))
	switch {
	case colorMap != 0:
		return nil, refuse(file, "is a TGA with a colour map.")
	case kind != 2 && kind != 10:
		return nil, refuse(file, fmt.Sprintf("is a TGA of image type %d, not a true-colour picture.", kind))
	case depth != 24 && depth != 32:
		return nil, refuse(file, fmt.Sprintf("is a TGA with %d bits a pixel, not 24 or 32.", depth))
	case descriptor&0x10 != 0:
		return nil, refuse(file, "is a TGA whose rows run from right to left.")
	}
	if err := checkSize(file, uint32(width), uint32(height)); err != nil {
		return nil, err
	}

	step, pixels := depth/8, width*height
	fromTop := descriptor&0x20 != 0
	output := make([]byte, tgaHeader+pixels*4)
	output[2] = 2
	binary.LittleEndian.PutUint16(output[12:], uint16(width))
	binary.LittleEndian.PutUint16(output[14:], uint16(height))
	output[16] = 32
	output[17] = 8
	at, written := tgaHeader+idLength, 0
	// put copies the pixel at from to the next place in the source's own row order.
	put := func(from int) {
		row, column := written/width, written%width
		if fromTop {
			row = height - 1 - row
		}
		to := tgaHeader + (row*width+column)*4
		output[to], output[to+1], output[to+2], output[to+3] = data[from], data[from+1], data[from+2], 255
		written++
	}
	early := refuse(file, "is cut short: its pixel data ends early.")
	if kind == 2 {
		if at+pixels*step > len(data) {
			return nil, early
		}
		for ; written < pixels; at += step {
			put(at)
		}
		return output, nil
	}
	for written < pixels {
		if at >= len(data) {
			return nil, early
		}
		packet := data[at]
		at++
		count := int(packet&0x7f) + 1
		if written+count > pixels {
			return nil, refuse(file, "is damaged: a run of pixels overruns the picture.")
		}
		if packet&0x80 != 0 {
			if at+step > len(data) {
				return nil, early
			}
			for range count {
				put(at)
			}
			at += step
		} else {
			if at+count*step > len(data) {
				return nil, early
			}
			for range count {
				put(at)
				at += step
			}
		}
	}
	return output, nil
}

// extension is the file's extension with its dot, in lower case; a name that only starts with a dot has none.
func extension(file string) string {
	base := file[strings.LastIndexAny(file, `/\`)+1:]
	dot := strings.LastIndex(base, ".")
	if dot <= 0 {
		return ""
	}
	return strings.ToLower(base[dot:])
}

// ReadPicture checks the picture file names by its extension, and returns the bytes to put into the map.
func ReadPicture(data []byte, file string) (*Picture, error) {
	switch extension(file) {
	case ".blp":
		if err := checkBLP(data, file); err != nil {
			return nil, err
		}
		return &Picture{Extension: "blp", Bytes: data}, nil
	case ".tga":
		rewritten, err := rewriteTGA(data, file)
		if err != nil {
			return nil, err
		}
		return &Picture{Extension: "tga", Bytes: rewritten}, nil
	}
	return nil, refuse(file, "must be a .tga or a .blp file.")
}

// LoadPreview reads the picture that settings.info.preview names: preview is a path from the project folder root.
// Errors about the setting name manifestFile; errors about the picture name its path.
func LoadPreview(root, preview, manifestFile string) (*Picture, error) {
	path, err := fsx.RelPath(preview)
	if err != nil {
		return nil, &diag.Error{
			Msg:   `settings.info.preview must be a path inside the project, not "` + preview + `".`,
			File:  manifestFile,
			Cause: err,
			Hint:  `Name a picture in the project folder, such as "preview.tga" beside moonwell.pkl.`,
		}
	}
	if strings.HasPrefix(mapdir.Key(path), "assets/") {
		return nil, &diag.Error{
			Msg:  "settings.info.preview names a file under assets/: " + path,
			File: manifestFile,
			Hint: "Keep the picture outside assets/, for example beside moonwell.pkl: every file under assets/ is also " +
				"imported into the map under its own name.",
		}
	}
	file, err := fsx.SafeJoin(root, path)
	if err != nil {
		return nil, err
	}
	info, err := fsx.Lstat(file)
	if err != nil {
		return nil, err
	}
	if info == nil || !info.Mode().IsRegular() {
		message := "settings.info.preview does not name a file: " + path
		if info == nil {
			message = "settings.info.preview names a file that does not exist: " + path
		}
		return nil, &diag.Error{
			Msg:  message,
			File: manifestFile,
			Hint: "The path starts at the project folder, where moonwell.pkl is.",
		}
	}
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, &diag.Error{
			Msg:   "Reading the preview picture failed: " + fsx.Reason(err),
			File:  path,
			Cause: err,
			Hint:  "Make sure no other program has the picture locked.",
		}
	}
	return ReadPicture(data, path)
}
