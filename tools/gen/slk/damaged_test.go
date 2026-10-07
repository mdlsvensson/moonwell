package slk_test

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/tools/gen/slk"
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

// longerTables are tables of this file's own, longer than the ones of the tests, with what a table of the game
// has side by side: carriage returns, records that are no cells, a Y before its X, a doubled quote, text after
// the end.
var longerTables = map[string]string{
	"three rows, the last with its Y before its X": "ID;PWXL;N;E\r\nC;X1;Y1;K\"ID\"\r\nC;X2;K\"note\"\r\n" +
		"C;X1;Y2;K\"abcd\"\r\nC;X2;K12\r\nC;X1;Y3;K\"efgh\"\r\nC;Y4;X1;K\"a;b\"\r\nC;X2;KTRUE\r\nE\r\n" +
		"C;X1;Y5;K\"after\"\r\n",
	"records that are no cells and a doubled quote": "ID;P\nB;X2;Y2\nC;X1;Y1;K\"ID\"\nF;P0;X1\n" +
		"C;X1;Y2;K\"say \"\"hi\"\";ok\";E0\nC;X1;Y3;N;K7\nE\n",
	"a column named twice": "C;X1;Y1;K\"ID\"\nC;X2;K\"ID\"\nC;X1;Y2;K1\n",
	"a quote that is not closed and a coordinate that is no number": "C;X1;Y1;Ka\nC;X1;Y2;K\"open\n" +
		"C;Xb;Y3;K1\n",
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

// TestADamagedTableIsReadOrRefusedByFileAndLineAndNeverPanics gives Parse the tables of the tests and the
// longer tables of this file, each cut at every length, after each of 60 seeded changes of its lines, quotes
// and white space, and with one character of ASCII white space put in at every place.
func TestADamagedTableIsReadOrRefusedByFileAndLineAndNeverPanics(t *testing.T) {
	named := tablesOfTheTests()
	maps.Copy(named, longerTables)
	var damaged outcomes
	for _, name := range slices.Sorted(maps.Keys(named)) {
		text := named[name]
		for length := range len(text) {
			damaged.readOrRefused(t, fmt.Sprintf("%s cut at %d bytes", name, length), text[:length])
		}
		for index := range uint64(60) {
			made := testkit.Changed(text, damageSeed, index)
			damaged.readOrRefused(t, fmt.Sprintf("%s, change %d of seed %d: %q", name, index, damageSeed, made), made)
		}
		for i, made := range testkit.Swept(text) {
			damaged.readOrRefused(t, fmt.Sprintf("%s with white space put in, text %d: %q", name, i, made), made)
		}
	}
	// The floor is against a test that passes because it gave Parse nothing.
	if damaged.read == 0 || damaged.refused == 0 {
		t.Errorf("%d damaged tables were read and %d refused; want some of each", damaged.read, damaged.refused)
	}
}
