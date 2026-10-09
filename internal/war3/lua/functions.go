package lua

import (
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

type Call struct {
	Name  string
	Args  [][]Token
	Start int
	End   int
}

type Function struct {
	Name     string
	Start    int
	End      int
	EndStart int
	Calls    []Call
}

func ParseFunctions(source, displayPath string) (functions []Function, err error) {
	tokens, fault := Tokenize(source)
	if fault != nil {
		return nil, errUnsafe(displayPath, source, fault.Offset, fault.Msg)
	}
	p := &parser{source: source, displayPath: displayPath, tokens: tokens}
	defer func() {
		switch recovered := recover().(type) {
		case nil:
		case parseAbort:
			functions, err = nil, recovered.err
		default:
			panic(recovered)
		}
	}()
	p.parseStatements(atTop)
	return p.functions, nil
}

func lineAndColumn(source string, offset int) (line, column int) {
	before := source[:offset]
	lineStart := strings.LastIndexByte(before, '\n') + 1
	return strings.Count(before, "\n") + 1, utf8.RuneCountInString(before[lineStart:]) + 1
}

func errUnsafe(displayPath, source string, offset int, what string) error {
	line, column := lineAndColumn(source, offset)
	return &diag.Error{
		Msg:    "Cannot safely read map Lua: " + what,
		File:   displayPath,
		Line:   line,
		Column: column,
		Hint:   "Re-save the map in World Editor to restore its generated Lua structure.",
	}
}
