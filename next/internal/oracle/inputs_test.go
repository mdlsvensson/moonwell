package oracle

import (
	"strings"
	"testing"
)

func TestPartingLineIsTheFirstLineTheTwoReadingsDoNotAgreeUpTo(t *testing.T) {
	// The two readings part at the first line that holds the letter.
	for _, c := range []struct {
		data, letter string
		line         int
		upTo         string
	}{
		{"a\nb\nc\nd\n", "c", 3, "a\nb\nc\n"},
		{"a\nb\nc\nd", "a", 1, "a\n"},
		{"a\nb\nc\nd", "d", 4, "a\nb\nc\nd"},
		{"a\r\nb\r\n", "b", 2, "a\r\nb\r\n"},
		{"a", "a", 1, "a"},
	} {
		alike := func(upTo []byte) bool { return !strings.Contains(string(upTo), c.letter) }
		line, upTo := PartingLine([]byte(c.data), alike)
		if line != c.line || string(upTo) != c.upTo {
			t.Errorf("%q, parting at %s: line %d and %q, want line %d and %q",
				c.data, c.letter, line, upTo, c.line, c.upTo)
		}
	}
}
