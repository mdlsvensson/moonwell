package fsx

import (
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
