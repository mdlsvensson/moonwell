package objects

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// stored returns one value of the manifest as the field stores it. It is false, after reporting why, for a value
// that the field cannot store, so that every value it returns can be written to an object file: an integer fits
// an int32, a real fits a float32, and a text has no NUL.
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

// storedInt is a whole number, or a Boolean as 1 or 0.
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
		return Value{Type: objmod.Int, Number: whole}, true
	}
	return Value{}, false
}

// storedReal is a number that a float32 holds: the files store reals in four bytes.
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

// storedText is a text without a NUL, which would end it in the file.
func (s *subject) storedText(field *FieldMeta, value any, path string) (Value, bool) {
	text, isText := value.(string)
	switch {
	case !isText:
		s.report(path, errNotA("a string", value, field))
	case strings.Contains(text, "\x00"):
		s.report(path, errHasNUL())
	default:
		return Value{Type: objmod.String, Text: text}, true
	}
	return Value{}, false
}

// number writes a number for a message, in plain decimal.
func number(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// shown writes a value of the manifest for a message: a number in plain decimal, anything else as JSON.
func shown(value any) string {
	switch v := value.(type) {
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
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Sprint(value)
	}
	return string(encoded)
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
