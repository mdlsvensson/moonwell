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

func ParseFunctions(source, displayPath string) (functions []Function, err error) {
	tokens, fault := Tokenize(source)
	if fault != nil {
		return nil, errUnsafe(displayPath, source, fault.Offset, fault.Msg)
	}
	r := &parser{source: source, displayPath: displayPath, tokens: tokens}
	defer func() {
		switch stopped := recover().(type) {
		case nil:
		case parseAbort:
			functions, err = nil, stopped.err
		default:
			panic(stopped)
		}
	}()
	r.parseStatements(atTop)
	return r.functions, nil
}

type parser struct {
	source, displayPath string
	tokens              []Token
	pos                 int
	depth               int
	functions           []Function
	calls               []Call
}

type parseAbort struct{ err error }

type syntaxContext uint8

const (
	inBlock syntaxContext = iota
	atTop
	inFunction
)

const maxDepth = 200

const unaryPrecedence = 11

var binaryPrecedence = map[string]int{
	"or": 1, "and": 2,
	"<": 3, ">": 3, "<=": 3, ">=": 3, "~=": 3, "==": 3,
	"|": 4, "~": 5, "&": 6, "<<": 7, ">>": 7, "..": 8,
	"+": 9, "-": 9, "*": 10, "/": 10, "//": 10, "%": 10, "^": 12,
}

func (r *parser) hasMore() bool { return r.pos < len(r.tokens) }

func (r *parser) peekRaw() string {
	if r.hasMore() {
		return r.tokens[r.pos].Raw
	}
	return ""
}

func (r *parser) peekIs(kind Kind) bool { return r.hasMore() && r.tokens[r.pos].Kind == kind }

func (r *parser) accept(raw string) bool {
	if r.peekRaw() != raw {
		return false
	}
	r.pos++
	return true
}

func (r *parser) expect(raw string) Token {
	if r.peekRaw() != raw {
		r.fail("expected '" + raw + "'")
	}
	r.pos++
	return r.tokens[r.pos-1]
}

func (r *parser) expectName() string {
	if !r.peekIs(NameToken) || keywords[r.peekRaw()] {
		r.fail("expected a name")
	}
	r.pos++
	return r.tokens[r.pos-1].Raw
}

func (r *parser) fail(what string) {
	offset := len(r.source)
	if r.hasMore() {
		offset = r.tokens[r.pos].Start
	}
	panic(parseAbort{errUnsafe(r.displayPath, r.source, offset, what)})
}

func (r *parser) enterNested() {
	r.depth++
	if r.depth > maxDepth {
		r.fail("nesting is too deep to establish safe edit boundaries")
	}
}

func (r *parser) parseStatements(where syntaxContext, stops ...string) {
	r.enterNested()
	for r.hasMore() && !slices.Contains(stops, r.peekRaw()) {
		if r.accept("return") {
			r.parseReturn(stops)
			break
		}
		r.parseStatement(where)
	}
	if where != atTop && !r.hasMore() {
		r.fail("unterminated block")
	}
	r.depth--
}

func (r *parser) parseReturn(stops []string) {
	ends := func() bool { return !r.hasMore() || slices.Contains(stops, r.peekRaw()) }
	if !ends() && r.peekRaw() != ";" {
		r.parseExpressionList()
	}
	r.accept(";")
	if !ends() {
		r.fail("return must end its block")
	}
}

func (r *parser) parseBlock() {
	r.parseStatements(inBlock, "end")
	r.expect("end")
}

func (r *parser) parseStatement(where syntaxContext) {
	start := r.tokens[r.pos].Start
	switch {
	case r.accept(";"):
	case r.accept("function"):
		r.parseFunction(start, where)
	case r.accept("local"):
		r.parseLocal()
	case r.accept("if"):
		r.parseIf()
	case r.accept("while"):
		r.parseExpression(1)
		r.expect("do")
		r.parseBlock()
	case r.accept("for"):
		r.parseLoop()
	case r.accept("do"):
		r.parseBlock()
	case r.accept("repeat"):
		r.parseStatements(inBlock, "until")
		r.expect("until")
		r.parseExpression(1)
	case r.accept("break"):
	case r.accept("goto"):
		r.expectName()
	case r.accept("::"):
		r.expectName()
		r.expect("::")
	default:
		r.parseAssignmentOrCall(where)
	}
}

func (r *parser) parseFunction(start int, where syntaxContext) {
	name, bare := r.parseFunctionName()
	if where != atTop || !bare {
		r.parseFunctionBody(inBlock)
		return
	}
	r.calls = []Call{}
	end := r.parseFunctionBody(inFunction)
	r.functions = append(r.functions, Function{
		Name: name, Start: start, End: end.End, EndStart: end.Start, Calls: r.calls,
	})
}

func (r *parser) parseFunctionName() (name string, bare bool) {
	name, bare = r.expectName(), true
	for r.accept(".") {
		bare = false
		r.expectName()
	}
	if r.accept(":") {
		bare = false
		r.expectName()
	}
	return name, bare
}

func (r *parser) parseFunctionBody(where syntaxContext) Token {
	r.expect("(")
	if !r.accept(")") {
		r.parseParameters()
		r.expect(")")
	}
	r.parseStatements(where, "end")
	return r.expect("end")
}

func (r *parser) parseParameters() {
	for !r.accept("...") {
		r.expectName()
		if !r.accept(",") {
			return
		}
	}
}

func (r *parser) parseLocal() {
	if r.accept("function") {
		r.expectName()
		r.parseFunctionBody(inBlock)
		return
	}
	r.expectName()
	for r.accept(",") {
		r.expectName()
	}
	if r.accept("=") {
		r.parseExpressionList()
	}
}

