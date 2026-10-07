package slk_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/tools/gen/slk"
)

// The tables below are hand-written miniatures in the shape of the game's files; none is copied from them.

// mini has a header row and three rows after it: a row with a cell in every column, a row that leaves columns
// out, and a row that names its Y before its X. Its lines end with a carriage return and a line feed.
var mini = strings.Join([]string{
	"ID;PWXL;N;E",
	"B;X4;Y4;D0",
	`C;X1;Y1;K"ID"`,
	`C;X2;K"field"`,
	`C;X3;K"count"`,
	`C;X4;K"note"`,
	`C;X1;Y2;K"abcd"`,
	`C;X2;K"Name"`,
	"C;X3;K12",
	`C;X4;K"a;b"`,
	`C;X1;Y3;K"efgh"`,
	"C;X3;K-1.5",
	`C;Y4;X1;K"ijkl"`,
	"C;X2;KTRUE",
	"E",
	`C;X1;Y5;K"after end"`,
}, "\r\n")

// otherRecords has records that are no cells, fields of a cell that are none of X, Y and K, and a cell in a
// column the header row does not name.
var otherRecords = strings.Join([]string{
	"ID;P",
	`C;X1;Y1;K"ID"`,
	"F;P0;FG0G;X1",
	`C;X1;Y2;K"abcd"`,
	`C;X2;K"no header";E0`,
	"P;Pgeneral",
	"C;X1;Y3;N;K7",
	"E",
}, "\n")

// doubledQuotes has a quoted value with two quotes in a row, which are one quote of the value.
var doubledQuotes = strings.Join([]string{`C;X1;Y1;K"ID"`, `C;X1;Y2;K"say ""hi"";ok"`, "E"}, "\n")

// rows is the rows of the table in text, each as its cells by column name.
func rows(t *testing.T, text string) []map[string]string {
	t.Helper()
	table, err := slk.Parse(text, "mini.slk")
	if err != nil {
		t.Fatal(err)
	}
	var cells []map[string]string
	for _, row := range table.Rows {
		cells = append(cells, cellsOf(table, row))
	}
	return cells
}

// cellsOf is the cells a row has, by column name.
func cellsOf(table slk.Table, row slk.Row) map[string]string {
	cells := map[string]string{}
	for _, column := range table.Columns {
		if cell, has := row.Get(column); has {
			cells[column] = cell
		}
	}
	return cells
}

func TestParseReadsCellsByHeaderWithTheLastYCarriedForward(t *testing.T) {
	table, err := slk.Parse(mini, "mini.slk")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(table.Columns, []string{"ID", "field", "count", "note"}) {
		t.Errorf("columns: %q", table.Columns)
	}
	want := []map[string]string{
		{"ID": "abcd", "field": "Name", "count": "12", "note": "a;b"},
		{"ID": "efgh", "count": "-1.5"},
		{"ID": "ijkl", "field": "TRUE"},
	}
	if got := rows(t, mini); !reflect.DeepEqual(got, want) {
		t.Fatalf("rows: %v", got)
	}
	// A row is named by its first cell, and tells an absent cell from an empty one.
	row := table.Rows[1]
	if _, has := row.Get("field"); has || row.Value("field") != "" || row.Value("count") != "-1.5" {
		t.Errorf("row: %v", cellsOf(table, row))
	}
	if row.First() != "efgh" || (slk.Row{}).First() != "" {
		t.Errorf("the first cell of the row is %q, of a row without cells %q", row.First(), slk.Row{}.First())
	}
}

// tables are tables that are read, each with the rows it has after its header row.
var tables = []struct {
	name, text string
	want       []map[string]string
}{
	{"records that are no cells, fields that are no coordinates, a cell without a header", otherRecords,
		[]map[string]string{{"ID": "abcd"}, {"ID": "7"}}},
	{"two quotes in a row inside a quoted value are one quote", doubledQuotes,
		[]map[string]string{{"ID": `say "hi";ok`}}},
	{"a value that is empty, quoted and bare", "C;X1;Y1;Ka\nC;X2;Kb\nC;X1;Y2;K\"\"\nC;X2;K\n",
		[]map[string]string{{"a": "", "b": ""}}},
	{"a record that ends with a semicolon", "C;X1;Y1;Ka\nC;X1;Y2;K\"b\";\nC;X1;Y3;Kc;\n",
		[]map[string]string{{"a": "b"}, {"a": "c"}}},
	{"a record without a value moves the place of the next cell", "C;X1;Y1;Ka\nC;X2;Kb\nC;Y2;X2\nC;K1\n",
		[]map[string]string{{"b": "1"}}},
	{"a later cell at the same place replaces the earlier one", "C;X1;Y1;Ka\nC;X1;Y2;K1\nC;X1;Y2;K2\n",
		[]map[string]string{{"a": "2"}}},
	{"the rows are in the order of their Y, whatever the order of the records",
		"C;X1;Y9;Kz\nC;X1;Y2;Ka\nC;X1;Y5;Kb\n", []map[string]string{{"a": "b"}, {"a": "z"}}},
	{"only a header row", "C;X1;Y1;Ka\nE\n", nil},
	{"columns and rows are in the order of their numbers, 10 after 2", numbered,
		[]map[string]string{{"b": "2", "c": "1"}, {"a": "3"}}},
}

