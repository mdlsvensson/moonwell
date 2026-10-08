package w3i

import (
	"cmp"
	"errors"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/binio"
)

type Edit struct {
	Start, End int
	Data       []byte
}

func TextEdit(field Field[string], value string) Edit {
	var w binio.Writer
	w.CString(value)
	return Edit{field.Start, field.End, w.Bytes()}
}

func IntEdit(field Field[int32], value int32) Edit {
	var w binio.Writer
	w.I32(value)
	return Edit{field.Start, field.End, w.Bytes()}
}

func FloatEdit(field Field[float32], value float32) Edit {
	var w binio.Writer
	w.F32(value)
	return Edit{field.Start, field.End, w.Bytes()}
}

func ByteEdit(field Field[uint8], value uint8) Edit {
	return Edit{field.Start, field.End, []byte{value}}
}

func ApplyEdits(source []byte, edits []Edit) ([]byte, error) {
	sorted := slices.Clone(edits)
	slices.SortStableFunc(sorted, func(a, b Edit) int { return cmp.Compare(a.Start, b.Start) })
	if !areDisjoint(sorted, len(source)) {
		return nil, errOverlappingEdits()
	}
	return applySorted(source, sorted), nil
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

func applySorted(source []byte, sorted []Edit) []byte {
	result := make([]byte, 0, len(source))
	kept := 0
	for _, edit := range sorted {
		result = append(result, source[kept:edit.Start]...)
		result = append(result, edit.Data...)
		kept = edit.End
	}
	return append(result, source[kept:]...)
}

func errOverlappingEdits() error {
	return errors.New("Invalid or overlapping map-info edits.")
}
