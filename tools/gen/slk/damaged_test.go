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

type outcomes struct{ read, refused int }

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

const damageSeed = 1986

const damagedFile = "tables/damaged.slk"

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
	if damaged.read == 0 || damaged.refused == 0 {
		t.Errorf("%d damaged tables were read and %d refused; want some of each", damaged.read, damaged.refused)
	}
}
