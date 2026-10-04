package objmod

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/next/internal/binio"
)

// NewFileVersion is the version of a file World Editor 3.00 writes when the map has none.
const NewFileVersion = 3

// NewMod is one modification of an object to append. Level and Column must be 0 in a simple table.
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
// new file with an empty original table. A source that does not read is Read's error. Any other error is a bug in
// the caller, not a problem with the map: a level or a column in a simple table, or a text the file cannot hold.
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
// into the file as its number, and a type that is none of the four is written with the value of a real.
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

// unwritable returns why the objects cannot be written, or nil: the first modification, in the order they are
// written, that a file of this kind has no way to hold.
func unwritable(objects []NewObject, kind TableKind) error {
	for _, object := range objects {
		for _, mod := range object.Mods {
			if err := refused(mod, kind); err != nil {
				return err
			}
		}
	}
	return nil
}

// refused returns why one modification cannot be written, or nil. A simple table has no place for a level or a
// column. A text ends at its first NUL, and the file's texts are UTF-8.
func refused(mod NewMod, kind TableKind) error {
	text := mod.Value.Text
	switch {
	case kind == Simple && (mod.Level != 0 || mod.Column != 0):
		return errNoLevels(mod)
	case mod.Value.Type == String && (strings.Contains(text, "\x00") || !utf8.ValidString(text)):
		return errText(text)
	}
	return nil
}

// ---- errors ----

// errNoLevels is not a diag error: it must be reported as Moonwell's own fault.
func errNoLevels(mod NewMod) error {
	return fmt.Errorf("Cannot write %s at level %d, column %d: a simple table has neither.",
		mod.Field, mod.Level, mod.Column)
}

// errText is not a diag error: it must be reported as Moonwell's own fault.
func errText(text string) error {
	return fmt.Errorf("Cannot write %q: it contains NUL or invalid UTF-8.", text)
}
