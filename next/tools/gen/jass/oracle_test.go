package jass_test

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
	"github.com/mdlsvensson/moonwell/next/tools/gen/jass"
	oldjass "github.com/mdlsvensson/moonwell/tools/gen/jass"
)

// What this file compares. An input is the bytes of a script. The other tree's parser is given them as its tree
// decodes a file (text.Lossy, which keeps a byte order mark; that parser trims one from the first line as white
// space), and this tree's as its own does (fsx.DecodeText, which drops a mark at the start). What each makes of
// them is compared whole, through the oracle: every type, every function with its parameters, every global, and
// a refusal word for word, the quoted line in it too.
//
// The inputs are every script of jass_test.go, two of them with a mark at their start; seeded changes of those
// scripts, one to three of a text (a line cut, a line doubled, a quote dropped, a character of ASCII white space
// put in at one place that is drawn), every third with a mark at its start; every placing of one character of
// ASCII white space in those scripts, each of the six kinds at each edge of each line and at each white-space
// character of a line; and, on a machine that has the game's files, common.j and blizzard.j of the export.
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

// alike reports whether the two trees make the same of an input.
func alike(data []byte) bool { return reflect.DeepEqual(otherTree(data), thisTree(data)) }

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
	same := reflect.DeepEqual(want, got)
	switch {
	case hasWiderSpace(data) && same:
		c.widerAlike++
	case hasWiderSpace(data):
		c.widerApart++
	case same:
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
		line, upTo := oracle.PartingLine(data, alike)
		what = fmt.Sprintf("%s, read up to line %d, where the trees part", what, line)
		oracle.Values(c.t, what, otherTree(upTo), thisTree(upTo))
	}
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
		"common": common, "blizzard": blizzard, "corners": corners, "indented": indented,
		"no declaration": noDeclaration,
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
		whole: 22, marked: 2, refused: 16, types: 7, functions: 13, globals: 11, widerAlike: 1, widerApart: 4,
	})
}

// The scripts of the tests, each changed in forty ways that are the same on every run, and a byte order mark
// before every third. A change is named by the seed and its index, which make it again.
func TestOracleOnSeededChanges(t *testing.T) {
	const seed, changesOfAScript = 2026_10_05, 40
	c := &comparison{t: t}
	changed, index := 0, uint64(0)
	named := testScripts()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		for range changesOfAScript {
			index++
			text := oracle.Changed(named[name], seed, index)
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
	if changed != 781 {
		t.Errorf("%d of %d inputs differ from their script, want 781", changed, index)
	}
	c.expect(tally{whole: 800, marked: 266, refused: 581, types: 113, functions: 252, globals: 214})
}

// The scripts of the tests with one character of ASCII white space put in, at every place and of every kind.
func TestOracleOnEveryPlacingOfWhiteSpace(t *testing.T) {
	c := &comparison{t: t}
	named := testScripts()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		for i, text := range oracle.Swept(named[name]) {
			c.input(fmt.Sprintf("%s with white space put in, text %d: %q", name, i, text), []byte(text))
			if t.Failed() {
				t.FailNow()
			}
		}
	}
	c.expect(tally{whole: 5034, refused: 1949, types: 4206, functions: 8092, globals: 7119})
}

// The two scripts of the game, read from the maintainer's export: this test reads the game's files, and takes
// longer than the others. A report names a file and a line, and shows no more of the file than one value.
func TestOracleOnTheScriptsOfTheGame(t *testing.T) {
	export := testkit.NeedExport(t, "MOONWELL_GAME_SCRIPTS")
	c := &comparison{t: t}
	for _, name := range []string{"common.j", "blizzard.j"} {
		data, err := os.ReadFile(export.Path("war3.w3mod", "scripts", name))
		if err != nil {
			t.Fatal(err)
		}
		c.input(name, data)
	}
	// The scripts of the game version 3.0.0.24268, which is the one the committed data is made from.
	c.expect(tally{whole: 2, types: 139, functions: 2737, globals: 2260})
}
