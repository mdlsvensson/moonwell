package main

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// gamePathsPath is the list of the game's own paths, which the program carries, by its path from the checkout.
const gamePathsPath = "data/game-paths.txt"

// modelExtensions is the types of file that a model can reference beside a texture: a model, and a particle
// effect as it is authored (.pkfx) and as Reforged stores it, baked (.pkb).
var modelExtensions = []string{"mdx", "mdl", "pkfx", "pkb"}

// writeGamePaths is the mode game-paths: it writes data/game-paths.txt from a list of the game's file names, one
// name on a line, and prints how many paths the list has. It writes nothing when the list names no path.
func writeGamePaths(checkout string, args []string, out io.Writer) error {
	listFile, version := args[0], args[1]
	data, err := os.ReadFile(listFile)
	if err != nil {
		return errFile(listFile, err)
	}
	paths := gamePaths(fsx.DecodeText(data))
	if len(paths) == 0 {
		return errNoGamePaths()
	}
	text := renderGamePaths(paths, version)
	if err := os.WriteFile(fileIn(checkout, gamePathsPath), []byte(text), 0o666); err != nil {
		return errInCheckout(checkout, gamePathsPath, err)
	}
	fmt.Fprintln(out, "wrote "+gamePathsPath+": "+strconv.Itoa(len(paths))+" paths.")
	return nil
}

// gamePaths is the in-game paths that a list of the game's file names names, one name on a line: each path once,
// sorted by bytes.
func gamePaths(list string) []string {
	named := map[string]bool{}
	// A carriage return before a line feed is white space at the end of its line, which normalizeGamePath takes
	// off.
	for line := range strings.SplitSeq(list, "\n") {
		if path, kept := normalizeGamePath(line); kept {
			named[path] = true
		}
	}
	return slices.Sorted(maps.Keys(named))
}

// normalizeGamePath turns one line of a list of the game's file names into an in-game path: lower case, "/".
// It is false for a blank line and for a type of file no model can reference.
//
// What says where the game stores the file goes: everything up to the last ":", then every folder up to the last
// one that is a container.
func normalizeGamePath(line string) (string, bool) {
	path := strings.ReplaceAll(strings.ToLower(fsx.TrimASCIISpace(line)), `\`, "/")
	path = path[strings.LastIndex(path, ":")+1:]
	steps := strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
	kept := strings.Join(steps[pastContainers(steps):], "/")
	return kept, canBeReferenced(kept)
}

// pastContainers is where the in-game path starts among the steps of a stored path: after the last step named
// as an archive of the game (.w3mod, .mpq). A path whose last step is named so has no in-game path: it is no
// file that a model can reference.
func pastContainers(steps []string) int {
	start := 0
	for i, step := range steps {
		if strings.HasSuffix(step, ".w3mod") || strings.HasSuffix(step, ".mpq") {
			start = i + 1
		}
	}
	return start
}

// canBeReferenced reports whether a path in lower case is of a type of file that a model can reference: a
// texture, as assets knows one, a model or a particle effect. The type is what follows the last dot.
func canBeReferenced(path string) bool {
	dot := strings.LastIndex(path, ".")
	if dot < 0 {
		return false
	}
	extension := path[dot+1:]
	return slices.Contains(modelExtensions, extension) || slices.Contains(assets.TextureExtensions, extension)
}

// renderGamePaths is the text of data/game-paths.txt: a line with the version of the game that the paths are
// from, then the paths in the order given, each on a line.
func renderGamePaths(paths []string, version string) string {
	var text strings.Builder
	text.WriteString("# Warcraft III " + version + "\n")
	for _, path := range paths {
		text.WriteString(path + "\n")
	}
	return text.String()
}

// ---- errors ----

func errNoGamePaths() error {
	return errors.New("no model or texture paths were recognized in the listfile; check the export's format " +
		"(one file name per line, UTF-8). The path list was not changed.")
}
