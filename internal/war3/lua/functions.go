package lua

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
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

func Functions(source, file string) (functions []Function, err error) {
	tokens, fault := Tokenize(source)
	if fault != nil {
		return nil, errUnsafe(file, source, fault.Offset, fault.Msg)
	}
	r := &reader{source: source, file: file, tokens: tokens}
	defer func() {
		switch stopped := recover().(type) {
		case nil:
		case unreadable:
			functions, err = nil, stopped.err
		default:
			panic(stopped)
		}
	}()
	r.statements(atTop)
	return r.functions, nil
}

type reader struct {
	source, file string
	tokens       []Token
	at           int
	depth        int
	functions    []Function
	calls        []Call
}

type unreadable struct{ err error }

type place uint8

const (
	inBlock place = iota
	atTop
	inFunction
)

const maxDepth = 200

const unaryBinding = 11

var binding = map[string]int{
	"or": 1, "and": 2,
	"<": 3, ">": 3, "<=": 3, ">=": 3, "~=": 3, "==": 3,
	"|": 4, "~": 5, "&": 6, "<<": 7, ">>": 7, "..": 8,
	"+": 9, "-": 9, "*": 10, "/": 10, "//": 10, "%": 10, "^": 12,
}

func (r *reader) more() bool { return r.at < len(r.tokens) }

func (r *reader) raw() string {
	if r.more() {
		return r.tokens[r.at].Raw
	}
	return ""
}

func (r *reader) is(kind Kind) bool { return r.more() && r.tokens[r.at].Kind == kind }

func (r *reader) take(raw string) bool {
	if r.raw() != raw {
		return false
	}
	r.at++
	return true
}

func (r *reader) expect(raw string) Token {
	if r.raw() != raw {
		r.fail("expected '" + raw + "'")
	}
	r.at++
	return r.tokens[r.at-1]
}

func (r *reader) name() string {
	if !r.is(NameToken) || keywords[r.raw()] {
		r.fail("expected a name")
	}
	r.at++
	return r.tokens[r.at-1].Raw
}

func (r *reader) fail(what string) {
	offset := len(r.source)
	if r.more() {
		offset = r.tokens[r.at].Start
	}
	panic(unreadable{errUnsafe(r.file, r.source, offset, what)})
}

func (r *reader) enter() {
	r.depth++
	if r.depth > maxDepth {
		r.fail("nesting is too deep to establish safe edit boundaries")
	}
}

func (r *reader) statements(where place, stops ...string) {
	r.enter()
	for r.more() && !slices.Contains(stops, r.raw()) {
		if r.take("return") {
			r.returned(stops)
			break
		}
		r.statement(where)
	}
	if where != atTop && !r.more() {
		r.fail("unterminated block")
	}
	r.depth--
}

func (r *reader) returned(stops []string) {
	ends := func() bool { return !r.more() || slices.Contains(stops, r.raw()) }
	if !ends() && r.raw() != ";" {
		r.expressions()
	}
	r.take(";")
	if !ends() {
		r.fail("return must end its block")
	}
}

func (r *reader) block() {
	r.statements(inBlock, "end")
	r.expect("end")
}

func (r *reader) statement(where place) {
	start := r.tokens[r.at].Start
	switch {
	case r.take(";"):
	case r.take("function"):
		r.function(start, where)
	case r.take("local"):
		r.local()
	case r.take("if"):
		r.conditional()
	case r.take("while"):
		r.expression(1)
		r.expect("do")
		r.block()
	case r.take("for"):
		r.loop()
	case r.take("do"):
		r.block()
	case r.take("repeat"):
		r.statements(inBlock, "until")
		r.expect("until")
		r.expression(1)
	case r.take("break"):
	case r.take("goto"):
		r.name()
	case r.take("::"):
		r.name()
		r.expect("::")
	default:
		r.assignmentOrCall(where)
	}
}

func (r *reader) function(start int, where place) {
	name, bare := r.functionName()
	if where != atTop || !bare {
		r.functionBody(inBlock)
		return
	}
	r.calls = []Call{}
	end := r.functionBody(inFunction)
	r.functions = append(r.functions, Function{
		Name: name, Start: start, End: end.End, EndStart: end.Start, Calls: r.calls,
	})
}

func (r *reader) functionName() (name string, bare bool) {
	name, bare = r.name(), true
	for r.take(".") {
		bare = false
		r.name()
	}
	if r.take(":") {
		bare = false
		r.name()
	}
	return name, bare
}

func (r *reader) functionBody(where place) Token {
	r.expect("(")
	if !r.take(")") {
		r.parameters()
		r.expect(")")
	}
	r.statements(where, "end")
	return r.expect("end")
}

func (r *reader) parameters() {
	for !r.take("...") {
		r.name()
		if !r.take(",") {
			return
		}
	}
}

func (r *reader) local() {
	if r.take("function") {
		r.name()
		r.functionBody(inBlock)
		return
	}
	r.name()
	for r.take(",") {
		r.name()
	}
	if r.take("=") {
		r.expressions()
	}
}

func (r *reader) conditional() {
	for {
		r.expression(1)
		r.expect("then")
		r.statements(inBlock, "elseif", "else", "end")
		if !r.take("elseif") {
			break
		}
	}
	if r.take("else") {
		r.statements(inBlock, "end")
	}
	r.expect("end")
}

