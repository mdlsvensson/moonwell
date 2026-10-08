package lua

import (
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type Kind uint8

const (
	NameToken Kind = iota
	NumberToken
	StringToken
	SymbolToken
)

type Token struct {
	Kind    Kind
	Raw     string
	Text    string
	Start   int
	End     int
	Line    int
	Escaped bool
}

func (t Token) is(symbol string) bool { return t.Kind == SymbolToken && t.Raw == symbol }

type Fault struct {
	Msg    string
	Offset int
}

var longSymbols = []string{"...", "..", "//", "<<", ">>", "==", "~=", "<=", ">=", "::"}

const singleSymbols = "+-*/%^#&~|<>=(){}[];:,."

var keywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true, "end": true, "false": true, "for": true,
	"function": true, "goto": true, "if": true, "in": true, "local": true, "nil": true, "not": true, "or": true,
	"repeat": true, "return": true, "then": true, "true": true, "until": true, "while": true,
}

func isSpace(c byte) bool { return strings.IndexByte(fsx.ASCIISpace, c) >= 0 }

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isNamePart(c byte) bool { return isNameStart(c) || isDigit(c) }

func run(s string, is func(byte) bool) int {
	n := 0
	for n < len(s) && is(s[n]) {
		n++
	}
	return n
}

func Tokenize(source string) ([]Token, *Fault) {
	l := &lexer{source: source, line: 1}
	for l.at < len(source) {
		if !l.skipSpace() && !l.skipComment() {
			l.token()
		}
	}
	return l.tokens, l.fault
}

type lexer struct {
	source string
	at     int
	line   int
	tokens []Token
	fault  *Fault
}

func (l *lexer) fail(msg string, offset int) {
	if l.fault == nil {
		l.fault = &Fault{Msg: msg, Offset: offset}
	}
}

func (l *lexer) skipSpace() bool {
	c := l.source[l.at]
	if !isSpace(c) {
		return false
	}
	if c == '\n' {
		l.line++
	}
	l.at++
	return true
}

func (l *lexer) skipComment() bool {
	if !strings.HasPrefix(l.source[l.at:], "--") {
		return false
	}
	if long, ok := longBracket(l.source, l.at+2); ok {
		l.skipLong(long)
		return true
	}
	if length := strings.IndexAny(l.source[l.at:], "\r\n"); length >= 0 {
		l.at += length
	} else {
		l.at = len(l.source)
	}
	return true
}

type bracket struct {
	content, contentEnd int
	end                 int
	closed              bool
}

func longBracket(source string, start int) (bracket, bool) {
	if start >= len(source) || source[start] != '[' {
		return bracket{}, false
	}
	level := run(source[start+1:], func(c byte) bool { return c == '=' })
	content := start + 1 + level + 1
	if content > len(source) || source[content-1] != '[' {
		return bracket{}, false
	}
	closing := "]" + strings.Repeat("=", level) + "]"
	length := strings.Index(source[content:], closing)
	if length < 0 {
		return bracket{content: content, contentEnd: len(source), end: len(source)}, true
	}
	contentEnd := content + length
	return bracket{content: content, contentEnd: contentEnd, end: contentEnd + len(closing), closed: true}, true
}

func (l *lexer) skipLong(long bracket) {
	if !long.closed {
		l.fail("unterminated long string or comment", l.at)
	}
	l.line += strings.Count(l.source[l.at:long.end], "\n")
	l.at = long.end
}

func (l *lexer) token() {
	token := Token{Start: l.at, Line: l.line}
	c, rest := l.source[l.at], l.source[l.at:]
	long, isLong := longBracket(l.source, l.at)
	switch {
	case c == '"' || c == '\'':
		token.Kind = StringToken
		l.quoted(&token)
	case isLong:
		token.Kind = StringToken
		token.Text = longText(l.source[long.content:long.contentEnd])
		l.skipLong(long)
	case isNameStart(c):
		token.Kind = NameToken
		l.at += run(rest, isNamePart)
	case isDigit(c) || (c == '.' && len(rest) > 1 && isDigit(rest[1])):
		token.Kind = NumberToken
		l.numeral()
	default:
		token.Kind = SymbolToken
		l.symbol()
	}
	token.End = l.at
	token.Raw = l.source[token.Start:token.End]
	if token.Kind != StringToken {
		token.Text = token.Raw
	}
	l.tokens = append(l.tokens, token)
}

