package slk_test

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/tools/gen/slk"
)

// tablesOfTheTests are the tables that the tests of this package keep in their constants and lists, the ones
// that are read and the ones that are refused, each with a name for a failure. A table that a test writes in
// its own body is not among them.
func tablesOfTheTests() map[string]string {
	named := map[string]string{
		"mini": mini, "two semicolons in a value": semicolonsInAValue, "two semicolons after the end": semicolonsAfterTheEnd,
		"beside the header": besideTheHeader,
	}
	for _, c := range tables {
		named[c.name] = c.text
	}
	for i, text := range emptyTables {
		named[fmt.Sprintf("empty table %d", i)] = text
	}
	for _, c := range malformed {
		named["malformed "+c.file] = c.text
	}
	for i, c := range emptyFields {
		named[fmt.Sprintf("empty field %d", i)] = c.text
	}
	for _, c := range widerSpace {
		named[c.name] = c.text
	}
	return named
}

// outcomes counts the damaged tables that were read and the ones that were refused.
type outcomes struct{ read, refused int }

// counted are tables of this file's own, each with what Parse makes of it cut at every length, after the seeded
// changes and with white space put in: how many it reads and how many it refuses. They are written here and
// not taken from the lists of the tests, so that a table added to a list changes no number, and a number that
// differs names its table: it is a reading of that table that changed.
var counted = []struct {
	name, text          string
	cut, changed, swept outcomes
}{
	{name: "three rows, the last with its Y before its X",
		text: "ID;PWXL;N;E\r\nC;X1;Y1;K\"ID\"\r\nC;X2;K\"note\"\r\nC;X1;Y2;K\"abcd\"\r\nC;X2;K12\r\n" +
			"C;X1;Y3;K\"efgh\"\r\nC;Y4;X1;K\"a;b\"\r\nC;X2;KTRUE\r\nE\r\nC;X1;Y5;K\"after\"\r\n",
		cut: outcomes{97, 38}, changed: outcomes{38, 22}, swept: outcomes{146, 70}},
	{name: "records that are no cells and a doubled quote",
		text: "ID;P\nB;X2;Y2\nC;X1;Y1;K\"ID\"\nF;P0;X1\nC;X1;Y2;K\"say \"\"hi\"\";ok\";E0\nC;X1;Y3;N;K7\nE\n",
		cut:  outcomes{57, 21}, changed: outcomes{43, 17}, swept: outcomes{92, 7}},
	{name: "a column named twice", text: "C;X1;Y1;K\"ID\"\nC;X2;K\"ID\"\nC;X1;Y2;K1\n",
		cut: outcomes{15, 21}, changed: outcomes{14, 46}, swept: outcomes{5, 34}},
	{name: "a quote that is not closed and a coordinate that is no number",
		text: "C;X1;Y1;Ka\nC;X1;Y2;K\"open\nC;Xb;Y3;K1\n",
		cut:  outcomes{17, 20}, changed: outcomes{2, 58}, swept: outcomes{0, 39}},
}

// damageSeed is the seed of the changes that TestADamagedTableIsReadOrRefusedByFileAndLineAndNeverPanics makes. A
// failure names the table and the index of the change: testkit.Changed makes the same text of the three again.
const damageSeed = 1986

// damagedFile is the name the test gives Parse for its errors.
const damagedFile = "tables/damaged.slk"

// readOrRefused gives Parse the text. It stops the test when Parse panics, when it returns a table with an
// error, and when the error does not start with the file and a line of the text; the error for two columns of
// one name is about the header row, whose cells stand on any lines, and names the file alone.
func (c *outcomes) readOrRefused(t *testing.T, what, text string) {
	t.Helper()
	var table slk.Table
	var err error
	if value := testkit.Panic(func() { table, err = slk.Parse(text, damagedFile) }); value != nil {
		t.Fatalf("%s: Parse panics: %v", what, value)
	}
	if err == nil {
		c.read++
		return
	}
	c.refused++
	if table.Columns != nil || table.Rows != nil {
		t.Fatalf("%s: a refused table is %v, want none", what, table)
	}
	rest, named := strings.CutPrefix(err.Error(), damagedFile+":")
	number, _, _ := strings.Cut(rest, ": ")
	line, notANumber := strconv.Atoi(number)
	onALine := notANumber == nil && line >= 1 && line <= strings.Count(text, "\n")+1
	if !named || !onALine && !strings.HasPrefix(rest, " duplicate column ") {
		t.Fatalf("%s: got %v, want an error that starts with %s and a line of the text", what, err, damagedFile)
	}
}

// damaged gives Parse the table cut at every length, after each of 60 seeded changes of its lines, quotes and
// white space, and with one character of ASCII white space put in at every place, and counts what it makes of
// each.
func damaged(t *testing.T, name, text string) (cut, changed, swept outcomes) {
	t.Helper()
	for length := range len(text) {
		cut.readOrRefused(t, fmt.Sprintf("%s cut at %d bytes", name, length), text[:length])
	}
	for index := range uint64(60) {
		made := testkit.Changed(text, damageSeed, index)
		changed.readOrRefused(t, fmt.Sprintf("%s, change %d of seed %d: %q", name, index, damageSeed, made), made)
	}
	for i, made := range testkit.Swept(text) {
		swept.readOrRefused(t, fmt.Sprintf("%s with white space put in, text %d: %q", name, i, made), made)
	}
	return cut, changed, swept
}

// TestADamagedTableIsReadOrRefusedByFileAndLineAndNeverPanics damages the tables of the tests and the tables of
// this file, and compares what Parse makes of the damaged forms of the latter with the numbers written beside
// them.
func TestADamagedTableIsReadOrRefusedByFileAndLineAndNeverPanics(t *testing.T) {
	named := tablesOfTheTests()
	inputs := 0
	for _, name := range slices.Sorted(maps.Keys(named)) {
		cut, changed, swept := damaged(t, name, named[name])
		inputs += cut.read + cut.refused + changed.read + changed.refused + swept.read + swept.refused
	}
	if len(named) < 34 || inputs < 4000 {
		t.Errorf("%d tables of the tests and %d damaged forms of them, want 34 and 4000 or more", len(named), inputs)
	}
	for _, c := range counted {
		cut, changed, swept := damaged(t, c.name, c.text)
		if cut != c.cut || changed != c.changed || swept != c.swept {
			t.Errorf("%s:\n got cut: %+v, changed: %+v, with white space: %+v\nwant cut: %+v, changed: %+v, with white space: %+v",
				c.name, cut, changed, swept, c.cut, c.changed, c.swept)
		}
	}
}
