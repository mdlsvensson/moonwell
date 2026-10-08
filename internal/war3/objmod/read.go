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

func Read(data []byte, kind TableKind, file string) (*File, error) {
	r := &reader{data: binio.NewReader(data), kind: kind, file: file}
	parsed := r.whole()
	if err := r.failure(); err != nil {
		return nil, err
	}
	return parsed, nil
}

type reader struct {
	data    *binio.Reader
	kind    TableKind
	file    string
	version int32
	problem error
}

func (r *reader) refuse(problem error) {
	if !r.stopped() {
		r.problem = problem
	}
}

func (r *reader) stopped() bool {
	return r.problem != nil || r.data.Err() != nil
}

func (r *reader) failure() error {
	failure := r.data.Err()
	switch {
	case r.problem != nil:
		return r.problem
	case failure == nil:
		return nil
	case failure.Unterminated:
		return errUnterminated(r.file)
	}
	return errTruncated(r.file)
}

func (r *reader) whole() *File {
	r.version = r.data.I32()
	if r.version < 1 || r.version > 3 {
		r.refuse(errVersion(r.file, r.version))
	}
	parsed := &File{Version: r.version}
	parsed.Original = r.table()
	parsed.Custom = r.table()
	if r.data.Len() != 0 {
		r.refuse(errTrailing(r.file))
	}
	return parsed
}

func (r *reader) table() Table {
	table := Table{CountOffset: r.data.Offset(), Objects: []Object{}}
	r.times(r.count("object", r.smallestObject()), func() {
		table.Objects = append(table.Objects, r.object())
	})
	table.Start, table.Stop = table.CountOffset+4, r.data.Offset()
	return table
}

func (r *reader) object() Object {
	object := Object{Start: r.data.Offset()}
	object.Base, object.ID = r.id(), r.id()
	r.times(r.setCount(), func() {
		object.Sets = append(object.Sets, r.set())
	})
	object.Stop = r.data.Offset()
	return object
}

func (r *reader) setCount() int {
	if r.version < 3 {
		return 1
	}
	count := r.data.I32()
	if count < 1 || count > maxSets {
		r.refuse(errSetCount(r.file, count))
		return 0
	}
	return int(count)
}

func (r *reader) set() Set {
	set := Set{Mods: []Modification{}}
	if r.version >= 3 {
		set.Flag = r.data.I32()
	}
	r.times(r.count("modification", r.smallestModification()), func() {
		set.Mods = append(set.Mods, r.modification())
	})
	return set
}

func (r *reader) modification() Modification {
	mod := Modification{Start: r.data.Offset()}
	mod.Field = r.id()
	valueType := r.data.I32()
	if r.kind == Leveled {
		mod.Level, mod.Column = r.data.I32(), r.data.I32()
	}
	mod.Value = r.value(valueType)
	mod.End = r.id()
	mod.Stop = r.data.Offset()
	return mod
}

func (r *reader) value(number int32) Value {
	switch valueType := ValueType(number); valueType {
	case Int:
		return Value{Type: Int, Int: r.data.I32()}
	case Real, Unreal:
		return Value{Type: valueType, Real: r.data.F32()}
	case String:
		return Value{Type: String, Text: r.text()}
	}
	r.refuse(errValueType(r.file, number))
	return Value{}
}

func (r *reader) count(what string, smallest int) int {
	count := r.data.I32()
	if count < 0 || int64(count)*int64(smallest) > int64(r.data.Len()) {
		r.refuse(errCount(r.file, what, count))
	}
	if r.stopped() {
		return 0
	}
	return int(count)
}

func (r *reader) times(count int, read func()) {
	for range count {
		if r.stopped() {
			return
		}
		read()
	}
}

func (r *reader) smallestObject() int {
	if r.version >= 3 {
		return 20
	}
	return 12
}

func (r *reader) smallestModification() int {
	if r.kind == Leveled {
		return 21
	}
	return 13
}

func (r *reader) id() ID {
	var id ID
	copy(id[:], r.data.Bytes(len(id)))
	return id
}

func (r *reader) text() string {
	raw := r.data.CString()
	if !utf8.Valid(raw) {
		r.refuse(errNotUTF8(r.file))
	}
	return fsx.WithoutMark(string(raw))
}

func errUnreadable(file, problem string) error {
	return &diag.Error{
		Msg:  "Cannot read object data: " + problem + ".",
		File: file,
		Hint: "Open and re-save this map in World Editor 3.00.",
	}
}

func errVersion(file string, version int32) error {
	return errUnreadable(file, fmt.Sprintf("unsupported version %d", version))
}

func errTruncated(file string) error {
	return errUnreadable(file, "truncated file")
}

func errUnterminated(file string) error {
	return errUnreadable(file, "unterminated string")
}

func errNotUTF8(file string) error {
	return errUnreadable(file, "invalid UTF-8 in a string")
}

func errCount(file, what string, count int32) error {
	return errUnreadable(file, fmt.Sprintf("%s count %d past end of file", what, count))
}

func errSetCount(file string, count int32) error {
	return errUnreadable(file, fmt.Sprintf("unsupported set count %d", count))
}

func errValueType(file string, number int32) error {
	return errUnreadable(file, fmt.Sprintf("unknown value type %d", number))
}

func errTrailing(file string) error {
	return errUnreadable(file, "trailing bytes after the custom objects")
}
