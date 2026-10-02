// Package mapdir works with a Warcraft III map saved as a folder: its file names, which the game matches without
// regard to letter case, and writing a set of planned changes into it.
package mapdir

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Key is a path's identity as Warcraft III and Windows see it: "/" separators, any letter case.
func Key(path string) string {
	return text.Lower(strings.ReplaceAll(path, `\`, "/"))
}

// Names maps the key of each wanted file to the spelling it has among entries, the names in a map folder: a map
// saved with war3mapskin.txt is patched under that name rather than gaining a second war3mapSkin.txt. Two spellings
// of one file fail, naming label(spelling).
func Names(entries, wanted []string, label func(name string) string) (map[string]string, error) {
	wantedKeys := map[string]bool{}
	for _, name := range wanted {
		wantedKeys[Key(name)] = true
	}
	sorted := slices.Clone(entries)
	text.Sort(sorted)
	names := map[string]string{}
	for _, name := range sorted {
		key := Key(name)
		if !wantedKeys[key] {
			continue
		}
		if other, ok := names[key]; ok {
			return nil, &diag.Error{
				Msg:  "Map files " + other + " and " + name + " differ only in letter case.",
				File: label(name),
				Hint: "Warcraft III ignores letter case in map paths; delete or rename one of them in the source map.",
			}
		}
		names[key] = name
	}
	return names, nil
}

// Change is the complete new content of one file of a map folder, or its removal.
type Change struct {
	Name   string // relative to the map folder, in the spelling to write
	Bytes  []byte
	Remove bool
}

// Apply writes changes into the map folder dir, in order. A failure is reported with the message failure, the file
// that could not be written and the hint to close the programs that may hold it.
func Apply(dir string, changes []Change, failure string) error {
	for _, change := range changes {
		file := filepath.Join(dir, filepath.FromSlash(change.Name))
		var err error
		if change.Remove {
			err = os.Remove(file)
		} else {
			err = os.WriteFile(file, change.Bytes, 0o666)
		}
		if err != nil {
			return &diag.Error{
				Msg:   failure,
				File:  file,
				Cause: err,
				Hint:  "Close Warcraft III or World Editor if they have the staged map open, then rebuild.",
			}
		}
	}
	return nil
}
