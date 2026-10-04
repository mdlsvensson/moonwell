package assets

import (
	"strings"
	"sync"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
)

// TextureExtensions are the texture types the game treats as one texture: Reforged stores what a model calls
// .tif or .blp as .dds or .blp.
var TextureExtensions = []string{"blp", "dds", "tga", "tif", "tiff", "png", "jpg"}

// anyTexture stands in a key for the extension of a texture, whichever of them the path has.
const anyTexture = ".<texture>"

// GamePathKey is how a path is compared with the game's own files: any letter case, either separator, a
// requested .mdl as the .mdx the game loads, and a texture without its extension.
func GamePathKey(path string) string {
	key := loadedModel(mapdir.Key(path))
	for _, extension := range TextureExtensions {
		if name, isTexture := strings.CutSuffix(key, "."+extension); isTexture {
			return name + anyTexture
		}
	}
	return key
}

// loadedModel is the file the game loads for the model at key: the .mdx of a .mdl, which it never loads, and
// key itself for any other file.
func loadedModel(key string) string {
	if name, isText := strings.CutSuffix(key, ".mdl"); isText {
		return name + ".mdx"
	}
	return key
}

// ParseGamePaths returns the keys of the paths a list names, one path on a line. A line that starts with "#"
// and a blank line name none, and the white space around a line is not part of its path.
func ParseGamePaths(list string) map[string]bool {
	keys := map[string]bool{}
	for line := range strings.Lines(list) {
		if path := strings.TrimSpace(line); path != "" && !strings.HasPrefix(path, "#") {
			keys[GamePathKey(path)] = true
		}
	}
	return keys
}

// embeddedGamePaths parses the list of the game's paths that the program carries, once.
var embeddedGamePaths = sync.OnceValue(func() map[string]bool {
	return ParseGamePaths(moonwell.GamePaths)
})

// LoadGamePaths returns the keys of the paths the game ships, from the list the program carries. Every call
// returns the one map, which is not to be changed.
func LoadGamePaths() map[string]bool { return embeddedGamePaths() }
