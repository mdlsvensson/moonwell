// Package luasrc reads Lua 5.3 source without running it: the modules a file requires, the globals it defines, and
// the functions and call statements of the war3map.lua World Editor writes.
package luasrc

import (
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/text"
)

// Kind is the kind of a token. Comments and white space are not tokens.
type Kind uint8

const (
	Name Kind = iota
	Number
	String
	Symbol
)

// Token is one token of Lua source.
type Token struct {
	Kind Kind
	// Raw is the token's source text.
	Raw string
	// Text is a string's content: what stands between the quotes, with its escapes as written, or a long string's
	// content without its first line break. For other kinds it is Raw.
	Text    string
	Start   int // byte offset of the token in the source
	End     int
	Line    int  // 1-based line of the token's start
	Escaped bool // a quoted string that has a backslash
}

// Fault is the first malformed token of a source.
type Fault struct {
	Msg    string
	Offset int // byte offset
}

var symbols = []string{"...", "..", "//", "<<", ">>", "==", "~=", "<=", ">=", "::"}

const singleSymbols = "+-*/%^#&~|<>=(){}[];:,."

func isNameStart(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isNamePart(c byte) bool { return isNameStart(c) || isDigit(c) }

// digits returns how many bytes at the start of s satisfy is.
func digits(s string, is func(byte) bool) int {
	n := 0
	for n < len(s) && is(s[n]) {
		n++
	}
	return n
}

// numeralLength is the length of the Lua numeral at the start of s, or 0 when there is none. A dot belongs to the
// numeral unless another dot follows it: `1..2` is `1`, `..`, `2`.
func numeralLength(s string) int {
	isDigits, exponent, n := isDigit, "eE", 0
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		isDigits, exponent, n = isHexDigit, "pP", 2
	}
	whole := digits(s[n:], isDigits)
	n += whole
	switch {
	case whole > 0:
		if n < len(s) && s[n] == '.' && !(n+1 < len(s) && s[n+1] == '.') {
			n++
			n += digits(s[n:], isDigits)
		}
	case n < len(s) && s[n] == '.' && digits(s[n+1:], isDigits) > 0:
		n++
		n += digits(s[n:], isDigits)
	default:
		return 0
	}
	if n < len(s) && strings.IndexByte(exponent, s[n]) >= 0 {
		m := n + 1
		if m < len(s) && (s[m] == '+' || s[m] == '-') {
			m++
		}
		if count := digits(s[m:], isDigit); count > 0 {
			n = m + count
		}
	}
	return n
}

