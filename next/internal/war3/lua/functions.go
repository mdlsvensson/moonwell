package lua

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
)

// Call is a call statement of a bare global function: `Name(args)`, with the tokens of each argument.
type Call struct {
	Name  string
	Args  [][]Token
	Start int // byte offsets of the statement, with a `;` that directly follows it
	End   int
}

// Function is a top-level `function Name(...) ... end` and the call statements directly in its body.
type Function struct {
	Name     string
	Start    int // byte offsets of the whole declaration
	End      int
	EndStart int // where its closing `end` begins
	Calls    []Call
}

// Functions reads the top-level function declarations of Lua source and the call statements directly in each,
// without running it. Whatever it cannot read with certainty is an error that names the file, the line and the
// column.
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

// reader walks the tokens of a source by Lua's grammar, far enough to know where every statement begins and ends,
// and keeps the top-level functions it passes. It has no way back: at a token the grammar does not allow, fail
// panics with an unreadable, which Functions turns into its error.
type reader struct {
	source, file string
	tokens       []Token
	at           int // the token read next
	depth        int // how many blocks and expressions are open
	functions    []Function
	calls        []Call // the call statements of the top-level function being read
}

// unreadable carries the error out of the reader.
type unreadable struct{ err error }

// place is where a run of statements stands, which decides what the reader keeps of it.
type place uint8

const (
	inBlock    place = iota // in a block or a function inside something else: nothing is kept
	atTop                   // the source itself: its functions with a bare name are kept
	inFunction              // directly in a top-level function: its call statements are kept
)

// maxDepth is how deep blocks and expressions may nest. A source that goes deeper is refused, so that no source
// can exhaust the stack.
const maxDepth = 200

// unaryBinding is how tightly a unary operator binds: tighter than every binary operator but `^`.
const unaryBinding = 11

// binding is how tightly each binary operator binds its operands.
var binding = map[string]int{
	"or": 1, "and": 2,
	"<": 3, ">": 3, "<=": 3, ">=": 3, "~=": 3, "==": 3,
	"|": 4, "~": 5, "&": 6, "<<": 7, ">>": 7, "..": 8,
	"+": 9, "-": 9, "*": 10, "/": 10, "//": 10, "%": 10, "^": 12,
}

func (r *reader) more() bool { return r.at < len(r.tokens) }

// raw is the text of the token read next, or "" at the end.
func (r *reader) raw() string {
	if r.more() {
		return r.tokens[r.at].Raw
	}
	return ""
}

// is reports whether a token of the kind is read next.
func (r *reader) is(kind Kind) bool { return r.more() && r.tokens[r.at].Kind == kind }

// take steps over the keyword or symbol when it is read next, and reports whether it was.
func (r *reader) take(raw string) bool {
	if r.raw() != raw {
		return false
	}
	r.at++
	return true
}

// expect steps over the keyword or symbol, which must be read next, and returns its token.
func (r *reader) expect(raw string) Token {
	if r.raw() != raw {
		r.fail("expected '" + raw + "'")
	}
	r.at++
	return r.tokens[r.at-1]
}

// name steps over a name, which must be read next, and returns it.
func (r *reader) name() string {
	if !r.is(NameToken) || keywords[r.raw()] {
		r.fail("expected a name")
	}
	r.at++
	return r.tokens[r.at-1].Raw
}

// fail stops the reader with an error at the token read next, or at the end of the source.
func (r *reader) fail(what string) {
	offset := len(r.source)
	if r.more() {
		offset = r.tokens[r.at].Start
	}
	panic(unreadable{errUnsafe(r.file, r.source, offset, what)})
}

// enter counts one more open block or expression.
func (r *reader) enter() {
	r.depth++
	if r.depth > maxDepth {
		r.fail("nesting is too deep to establish safe edit boundaries")
	}
}

// statements reads statements up to one of the stop words, which it leaves, or up to the end of the source. Only
// the top level may end with the source.
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

