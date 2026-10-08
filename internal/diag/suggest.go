package diag

import (
	"fmt"
	"slices"
	"strings"
)

func ClosestNames(names []string, key string, max int) []string {
	matches := nearMatches(names, key)
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

func nearMatches(names []string, key string) []match {
	target := []rune(strings.ToLower(key))
	limit := max(1, len(target)/4)
	var matches []match
	for _, name := range names {
		if name == key {
			continue
		}
		candidate := []rune(strings.ToLower(name))
		if lengthDiff := len(candidate) - len(target); lengthDiff > limit || -lengthDiff > limit {
			continue
		}
		if dist := runeEditDistance(target, candidate); dist <= limit {
			matches = append(matches, match{name, dist})
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
	return runeEditDistance([]rune(a), []rune(b))
}

func runeEditDistance(a, b []rune) int {
	var prevPrevRow []int
	prevRow := make([]int, len(b)+1)
	for j := range prevRow {
		prevRow[j] = j
	}
	for i, charA := range a {
		row := make([]int, len(b)+1)
		row[0] = i + 1
		for j, charB := range b {
			substituted := prevRow[j]
			if charA != charB {
				substituted++
			}
			row[j+1] = min(prevRow[j+1]+1, row[j]+1, substituted)
			if i > 0 && j > 0 && charA == b[j-1] && a[i-1] == charB {
				row[j+1] = min(row[j+1], prevPrevRow[j-1]+1)
			}
		}
		prevPrevRow, prevRow = prevRow, row
	}
	return prevRow[len(b)]
}
