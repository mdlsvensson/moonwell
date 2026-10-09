package build

import (
	"path/filepath"
	"testing"
)

func TestCountsInProjectTakesSourcesModulesAssetsObjectFilesAndManifestsOnly(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "proj")
	tests := map[string]bool{
		"src/main.yue":                     true,
		"src/game/units.yue":               true,
		"src/notes.txt":                    false,
		"src/generated/objects.yue":        false,
		"moonwell.pkl":                     true,
		"moonwell.local.pkl":               true,
		"PklProject":                       true,
		"PklProject.deps.json":             true,
		"assets/icons/a.blp":               true,
		"objects/units.pkl":                true,
		"objects/human/barracks/units.pkl": true,
		"objects/notes.txt":                false,
		"lua/tools/init.lua":               true,
		"lua/notes.txt":                    false,
		"dist/stage/lua/main.lua":          false,
		"moonwell.lock":                    false,
		"README.md":                        false,
	}
	for file, want := range tests {
		if got := isProjectSource(dir, filepath.Join(dir, filepath.FromSlash(file))); got != want {
			t.Errorf("countsInProject(%s) = %v", file, got)
		}
	}
}

func TestCountsInLibraryPassesOverWhatIsUnderADotFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "lib", "src")
	tests := map[string]bool{
		"example/greet.lua":    true,
		"example":              true,
		".git/index":           false,
		"example/.cache/x.lua": false,
	}
	for file, want := range tests {
		if got := isLibrarySource(dir, filepath.Join(dir, filepath.FromSlash(file))); got != want {
			t.Errorf("countsInLibrary(%s) = %v", file, got)
		}
	}
}
