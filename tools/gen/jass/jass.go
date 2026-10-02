// Package jass parses the subset of JASS that common.j and blizzard.j use: declarations only, bodies skipped.
package jass

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/text"
)

// Param is one parameter of a function.
type Param struct{ Type, Name string }

// Function is a native or a function header.
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

// s is JavaScript's \s, and dot its ".": the patterns are the TypeScript generator's.
const s, dot = text.SpaceClass, text.NotLineBreak

var (
	typeLine    = regexp.MustCompile(`^type` + s + `+(\w+)` + s + `+extends` + s + `+(\w+)$`)
	headerLine  = regexp.MustCompile(`^(constant` + s + `+)?(native|function)` + s + `+(\w+)` + s + `+takes` + s + `+(` + dot + `+?)` + s + `+returns` + s + `+(\w+)$`)
	globalLine  = regexp.MustCompile(`^(constant` + s + `+)?(\w+)` + s + `+(array` + s + `+)?(\w+)(` + s + `*=` + dot + `*)?$`)
	paramText   = regexp.MustCompile(`^(\w+)` + s + `+(\w+)$`)
	endFunction = regexp.MustCompile(`^endfunction\b`)
)

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

func parseParams(list string) ([]Param, bool) {
	params := []Param{}
	if list == "nothing" {
		return params, true
	}
	for _, part := range strings.Split(list, ",") {
		match := paramText.FindStringSubmatch(text.Trim(part))
		if match == nil {
			return nil, false
		}
		params = append(params, Param{Type: match[1], Name: match[2]})
	}
	return params, true
}

// Parse reads the declarations of a script. source is the file name recorded on every entry, such as "common.j";
// an error names source and the line.
func Parse(script, source string) (File, error) {
	file := File{Types: []Type{}, Functions: []Function{}, Globals: []Global{}}
	const top, globals, body = 0, 1, 2
	state, bodyStart := top, 0
	for index, raw := range strings.Split(strings.ReplaceAll(script, "\r\n", "\n"), "\n") {
		line := text.Trim(stripComment(raw))
		fail := func() (File, error) {
			return File{}, errors.New(source + ":" + strconv.Itoa(index+1) + ": cannot read " + text.Quote(text.Trim(raw)))
		}
		if line == "" {
			continue
		}
		if state == body {
			if endFunction.MatchString(line) {
				state = top
			}
			continue
		}
		if state == globals {
			if line == "endglobals" {
				state = top
				continue
			}
			match := globalLine.FindStringSubmatch(line)
			if match == nil {
				return fail()
			}
			file.Globals = append(file.Globals, Global{
				Name: match[4], Source: source, Type: match[2], Constant: match[1] != "", Array: match[3] != "",
			})
			continue
		}
		if line == "globals" {
			state = globals
			continue
		}
		if match := typeLine.FindStringSubmatch(line); match != nil {
			file.Types = append(file.Types, Type{Name: match[1], Extends: match[2]})
			continue
		}
		header := headerLine.FindStringSubmatch(line)
		if header == nil {
			return fail()
		}
		params, ok := parseParams(header[4])
		if !ok {
			return fail()
		}
		file.Functions = append(file.Functions, Function{
			Name: header[3], Source: source, Constant: header[1] != "", Params: params, Returns: header[5],
		})
		if header[2] == "function" {
			state, bodyStart = body, index
		}
	}
	switch state {
	case body:
		return File{}, errors.New(source + ":" + strconv.Itoa(bodyStart+1) + ": the function never reaches endfunction")
	case globals:
		return File{}, errors.New(source + ": the globals block never reaches endglobals")
	}
	return file, nil
}