func (r *parser) parseIf() {
	for {
		r.parseExpression(1)
		r.expect("then")
		r.parseStatements(inBlock, "elseif", "else", "end")
		if !r.accept("elseif") {
			break
		}
	}
	if r.accept("else") {
		r.parseStatements(inBlock, "end")
	}
	r.expect("end")
}

func (r *parser) parseLoop() {
	r.expectName()
	if r.accept("=") {
		r.parseExpression(1)
		r.expect(",")
		r.parseExpression(1)
		if r.accept(",") {
			r.parseExpression(1)
		}
	} else {
		for r.accept(",") {
			r.expectName()
		}
		r.expect("in")
		r.parseExpressionList()
	}
	r.expect("do")
	r.parseBlock()
}

func (r *parser) parseAssignmentOrCall(where syntaxContext) {
	target := r.parsePrefix()
	switch {
	case r.peekRaw() == "=" || r.peekRaw() == ",":
		r.parseAssignment(target)
	case !target.call:
		r.fail("expected an assignment or call statement")
	case target.direct != nil && where == inFunction:
		r.calls = append(r.calls, r.includeSemicolon(*target.direct))
	}
}

func (r *parser) parseAssignment(first prefixExpr) {
	if !first.assignable {
		r.fail("invalid assignment target")
	}
	for r.accept(",") {
		if !r.parsePrefix().assignable {
			r.fail("invalid assignment target")
		}
	}
	r.expect("=")
	r.parseExpressionList()
}

func (r *parser) includeSemicolon(call Call) Call {
	if r.peekRaw() != ";" {
		return call
	}
	semicolon := r.tokens[r.pos]
	if fsx.TrimASCIISpace(r.source[call.End:semicolon.Start]) != "" {
		return call
	}
	r.pos++
	call.End = semicolon.End
	return call
}

func (r *parser) parseExpressionList() {
	r.parseExpression(1)
	for r.accept(",") {
		r.parseExpression(1)
	}
}

func (r *parser) parseExpression(minPrecedence int) {
	r.enterNested()
	r.parseOperand()
	for {
		operator := binaryPrecedence[r.peekRaw()]
		if operator == 0 || operator < minPrecedence {
			break
		}
		r.pos++
		r.parseExpression(operator + 1)
	}
	r.depth--
}

func (r *parser) parseOperand() {
	switch raw := r.peekRaw(); {
	case raw == "not" || raw == "#" || raw == "-" || raw == "~":
		r.pos++
		r.parseExpression(unaryPrecedence)
	case r.accept("function"):
		r.parseFunctionBody(inBlock)
	case raw == "{":
		r.parseTable()
	case raw == "nil" || raw == "true" || raw == "false" || raw == "..." || r.peekIs(NumberToken) || r.peekIs(StringToken):
		r.pos++
	default:
		r.parsePrefix()
	}
}

type prefixExpr struct {
	bare       string
	assignable bool
	call       bool
	direct     *Call
}

func (r *parser) parsePrefix() prefixExpr {
	start := len(r.source)
	if r.hasMore() {
		start = r.tokens[r.pos].Start
	}
	var current prefixExpr
	if r.accept("(") {
		r.parseExpression(1)
		r.expect(")")
	} else {
		current = prefixExpr{bare: r.expectName(), assignable: true}
	}
	for {
		next, found := r.parseSuffix(current, start)
		if !found {
			return current
		}
		current = next
	}
}

func (r *parser) parseSuffix(before prefixExpr, start int) (after prefixExpr, found bool) {
	switch {
	case r.accept("["):
		r.parseExpression(1)
		r.expect("]")
		return prefixExpr{assignable: true}, true
	case r.accept("."):
		r.expectName()
		return prefixExpr{assignable: true}, true
	case r.accept(":"):
		r.expectName()
		r.parseArguments()
		return prefixExpr{call: true}, true
	case r.peekRaw() == "(" || r.peekRaw() == "{" || r.peekIs(StringToken):
		args := r.parseArguments()
		after = prefixExpr{call: true}
		if before.bare != "" {
			after.direct = &Call{Name: before.bare, Args: args, Start: start, End: r.tokens[r.pos-1].End}
		}
		return after, true
	}
	return before, false
}

func (r *parser) parseArguments() [][]Token {
	if r.accept("(") {
		return r.parseArgumentList()
	}
	start := r.pos
	switch {
	case r.peekRaw() == "{":
		r.parseTable()
	case r.peekIs(StringToken):
		r.pos++
	default:
		r.fail("expected call arguments")
	}
	return [][]Token{r.tokens[start:r.pos]}
}

func (r *parser) parseArgumentList() [][]Token {
	args := [][]Token{}
	if r.accept(")") {
		return args
	}
	for {
		start := r.pos
		r.parseExpression(1)
		args = append(args, r.tokens[start:r.pos])
		if !r.accept(",") {
			break
		}
	}
	r.expect(")")
	return args
}

func (r *parser) parseTable() {
	r.expect("{")
	for !r.accept("}") {
		r.parseTableField()
		if !r.accept(",") && !r.accept(";") {
			r.expect("}")
			return
		}
	}
}

func (r *parser) parseTableField() {
	switch {
	case r.accept("["):
		r.parseExpression(1)
		r.expect("]")
		r.expect("=")
	case r.peekIs(NameToken) && tokenAt(r.tokens, r.pos+1).isSymbol("="):
		r.expectName()
		r.expect("=")
	}
	r.parseExpression(1)
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
