package slk

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

type Row struct {
	values map[string]string
}

func (r Row) Get(column string) (string, bool) {
	value, has := r.values[column]
	return value, has
}

func (r Row) Value(column string) string { return r.values[column] }

type Table struct {
	Columns []string
	Rows    []Row
}

func Parse(text, displayPath string) (Table, error) {
	cells, err := readCells(text, displayPath)
	if err != nil {
		return Table{}, err
	}
	return buildTable(cells, displayPath)
}

type grid map[int]map[int]string

type reader struct {
	displayPath string
	line        int
	x, y        int
}

func readCells(text, displayPath string) (grid, error) {
	cells := grid{}
	r := reader{displayPath: displayPath, x: -1, y: -1}
	for index, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if line == "E" || strings.HasPrefix(line, "E;") {
			break
		}
		if !strings.HasPrefix(line, "C;") {
			continue
		}
		r.line = index + 1
		value, hasValue, err := r.readRecord(line)
		switch {
		case err != nil:
			return nil, err
		case !hasValue:
			continue
		case r.x < 0 || r.y < 0:
			return nil, errNoCoordinate(displayPath, r.line)
		}
		if cells[r.y] == nil {
			cells[r.y] = map[int]string{}
		}
		cells[r.y][r.x] = value
	}
	return cells, nil
}

func (r *reader) readRecord(line string) (value string, hasValue bool, err error) {
	for pos := len("C;"); pos < len(line); pos++ {
		switch line[pos] {
		case 'K':
			value, pos, err = r.readValue(line, pos+1)
			hasValue = true
		case ';':
			err = errEmptyField(r.displayPath, r.line)
		default:
			pos, err = r.readField(line, pos)
		}
		if err != nil {
			return "", false, err
		}
		if pos < len(line) && line[pos] != ';' {
			return "", false, errNoSemicolon(r.displayPath, r.line)
		}
	}
	return value, hasValue, nil
}

func (r *reader) readField(line string, pos int) (int, error) {
	text, _, _ := strings.Cut(line[pos+1:], ";")
	end := pos + 1 + len(text)
	letter := line[pos]
	if letter != 'X' && letter != 'Y' {
		return end, nil
	}
	number, ok := parseCoordinate(text)
	if !ok {
		return 0, errBadCoordinate(r.displayPath, r.line, letter, text)
	}
	if letter == 'X' {
		r.x = number
	} else {
		r.y = number
	}
	return end, nil
}

func parseCoordinate(digits string) (int, bool) {
	if digits == "" || strings.Trim(digits, "0123456789") != "" {
		return 0, false
	}
	number, err := strconv.Atoi(digits)
	return number, err == nil
}

func (r *reader) readValue(line string, pos int) (string, int, error) {
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
	return "", 0, errUnterminated(r.displayPath, r.line)
}

func buildTable(cells grid, displayPath string) (Table, error) {
	ys := sortedKeys(cells)
	if len(ys) == 0 {
		return Table{}, nil
	}
	header := cells[ys[0]]
	columns, err := buildColumns(header, displayPath)
	if err != nil {
		return Table{}, err
	}
	table := Table{Columns: columns}
	for _, y := range ys[1:] {
		table.Rows = append(table.Rows, buildRow(cells[y], header))
	}
	return table, nil
}

func buildColumns(header map[int]string, displayPath string) ([]string, error) {
	var columns []string
	for _, x := range sortedKeys(header) {
		if slices.Contains(columns, header[x]) {
			return nil, errDuplicateColumn(displayPath, header[x])
		}
		columns = append(columns, header[x])
	}
	return columns, nil
}

func buildRow(cells, header map[int]string) Row {
	row := Row{values: map[string]string{}}
	for x, cell := range cells {
		if name, named := header[x]; named {
			row.values[name] = cell
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

func errNoCoordinate(displayPath string, line int) error {
	return fmt.Errorf("%s:%d: cell without an X or Y coordinate", displayPath, line)
}

func errBadCoordinate(displayPath string, line int, letter byte, text string) error {
	return fmt.Errorf("%s:%d: bad %c coordinate '%s'", displayPath, line, letter, text)
}

func errUnterminated(displayPath string, line int) error {
	return fmt.Errorf("%s:%d: unterminated quoted string", displayPath, line)
}

func errNoSemicolon(displayPath string, line int) error {
	return fmt.Errorf("%s:%d: expected ';' after a value", displayPath, line)
}

func errEmptyField(displayPath string, line int) error {
	return fmt.Errorf("%s:%d: empty field: two ';' with nothing between them", displayPath, line)
}

func errDuplicateColumn(displayPath, name string) error {
	return fmt.Errorf("%s: duplicate column '%s' in the header row", displayPath, name)
}
