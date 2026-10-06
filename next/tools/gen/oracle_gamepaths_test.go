package main

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// ---- the runs of game-paths ----

// gamePathsOf is the line of the mode game-paths for a list with this text and for a version.
func gamePathsOf(list, version string) lineOfARun {
	return func(t testing.TB, outside string) []string {
		return []string{"game-paths", testkit.WriteFile(t, outside, "listfile.txt", []byte(list)), version}
	}
}

// listOfAnotherVersion is a data/game-paths.txt that a run finds in its checkout.
const listOfAnotherVersion = "# Warcraft III 1.0.0\nunits/old.mdx\n"

// anotherList lays a data/game-paths.txt of another version.
func anotherList(c checkout) { c.write(gamePathsPath, listOfAnotherVersion) }

// noList lays a data folder that holds nothing.
func noList(c checkout) { c.folder("data") }

// linesOfEveryForm is the lines of a list of file names in every form that the tests of gamepaths_test.go name:
// what says where the game stores a file, both separators, mixed case, ASCII white space around a name, blank
// lines, a path that several lines name, every type that is kept, and names that are not kept.
var linesOfEveryForm = []string{
	`war3.w3mod:Units\Human\Footman\Footman.mdx`,
	"war3.w3mod:_hd.w3mod:Doodads/LordaeronSummer/Plants/Corn/plant1_Normal.dds",
	"war3.w3mod:_locales/enus.w3mod:Textures/Black32.blp",
	"_hd.w3mod/_locales/dede.w3mod/Textures/Black32.blp",
	`war3.mpq:Abilities\Spells\Human\Heal\Heal.mdl`,
	"  Effects/Fire.pkfx  ",
	`war3.w3mod:_de.w3mod:abilities\ribbon\chainlightning.pkb`,
	"Textures//Deep.w3mod//Black32.tga",
	"textures/odd.mpq.png",
	"",
	"   ",
	"war3.w3mod:Sound/Music/mp3Music/ArthasTheme.mp3",
	"war3.w3mod:Units/UnitData.slk",
	"war3.w3mod:",
	"Units/Footman",
	"units.mdx/footman",
	"Units/Footman.",
	"units/footman.mdx.w3mod",
	"UNITS/HUMAN/FOOTMAN/FOOTMAN.MDX",
	"\t\v\f Units/A.mdx ",
	"Textures/Kept.tif", "Textures/Kept.tiff", "Textures/Kept.jpg", "Textures/Kept.PNG",
	"Textures/NotKept.jpeg", "Textures/NotKept.bmp", "Textures/NotKept.mdx2", "Textures/NotKept.md",
	"abilities/z.mdx", "Abilities/a.mdx", "abilities.mdx",
}

// oddLines is lines of other shapes: a carriage return, a tab and a space inside a name; several colons; dots
// and slashes at the ends; letters outside ASCII, in both cases; a name that is only a type; a type in a
// folder's name; and ASCII white space at the edges of a step inside a line, which is after the colon, on both
// sides of a slash, and on both sides of the dot.
const oddLines = "Units/A\rB.mdx\n" +
	"Units/Tab\there.mdx\n" +
	"Units/With space.blp\n" +
	"a:b:c:Units/Colons.mdx\n" +
	"Units/Colon.mdx:\n" +
	"/Units/Leading.mdx\n" +
	"Units/Trailing.mdx/\n" +
	"Units/Two..mdx\n" +
	"..mdx\n" +
	".mdx\n" +
	"mdx\n" +
	"Units/\xC3\x84RGER.MDX\n" +
	"units/\xC3\xA4rger.blp\n" +
	"Units/\xCE\x91\xCE\xA3.dds\n" +
	"Units/\xE6\x97\xA5\xE6\x9C\xAC.tga\n" +
	"Units.w3mod.mdx/Inside.mdx\n" +
	"Units/Upper.MPQ/Inside.mdx\n" +
	"war3.w3mod: Units/AfterTheColon.mdx\n" +
	"war3.w3mod:\tUnits/AfterTheColonATab.mdx\n" +
	"war3.w3mod :Units/BeforeTheColon.mdx\n" +
	"Units / AroundTheSlash.mdx\n" +
	"Units.w3mod /AfterAContainer.mdx\n" +
	"Units/BeforeTheDot .mdx\n" +
	"Units/AfterTheDot. mdx\n" +
	"Units/BeforeALastColon.mdx :\n"

