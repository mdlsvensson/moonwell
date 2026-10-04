package objects

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// stored returns one value of the manifest as the field stores it. It is false, after reporting why, for a value
// that the field cannot store, so that every value it returns can be written to an object file: an integer fits
// an int32, a real fits a float32, and a text is UTF-8 without a NUL.
func (s *subject) stored(field *FieldMeta, value any, path string) (Value, bool) {
	switch {
	case field.List:
		return s.storedList(field, value, path)
	case field.Storage == "int":
		return s.storedInt(field, value, path)
	case field.Storage == "real" || field.Storage == "unreal":
		return s.storedReal(field, value, path)
	}
	return s.storedText(field, value, path)
}

// storedList is a list as one text, its entries joined by commas. A text is taken as the list already joined.
func (s *subject) storedList(field *FieldMeta, value any, path string) (Value, bool) {
	if text, isText := value.(string); isText {
		return s.storedText(field, text, path)
	}
	entries, isList := value.([]any)
	if !isList {
		s.report(path, errNotA("a string or a List<String>", value, field))
		return Value{}, false
	}
	parts, storable := make([]string, len(entries)), true
	for i, entry := range entries {
		text, ok := s.storedText(field, entry, fmt.Sprintf("%s[%d]", path, i))
		parts[i], storable = text.Text, storable && ok
	}
	return Value{Type: objmod.String, Text: strings.Join(parts, ",")}, storable
}

// storedInt is a whole number, or a Boolean as 1 or 0. The number is held as the int32 the file stores, so the
// zero below 0, which Pkl prints as -0.0, is 0.
func (s *subject) storedInt(field *FieldMeta, value any, path string) (Value, bool) {
	if truth, isBool := value.(bool); isBool {
		if truth {
			return Value{Type: objmod.Int, Number: 1}, true
		}
		return Value{Type: objmod.Int}, true
	}
	whole, isNumber := value.(float64)
	switch {
	case !isNumber || whole != math.Trunc(whole) || math.IsInf(whole, 0):
		expected := "an integer"
		if field.Type == "bool" {
			expected = "a Boolean"
		}
		s.report(path, errNotA(expected, value, field))
	case whole < math.MinInt32 || whole > math.MaxInt32:
		s.report(path, errOutOfRange(whole, "an integer", field))
	default:
		return Value{Type: objmod.Int, Number: float64(int32(whole))}, true
	}
	return Value{}, false
}

// storedReal is a number that a float32 holds: the files store reals in four bytes. The zero below 0 keeps its
// sign, as those bytes do.
func (s *subject) storedReal(field *FieldMeta, value any, path string) (Value, bool) {
	amount, isNumber := value.(float64)
	switch {
	case !isNumber:
		s.report(path, errNotA("a number", value, field))
	case math.IsNaN(amount) || math.Abs(amount) > math.MaxFloat32:
		s.report(path, errOutOfRange(amount, "a real number", field))
	default:
		kind := objmod.Unreal
		if field.Storage == "real" {
			kind = objmod.Real
		}
		return Value{Type: kind, Number: amount}, true
	}
	return Value{}, false
}

// storedText is a text as the files store one: UTF-8, and without a NUL, which would end it in the file.
func (s *subject) storedText(field *FieldMeta, value any, path string) (Value, bool) {
	text, isText := value.(string)
	switch {
	case !isText:
		s.report(path, errNotA("a string", value, field))
	case strings.Contains(text, "\x00"):
		s.report(path, errHasNUL())
	case !utf8.ValidString(text):
		s.report(path, errNotUTF8())
	default:
		return Value{Type: objmod.String, Text: text}, true
	}
	return Value{}, false
}

// number writes a number for a message, in plain decimal. A message has one zero: the zero below 0 is 0.
func number(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// shown writes a value of the manifest for a message, as JSON with no space in it: a number in plain decimal, a
// text between quotes with its characters as they are, and the keys of an object sorted. Pkl lets through neither
// an object nor a null, which are shown all the same.
func shown(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return number(v)
	case string:
		return quoted(v)
	case []any:
		entries := make([]string, len(v))
		for i, entry := range v {
			entries[i] = shown(entry)
		}
		return "[" + strings.Join(entries, ",") + "]"
	case map[string]any:
		entries := make([]string, 0, len(v))
		for _, key := range slices.Sorted(maps.Keys(v)) {
			entries = append(entries, quoted(key)+":"+shown(v[key]))
		}
		return "{" + strings.Join(entries, ",") + "}"
	}
	return fmt.Sprint(value) // no value that JSON decodes to is of another type
}

// ---- errors ----

// storedAs says how the field stores its value, for the hint of a value it cannot store.
func storedAs(field *FieldMeta) string {
	kind := map[string]string{"int": "an integer", "real": "a real number", "unreal": "a real number"}[field.Storage]
	switch {
	case field.List:
		kind = "a comma-separated list"
	case field.Storage == "int" && field.Type == "bool":
		kind = "a Boolean (1 or 0)"
	case kind == "":
		kind = "a string"
	}
	return describeField(field) + " is stored as " + kind + "."
}

func errNotA(expected string, value any, field *FieldMeta) fault {
	return fault{"expected " + expected + ", got " + shown(value) + ".", storedAs(field)}
}

func errOutOfRange(value float64, kind string, field *FieldMeta) fault {
	return fault{number(value) + " is out of range for " + kind + ".", storedAs(field)}
}

func errHasNUL() fault {
	return fault{"the string contains a NUL character.", "Remove it: the game ends strings at NUL."}
}

func errNotUTF8() fault {
	return fault{"the string is not valid UTF-8.", "The files of a map store text as UTF-8."}
}
