package settings

import (
	"fmt"
	"math"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

// patcher gathers the edits of one script: it finds the functions and the calls World Editor wrote, and replaces,
// adds and removes call statements. It holds the source, its top-level functions, the edits so far and the first
// failure.
//
// A script is edited only where its shape is the one World Editor writes; anything else is a failure. Once the
// patcher has failed, its methods find nothing and edit nothing: they return zero values, so a step reads straight
// through and patchLua asks once, at the end, what went wrong.
type patcher struct {
	source    string
	file      string // the name errors give
	eol       string // the line ending of the source
	functions []lua.Function
	edits     []lua.Edit
	failure   error
}

// newPatcher reads the functions of a script. A script that does not read is refused.
func newPatcher(source, file string) (*patcher, error) {
	functions, err := lua.Functions(source, file)
	if err != nil {
		return nil, err
	}
	eol := "\n"
	if strings.Contains(source, "\r\n") {
		eol = "\r\n"
	}
	return &patcher{source: source, file: file, eol: eol, functions: functions}, nil
}

// refuse records why the settings do not go into this script, unless an earlier reason is recorded.
func (p *patcher) refuse(reason error) {
	if p.failure == nil {
		p.failure = reason
	}
}

func (p *patcher) failed() bool { return p.failure != nil }

// ---- finding ----

// function is the one top-level function of the name.
func (p *patcher) function(name string) lua.Function {
	var found []lua.Function
	for _, function := range p.functions {
		if function.Name == name {
			found = append(found, function)
		}
	}
	switch {
	case p.failed():
		return lua.Function{}
	case len(found) != 1:
		p.refuse(errFunctionCount(p.file, name, len(found)))
		return lua.Function{}
	}
	return found[0]
}

// callsNamed is the calls of the name directly in the function. Each must have as many arguments as the native
// takes.
func (p *patcher) callsNamed(fn lua.Function, name string, arity int) []lua.Call {
	if p.failed() {
		return nil
	}
	var calls []lua.Call
	for _, call := range fn.Calls {
		if call.Name != name {
			continue
		}
		if len(call.Args) != arity {
			p.refuse(errArity(p.file, name, fn.Name, arity))
			return nil
		}
		calls = append(calls, call)
	}
	return calls
}

// one is the call of calls, which must be exactly one. label is what a refusal calls it.
func (p *patcher) one(calls []lua.Call, label string) lua.Call {
	switch {
	case p.failed():
		return lua.Call{}
	case len(calls) != 1:
		p.refuse(errNotOne(p.file, label, len(calls)))
		return lua.Call{}
	}
	return calls[0]
}

// unique is the one call of the name directly in the function.
func (p *patcher) unique(fn lua.Function, name string, arity int) lua.Call {
	return p.one(p.callsNamed(fn, name, arity), name+" in "+fn.Name+"()")
}

// optional is the call of calls and whether there is one. There must not be more.
func (p *patcher) optional(calls []lua.Call, label string) (lua.Call, bool) {
	switch {
	case p.failed() || len(calls) == 0:
		return lua.Call{}, false
	case len(calls) > 1:
		p.refuse(errMoreThanOne(p.file, label, len(calls)))
		return lua.Call{}, false
	}
	return calls[0], true
}

// forPlayer is the calls of the name directly in the function whose first argument is the player of the id. Every
// call of the name must name its player as `Player(n)` with a literal n, or it cannot be known which calls are
// this player's.
func (p *patcher) forPlayer(fn lua.Function, name string, arity, id int) []lua.Call {
	var calls []lua.Call
	for _, call := range p.callsNamed(fn, name, arity) {
		target, ok := lua.PlayerID(call.Args[0])
		if !ok {
			p.refuse(errUnknownPlayer(p.file, name, fn.Name))
			return nil
		}
		if target == id {
			calls = append(calls, call)
		}
	}
	return calls
}

// literalIs reports whether an argument is the whole number, written as a literal.
func literalIs(argument []lua.Token, want int) bool {
	value, ok := lua.LiteralNumber(argument)
	return ok && value == float64(want)
}

// ---- editing ----

// replace puts a statement in the place of the call.
func (p *patcher) replace(call lua.Call, statement string) {
	if p.failed() {
		return
	}
	p.edits = append(p.edits, lua.Edit{Start: call.Start, End: call.End, Text: statement + p.semicolon(call)})
}

// insertAfter adds statements after the call: each on a line of its own with the call's indentation when the call
// starts its line, and after a space otherwise. Each ends as the call does, with or without a `;`.
func (p *patcher) insertAfter(call lua.Call, statements []string) {
	if p.failed() || len(statements) == 0 {
		return
	}
	var added strings.Builder
	for _, statement := range statements {
		added.WriteString(p.separator(call.Start) + statement + p.semicolon(call))
	}
	p.edits = append(p.edits, lua.Edit{Start: call.End, End: call.End, Text: added.String()})
}

// insertBefore adds statements before what stands at the offset: each on a line of its own with the indentation
// of that line when nothing but blanks is before the offset on it, and before a space otherwise.
func (p *patcher) insertBefore(at int, statements []string) {
	if p.failed() || len(statements) == 0 {
		return
	}
	var added strings.Builder
	for _, statement := range statements {
		added.WriteString(statement + p.separator(at))
	}
	p.edits = append(p.edits, lua.Edit{Start: at, End: at, Text: added.String()})
}

// remove takes the call out. Its whole line goes when the call is alone on it and the statement after it starts
// with a name, which cannot continue an expression of the statement before. In any other place a `;` is left
// where the call was, so that the statements around it stay apart.
func (p *patcher) remove(call lua.Call) {
	if p.failed() {
		return
	}
	rest := restOfLine(p.source[call.End:])
	_, alone := p.indentation(call.Start)
	if alone && rest != "" && startsWithName(p.source[call.End+len(rest):]) {
		p.edits = append(p.edits, lua.Edit{Start: lineStart(p.source, call.Start), End: call.End + len(rest)})
		return
	}
	p.edits = append(p.edits, lua.Edit{Start: call.Start, End: call.End, Text: ";"})
}

// semicolon is the `;` that ends the call, which the range of a call includes when one follows it directly. A
// statement put in the call's place keeps it, so that a statement after it that starts with `(` is not read as its
// arguments.
func (p *patcher) semicolon(call lua.Call) string {
	if p.source[call.End-1] == ';' {
		return ";"
	}
	return ""
}

// separator is what stands between two statements at the offset: a line ending and the indentation of the line,
// when the offset is where the line's own text starts, and a space otherwise.
func (p *patcher) separator(at int) string {
	if indentation, alone := p.indentation(at); alone {
		return p.eol + indentation
	}
	return " "
}

// indentation is what stands on the line before the offset, and whether that is nothing but blanks.
func (p *patcher) indentation(at int) (prefix string, blank bool) {
	prefix = p.source[lineStart(p.source, at):at]
	return prefix, strings.Trim(prefix, " \t") == ""
}

// ---- lines ----

// lineStart is the offset at which the line of the offset begins.
func lineStart(source string, at int) int {
	return strings.LastIndexByte(source[:at], '\n') + 1
}

// restOfLine is the blanks and the line ending that text starts with, or "" when something else stands before the
// end of its first line, or the line does not end.
func restOfLine(text string) string {
	blanks := len(text) - len(strings.TrimLeft(text, " \t"))
	switch after := text[blanks:]; {
	case strings.HasPrefix(after, "\r\n"):
		return text[:blanks+2]
	case strings.HasPrefix(after, "\n"):
		return text[:blanks+1]
	}
	return ""
}

// startsWithName reports whether the first thing in text after white space is a letter or an underscore: a name
// or a keyword, and not a comment, a number, a string or a symbol.
func startsWithName(text string) bool {
	text = strings.TrimLeft(text, fsx.ASCIISpace)
	if text == "" {
		return false
	}
	first := text[0]
	return first == '_' || first >= 'A' && first <= 'Z' || first >= 'a' && first <= 'z'
}

// ---- the map info ----

// finite reports whether every value is a number. The script takes a value of the map info as it is, and a file
// that a tool or a hand changed can hold one that is none.
func finite(values ...float32) bool {
	for _, value := range values {
		if math.IsInf(float64(value), 0) || math.IsNaN(float64(value)) {
			return false
		}
	}
	return true
}

// ---- errors ----

// resaveLua is the hint of a script whose shape is not the one World Editor writes.
const resaveLua = "Re-save the map in World Editor to restore its generated Lua initialization."

// resaveInfo is the hint of a map info with a value World Editor does not write.
const resaveInfo = "Re-save the map in World Editor to restore its map info."

// errLua says that the settings do not go into the script as it is, and why.
func errLua(file, problem string) error {
	return errLuaHint(file, problem, resaveLua)
}

// errLuaHint is errLua for a problem that saving the map again does not solve.
func errLuaHint(file, problem, hint string) error {
	return &diag.Error{Msg: "Cannot apply map settings to Lua: " + problem, File: file, Hint: hint}
}

func errFunctionCount(file, name string, count int) error {
	return errLua(file, fmt.Sprintf("expected exactly one global function %s(), found %d.", name, count))
}

func errArity(file, name, function string, arity int) error {
	return errLua(file, fmt.Sprintf("%s in %s() must have %d argument(s).", name, function, arity))
}

func errNotOne(file, label string, count int) error {
	return errLua(file, fmt.Sprintf("expected exactly one direct %s call, found %d.", label, count))
}

func errMoreThanOne(file, label string, count int) error {
	return errLua(file, fmt.Sprintf("expected at most one %s call, found %d.", label, count))
}

func errUnknownPlayer(file, name, function string) error {
	return errLua(file, fmt.Sprintf("cannot identify the player in a %s call in %s().", name, function))
}
