package slk_test

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
	"github.com/mdlsvensson/moonwell/next/tools/gen/slk"
	oldslk "github.com/mdlsvensson/moonwell/tools/gen/slk"
)

// What this file compares. An input is the bytes of a table. The other tree's parser is given them as its tree
// decodes a file (text.Lossy, which keeps a byte order mark), and this tree's as its own does (fsx.DecodeText,
// which drops a mark at the start). What each makes of them is compared whole, through the oracle: the names of
// the columns, every row with its first cell and its cells, and a refusal word for word.
//
// The inputs are every table of slk_test.go but the ones named below, one of them with a mark at its start;
// seeded changes of those tables (a line cut, a line doubled, a quote dropped, ASCII white space put where the
// text has some, and at its start and its end); and, on a machine that has the game's files, every .slk file
// directly in war3.w3mod/units of the export.
//
// Two classes of inputs are counted apart, each decided by its bytes alone. Both trees are still given every
// input of a class, and the tally says on how many they agree:
//
//   - A table with a character that is white space outside ASCII (hasWiderSpace). A table has no white space of
//     its own, so the two trees make the same of these too (TestWhiteSpaceOutsideASCIIIsPartOfAValue).
//   - A table that starts with a byte order mark with a C record or the E record right after it
//     (hasAMarkBeforeARecord). The other tree's decoder keeps the mark and its parser takes the line for no
//     record; this tree's decoder drops the mark, and the parser reads the record like the first of any other
//     text. No table of the game starts with a mark.
//
// Not among the inputs: a C record with an empty field, which is two semicolons in a row outside a quoted value.
// The other tree's parser cannot be given one: it reads past the field and panics. This tree refuses the record
// (TestParseRefusesARecordWithAnEmptyField). No seeded change makes one, and no table of the game has one. Nor
// has a table of the tests bytes that are not UTF-8: the two decoders write a run of them differently, which is
// no difference of the parsers.

const mark = "\xEF\xBB\xBF"

// read is what a parser makes of a table, in the shape the two trees are compared in.
type read struct {
	Columns []string
	Rows    []readRow
	Refusal string // what the error says; "" for a table that is read
}

type readRow struct {
	First string
	Cells map[string]string
}

// otherTree is what the other tree makes of the bytes of a table. It stops the test when that parser panics.
func otherTree(t *testing.T, what string, data []byte) (made read) {
	t.Helper()
	defer func() {
		if failure := recover(); failure != nil {
			t.Fatalf("%s: the other tree's parser panics: %v", what, failure)
		}
	}()
	table, err := oldslk.Parse(oldtext.Lossy(data), "table.slk")
	if err != nil {
		made.Refusal = err.Error()
	}
	made.Columns = table.Columns
	for _, row := range table.Rows {
		made.Rows = append(made.Rows, readRow{row.First(), row.Cells()})
	}
	return made
}

