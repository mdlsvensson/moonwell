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
	negative := NewReader(nil)
	negative.Skip(-1)
	if negative.Err() == nil {
		t.Error("a negative size is a failure")
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
