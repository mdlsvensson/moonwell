package ini_test

import (
	"bytes"
	"fmt"
	"maps"
	"math/rand/v2"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	oldtext "github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/tools/gen/ini"
	oldini "github.com/mdlsvensson/moonwell/tools/gen/ini"
)

// What this file compares. An input is the bytes of one file, or of several that are read into one result. The
// other tree's parser is given each as its tree decodes a file (text.Lossy, which keeps a byte order mark; that
// parser drops one at the start itself), and this tree's as its own does (fsx.DecodeText, which drops a mark at
// the start). What each makes of them is compared whole, through the oracle: every section, every key and every
// value. The other tree keeps the order in which sections and keys came and this tree keeps none, so what the
// two hold is compared, and not its order.
//
// The inputs are every text of ini_test.go, some of them with a mark at their start; seeded changes of those
// texts (a line cut, a line doubled, a quote dropped, ASCII white space put where the text has some, and at its
// start and its end), every third with a mark at its start; and, on a machine that has the game's files, every
// file directly in the locale's units folder whose name ends with strings.txt, each alone and all of them read
// into one result, and ui/worldeditstrings.txt.
//
// One class of inputs is counted apart, decided by its bytes alone: a text with a character that is white space
// outside ASCII (hasWiderSpace). The other tree trims such a character from a line, a key, a value and the name
// of a section, and this tree takes it for text (TestWhiteSpaceOutsideASCIIIsText). Both trees are still given
// every input of the class, and the tally says on how many they agree: they do where the character stands inside
// a value or a name.
//
// Not among the inputs: bytes that are not UTF-8. The two decoders write a run of them differently, which is no
// difference of the parsers.

const mark = "\xEF\xBB\xBF"

// otherTree is the sections that the other tree reads from the files, one after the other into one result.
func otherTree(files [][]byte) ini.File {
	var read *oldini.File
	for _, data := range files {
		read = oldini.Parse(oldtext.Lossy(data), read)
	}
	sections := ini.File{}
	for name, section := range read.All() {
		sections[name] = maps.Collect(section.All())
	}
	return sections
}

// thisTree is the sections that this tree reads from the files, one after the other into one result.
func thisTree(files [][]byte) ini.File {
	var read ini.File
	for _, data := range files {
		read = ini.Parse(fsx.DecodeText(data), read)
	}
	return read
}

// hasWiderSpace reports whether an input has a character that the other tree takes for white space and this
// tree for text: a no-break space, another space outside ASCII, a line or a paragraph separator, or a byte order
// mark that is not at the start of its file.
func hasWiderSpace(files [][]byte) bool {
	return slices.ContainsFunc(files, func(data []byte) bool {
		return strings.ContainsFunc(strings.TrimPrefix(string(data), mark), func(r rune) bool {
			return (r >= 0x2000 && r <= 0x200A) ||
				slices.Contains([]rune{0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF}, r)
		})
	})
}

// tally is how many inputs both trees were given, by what was done with them.
type tally struct {
	whole    int // inputs outside the class, compared whole through the oracle
	marked   int // of those, the inputs with a file that starts with a byte order mark
	sections int // of those, the sections that the other tree reads
	keys     int // of those, the keys that the other tree reads
	// The inputs of the class, by whether the two trees make the same of them.
	widerAlike, widerApart int
}

// comparison gives inputs to both trees, and counts them.
type comparison struct {
	t *testing.T
	tally
}

// input gives one input to both trees. An input of the class is counted by whether they agree; any other is
// compared whole, and a difference in one file is reported with the line at which the two trees part.
func (c *comparison) input(what string, files ...[]byte) {
	c.t.Helper()
	want, got := otherTree(files), thisTree(files)
	alike := reflect.DeepEqual(want, got)
	switch {
	case hasWiderSpace(files) && alike:
		c.widerAlike++
	case hasWiderSpace(files):
		c.widerApart++
	case alike || len(files) != 1:
		c.whole, c.sections = c.whole+1, c.sections+len(want)
		for _, section := range want {
			c.keys += len(section)
		}
		if slices.ContainsFunc(files, func(data []byte) bool { return bytes.HasPrefix(data, []byte(mark)) }) {
			c.marked++
		}
		oracle.Values(c.t, what, want, got)
	default:
		line, lines := partingLine(files[0])
		what = fmt.Sprintf("%s, read up to line %d, where the trees part", what, line)
		oracle.Values(c.t, what, otherTree(lines), thisTree(lines))
	}
}