func (l *lexer) quoted(token *Token) {
	quote, content := l.source[l.at], l.at+1
	for l.at = content; l.at < len(l.source); {
		switch c := l.source[l.at]; c {
		case quote:
			token.Text = l.source[content:l.at]
			l.at++
			return
		case '\n', '\r':
			l.fail("unescaped newline in quoted string", token.Start)
			token.Text = l.source[content:l.at]
			return
		case '\\':
			token.Escaped = true
			l.escape()
		default:
			l.at++
		}
	}
	l.fail("unterminated quoted string", token.Start)
	token.Text = l.source[content:]
}

func (l *lexer) escape() {
	l.at++
	rest := l.source[l.at:]
	switch {
	case rest == "":
	case rest[0] == 'z':
		l.at++
		for l.at < len(l.source) && l.skipSpace() {
		}
	case strings.HasPrefix(rest, "\r\n"):
		l.line++
		l.at += 2
	default:
		if rest[0] == '\n' {
			l.line++
		}
		l.at++
	}
}

func longText(content string) string {
	if rest, ok := strings.CutPrefix(content, "\r\n"); ok {
		return rest
	}
	return strings.TrimPrefix(content, "\n")
}

func (l *lexer) numeral() {
	rest := l.source[l.at:]
	length := numeralLength(rest)
	if length == 0 || runsOn(rest[length:]) {
		l.fail("invalid numeral", l.at)
		length = malformedLength(rest)
	}
	l.at += length
}

func numeralLength(s string) int {
	isDigits, exponent, prefix := isDigit, "eE", 0
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		isDigits, exponent, prefix = isHexDigit, "pP", 2
	}
	mantissa := mantissaLength(s[prefix:], isDigits)
	if mantissa == 0 {
		return 0
	}
	n := prefix + mantissa
	return n + exponentLength(s[n:], exponent)
}

func mantissaLength(s string, isDigits func(byte) bool) int {
	whole := run(s, isDigits)
	after := s[whole:]
	if !strings.HasPrefix(after, ".") {
		return whole
	}
	fraction := run(after[1:], isDigits)
	switch {
	case whole > 0 && strings.HasPrefix(after, ".."):
		return whole
	case whole == 0 && fraction == 0:
		return 0
	}
	return whole + 1 + fraction
}

func exponentLength(s, letters string) int {
	if s == "" || strings.IndexByte(letters, s[0]) < 0 {
		return 0
	}
	n := 1
	if n < len(s) && isSign(s[n]) {
		n++
	}
	digits := run(s[n:], isDigit)
	if digits == 0 {
		return 0
	}
	return n + digits
}

func isSign(c byte) bool { return c == '+' || c == '-' }

func runsOn(after string) bool {
	if after == "" {
		return false
	}
	return isNameStart(after[0]) || (after[0] == '.' && !strings.HasPrefix(after, ".."))
}

func malformedLength(s string) int {
	n := 1
	for n < len(s) && (isNamePart(s[n]) || s[n] == '.' || (isSign(s[n]) && strings.IndexByte("eEpP", s[n-1]) >= 0)) {
		n++
	}
	return n
}

func (l *lexer) symbol() {
	rest := l.source[l.at:]
	for _, symbol := range longSymbols {
		if strings.HasPrefix(rest, symbol) {
			l.at += len(symbol)
			return
		}
	}
	if strings.IndexByte(singleSymbols, rest[0]) < 0 {
		l.fail("unsupported symbol", l.at)
	}
	_, size := utf8.DecodeRuneInString(rest)
	l.at += size
}

func simpleTokens(source string) []Token {
	tokens, _ := Tokenize(source)
	simple := make([]Token, 0, len(tokens))
	for _, token := range tokens {
		switch {
		case token.Kind == NumberToken:
		case token.Kind == SymbolToken && strings.Trim(token.Raw, ".") != "":
			simple = append(simple, characters(token)...)
		default:
			simple = append(simple, token)
		}
	}
	return simple
}

func characters(symbol Token) []Token {
	var split []Token
	for offset := 0; offset < len(symbol.Raw); {
		_, size := utf8.DecodeRuneInString(symbol.Raw[offset:])
		piece := symbol
		piece.Raw = symbol.Raw[offset : offset+size]
		piece.Text = piece.Raw
		piece.Start = symbol.Start + offset
		piece.End = piece.Start + size
		split = append(split, piece)
		offset += size
	}
	return split
}

func tokenAt(tokens []Token, i int) Token {
	if i < 0 || i >= len(tokens) {
		return Token{Kind: SymbolToken}
	}
	return tokens[i]
}
