package objmod

import (
	"fmt"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type TableKind int

const (
	Simple TableKind = iota
	Leveled
)

func KindOf(fileName string) TableKind {
	switch strings.ToLower(path.Ext(fileName)) {
	case ".w3a", ".w3d", ".w3q":
		return Leveled
	}
	return Simple
}

type ValueType int

const (
	Int ValueType = iota
	Real
	Unreal
	String
)

type Value struct {
	Type ValueType
	Int  int32
	Real float32
	Text string
}

type Modification struct {
	Field       ID
	Level       int32
	Column      int32
	Value       Value
	End         ID
	Start, Stop int
}

type Set struct {
	Flag int32
	Mods []Modification
}

type Object struct {
	Base, ID    ID
	Sets        []Set
	Start, Stop int
}

type Table struct {
	CountOffset, Start, Stop int
	Objects                  []Object
}

type File struct {
	Version  int32
	Original Table
	Custom   Table
}

const maxSets = 64

func Read(data []byte, kind TableKind, displayPath string) (*File, error) {
	r := &reader{input: binio.NewReader(data), kind: kind, displayPath: displayPath}
	parsed := r.readFile()
	if err := r.readErr(); err != nil {
		return nil, err
	}
	return parsed, nil
}

type reader struct {
	input       *binio.Reader
	kind        TableKind
	displayPath string
	version     int32
	err         error
}

func (r *reader) fail(err error) {
	if !r.hasFailed() {
		r.err = err
	}
}

func (r *reader) hasFailed() bool {
	return r.err != nil || r.input.Err() != nil
}

func (r *reader) readErr() error {
	readErr := r.input.Err()
	switch {
	case r.err != nil:
		return r.err
	case readErr == nil:
		return nil
	case readErr.Unterminated:
		return errUnterminated(r.displayPath)
	}
	return errTruncated(r.displayPath)
}

func (r *reader) readFile() *File {
	r.version = r.input.I32()
	if r.version < 1 || r.version > 3 {
		r.fail(errUnsupportedVersion(r.displayPath, r.version))
	}
	parsed := &File{Version: r.version}
	parsed.Original = r.readTable()
	parsed.Custom = r.readTable()
	if r.input.Len() != 0 {
		r.fail(errTrailingData(r.displayPath))
	}
	return parsed
}

func (r *reader) readTable() Table {
	table := Table{CountOffset: r.input.Offset(), Objects: []Object{}}
	r.repeat(r.readCount("object", r.minObjectSize()), func() {
		table.Objects = append(table.Objects, r.readObject())
	})
	table.Start, table.Stop = table.CountOffset+4, r.input.Offset()
	return table
}

func (r *reader) readObject() Object {
	object := Object{Start: r.input.Offset()}
	object.Base, object.ID = r.readID(), r.readID()
	r.repeat(r.readSetCount(), func() {
		object.Sets = append(object.Sets, r.readSet())
	})
	object.Stop = r.input.Offset()
	return object
}

func (r *reader) readSetCount() int {
	if r.version < 3 {
		return 1
	}
	count := r.input.I32()
	if count < 1 || count > maxSets {
		r.fail(errSetCount(r.displayPath, count))
		return 0
	}
	return int(count)
}

func (r *reader) readSet() Set {
	set := Set{Mods: []Modification{}}
	if r.version >= 3 {
		set.Flag = r.input.I32()
	}
	r.repeat(r.readCount("modification", r.minModificationSize()), func() {
		set.Mods = append(set.Mods, r.readModification())
	})
	return set
}

func (r *reader) readModification() Modification {
	mod := Modification{Start: r.input.Offset()}
	mod.Field = r.readID()
	valueType := r.input.I32()
	if r.kind == Leveled {
		mod.Level, mod.Column = r.input.I32(), r.input.I32()
	}
	mod.Value = r.readValue(valueType)
	mod.End = r.readID()
	mod.Stop = r.input.Offset()
	return mod
}

func (r *reader) readValue(number int32) Value {
	switch valueType := ValueType(number); valueType {
	case Int:
		return Value{Type: Int, Int: r.input.I32()}
	case Real, Unreal:
		return Value{Type: valueType, Real: r.input.F32()}
	case String:
		return Value{Type: String, Text: r.readText()}
	}
	r.fail(errUnknownValueType(r.displayPath, number))
	return Value{}
}

func (r *reader) readCount(what string, smallest int) int {
	count := r.input.I32()
	if count < 0 || int64(count)*int64(smallest) > int64(r.input.Len()) {
		r.fail(errBadCount(r.displayPath, what, count))
	}
	if r.hasFailed() {
		return 0
	}
	return int(count)
}

func (r *reader) repeat(count int, read func()) {
	for range count {
		if r.hasFailed() {
			return
		}
		read()
	}
}

func (r *reader) minObjectSize() int {
	if r.version >= 3 {
		return 20
	}
	return 12
}

func (r *reader) minModificationSize() int {
	if r.kind == Leveled {
		return 21
	}
	return 13
}

func (r *reader) readID() ID {
	var id ID
	copy(id[:], r.input.Bytes(len(id)))
	return id
}

func (r *reader) readText() string {
	raw := r.input.CString()
	if !utf8.Valid(raw) {
		r.fail(errNotUTF8(r.displayPath))
	}
	return fsx.TrimBOM(string(raw))
}

func errUnreadable(displayPath, problem string) error {
	return &diag.Error{
		Msg:  "Cannot read object data: " + problem + ".",
		File: displayPath,
		Hint: "Open and re-save this map in World Editor 3.00.",
	}
}

func errUnsupportedVersion(displayPath string, version int32) error {
	return errUnreadable(displayPath, fmt.Sprintf("unsupported version %d", version))
}

func errTruncated(displayPath string) error {
	return errUnreadable(displayPath, "truncated file")
}

func errUnterminated(displayPath string) error {
	return errUnreadable(displayPath, "unterminated string")
}

func errNotUTF8(displayPath string) error {
	return errUnreadable(displayPath, "invalid UTF-8 in a string")
}

func errBadCount(displayPath, what string, count int32) error {
	return errUnreadable(displayPath, fmt.Sprintf("%s count %d past end of file", what, count))
}

func errSetCount(displayPath string, count int32) error {
	return errUnreadable(displayPath, fmt.Sprintf("unsupported set count %d", count))
}

func errUnknownValueType(displayPath string, number int32) error {
	return errUnreadable(displayPath, fmt.Sprintf("unknown value type %d", number))
}

func errTrailingData(displayPath string) error {
	return errUnreadable(displayPath, "trailing bytes after the custom objects")
}
