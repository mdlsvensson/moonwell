package main

import (
	"fmt"
	"maps"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
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
		{`war3.w3mod:_de.w3mod:abilities\ribbon\chainlightning.pkb`, "abilities/ribbon/chainlightning.pkb"},
		{"Textures//Deep.w3mod//Black32.tga", "black32.tga"},
		{"Textures/Deep.mpq/Black32.tga", "black32.tga"},
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

func TestNormalizeGamePathTakesASCIIWhiteSpaceOffALineAndLowersItAsGoDoes(t *testing.T) {
	for _, c := range []struct {
		line, want string
		kept       bool
	}{
		{"\t\v\f Units/A.mdx \r", "units/a.mdx", true},
		{"Units/A.mdx\n", "units/a.mdx", true},
		{"Units/A.mdx\xC2\xA0", "units/a.mdx\xC2\xA0", false},
		{"\xC2\xA0Units/A.mdx", "\xC2\xA0units/a.mdx", true},
		{"Units/A.mdx\xE2\x80\xA8", "units/a.mdx\xE2\x80\xA8", false},
		{"\xEF\xBB\xBFUnits/A.mdx", "\xEF\xBB\xBFunits/a.mdx", true},
		{"Units/\xC4\xB0.MDX", "units/i.mdx", true},
		{"war3.w3mod: Units/A.mdx", " units/a.mdx", true},
		{"war3.w3mod:\tUnits/A.mdx", "\tunits/a.mdx", true},
		{"Units / A.mdx", "units / a.mdx", true},
		{"Units/A .mdx", "units/a .mdx", true},
		{"Units/A. mdx", "units/a. mdx", false},
		{"Units/A.mdx :", "", false},
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
	if got := renderGamePaths(parseGamePaths(list), "3.0.0.24268"); got != want {
		t.Errorf("renderGamePaths = %q", got)
	}
}

func TestRenderGamePathsSortsThePathsByBytes(t *testing.T) {
	list := "b.mdx\n\xF0\x90\x80\x80.mdx\nB.blp\n\xEE\x80\x80.mdx\na/z.mdx\na.mdx"
	want := "# Warcraft III 1\na.mdx\na/z.mdx\nb.blp\nb.mdx\n\xEE\x80\x80.mdx\n\xF0\x90\x80\x80.mdx\n"
	if got := renderGamePaths(parseGamePaths(list), "1"); got != want {
		t.Errorf("renderGamePaths = %q, want %q", got, want)
	}
}

func TestTheCommittedListOfTheGamesPathsRendersToItself(t *testing.T) {
	committed := string(realFile(t, "data/game-paths.txt"))
	first, _, _ := strings.Cut(committed, "\n")
	version, found := strings.CutPrefix(first, "# Warcraft III ")
	if !found {
		t.Fatalf("the first line of data/game-paths.txt is %q", first)
	}
	if got := renderGamePaths(parseGamePaths(committed), version); got != committed {
		t.Errorf("data/game-paths.txt renders to %d bytes that are not its %d", len(got), len(committed))
	}
}

func TestRenderGamePathsOfAListThatNamesNothingIsTheLineWithTheVersion(t *testing.T) {
	for _, list := range []string{"", "\r\n\r\n", "war3.w3mod:Sound/Hit.wav\n"} {
		if got := renderGamePaths(parseGamePaths(list), "2.0.0"); got != "# Warcraft III 2.0.0\n" {
			t.Errorf("renderGamePaths(%q) = %q", list, got)
		}
	}
}

func TestTheModeGamePathsWritesTheListAndPrintsHowManyPathsItHas(t *testing.T) {
	c := newFakeCheckout(t)
	c.makeDir("data")
	list := writeExportFile(t, "listfile.txt", "war3.w3mod:Units/Human/Footman/Footman.mdx\n")
	printed, files, err := c.runGen("game-paths", list, "2.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if printed != "wrote data/game-paths.txt: 1 paths.\n" {
		t.Errorf("printed %q", printed)
	}
	want := withGoMod(map[string]string{
		"data/game-paths.txt": "# Warcraft III 2.0.0\nunits/human/footman/footman.mdx\n",
	})
	if got := toTexts(files); !maps.Equal(got, want) {
		t.Errorf("wrote %q", got)
	}
}

func TestTheModeGamePathsDecodesTheListAndCountsEachPathOnce(t *testing.T) {
	c := newFakeCheckout(t)
	c.writeFile("data/game-paths.txt", "# Warcraft III 1.0.0\nunits/old.mdx\n")
	list := writeExportFile(t, "listfile.txt",
		"\xEF\xBB\xBFUnits\\B\xFF\xFE.mdx\r\n\r\nwar3.w3mod:units/a.MDX\r\nUnits/A.mdx\r\nSound/Hit.wav")
	printed, files, err := c.runGen("game-paths", list, "2.0.0")
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
	for _, version := range []string{"2.0.0", "2.0.0\nunits/new.mdx"} {
		c := newFakeCheckout(t)
		c.writeFile("data/game-paths.txt", existing)
		printed, files, err := c.runGen("game-paths", writeExportFile(t, "listfile.txt", "war3.w3mod:Sound/Hit.wav\n"), version)
		if err == nil {
			t.Fatalf("version %q: a listfile without a model or texture path was accepted", version)
		}
		checkContains(t, err.Error(), "no model or texture paths were recognized", "The path list was not changed")
		if printed != "" {
			t.Errorf("version %q: printed %q", version, printed)
		}
		if got := toTexts(files); !maps.Equal(got, withGoMod(map[string]string{"data/game-paths.txt": existing})) {
			t.Errorf("version %q: the list changed to %q", version, got)
		}
	}
}

func TestTheModeGamePathsNamesAListItCannotReadAsTheLineDid(t *testing.T) {
	c := newFakeCheckout(t)
	c.writeFile("data/game-paths.txt", "# Warcraft III 1.0.0\nunits/old.mdx\n")
	missing := filepath.Join(t.TempDir(), "no-listfile.txt")
	printed, files, err := c.runGen("game-paths", missing, "2.0.0")
	if err == nil {
		t.Fatal("a listfile that is not there was read")
	}
	if !strings.HasPrefix(err.Error(), missing+": ") || strings.Count(err.Error(), missing) != 1 {
		t.Errorf("got %q, want the path as the line gave it, once, and then the reason", err)
	}
	if printed != "" || string(files["data/game-paths.txt"]) != "# Warcraft III 1.0.0\nunits/old.mdx\n" {
		t.Errorf("the refused run printed %q and left %q", printed, toTexts(files))
	}
}

func TestTheModeGamePathsNamesTheFileItCannotWriteByItsPathFromTheCheckout(t *testing.T) {
	c := newFakeCheckout(t)
	list := writeExportFile(t, "listfile.txt", "Units/A.mdx\n")
	printed, files, err := c.runGen("game-paths", list, "2.0.0")
	if err == nil {
		t.Fatal("the list was written into a folder that is not there")
	}
	checkContains(t, err.Error(), "data/game-paths.txt")
	if strings.Contains(err.Error(), c.root) {
		t.Errorf("the message holds the path of the checkout: %v", err)
	}
	if printed != "" || !asNew(files) {
		t.Errorf("the failed run printed %q and left %q", printed, toTexts(files))
	}
}

func TestTheModeGamePathsWritesTheCommittedListFromTheGamesList(t *testing.T) {
	list := testkit.NeedExport(t, "MOONWELL_GAME_LISTFILE").Path()
	want := string(realFile(t, gamePathsPath))
	first, paths, _ := strings.Cut(want, "\n")
	version, found := strings.CutPrefix(first, "# Warcraft III ")
	if !found {
		t.Fatalf("the first line of %s is %q", gamePathsPath, first)
	}
	c := newFakeCheckout(t)
	c.makeDir("data")
	printed, files, err := c.runGen("game-paths", list, version)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(files[gamePathsPath]); got != want {
		t.Errorf("the game's list does not give the committed %s: %s", gamePathsPath, describeDifference(want, got))
	}
	if count := fmt.Sprintf("wrote data/game-paths.txt: %d paths.\n", strings.Count(paths, "\n")); printed != count {
		t.Errorf("the run printed %q, want %q", printed, count)
	}
}
