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

// TableKind says whether a file stores a level and a data pointer with each modification.
type TableKind int

const (
	Simple  TableKind = iota // w3u, w3t, w3b, w3h: no level and no data pointer
	Leveled                  // w3a, w3d, w3q: both
)

// KindOf returns the table kind of a modification file by the extension of its name, in either letter case.
func KindOf(fileName string) TableKind {
	switch strings.ToLower(path.Ext(fileName)) {
	case ".w3a", ".w3d", ".w3q":
		return Leveled
	}
	return Simple
}

// ValueType is the type of a modification's value. Its number is the one the file stores for the type.
type ValueType int

const (
	Int ValueType = iota
	Real
	Unreal
	String
)

// Value is the value of a modification, in the field its type names; the other two are zero in a value Read
// returns.
type Value struct {
	Type ValueType
	Int  int32
	Real float32 // for Real and Unreal
	Text string
}

// Modification is one changed field of an object. Level and Column (the data pointer) are 0 in a simple table,
// which stores neither. End is the token that closes the modification: four NUL bytes in what World Editor 3.00
// saves. Start and Stop are the offsets its bytes run between.
type Modification struct {
	Field       ID
	Level       int32
	Column      int32
	Value       Value
	End         ID
	Start, Stop int
}

// Set is one set of modifications. A version 3 file stores the sets of an object and a flag with each; an object
// of version 1 or 2 has one set, with flag 0.
type Set struct {
	Flag int32
	Mods []Modification
}

// Object is one object of a table. In the original table ID is four NUL bytes: the object is the standard one
// that Base names. Start and Stop are the offsets its bytes run between.
type Object struct {
	Base, ID    ID
	Sets        []Set
	Start, Stop int
}

// Table is one of a file's two tables. CountOffset is where its count of objects is stored; the objects run from
// Start to Stop.
type Table struct {
	CountOffset, Start, Stop int
	Objects                  []Object
}

// File is a modification file: the standard objects it changes, and the custom objects.
type File struct {
	Version  int32
	Original Table
	Custom   Table
}

// maxSets is the most sets an object may have. World Editor 3.00 writes one per object; the limit only refuses a
// count that cannot be one.
const maxSets = 64

// Read reads an object modification file. file is the name its errors give. A file that does not read gives nil
// and a diag error that says the first thing wrong with it.
func Read(data []byte, kind TableKind, file string) (*File, error) {
	r := &reader{data: binio.NewReader(data), kind: kind, file: file}
	parsed := r.whole()
	if err := r.failure(); err != nil {
		return nil, err
	}
	return parsed, nil
}

// reader reads the file front to back. The byte reader keeps its first failure and returns zeroes after it, so the
// parts below read straight through and Read asks once what went wrong. A value that is wrong is passed to refuse,
// which drops it when the bytes had already run out: such a value is a zero that was never in the file. The error
// is then the first thing wrong in the file, in the order of its bytes.
type reader struct {
	data    *binio.Reader
	kind    TableKind
	file    string
	version int32
	problem error
}

// refuse records a problem with a value, unless an earlier problem is recorded or the bytes ran out before it.
func (r *reader) refuse(problem error) {
	if !r.stopped() {
		r.problem = problem
	}
}

// stopped reports whether something is wrong with what was read so far.
func (r *reader) stopped() bool {
	return r.problem != nil || r.data.Err() != nil
}

// failure is the first thing wrong with what was read, or nil.
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

// ---- the file, in the order of its bytes ----

// whole reads the version, which decides the layout of an object, then the two tables. Nothing may follow them.
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

// table reads a count of objects and then that many objects.
func (r *reader) table() Table {
	table := Table{CountOffset: r.data.Offset(), Objects: []Object{}}
	r.times(r.count("object", r.smallestObject()), func() {
		table.Objects = append(table.Objects, r.object())
	})
	table.Start, table.Stop = table.CountOffset+4, r.data.Offset()
	return table
}

// object reads the base and the id of an object and then its sets.
func (r *reader) object() Object {
	object := Object{Start: r.data.Offset()}
	object.Base, object.ID = r.id(), r.id()
	r.times(r.setCount(), func() {
		object.Sets = append(object.Sets, r.set())
	})
	object.Stop = r.data.Offset()
	return object
}

// setCount is the number of sets of the object being read: stored from version 3 on, and one before it.
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

// set reads the flag of a set, which a file has from version 3 on, then a count of modifications and that many
// modifications.
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

// modification reads a field, the number of a value type, in a leveled table a level and a column, then the value
// and the end token. The type is checked after the level and the column are read, so a file that ends inside them
// is cut short and not of an unknown type.
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

// value reads a value of the type the file gave by its number.
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

// ---- counts ----

// count reads how many objects or modifications follow. It refuses a count below zero, and one whose items could
// not fit in the bytes that are left when each has its smallest size. After a problem the count is none, so that
// nothing is read as an item.
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

// times reads count items, and stops at the first problem: what follows it is not what the file meant.
func (r *reader) times(count int, read func()) {
	for range count {
		if r.stopped() {
			return
		}
		read()
	}
}

// smallestObject is the size of an object without modifications: its base, its id and a count of modifications,
// and from version 3 on a count of sets and the flag of one set.
func (r *reader) smallestObject() int {
	if r.version >= 3 {
		return 20
	}
	return 12
}

// smallestModification is the size of a modification whose value is an empty string: its field, its type, the
// NUL and the end token, and in a leveled table the level and the data pointer.
func (r *reader) smallestModification() int {
	if r.kind == Leveled {
		return 21
	}
	return 13
}

// ---- values ----

// id reads the four bytes of an id. Any four bytes are one.
func (r *reader) id() ID {
	var id ID
	copy(id[:], r.data.Bytes(len(id)))
	return id
}

// text reads a NUL-terminated string, which must be UTF-8. A byte order mark at its start is not part of the
// value.
func (r *reader) text() string {
	raw := r.data.CString()
	if !utf8.Valid(raw) {
		r.refuse(errNotUTF8(r.file))
	}
	return fsx.WithoutMark(string(raw))
}

// ---- errors ----

// errUnreadable says that the file cannot be read as object data, and why.
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

// errCount says that a count of objects or of modifications is more than the file has room for.
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
