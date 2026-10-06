package slk_test

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
	"github.com/mdlsvensson/moonwell/next/tools/gen/slk"
	oldslk "github.com/mdlsvensson/moonwell/tools/gen/slk"
)

// What this file compares. An input is the bytes of a table. The other tree's parser is given them as its tree
// decodes a file (text.Lossy, which keeps a byte order mark), and this tree's as its own does (fsx.DecodeText,
// which drops a mark at the start). What each makes of them is compared whole, through the oracle: the names of
// the columns, every row with its first cell and its cells, and a refusal word for word.
//
// The inputs are every table of slk_test.go, one of them with a mark at its start too; seeded changes of those
// tables, one to three of a text (a line cut, a line doubled, a quote dropped, a character of ASCII white space
// put in at one place that is drawn); every placing of one character of ASCII white space in those tables, each
// of the six kinds at each edge of each line and at each white-space character of a line; and, on a machine that
// has the game's files, every .slk file directly in war3.w3mod/units of the export.
//
// Three classes of inputs are counted apart, each decided by its bytes alone. Both trees are still given every
// input of a class, and the tally says what they make of it:
//
//   - A table with a character that is white space outside ASCII (hasWiderSpace). A table has no white space of
//     its own, so the two trees make the same of these too (TestWhiteSpaceOutsideASCIIIsPartOfAValue).
//   - A table that starts with a byte order mark with a C record or the E record right after it
//     (hasAMarkBeforeARecord). The other tree's decoder keeps the mark and its parser takes the line for no
//     record; this tree's decoder drops the mark, and the parser reads the record like the first of any other
//     text. No table of the game starts with a mark.
//   - A table with a C record that has two semicolons in a row (hasTwoSemicolons). Where they are a field with
//     nothing in it, the other tree's parser reads past the field and panics, and this tree refuses the record
//     (TestParseRefusesARecordWithAnEmptyField): the panic is seen here, and the refusal is required beside it.
//     Where they are none, inside a quoted value or behind what stops the reading first, the two trees are
//     compared whole. The other tree's parser panics on no input outside this class, or the test fails. No
//     seeded change and no placing of white space makes such a record, and no table of the game has one.
//
// Not among the inputs: bytes that are not UTF-8 in a table of the tests. The two decoders write a run of them
// differently, which is no difference of the parsers.

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

