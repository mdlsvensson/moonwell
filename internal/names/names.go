// Package names compares and lists names for error messages: "did you mean" suggestions and word lists.
package names

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf16"

	"github.com/mdlsvensson/moonwell/internal/text"
)

// EditDistance is the Levenshtein distance between a and b, counted in UTF-16 code units.
func EditDistance(a, b string) int {
	return distance(utf16.Encode([]rune(a)), utf16.Encode([]rune(b)))
}

func distance(a, b []uint16) int {
	previous := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current := make([]int, len(b)+1)
		current[0] = i
		for j := 1; j <= len(b); j++ {
			substitution := previous[j-1]
			if a[i-1] != b[j-1] {
				substitution++
			}
			current[j] = min(previous[j]+1, current[j-1]+1, substitution)
		}
		previous = current
	}
	return previous[len(b)]
}

// JoinWords renders `a`, `a or b`, `a, b or c`. With max >= 0, at most max words are shown and the rest become
// `and N more`.
func JoinWords(words []string, conjunction string, max int) string {
	if max < 0 || max > len(words) {
		max = len(words)
	}
	shown := words[:max]
	if len(words) > max {
		return fmt.Sprintf("%s and %d more", strings.Join(shown, ", "), len(words)-max)
	}
	if len(shown) < 2 {
		return strings.Join(shown, "")
	}
	return strings.Join(shown[:len(shown)-1], ", ") + " " + conjunction + " " + shown[len(shown)-1]
}

// Closest returns up to max of names closest to key, ignoring letter case: at most a quarter of its length in edits
// (at least one), nearest first, then by name. key itself is never returned.
func Closest(names []string, key string, max int) []string {
	wanted := utf16.Encode([]rune(text.Lower(key)))
	limit := len(wanted) / 4
	if limit < 1 {
		limit = 1
	}
	type match struct {
		name     string
		distance int
	}
	var matches []match
	for _, name := range names {
		if name == key {
			continue
		}
		if gap := text.UTF16Len(name) - len(wanted); gap > limit || -gap > limit {
			continue
		}
		if d := distance(wanted, utf16.Encode([]rune(text.Lower(name)))); d <= limit {
			matches = append(matches, match{name, d})
		}
	}
	slices.SortStableFunc(matches, func(a, b match) int {
		if a.distance != b.distance {
			return a.distance - b.distance
		}
		return text.Compare(a.name, b.name)
	})
	var out []string
	for _, m := range matches[:min(len(matches), max)] {
		out = append(out, m.name)
	}
	return out
}
