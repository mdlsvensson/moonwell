package mpq

import (
	"encoding/binary"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/internal/w3i"
)

// HM3WHeader builds the legacy 512-byte "HM3W" map header: magic, 4 unused bytes, the name (NUL-terminated), flags
// and the maximum number of players.
func HM3WHeader(name string, flags, maxPlayers uint32) []byte {
	out := make([]byte, 512)
	copy(out, "HM3W")
	encoded := []byte(name)
	if limit := 512 - 8 - 1 - 8; len(encoded) > limit {
		encoded = encoded[:limit]
	}
	copy(out[8:], encoded)
	at := 8 + len(encoded) + 1
	binary.LittleEndian.PutUint32(out[at:], flags)
	binary.LittleEndian.PutUint32(out[at+4:], maxPlayers)
	return out
}

// Archive metadata a saved map folder may carry; stale copies must never be packed.
var archiveMetadata = map[string]bool{"(ATTRIBUTES)": true, "(LISTFILE)": true, "(SIGNATURE)": true}

// PackMap packs a staged map folder into the bytes of a .w3x.
func PackMap(mapDir, mapName string) ([]byte, error) {
	info, err := os.ReadFile(filepath.Join(mapDir, "war3map.w3i"))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, &diag.Error{
			Msg:  "war3map.w3i is missing from the map folder.",
			File: mapDir,
			Hint: "Save the source map from World Editor in folder format.",
		}
	}
	if err != nil {
		return nil, err
	}
	paths, err := fsx.ListFiles(mapDir)
	if err != nil {
		return nil, err
	}
	var files []File
	for _, path := range paths {
		if archiveMetadata[text.Upper(path)] {
			continue
		}
		data, err := os.ReadFile(filepath.Join(mapDir, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		files = append(files, File{Name: strings.ReplaceAll(path, "/", `\`), Data: data})
	}
	header, err := w3i.ReadHeader(info)
	if err != nil {
		return nil, err
	}
	var options Options
	if !header.Headerless() {
		options.Prefix = HM3WHeader(mapName, 0, 0)
	}
	return Write(files, options)
}
