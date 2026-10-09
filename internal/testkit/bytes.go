package testkit

import (
	"encoding/binary"
	"math"
	"slices"
)

func Concat(parts ...[]byte) []byte { return slices.Concat(parts...) }

func U32(value uint32) []byte { return binary.LittleEndian.AppendUint32(nil, value) }

func F32(value float32) []byte { return U32(math.Float32bits(value)) }

func SetU32(data []byte, offset int, value uint32) []byte {
	copied := slices.Clone(data)
	binary.LittleEndian.PutUint32(copied[offset:], value)
	return copied
}

func FixedField(value string, size int) []byte {
	out := make([]byte, size)
	copy(out[:size-1], value)
	return out
}
