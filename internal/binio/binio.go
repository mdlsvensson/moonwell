package binio

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
)

type Error struct {
	Offset       int
	Unterminated bool
}

func (e *Error) Error() string {
	if e.Unterminated {
		return fmt.Sprintf("unterminated string at offset %d", e.Offset)
	}
	return fmt.Sprintf("unexpected end of data at offset %d", e.Offset)
}

type Reader struct {
	data []byte
	pos  int
	err  *Error
}

func NewReader(data []byte) *Reader { return &Reader{data: data} }

func (r *Reader) Offset() int { return r.pos }

func (r *Reader) Len() int { return len(r.data) - r.pos }

func (r *Reader) Err() *Error { return r.err }

func (r *Reader) take(n int) []byte {
	if r.err != nil {
		return nil
	}
	if n < 0 || n > r.Len() {
		r.err = &Error{Offset: r.pos}
		return nil
	}
	b := r.data[r.pos : r.pos+n]
	r.pos += n
	return b
}

func (r *Reader) Skip(n int) { r.take(n) }

func (r *Reader) Bytes(n int) []byte { return r.take(n) }

func (r *Reader) U8() uint8 {
	if b := r.take(1); b != nil {
		return b[0]
	}
	return 0
}

func (r *Reader) U16() uint16 {
	if b := r.take(2); b != nil {
		return binary.LittleEndian.Uint16(b)
	}
	return 0
}

func (r *Reader) U32() uint32 {
	if b := r.take(4); b != nil {
		return binary.LittleEndian.Uint32(b)
	}
	return 0
}

func (r *Reader) I32() int32 { return int32(r.U32()) }

func (r *Reader) F32() float32 { return math.Float32frombits(r.U32()) }

func (r *Reader) CString() []byte {
	if r.err != nil {
		return nil
	}
	n := bytes.IndexByte(r.data[r.pos:], 0)
	if n < 0 {
		r.err = &Error{Offset: r.pos, Unterminated: true}
		return nil
	}
	s := r.take(n)
	r.pos++
	return s
}

type Writer struct {
	data []byte
}

func (w *Writer) Len() int { return len(w.data) }

func (w *Writer) Bytes() []byte { return w.data }

func (w *Writer) Write(b []byte) { w.data = append(w.data, b...) }

func (w *Writer) Zero(n int) { w.data = append(w.data, make([]byte, n)...) }

func (w *Writer) U8(v uint8) { w.data = append(w.data, v) }

func (w *Writer) U16(v uint16) { w.data = binary.LittleEndian.AppendUint16(w.data, v) }

func (w *Writer) U32(v uint32) { w.data = binary.LittleEndian.AppendUint32(w.data, v) }

func (w *Writer) I32(v int32) { w.U32(uint32(v)) }

func (w *Writer) F32(v float32) { w.U32(math.Float32bits(v)) }

func (w *Writer) CString(s string) {
	w.data = append(w.data, s...)
	w.data = append(w.data, 0)
}
