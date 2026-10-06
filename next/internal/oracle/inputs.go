package oracle

import (
	"bytes"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// Changed is testkit.Changed, for the oracles that ask for it here.
func Changed(text string, seed, index uint64) string { return testkit.Changed(text, seed, index) }

// Swept is testkit.Swept, for the oracles that ask for it here.
func Swept(text string) []string { return testkit.Swept(text) }

// PartingLine is a line of an input, counted from 1, at which two readings of it part: they make the same of the
// lines before it, and not of the lines up to it. alike says of the first lines of the input whether the two
// readings make the same of them. It is for an input the readings do not agree on: the line is found by halving
// between no line, which they agree on, and the whole input. It returns the line and the input up to it.
func PartingLine(data []byte, alike func(upTo []byte) bool) (int, []byte) {
	lines := bytes.SplitAfter(data, []byte("\n"))
	upTo := func(count int) []byte { return bytes.Join(lines[:count], nil) }
	low, high := 0, len(lines)
	for high-low > 1 {
		if middle := (low + high) / 2; alike(upTo(middle)) {
			low = middle
		} else {
			high = middle
		}
	}
	return high, upTo(high)
}
