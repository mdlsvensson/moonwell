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

func (t Token) isSymbol(symbol string) bool { return t.Kind == SymbolToken && t.Raw == symbol }

type Fault struct {
	Msg    string
	Offset int
}

func Tokenize(source string) ([]Token, *Fault) {
	l := &lexer{source: source, line: 1}
	for l.pos < len(source) {
		if !l.skipSpace() && !l.skipComment() {
			l.readToken()
		}
	}
	return l.tokens, l.fault
}

type lexer struct {
	source string
	pos    int
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
	c := l.source[l.pos]
	if !isSpace(c) {
		return false
	}
	if c == '\n' {
		l.line++
	}
	l.pos++
	return true
}

func (l *lexer) skipComment() bool {
	if !strings.HasPrefix(l.source[l.pos:], "--") {
		return false
	}
	if long, ok := findLongBracket(l.source, l.pos+2); ok {
		l.skipLong(long)
		return true
	}
	if length := strings.IndexAny(l.source[l.pos:], "\r\n"); length >= 0 {
		l.pos += length
	} else {
		l.pos = len(l.source)
	}
	return true
}

type bracket struct {
	content, contentEnd int
	end                 int
	closed              bool
}

func findLongBracket(source string, start int) (bracket, bool) {
	if start >= len(source) || source[start] != '[' {
		return bracket{}, false
	}
	level := countPrefix(source[start+1:], func(c byte) bool { return c == '=' })
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
		l.fail("unterminated long string or comment", l.pos)
	}
	l.line += strings.Count(l.source[l.pos:long.end], "\n")
	l.pos = long.end
}

func (l *lexer) readToken() {
	token := Token{Start: l.pos, Line: l.line}
	c, rest := l.source[l.pos], l.source[l.pos:]
	long, isLong := findLongBracket(l.source, l.pos)
	switch {
	case c == '"' || c == '\'':
		token.Kind = StringToken
		l.readString(&token)
	case isLong:
		token.Kind = StringToken
		token.Text = longBracketText(l.source[long.content:long.contentEnd])
		l.skipLong(long)
	case isNameStart(c):
		token.Kind = NameToken
		l.pos += countPrefix(rest, isNamePart)
	case isDigit(c) || (c == '.' && len(rest) > 1 && isDigit(rest[1])):
		token.Kind = NumberToken
		l.readNumeral()
	default:
		token.Kind = SymbolToken
		l.readSymbol()
	}
	token.End = l.pos
	token.Raw = l.source[token.Start:token.End]
	if token.Kind != StringToken {
		token.Text = token.Raw
	}
	l.tokens = append(l.tokens, token)
}

func (l *lexer) readString(token *Token) {
	quote, content := l.source[l.pos], l.pos+1
	for l.pos = content; l.pos < len(l.source); {
		switch c := l.source[l.pos]; c {
		case quote:
			token.Text = l.source[content:l.pos]
			l.pos++
			return
		case '\n', '\r':
			l.fail("unescaped newline in quoted string", token.Start)
			token.Text = l.source[content:l.pos]
			return
		case '\\':
			token.Escaped = true
			l.readEscape()
		default:
			l.pos++
		}
	}
	l.fail("unterminated quoted string", token.Start)
	token.Text = l.source[content:]
}

func (l *lexer) readEscape() {
	l.pos++
	rest := l.source[l.pos:]
	switch {
	case rest == "":
	case rest[0] == 'z':
		l.pos++
		for l.pos < len(l.source) && l.skipSpace() {
		}
	case strings.HasPrefix(rest, "\r\n"):
		l.line++
		l.pos += 2
	default:
		if rest[0] == '\n' {
			l.line++
		}
		l.pos++
	}
}

func longBracketText(content string) string {
	if rest, ok := strings.CutPrefix(content, "\r\n"); ok {
		return rest
	}
	return strings.TrimPrefix(content, "\n")
}

func (l *lexer) readNumeral() {
	rest := l.source[l.pos:]
	length := numeralLength(rest)
	if length == 0 || continuesNumeral(rest[length:]) {
		l.fail("invalid numeral", l.pos)
		length = malformedLength(rest)
	}
	l.pos += length
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
	whole := countPrefix(s, isDigits)
	after := s[whole:]
	if !strings.HasPrefix(after, ".") {
		return whole
	}
	fraction := countPrefix(after[1:], isDigits)
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
	digits := countPrefix(s[n:], isDigit)
	if digits == 0 {
		return 0
	}
	return n + digits
}

func continuesNumeral(after string) bool {
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

var longSymbols = []string{"...", "..", "//", "<<", ">>", "==", "~=", "<=", ">=", "::"}

const singleSymbols = "+-*/%^#&~|<>=(){}[];:,."

func (l *lexer) readSymbol() {
	rest := l.source[l.pos:]
	for _, symbol := range longSymbols {
		if strings.HasPrefix(rest, symbol) {
			l.pos += len(symbol)
			return
		}
	}
	if strings.IndexByte(singleSymbols, rest[0]) < 0 {
		l.fail("unsupported symbol", l.pos)
	}
	_, size := utf8.DecodeRuneInString(rest)
	l.pos += size
}

var keywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true, "end": true, "false": true, "for": true,
	"function": true, "goto": true, "if": true, "in": true, "local": true, "nil": true, "not": true, "or": true,
	"repeat": true, "return": true, "then": true, "true": true, "until": true, "while": true,
}

func isSpace(c byte) bool { return strings.IndexByte(fsx.ASCIISpace, c) >= 0 }

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isNamePart(c byte) bool { return isNameStart(c) || isDigit(c) }

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isSign(c byte) bool { return c == '+' || c == '-' }

func countPrefix(s string, is func(byte) bool) int {
	n := 0
	for n < len(s) && is(s[n]) {
		n++
	}
	return n
}
