// Package slk reads the SYLK subset the game's .slk files use: C records with X, Y and K (the last Y, and X, carried
// forward), quoted or bare values, and E as the end. Every other record and cell field is ignored.
package slk

import (
	"errors"
	"slices"
	"strconv"
	"strings"
)

// Row is one record after the header row: its cells by column name, in column order. A missing cell is absent.
type Row struct {
	columns []string
	values  map[string]string
}

// Get returns the cell of a column, and whether the row has it.
func (r Row) Get(column string) (string, bool) {
	value, ok := r.values[column]
	return value, ok
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

// Cells returns the row as a map, for comparing in tests.
func (r Row) Cells() map[string]string { return r.values }

// Table is the header row's names and the rows after it.
type Table struct {
	Columns []string
	Rows    []Row
}

// Parse reads the table in text; file names it in errors.
func Parse(text, file string) (Table, error) {
	cells := map[int]map[int]string{}
	x, y := -1, -1
	for index, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		fail := func(problem string) (Table, error) {
			return Table{}, errors.New(file + ":" + strconv.Itoa(index+1) + ": " + problem)
		}
		if line == "E" || strings.HasPrefix(line, "E;") {
			break
		}
		if !strings.HasPrefix(line, "C;") {
			continue
		}
		value, hasValue := "", false
		for pos := 2; pos < len(line); pos++ {
			kind := line[pos]
			if kind == 'K' {
				var ok bool
				if value, pos, ok = readValue(line, pos+1); !ok {
					return fail("unterminated quoted string")
				}
				hasValue = true
			} else {
				end := strings.IndexByte(line[pos:], ';')
				if end < 0 {
					end = len(line)
				} else {
					end += pos
				}
				field := line[pos+1 : end]
				if kind == 'X' || kind == 'Y' {
					number, ok := coordinate(field)
					if !ok {
						return fail("bad " + string(kind) + " coordinate '" + field + "'")
					}
					if kind == 'X' {
						x = number
					} else {
						y = number
					}
				}
				pos = end
			}
			if pos < len(line) && line[pos] != ';' {
				return fail("expected ';' after a value")
			}
		}
		if !hasValue {
			continue
		}
		if x < 0 || y < 0 {
			return fail("cell without an X or Y coordinate")
		}
		if cells[y] == nil {
			cells[y] = map[int]string{}
		}
		cells[y][x] = value
	}

	ys := sortedKeys(cells)
	if len(ys) == 0 {
		return Table{}, nil
	}
	header := cells[ys[0]]
	var table Table
	seen := map[string]bool{}
	for _, column := range sortedKeys(header) {
		name := header[column]
		if seen[name] {
			return Table{}, errors.New(file + ": duplicate column '" + name + "' in the header row")
		}
		seen[name] = true
		table.Columns = append(table.Columns, name)
	}
	for _, rowY := range ys[1:] {
		row := Row{values: map[string]string{}}
		for _, column := range sortedKeys(cells[rowY]) {
			if name, ok := header[column]; ok {
				row.columns = append(row.columns, name)
				row.values[name] = cells[rowY][column]
			}
		}
		table.Rows = append(table.Rows, row)
	}
	return table, nil
}

// readValue reads a K value starting at pos: a quoted string, where "" is one quote, or bare text up to the next
// ";". It returns the value and the position after it.
func readValue(line string, pos int) (string, int, bool) {
	if pos >= len(line) || line[pos] != '"' {
		end := strings.IndexByte(line[min(pos, len(line)):], ';')
		if end < 0 {
			return line[min(pos, len(line)):], len(line), true
		}
		return line[pos : pos+end], pos + end, true
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
			return value.String(), i + 1, true
		}
	}
	return "", 0, false
}

// coordinate reads the digits of an X or Y field.
func coordinate(field string) (int, bool) {
	if field == "" || strings.Trim(field, "0123456789") != "" {
		return 0, false
	}
	number, err := strconv.Atoi(field)
	return number, err == nil
}

func sortedKeys[V any](m map[int]V) []int {
	keys := make([]int, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}