// returned reads what follows a `return`: its values and a `;`, if any, and then nothing but the end of its block.
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

// block reads the statements of a block and the `end` that closes it.
func (r *reader) block() {
	r.statements(inBlock, "end")
	r.expect("end")
}

// statement reads one statement.
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
		// Whether a loop is there to break out of is for Lua to say.
	case r.take("goto"):
		r.name()
	case r.take("::"):
		r.name()
		r.expect("::")
	default:
		r.assignmentOrCall(where)
	}
}

// function reads a function statement after its `function`, which is at the offset start. A function with a bare
// name at the top level is kept, with the call statements directly in its body.
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

// functionName reads the name of a function statement: `a`, `a.b.c` or `a.b:c`. Only the first is bare.
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

// functionBody reads the parameters and the body of a function, and returns the `end` that closes it.
func (r *reader) functionBody(where place) Token {
	r.expect("(")
	if !r.take(")") {
		r.parameters()
		r.expect(")")
	}
	r.statements(where, "end")
	return r.expect("end")
}

// parameters reads the names of a parameter list, which may end with `...`.
func (r *reader) parameters() {
	for !r.take("...") {
		r.name()
		if !r.take(",") {
			return
		}
	}
}

// local reads a local function or the declaration of local names, after its `local`.
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

// conditional reads an `if` with its branches, after its `if`.
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

// loop reads a numeric or a generic `for`, after its `for`.
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

// assignmentOrCall reads a statement that starts with a prefix expression. Directly in a top-level function, a
// call of a bare name is kept.
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

// assignment reads the rest of an assignment, after its first target.
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

// withSemicolon extends a call statement over the `;` after it when only white space stands between the two. A
// comment there must outlive an edit of the call, so the `;` behind one is a statement of its own.
func (r *reader) withSemicolon(call Call) Call {
	if r.raw() != ";" {
		return call
	}
	semicolon := r.tokens[r.at]
	if strings.Trim(r.source[call.End:semicolon.Start], whiteSpace) != "" {
		return call
	}
	r.at++
	call.End = semicolon.End
	return call
}

// expressions reads a list of expressions.
func (r *reader) expressions() {
	r.expression(1)
	for r.take(",") {
		r.expression(1)
	}
}

// expression reads an operand and every binary operator after it that binds at least as tightly as minimum, each
// with its right side. A chain of one right-associative operator (`a .. b .. c`) is read in the loop, not by
// nesting, so a long one stays within the depth limit.
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

// operand reads what an operator applies to: a constant, a table, a function, a prefix expression, or a unary
// operator with its own operand.
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

// prefixed is what a statement needs to know of a prefix expression: whether it can be assigned to, and whether
// it is a call. A call keeps its identity only while it is a bare name called once: any suffix or operator after
// it makes it something else.
type prefixed struct {
	bare       string // the name, while nothing follows it
	assignable bool   // a name, an index or a field
	call       bool
	direct     *Call // the call of a bare name, when that is all the expression is
}

// prefix reads a prefix expression: a name or an expression in brackets, and every index, field and call after
// it.
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

// suffix reads one index, field, method call or call after a prefix expression that began at the offset start,
// and returns what the expression has become. found is false when none follows.
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

// arguments reads the arguments of a call and returns the tokens of each: a list in brackets, or one table or
// one string without them.
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

// argumentList reads the expressions of an argument list and its closing bracket, after the opening one.
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

// table reads a table constructor.
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

// field reads one field of a table constructor: `[key] = value`, `name = value` or a value.
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

// position is the 1-based line and column of a byte offset in a source. A column counts characters.
func position(source string, offset int) (line, column int) {
	before := source[:offset]
	lineStart := strings.LastIndexByte(before, '\n') + 1
	return strings.Count(before, "\n") + 1, utf8.RuneCountInString(before[lineStart:]) + 1
}

// ---- errors ----

// errUnsafe says that the Lua of a map has a place the reader cannot follow, so that no edit of it would be safe.
// what names the trouble, and offset is where in the source it is.
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
