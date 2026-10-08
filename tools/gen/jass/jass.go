package jass

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type Param struct{ Type, Name string }

type Function struct {
	Name, Source string
	Constant     bool
	Params       []Param
	Returns      string
}

type Global struct {
	Name, Source, Type string
	Constant, Array    bool
}

type Type struct{ Name, Extends string }

type File struct {
	Types     []Type
	Functions []Function
	Globals   []Global
}

const ws, char = `[\t\n\v\f\r ]`, `[^\n\r]`

var (
	typeLine   = regexp.MustCompile(`^type` + ws + `+(\w+)` + ws + `+extends` + ws + `+(\w+)$`)
	headerLine = regexp.MustCompile(`^(constant` + ws + `+)?(native|function)` + ws + `+(\w+)` + ws + `+takes` + ws +
		`+(` + char + `+?)` + ws + `+returns` + ws + `+(\w+)$`)
	globalLine = regexp.MustCompile(`^(constant` + ws + `+)?(\w+)` + ws + `+(array` + ws + `+)?(\w+)(` + ws + `*=` +
		char + `*)?$`)
	paramText   = regexp.MustCompile(`^(\w+)` + ws + `+(\w+)$`)
	endFunction = regexp.MustCompile(`^endfunction\b`)
)

func Parse(script, source string) (File, error) {
	r := reader{source: source, file: File{Types: []Type{}, Functions: []Function{}, Globals: []Global{}}}
	for index, raw := range strings.Split(script, "\n") {
		if err := r.read(index+1, raw); err != nil {
			return File{}, err
		}
	}
	if err := r.ended(); err != nil {
		return File{}, err
	}
	return r.file, nil
}

const (
	atTheTop = iota
	inGlobals
	inABody
)

type reader struct {
	source    string
	file      File
	place     int
	bodyStart int
}

func (r *reader) read(number int, raw string) error {
	line := trim(stripComment(raw))
	switch {
	case line == "":
	case r.place == inABody:
		if endFunction.MatchString(line) {
			r.place = atTheTop
		}
	case r.place == inGlobals:
		return r.global(number, raw, line)
	default:
		return r.declaration(number, raw, line)
	}
	return nil
}

func (r *reader) global(number int, raw, line string) error {
	if line == "endglobals" {
		r.place = atTheTop
		return nil
	}
	match := globalLine.FindStringSubmatch(line)
	if match == nil {
		return errCannotRead(r.source, number, raw)
	}
	r.file.Globals = append(r.file.Globals, Global{
		Name: match[4], Source: r.source, Type: match[2], Constant: match[1] != "", Array: match[3] != "",
	})
	return nil
}

func (r *reader) declaration(number int, raw, line string) error {
	if line == "globals" {
		r.place = inGlobals
		return nil
	}
	if match := typeLine.FindStringSubmatch(line); match != nil {
		r.file.Types = append(r.file.Types, Type{Name: match[1], Extends: match[2]})
		return nil
	}
	header := headerLine.FindStringSubmatch(line)
	if header == nil {
		return errCannotRead(r.source, number, raw)
	}
	params, ok := parseParams(header[4])
	if !ok {
		return errCannotRead(r.source, number, raw)
	}
	r.file.Functions = append(r.file.Functions, Function{
		Name: header[3], Source: r.source, Constant: header[1] != "", Params: params, Returns: header[5],
	})
	if header[2] == "function" {
		r.place, r.bodyStart = inABody, number
	}
	return nil
}

func (r *reader) ended() error {
	switch r.place {
	case inABody:
		return errNoEndFunction(r.source, r.bodyStart)
	case inGlobals:
		return errNoEndGlobals(r.source)
	}
	return nil
}

func stripComment(line string) string {
	inString := false
	for i := 0; i < len(line); i++ {
		switch c := line[i]; {
		case inString && c == '\\':
			i++
		case c == '"':
			inString = !inString
		case !inString && c == '/' && i+1 < len(line) && line[i+1] == '/':
			return line[:i]
		}
	}
	return line
}

func parseParams(list string) ([]Param, bool) {
	params := []Param{}
	if list == "nothing" {
		return params, true
	}
	for _, part := range strings.Split(list, ",") {
		match := paramText.FindStringSubmatch(trim(part))
		if match == nil {
			return nil, false
		}
		params = append(params, Param{Type: match[1], Name: match[2]})
	}
	return params, true
}

func trim(text string) string { return strings.Trim(text, " \t\n\v\f\r") }

func quoted(line string) string {
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(line); err != nil {
		return strconv.Quote(line)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func errCannotRead(source string, line int, raw string) error {
	return fmt.Errorf("%s:%d: cannot read %s", source, line, quoted(trim(raw)))
}

func errNoEndFunction(source string, line int) error {
	return fmt.Errorf("%s:%d: the function never reaches endfunction", source, line)
}

func errNoEndGlobals(source string) error {
	return fmt.Errorf("%s: the globals block never reaches endglobals", source)
}
