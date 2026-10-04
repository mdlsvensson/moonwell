package objmod

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/next/internal/binio"
)

// NewFileVersion is the version of a file World Editor 3.00 writes when the map has none.
const NewFileVersion = 3

// NewMod is one modification of an object to append. Level and Column must be 0 in a simple table. Value must
// have one of the four types, and a Real or an Unreal must be a finite number.
type NewMod struct {
	Field  ID
	Level  int32
	Column int32
	Value  Value
}

// NewObject is an object to append to the custom table.
type NewObject struct {
	Base, ID ID
	Mods     []NewMod
}

// Append adds objects, in order, to the custom table of source and copies everything else. A nil source gives a
// new file with an empty original table. The source is not changed and the result shares no bytes with it.
//
// A source that does not read is Read's error. Any other error is a bug in the caller, not a problem with the
// map, and so is not a diag error. Append refuses, and then returns no bytes:
//
//   - an object whose Base or ID, or a modification whose Field, is four NUL bytes: an id that was never set;
//   - a value whose Type is none of the four;
//   - a Level or a Column other than 0 in a simple table, which stores neither;
//   - a String whose Text has a NUL, which would end it early, or is not UTF-8;
//   - a Real or an Unreal that is infinite or not a number.
//
// It does not judge what a value means: any int, any finite real and any other text is written. Only the objects
// it adds are checked. What the source holds is copied as it is.
func Append(source []byte, kind TableKind, objects []NewObject, file string) ([]byte, error) {
	w, err := begin(source, kind, len(objects), file)
	if err != nil {
		return nil, err
	}
	if err := unwritable(objects, kind); err != nil {
		return nil, err
	}
	for _, object := range objects {
		w.object(object)
	}
	return w.out.Bytes(), nil
}

// writer writes objects after what a file already holds, in the layout of the file's version and table kind.
type writer struct {
	out     binio.Writer
	version int32
	kind    TableKind
}

// begin returns a writer that holds everything that comes before the new objects: the source with the count of its
// custom table raised by added, or for a nil source the start of a new file.
func begin(source []byte, kind TableKind, added int, file string) (*writer, error) {
	if source == nil {
		w := &writer{version: NewFileVersion, kind: kind}
		w.out.I32(NewFileVersion)
		w.out.I32(0) // no original objects
		w.out.I32(int32(added))
		return w, nil
	}
	parsed, err := Read(source, kind, file)
	if err != nil {
		return nil, err
	}
	w := &writer{version: parsed.Version, kind: kind}
	custom := parsed.Custom
	w.out.Write(source[:custom.CountOffset])
	w.out.I32(int32(len(custom.Objects) + added))
	// The custom table runs to the end of the file: Read refuses bytes after it.
	w.out.Write(source[custom.Start:custom.Stop])
	return w, nil
}

// object writes one object as World Editor 3.00 writes a custom one. Version 3 stores a count of sets and a flag
// with each set, and the object gets one set with flag 0; versions 1 and 2 store neither.
func (w *writer) object(object NewObject) {
	w.out.Write(object.Base[:])
	w.out.Write(object.ID[:])
	if w.version >= 3 {
		w.out.I32(1) // one set
		w.out.I32(0) // its flag
	}
	w.out.I32(int32(len(object.Mods)))
	for _, mod := range object.Mods {
		w.modification(mod)
	}
}

// modification writes one modification with the end token World Editor 3.00 writes, four NUL bytes. The type goes
// into the file as its number.
func (w *writer) modification(mod NewMod) {
	w.out.Write(mod.Field[:])
	w.out.I32(int32(mod.Value.Type))
	if w.kind == Leveled {
		w.out.I32(mod.Level)
		w.out.I32(mod.Column)
	}
	switch mod.Value.Type {
	case Int:
		w.out.I32(mod.Value.Int)
	case String:
		w.out.CString(mod.Value.Text)
	default: // Real and Unreal
		w.out.F32(mod.Value.Real)
	}
	w.out.I32(0) // the end token
}

// unwritable returns why the objects cannot be written, or nil: the first thing wrong with them, in the order
// they would be written.
func unwritable(objects []NewObject, kind TableKind) error {
	for _, object := range objects {
		if err := refused(object, kind); err != nil {
			return err
		}
	}
	return nil
}

// refused returns why one object cannot be written, or nil. It looks at the object in the order of its bytes: its
// two ids, then of each modification the field, the type, the level and the column, and the value. Every reason
// is a bug in the caller, so none is a diag error: each must be reported as Moonwell's own fault.
func refused(object NewObject, kind TableKind) error {
	if object.Base == (ID{}) {
		return fmt.Errorf("Cannot write the object %q: its base is four NUL bytes.", object.ID)
	}
	if object.ID == (ID{}) {
		return fmt.Errorf("Cannot write an object based on %q: its id is four NUL bytes.", object.Base)
	}
	for _, mod := range object.Mods {
		value := mod.Value
		switch {
		case mod.Field == (ID{}):
			return fmt.Errorf("Cannot write a modification of %q: its field is four NUL bytes.", object.ID)
		case value.Type < Int || value.Type > String:
			return fmt.Errorf("Cannot write %s: its value type %d is none of the four.", mod.Field, value.Type)
		case kind == Simple && (mod.Level != 0 || mod.Column != 0):
			return fmt.Errorf("Cannot write %s at level %d, column %d: a simple table has neither.",
				mod.Field, mod.Level, mod.Column)
		case value.Type == String && !storable(value.Text):
			return fmt.Errorf("Cannot write %q: it contains NUL or invalid UTF-8.", value.Text)
		case (value.Type == Real || value.Type == Unreal) && !finite(value.Real):
			return fmt.Errorf("Cannot write %v as a float32.", value.Real)
		}
	}
	return nil
}

// storable reports whether a file can hold the text: a NUL would end it early, and the file's texts are UTF-8.
func storable(text string) bool {
	return !strings.Contains(text, "\x00") && utf8.ValidString(text)
}

// finite reports whether a real is a number and not infinite.
func finite(number float32) bool {
	return !math.IsNaN(float64(number)) && !math.IsInf(float64(number), 0)
}
