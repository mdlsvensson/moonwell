package jass_test

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
	"github.com/mdlsvensson/moonwell/next/tools/gen/jass"
	oldjass "github.com/mdlsvensson/moonwell/tools/gen/jass"
)

// What this file compares. An input is the bytes of a script. The other tree's parser is given them as its tree
// decodes a file (text.Lossy, which keeps a byte order mark; that parser trims one from the first line as white
// space), and this tree's as its own does (fsx.DecodeText, which drops a mark at the start). What each makes of
// them is compared whole, through the oracle: every type, every function with its parameters, every global, and
// a refusal word for word, the quoted line in it too.
//
// The inputs are every script of jass_test.go, one of them with a mark at its start; seeded changes of those
// scripts (a line cut, a line doubled, a quote dropped, ASCII white space put where the text has some, and at
// its start and its end), every third with a mark at its start; and, on a machine that has the game's files,
// common.j and blizzard.j of the export.
//
// One class of inputs is counted apart, decided by its bytes alone: a script with a character that is white
// space outside ASCII (hasWiderSpace). The other tree trims such a character from a line, takes it for the white
// space between two words, and takes a line or a paragraph separator for the end of what a parameter list or an
// initial value may hold; this tree takes each for text, and writes the two separators as escapes in the line a
// refusal quotes (TestWhiteSpaceOutsideASCIIIsText). Both trees are still given every input of the class, and
// the tally says on how many they agree: they do where the character stands in a comment or inside a string.
//
// Not among the inputs: bytes that are not UTF-8. The two decoders write a run of them differently, which is no
// difference of the parsers.

const mark = "\xEF\xBB\xBF"

// read is what a parser makes of a script, in the shape the two trees are compared in.
type read struct {
	File    jass.File
	Refusal string // what the error says; "" for a script that is read
}

// otherTree is what the other tree makes of the bytes of a script, in the types of this tree. A list that is nil
// stays nil.
func otherTree(data []byte) read {
	file, err := oldjass.Parse(oldtext.Lossy(data), "script.j")
	made := read{File: jass.File{
		Types:   each(file.Types, func(t oldjass.Type) jass.Type { return jass.Type(t) }),
		Globals: each(file.Globals, func(g oldjass.Global) jass.Global { return jass.Global(g) }),
		Functions: each(file.Functions, func(f oldjass.Function) jass.Function {
			params := each(f.Params, func(p oldjass.Param) jass.Param { return jass.Param(p) })
			return jass.Function{f.Name, f.Source, f.Constant, params, f.Returns}
		}),
	}}
	if err != nil {
		made.Refusal = err.Error()
	}
	return made
}

// each is the list with every item changed; nil for a list that is nil.
func each[From, To any](list []From, change func(From) To) []To {
	if list == nil {
		return nil
	}
	changed := make([]To, 0, len(list))
	for _, item := range list {
		changed = append(changed, change(item))
	}
	return changed
}

// thisTree is what this tree makes of the bytes of a script.
func thisTree(data []byte) read {
	file, err := jass.Parse(fsx.DecodeText(data), "script.j")
	made := read{File: file}
	if err != nil {
		made.Refusal = err.Error()
	}
	return made
}

// hasWiderSpace reports whether the input has a character that the other tree takes for white space and this
// tree for text: a no-break space, another space outside ASCII, a line or a paragraph separator, or a byte order
// mark that is not at the start.
func hasWiderSpace(data []byte) bool {
	return strings.ContainsFunc(strings.TrimPrefix(string(data), mark), func(r rune) bool {
		return (r >= 0x2000 && r <= 0x200A) ||
			slices.Contains([]rune{0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF}, r)
	})
}

// tally is how many inputs both trees were given, by what was done with them.
type tally struct {
	whole   int // inputs outside the class, compared whole through the oracle
	marked  int // of those, the inputs that start with a byte order mark
	refused int // of those, the inputs that the other tree refuses
	// Of those, what the other tree reads.
	types, functions, globals int
	// The inputs of the class, by whether the two trees make the same of them.
	widerAlike, widerApart int
}

