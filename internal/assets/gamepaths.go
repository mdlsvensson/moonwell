package assets

import (
	"strings"
	"sync"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

var TextureExtensions = []string{"blp", "dds", "tga", "tif", "tiff", "png", "jpg"}

func LoadGamePaths() map[string]bool { return embeddedGamePaths() }

var embeddedGamePaths = sync.OnceValue(func() map[string]bool {
	return ParseGamePaths(moonwell.GamePaths)
})

func ParseGamePaths(list string) map[string]bool {
	keys := map[string]bool{}
	for line := range strings.Lines(list) {
		if path := strings.TrimSpace(line); path != "" && !strings.HasPrefix(path, "#") {
			keys[gamePathKey(path)] = true
		}
	}
	return keys
}

const anyTexture = ".<texture>"

func gamePathKey(path string) string {
	key := loadedModelKey(mapdir.Key(path))
	for _, extension := range TextureExtensions {
		if name, isTexture := strings.CutSuffix(key, "."+extension); isTexture {
			return name + anyTexture
		}
	}
	return key
}

func loadedModelKey(key string) string {
	if name, isText := strings.CutSuffix(key, ".mdl"); isText {
		return name + ".mdx"
	}
	return key
}
