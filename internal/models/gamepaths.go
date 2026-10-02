package models

import (
	"regexp"
	"slices"
	"strings"
	"sync"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// TextureExtensions are the texture types the game treats as one texture: Reforged stores what models call .tif or
// .blp as .dds (HD) or .blp (SD).
var TextureExtensions = []string{"blp", "dds", "tga", "tif", "tiff", "png", "jpg"}

var keptExtensions = append([]string{"mdx", "mdl", "pkfx", "pkb"}, TextureExtensions...)

var (
	extension  = regexp.MustCompile(`\.([a-z0-9]+)$`)
	container  = regexp.MustCompile(`\.(w3mod|mpq)$`)
	lineBreaks = regexp.MustCompile(`\r?\n`)
)

func extensionOf(path string) (string, bool) {
	match := extension.FindStringSubmatch(path)
	if match == nil {
		return "", false
	}
	return match[1], true
}

// NormalizeGamePath turns one line of a CASC file-name export into an in-game path (lower case, "/"). It is false
// for a blank line and for a type a model cannot reference. Storage prefixes go: everything up to the last ":",
// then every folder up to the last ".w3mod" or ".mpq" container folder.
func NormalizeGamePath(line string) (string, bool) {
	path := strings.ReplaceAll(text.Lower(text.Trim(line)), `\`, "/")
	path = path[strings.LastIndex(path, ":")+1:]
	var segments []string
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	last := -1
	for i, segment := range segments {
		if i < len(segments)-1 && container.MatchString(segment) {
			last = i
		}
	}
	kept := strings.Join(segments[last+1:], "/")
	ext, ok := extensionOf(kept)
	return kept, kept != "" && ok && slices.Contains(keptExtensions, ext)
}

// RenderGamePaths renders the text of game-paths.txt for a CASC file-name export: a version header, then sorted
// unique paths.
func RenderGamePaths(list, version string) string {
	seen := map[string]bool{}
	var paths []string
	for _, line := range lineBreaks.Split(list, -1) {
		if path, ok := NormalizeGamePath(line); ok && !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	text.Sort(paths)
	return "# Warcraft III " + version + "\n" + strings.Join(append(paths, ""), "\n")
}

// GamePathKey is how a path is compared with the in-game list: any letter case, either separator, a requested .mdl
// as the .mdx the game loads, and textures without their extension.
func GamePathKey(path string) string {
	key := text.Lower(strings.ReplaceAll(path, `\`, "/"))
	if strings.HasSuffix(key, ".mdl") {
		key = strings.TrimSuffix(key, ".mdl") + ".mdx"
	}
	if ext, ok := extensionOf(key); ok && slices.Contains(TextureExtensions, ext) {
		return key[:len(key)-len(ext)-1] + ".<texture>"
	}
	return key
}

// ParseGamePaths returns the match keys of every path in a game-paths.txt text; "#" lines and blank lines are
// skipped.
func ParseGamePaths(list string) map[string]bool {
	keys := map[string]bool{}
	for _, line := range lineBreaks.Split(list, -1) {
		if path := text.Trim(line); path != "" && !strings.HasPrefix(path, "#") {
			keys[GamePathKey(path)] = true
		}
	}
	return keys
}

// LoadGamePaths returns the embedded in-game path list as match keys, parsed once per run.
var LoadGamePaths = sync.OnceValue(func() map[string]bool {
	return ParseGamePaths(moonwell.GamePaths)
})
