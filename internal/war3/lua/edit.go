package lua

import (
	"cmp"
	"errors"
	"slices"
	"strings"
)

type Edit struct {
	Start, End int
	Text       string
}

func ApplyEdits(source string, edits []Edit) (string, error) {
	sorted := slices.Clone(edits)
	slices.SortStableFunc(sorted, compareEdits)
	if !areDisjoint(sorted, len(source)) {
		return "", errOverlappingEdits()
	}
	return applySorted(source, sorted), nil
}

func compareEdits(a, b Edit) int {
	return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.End, b.End))
}

func areDisjoint(sorted []Edit, size int) bool {
	end := 0
	for _, edit := range sorted {
		if edit.Start < end || edit.End < edit.Start || edit.End > size {
			return false
		}
		end = edit.End
	}
	return true
}

func applySorted(source string, sorted []Edit) string {
	var result strings.Builder
	kept := 0
	for _, edit := range sorted {
		result.WriteString(source[kept:edit.Start])
		result.WriteString(edit.Text)
		kept = edit.End
	}
	result.WriteString(source[kept:])
	return result.String()
}

func errOverlappingEdits() error {
	return errors.New("Overlapping or invalid Lua edits.")
}