// comparison gives inputs to both trees, and counts them.
type comparison struct {
	t *testing.T
	tally
}

// input gives one input to both trees. An input of the class is counted by whether they agree; any other is
// compared whole, and a difference is reported with the line of the input at which the two trees part.
func (c *comparison) input(what string, data []byte) {
	c.t.Helper()
	want, got := otherTree(data), thisTree(data)
	alike := reflect.DeepEqual(want, got)
	switch {
	case hasWiderSpace(data) && alike:
		c.widerAlike++
	case hasWiderSpace(data):
		c.widerApart++
	case alike:
		c.whole++
		c.types, c.functions = c.types+len(want.File.Types), c.functions+len(want.File.Functions)
		c.globals += len(want.File.Globals)
		if want.Refusal != "" {
			c.refused++
		}
		if bytes.HasPrefix(data, []byte(mark)) {
			c.marked++
		}
		oracle.Values(c.t, what, want, got)
	default:
		line, lines := partingLine(data)
		what = fmt.Sprintf("%s, read up to line %d, where the trees part", what, line)
		oracle.Values(c.t, what, otherTree(lines), thisTree(lines))
	}
}

// partingLine is the line of an input, counted from 1, at which the two trees part: they make the same of the
// lines before it, and not of the lines up to it. It returns that line and the input up to it.
func partingLine(data []byte) (int, []byte) {
	lines := bytes.SplitAfter(data, []byte("\n"))
	upTo := func(count int) []byte { return bytes.Join(lines[:count], nil) }
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

// testScripts are the scripts of jass_test.go that hold no white space outside ASCII, each with a name for a
// report.
func testScripts() map[string]string {
	named := map[string]string{
		"common": common, "blizzard": blizzard, "corners": corners, "no declaration": noDeclaration,
	}
	for i, c := range refused {
		named[fmt.Sprintf("refused script %02d, of %s", i, c.source)] = c.text
	}
	return named
}

func TestOracleOnTheScriptsOfTheTests(t *testing.T) {
	c := &comparison{t: t}
	for name, text := range testScripts() {
		c.input(name, []byte(text))
	}
	c.input("common after a byte order mark", []byte(mark+common))
	c.input("a refused line after a byte order mark", []byte(mark+" library Foo\n"))
	for _, wide := range widerSpace {
		c.input(wide.name, []byte(wide.text))
	}
	c.expect(tally{
		whole: 20, marked: 2, refused: 15, types: 7, functions: 11, globals: 10, widerAlike: 1, widerApart: 4,
	})
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

// The scripts of the tests, each changed in forty ways that are the same on every run: one to three changes of
// a text, and a byte order mark before every third. A change is named by the seed and its index, which make it
// again.
func TestOracleOnSeededChanges(t *testing.T) {
	const seed, changesOfAScript = 2026_10_05, 40
	c := &comparison{t: t}
	put := map[byte]int{}
	changed, index := 0, 0
	named := testScripts()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		for range changesOfAScript {
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
	if changed != 703 {
		t.Errorf("%d of %d inputs differ from their script, want 703", changed, index)
	}
	c.expect(tally{whole: 720, marked: 240, refused: 514, types: 116, functions: 206, globals: 186})
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

// The two scripts of the game, read from the maintainer's export: this test reads the game's files, and takes
// longer than the others. A report names a file and a line, and shows no more of the file than one value.
func TestOracleOnTheScriptsOfTheGame(t *testing.T) {
	c := &comparison{t: t}
	for _, name := range []string{"common.j", "blizzard.j"} {
		data, err := os.ReadFile(exportPath(t, "MOONWELL_GAME_SCRIPTS", "war3.w3mod", "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		c.input(name, data)
	}
	// The scripts of the game version 3.0.0.24268, which is the one the committed data is made from.
	c.expect(tally{whole: 2, types: 139, functions: 2737, globals: 2260})
}
