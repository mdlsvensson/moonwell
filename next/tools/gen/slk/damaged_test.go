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

// everyTable is every table of the tests of this package, the ones that are read and the ones that are refused,
// each with a name for a failure.
func everyTable() map[string]string {
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

// damageSeed is the seed of the changes that TestADamagedTableIsReadOrRefusedByFileAndLineAndNeverPanics makes. A
// failure names the table and the index of the change: testkit.Changed makes the same text of the three again.
const damageSeed = 1986

// damagedFile is the name the test gives Parse for its errors.
const damagedFile = "tables/damaged.slk"

// outcomes counts the damaged tables that were read and the ones that were refused.
type outcomes struct{ read, refused int }

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

// TestADamagedTableIsReadOrRefusedByFileAndLineAndNeverPanics gives Parse every table of the tests cut at every
// length, after each of 60 seeded changes of its lines, quotes and white space, and with one character of ASCII
// white space put in at every place.
func TestADamagedTableIsReadOrRefusedByFileAndLineAndNeverPanics(t *testing.T) {
	var cut, changed, swept outcomes
	named := everyTable()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		text := named[name]
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
	}
	// The numbers are what Parse makes of these inputs: one that differs is a reading that changed.
	if len(named) != 34 || cut != (outcomes{770, 325}) || changed != (outcomes{1299, 741}) ||
		swept != (outcomes{985, 479}) {
		t.Errorf("%d tables; cut: %+v, changed: %+v, with white space put in: %+v", len(named), cut, changed, swept)
	}
}