// otherTree is what the other tree makes of the bytes of a table, and whether its parser panics on them.
func otherTree(data []byte) (made read, panics bool) {
	defer func() {
		if recover() != nil {
			made, panics = read{}, true
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
	return made, false
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

// alike reports whether the two trees make the same of an input, the other tree's parser not panicking.
func alike(data []byte) bool {
	want, panics := otherTree(data)
	return !panics && reflect.DeepEqual(want, thisTree(data))
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

// hasTwoSemicolons reports whether a line of the input starts as a C record and has two semicolons in a row:
// every record with an empty field is such a line.
func hasTwoSemicolons(data []byte) bool {
	return slices.ContainsFunc(strings.Split(string(data), "\n"), func(line string) bool {
		return strings.HasPrefix(line, "C;") && strings.Contains(line, ";;")
	})
}

// tally is how many inputs both trees were given, by what was done with them.
type tally struct {
	whole   int // inputs outside the classes, compared whole through the oracle
	marked  int // of those, the inputs that start with a byte order mark
	refused int // of those, the inputs that the other tree refuses
	rows    int // of those, the rows that the other tree reads
	// The inputs with white space outside ASCII, and those with a mark before a record, by whether the two trees
	// make the same of them.
	widerAlike, widerApart   int
	markedAlike, markedApart int
	// The inputs with two semicolons in a row in a record: those that the other tree's parser panics on and
	// this tree refuses for an empty field, and those that are compared whole.
	emptyRefused, emptyWhole int
}

// comparison gives inputs to both trees, and counts them.
type comparison struct {
	t *testing.T
	tally
}

// input gives one input to both trees and counts it by its class. An input of no class is compared whole.
func (c *comparison) input(what string, data []byte) {
	c.t.Helper()
	want, panics := otherTree(data)
	got := thisTree(data)
	same := !panics && reflect.DeepEqual(want, got)
	switch {
	case panics:
		c.emptyField(what, data, got)
	case hasWiderSpace(data):
		count(same, &c.widerAlike, &c.widerApart)
	case hasAMarkBeforeARecord(data):
		count(same, &c.markedAlike, &c.markedApart)
	case !same:
		c.parted(what, data)
	case hasTwoSemicolons(data):
		c.emptyWhole++
		oracle.Values(c.t, what, want, got)
	default:
		c.whole, c.rows = c.whole+1, c.rows+len(want.Rows)
		if want.Refusal != "" {
			c.refused++
		}
		if bytes.HasPrefix(data, []byte(mark)) {
			c.marked++
		}
		oracle.Values(c.t, what, want, got)
	}
}

// emptyField counts an input that the other tree's parser panics on. The input must be of the class, and this
// tree must refuse it for an empty field.
func (c *comparison) emptyField(what string, data []byte, got read) {
	c.t.Helper()
	switch {
	case !hasTwoSemicolons(data):
		c.t.Errorf("%s: the other tree's parser panics, and no record has two semicolons in a row", what)
	case !strings.Contains(got.Refusal, "empty field"):
		c.t.Errorf("%s: the other tree's parser panics on an empty field, and this tree says %q", what, got.Refusal)
	default:
		c.emptyRefused++
	}
}

// parted reports an input of no class that the two trees do not make the same of: with the line at which they
// part, and what each makes of the input up to that line.
func (c *comparison) parted(what string, data []byte) {
	c.t.Helper()
	line, upTo := oracle.PartingLine(data, alike)
	want, _ := otherTree(upTo)
	oracle.Values(c.t, fmt.Sprintf("%s, read up to line %d, where the trees part", what, line), want, thisTree(upTo))
}

// count adds one to the first number when the two trees make the same of an input, and else to the second.
func count(same bool, alike, apart *int) {
	if same {
		*alike++
	} else {
		*apart++
	}
}

// expect fails the test unless the tally is the one wanted.
func (c *comparison) expect(want tally) {
	c.t.Helper()
	if c.tally != want {
		c.t.Errorf("the inputs were\n%+v, want\n%+v", c.tally, want)
	}
}

// testTables are the tables of slk_test.go of no class, each with a name for a report.
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

// TestOracleOnTheRecordedRefusals holds testdata/recorded/refusals.txt to what the other tree says of every table
// the recording names. It is the test that writes the recording: MOONWELL_RECORD=1 with -run of this test alone.
// Of a table with two semicolons in a row (hasTwoSemicolons, the third class of the note above) the recording
// holds this tree's words: the other tree's parser reads past the empty field and panics.
func TestOracleOnTheRecordedRefusals(t *testing.T) {
	var said []testkit.Refusal
	for _, table := range refusedTables() {
		err := func() (err error) {
			defer func() {
				if recover() != nil || hasTwoSemicolons([]byte(table.text)) {
					_, err = slk.Parse(table.text, table.file)
				}
			}()
			_, err = oldslk.Parse(table.text, table.file)
			return err
		}()
		said = append(said, oracle.RefusalOf(table.name, err))
	}
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
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
	// Two semicolons in a row: the four empty fields, and three records in which they are none, the last with a
	// bad coordinate before them.
	for _, empty := range emptyFields {
		c.input(fmt.Sprintf("an empty field: %q", empty.text), []byte(empty.text))
	}
	for _, text := range []string{semicolonsInAValue, semicolonsAfterTheEnd, "C;Xa;;Y1;K1\n"} {
		c.input(fmt.Sprintf("two semicolons and no empty field: %q", text), []byte(text))
	}
	c.expect(tally{
		whole: 26, marked: 1, refused: 10, rows: 18, widerAlike: 2, markedAlike: 1, markedApart: 2,
		emptyRefused: 4, emptyWhole: 3,
	})
}

// The tables of the tests, each changed in forty ways that are the same on every run. A change is named by the
// seed and its index, which make it again.
func TestOracleOnSeededChanges(t *testing.T) {
	const seed, changesOfATable = 2026_10_05, 40
	c := &comparison{t: t}
	changed, index := 0, uint64(0)
	named := testTables()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		for range changesOfATable {
			index++
			text := oracle.Changed(named[name], seed, index)
			if text != named[name] {
				changed++
			}
			c.input(fmt.Sprintf("seed %d, change %d, of %s: %q", seed, index, name, text), []byte(text))
			if t.Failed() {
				t.FailNow()
			}
		}
	}
	if changed != 964 {
		t.Errorf("%d of %d inputs differ from their table, want 964", changed, index)
	}
	c.expect(tally{whole: 1000, refused: 341, rows: 363})
}

// The tables of the tests with one character of ASCII white space put in, at every place and of every kind.
func TestOracleOnEveryPlacingOfWhiteSpace(t *testing.T) {
	c := &comparison{t: t}
	named := testTables()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		for i, text := range oracle.Swept(named[name]) {
			c.input(fmt.Sprintf("%s with white space put in, text %d: %q", name, i, text), []byte(text))
			if t.Failed() {
				t.FailNow()
			}
		}
	}
	c.expect(tally{whole: 1163, refused: 362, rows: 1219})
}

// The tables of the game, read from the maintainer's export: this test reads the game's files, and takes longer
// than the others. A report names a file and a line, and shows no more of the file than one value.
func TestOracleOnTheTablesOfTheGame(t *testing.T) {
	export := testkit.NeedExport(t, "MOONWELL_GAME_DATA")
	c := &comparison{t: t}
	for _, name := range export.Entries("war3.w3mod", "units") {
		if !strings.HasSuffix(strings.ToLower(name), ".slk") {
			continue
		}
		data, err := os.ReadFile(export.Path("war3.w3mod", "units", name))
		if err != nil {
			t.Fatal(err)
		}
		c.input(name, data)
	}
	// The tables of the game version 3.0.0.24268, which is the one the committed data is made from.
	c.expect(tally{whole: 17, rows: 9247})
}
