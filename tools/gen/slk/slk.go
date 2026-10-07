// Package slk reads the part of SYLK that the game's .slk tables use: C records with the fields X, Y and K, quoted
// or bare values, and the E record as the end. A record that names no X or no Y has the one of the record before
// it. Every other record, and every other field of a C record, is passed over.
//
// It takes the text of a table, decoded, and the name of its file; it returns the names of the header row and the
// rows after it by those names, or an error that names the file and the line.
//
// It must not know what a table holds or which columns the generator reads, nor how a file is found and decoded:
// it has no white space, so a byte order mark and a space of any kind are part of the line they stand in.
//
// It imports no package of the module.
package slk

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// Row is one record after the header row: its cells by column name. A cell the row does not have is absent.
type Row struct {
	columns []string          // the columns the row has a cell in, in column order
	values  map[string]string // the cells by column name
}

// Get returns the cell of a column, and whether the row has it.
func (r Row) Get(column string) (string, bool) {
	value, has := r.values[column]
	return value, has
}

// Value returns the cell of a column, "" when the row has none.
func (r Row) Value(column string) string { return r.values[column] }

// First returns the row's first cell, which names the row in a message.
func (r Row) First() string {
	if len(r.columns) == 0 {
		return ""
	}
	return r.values[r.columns[0]]
}

// Table is the header row's names, in column order, and the rows after it.
type Table struct {
	Columns []string
	Rows    []Row
}

// Parse reads the table in text; file names it in an error, with the line.
func Parse(text, file string) (Table, error) {
	cells, err := readCells(text, file)
	if err != nil {
		return Table{}, err
	}
	return tableOf(cells, file)
}

// grid is the cells of a table by their Y, which is the row, and then by their X, which is the column.
type grid map[int]map[int]string

// reader reads the C records of a table one after the other.
type reader struct {
	file string
	line int // the line of the record, counted from 1
	x, y int // the coordinates that the records so far named last; -1 before any
}

// readCells reads the C records up to the E record, and returns the cells they give.
func readCells(text, file string) (grid, error) {
	cells := grid{}
	r := reader{file: file, x: -1, y: -1}
	for index, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if line == "E" || strings.HasPrefix(line, "E;") {
			break
		}
		if !strings.HasPrefix(line, "C;") {
			continue
		}
		r.line = index + 1
		value, hasValue, err := r.record(line)
		switch {
		case err != nil:
			return nil, err
		case !hasValue:
			continue
		case r.x < 0 || r.y < 0:
			return nil, errNoCoordinate(file, r.line)
		}
		if cells[r.y] == nil {
			cells[r.y] = map[int]string{}
		}
		cells[r.y][r.x] = value
	}
	return cells, nil
}

// record reads the fields of one C record, each a letter and what belongs to it, with a ";" between two. An X and
// a Y move the reader, a K is the value of the record, and a field of another letter is passed over.
func (r *reader) record(line string) (value string, hasValue bool, err error) {
	for pos := len("C;"); pos < len(line); pos++ {
		switch line[pos] {
		case 'K':
			value, pos, err = r.value(line, pos+1)
			hasValue = true
		case ';':
			err = errEmptyField(r.file, r.line)
		default:
			pos, err = r.field(line, pos)
		}
		if err != nil {
			return "", false, err
		}
		if pos < len(line) && line[pos] != ';' {
			return "", false, errNoSemicolon(r.file, r.line)
		}
	}
	return value, hasValue, nil
}

// field reads the field whose letter is at pos, up to the next ";" or the end of the line: the number of an X or
// of a Y is where the next cell goes. It returns the position after the field.
func (r *reader) field(line string, pos int) (int, error) {
	text, _, _ := strings.Cut(line[pos+1:], ";")
	end := pos + 1 + len(text)
	letter := line[pos]
	if letter != 'X' && letter != 'Y' {
		return end, nil
	}
	number, ok := coordinate(text)
	if !ok {
		return 0, errBadCoordinate(r.file, r.line, letter, text)
	}
	if letter == 'X' {
		r.x = number
	} else {
		r.y = number
	}
	return end, nil
}

// coordinate reads the digits of an X or of a Y.
func coordinate(digits string) (int, bool) {
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return 0, false
	}
	number, err := strconv.Atoi(digits)
	return number, err == nil
}

// value reads the value of a K that starts at pos: a quoted string, in which two quotes in a row are one quote, or
// bare text up to the next ";". It returns the value and the position after it.
func (r *reader) value(line string, pos int) (string, int, error) {
	if pos == len(line) || line[pos] != '"' {
		bare, _, _ := strings.Cut(line[pos:], ";")
		return bare, pos + len(bare), nil
	}
	var value strings.Builder
	for i := pos + 1; i < len(line); i++ {
		switch {
		case line[i] != '"':
			value.WriteByte(line[i])
		case i+1 < len(line) && line[i+1] == '"':
			value.WriteByte('"')
			i++
		default:
			return value.String(), i + 1, nil
		}
	}
	return "", 0, errUnterminated(r.file, r.line)
}

// tableOf makes a table of the cells. The row with the lowest Y is the header row, whose cells name the columns;
// every row after it holds its cells by those names, in the order of their Y.
func tableOf(cells grid, file string) (Table, error) {
	ys := sortedKeys(cells)
	if len(ys) == 0 {
		return Table{}, nil
	}
	header := cells[ys[0]]
	columns, err := columnsOf(header, file)
	if err != nil {
		return Table{}, err
	}
	table := Table{Columns: columns}
	for _, y := range ys[1:] {
		table.Rows = append(table.Rows, rowOf(cells[y], header))
	}
	return table, nil
}

// columnsOf is the names of the header row, in column order. No two columns have one name.
func columnsOf(header map[int]string, file string) ([]string, error) {
	var columns []string
	for _, x := range sortedKeys(header) {
		if slices.Contains(columns, header[x]) {
			return nil, errDuplicateColumn(file, header[x])
		}
		columns = append(columns, header[x])
	}
	return columns, nil
}

// rowOf is the cells of a row by the names of the header row. A cell in a column the header row does not name is
// left out.
func rowOf(cells, header map[int]string) Row {
	row := Row{values: map[string]string{}}
	for _, x := range sortedKeys(cells) {
		if name, named := header[x]; named {
			row.columns = append(row.columns, name)
			row.values[name] = cells[x]
		}
	}
	return row
}

func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

// ---- errors ----

func errNoCoordinate(file string, line int) error {
	return fmt.Errorf("%s:%d: cell without an X or Y coordinate", file, line)
}

func errBadCoordinate(file string, line int, letter byte, text string) error {
	return fmt.Errorf("%s:%d: bad %c coordinate '%s'", file, line, letter, text)
}

func errUnterminated(file string, line int) error {
	return fmt.Errorf("%s:%d: unterminated quoted string", file, line)
}

func errNoSemicolon(file string, line int) error {
	return fmt.Errorf("%s:%d: expected ';' after a value", file, line)
}

func errEmptyField(file string, line int) error {
	return fmt.Errorf("%s:%d: empty field: two ';' with nothing between them", file, line)
}

func errDuplicateColumn(file, name string) error {
	return fmt.Errorf("%s: duplicate column '%s' in the header row", file, name)
}
