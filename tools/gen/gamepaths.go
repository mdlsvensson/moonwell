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

const gamePathsPath = "data/game-paths.txt"

var modelExtensions = []string{"mdx", "mdl", "pkfx", "pkb"}

func writeGamePaths(checkout string, args []string, out io.Writer) error {
	listFile, version := args[0], args[1]
	data, err := os.ReadFile(listFile)
	if err != nil {
		return errFile(listFile, err)
	}
	paths := parseGamePaths(fsx.DecodeText(data))
	if len(paths) == 0 {
		return errNoGamePaths()
	}
	text := renderGamePaths(paths, version)
	if err := os.WriteFile(pathIn(checkout, gamePathsPath), []byte(text), 0o666); err != nil {
		return errInCheckout(checkout, gamePathsPath, err)
	}
	fmt.Fprintln(out, "wrote "+gamePathsPath+": "+strconv.Itoa(len(paths))+" paths.")
	return nil
}

func parseGamePaths(list string) []string {
	seen := map[string]bool{}
	for line := range strings.SplitSeq(list, "\n") {
		if path, ok := normalizeGamePath(line); ok {
			seen[path] = true
		}
	}
	return slices.Sorted(maps.Keys(seen))
}

func normalizeGamePath(line string) (string, bool) {
	listed := strings.ReplaceAll(strings.ToLower(fsx.TrimASCIISpace(line)), `\`, "/")
	listed = listed[strings.LastIndex(listed, ":")+1:]
	segments := strings.FieldsFunc(listed, func(r rune) bool { return r == '/' })
	path := strings.Join(segments[afterLastContainer(segments):], "/")
	return path, isModelOrTexture(path)
}

func afterLastContainer(segments []string) int {
	start := 0
	for i, segment := range segments {
		if strings.HasSuffix(segment, ".w3mod") || strings.HasSuffix(segment, ".mpq") {
			start = i + 1
		}
	}
	return start
}

func isModelOrTexture(path string) bool {
	dot := strings.LastIndex(path, ".")
	if dot < 0 {
		return false
	}
	extension := path[dot+1:]
	return slices.Contains(modelExtensions, extension) || slices.Contains(assets.TextureExtensions, extension)
}

func renderGamePaths(paths []string, version string) string {
	var text strings.Builder
	text.WriteString("# Warcraft III " + version + "\n")
	for _, path := range paths {
		text.WriteString(path + "\n")
	}
	return text.String()
}

func errNoGamePaths() error {
	return errors.New("no model or texture paths were recognized in the listfile; check the export's format " +
		"(one file name per line, UTF-8). The path list was not changed.")
}
