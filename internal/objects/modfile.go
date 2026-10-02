package objects

import (
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/binio"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// TableKind says whether a modification file stores a level and a data pointer with each modification.
type TableKind int

const (
	// Simple tables (w3u, w3t, w3b, w3h) have neither.
	Simple TableKind = iota
	// Leveled tables (w3a, w3d, w3q) have both.
	Leveled
)

var leveledFile = regexp.MustCompile(`(?i)\.(w3a|w3d|w3q)$`)

// KindOf returns the table kind of a modification file by its extension.
func KindOf(fileName string) TableKind {
	if leveledFile.MatchString(fileName) {
		return Leveled
	}
	return Simple
}

// ModValue is the value of a modification. Type is int, real, unreal or string; Number holds the first three.
type ModValue struct {
	Type   string
	Number float64
	Text   string
}

// IntValue, RealValue and TextValue build values.
func IntValue(n int32) ModValue                 { return ModValue{Type: "int", Number: float64(n)} }
func RealValue(kind string, n float64) ModValue { return ModValue{Type: kind, Number: n} }
func TextValue(s string) ModValue               { return ModValue{Type: "string", Text: s} }

// Modification is one changed field of an object. Level and Column (the data pointer) are 0 in simple tables; End
// is the end token, four NUL characters for 0.
type Modification struct {
	Field       string
	Level       int32
	Column      int32
	Value       ModValue
	End         string
	Start, Stop int
}

// ModSet is one set of modifications; version 1 and 2 objects have one implicit set with flag 0.
type ModSet struct {
	Flag int32
	Mods []Modification
}

// ObjectEntry is one object of a table. Start and Stop let the writer copy the object verbatim.
type ObjectEntry struct {
	Base, ID    string
	Sets        []ModSet
	Start, Stop int
}

// ModTable is one of a file's two tables. CountOffset is where its object count sits; Start and Stop span the
// objects after it.
type ModTable struct {
	CountOffset, Start, Stop int
	Objects                  []ObjectEntry
}

// ModFile is a modification file: the standard objects it changes, and the custom objects.
type ModFile struct {
	Version  int32
	Original ModTable
	Custom   ModTable
}

var varTypes = []string{"int", "real", "unreal", "string"}

// World Editor 3.00 writes one set per object; the limit only rejects garbage counts.
const maxSets = 64

// latin1 reads bytes as Latin-1 characters, as the files store ids.
func latin1(data []byte) string {
	var b strings.Builder
	for _, c := range data {
		b.WriteRune(rune(c))
	}
	return b.String()
}

// fromLatin1 returns the four bytes of an id; false when it is not four Latin-1 characters.
func fromLatin1(id string) ([]byte, bool) {
	out := make([]byte, 0, 4)
	for _, r := range id {
		if r > 0xff || r == utf8.RuneError {
			return nil, false
		}
		out = append(out, byte(r))
	}
	return out, len(out) == 4
}

// modReader keeps its first failure and reads nothing after it.
type modReader struct {
	data *binio.Reader
	size int
	file string
	err  error
}

func (r *modReader) invalid(problem string) {
	if r.err == nil {
		r.err = &diag.Error{
			Msg:  "Cannot read object data: " + problem + ".",
			File: r.file,
			Hint: "Open and re-save this map in World Editor 3.00.",
		}
	}
}

func (r *modReader) check() {
	if failure := r.data.Err(); failure != nil {
		if failure.Unterminated {
			r.invalid("unterminated string")
		} else {
			r.invalid("truncated file")
		}
	}
}

func (r *modReader) int() int32 {
	value := r.data.I32()
	r.check()
	return value
}

func (r *modReader) float() float32 {
	value := r.data.F32()
	r.check()
	return value
}

func (r *modReader) id() string {
	raw := r.data.Bytes(4)
	r.check()
	return latin1(raw)
}

func (r *modReader) text() string {
	raw := r.data.CString()
	r.check()
	if !utf8.Valid(raw) {
		r.invalid("invalid UTF-8 in a string")
	}
	return string(raw)
}

// count reads a count and rejects one whose items could not fit in the remaining bytes.
func (r *modReader) count(what string, minSize int) int {
	count := r.int()
	if r.err == nil && (count < 0 || int64(count)*int64(minSize) > int64(r.size-r.data.Offset())) {
		r.invalid(fmt.Sprintf("%s count %d past end of file", what, count))
	}
	if r.err != nil {
		return 0
	}
	return int(count)
}

func readTable(r *modReader, version int32, kind TableKind) ModTable {
	leveled := kind == Leveled
	// Smallest encodings: an empty object, and a modification with a one-byte (empty) string.
	minObject, minMod := 12, 8+1+4
	if version >= 3 {
		minObject = 20
	}
	if leveled {
		minMod = 16 + 1 + 4
	}
	table := ModTable{CountOffset: r.data.Offset(), Objects: []ObjectEntry{}}
	for range r.count("object", minObject) {
		object := ObjectEntry{Start: r.data.Offset()}
		object.Base, object.ID = r.id(), r.id()
		setCount := int32(1)
		if version >= 3 {
			setCount = r.int()
		}
		if r.err == nil && (setCount < 1 || setCount > maxSets) {
			r.invalid(fmt.Sprintf("unsupported set count %d", setCount))
		}
		if r.err != nil {
			break
		}
		for range setCount {
			set := ModSet{Mods: []Modification{}}
			if version >= 3 {
				set.Flag = r.int()
			}
			for range r.count("modification", minMod) {
				mod := Modification{Start: r.data.Offset()}
				mod.Field = r.id()
				varType := r.int()
				if leveled {
					mod.Level, mod.Column = r.int(), r.int()
				}
				if r.err == nil && (varType < 0 || int(varType) >= len(varTypes)) {
					r.invalid(fmt.Sprintf("unknown value type %d", varType))
				}
				if r.err != nil {
					break
				}
				mod.Value.Type = varTypes[varType]
				switch mod.Value.Type {
				case "string":
					mod.Value.Text = r.text()
				case "int":
					mod.Value.Number = float64(r.int())
				default:
					mod.Value.Number = float64(r.float())
				}
				mod.End = r.id()
				mod.Stop = r.data.Offset()
				set.Mods = append(set.Mods, mod)
			}
			object.Sets = append(object.Sets, set)
		}
		object.Stop = r.data.Offset()
		table.Objects = append(table.Objects, object)
	}
	table.Start, table.Stop = table.CountOffset+4, r.data.Offset()
	return table
}

// ReadModFile reads a modification file (w3u, w3t, w3b, w3d, w3a, w3h, w3q and their skin files). file is the name
// errors give.
func ReadModFile(data []byte, kind TableKind, file string) (*ModFile, error) {
	r := &modReader{data: binio.NewReader(data), size: len(data), file: file}
	version := r.int()
	if r.err == nil && version != 1 && version != 2 && version != 3 {
		r.invalid(fmt.Sprintf("unsupported version %d", version))
	}
	parsed := &ModFile{Version: version}
	if r.err == nil {
		parsed.Original = readTable(r, version, kind)
	}
	if r.err == nil {
		parsed.Custom = readTable(r, version, kind)
	}
	if r.err == nil && r.data.Offset() != len(data) {
		r.invalid("trailing bytes after the custom objects")
	}
	if r.err != nil {
		return nil, r.err
	}
	return parsed, nil
}

// NewMod is one modification of an object Moonwell adds. Level and Column must be 0 in simple tables.
type NewMod struct {
	Field  string
	Level  int64
	Column int64
	Value  ModValue
}

// NewObject is an object Moonwell adds to the custom table.
type NewObject struct {
	Base, ID string
	Mods     []NewMod
}

// Version 3 objects get set count 1 and flag 0, and every custom modification the end token 0 (names fixture).
// Versions 1 and 2 have no set fields and end token 0, as the reference library writes version 2 files.
const (
	setCount = 1
	setFlag  = 0
	endToken = 0
	// NewFileVersion is the version of a file World Editor 3.00 writes when the map has none (names fixture); such
	// a file has an empty original table.
	NewFileVersion = 3
	float32Max     = 3.4028234663852886e38
)

// modWriter encodes Moonwell's objects. A bad value is an internal error: the resolver rejects them before
// planning.
type modWriter struct {
	out []byte
	err error
}

func (w *modWriter) fail(format string, args ...any) {
	if w.err == nil {
		w.err = fmt.Errorf(format, args...)
	}
}

func (w *modWriter) int(value float64) {
	if value != math.Trunc(value) || value < -(1<<31) || value >= 1<<31 || math.IsNaN(value) {
		w.fail("Cannot write %s as an int32.", text.Number(value))
		return
	}
	w.out = binary.LittleEndian.AppendUint32(w.out, uint32(int32(value)))
}

func (w *modWriter) float(value float64) {
	if math.IsInf(value, 0) || math.IsNaN(value) || math.Abs(value) > float32Max {
		w.fail("Cannot write %s as a float32.", text.Number(value))
		return
	}
	w.out = binary.LittleEndian.AppendUint32(w.out, math.Float32bits(float32(value)))
}

func (w *modWriter) id(value string) {
	raw, ok := fromLatin1(value)
	if !ok {
		w.fail("Cannot write %s as an object id: it must be 4 Latin-1 characters.", text.Quote(value))
		return
	}
	w.out = append(w.out, raw...)
}

func (w *modWriter) text(value string) {
	if strings.Contains(value, "\x00") || !utf8.ValidString(value) {
		w.fail("Cannot write %s: it contains NUL or an unpaired surrogate.", text.Quote(value))
		return
	}
	w.out = append(append(w.out, value...), 0)
}

func (w *modWriter) object(object NewObject, version int32, kind TableKind) {
	w.id(object.Base)
	w.id(object.ID)
	if version >= 3 {
		w.int(setCount)
		w.int(setFlag)
	}
	w.int(float64(len(object.Mods)))
	for _, mod := range object.Mods {
		w.id(mod.Field)
		w.int(float64(slices.Index(varTypes, mod.Value.Type)))
		if kind == Leveled {
			w.int(float64(mod.Level))
			w.int(float64(mod.Column))
		} else if mod.Level != 0 || mod.Column != 0 {
			w.fail("Cannot write %s at level %d, column %d: a simple table has neither.", mod.Field, mod.Level, mod.Column)
		}
		switch mod.Value.Type {
		case "string":
			w.text(mod.Value.Text)
		case "int":
			w.int(mod.Value.Number)
		default:
			w.float(mod.Value.Number)
		}
		w.int(endToken)
	}
}

// AppendObjects appends objects, in the order given, to the custom table of source, copying every existing byte
// verbatim. With a nil source it writes a new file with an empty original table.
func AppendObjects(source []byte, kind TableKind, objects []NewObject, file string) ([]byte, error) {
	w := &modWriter{}
	if source == nil {
		w.int(NewFileVersion)
		w.int(0)
		w.int(float64(len(objects)))
		for _, object := range objects {
			w.object(object, NewFileVersion, kind)
		}
		return w.out, w.err
	}
	parsed, err := ReadModFile(source, kind, file)
	if err != nil {
		return nil, err
	}
	custom := parsed.Custom
	w.out = append(w.out, source[:custom.CountOffset]...)
	w.int(float64(len(custom.Objects) + len(objects)))
	// The custom table runs to the end of the file: the reader rejects trailing bytes.
	w.out = append(w.out, source[custom.Start:custom.Stop]...)
	for _, object := range objects {
		w.object(object, parsed.Version, kind)
	}
	return w.out, w.err
}
