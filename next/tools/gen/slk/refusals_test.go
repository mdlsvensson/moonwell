package slk_test

import (
	"fmt"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/tools/gen/slk"
)

// refusedTable is a table that Parse refuses, with the name of its file and a name for the recording.
type refusedTable struct{ name, text, file string }

// refusedTables are the tables the tests of this package hold as refused: the malformed ones, the ones with an
// empty field, one with a fault before its empty field, and a column twice under a name with a space.
func refusedTables() []refusedTable {
	var refused []refusedTable
	for _, c := range malformed {
		refused = append(refused, refusedTable{"malformed " + c.file, c.text, c.file})
	}
	for i, c := range emptyFields {
		refused = append(refused, refusedTable{fmt.Sprintf("empty field %d", i), c.text, "h.slk"})
	}
	return append(refused,
		refusedTable{"a bad coordinate before an empty field", "C;Xa;;Y1;K1\n", "h.slk"},
		refusedTable{"a column twice, the later record first", "C;X2;Y1;K\"a b\"\nC;X1;K\"a b\"\nE\n", "tables/units.slk"},
	)
}

// TestRefusalsAreAsRecorded holds what Parse says of every table of refusedTables to the recording: the text of
// each error, whole. The tests of an error beside it ask for its place and its distinguishing words; the words
// themselves are held here, where a change of them is one line of a diff.
func TestRefusalsAreAsRecorded(t *testing.T) {
	var said []testkit.Refusal
	for _, table := range refusedTables() {
		_, err := slk.Parse(table.text, table.file)
		if err == nil {
			t.Errorf("%s is read", table.name)
		}
		said = append(said, testkit.RefusalOf(table.name, err))
	}
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
}
