package fsx

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	byteOrderMark = "\xEF\xBB\xBF"
	replacement   = string(utf8.RuneError)
)

const ASCIISpace = " \t\n\v\f\r"

func TrimASCIISpace(text string) string { return strings.Trim(text, ASCIISpace) }

func WithoutMark[T string | []byte](text T) T {
	if len(text) >= len(byteOrderMark) && string(text[:len(byteOrderMark)]) == byteOrderMark {
		return text[len(byteOrderMark):]
	}
	return text
}

func DecodeText(b []byte) string {
	return WithoutMark(strings.ToValidUTF8(string(b), replacement))
}

func TextWithMark(data []byte) (mark, text string, ok bool) {
	if !utf8.Valid(data) {
		return "", "", false
	}
	text = WithoutMark(string(data))
	return string(data[:len(data)-len(text)]), text, true
}

var shortEscapes = map[byte]string{'"': `\"`, '\\': `\\`, '\b': `\b`, '\f': `\f`, '\n': `\n`, '\r': `\r`, '\t': `\t`}

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
