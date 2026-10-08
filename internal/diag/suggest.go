package diag

import (
	"fmt"
	"slices"
	"strings"
)

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

type match struct {
	name     string
	distance int
}

func near(names []string, key string) []match {
	wanted := []rune(strings.ToLower(key))
	limit := max(1, len(wanted)/4)
	var matches []match
	for _, name := range names {
		if name == key {
			continue
		}
		candidate := []rune(strings.ToLower(name))
		if gap := len(candidate) - len(wanted); gap > limit || -gap > limit {
			continue
		}
		if d := distance(wanted, candidate); d <= limit {
			matches = append(matches, match{name, d})
		}
	}
	return matches
}

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

func EditDistance(a, b string) int {
	return distance([]rune(a), []rune(b))
}

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
