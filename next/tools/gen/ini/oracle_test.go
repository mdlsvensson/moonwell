package ini_test

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	oldtext "github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/tools/gen/ini"
	oldini "github.com/mdlsvensson/moonwell/tools/gen/ini"
)

// What this file compares. An input is the bytes of one file, or of several that are read into one result. The
// other tree's parser is given each as its tree decodes a file (text.Lossy, which keeps a byte order mark; that
// parser drops one at the start itself), and this tree's as its own does (fsx.DecodeText, which drops a mark at
// the start). The other tree reads the first file into a new result and each later one into that result; this
// tree reads the first with Parse and each later one with Add. What each makes of them is compared whole, through
// the oracle: every section, every key and every value. The other tree keeps the order in which sections and
// keys came and this tree keeps none, so what the two hold is compared, and not its order.
//
// The inputs are every text of ini_test.go, some of them with a mark at their start; seeded changes of those
// texts, one to three of a text (a line cut, a line doubled, a quote dropped, a character of ASCII white space
// put in at one place that is drawn), every third with a mark at its start; every placing of one character of
// ASCII white space in those texts, each of the six kinds at each edge of each line and at each white-space
// character of a line; and, on a machine that has the game's files, every file directly in the locale's units
// folder whose name ends with strings.txt, each alone and all of them read into one result, and
// ui/worldeditstrings.txt.
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

// file is the bytes of one file of an input, with its name for a report.
type file struct {
	name string
	data []byte
}

// otherTree is the sections that the other tree reads from the files, the first into a new result and each
// later one into that result.
func otherTree(files []file) ini.File {
	var read *oldini.File
	for _, f := range files {
		read = oldini.Parse(oldtext.Lossy(f.data), read)
	}
	sections := ini.File{}
	for name, section := range read.All() {
		sections[name] = maps.Collect(section.All())
	}
	return sections
}

// thisTree is the sections that this tree reads from the files: the first is parsed, and each later one added.
func thisTree(files []file) ini.File {
	read := ini.Parse(fsx.DecodeText(files[0].data))
	for _, f := range files[1:] {
		read.Add(fsx.DecodeText(f.data))
	}
	return read
}

// alike reports whether the two trees read the same from the files.
func alike(files []file) bool { return reflect.DeepEqual(otherTree(files), thisTree(files)) }

