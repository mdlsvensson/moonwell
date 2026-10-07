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
		if one, other := gamePathKey(tt.one), gamePathKey(tt.other); (one == other) != tt.same {
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
		if got := gamePathKey(tt.path); got != tt.key {
			t.Errorf("gamePathKey(%q) = %q, want %q", tt.path, got, tt.key)
		}
	}
	for _, extension := range TextureExtensions {
		if got := gamePathKey("Textures/A." + extension); got != "textures/a.<texture>" {
			t.Errorf("a .%s texture has the key %q", extension, got)
		}
	}
	// The types written out: a model that names a texture of any of them is given the game's file of another.
	for _, extension := range []string{"blp", "dds", "tga", "tif", "tiff", "png", "jpg"} {
		if got := gamePathKey("Textures/A." + extension); got != "textures/a.<texture>" {
			t.Errorf("a .%s texture has the key %q", extension, got)
		}
	}
	// What looks like one of them and is none keeps its extension.
	for _, extension := range []string{"jpeg", "bmp", "tg", "ddsx", "wav", "pkb", "pkfx"} {
		if got := gamePathKey("Textures/A." + extension); got != "textures/a."+extension {
			t.Errorf("a .%s file has the key %q", extension, got)
		}
	}
}

// Letter case folds one character to one character: a capital I with a dot is a plain i.
func TestGamePathKeyFoldsEachLetterToOneLetter(t *testing.T) {
	if got := gamePathKey("Textures\\\xc4\xb0con.blp"); got != "textures/icon.<texture>" {
		t.Errorf("the key is %q", got)
	}
}

func TestParseGamePathsSkipsCommentsAndBlankLines(t *testing.T) {
	keys := ParseGamePaths("# Warcraft III 3.0\n\ntextures/black32.blp\nunits/human/footman/footman.mdx\n")
	if len(keys) != 2 || !keys[gamePathKey(`Textures\Black32.dds`)] || !keys[gamePathKey(`Units\Human\Footman\Footman.mdl`)] {
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
	if len(keys) < 30000 || !keys[gamePathKey(`Units\Human\Footman\Footman.mdx`)] || !keys[gamePathKey(`Textures\Black32.blp`)] {
		t.Errorf("the embedded list has %d keys", len(keys))
	}
	// Every path of the list has its key, and two paths that are one file to the game share one. The number is
	// that of the list of Warcraft III 3.0.0.24268: it changes when the list is generated again for a new version
	// of the game, and then this row changes with it.
	if len(keys) != 42114 {
		t.Errorf("the embedded list has %d keys, want 42114", len(keys))
	}
	if again := LoadGamePaths(); reflect.ValueOf(again).Pointer() != reflect.ValueOf(keys).Pointer() {
		t.Error("the embedded list is parsed again on a second call")
	}
}
