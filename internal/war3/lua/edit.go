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
	ordered := slices.Clone(edits)
	slices.SortStableFunc(ordered, inOrder)
	if !apart(ordered, len(source)) {
		return "", errEdits()
	}
	return spliced(source, ordered), nil
}

func inOrder(a, b Edit) int {
	return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.End, b.End))
}

func apart(ordered []Edit, size int) bool {
	kept := 0
	for _, edit := range ordered {
		if edit.Start < kept || edit.End < edit.Start || edit.End > size {
			return false
		}
		kept = edit.End
	}
	return true
}

func spliced(source string, ordered []Edit) string {
	var result strings.Builder
	kept := 0
	for _, edit := range ordered {
		result.WriteString(source[kept:edit.Start])
		result.WriteString(edit.Text)
		kept = edit.End
	}
	result.WriteString(source[kept:])
	return result.String()
}

func errEdits() error {
	return errors.New("Overlapping or invalid Lua edits.")
}
