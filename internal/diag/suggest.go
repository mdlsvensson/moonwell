package diag

import (
	"fmt"
	"slices"
	"strings"
)

// Closest returns up to max of names that are close to key, for a "did you mean" hint; with max < 0, every close
// name. A name is close when it is within a quarter of key's length in edits, as EditDistance counts them, and
// at least one edit is always allowed; letter case is ignored. The nearest name comes first, and names equally
// near are in byte order. key itself is never returned.
func Closest(names []string, key string, max int) []string {
	matches := near(names, key)
	slices.SortStableFunc(matches, func(a, b match) int {
		if a.distance != b.distance {
			return a.distance - b.distance
		}
		return strings.Compare(a.name, b.name)
	})
	if max < 0 || max > len(matches) {
		max = len(matches)
	}
	var closest []string
	for _, m := range matches[:max] {
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

// EditDistance is how many edits turn a into b: a character inserted, removed or replaced, or two neighbours
// that change places, which is the slip of a typing hand and counts as one. A character is a Unicode code point,
// whatever its length in bytes.
func EditDistance(a, b string) int {
	return distance([]rune(a), []rune(b))
}

// distance fills the table of edits a row at a time: a row holds the distance from the characters of a read so
// far to each prefix of b. The row above is needed to fill the next, and the one above that for two neighbours
// that changed places.
func distance(a, b []rune) int {
	var twoAbove []int
	above := make([]int, len(b)+1)
	for j := range above {
		above[j] = j
	}
	for i, inA := range a {
		row := make([]int, len(b)+1)
		row[0] = i + 1
		for j, inB := range b {
			replaced := above[j]
			if inA != inB {
				replaced++
			}
			row[j+1] = min(above[j+1]+1, row[j]+1, replaced)
			if i > 0 && j > 0 && inA == b[j-1] && a[i-1] == inB {
				row[j+1] = min(row[j+1], twoAbove[j-1]+1)
			}
		}
		twoAbove, above = above, row
	}
	return above[len(b)]
}