// Tokenize splits Lua source into tokens. It never fails: the first malformed token is returned as the fault, and
// lexing goes on in the way that loses least. An unterminated quoted string ends at its line break, an unterminated
// long bracket runs to the end, a bad numeral takes the letters, digits and dots that follow it, and an unknown
// character is a symbol of its own.
func Tokenize(source string) ([]Token, *Fault) {
	var tokens []Token
	var fault *Fault
	fail := func(msg string, offset int) {
		if fault == nil {
			fault = &Fault{Msg: msg, Offset: offset}
		}
	}
	n := len(source)
	line := 1

	// longBracket reports whether a long bracket ("[[", "[==[") opens at start, and where its content begins.
	longBracket := func(start int) (content, level int, ok bool) {
		j := start + 1
		for j < n && source[j] == '=' {
			j++
		}
		if j < n && source[j] == '[' {
			return j + 1, j - start - 1, true
		}
		return 0, 0, false
	}
	// readLong finds the end of a long bracket's content and of the bracket itself.
	readLong := func(content, level int) (contentEnd, end int, closed bool) {
		closing := "]" + strings.Repeat("=", level) + "]"
		found := strings.Index(source[content:], closing)
		if found < 0 {
			return n, n, false
		}
		return content + found, content + found + len(closing), true
	}

	at := 0
	for at < n {
		start := at
		c := source[at]
		r, size := rune(c), 1
		if c >= utf8.RuneSelf {
			r, size = utf8.DecodeRuneInString(source[at:])
		}
		if text.IsSpace(r) {
			if r == '\n' {
				line++
			}
			at += size
			continue
		}
		if strings.HasPrefix(source[at:], "--") {
			if at+2 < n && source[at+2] == '[' {
				if content, level, ok := longBracket(at + 2); ok {
					_, end, closed := readLong(content, level)
					if !closed {
						fail("unterminated long string or comment", start)
					}
					line += strings.Count(source[at:end], "\n")
					at = end
					continue
				}
			}
			at += 2
			for at < n && source[at] != '\n' && source[at] != '\r' {
				at++
			}
			continue
		}

		token := Token{Start: start, Line: line}
		switch {
		case c == '"' || c == '\'':
			token.Kind = String
			at++
			closed := false
			for at < n {
				ch := source[at]
				if ch == c {
					closed = true
					break
				}
				if ch == '\n' || ch == '\r' {
					fail("unescaped newline in quoted string", start)
					break
				}
				if ch != '\\' {
					at++
					continue
				}
				token.Escaped = true
				at++
				if at >= n {
					break
				}
				if source[at] == 'z' {
					at++
					for at < n {
						space, spaceSize := utf8.DecodeRuneInString(source[at:])
						if !text.IsSpace(space) {
							break
						}
						if space == '\n' {
							line++
						}
						at += spaceSize
					}
					continue
				}
				if source[at] == '\r' && at+1 < n && source[at+1] == '\n' {
					at++
				}
				if source[at] == '\n' {
					line++
				}
				_, escapedSize := utf8.DecodeRuneInString(source[at:])
				at += escapedSize
			}
			token.Text = source[start+1 : at]
			if closed {
				at++
			} else if at >= n {
				fail("unterminated quoted string", start)
			}
		case c == '[' && func() bool { _, _, ok := longBracket(at); return ok }():
			token.Kind = String
			content, level, _ := longBracket(at)
			contentEnd, end, closed := readLong(content, level)
			if !closed {
				fail("unterminated long string or comment", start)
			}
			token.Text = source[content:contentEnd]
			if strings.HasPrefix(token.Text, "\r\n") {
				token.Text = token.Text[2:]
			} else if strings.HasPrefix(token.Text, "\n") {
				token.Text = token.Text[1:]
			}
			line += strings.Count(source[at:end], "\n")
			at = end
		case isNameStart(c):
			token.Kind = Name
			at += digits(source[at:], isNamePart)
		case isDigit(c) || (c == '.' && at+1 < n && isDigit(source[at+1])):
			token.Kind = Number
			length := numeralLength(source[at:])
			next := at + length
			bad := length == 0 || (next < n &&
				(isNameStart(source[next]) || (source[next] == '.' && !(next+1 < n && source[next+1] == '.'))))
			if bad {
				fail("invalid numeral", start)
				next = at + 1
				for next < n && (isNamePart(source[next]) || source[next] == '.' ||
					((source[next] == '+' || source[next] == '-') && strings.IndexByte("eEpP", source[next-1]) >= 0)) {
					next++
				}
			}
			at = next
		default:
			token.Kind = Symbol
			matched := false
			for _, symbol := range symbols {
				if strings.HasPrefix(source[at:], symbol) {
					at += len(symbol)
					matched = true
					break
				}
			}
			if !matched {
				if c >= utf8.RuneSelf || strings.IndexByte(singleSymbols, c) < 0 {
					fail("unsupported symbol", start)
				}
				at += size
			}
		}
		token.End = at
		token.Raw = source[start:at]
		if token.Kind != String {
			token.Text = token.Raw
		}
		tokens = append(tokens, token)
	}
	return tokens, fault
}

// A simple token, as the scanners that only look for names and punctuation want it: numbers are left out, and every
// symbol except a run of dots is one character.
type simpleKind uint8

const (
	simpleName simpleKind = iota
	simpleString
	simplePunct
)

type simpleToken struct {
	kind    simpleKind
	value   string
	line    int
	escaped bool
}

func simpleTokens(source string) []simpleToken {
	tokens, _ := Tokenize(source)
	out := make([]simpleToken, 0, len(tokens))
	for _, token := range tokens {
		switch token.Kind {
		case Name:
			out = append(out, simpleToken{kind: simpleName, value: token.Raw, line: token.Line})
		case String:
			out = append(out, simpleToken{simpleString, token.Text, token.Line, token.Escaped})
		case Symbol:
			if strings.Trim(token.Raw, ".") == "" {
				out = append(out, simpleToken{kind: simplePunct, value: token.Raw, line: token.Line})
				continue
			}
			for _, r := range token.Raw {
				out = append(out, simpleToken{kind: simplePunct, value: string(r), line: token.Line})
			}
		}
	}
	return out
}

func (t simpleToken) isPunct(value string) bool { return t.kind == simplePunct && t.value == value }

var keywords = map[string]bool{
	"and": true, "break": true, "do": true, "else": true, "elseif": true, "end": true, "false": true, "for": true,
	"function": true, "goto": true, "if": true, "in": true, "local": true, "nil": true, "not": true, "or": true,
	"repeat": true, "return": true, "then": true, "true": true, "until": true, "while": true,
}