// partingLine is the line of a file, counted from 1, at which the two trees part: they make the same of the
// lines before it, and not of the lines up to it. It returns that line and the file up to it, as an input.
func partingLine(data []byte) (int, [][]byte) {
	lines := bytes.SplitAfter(data, []byte("\n"))
	upTo := func(count int) [][]byte { return [][]byte{bytes.Join(lines[:count], nil)} }
	low, high := 0, len(lines)
	for high-low > 1 {
		middle := (low + high) / 2
		if reflect.DeepEqual(otherTree(upTo(middle)), thisTree(upTo(middle))) {
			low = middle
		} else {
			high = middle
		}
	}
	return high, upTo(high)
}

// expect fails the test unless the tally is the one wanted.
func (c *comparison) expect(want tally) {
	c.t.Helper()
	if c.tally != want {
		c.t.Errorf("the inputs were\n%+v, want\n%+v", c.tally, want)
	}
}

// testTexts are the texts of ini_test.go that hold no white space outside ASCII, each with a name for a report.
func testTexts() map[string]string {
	named := map[string]string{"sections": sections, "merged": merged, "merged later": mergedLater}
	for _, c := range lines {
		named[c.name] = c.text
	}
	return named
}

func TestOracleOnTheTextsOfTheTests(t *testing.T) {
	c := &comparison{t: t}
	for name, text := range testTexts() {
		c.input(name, []byte(text))
	}
	c.input("sections after a byte order mark", []byte(mark+sections))
	c.input("two texts into one result", []byte(merged), []byte(mergedLater))
	c.input("two texts into one result, each after a byte order mark", []byte(mark+merged), []byte(mark+mergedLater))
	c.input("three texts into one result", []byte(sections), []byte(mergedLater), []byte(sections))
	for _, wide := range widerSpace {
		c.input(wide.name, []byte(wide.text))
	}
	c.input("a byte order mark at the start of the second text", []byte(merged), []byte(mark+"[b]\nName=B\n"))
	c.input("two byte order marks at the start", []byte(mark+mark+"[b]\nName=B\n"))
	c.expect(tally{whole: 18, marked: 3, sections: 25, keys: 44, widerAlike: 1, widerApart: 3})
}

// asciiSpace is the six characters of white space in ASCII.
const asciiSpace = " \t\n\v\f\r"

// changer makes the changes of one seed and one index, and counts the kinds of white space it puts in.
type changer struct {
	random *rand.Rand
	put    map[byte]int
}

// change is the text after one change: a line cut, a line doubled, a quote dropped, or white space put in. A
// text without a quote has white space put in for the third.
func (c changer) change(text string) string {
	lines := strings.Split(text, "\n")
	line := c.random.IntN(len(lines))
	quotes := offsetsOf(text, `"`)
	switch kind := c.random.IntN(4); {
	case kind == 0:
		return strings.Join(slices.Delete(lines, line, line+1), "\n")
	case kind == 1:
		return strings.Join(slices.Insert(lines, line, lines[line]), "\n")
	case kind == 2 && len(quotes) > 0:
		at := quotes[c.random.IntN(len(quotes))]
		return text[:at] + text[at+1:]
	}
	return c.space(text)
}

// space puts one character of ASCII white space, of any of the six kinds, where the text has white space: in
// the place of a character, before it or after it; or at the start of the text, or at its end.
func (c changer) space(text string) string {
	kind := asciiSpace[c.random.IntN(len(asciiSpace))]
	c.put[kind]++
	places := offsetsOf(text, asciiSpace)
	place := c.random.IntN(len(places) + 2)
	switch {
	case place == len(places):
		return string(kind) + text
	case place == len(places)+1:
		return text + string(kind)
	}
	at := places[place]
	switch c.random.IntN(3) {
	case 0:
		return text[:at] + string(kind) + text[at+1:]
	case 1:
		return text[:at] + string(kind) + text[at:]
	}
	return text[:at+1] + string(kind) + text[at+1:]
}

