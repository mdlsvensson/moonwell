package slk_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/tools/gen/slk"
)

// Hand-written excerpts in the shape of the game's files; not copied from them.
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

func rows(t *testing.T, text string) []map[string]string {
	t.Helper()
	table, err := slk.Parse(text, "mini.slk")
	if err != nil {
		t.Fatal(err)
	}
	var cells []map[string]string
	for _, row := range table.Rows {
		cells = append(cells, row.Cells())
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
		t.Errorf("rows: %v", got)
	}
	// A row is named by its first cell, and tells an absent cell from an empty one.
	row := table.Rows[1]
	if _, has := row.Get("field"); has || row.Value("field") != "" || row.First() != "efgh" {
		t.Errorf("row: %v", row.Cells())
	}
}

func TestParseIgnoresFormattingRecordsUnknownCellFieldsAndCellsWithoutAHeader(t *testing.T) {
	text := strings.Join([]string{
		"ID;P",
		`C;X1;Y1;K"ID"`,
		"F;P0;FG0G;X1",
		`C;X1;Y2;K"abcd"`,
		`C;X2;K"no header";E0`,
		"P;Pgeneral",
		"C;X1;Y3;N;K7",
		"E",
	}, "\n")
	if got := rows(t, text); !reflect.DeepEqual(got, []map[string]string{{"ID": "abcd"}, {"ID": "7"}}) {
		t.Errorf("rows: %v", got)
	}
}

func TestParseReadsDoubledQuotesInsideAQuotedStringAsOneQuote(t *testing.T) {
	text := strings.Join([]string{`C;X1;Y1;K"ID"`, `C;X1;Y2;K"say ""hi"";ok"`, "E"}, "\n")
	if got := rows(t, text); !reflect.DeepEqual(got, []map[string]string{{"ID": `say "hi";ok`}}) {
		t.Errorf("rows: %v", got)
	}
}

func TestParseNamesTheFileAndLineOfAMalformedRecord(t *testing.T) {
	for _, c := range []struct{ text, file, want string }{
		{"ID;P\nC;X1;K\"ID\"\n", "a.slk", "a.slk:2"},
		{"ID;P\nC;X1;Y1;K\"ID\n", "b.slk", "b.slk:2"},
		{"ID;P\nC;Xa;Y1;K1\n", "c.slk", "c.slk:2"},
		{"ID;P\nC;X1;Y1;K\"ID\"\nC;X2;K\"ID\"\n", "d.slk", "duplicate column"},
	} {
		if _, err := slk.Parse(c.text, c.file); err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: got %v, want an error with %q", c.file, err, c.want)
		}
	}
}