func TestParseReadsTables(t *testing.T) {
	for _, c := range tables {
		if got := rows(t, c.text); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: rows %v, want %v", c.name, got, c.want)
		}
	}
}

// Zero is a coordinate like any other: a cell at X0 or at Y0 is a cell, and the column and the row of the number
// 0 come first.
func TestACellAtTheCoordinateZeroIsACell(t *testing.T) {
	const text = "C;X1;Y0;Kb\nC;X0;Ka\nC;X0;Y1;K1\nC;X1;K2\n"
	table, err := slk.Parse(text, "zero.slk")
	if err != nil || !reflect.DeepEqual(table.Columns, []string{"a", "b"}) {
		t.Fatalf("columns %q, error %v", table.Columns, err)
	}
	if got := rows(t, text); !reflect.DeepEqual(got, []map[string]string{{"a": "1", "b": "2"}}) {
		t.Errorf("rows %v", got)
	}
}

// numbered has a column and a row with the number 10 beside ones with the numbers 1 and 2, and gives the cells of
// its second row with the last column first.
const numbered = "C;X1;Y1;Ka\nC;X2;Kb\nC;X10;Kc\nC;X10;Y2;K1\nC;X2;K2\nC;X1;Y10;K3\n"

// A coordinate is a number: the tenth column comes after the second, and so does the tenth row. The first cell
// of a row is the one in its lowest column.
func TestColumnsAndRowsAreInTheOrderOfTheirNumbers(t *testing.T) {
	table, err := slk.Parse(numbered, "numbered.slk")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(table.Columns, []string{"a", "b", "c"}) {
		t.Errorf("columns: %q", table.Columns)
	}
	if len(table.Rows) != 2 || table.Rows[0].First() != "2" || table.Rows[1].First() != "3" {
		t.Errorf("rows: %v, want the row of Y2 with the first cell 2, then the row of Y10", rows(t, numbered))
	}
}

// besideTheHeader has a header row that names its second column alone, and a row with a cell in each of the two.
const besideTheHeader = "C;X2;Y1;K\"ID\"\nC;X1;Y2;K\"no header\"\nC;X2;K\"abcd\"\n"

// A cell in a column that the header row does not name is left out of its row: the row has no cell without a
// name, and its first cell is the first that has one.
func TestACellInAColumnTheHeaderRowDoesNotNameIsLeftOut(t *testing.T) {
	for name, text := range map[string]string{
		"a cell after the columns of the header row": otherRecords,
		"a cell before them":                         besideTheHeader,
	} {
		table, err := slk.Parse(text, "beside.slk")
		if err != nil || len(table.Rows) == 0 {
			t.Errorf("%s: table %v, error %v", name, table, err)
			continue
		}
		row := table.Rows[0]
		if cell, has := row.Get(""); has || row.Value("") != "" {
			t.Errorf("%s: the row has the cell %q under no name", name, cell)
		}
		if row.First() != "abcd" {
			t.Errorf("%s: the first cell of the row is %q, want the cell of the column ID", name, row.First())
		}
	}
}

// emptyTables are texts without a cell: no text, no record, a cell after each form of the end, and a record
// without a value.
var emptyTables = []string{"", "ID;P\nE\n", "E\nC;X1;Y1;Ka\n", "E;x\nC;X1;Y1;Ka\n", "C;X1;Y1\n"}

func TestATextWithoutACellIsAnEmptyTable(t *testing.T) {
	for _, text := range emptyTables {
		table, err := slk.Parse(text, "empty.slk")
		if err != nil || table.Columns != nil || table.Rows != nil {
			t.Errorf("%q: table %v, error %v", text, table, err)
		}
	}
}

// malformed are the tables that are refused, each with the place and the distinguishing words of its error.
var malformed = []struct{ text, file, place, words string }{
	{"ID;P\nC;X1;K\"ID\"\n", "a.slk", "a.slk:2: ", "without an X or Y coordinate"},
	{"ID;P\nC;X1;Y1;K\"ID\n", "b.slk", "b.slk:2: ", "unterminated quoted string"},
	{"ID;P\nC;Xa;Y1;K1\n", "c.slk", "c.slk:2: ", "bad X coordinate 'a'"},
	{"ID;P\nC;X1;Y1;K\"ID\"\nC;X2;K\"ID\"\n", "d.slk", "d.slk: ", "duplicate column 'ID'"},
	{"C;X1;Y1;K\"ID\"\n\nC;X1;Y;K1\n", "e.slk", "e.slk:3: ", "bad Y coordinate ''"},
	{"C;X1;Y1;K\"ID\"x\n", "f.slk", "f.slk:1: ", "expected ';' after a value"},
	{"C;X1;Y99999999999999999999;K1\n", "g.slk", "g.slk:1: ", "bad Y coordinate"},
	// A coordinate is digits and nothing else, and a cell needs an X as it needs a Y.
	{"C;X+1;Y1;K1\n", "i.slk", "i.slk:1: ", "bad X coordinate '+1'"},
	{"C;X1;Y-1;K1\n", "j.slk", "j.slk:1: ", "bad Y coordinate '-1'"},
	{"C;Y1;K1\n", "k.slk", "k.slk:1: ", "without an X or Y coordinate"},
}