// offsetsOf is the offsets in text of every byte that is one of the characters.
func offsetsOf(text, characters string) []int {
	var offsets []int
	for i := range len(text) {
		if strings.IndexByte(characters, text[i]) >= 0 {
			offsets = append(offsets, i)
		}
	}
	return offsets
}

// The texts of the tests, each changed in forty ways that are the same on every run: one to three changes of a
// text, and a byte order mark before every third. A change is named by the seed and its index, which make it
// again.
func TestOracleOnSeededChanges(t *testing.T) {
	const seed, changesOfAText = 2026_10_05, 40
	c := &comparison{t: t}
	put := map[byte]int{}
	changed, index := 0, 0
	named := testTexts()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		for range changesOfAText {
			index++
			maker := changer{rand.New(rand.NewPCG(seed, uint64(index))), put}
			text := named[name]
			for range 1 + maker.random.IntN(3) {
				text = maker.change(text)
			}
			if text != named[name] {
				changed++
			}
			if index%3 == 0 {
				text = mark + text
			}
			c.input(fmt.Sprintf("seed %d, change %d, of %s: %q", seed, index, name, text), []byte(text))
			if t.Failed() {
				t.FailNow()
			}
		}
	}
	if len(put) != len(asciiSpace) {
		t.Errorf("white space of %d kinds was put in, want all %d", len(put), len(asciiSpace))
	}
	if changed != 500 {
		t.Errorf("%d of %d inputs differ from their text, want 500", changed, index)
	}
	c.expect(tally{whole: 520, marked: 173, sections: 454, keys: 736})
}

// exportPath is a folder or a file of the game's files: the folder the environment variable names, and then each
// name below it, found without regard to letter case. Without the variable the test is skipped, or fails when
// MOONWELL_REQUIRE_EXPORTS is 1.
func exportPath(t *testing.T, variable string, below ...string) string {
	t.Helper()
	folder := os.Getenv(variable)
	if folder == "" && os.Getenv("MOONWELL_REQUIRE_EXPORTS") == "1" {
		t.Fatalf("%s is not set, and MOONWELL_REQUIRE_EXPORTS=1 requires the game's files", variable)
	}
	if folder == "" {
		t.Skipf("%s is not set: the game's files are not compared", variable)
	}
	for _, name := range below {
		entries := exportEntries(t, folder, func(entry string) bool { return entry == name })
		if len(entries) != 1 {
			t.Fatalf("%s has %d entries named %s, want 1", folder, len(entries), name)
		}
		folder = filepath.Join(folder, entries[0])
	}
	return folder
}

// exportEntries is the names of the entries of a folder whose name in small letters is wanted, sorted.
func exportEntries(t *testing.T, folder string, wanted func(lowerName string) bool) []string {
	t.Helper()
	entries, err := os.ReadDir(folder)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, entry := range entries {
		if wanted(strings.ToLower(entry.Name())) {
			names = append(names, entry.Name())
		}
	}
	return names
}

// The strings files of the game, read from the maintainer's export: this test reads the game's files, and takes
// longer than the others. A report names a file and a line, and shows no more of the file than one value.
func TestOracleOnTheStringsOfTheGame(t *testing.T) {
	const variable, game, locales, locale = "MOONWELL_GAME_DATA", "war3.w3mod", "_locales", "enus.w3mod"
	units := exportPath(t, variable, game, locales, locale, "units")
	editor := exportPath(t, variable, game, locales, locale, "ui", "worldeditstrings.txt")
	c := &comparison{t: t}
	read := func(file string) []byte {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	var all [][]byte
	isStrings := func(entry string) bool { return strings.HasSuffix(entry, "strings.txt") }
	for _, name := range exportEntries(t, units, isStrings) {
		all = append(all, read(filepath.Join(units, name)))
		c.input("units/"+name, all[len(all)-1])
	}
	c.input("the strings files of units, read into one result", all...)
	c.input("ui/"+filepath.Base(editor), read(editor))
	// The files of the game version 3.0.0.24268, which is the one the committed data is made from: thirty
	// strings files, seven of which start with a byte order mark, each alone and all into one result, and the
	// editor's strings.
	c.expect(tally{whole: 32, marked: 8, sections: 7501, keys: 37749})
}
