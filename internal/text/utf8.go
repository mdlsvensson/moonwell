package text

import (
	"strings"
	"unicode/utf8"
)

const bom = "\xEF\xBB\xBF"

// Lossy decodes UTF-8 as Deno.readTextFile does: each maximal invalid sequence becomes one U+FFFD, and a byte order
// mark stays. (Go's strings.ToValidUTF8 would merge neighbouring invalid sequences into one replacement.)
func Lossy(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	// The UTF-8 decoder of the Encoding Standard.
	var out strings.Builder
	out.Grow(len(b))
	var codePoint rune
	needed, seen := 0, 0
	lower, upper := byte(0x80), byte(0xBF)
	for i := 0; i < len(b); i++ {
		c := b[i]
		if needed == 0 {
			switch {
			case c <= 0x7F:
				out.WriteByte(c)
			case c >= 0xC2 && c <= 0xDF:
				needed, codePoint = 1, rune(c&0x1F)
			case c >= 0xE0 && c <= 0xEF:
				if c == 0xE0 {
					lower = 0xA0
				}
				if c == 0xED {
					upper = 0x9F
				}
				needed, codePoint = 2, rune(c&0xF)
			case c >= 0xF0 && c <= 0xF4:
				if c == 0xF0 {
					lower = 0x90
				}
				if c == 0xF4 {
					upper = 0x8F
				}
				needed, codePoint = 3, rune(c&0x7)
			default:
				out.WriteRune(utf8.RuneError)
			}
			continue
		}
		if c < lower || c > upper {
			codePoint, needed, seen = 0, 0, 0
			lower, upper = 0x80, 0xBF
			out.WriteRune(utf8.RuneError)
			i-- // the byte that broke the sequence is read again
			continue
		}
		lower, upper = 0x80, 0xBF
		codePoint = codePoint<<6 | rune(c&0x3F)
		seen++
		if seen == needed {
			out.WriteRune(codePoint)
			codePoint, needed, seen = 0, 0, 0
		}
	}
	if needed != 0 {
		out.WriteRune(utf8.RuneError)
	}
	return out.String()
}

// Decode decodes UTF-8 as `new TextDecoder().decode` does: like Lossy, with a leading byte order mark removed.
func Decode(b []byte) string {
	return strings.TrimPrefix(Lossy(b), bom)
}

// Strict decodes UTF-8 as a fatal TextDecoder does: false for invalid bytes, and a leading byte order mark removed.
func Strict(b []byte) (string, bool) {
	if !utf8.Valid(b) {
		return "", false
	}
	return strings.TrimPrefix(string(b), bom), true
}
