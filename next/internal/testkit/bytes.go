package testkit

import (
	"encoding/binary"
	"math"
	"slices"
)

// Concat joins byte slices.
func Concat(parts ...[]byte) []byte { return slices.Concat(parts...) }

// U32 is value as four little-endian bytes.
func U32(value uint32) []byte { return binary.LittleEndian.AppendUint32(nil, value) }

// F32 is value as four little-endian bytes.
func F32(value float32) []byte { return U32(math.Float32bits(value)) }

// SetU32 returns a copy of data with the uint32 at offset replaced.
func SetU32(data []byte, offset int, value uint32) []byte {
	copied := slices.Clone(data)
	binary.LittleEndian.PutUint32(copied[offset:], value)
	return copied
}

// Fixed is a NUL-padded string field of size bytes.
func Fixed(value string, size int) []byte {
	out := make([]byte, size)
	copy(out[:size-1], value)
	return out
}
