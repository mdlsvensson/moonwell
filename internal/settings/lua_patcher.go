package settings

import (
	"fmt"
	"math"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

type luaPatcher struct {
	source      string
	displayPath string
	eol         string
	functions   []lua.Function
	edits       []lua.Edit
	err         error
}

func newPatcher(source, displayPath string) (*luaPatcher, error) {
	functions, err := lua.ParseFunctions(source, displayPath)
	if err != nil {
		return nil, err
	}
	eol := "\n"
	if strings.Contains(source, "\r\n") {
		eol = "\r\n"
	}
	return &luaPatcher{source: source, displayPath: displayPath, eol: eol, functions: functions}, nil
}

func (p *luaPatcher) fail(err error) {
	if p.err == nil {
		p.err = err
	}
}

func (p *luaPatcher) hasFailed() bool { return p.err != nil }

func (p *luaPatcher) findFunction(name string) lua.Function {
	var found []lua.Function
	for _, function := range p.functions {
		if function.Name == name {
			found = append(found, function)
		}
	}
	switch {
	case p.hasFailed():
		return lua.Function{}
	case len(found) != 1:
		p.fail(errFunctionCount(p.displayPath, name, len(found)))
		return lua.Function{}
	}
	return found[0]
}

func (p *luaPatcher) findCalls(fn lua.Function, name string, arity int) []lua.Call {
	if p.hasFailed() {
		return nil
	}
	var calls []lua.Call
	for _, call := range fn.Calls {
		if call.Name != name {
			continue
		}
		if len(call.Args) != arity {
			p.fail(errArity(p.displayPath, name, fn.Name, arity))
			return nil
		}
		calls = append(calls, call)
	}
	return calls
}

func (p *luaPatcher) requireOne(calls []lua.Call, label string) lua.Call {
	switch {
	case p.hasFailed():
		return lua.Call{}
	case len(calls) != 1:
		p.fail(errNotOne(p.displayPath, label, len(calls)))
		return lua.Call{}
	}
	return calls[0]
}

func (p *luaPatcher) findOneCall(fn lua.Function, name string, arity int) lua.Call {
	return p.requireOne(p.findCalls(fn, name, arity), name+" in "+fn.Name+"()")
}

func (p *luaPatcher) atMostOne(calls []lua.Call, label string) (lua.Call, bool) {
	switch {
	case p.hasFailed() || len(calls) == 0:
		return lua.Call{}, false
	case len(calls) > 1:
		p.fail(errMoreThanOne(p.displayPath, label, len(calls)))
		return lua.Call{}, false
	}
	return calls[0], true
}

func (p *luaPatcher) findPlayerCalls(fn lua.Function, name string, arity, id int) []lua.Call {
	var calls []lua.Call
	for _, call := range p.findCalls(fn, name, arity) {
		target, ok := lua.ParsePlayerID(call.Args[0])
		if !ok {
			p.fail(errUnknownPlayer(p.displayPath, name, fn.Name))
			return nil
		}
		if target == id {
			calls = append(calls, call)
		}
	}
	return calls
}

func isIntLiteral(argument []lua.Token, want int) bool {
	value, ok := lua.ParseNumberLiteral(argument)
	return ok && value == float64(want)
}

func (p *luaPatcher) replace(call lua.Call, statement string) {
	if p.hasFailed() {
		return
	}
	p.edits = append(p.edits, lua.Edit{Start: call.Start, End: call.End, Text: statement + p.semicolonAfter(call)})
}

func (p *luaPatcher) insertAfter(call lua.Call, statements []string) {
	if p.hasFailed() || len(statements) == 0 {
		return
	}
	var added strings.Builder
	for _, statement := range statements {
		added.WriteString(p.separatorAt(call.Start) + statement + p.semicolonAfter(call))
	}
	p.edits = append(p.edits, lua.Edit{Start: call.End, End: call.End, Text: added.String()})
}

func (p *luaPatcher) insertBefore(at int, statements []string) {
	if p.hasFailed() || len(statements) == 0 {
		return
	}
	var added strings.Builder
	for _, statement := range statements {
		added.WriteString(statement + p.separatorAt(at))
	}
	p.edits = append(p.edits, lua.Edit{Start: at, End: at, Text: added.String()})
}

func (p *luaPatcher) remove(call lua.Call) {
	if p.hasFailed() {
		return
	}
	rest := restOfLine(p.source[call.End:])
	_, alone := p.indentationAt(call.Start)
	if alone && rest != "" && startsWithName(p.source[call.End+len(rest):]) {
		p.edits = append(p.edits, lua.Edit{Start: lineStart(p.source, call.Start), End: call.End + len(rest)})
		return
	}
	p.edits = append(p.edits, lua.Edit{Start: call.Start, End: call.End, Text: ";"})
}

func (p *luaPatcher) semicolonAfter(call lua.Call) string {
	if p.source[call.End-1] == ';' {
		return ";"
	}
	return ""
}

func (p *luaPatcher) separatorAt(at int) string {
	if indentation, alone := p.indentationAt(at); alone {
		return p.eol + indentation
	}
	return " "
}

func (p *luaPatcher) indentationAt(at int) (prefix string, blank bool) {
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

func areFinite(values ...float32) bool {
	for _, value := range values {
		if math.IsInf(float64(value), 0) || math.IsNaN(float64(value)) {
			return false
		}
	}
	return true
}

const resaveLua = "Re-save the map in World Editor to restore its generated Lua initialization."

const resaveInfo = "Re-save the map in World Editor to restore its map info."

func errLua(displayPath, problem string) error {
	return errLuaHint(displayPath, problem, resaveLua)
}

func errLuaHint(displayPath, problem, hint string) error {
	return &diag.Error{Msg: "Cannot apply map settings to Lua: " + problem, File: displayPath, Hint: hint}
}

func errFunctionCount(displayPath, name string, count int) error {
	return errLua(displayPath, fmt.Sprintf("expected exactly one global function %s(), found %d.", name, count))
}

func errArity(displayPath, name, function string, arity int) error {
	return errLua(displayPath, fmt.Sprintf("%s in %s() must have %d argument(s).", name, function, arity))
}

func errNotOne(displayPath, label string, count int) error {
	return errLua(displayPath, fmt.Sprintf("expected exactly one direct %s call, found %d.", label, count))
}

func errMoreThanOne(displayPath, label string, count int) error {
	return errLua(displayPath, fmt.Sprintf("expected at most one %s call, found %d.", label, count))
}

func errUnknownPlayer(displayPath, name, function string) error {
	return errLua(displayPath, fmt.Sprintf("cannot identify the player in a %s call in %s().", name, function))
}
