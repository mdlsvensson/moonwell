package w3i

import (
	"encoding/binary"
	"errors"
	"math"
	"slices"
)

// Edit replaces the bytes from Start to End with Bytes.
type Edit struct {
	Start, End int
	Bytes      []byte
}

// TextEdit replaces a string field; the new value may have another length.
func TextEdit(field Field[string], value string) Edit {
	return Edit{field.Start, field.End, append([]byte(value), 0)}
}

// IntEdit replaces a 32-bit integer field.
func IntEdit(field Field[int32], value int32) Edit {
	return Edit{field.Start, field.End, binary.LittleEndian.AppendUint32(nil, uint32(value))}
}

// FloatEdit replaces a 32-bit float field.
func FloatEdit(field Field[float32], value float32) Edit {
	return Edit{field.Start, field.End, binary.LittleEndian.AppendUint32(nil, math.Float32bits(value))}
}

// ByteEdit replaces a one-byte field.
func ByteEdit(field Field[uint8], value uint8) Edit {
	return Edit{field.Start, field.End, []byte{value}}
}

// ApplyEdits returns source with the edits made. The edits may come in any order and must not overlap. An error
// here is a bug in the caller, not a problem with the map.
func ApplyEdits(source []byte, edits []Edit) ([]byte, error) {
	ordered := slices.Clone(edits)
	slices.SortStableFunc(ordered, func(a, b Edit) int { return a.Start - b.Start })
	input, size := 0, len(source)
	for _, edit := range ordered {
		if edit.Start < 0 || edit.End < edit.Start || edit.End > len(source) || edit.Start < input {
			return nil, errors.New("Invalid or overlapping map-info edits.")
		}
		size += len(edit.Bytes) - (edit.End - edit.Start)
		input = edit.End
	}
	result := make([]byte, 0, size)
	input = 0
	for _, edit := range ordered {
		result = append(result, source[input:edit.Start]...)
		result = append(result, edit.Bytes...)
		input = edit.End
	}
	return append(result, source[input:]...), nil
}
