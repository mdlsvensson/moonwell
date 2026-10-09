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

func (r *objectResolver) toValue(field *FieldMeta, value any, path string) (Value, bool) {
	switch {
	case field.List:
		return r.toListValue(field, value, path)
	case field.Storage == "int":
		return r.toIntValue(field, value, path)
	case field.Storage == "real" || field.Storage == "unreal":
		return r.toRealValue(field, value, path)
	}
	return r.toTextValue(field, value, path)
}

func (r *objectResolver) toListValue(field *FieldMeta, value any, path string) (Value, bool) {
	if text, isText := value.(string); isText {
		return r.toTextValue(field, text, path)
	}
	entries, isList := value.([]any)
	if !isList {
		r.report(path, errWrongType("a string or a List<String>", value, field))
		return Value{}, false
	}
	parts, storable := make([]string, len(entries)), true
	for i, entry := range entries {
		text, ok := r.toTextValue(field, entry, fmt.Sprintf("%s[%d]", path, i))
		parts[i], storable = text.Text, storable && ok
	}
	return Value{Type: objmod.String, Text: strings.Join(parts, ",")}, storable
}

func (r *objectResolver) toIntValue(field *FieldMeta, value any, path string) (Value, bool) {
	if flag, isBool := value.(bool); isBool {
		if flag {
			return Value{Type: objmod.Int, Number: 1}, true
		}
		return Value{Type: objmod.Int}, true
	}
	number, isNumber := value.(float64)
	switch {
	case !isNumber || number != math.Trunc(number) || math.IsInf(number, 0):
		expected := "an integer"
		if field.Type == "bool" {
			expected = "a Boolean"
		}
		r.report(path, errWrongType(expected, value, field))
	case number < math.MinInt32 || number > math.MaxInt32:
		r.report(path, errOutOfRange(number, "an integer", field))
	default:
		return Value{Type: objmod.Int, Number: float64(int32(number))}, true
	}
	return Value{}, false
}

func (r *objectResolver) toRealValue(field *FieldMeta, value any, path string) (Value, bool) {
	number, isNumber := value.(float64)
	switch {
	case !isNumber:
		r.report(path, errWrongType("a number", value, field))
	case math.IsNaN(number) || math.Abs(number) > math.MaxFloat32:
		r.report(path, errOutOfRange(number, "a real number", field))
	default:
		valueType := objmod.Unreal
		if field.Storage == "real" {
			valueType = objmod.Real
		}
		return Value{Type: valueType, Number: number}, true
	}
	return Value{}, false
}

func (r *objectResolver) toTextValue(field *FieldMeta, value any, path string) (Value, bool) {
	text, isText := value.(string)
	switch {
	case !isText:
		r.report(path, errWrongType("a string", value, field))
	case strings.Contains(text, "\x00"):
		r.report(path, errHasNUL())
	case !utf8.ValidString(text):
		r.report(path, errNotUTF8())
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

func describeStorage(field *FieldMeta) string {
	storage := map[string]string{"int": "an integer", "real": "a real number", "unreal": "a real number"}[field.Storage]
	switch {
	case field.List:
		storage = "a comma-separated list"
	case field.Storage == "int" && field.Type == "bool":
		storage = "a Boolean (1 or 0)"
	case storage == "":
		storage = "a string"
	}
	return describeField(field) + " is stored as " + storage + "."
}

func errWrongType(expected string, value any, field *FieldMeta) issue {
	return issue{"expected " + expected + ", got " + formatValue(value) + ".", describeStorage(field)}
}

func errOutOfRange(value float64, kind string, field *FieldMeta) issue {
	return issue{formatNumber(value) + " is out of range for " + kind + ".", describeStorage(field)}
}

func errHasNUL() issue {
	return issue{"the string contains a NUL character.", "Remove it: the game ends strings at NUL."}
}

func errNotUTF8() issue {
	return issue{"the string is not valid UTF-8.", "The files of a map store text as UTF-8."}
}