// gamePathsRuns is the runs of game-paths: a list in every form the tests of gamepaths_test.go name (where the
// game stores a file, both separators, mixed case, blank lines, a path named twice, types that are not kept),
// with both kinds of line break; lines of other shapes, ASCII white space at the edges of a step inside a line
// among them; a byte order mark at the start of the list, which both trees pass over, the other tree as white
// space before the first name and this tree as it decodes the file; one byte that is no UTF-8; a version that is
// empty; seeded changes of the list (a line cut, a line doubled, a character of ASCII white space put in); a
// list that names nothing, in three forms and with and without a list in the checkout; a list file that is not
// there and one that is a folder; a wrong count of arguments; a data folder that is not there, and a folder at
// the place of the list; a run in a folder below the checkout; the runs of the classes, which the header of
// oracle_test.go names; and a folder that is no checkout.
func gamePathsRuns() []oracleRun {
	withLineFeeds := strings.Join(linesOfEveryForm, "\n")
	runs := []oracleRun{
		{name: "a list in every form, over the list of another version", lay: anotherList,
			line: gamePathsOf(strings.Join(linesOfEveryForm, "\r\n")+"\r\n", "3.0.0.24268")},
		{name: "a list in every form with line feeds and no last one, into an empty data folder", lay: noList,
			line: gamePathsOf(withLineFeeds, "3.0.0.24268")},
		{name: "lines of other shapes", lay: noList, line: gamePathsOf(oddLines, "2.0.0")},
		{name: "a byte order mark at the start of the list", lay: noList,
			line: gamePathsOf(mark+"Units/B.mdx\r\n \tUnits/A.mdx\r\n", "2.0.0")},
		{name: "one byte that is no UTF-8", lay: noList, line: gamePathsOf("Units/B\xFF.mdx\nUnits/A.mdx\n", "2.0.0")},
		{name: "a version that is empty", lay: noList, line: gamePathsOf("Units/A.mdx\n", "")},
		{name: "a list that is empty, beside a list", lay: anotherList, line: gamePathsOf("", "2.0.0")},
		{name: "a list of blank lines, beside a list", lay: anotherList, line: gamePathsOf("\r\n \r\n\t\n", "2.0.0")},
		{name: "a list of types that are not kept, beside a list", lay: anotherList,
			line: gamePathsOf("war3.w3mod:Sound/Hit.wav\nUnits/UnitData.slk\n", "2.0.0")},
		{name: "a list that names nothing, into an empty data folder", lay: noList,
			line: gamePathsOf("war3.w3mod:Sound/Hit.wav\n", "2.0.0")},
		{name: "a list file that is not there", lay: anotherList, class: "AsGiven", cannotGive: &named{argument: 1},
			line: func(_ testing.TB, outside string) []string {
				return []string{"game-paths", filepath.Join(outside, "no-listfile.txt"), "2.0.0"}
			}},
		{name: "a folder for the list file", lay: anotherList, class: "AsGiven", cannotGive: &named{argument: 1},
			line: func(t testing.TB, outside string) []string {
				testkit.WriteFile(t, outside, "folder/listfile.txt", []byte("Units/A.mdx\n"))
				return []string{"game-paths", filepath.Join(outside, "folder"), "2.0.0"}
			}},
		{name: "game-paths alone", lay: anotherList, line: words("game-paths")},
		{name: "game-paths without a version", lay: anotherList, line: words("game-paths", "listfile.txt")},
		{name: "game-paths with an argument too many", lay: anotherList,
			line: words("game-paths", "listfile.txt", "2.0.0", "more")},
		{name: "no data folder for the list", class: "FromCheckout", line: gamePathsOf("Units/A.mdx\n", "2.0.0")},
		{name: "a folder at the place of the list", class: "FromCheckout", line: gamePathsOf("Units/A.mdx\n", "2.0.0"),
			lay: func(c checkout) { c.write(gamePathsPath+"/held.txt", "held\n") }},
		{name: "game-paths in a folder below the checkout", below: "tools/gen/slk", lay: noList,
			line: gamePathsOf("Units/A.mdx\n", "2.0.0")},
		{name: "a version with a line feed", lay: anotherList, class: "CountOfPaths",
			line: gamePathsOf("Units/B.mdx\nUnits/A.mdx\nSound/Hit.wav\n", "2.0.0\nunits/new.mdx")},
		{name: "a no-break space before a name of the list", lay: noList, class: "WiderSpace",
			line:  gamePathsOf("Units/A.mdx\n\xC2\xA0Units/B.mdx\n", "2.0.0"),
			apart: inTheListWritten("\nunits/b.mdx\n", "\n\xC2\xA0units/b.mdx\n")},
		{name: "a byte order mark at the start of the second line of the list", lay: noList, class: "WiderSpace",
			line:  gamePathsOf("Units/A.mdx\n"+mark+"Units/B.mdx\n", "2.0.0"),
			apart: inTheListWritten("\nunits/b.mdx\n", "\n"+mark+"units/b.mdx\n")},
		{name: "a no-break space after a name of the list", lay: noList, class: "WiderSpace",
			line: gamePathsOf(aNameThenWiderSpace, "2.0.0"),
			apart: map[string][]place{
				gamePathsPath:  {{"units/b.mdx\n", ""}},
				standardOutput: {{": 2 paths.", ": 1 paths."}},
			}},
		{name: "a capital I with a dot above in a name of the list", lay: noList, class: "DottedI",
			line:  gamePathsOf("Units/\xC4\xB0.MDX\n", "2.0.0"),
			apart: inTheListWritten("\nunits/i\xCC\x87.mdx\n", "\nunits/i.mdx\n")},
		{name: "two bytes that are no UTF-8, side by side", lay: noList, class: "NoUTF8",
			line:  gamePathsOf("Units\\B\xFF\xFE.mdx\n", "2.0.0"),
			apart: inTheListWritten("\nunits/b\xEF\xBF\xBD\xEF\xBF\xBD.mdx\n", "\nunits/b\xEF\xBF\xBD.mdx\n")},
		{name: "a character above U+FFFF beside one from U+E000 on", lay: noList, class: "ByBytes",
			line: gamePathsOf("\xF0\x90\x80\x80.mdx\n\xEE\x80\x80.mdx\n", "2.0.0"),
			apart: inTheListWritten("\n\xF0\x90\x80\x80.mdx\n\xEE\x80\x80.mdx\n",
				"\n\xEE\x80\x80.mdx\n\xF0\x90\x80\x80.mdx\n")},
	}
	runs = append(runs, changedLists(withLineFeeds+"\n")...)
	runs = append(runs, noCheckoutRuns("game-paths", gamePathsOf("Units/A.mdx\n", "2.0.0"))...)
	return append(runs, noCheckoutRuns("game-paths alone", words("game-paths"))...)
}

