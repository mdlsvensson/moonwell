package fsx

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	byteOrderMark = "\xEF\xBB\xBF"         // U+FEFF in UTF-8
	replacement   = string(utf8.RuneError) // U+FFFD
)

// ASCIISpace is the white space of ASCII: the six characters that C's isspace takes, and with it Lua, YueScript
// and the game's text files. Where Moonwell reads what they read, and where it tidies what a program printed,
// these are white space and nothing outside ASCII is: no no-break space, no line separator, no byte order mark.
const ASCIISpace = " \t\n\v\f\r"

// TrimASCIISpace is a text without the white space of ASCII at its start and at its end.
func TrimASCIISpace(text string) string { return strings.Trim(text, ASCIISpace) }

// WithoutMark is a text without the byte order mark at its start. It is the one rule for the mark: a mark at the
// very start of a text is no part of the text and is dropped, once; a mark anywhere else, a second one behind
// the first among them, is content like any other.
//
// The rule is for a text that is read and looked into: a source, the map's script and its text files,
// .gitignore, .luarc.json, a library's own file, what a program printed, and a string of war3map.w3i, of an
// object file or of a model. It is not for a value that is written back as it was read. A path of war3map.imp
// is one: its bytes are the name of a file in the map, the index is written anew from the paths that were read,
// and so a path keeps a mark at its start.
func WithoutMark[T string | []byte](text T) T {
	if len(text) >= len(byteOrderMark) && string(text[:len(byteOrderMark)]) == byteOrderMark {
		return text[len(byteOrderMark):]
	}
	return text
}

// DecodeText reads bytes as UTF-8 text: invalid bytes become U+FFFD and a leading byte order mark is dropped.
// Neighbouring invalid bytes become one U+FFFD together.
func DecodeText(b []byte) string {
	return WithoutMark(strings.ToValidUTF8(string(b), replacement))
}

// TextWithMark splits a file's bytes into a leading byte order mark ("" when there is none) and the text after
// it, so that the text can be changed and the mark put back. ok is false when the bytes are not valid UTF-8.
func TextWithMark(data []byte) (mark, text string, ok bool) {
	if !utf8.Valid(data) {
		return "", "", false
	}
	text = WithoutMark(string(data))
	return string(data[:len(data)-len(text)]), text, true
}

// shortEscapes is the bytes that Quoted writes as a backslash and one character.
var shortEscapes = map[byte]string{'"': `\"`, '\\': `\\`, '\b': `\b`, '\f': `\f`, '\n': `\n`, '\r': `\r`, '\t': `\t`}

// Quoted writes text between double quotes as a JSON string that escapes no more than it must. Files and output
// that must stay the same byte for byte are written with it, so what it escapes is exactly this:
//
//   - the double quote and the backslash, each with a backslash before it;
//   - backspace (0x08), form feed (0x0C), line feed (0x0A), carriage return (0x0D) and tab (0x09), as a backslash
//     and the letter b, f, n, r or t;
//   - every other byte below 0x20, as a backslash, the letter u, two zeros and the byte in two hexadecimal digits
//     with lower-case letters.
//
// Every other byte is written as it is: the markup characters <, > and &, the slash, DEL (0x7F), and the bytes of
// a character outside ASCII, the line separator U+2028 and the paragraph separator U+2029 among them. Bytes that
// are not UTF-8 are kept as they are too.
func Quoted(text string) string {
	var out strings.Builder
	out.WriteByte('"')
	for i := range len(text) {
		c := text[i]
		switch escape, short := shortEscapes[c]; {
		case short:
			out.WriteString(escape)
		case c < 0x20:
			fmt.Fprintf(&out, `\u%04x`, c)
		default:
			out.WriteByte(c)
		}
	}
	out.WriteByte('"')
	return out.String()
}
