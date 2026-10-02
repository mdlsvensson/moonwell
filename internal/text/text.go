// Package text holds the places where JavaScript's strings and Go's differ. Moonwell's output was first defined by a
// TypeScript program, so lengths, sort order, quoting, whitespace, case mapping and UTF-8 decoding follow JavaScript
// wherever they reach a file or a message.
package text

import (
	"slices"
	"strings"
	"unicode/utf8"
)

// UTF16Len is the length of s in UTF-16 code units: JavaScript's String length.
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// UTF16Offset is the UTF-16 index of the byte offset in s.
func UTF16Offset(s string, byteOffset int) int {
	return UTF16Len(s[:min(byteOffset, len(s))])
}

// units returns the UTF-16 code units of r; the second is 0 for a rune of one unit.
func units(r rune) (uint16, uint16) {
	if r < 0x10000 {
		return uint16(r), 0
	}
	r -= 0x10000
	return uint16(0xD800 + (r >> 10)), uint16(0xDC00 + (r & 0x3FF))
}

// Compare orders a and b by UTF-16 code units, as JavaScript's default sort and its < operator do. It differs from
// byte order only between a rune above U+FFFF and one from U+E000 to U+FFFF.
func Compare(a, b string) int {
	for len(a) > 0 && len(b) > 0 {
		ra, sizeA := utf8.DecodeRuneInString(a)
		rb, sizeB := utf8.DecodeRuneInString(b)
		if ra != rb {
			a1, a2 := units(ra)
			b1, b2 := units(rb)
			if a1 != b1 {
				return int(a1) - int(b1)
			}
			return int(a2) - int(b2)
		}
		a, b = a[sizeA:], b[sizeB:]
	}
	return len(a) - len(b)
}

// Less reports whether a sorts before b in JavaScript's order.
func Less(a, b string) bool { return Compare(a, b) < 0 }

// Sort sorts s in JavaScript's order.
func Sort(s []string) { slices.SortFunc(s, Compare) }

const hex = "0123456789abcdef"

// Quote is JSON.stringify of a string: only the quote, the backslash and control characters are escaped.
func Quote(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 2)
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if c < 0x20 {
				b.WriteString(`\u00`)
				b.WriteByte(hex[c>>4])
				b.WriteByte(hex[c&0xF])
			} else {
				b.WriteByte(c)
			}
		}
	}
	b.WriteByte('"')
	return b.String()
}

// IsSpace reports whether r matches JavaScript's \s: its white space and its line terminators.
func IsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF:
		return true
	}
	return r >= 0x2000 && r <= 0x200A
}

// SpaceClass is JavaScript's \s as a character class for Go's regular expressions, whose own \s is ASCII only and
// has no vertical tab.
const SpaceClass = "[" + SpaceSet + "]"

// SpaceSet is the inside of SpaceClass, for building other classes such as "[^=" + SpaceSet + "]".
const SpaceSet = `\t\n\v\f\r \x{A0}\x{1680}\x{2000}-\x{200A}\x{2028}\x{2029}\x{202F}\x{205F}\x{3000}\x{FEFF}`

// NotLineBreak is JavaScript's `.` as a character class: anything but a line terminator.
const NotLineBreak = `[^\n\r\x{2028}\x{2029}]`

// Trim is String.prototype.trim.
func Trim(s string) string { return strings.TrimFunc(s, IsSpace) }

// The entries of Unicode's SpecialCasing that turn up in file names. JavaScript applies the whole table and the
// final-sigma rule; the rest is not reproduced here.
var upperSpecial = strings.NewReplacer(
	"ß", "SS", "ﬀ", "FF", "ﬁ", "FI", "ﬂ", "FL", "ﬃ", "FFI", "ﬄ", "FFL", "ﬅ", "ST", "ﬆ", "ST",
)

// Upper is toUpperCase.
func Upper(s string) string {
	if isASCII(s) {
		return strings.ToUpper(s)
	}
	return strings.ToUpper(upperSpecial.Replace(s))
}

// Lower is toLowerCase.
func Lower(s string) string {
	if isASCII(s) {
		return strings.ToLower(s)
	}
	return strings.ToLower(strings.ReplaceAll(s, "İ", "i̇"))
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// TruncateUTF16 returns the start of s, at most limit UTF-16 code units long: JavaScript's s.slice(0, limit), but
// never cutting a character in two.
func TruncateUTF16(s string, limit int) string {
	units := 0
	for i, r := range s {
		size := 1
		if r >= 0x10000 {
			size = 2
		}
		if units+size > limit {
			return s[:i]
		}
		units += size
	}
	return s
}
