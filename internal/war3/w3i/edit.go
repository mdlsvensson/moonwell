package w3i

import (
	"cmp"
	"errors"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/binio"
)

type Edit struct {
	Start, End int
	Bytes      []byte
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
	ordered := slices.Clone(edits)
	slices.SortStableFunc(ordered, func(a, b Edit) int { return cmp.Compare(a.Start, b.Start) })
	if !apart(ordered, len(source)) {
		return nil, errEdits()
	}
	return spliced(source, ordered), nil
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

func errEdits() error {
	return errors.New("Invalid or overlapping map-info edits.")
}
