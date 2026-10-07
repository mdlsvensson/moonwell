// Package jass reads the declarations of a JASS script, in the part of the language that common.j and blizzard.j
// use: types, natives, the headers of functions and the lines of a globals block. The body of a function is
// passed over.
//
// It takes the text of a script, decoded, and the name of its file; it returns the types, the functions and the
// globals in the order the script declares them, or an error that names the file and the line.
//
// It must not know which of the declarations the generator keeps, nor how a script is found and decoded: a byte
// order mark is text to it, and its white space is ASCII.
//
// It imports no package of the module.
package jass

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Param is one parameter of a function.
type Param struct{ Type, Name string }

// Function is a native or the header of a function.
type Function struct {
	Name, Source string
	Constant     bool
	Params       []Param
	Returns      string
}

// Global is a line of a globals block.
type Global struct {
	Name, Source, Type string
	Constant, Array    bool
}

// Type is a type and the type it extends.
type Type struct{ Name, Extends string }

// File is what one script declares.
type File struct {
	Types     []Type
	Functions []Function
	Globals   []Global
}

// ws is one character of ASCII white space, and char one character of a line: any but a line feed and a carriage
// return. So a carriage return alone parts two words, and is no part of a parameter list or of what follows an
// equals sign.
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

// Parse reads the declarations of a script. source is the file name recorded on every entry; an error names
// source and the line. A line ends at a line feed: a carriage return before one is white space at the end of
// the line, and goes with it.
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

// The places a line of a script can be in.
const (
	atTheTop = iota
	inGlobals
	inABody
)

// reader reads a script line by line.
type reader struct {
	source    string
	file      File // what the lines so far declare
	place     int  // where the next line is: atTheTop, inGlobals or inABody
	bodyStart int  // the line of the header whose body the reader is in
}

// read reads the line with the number, counted from 1: without its comment and the white space at its ends, an
// empty line is passed over, and so is a line of a body that does not end the body.
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

// global reads a line of a globals block: a global, or the end of the block.
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

// declaration reads a line outside a block and a body: the start of a globals block, a type, a native, or the
// header of a function, whose body the lines after it are.
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

// ended is the failure of a script that ends inside a body or inside a globals block.
func (r *reader) ended() error {
	switch r.place {
	case inABody:
		return errNoEndFunction(r.source, r.bodyStart)
	case inGlobals:
		return errNoEndGlobals(r.source)
	}
	return nil
}

// stripComment returns the line without its "//" comment; a "//" inside a string literal is kept.
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

// parseParams reads what stands between takes and returns: the word nothing, or parameters with commas between
// them, each a type and a name. It is false for a list with anything else in it.
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

// trim takes the ASCII white space off both ends of text.
func trim(text string) string { return strings.Trim(text, " \t\n\v\f\r") }

// quoted writes a line between double quotes, as a JSON string that leaves the markup characters as they are: a
// reader of the message sees a tab or another control character in the line as an escape.
func quoted(line string) string {
	var out strings.Builder
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(line); err != nil {
		// A string always encodes, and writing into memory does not fail.
		return strconv.Quote(line)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

// ---- errors ----

// errCannotRead names a line that is no declaration, as it is written and without the white space at its ends.
func errCannotRead(source string, line int, raw string) error {
	return fmt.Errorf("%s:%d: cannot read %s", source, line, quoted(trim(raw)))
}

func errNoEndFunction(source string, line int) error {
	return fmt.Errorf("%s:%d: the function never reaches endfunction", source, line)
}

func errNoEndGlobals(source string) error {
	return fmt.Errorf("%s: the globals block never reaches endglobals", source)
}
