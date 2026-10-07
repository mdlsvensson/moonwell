package binio

import (
	"bytes"
	"testing"
)

func TestWriterAndReaderRoundTrip(t *testing.T) {
	var w Writer
	w.U8(0xAB)
	w.U16(0x1234)
	w.U32(0xDEADBEEF)
	w.I32(-2)
	w.F32(0.5)
	w.CString("héro")
	w.Zero(3)
	w.Write([]byte{1, 2})

	want := []byte{0xAB, 0x34, 0x12, 0xEF, 0xBE, 0xAD, 0xDE, 0xFE, 0xFF, 0xFF, 0xFF, 0, 0, 0, 0x3F}
	if !bytes.Equal(w.Bytes()[:len(want)], want) {
		t.Errorf("wrote % X", w.Bytes()[:len(want)])
	}

	r := NewReader(w.Bytes())
	if r.U8() != 0xAB || r.U16() != 0x1234 || r.U32() != 0xDEADBEEF || r.I32() != -2 || r.F32() != 0.5 {
		t.Error("numbers did not round trip")
	}
	if got := string(r.CString()); got != "héro" {
		t.Errorf("CString = %q", got)
	}
	r.Skip(3)
	if got := r.Bytes(2); !bytes.Equal(got, []byte{1, 2}) || r.Len() != 0 || r.Offset() != w.Len() {
		t.Errorf("tail = % X, %d left", got, r.Len())
	}
	if r.Err() != nil {
		t.Errorf("Err = %v", r.Err())
	}
}

// Every type is read once from fixed bytes, then written back: the Writer must produce the same bytes.
func TestEveryTypeReadsFromBytesAndWritesBackToThem(t *testing.T) {
	data := []byte{
		0x7F,       // U8
		0x01, 0x80, // U16
		0x78, 0x56, 0x34, 0x12, // U32
		0x00, 0x00, 0x00, 0x80, // I32: math.MinInt32
		0x00, 0x00, 0xC0, 0x3F, // F32: 1.5
		'a', 'b', 0, // CString
		0, 0, 0, // padding, read with Skip
		9, 8, 7, // raw bytes
	}

	r := NewReader(data)
	u8, u16, u32, i32, f32 := r.U8(), r.U16(), r.U32(), r.I32(), r.F32()
	text := r.CString()
	r.Skip(3)
	tail := r.Bytes(3)
	if r.Err() != nil || r.Len() != 0 {
		t.Fatalf("reading failed: %v, %d bytes left", r.Err(), r.Len())
	}
	if u8 != 0x7F || u16 != 0x8001 || u32 != 0x12345678 || i32 != -1<<31 || f32 != 1.5 || string(text) != "ab" {
		t.Errorf("read %#x %#x %#x %d %v %q", u8, u16, u32, i32, f32, text)
	}

	var w Writer
	w.U8(u8)
	w.U16(u16)
	w.U32(u32)
	w.I32(i32)
	w.F32(f32)
	w.CString(string(text))
	w.Zero(3)
	w.Write(tail)
	if !bytes.Equal(w.Bytes(), data) {
		t.Errorf("wrote % X, want % X", w.Bytes(), data)
	}
}

func TestReaderKeepsItsFirstFailure(t *testing.T) {
	r := NewReader([]byte{1, 2, 3})
	r.Skip(2)
	if got := r.U32(); got != 0 {
		t.Errorf("a read past the end returned %d", got)
	}
	err := r.Err()
	if err == nil || err.Offset != 2 || err.Unterminated {
		t.Fatalf("Err = %+v", err)
	}
	// Later reads return zero and leave the first failure in place, even where data remains.
	if r.U8() != 0 || r.CString() != nil || r.Offset() != 2 || r.Err() != err {
		t.Error("the reader went on after a failure")
	}
}

func TestReaderFailsOnASizeItCannotTake(t *testing.T) {
	cases := []struct {
		name string
		read func(r *Reader)
	}{
		{"negative skip", func(r *Reader) { r.Skip(-1) }},
		{"skip past the end", func(r *Reader) { r.Skip(4) }},
		{"negative bytes", func(r *Reader) { r.Bytes(-1) }},
		{"bytes past the end", func(r *Reader) { r.Bytes(4) }},
		{"oversized bytes", func(r *Reader) { r.Bytes(int(^uint(0) >> 1)) }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewReader([]byte{1, 2, 3})
			r.U8()
			c.read(r)
			err := r.Err()
			if err == nil || err.Offset != 1 || err.Unterminated {
				t.Fatalf("Err = %+v", err)
			}
			// A failed read consumes nothing.
			if r.Offset() != 1 || r.Len() != 2 {
				t.Errorf("offset %d, %d left", r.Offset(), r.Len())
			}
		})
	}
}

func TestReaderTakesExactlyTheBytesLeft(t *testing.T) {
	r := NewReader([]byte{1, 2, 3})
	if got := r.Bytes(3); !bytes.Equal(got, []byte{1, 2, 3}) || r.Len() != 0 || r.Err() != nil {
		t.Errorf("Bytes(3) = % X, %d left, Err = %v", got, r.Len(), r.Err())
	}
	if got := r.Bytes(0); got == nil || len(got) != 0 || r.Err() != nil {
		t.Errorf("Bytes(0) at the end = %v, Err = %v", got, r.Err())
	}
}

func TestReaderReportsAStringWithoutItsNUL(t *testing.T) {
	r := NewReader([]byte{0, 'a', 'b'})
	if got := r.CString(); got == nil || len(got) != 0 {
		t.Errorf("an empty string reads as %v", got)
	}
	r.CString()
	if err := r.Err(); err == nil || !err.Unterminated || err.Offset != 1 {
		t.Errorf("Err = %+v", err)
	}
}
