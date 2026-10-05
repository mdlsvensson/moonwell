package main

import (
	"maps"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeGamePathStripsStoragePrefixesAndKeepsOnlyModelReferencedFileTypes(t *testing.T) {
	for _, c := range []struct{ line, want string }{
		{`war3.w3mod:Units\Human\Footman\Footman.mdx`, "units/human/footman/footman.mdx"},
		{"war3.w3mod:_hd.w3mod:Doodads/LordaeronSummer/Plants/Corn/plant1_Normal.dds",
			"doodads/lordaeronsummer/plants/corn/plant1_normal.dds"},
		{"war3.w3mod:_locales/enus.w3mod:Textures/Black32.blp", "textures/black32.blp"},
		{"_hd.w3mod/_locales/dede.w3mod/Textures/Black32.blp", "textures/black32.blp"},
		{`war3.mpq:Abilities\Spells\Human\Heal\Heal.mdl`, "abilities/spells/human/heal/heal.mdl"},
		{"  Effects/Fire.pkfx  ", "effects/fire.pkfx"},
		// Reforged stores its particle effects baked, as .pkb.
		{`war3.w3mod:_de.w3mod:abilities\ribbon\chainlightning.pkb`, "abilities/ribbon/chainlightning.pkb"},
		// A folder is a container, and a file is none: the last step of a path is kept whatever its name.
		{"Textures//Deep.w3mod//Black32.tga", "black32.tga"},
		{"textures/odd.mpq.png", "textures/odd.mpq.png"},
	} {
		if got, ok := normalizeGamePath(c.line); !ok || got != c.want {
			t.Errorf("normalizeGamePath(%q) = %q, %v, want %q", c.line, got, ok, c.want)
		}
	}
	for _, line := range []string{
		"war3.w3mod:Sound/Music/mp3Music/ArthasTheme.mp3", "war3.w3mod:Units/UnitData.slk", "", "   ",
		"war3.w3mod:", "Units/Footman", "units.mdx/footman", "Units/Footman.", "units/footman.mdx.w3mod",
	} {
		if got, ok := normalizeGamePath(line); ok {
			t.Errorf("normalizeGamePath(%q) kept %q", line, got)
		}
	}
}

// The white space taken off a line is ASCII, and letter case is Go's: what is not ASCII white space is part of
// the path, a byte order mark inside a text among it.
func TestNormalizeGamePathTakesASCIIWhiteSpaceOffALineAndLowersItAsGoDoes(t *testing.T) {
	for _, c := range []struct {
		line, want string
		kept       bool
	}{
		{"\t\v\f Units/A.mdx \r", "units/a.mdx", true},
		{"Units/A.mdx\n", "units/a.mdx", true},
		// A no-break space after the type of a file makes it another type; before the path it is part of it.
		{"Units/A.mdx\xC2\xA0", "units/a.mdx\xC2\xA0", false},
		{"\xC2\xA0Units/A.mdx", "\xC2\xA0units/a.mdx", true},
		{"Units/A.mdx\xE2\x80\xA8", "units/a.mdx\xE2\x80\xA8", false},
		{"\xEF\xBB\xBFUnits/A.mdx", "\xEF\xBB\xBFunits/a.mdx", true},
		// A capital I with a dot above is lowered to the plain letter i.
		{"Units/\xC4\xB0.MDX", "units/i.mdx", true},
	} {
		if got, ok := normalizeGamePath(c.line); got != c.want || ok != c.kept {
			t.Errorf("normalizeGamePath(%q) = %q, %v, want %q, %v", c.line, got, ok, c.want, c.kept)
		}
	}
}

func TestRenderGamePathsWritesAHeaderAndSortedUniquePaths(t *testing.T) {
	list := strings.Join([]string{
		"war3.w3mod:Textures/Black32.blp",
		"war3.w3mod:_hd.w3mod:Textures/Black32.blp",
		"war3.w3mod:Units/Human/Footman/Footman.mdx",
		"war3.w3mod:Sound/Hit.wav",
		"war3.w3mod:Abilities/Spells/Human/Heal/Heal.mdx",
	}, "\r\n")
	want := strings.Join([]string{
		"# Warcraft III 3.0.0.24268",
		"abilities/spells/human/heal/heal.mdx",
		"textures/black32.blp",
		"units/human/footman/footman.mdx",
		"",
	}, "\n")
	if got := renderGamePaths(list, "3.0.0.24268"); got != want {
		t.Errorf("renderGamePaths = %q", got)
	}
}

// The paths are sorted by their bytes: U+E000 stands ahead of U+10000, which an order by UTF-16 units puts first.
func TestRenderGamePathsSortsThePathsByBytes(t *testing.T) {
	list := "b.mdx\n\xF0\x90\x80\x80.mdx\nB.blp\n\xEE\x80\x80.mdx\na/z.mdx\na.mdx"
	want := "# Warcraft III 1\na.mdx\na/z.mdx\nb.blp\nb.mdx\n\xEE\x80\x80.mdx\n\xF0\x90\x80\x80.mdx\n"
	if got := renderGamePaths(list, "1"); got != want {
		t.Errorf("renderGamePaths = %q, want %q", got, want)
	}
}

// The committed list is a list of names too, and its first line, which names no file a model references, gives
// the version: it renders to itself, byte for byte. The file of the real checkout is read, and none is written.
func TestTheCommittedListOfTheGamesPathsRendersToItself(t *testing.T) {
	committed := string(realFile(t, "data/game-paths.txt"))
	first, _, _ := strings.Cut(committed, "\n")
	version, found := strings.CutPrefix(first, "# Warcraft III ")
	if !found {
		t.Fatalf("the first line of data/game-paths.txt is %q", first)
	}
	if got := renderGamePaths(committed, version); got != committed {
		t.Errorf("data/game-paths.txt renders to %d bytes that are not its %d", len(got), len(committed))
	}
}

func TestRenderGamePathsOfAListThatNamesNothingIsTheLineWithTheVersion(t *testing.T) {
	for _, list := range []string{"", "\r\n\r\n", "war3.w3mod:Sound/Hit.wav\n"} {
		if got := renderGamePaths(list, "2.0.0"); got != "# Warcraft III 2.0.0\n" {
			t.Errorf("renderGamePaths(%q) = %q", list, got)
		}
	}
}

func TestTheModeGamePathsWritesTheListAndPrintsHowManyPathsItHas(t *testing.T) {
	c := newCheckout(t)
	c.folder("data")
	list := exported(t, "listfile.txt", "war3.w3mod:Units/Human/Footman/Footman.mdx\n")
	printed, files, err := c.run("game-paths", list, "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if printed != "wrote data/game-paths.txt: 1 paths.\n" {
		t.Errorf("printed %q", printed)
	}
	want := map[string]string{"data/game-paths.txt": "# Warcraft III 2.0.0\nunits/human/footman/footman.mdx\n"}
	if got := texts(files); !maps.Equal(got, want) {
		t.Errorf("wrote %q", got)
	}
}

// The list is read as UTF-8: a byte order mark at its start is dropped, and bytes that are no UTF-8 become one
// replacement character together. A path named twice is written and counted once.
func TestTheModeGamePathsDecodesTheListAndCountsEachPathOnce(t *testing.T) {
	c := newCheckout(t)
	c.write("data/game-paths.txt", "# Warcraft III 1.0.0\nunits/old.mdx\n")
	list := exported(t, "listfile.txt",
		"\xEF\xBB\xBFUnits\\B\xFF\xFE.mdx\r\n\r\nwar3.w3mod:units/a.MDX\r\nUnits/A.mdx\r\nSound/Hit.wav")
	printed, files, err := c.run("game-paths", list, "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if printed != "wrote data/game-paths.txt: 2 paths.\n" {
		t.Errorf("printed %q", printed)
	}
	want := "# Warcraft III 2.0.0\nunits/a.mdx\nunits/b\xEF\xBF\xBD.mdx\n"
	if got := string(files["data/game-paths.txt"]); got != want {
		t.Errorf("wrote %q, want %q", got, want)
	}
}

func TestTheModeGamePathsFailsAndKeepsTheExistingListWhenNoPathIsRecognized(t *testing.T) {
	const existing = "# Warcraft III 1.0.0\nunits/old.mdx\n"
	// A version with a line feed gives the list a second line, which is no path: the count is of the paths.
	for _, version := range []string{"2.0.0", "2.0.0\nunits/new.mdx"} {
		c := newCheckout(t)
		c.write("data/game-paths.txt", existing)
		printed, files, err := c.run("game-paths", exported(t, "listfile.txt", "war3.w3mod:Sound/Hit.wav\n"), version)
		if err == nil {
			t.Fatalf("version %q: a listfile without a model or texture path was accepted", version)
		}
		contains(t, err.Error(), "no model or texture paths were recognized", "The path list was not changed")
		if printed != "" {
			t.Errorf("version %q: printed %q", version, printed)
		}
		if got := texts(files); !maps.Equal(got, map[string]string{"data/game-paths.txt": existing}) {
			t.Errorf("version %q: the list changed to %q", version, got)
		}
	}
}

// A file that the command line names is named in a failure as the line gave it, and its failure has the shape of
// one on a file of the checkout: the path, then the system's reason.
func TestTheModeGamePathsNamesAListItCannotReadAsTheLineDid(t *testing.T) {
	c := newCheckout(t)
	c.write("data/game-paths.txt", "# Warcraft III 1.0.0\nunits/old.mdx\n")
	missing := filepath.Join(t.TempDir(), "no-listfile.txt")
	printed, files, err := c.run("game-paths", missing, "2.0.0")
	if err == nil {
		t.Fatal("a listfile that is not there was read")
	}
	if !strings.HasPrefix(err.Error(), missing+": ") || strings.Count(err.Error(), missing) != 1 {
		t.Errorf("got %q, want the path as the line gave it, once, and then the reason", err)
	}
	if printed != "" || string(files["data/game-paths.txt"]) != "# Warcraft III 1.0.0\nunits/old.mdx\n" {
		t.Errorf("the refused run printed %q and left %q", printed, texts(files))
	}
}

// A file of the checkout is named by its path from there: no message holds the path of the checkout.
func TestTheModeGamePathsNamesTheFileItCannotWriteByItsPathFromTheCheckout(t *testing.T) {
	c := newCheckout(t)
	list := exported(t, "listfile.txt", "Units/A.mdx\n")
	printed, files, err := c.run("game-paths", list, "2.0.0")
	if err == nil {
		t.Fatal("the list was written into a folder that is not there")
	}
	contains(t, err.Error(), "data/game-paths.txt")
	if strings.Contains(err.Error(), c.root) {
		t.Errorf("the message holds the path of the checkout: %v", err)
	}
	if printed != "" || len(files) != 0 {
		t.Errorf("the failed run printed %q and left %q", printed, texts(files))
	}
}
