// Package binio reads and writes the little-endian binary formats of Warcraft III maps.
//
// A Reader keeps its first failure: after a read past the end, every later read returns zero and Err reports where
// it went wrong. A parser reads straight through and checks once.
package binio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

// Error is a Reader's first failure.
type Error struct {
	Offset       int
	Unterminated bool // a string without its NUL; otherwise a read past the end
}

func (e *Error) Error() string {
	if e.Unterminated {
		return fmt.Sprintf("unterminated string at offset %d", e.Offset)
	}
	return fmt.Sprintf("unexpected end of data at offset %d", e.Offset)
}

// Reader reads from a byte slice.
type Reader struct {
	data   []byte
	offset int
	err    *Error
}

// NewReader returns a Reader at the start of data.
func NewReader(data []byte) *Reader { return &Reader{data: data} }

// Offset is the position of the next read.
func (r *Reader) Offset() int { return r.offset }

// Len is the number of bytes not read yet.
func (r *Reader) Len() int { return len(r.data) - r.offset }

// Err is the first failure, or nil.
func (r *Reader) Err() *Error { return r.err }

// take returns the next n bytes, or nil after a failure.
func (r *Reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > r.Len() {
		r.err = &Error{Offset: r.offset}
		return nil
	}
	b := r.data[r.offset : r.offset+n]
	r.offset += n
	return b
}

// Skip passes over n bytes.
func (r *Reader) Skip(n int) { r.take(n) }

// Bytes returns the next n bytes. The slice aliases the Reader's data.
func (r *Reader) Bytes(n int) []byte { return r.take(n) }

// U8 reads one byte.
func (r *Reader) U8() uint8 {
	if b := r.take(1); b != nil {
		return b[0]
	}
	return 0
}

// U16 reads an unsigned 16-bit integer.
func (r *Reader) U16() uint16 {
	if b := r.take(2); b != nil {
		return binary.LittleEndian.Uint16(b)
	}
	return 0
}

// U32 reads an unsigned 32-bit integer.
func (r *Reader) U32() uint32 {
	if b := r.take(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}

// I32 reads a signed 32-bit integer.
func (r *Reader) I32() int32 { return int32(r.U32()) }

// F32 reads a 32-bit float.
func (r *Reader) F32() float32 { return math.Float32frombits(r.U32()) }

// CString reads a NUL-terminated string and returns its bytes without the NUL.
func (r *Reader) CString() []byte {
	if r.err != nil {
		return nil
	}
	end := bytes.IndexByte(r.data[r.offset:], 0)
	if end < 0 {
		r.err = &Error{Offset: r.offset, Unterminated: true}
		return nil
	}
	b := r.data[r.offset : r.offset+end]
	r.offset += end + 1
	return b
}

// Writer builds a byte slice.
type Writer struct {
	data []byte
}

// Len is the number of bytes written.
func (w *Writer) Len() int { return len(w.data) }

// Bytes is what was written.
func (w *Writer) Bytes() []byte { return w.data }

// Write appends b.
func (w *Writer) Write(b []byte) { w.data = append(w.data, b...) }

// Zero appends n zero bytes.
func (w *Writer) Zero(n int) { w.data = append(w.data, make([]byte, n)...) }

// U8 appends one byte.
func (w *Writer) U8(v uint8) { w.data = append(w.data, v) }

// U16 appends an unsigned 16-bit integer.
func (w *Writer) U16(v uint16) { w.data = binary.LittleEndian.AppendUint16(w.data, v) }

// U32 appends an unsigned 32-bit integer.
func (w *Writer) U32(v uint32) { w.data = binary.LittleEndian.AppendUint32(w.data, v) }

// I32 appends a signed 32-bit integer.
func (w *Writer) I32(v int32) { w.U32(uint32(v)) }

// F32 appends a 32-bit float.
func (w *Writer) F32(v float32) { w.U32(math.Float32bits(v)) }

// CString appends s and a NUL.
func (w *Writer) CString(s string) {
	w.data = append(w.data, s...)
	w.data = append(w.data, 0)
}
