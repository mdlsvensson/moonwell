// Package lua reads Lua 5.3 source without running it, writes Lua literals, and splices edits into a source. It
// takes source text and returns tokens, what scanners find in them, and the source with edits made. It knows
// nothing of modules, bundles or maps beyond war3map.lua's shape.
package lua

import (
	"strings"
	"unicode/utf8"
)

// Kind is the kind of a token. Comments and white space are not tokens.
type Kind uint8

const (
	NameToken Kind = iota
	NumberToken
	StringToken
	SymbolToken
)

// Token is one token of Lua source.
type Token struct {
	Kind Kind
	// Raw is the token's source text.
	Raw string
	// Text is a string's content: what stands between the quotes, with its escapes as written, or a long string's
	// content without its first line break. For other kinds it is Raw.
	Text    string
	Start   int // byte offsets in the source
	End     int
	Line    int  // 1-based line of the token's start
	Escaped bool // a quoted string that has a backslash
}

// is reports whether the token is the symbol.
func (t Token) is(symbol string) bool { return t.Kind == SymbolToken && t.Raw == symbol }

// Fault is the first malformed token of a source.
type Fault struct {
	Msg    string
	Offset int // byte offset
}

// whiteSpace is what Lua skips between tokens.
const whiteSpace = " \t\n\v\f\r"

// longSymbols are the symbols of several characters, a longer one before the one it starts with.
var longSymbols = []string{"...", "..", "//", "<<", ">>", "==", "~=", "<=", ">=", "::"}

const singleSymbols = "+-*/%^#&~|<>=(){}[];:,."

var keywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true, "end": true, "false": true, "for": true,
	"function": true, "goto": true, "if": true, "in": true, "local": true, "nil": true, "not": true, "or": true,
	"repeat": true, "return": true, "then": true, "true": true, "until": true, "while": true,
}

func isSpace(c byte) bool { return strings.IndexByte(whiteSpace, c) >= 0 }

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isNamePart(c byte) bool { return isNameStart(c) || isDigit(c) }

// run returns how many bytes at the start of s satisfy is.
func run(s string, is func(byte) bool) int {
	n := 0
	for n < len(s) && is(s[n]) {
		n++
	}
	return n
}

// Tokenize splits Lua source into tokens. It never fails: the first malformed token is returned as the fault, and
// reading goes on in the way that loses least. An unterminated quoted string ends at its line break, an unterminated
// long bracket runs to the end, a bad numeral takes the letters, digits and dots that follow it, and a character
// that Lua has no symbol for is a symbol of its own.
func Tokenize(source string) ([]Token, *Fault) {
	l := &lexer{source: source, line: 1}
	for l.at < len(source) {
		if !l.skipSpace() && !l.skipComment() {
			l.token()
		}
	}
	return l.tokens, l.fault
}

// lexer is a place in a source, with the tokens read up to it.
type lexer struct {
	source string
	at     int // byte offset of what is read next
	line   int // 1-based line of at
	tokens []Token
	fault  *Fault
}

// fail notes a malformed token at the offset. Only the first is kept.
func (l *lexer) fail(msg string, offset int) {
	if l.fault == nil {
		l.fault = &Fault{Msg: msg, Offset: offset}
	}
}

// skipSpace steps over one white space character and reports whether there was one.
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

// skipComment steps over a comment and reports whether there was one. A short comment ends before its line break.
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

// bracket is a long bracket: `[[ ... ]]` or, with a level, `[==[ ... ]==]`.
type bracket struct {
	content, contentEnd int // the text between the opening and the closing bracket
	end                 int // where the closing bracket ends
	closed              bool
}

// longBracket finds the long bracket that opens at start, when one does. One that never closes runs to the end of
// the source.
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

// skipLong steps over the long bracket that opens here, as a comment or as a string, and counts its lines. One that
// never closes is a fault at the offset where it opens.
func (l *lexer) skipLong(long bracket) {
	if !long.closed {
		l.fail("unterminated long string or comment", l.at)
	}
	l.line += strings.Count(l.source[l.at:long.end], "\n")
	l.at = long.end
}

// token reads the token that starts here.
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

// quoted reads a quoted string into the token: its text and whether it has an escape. A string that its line ends
// before the closing quote stops at the line break, and one that the source ends in runs to the end.
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

// escape steps over a backslash and the byte it escapes, so that an escaped quote or line break does not end the
// string. `\z` also takes the white space that follows it, and a backslash before a return and a line feed takes
// both. No byte of a character of several bytes is a quote, a backslash or a line break, so the rest of an escaped
// one needs no care.
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

// longText is the text of a long string with the content given: Lua leaves out a line break that directly follows
// the opening bracket.
func longText(content string) string {
	if rest, ok := strings.CutPrefix(content, "\r\n"); ok {
		return rest
	}
	return strings.TrimPrefix(content, "\n")
}

// numeral reads a numeral. A malformed one is a fault, and takes the letters, digits and dots that follow it.
func (l *lexer) numeral() {
	rest := l.source[l.at:]
	length := numeralLength(rest)
	if length == 0 || runsOn(rest[length:]) {
		l.fail("invalid numeral", l.at)
		length = malformedLength(rest)
	}
	l.at += length
}

// numeralLength is the length of the Lua numeral at the start of s, or 0 when there is none: digits with a dot
// among them, then an exponent. After `0x` the digits are hexadecimal and the exponent letter is `p`.
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

// mantissaLength is the length of the digits at the start of s, with one dot before, among or after them; 0 when
// there is no digit. A dot after digits belongs to the numeral unless another dot follows it: `1..2` is `1`, `..`,
// `2`.
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

// exponentLength is the length of the exponent at the start of s: one of the letters, a sign if there is one, and
// decimal digits. Without a digit there is no exponent, and the length is 0.
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

// runsOn reports whether what stands after a numeral would be part of it: a letter, or a dot that does not start
// `..`.
func runsOn(after string) bool {
	if after == "" {
		return false
	}
	return isNameStart(after[0]) || (after[0] == '.' && !strings.HasPrefix(after, ".."))
}

// malformedLength is how much of s a malformed numeral takes: its first byte, then every letter, digit and dot,
// and a sign that follows an exponent letter.
func malformedLength(s string) int {
	n := 1
	for n < len(s) && (isNamePart(s[n]) || s[n] == '.' || (isSign(s[n]) && strings.IndexByte("eEpP", s[n-1]) >= 0)) {
		n++
	}
	return n
}

// symbol reads a symbol: the longest that Lua has here, or else one character. A character that Lua has no symbol
// for is a fault.
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

// simpleTokens returns the tokens of a source as the scanners that only look for names and punctuation want them:
// without the numbers, and with every symbol but a run of dots split into its characters.
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

// characters splits a symbol into one symbol for each of its characters.
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

// tokenAt is tokens[i], or a symbol that no source has when there is no such token.
func tokenAt(tokens []Token, i int) Token {
	if i < 0 || i >= len(tokens) {
		return Token{Kind: SymbolToken}
	}
	return tokens[i]
}
