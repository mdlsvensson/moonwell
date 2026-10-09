package lua

import (
	"slices"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

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

func (p *parser) parseStatements(where syntaxContext, stops ...string) {
	p.enterNested()
	for p.hasMore() && !slices.Contains(stops, p.peekRaw()) {
		if p.accept("return") {
			p.parseReturn(stops)
			break
		}
		p.parseStatement(where)
	}
	if where != atTop && !p.hasMore() {
		p.fail("unterminated block")
	}
	p.depth--
}

func (p *parser) parseReturn(stops []string) {
	ends := func() bool { return !p.hasMore() || slices.Contains(stops, p.peekRaw()) }
	if !ends() && p.peekRaw() != ";" {
		p.parseExpressionList()
	}
	p.accept(";")
	if !ends() {
		p.fail("return must end its block")
	}
}

func (p *parser) parseBlock() {
	p.parseStatements(inBlock, "end")
	p.expect("end")
}

func (p *parser) parseStatement(where syntaxContext) {
	start := p.tokens[p.pos].Start
	switch {
	case p.accept(";"):
	case p.accept("function"):
		p.parseFunction(start, where)
	case p.accept("local"):
		p.parseLocal()
	case p.accept("if"):
		p.parseIf()
	case p.accept("while"):
		p.parseExpression(1)
		p.expect("do")
		p.parseBlock()
	case p.accept("for"):
		p.parseLoop()
	case p.accept("do"):
		p.parseBlock()
	case p.accept("repeat"):
		p.parseStatements(inBlock, "until")
		p.expect("until")
		p.parseExpression(1)
	case p.accept("break"):
	case p.accept("goto"):
		p.expectName()
	case p.accept("::"):
		p.expectName()
		p.expect("::")
	default:
		p.parseAssignmentOrCall(where)
	}
}

func (p *parser) parseFunction(start int, where syntaxContext) {
	name, bare := p.parseFunctionName()
	if where != atTop || !bare {
		p.parseFunctionBody(inBlock)
		return
	}
	p.calls = []Call{}
	end := p.parseFunctionBody(inFunction)
	p.functions = append(p.functions, Function{
		Name: name, Start: start, End: end.End, EndStart: end.Start, Calls: p.calls,
	})
}

func (p *parser) parseFunctionName() (name string, bare bool) {
	name, bare = p.expectName(), true
	for p.accept(".") {
		bare = false
		p.expectName()
	}
	if p.accept(":") {
		bare = false
		p.expectName()
	}
	return name, bare
}

func (p *parser) parseFunctionBody(where syntaxContext) Token {
	p.expect("(")
	if !p.accept(")") {
		p.parseParameters()
		p.expect(")")
	}
	p.parseStatements(where, "end")
	return p.expect("end")
}

func (p *parser) parseParameters() {
	for !p.accept("...") {
		p.expectName()
		if !p.accept(",") {
			return
		}
	}
}

func (p *parser) parseLocal() {
	if p.accept("function") {
		p.expectName()
		p.parseFunctionBody(inBlock)
		return
	}
	p.expectName()
	for p.accept(",") {
		p.expectName()
	}
	if p.accept("=") {
		p.parseExpressionList()
	}
}

func (p *parser) parseIf() {
	for {
		p.parseExpression(1)
		p.expect("then")
		p.parseStatements(inBlock, "elseif", "else", "end")
		if !p.accept("elseif") {
			break
		}
	}
	if p.accept("else") {
		p.parseStatements(inBlock, "end")
	}
	p.expect("end")
}

func (p *parser) parseLoop() {
	p.expectName()
	if p.accept("=") {
		p.parseExpression(1)
		p.expect(",")
		p.parseExpression(1)
		if p.accept(",") {
			p.parseExpression(1)
		}
	} else {
		for p.accept(",") {
			p.expectName()
		}
		p.expect("in")
		p.parseExpressionList()
	}
	p.expect("do")
	p.parseBlock()
}

func (p *parser) parseAssignmentOrCall(where syntaxContext) {
	target := p.parsePrefix()
	switch {
	case p.peekRaw() == "=" || p.peekRaw() == ",":
		p.parseAssignment(target)
	case !target.call:
		p.fail("expected an assignment or call statement")
	case target.direct != nil && where == inFunction:
		p.calls = append(p.calls, p.includeSemicolon(*target.direct))
	}
}

func (p *parser) parseAssignment(first prefixExpr) {
	if !first.assignable {
		p.fail("invalid assignment target")
	}
	for p.accept(",") {
		if !p.parsePrefix().assignable {
			p.fail("invalid assignment target")
		}
	}
	p.expect("=")
	p.parseExpressionList()
}

func (p *parser) includeSemicolon(call Call) Call {
	if p.peekRaw() != ";" {
		return call
	}
	semicolon := p.tokens[p.pos]
	if fsx.TrimASCIISpace(p.source[call.End:semicolon.Start]) != "" {
		return call
	}
	p.pos++
	call.End = semicolon.End
	return call
}

func (p *parser) parseExpressionList() {
	p.parseExpression(1)
	for p.accept(",") {
		p.parseExpression(1)
	}
}

const unaryPrecedence = 11

var binaryPrecedence = map[string]int{
	"or": 1, "and": 2,
	"<": 3, ">": 3, "<=": 3, ">=": 3, "~=": 3, "==": 3,
	"|": 4, "~": 5, "&": 6, "<<": 7, ">>": 7, "..": 8,
	"+": 9, "-": 9, "*": 10, "/": 10, "//": 10, "%": 10, "^": 12,
}

func (p *parser) parseExpression(minPrecedence int) {
	p.enterNested()
	p.parseOperand()
	for {
		operator := binaryPrecedence[p.peekRaw()]
		if operator == 0 || operator < minPrecedence {
			break
		}
		p.pos++
		p.parseExpression(operator + 1)
	}
	p.depth--
}

func (p *parser) parseOperand() {
	switch raw := p.peekRaw(); {
	case raw == "not" || raw == "#" || raw == "-" || raw == "~":
		p.pos++
		p.parseExpression(unaryPrecedence)
	case p.accept("function"):
		p.parseFunctionBody(inBlock)
	case raw == "{":
		p.parseTable()
	case raw == "nil" || raw == "true" || raw == "false" || raw == "..." || p.peekIs(NumberToken) || p.peekIs(StringToken):
		p.pos++
	default:
		p.parsePrefix()
	}
}

type prefixExpr struct {
	bare       string
	assignable bool
	call       bool
	direct     *Call
}

func (p *parser) parsePrefix() prefixExpr {
	start := len(p.source)
	if p.hasMore() {
		start = p.tokens[p.pos].Start
	}
	var current prefixExpr
	if p.accept("(") {
		p.parseExpression(1)
		p.expect(")")
	} else {
		current = prefixExpr{bare: p.expectName(), assignable: true}
	}
	for {
		next, found := p.parseSuffix(current, start)
		if !found {
			return current
		}
		current = next
	}
}

func (p *parser) parseSuffix(before prefixExpr, start int) (after prefixExpr, found bool) {
	switch {
	case p.accept("["):
		p.parseExpression(1)
		p.expect("]")
		return prefixExpr{assignable: true}, true
	case p.accept("."):
		p.expectName()
		return prefixExpr{assignable: true}, true
	case p.accept(":"):
		p.expectName()
		p.parseArguments()
		return prefixExpr{call: true}, true
	case p.peekRaw() == "(" || p.peekRaw() == "{" || p.peekIs(StringToken):
		args := p.parseArguments()
		after = prefixExpr{call: true}
		if before.bare != "" {
			after.direct = &Call{Name: before.bare, Args: args, Start: start, End: p.tokens[p.pos-1].End}
		}
		return after, true
	}
	return before, false
}

func (p *parser) parseArguments() [][]Token {
	if p.accept("(") {
		return p.parseArgumentList()
	}
	start := p.pos
	switch {
	case p.peekRaw() == "{":
		p.parseTable()
	case p.peekIs(StringToken):
		p.pos++
	default:
		p.fail("expected call arguments")
	}
	return [][]Token{p.tokens[start:p.pos]}
}

func (p *parser) parseArgumentList() [][]Token {
	args := [][]Token{}
	if p.accept(")") {
		return args
	}
	for {
		start := p.pos
		p.parseExpression(1)
		args = append(args, p.tokens[start:p.pos])
		if !p.accept(",") {
			break
		}
	}
	p.expect(")")
	return args
}

func (p *parser) parseTable() {
	p.expect("{")
	for !p.accept("}") {
		p.parseTableField()
		if !p.accept(",") && !p.accept(";") {
			p.expect("}")
			return
		}
	}
}

func (p *parser) parseTableField() {
	switch {
	case p.accept("["):
		p.parseExpression(1)
		p.expect("]")
		p.expect("=")
	case p.peekIs(NameToken) && tokenAt(p.tokens, p.pos+1).isSymbol("="):
		p.expectName()
		p.expect("=")
	}
	p.parseExpression(1)
}

func (p *parser) hasMore() bool { return p.pos < len(p.tokens) }

func (p *parser) peekRaw() string {
	if p.hasMore() {
		return p.tokens[p.pos].Raw
	}
	return ""
}

func (p *parser) peekIs(kind Kind) bool { return p.hasMore() && p.tokens[p.pos].Kind == kind }

func (p *parser) accept(raw string) bool {
	if p.peekRaw() != raw {
		return false
	}
	p.pos++
	return true
}

func (p *parser) expect(raw string) Token {
	if p.peekRaw() != raw {
		p.fail("expected '" + raw + "'")
	}
	p.pos++
	return p.tokens[p.pos-1]
}

func (p *parser) expectName() string {
	if !p.peekIs(NameToken) || keywords[p.peekRaw()] {
		p.fail("expected a name")
	}
	p.pos++
	return p.tokens[p.pos-1].Raw
}

func (p *parser) fail(what string) {
	offset := len(p.source)
	if p.hasMore() {
		offset = p.tokens[p.pos].Start
	}
	panic(parseAbort{errUnsafe(p.displayPath, p.source, offset, what)})
}

func (p *parser) enterNested() {
	p.depth++
	if p.depth > maxDepth {
		p.fail("nesting is too deep to establish safe edit boundaries")
	}
}
