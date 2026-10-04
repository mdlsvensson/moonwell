package diag

import (
	"fmt"
	"slices"
	"strings"
)

// Closest returns up to max of names that are close to key, for a "did you mean" hint. A name is close when it is
// within a quarter of key's length in edits, and at least one edit is always allowed; letter case is ignored. The
// nearest name comes first, and names equally near are in byte order. key itself is never returned.
func Closest(names []string, key string, max int) []string {
	matches := near(names, key)
	slices.SortStableFunc(matches, func(a, b match) int {
		if a.distance != b.distance {
			return a.distance - b.distance
		}
		return strings.Compare(a.name, b.name)
	})
	var closest []string
	for _, m := range matches[:min(len(matches), max)] {
		closest = append(closest, m.name)
	}
	return closest
}

// match is a name and how many edits it is from the key.
type match struct {
	name     string
	distance int
}

// near returns the names within the allowed number of edits of key, in the order of names.
func near(names []string, key string) []match {
	wanted := []rune(strings.ToLower(key))
	limit := max(1, len(wanted)/4)
	var matches []match
	for _, name := range names {
		if name == key {
			continue
		}
		candidate := []rune(strings.ToLower(name))
		// Two names are at least as many edits apart as their lengths differ, so such a pair needs no comparing.
		if gap := len(candidate) - len(wanted); gap > limit || -gap > limit {
			continue
		}
		if d := distance(wanted, candidate); d <= limit {
			matches = append(matches, match{name, d})
		}
	}
	return matches
}

// JoinWords renders `a`, `a or b`, `a, b or c`. With max >= 0, at most max words are shown and the rest become
// `and N more`; with max < 0, all are shown.
func JoinWords(words []string, conjunction string, max int) string {
	if max < 0 || max > len(words) {
		max = len(words)
	}
	shown := words[:max]
	if hidden := len(words) - max; hidden > 0 {
		return fmt.Sprintf("%s and %d more", strings.Join(shown, ", "), hidden)
	}
	if len(shown) < 2 {
		return strings.Join(shown, "")
	}
	last := len(shown) - 1
	return strings.Join(shown[:last], ", ") + " " + conjunction + " " + shown[last]
}

// EditDistance is the Levenshtein distance between a and b: how many characters must be inserted, removed or
// replaced to turn one into the other. A character is a Unicode code point, whatever its length in bytes.
func EditDistance(a, b string) int {
	return distance([]rune(a), []rune(b))
}

// distance fills the Levenshtein table a row at a time: a row holds the distance from the characters of a read so
// far to each prefix of b, and only the row above is needed to fill the next.
func distance(a, b []rune) int {
	above := make([]int, len(b)+1)
	row := make([]int, len(b)+1)
	for j := range above {
		above[j] = j
	}
	for i, inA := range a {
		row[0] = i + 1
		for j, inB := range b {
			replaced := above[j]
			if inA != inB {
				replaced++
			}
			row[j+1] = min(above[j+1]+1, row[j]+1, replaced)
		}
		above, row = row, above
	}
	return above[len(b)]
}
