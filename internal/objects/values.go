package objects

import (
	"fmt"
	"maps"
	"math"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/war3/objmod"
)

func (s *objectResolver) toValue(field *FieldMeta, value any, path string) (Value, bool) {
	switch {
	case field.List:
		return s.toListValue(field, value, path)
	case field.Storage == "int":
		return s.toIntValue(field, value, path)
	case field.Storage == "real" || field.Storage == "unreal":
		return s.toRealValue(field, value, path)
	}
	return s.toTextValue(field, value, path)
}

func (s *objectResolver) toListValue(field *FieldMeta, value any, path string) (Value, bool) {
	if text, isText := value.(string); isText {
		return s.toTextValue(field, text, path)
	}
	entries, isList := value.([]any)
	if !isList {
		s.report(path, errWrongType("a string or a List<String>", value, field))
		return Value{}, false
	}
	parts, storable := make([]string, len(entries)), true
	for i, entry := range entries {
		text, ok := s.toTextValue(field, entry, fmt.Sprintf("%s[%d]", path, i))
		parts[i], storable = text.Text, storable && ok
	}
	return Value{Type: objmod.String, Text: strings.Join(parts, ",")}, storable
}

func (s *objectResolver) toIntValue(field *FieldMeta, value any, path string) (Value, bool) {
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
		s.report(path, errWrongType(expected, value, field))
	case whole < math.MinInt32 || whole > math.MaxInt32:
		s.report(path, errOutOfRange(whole, "an integer", field))
	default:
		return Value{Type: objmod.Int, Number: float64(int32(whole))}, true
	}
	return Value{}, false
}

func (s *objectResolver) toRealValue(field *FieldMeta, value any, path string) (Value, bool) {
	amount, isNumber := value.(float64)
	switch {
	case !isNumber:
		s.report(path, errWrongType("a number", value, field))
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

func (s *objectResolver) toTextValue(field *FieldMeta, value any, path string) (Value, bool) {
	text, isText := value.(string)
	switch {
	case !isText:
		s.report(path, errWrongType("a string", value, field))
	case strings.Contains(text, "\x00"):
		s.report(path, errHasNUL())
	case !utf8.ValidString(text):
		s.report(path, errNotUTF8())
	default:
		return Value{Type: objmod.String, Text: text}, true
	}
	return Value{}, false
}

func formatNumber(v float64) string {
	if v == 0 {
		return "0"
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func formatValue(value any) string {
	switch v := value.(type) {
	case nil:
		return "null"
	case bool:
		return strconv.FormatBool(v)
	case float64:
		return formatNumber(v)
	case string:
		return fsx.QuoteJSON(v)
	case []any:
		entries := make([]string, len(v))
		for i, entry := range v {
			entries[i] = formatValue(entry)
		}
		return "[" + strings.Join(entries, ",") + "]"
	case map[string]any:
		entries := make([]string, 0, len(v))
		for _, key := range slices.Sorted(maps.Keys(v)) {
			entries = append(entries, fsx.QuoteJSON(key)+":"+formatValue(v[key]))
		}
		return "{" + strings.Join(entries, ",") + "}"
	}
	return fmt.Sprint(value)
}

func storageName(field *FieldMeta) string {
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

func errWrongType(expected string, value any, field *FieldMeta) issue {
	return issue{"expected " + expected + ", got " + formatValue(value) + ".", storageName(field)}
}

func errOutOfRange(value float64, kind string, field *FieldMeta) issue {
	return issue{formatNumber(value) + " is out of range for " + kind + ".", storageName(field)}
}

func errHasNUL() issue {
	return issue{"the string contains a NUL character.", "Remove it: the game ends strings at NUL."}
}

func errNotUTF8() issue {
	return issue{"the string is not valid UTF-8.", "The files of a map store text as UTF-8."}
}