func (r *reader) loop() {
	r.name()
	if r.take("=") {
		r.expression(1)
		r.expect(",")
		r.expression(1)
		if r.take(",") {
			r.expression(1)
		}
	} else {
		for r.take(",") {
			r.name()
		}
		r.expect("in")
		r.expressions()
	}
	r.expect("do")
	r.block()
}

func (r *reader) assignmentOrCall(where place) {
	target := r.prefix()
	switch {
	case r.raw() == "=" || r.raw() == ",":
		r.assignment(target)
	case !target.call:
		r.fail("expected an assignment or call statement")
	case target.direct != nil && where == inFunction:
		r.calls = append(r.calls, r.withSemicolon(*target.direct))
	}
}

func (r *reader) assignment(first prefixed) {
	if !first.assignable {
		r.fail("invalid assignment target")
	}
	for r.take(",") {
		if !r.prefix().assignable {
			r.fail("invalid assignment target")
		}
	}
	r.expect("=")
	r.expressions()
}

func (r *reader) withSemicolon(call Call) Call {
	if r.raw() != ";" {
		return call
	}
	semicolon := r.tokens[r.at]
	if fsx.TrimASCIISpace(r.source[call.End:semicolon.Start]) != "" {
		return call
	}
	r.at++
	call.End = semicolon.End
	return call
}

func (r *reader) expressions() {
	r.expression(1)
	for r.take(",") {
		r.expression(1)
	}
}

func (r *reader) expression(minimum int) {
	r.enter()
	r.operand()
	for {
		operator := binding[r.raw()]
		if operator == 0 || operator < minimum {
			break
		}
		r.at++
		r.expression(operator + 1)
	}
	r.depth--
}

func (r *reader) operand() {
	switch raw := r.raw(); {
	case raw == "not" || raw == "#" || raw == "-" || raw == "~":
		r.at++
		r.expression(unaryBinding)
	case r.take("function"):
		r.functionBody(inBlock)
	case raw == "{":
		r.table()
	case raw == "nil" || raw == "true" || raw == "false" || raw == "..." || r.is(NumberToken) || r.is(StringToken):
		r.at++
	default:
		r.prefix()
	}
}

type prefixed struct {
	bare       string
	assignable bool
	call       bool
	direct     *Call
}

func (r *reader) prefix() prefixed {
	start := len(r.source)
	if r.more() {
		start = r.tokens[r.at].Start
	}
	var current prefixed
	if r.take("(") {
		r.expression(1)
		r.expect(")")
	} else {
		current = prefixed{bare: r.name(), assignable: true}
	}
	for {
		next, found := r.suffix(current, start)
		if !found {
			return current
		}
		current = next
	}
}

func (r *reader) suffix(before prefixed, start int) (after prefixed, found bool) {
	switch {
	case r.take("["):
		r.expression(1)
		r.expect("]")
		return prefixed{assignable: true}, true
	case r.take("."):
		r.name()
		return prefixed{assignable: true}, true
	case r.take(":"):
		r.name()
		r.arguments()
		return prefixed{call: true}, true
	case r.raw() == "(" || r.raw() == "{" || r.is(StringToken):
		args := r.arguments()
		after = prefixed{call: true}
		if before.bare != "" {
			after.direct = &Call{Name: before.bare, Args: args, Start: start, End: r.tokens[r.at-1].End}
		}
		return after, true
	}
	return before, false
}

func (r *reader) arguments() [][]Token {
	if r.take("(") {
		return r.argumentList()
	}
	start := r.at
	switch {
	case r.raw() == "{":
		r.table()
	case r.is(StringToken):
		r.at++
	default:
		r.fail("expected call arguments")
	}
	return [][]Token{r.tokens[start:r.at]}
}

func (r *reader) argumentList() [][]Token {
	args := [][]Token{}
	if r.take(")") {
		return args
	}
	for {
		start := r.at
		r.expression(1)
		args = append(args, r.tokens[start:r.at])
		if !r.take(",") {
			break
		}
	}
	r.expect(")")
	return args
}

func (r *reader) table() {
	r.expect("{")
	for !r.take("}") {
		r.field()
		if !r.take(",") && !r.take(";") {
			r.expect("}")
			return
		}
	}
}

func (r *reader) field() {
	switch {
	case r.take("["):
		r.expression(1)
		r.expect("]")
		r.expect("=")
	case r.is(NameToken) && tokenAt(r.tokens, r.at+1).is("="):
		r.name()
		r.expect("=")
	}
	r.expression(1)
}

func position(source string, offset int) (line, column int) {
	before := source[:offset]
	lineStart := strings.LastIndexByte(before, '\n') + 1
	return strings.Count(before, "\n") + 1, utf8.RuneCountInString(before[lineStart:]) + 1
}

func errUnsafe(file, source string, offset int, what string) error {
	line, column := position(source, offset)
	return &diag.Error{
		Msg:    "Cannot safely read map Lua: " + what,
		File:   file,
		Line:   line,
		Column: column,
		Hint:   "Re-save the map in World Editor to restore its generated Lua structure.",
	}
}
