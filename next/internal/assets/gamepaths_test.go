package assets

import (
	"maps"
	"reflect"
	"slices"
	"testing"
)

func TestGamePathKeyIgnoresTextureExtensionsLetterCaseAndSeparatorsAndReadsMdlAsMdx(t *testing.T) {
	tests := []struct {
		one, other string
		same       bool
	}{
		{`Doodads\LordaeronSummer\Plants\Corn\plant1_Normal.tif`, "doodads/lordaeronsummer/plants/corn/plant1_normal.dds", true},
		{`Textures\Black32.BLP`, "textures/black32.dds", true},
		{`Models\Glow.mdl`, "models/glow.mdx", true},
		{`Models\Glow.mdx`, "models/glow.blp", false},
		{`Textures\Other\Black32.blp`, "textures/black32.blp", false},
	}
	for _, tt := range tests {
		if one, other := GamePathKey(tt.one), GamePathKey(tt.other); (one == other) != tt.same {
			t.Errorf("%q has the key %q and %q the key %q, want the same: %v", tt.one, one, tt.other, other, tt.same)
		}
	}
}

func TestGamePathKeyTakesOnlyTheLastExtensionAndOnlyATexturesOff(t *testing.T) {
	tests := []struct{ path, key string }{
		{`Textures\Black32.BLP`, "textures/black32.<texture>"},
		{"a.tiff", "a.<texture>"},
		{"a.tif.png", "a.tif.<texture>"},
		{".jpg", ".<texture>"},
		{"a.blp/b", "a.blp/b"},
		{"a.blpx", "a.blpx"},
		{"blp", "blp"},
		{"Units/Footman.MDL", "units/footman.mdx"},
		{"a.mdl.mdl", "a.mdl.mdx"},
		{"a.mdl/b", "a.mdl/b"},
		{"Sound/Horn.wav", "sound/horn.wav"},
		{"Particles/Fire.pkb", "particles/fire.pkb"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := GamePathKey(tt.path); got != tt.key {
			t.Errorf("GamePathKey(%q) = %q, want %q", tt.path, got, tt.key)
		}
	}
	for _, extension := range TextureExtensions {
		if got := GamePathKey("Textures/A." + extension); got != "textures/a.<texture>" {
			t.Errorf("a .%s texture has the key %q", extension, got)
		}
	}
}

// Letter case folds one character to one character: a capital I with a dot is a plain i.
func TestGamePathKeyFoldsEachLetterToOneLetter(t *testing.T) {
	if got := GamePathKey("Textures\\\xc4\xb0con.blp"); got != "textures/icon.<texture>" {
		t.Errorf("the key is %q", got)
	}
}

func TestParseGamePathsSkipsCommentsAndBlankLines(t *testing.T) {
	keys := ParseGamePaths("# Warcraft III 3.0\n\ntextures/black32.blp\nunits/human/footman/footman.mdx\n")
	if len(keys) != 2 || !keys[GamePathKey(`Textures\Black32.dds`)] || !keys[GamePathKey(`Units\Human\Footman\Footman.mdl`)] {
		t.Errorf("keys = %v", keys)
	}
	if got := ParseGamePaths("# not generated yet\n"); len(got) != 0 {
		t.Errorf("a list with only a comment has keys %v", got)
	}
}

// A list checked out with the line ends of Windows, or indented by hand, holds the same paths. White space is
// what Unicode calls so: a no-break space and a next-line character go, and a byte order mark is part of a path.
func TestParseGamePathsTakesTheWhiteSpaceOffEachLine(t *testing.T) {
	list := "  # a comment\r\n\t textures/a.blp \t\r\n \r\n\xc2\xa0textures/b.blp\xc2\x85\n\xef\xbb\xbftextures/c.blp\ntextures/d e.blp"
	want := []string{"textures/a.<texture>", "textures/b.<texture>", "textures/d e.<texture>", "\xef\xbb\xbftextures/c.<texture>"}
	if got := slices.Sorted(maps.Keys(ParseGamePaths(list))); !slices.Equal(got, want) {
		t.Errorf("keys = %q, want %q", got, want)
	}
}

func TestTheEmbeddedPathListIsTheGames(t *testing.T) {
	keys := LoadGamePaths()
	if len(keys) < 30000 || !keys[GamePathKey(`Units\Human\Footman\Footman.mdx`)] || !keys[GamePathKey(`Textures\Black32.blp`)] {
		t.Errorf("the embedded list has %d keys", len(keys))
	}
	if again := LoadGamePaths(); reflect.ValueOf(again).Pointer() != reflect.ValueOf(keys).Pointer() {
		t.Error("the embedded list is parsed again on a second call")
	}
}
