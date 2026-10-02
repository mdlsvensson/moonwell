package ordered

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/text"
)

// Object is a JSON object of a tree that Decode returns.
type Object = Map[any]

// Decode parses JSON into a tree, as JSON.parse does: *Object for an object, []any for an array, and string,
// float64, bool or nil for the rest. Objects keep their key order.
func Decode(data []byte) (any, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	value, err := decodeValue(decoder)
	if err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errors.New("unexpected data after the JSON value")
	}
	return value, nil
}

func decodeValue(decoder *json.Decoder) (any, error) {
	token, err := decoder.Token()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return nil, io.ErrUnexpectedEOF
		}
		return nil, err
	}
	switch token {
	case json.Delim('{'):
		object := &Object{}
		for decoder.More() {
			key, err := decoder.Token()
			if err != nil {
				return nil, err
			}
			name, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("expected an object key, found %v", key)
			}
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			object.Set(name, value)
		}
		_, err := decoder.Token()
		return object, err
	case json.Delim('['):
		array := []any{}
		for decoder.More() {
			value, err := decodeValue(decoder)
			if err != nil {
				return nil, err
			}
			array = append(array, value)
		}
		_, err := decoder.Token()
		return array, err
	}
	return token, nil
}

// keyed is any *Map[V]: Stringify writes it as an object.
type keyed interface {
	Keys() []string
	value(key string) any
}

func (m *Map[V]) value(key string) any { return m.values[key] }

// Stringify is JSON.stringify(value, null, indent) for a tree: nil, bool, a number, a string, a slice, or a
// *Map of such values. Indent 0 gives the compact form. A number that is not finite is written as null.
func Stringify(value any, indent int) string {
	var out strings.Builder
	stringify(&out, value, indent, 0)
	return out.String()
}

func stringify(out *strings.Builder, value any, indent, depth int) {
	newline := func(depth int) {
		if indent > 0 {
			out.WriteByte('\n')
			out.WriteString(strings.Repeat(" ", indent*depth))
		}
	}
	list := func(open, close byte, count int, item func(i int)) {
		out.WriteByte(open)
		for i := range count {
			if i > 0 {
				out.WriteByte(',')
			}
			newline(depth + 1)
			item(i)
		}
		if count > 0 {
			newline(depth)
		}
		out.WriteByte(close)
	}
	number := func(v float64) {
		if math.IsInf(v, 0) || math.IsNaN(v) {
			out.WriteString("null")
		} else {
			out.WriteString(text.Number(v))
		}
	}
	switch v := value.(type) {
	case nil:
		out.WriteString("null")
	case bool:
		if v {
			out.WriteString("true")
		} else {
			out.WriteString("false")
		}
	case string:
		out.WriteString(text.Quote(v))
	case float64:
		number(v)
	case float32:
		number(float64(v))
	case int:
		number(float64(v))
	case int32:
		number(float64(v))
	case int64:
		number(float64(v))
	case uint32:
		number(float64(v))
	case []any:
		list('[', ']', len(v), func(i int) { stringify(out, v[i], indent, depth+1) })
	case []string:
		list('[', ']', len(v), func(i int) { out.WriteString(text.Quote(v[i])) })
	case keyed:
		keys := v.Keys()
		list('{', '}', len(keys), func(i int) {
			out.WriteString(text.Quote(keys[i]))
			out.WriteByte(':')
			if indent > 0 {
				out.WriteByte(' ')
			}
			stringify(out, v.value(keys[i]), indent, depth+1)
		})
	default:
		panic(fmt.Sprintf("ordered.Stringify: unsupported value of type %T", value))
	}
}
