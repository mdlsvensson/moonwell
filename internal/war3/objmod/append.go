package objmod

import (
	"fmt"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/binio"
)

const NewFileVersion = 3

const (
	originalObjectCount = 0
	newObjectSetCount   = 1
	newObjectSetFlag    = 0
)

var emptyEndToken ID

type NewMod struct {
	Field  ID
	Level  int32
	Column int32
	Value  Value
}

type NewObject struct {
	Base, ID ID
	Mods     []NewMod
}

func AppendObjects(parsed *File, source []byte, kind TableKind, objects []NewObject) ([]byte, error) {
	if err := checkWritable(objects, kind); err != nil {
		return nil, err
	}
	w := newWriter(parsed, source, kind, len(objects))
	for _, object := range objects {
		w.writeObject(object)
	}
	return w.out.Bytes(), nil
}

type writer struct {
	out     binio.Writer
	version int32
	kind    TableKind
}

func newWriter(parsed *File, source []byte, kind TableKind, added int) *writer {
	if parsed == nil {
		w := &writer{version: NewFileVersion, kind: kind}
		w.out.I32(NewFileVersion)
		w.out.I32(originalObjectCount)
		w.out.I32(int32(added))
		return w
	}
	w := &writer{version: parsed.Version, kind: kind}
	custom := parsed.Custom
	w.out.Write(source[:custom.CountOffset])
	w.out.I32(int32(len(custom.Objects) + added))
	w.out.Write(source[custom.Start:custom.Stop])
	return w
}

func (w *writer) writeObject(object NewObject) {
	w.out.Write(object.Base[:])
	w.out.Write(object.ID[:])
	if w.version >= 3 {
		w.out.I32(newObjectSetCount)
		w.out.I32(newObjectSetFlag)
	}
	w.out.I32(int32(len(object.Mods)))
	for _, mod := range object.Mods {
		w.writeModification(mod)
	}
}

func (w *writer) writeModification(mod NewMod) {
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
	default:
		w.out.F32(mod.Value.Real)
	}
	w.out.Write(emptyEndToken[:])
}

func checkWritable(objects []NewObject, kind TableKind) error {
	for _, object := range objects {
		if err := checkObject(object, kind); err != nil {
			return err
		}
	}
	return nil
}

func checkObject(object NewObject, kind TableKind) error {
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
		case value.Type == String && !isStorableText(value.Text):
			return fmt.Errorf("Cannot write %q: it contains NUL or invalid UTF-8.", value.Text)
		case (value.Type == Real || value.Type == Unreal) && !isFinite(value.Real):
			return fmt.Errorf("Cannot write %v as a float32.", value.Real)
		}
	}
	return nil
}

func isStorableText(text string) bool {
	return !strings.Contains(text, "\x00") && utf8.ValidString(text)
}

func isFinite(number float32) bool {
	return !math.IsNaN(float64(number)) && !math.IsInf(float64(number), 0)
}