// thisTree is what this tree makes of the bytes of a table. A row gives its cells by the name of a column, so
// they are asked for by every column the table has; a cell that Get and Value give differently is left out, and
// so shows as a difference.
func thisTree(data []byte) (made read) {
	table, err := slk.Parse(fsx.DecodeText(data), "table.slk")
	if err != nil {
		made.Refusal = err.Error()
	}
	made.Columns = table.Columns
	for _, row := range table.Rows {
		cells := map[string]string{}
		for _, column := range table.Columns {
			if cell, has := row.Get(column); has && cell == row.Value(column) {
				cells[column] = cell
			}
		}
		made.Rows = append(made.Rows, readRow{row.First(), cells})
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

// hasAMarkBeforeARecord reports whether the input starts with a byte order mark and its first line is a C record
// or the E record.
func hasAMarkBeforeARecord(data []byte) bool {
	text, marked := strings.CutPrefix(string(data), mark)
	line, _, _ := strings.Cut(strings.ReplaceAll(text, "\r\n", "\n"), "\n")
	return marked && (line == "E" || strings.HasPrefix(line, "E;") || strings.HasPrefix(line, "C;"))
}

// tally is how many inputs both trees were given, by what was done with them.
type tally struct {
	whole   int // inputs outside the classes, compared whole through the oracle
	marked  int // of those, the inputs that start with a byte order mark
	refused int // of those, the inputs that the other tree refuses
	rows    int // of those, the rows that the other tree reads
	// The inputs of each class, by whether the two trees make the same of them.
	widerAlike, widerApart   int
	markedAlike, markedApart int
}

// comparison gives inputs to both trees, and counts them.
type comparison struct {
	t *testing.T
	tally
}

// input gives one input to both trees. An input of a class is counted by whether they agree; any other is
// compared whole, and a difference is reported with the line of the input at which the two trees part.
func (c *comparison) input(what string, data []byte) {
	c.t.Helper()
	want, got := otherTree(c.t, what, data), thisTree(data)
	alike := reflect.DeepEqual(want, got)
	switch {
	case hasWiderSpace(data):
		count(alike, &c.widerAlike, &c.widerApart)
	case hasAMarkBeforeARecord(data):
		count(alike, &c.markedAlike, &c.markedApart)
	case alike:
		c.whole, c.rows = c.whole+1, c.rows+len(want.Rows)
		if want.Refusal != "" {
			c.refused++
		}
		if bytes.HasPrefix(data, []byte(mark)) {
			c.marked++
		}
		oracle.Values(c.t, what, want, got)
	default:
		line, lines := partingLine(c.t, what, data)
		what = fmt.Sprintf("%s, read up to line %d, where the trees part", what, line)
		oracle.Values(c.t, what, otherTree(c.t, what, lines), thisTree(lines))
	}
}

func count(alike bool, ifAlike, ifApart *int) {
	if alike {
		*ifAlike++
	} else {
		*ifApart++
	}
}

// partingLine is the line of an input, counted from 1, at which the two trees part: they make the same of the
// lines before it, and not of the lines up to it. It returns that line and the input up to it.
func partingLine(t *testing.T, what string, data []byte) (int, []byte) {
	t.Helper()
	lines := bytes.SplitAfter(data, []byte("\n"))
	upTo := func(count int) []byte { return bytes.Join(lines[:count], nil) }
	low, high := 0, len(lines)
	for high-low > 1 {
		middle := (low + high) / 2
		if reflect.DeepEqual(otherTree(t, what, upTo(middle)), thisTree(upTo(middle))) {
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

// testTables are the tables of slk_test.go that both trees can be given, each with a name for a report.
func testTables() map[string]string {
	named := map[string]string{"mini": mini}
	for _, c := range tables {
		named[c.name] = c.text
	}
	for i, text := range emptyTables {
		named[fmt.Sprintf("empty table %d", i)] = text
	}
	for _, c := range malformed {
		named["malformed "+c.file] = c.text
	}
	return named
}

func TestOracleOnTheTablesOfTheTests(t *testing.T) {
	c := &comparison{t: t}
	for name, text := range testTables() {
		c.input(name, []byte(text))
	}
	c.input("mini after a byte order mark", []byte(mark+mini))
	for _, wide := range widerSpace {
		c.input(wide.name, []byte(wide.text))
	}
	// After a mark: a record that names a column, the end of a table that has a cell after it, and a record
	// without a value, which the two trees make the same of.
	for _, text := range []string{doubledQuotes, "E\nC;X1;Y1;Ka\n", "C;X1;Y1\nC;X1;Y1;Ka\nC;X1;Y2;Kb\n"} {
		c.input(fmt.Sprintf("a byte order mark and then %q", text), []byte(mark+text))
	}
	c.expect(tally{whole: 21, marked: 1, refused: 7, rows: 16, widerAlike: 2, markedAlike: 1, markedApart: 2})
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

// The tables of the tests, each changed in forty ways that are the same on every run: one to three changes of a
// text. A change is named by the seed and its index, which make it again.
func TestOracleOnSeededChanges(t *testing.T) {
	const seed, changesOfATable = 2026_10_05, 40
	c := &comparison{t: t}
	put := map[byte]int{}
	changed, index := 0, 0
	named := testTables()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		for range changesOfATable {
			index++
			maker := changer{rand.New(rand.NewPCG(seed, uint64(index))), put}
			text := named[name]
			for range 1 + maker.random.IntN(3) {
				text = maker.change(text)
			}
			if text != named[name] {
				changed++
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
	if changed != 779 {
		t.Errorf("%d of %d inputs differ from their table, want 779", changed, index)
	}
	c.expect(tally{whole: 800, refused: 274, rows: 310})
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

// The tables of the game, read from the maintainer's export: this test reads the game's files, and takes longer
// than the others. A report names a file and a line, and shows no more of the file than one value.
func TestOracleOnTheTablesOfTheGame(t *testing.T) {
	folder := exportPath(t, "MOONWELL_GAME_DATA", "war3.w3mod", "units")
	c := &comparison{t: t}
	isTable := func(entry string) bool { return strings.HasSuffix(entry, ".slk") }
	for _, name := range exportEntries(t, folder, isTable) {
		data, err := os.ReadFile(filepath.Join(folder, name))
		if err != nil {
			t.Fatal(err)
		}
		c.input(name, data)
	}
	// The tables of the game version 3.0.0.24268, which is the one the committed data is made from.
	c.expect(tally{whole: 17, rows: 9247})
}