func TestParseNamesTheFileAndLineOfAMalformedRecord(t *testing.T) {
	for _, c := range malformed {
		table, err := slk.Parse(c.text, c.file)
		if err == nil || !strings.HasPrefix(err.Error(), c.place) || !strings.Contains(err.Error(), c.words) {
			t.Errorf("%s: got %v, want an error at %q with %q", c.file, err, c.place, c.words)
		}
		if table.Columns != nil || table.Rows != nil {
			t.Errorf("%s: a refused table is %v, want none", c.file, table)
		}
	}
}

// emptyFields are tables with a record that has a field with nothing in it, each with the place of its error:
// the field is the first of the record, it stands between two others, and it follows a quoted and a bare value.
var emptyFields = []struct{ text, place string }{
	{"C;;X1;Y1;K1\n", "h.slk:1: "},
	{"C;X1;Y1;Ka\nC;X1;;Y2;K1\n", "h.slk:2: "},
	{"C;X1;Y1;K\"a\";;\n", "h.slk:1: "},
	{"C;X1;Y1;Ka;;b\n", "h.slk:1: "},
}

// Tables with two semicolons in a row that are no empty field: the two stand inside a quoted value, or after the
// end of the table.
const (
	semicolonsInAValue    = "C;X1;Y1;Ka\nC;X1;Y2;K\"b;;c\"\n"
	semicolonsAfterTheEnd = "C;X1;Y1;Ka\nE\nC;;X1;Y2;K1\n"
)

// A field holds at least the letter that says what it is. A record with two semicolons in a row outside a quoted
// value has a field that holds nothing, and is refused with its line.
func TestParseRefusesARecordWithAnEmptyField(t *testing.T) {
	for _, c := range emptyFields {
		_, err := slk.Parse(c.text, "h.slk")
		if err == nil || !strings.HasPrefix(err.Error(), c.place) || !strings.Contains(err.Error(), "empty field") {
			t.Errorf("%q: got %v, want an error at %q about an empty field", c.text, err, c.place)
		}
	}
	if got := rows(t, semicolonsInAValue); !reflect.DeepEqual(got, []map[string]string{{"a": "b;;c"}}) {
		t.Errorf("two semicolons inside a quoted value: rows %v", got)
	}
	if got := rows(t, semicolonsAfterTheEnd); got != nil {
		t.Errorf("two semicolons after the end of the table: rows %v", got)
	}
	// A fault that comes before the empty field in its record is the one that is reported.
	if _, err := slk.Parse("C;Xa;;Y1;K1\n", "h.slk"); err == nil || !strings.Contains(err.Error(), "bad X coordinate") {
		t.Errorf("a bad coordinate before an empty field: got %v", err)
	}
}

// widerSpace are tables with a character that is white space outside ASCII. A table has no white space of its
// own: such a character is part of the value it stands in, and a line that starts with one is no cell.
var widerSpace = []struct {
	name, text string
	columns    []string
	want       []map[string]string
}{
	{"a no-break space and an ideographic space in a value",
		"C;X1;Y1;K\"ID\"\nC;X2;K\"note\"\nC;X1;Y2;K\xC2\xA0a\xC2\xA0\nC;X2;K\"\xE3\x80\x80b\xC2\xA0\"\n",
		[]string{"ID", "note"}, []map[string]string{{"ID": "\xC2\xA0a\xC2\xA0", "note": "\xE3\x80\x80b\xC2\xA0"}}},
	{"a byte order mark before a record that is not the first, and a line separator in a column name",
		"C;X1;Y1;K\"ID\"\nC;X2;K\"a\xE2\x80\xA8b\"\nC;X1;Y2;Kx\n\xEF\xBB\xBFC;X2;Ky\n",
		[]string{"ID", "a\xE2\x80\xA8b"}, []map[string]string{{"ID": "x"}}},
}

func TestWhiteSpaceOutsideASCIIIsPartOfAValue(t *testing.T) {
	for _, c := range widerSpace {
		table, err := slk.Parse(c.text, "wide.slk")
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if !reflect.DeepEqual(table.Columns, c.columns) {
			t.Errorf("%s: columns %q, want %q", c.name, table.Columns, c.columns)
		}
		if got := rows(t, c.text); !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: rows %q, want %q", c.name, got, c.want)
		}
	}
}
