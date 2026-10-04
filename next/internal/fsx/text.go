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
