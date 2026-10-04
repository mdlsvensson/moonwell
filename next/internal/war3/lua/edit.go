package lua

import (
	"cmp"
	"errors"
	"slices"
	"strings"
)

// Edit replaces the bytes from Start to End of a source with Text. Start equal to End inserts.
type Edit struct {
	Start, End int
	Text       string
}

// ApplyEdits returns source with the edits made. They may come in any order and must not overlap; insertions at
// one offset keep the order they were given in. An error here is a bug in the caller.
//
// Every offset is one of source. An insertion where a replaced range starts goes before the replacement, and one
// where it ends goes after; an insertion further inside the range overlaps it.
func ApplyEdits(source string, edits []Edit) (string, error) {
	ordered := slices.Clone(edits)
	slices.SortStableFunc(ordered, inOrder)
	if !apart(ordered, len(source)) {
		return "", errEdits()
	}
	return spliced(source, ordered), nil
}

// inOrder orders edits by where they start. Of those that start at one offset the insertions come first, since
// they end there, and the sort keeps them in the order given.
func inOrder(a, b Edit) int {
	return cmp.Or(cmp.Compare(a.Start, b.Start), cmp.Compare(a.End, b.End))
}

// apart reports whether edits in their order each change a range of a source of the given size that begins at or
// after the end of the edit before.
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

// spliced copies source and puts each edit's text in place of its range. The edits are ordered and apart.
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

// ---- errors ----

// errEdits is not a diag error: it must be reported as Moonwell's own fault.
func errEdits() error {
	return errors.New("Overlapping or invalid Lua edits.")
}
