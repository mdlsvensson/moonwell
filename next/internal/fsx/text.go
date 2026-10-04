package fsx

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	byteOrderMark = "\xEF\xBB\xBF"
	replacement   = string(utf8.RuneError) // U+FFFD
)

// DecodeText reads bytes as UTF-8 text: invalid bytes become U+FFFD and a leading byte order mark is dropped.
// Neighbouring invalid bytes become one U+FFFD together.
func DecodeText(b []byte) string {
	return strings.TrimPrefix(strings.ToValidUTF8(string(b), replacement), byteOrderMark)
}

// TextWithMark splits a file's bytes into a leading byte order mark ("" when there is none) and the text after
// it, so that the text can be changed and the mark put back. ok is false when the bytes are not valid UTF-8.
func TextWithMark(data []byte) (mark, text string, ok bool) {
	if !utf8.Valid(data) {
		return "", "", false
	}
	if text, marked := strings.CutPrefix(string(data), byteOrderMark); marked {
		return byteOrderMark, text, true
	}
	return "", string(data), true
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