// hasWiderSpace reports whether an input has a character that the other tree takes for white space and this
// tree for text: a no-break space, another space outside ASCII, a line or a paragraph separator, or a byte order
// mark that is not at the start of its file.
func hasWiderSpace(files []file) bool {
	return slices.ContainsFunc(files, func(f file) bool {
		return strings.ContainsFunc(strings.TrimPrefix(string(f.data), mark), func(r rune) bool {
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

// text gives both trees an input of one file, which is named as the input is.
func (c *comparison) text(what string, data []byte) {
	c.t.Helper()
	c.input(what, file{what, data})
}

// input gives one input to both trees. An input of the class is counted by whether they agree; any other is
// compared whole.
func (c *comparison) input(what string, files ...file) {
	c.t.Helper()
	want, got := otherTree(files), thisTree(files)
	same := reflect.DeepEqual(want, got)
	switch {
	case hasWiderSpace(files) && same:
		c.widerAlike++
	case hasWiderSpace(files):
		c.widerApart++
	case !same:
		c.parted(what, files)
	default:
		c.whole, c.sections = c.whole+1, c.sections+len(want)
		for _, section := range want {
			c.keys += len(section)
		}
		if slices.ContainsFunc(files, func(f file) bool { return bytes.HasPrefix(f.data, []byte(mark)) }) {
			c.marked++
		}
		oracle.Values(c.t, what, want, got)
	}
}

// parted reports an input outside the class that the two trees do not make the same of. It names the file that
// first parts them when it is read after the ones before it, and the line of that file at which they part, and
// compares what each tree makes of the input up to that line.
func (c *comparison) parted(what string, files []file) {
	c.t.Helper()
	parting := 0
	for parting+1 < len(files) && alike(files[:parting+1]) {
		parting++
	}
	before, last := files[:parting], files[parting]
	upTo := func(data []byte) []file { return append(slices.Clone(before), file{last.name, data}) }
	line, read := oracle.PartingLine(last.data, func(data []byte) bool { return alike(upTo(data)) })
	if len(files) > 1 {
		what += ", when " + last.name + " is read"
	}
	what = fmt.Sprintf("%s, read up to line %d, where the trees part", what, line)
	oracle.Values(c.t, what, otherTree(upTo(read)), thisTree(upTo(read)))
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
		c.text(name, []byte(text))
	}
	c.text("sections after a byte order mark", []byte(mark+sections))
	first, later := file{"merged", []byte(merged)}, file{"merged later", []byte(mergedLater)}
	c.input("two texts into one result", first, later)
	c.input("two texts into one result, each after a byte order mark",
		file{"merged", []byte(mark + merged)}, file{"merged later", []byte(mark + mergedLater)})
	whole := file{"sections", []byte(sections)}
	c.input("three texts into one result", whole, later, whole)
	c.input("keys without a section after a text with sections", first, file{"keys", []byte("Name=Three\nTip=U\n")})
	for _, wide := range widerSpace {
		c.text(wide.name, []byte(wide.text))
	}
	c.input("a byte order mark at the start of the second text", first, file{"marked", []byte(mark + "[b]\nName=B\n")})
	c.text("two byte order marks at the start", []byte(mark+mark+"[b]\nName=B\n"))
	c.expect(tally{whole: 19, marked: 3, sections: 26, keys: 46, widerAlike: 1, widerApart: 3})
}

// The texts of the tests, each changed in forty ways that are the same on every run, and a byte order mark
// before every third. A change is named by the seed and its index, which make it again.
func TestOracleOnSeededChanges(t *testing.T) {
	const seed, changesOfAText = 2026_10_05, 40
	c := &comparison{t: t}
	changed, index := 0, uint64(0)
	named := testTexts()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		for range changesOfAText {
			index++
			text := oracle.Changed(named[name], seed, index)
			if text != named[name] {
				changed++
			}
			if index%3 == 0 {
				text = mark + text
			}
			c.text(fmt.Sprintf("seed %d, change %d, of %s: %q", seed, index, name, text), []byte(text))
			if t.Failed() {
				t.FailNow()
			}
		}
	}
	if changed != 500 {
		t.Errorf("%d of %d inputs differ from their text, want 500", changed, index)
	}
	c.expect(tally{whole: 520, marked: 173, sections: 454, keys: 736})
}

// The texts of the tests with one character of ASCII white space put in, at every place and of every kind.
func TestOracleOnEveryPlacingOfWhiteSpace(t *testing.T) {
	c := &comparison{t: t}
	named := testTexts()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		for i, text := range oracle.Swept(named[name]) {
			c.text(fmt.Sprintf("%s with white space put in, text %d: %q", name, i, text), []byte(text))
			if t.Failed() {
				t.FailNow()
			}
		}
	}
	c.expect(tally{whole: 1270, sections: 1761, keys: 4306})
}

// The strings files of the game, read from the maintainer's export: this test reads the game's files, and takes
// longer than the others. A report names a file and a line, and shows no more of the file than one value.
func TestOracleOnTheStringsOfTheGame(t *testing.T) {
	export := testkit.NeedExport(t, "MOONWELL_GAME_DATA")
	locale := []string{"war3.w3mod", "_locales", "enus.w3mod"}
	read := func(below ...string) []byte {
		data, err := os.ReadFile(export.Path(append(slices.Clone(locale), below...)...))
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	c := &comparison{t: t}
	var all []file
	for _, name := range export.Entries(append(slices.Clone(locale), "units")...) {
		if strings.HasSuffix(strings.ToLower(name), "strings.txt") {
			all = append(all, file{"units/" + name, read("units", name)})
			c.input(all[len(all)-1].name, all[len(all)-1])
		}
	}
	c.input("the strings files of units, read into one result", all...)
	c.text("ui/worldeditstrings.txt", read("ui", "worldeditstrings.txt"))
	// The files of the game version 3.0.0.24268, which is the one the committed data is made from: thirty
	// strings files, seven of which start with a byte order mark, each alone and all into one result, and the
	// editor's strings.
	c.expect(tally{whole: 32, marked: 8, sections: 7501, keys: 37749})
}
