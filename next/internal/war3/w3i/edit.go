package w3i

import (
	"cmp"
	"errors"
	"slices"

	"github.com/mdlsvensson/moonwell/next/internal/binio"
)

// Edit replaces the bytes from Start to End with Bytes.
type Edit struct {
	Start, End int
	Bytes      []byte
}

// TextEdit replaces a string field; the new value may have another length.
func TextEdit(field Field[string], value string) Edit {
	var w binio.Writer
	w.CString(value)
	return Edit{field.Start, field.End, w.Bytes()}
}

// IntEdit replaces a 32-bit integer field.
func IntEdit(field Field[int32], value int32) Edit {
	var w binio.Writer
	w.I32(value)
	return Edit{field.Start, field.End, w.Bytes()}
}

// FloatEdit replaces a 32-bit float field.
func FloatEdit(field Field[float32], value float32) Edit {
	var w binio.Writer
	w.F32(value)
	return Edit{field.Start, field.End, w.Bytes()}
}

// ByteEdit replaces a one-byte field.
func ByteEdit(field Field[uint8], value uint8) Edit {
	return Edit{field.Start, field.End, []byte{value}}
}

// ApplyEdits returns source with the edits made. The edits may come in any order, and all of them give their
// offsets in source. They must lie inside source and must not overlap; edits that start at one offset are made in
// the order given, so only the last of them may replace bytes. An error here is a bug in the caller, not a problem
// with the map.
func ApplyEdits(source []byte, edits []Edit) ([]byte, error) {
	ordered := slices.Clone(edits)
	slices.SortStableFunc(ordered, func(a, b Edit) int { return cmp.Compare(a.Start, b.Start) })
	if !apart(ordered, len(source)) {
		return nil, errEdits()
	}
	return spliced(source, ordered), nil
}

// apart reports whether edits ordered by their start each replace a range of a file of the given size that begins
// at or after the end of the edit before.
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

// spliced copies source and puts each edit's bytes in place of its range. The edits are ordered and apart.
func spliced(source []byte, ordered []Edit) []byte {
	result := make([]byte, 0, len(source))
	kept := 0
	for _, edit := range ordered {
		result = append(result, source[kept:edit.Start]...)
		result = append(result, edit.Bytes...)
		kept = edit.End
	}
	return append(result, source[kept:]...)
}

// ---- errors ----

// errEdits is not a diag error: it must be reported as Moonwell's own fault.
func errEdits() error {
	return errors.New("Invalid or overlapping map-info edits.")
}
