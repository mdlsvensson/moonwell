package settings

import (
	"fmt"
	"math"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

type patcher struct {
	source    string
	file      string
	eol       string
	functions []lua.Function
	edits     []lua.Edit
	failure   error
}

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

func (p *patcher) refuse(reason error) {
	if p.failure == nil {
		p.failure = reason
	}
}

func (p *patcher) failed() bool { return p.failure != nil }

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

func (p *patcher) unique(fn lua.Function, name string, arity int) lua.Call {
	return p.one(p.callsNamed(fn, name, arity), name+" in "+fn.Name+"()")
}

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

func literalIs(argument []lua.Token, want int) bool {
	value, ok := lua.LiteralNumber(argument)
	return ok && value == float64(want)
}

func (p *patcher) replace(call lua.Call, statement string) {
	if p.failed() {
		return
	}
	p.edits = append(p.edits, lua.Edit{Start: call.Start, End: call.End, Text: statement + p.semicolon(call)})
}

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

func (p *patcher) semicolon(call lua.Call) string {
	if p.source[call.End-1] == ';' {
		return ";"
	}
	return ""
}

func (p *patcher) separator(at int) string {
	if indentation, alone := p.indentation(at); alone {
		return p.eol + indentation
	}
	return " "
}

func (p *patcher) indentation(at int) (prefix string, blank bool) {
	prefix = p.source[lineStart(p.source, at):at]
	return prefix, strings.Trim(prefix, " \t") == ""
}

func lineStart(source string, at int) int {
	return strings.LastIndexByte(source[:at], '\n') + 1
}

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

func startsWithName(text string) bool {
	text = strings.TrimLeft(text, fsx.ASCIISpace)
	if text == "" {
		return false
	}
	first := text[0]
	return first == '_' || first >= 'A' && first <= 'Z' || first >= 'a' && first <= 'z'
}

func finite(values ...float32) bool {
	for _, value := range values {
		if math.IsInf(float64(value), 0) || math.IsNaN(float64(value)) {
			return false
		}
	}
	return true
}

const resaveLua = "Re-save the map in World Editor to restore its generated Lua initialization."

const resaveInfo = "Re-save the map in World Editor to restore its map info."

func errLua(file, problem string) error {
	return errLuaHint(file, problem, resaveLua)
}

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