// aNameThenWiderSpace is a list whose second line ends with a no-break space: to the other tree a path with
// white space after it, and to this tree a file of a type that is not kept.
const aNameThenWiderSpace = "Units/A.mdx\nUnits/B.mdx\xC2\xA0\n"

// inTheListWritten is what a run of an accepted difference in a list carries: the one place of
// data/game-paths.txt that the two trees write apart, as the other tree writes it and as this tree must.
func inTheListWritten(other, this literal) map[string][]place {
	return map[string][]place{gamePathsPath: {{other, this}}}
}

// changedLists is a list after seeded changes, eight times: one to three changes of each, a line cut, a line
// doubled, or a character of ASCII white space put in. A change is named by the seed and its index, which make
// it again.
func changedLists(list string) []oracleRun {
	const seed, changes = 2026_10_05, 8
	var runs []oracleRun
	for index := uint64(1); index <= changes; index++ {
		runs = append(runs, oracleRun{
			name: fmt.Sprintf("the list in every form, changed: seed %d, change %d", seed, index),
			lay:  anotherList,
			line: gamePathsOf(oracle.Changed(list, seed, index), "2.0.0"),
		})
	}
	return runs
}

// theGamesList is the run on the game's own list of file names, with the version that the committed list
// states: what each tree writes is the committed data/game-paths.txt.
func theGamesList(t testing.TB, export testkit.Export) []oracleRun {
	first, _, _ := strings.Cut(string(realFile(t, gamePathsPath)), "\n")
	version, found := strings.CutPrefix(first, "# Warcraft III ")
	if !found {
		t.Fatalf("the first line of %s is %q", gamePathsPath, first)
	}
	return []oracleRun{{
		name:        "the game's list of file names, with the version of the committed list",
		lay:         noList,
		line:        words("game-paths", export.Path(), version),
		asCommitted: []string{gamePathsPath},
	}}
}
