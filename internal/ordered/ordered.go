// Package ordered has a JSON object that keeps its key order.
//
// Moonwell's manifest is evaluated to JSON, and the order of its mappings reaches the files Moonwell writes. That
// order was first defined by JavaScript, which lists an object's keys in a particular way: keys that are array
// indexes ("0", "7", "10") come first, in numeric order, and all other keys follow in the order they were added.
package ordered

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"
	"slices"
	"strconv"
)

// Map is a JSON object whose keys keep JavaScript's order. The zero value is an empty map.
type Map[V any] struct {
	keys   []string // in insertion order
	values map[string]V
}

// Set adds or replaces a key. A replaced key keeps its place.
func (m *Map[V]) Set(key string, value V) {
	if m.values == nil {
		m.values = map[string]V{}
	}
	if _, ok := m.values[key]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Get returns the value of key.
func (m *Map[V]) Get(key string) (V, bool) {
	value, ok := m.values[key]
	return value, ok
}

// Has reports whether key is present.
func (m *Map[V]) Has(key string) bool {
	_, ok := m.values[key]
	return ok
}

// Delete removes key.
func (m *Map[V]) Delete(key string) {
	if _, ok := m.values[key]; !ok {
		return
	}
	delete(m.values, key)
	m.keys = slices.DeleteFunc(m.keys, func(k string) bool { return k == key })
}

// Len is the number of keys.
func (m *Map[V]) Len() int { return len(m.keys) }

// arrayIndex reports whether key is a canonical array index, and its value: decimal digits without a leading zero,
// below 2^32-1.
func arrayIndex(key string) (uint64, bool) {
	if key == "" || len(key) > 10 || (len(key) > 1 && key[0] == '0') {
		return 0, false
	}
	for i := 0; i < len(key); i++ {
		if key[i] < '0' || key[i] > '9' {
			return 0, false
		}
	}
	n, err := strconv.ParseUint(key, 10, 64)
	return n, err == nil && n < 1<<32-1
}

// Keys returns the keys in JavaScript's order: array indexes first, in numeric order, then the rest as added.
func (m *Map[V]) Keys() []string {
	type index struct {
		key string
		n   uint64
	}
	var indexes []index
	rest := make([]string, 0, len(m.keys))
	for _, key := range m.keys {
		if n, ok := arrayIndex(key); ok {
			indexes = append(indexes, index{key, n})
		} else {
			rest = append(rest, key)
		}
	}
	if len(indexes) == 0 {
		return rest
	}
	slices.SortFunc(indexes, func(a, b index) int {
		if a.n < b.n {
			return -1
		}
		return 1
	})
	keys := make([]string, 0, len(m.keys))
	for _, i := range indexes {
		keys = append(keys, i.key)
	}
	return append(keys, rest...)
}

// All ranges over the keys and values in JavaScript's order.
func (m *Map[V]) All() iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		for _, key := range m.Keys() {
			if !yield(key, m.values[key]) {
				return
			}
		}
	}
}

// UnmarshalJSON reads a JSON object, or null as an empty map. A repeated key keeps its first place and its last
// value, as in JavaScript.
func (m *Map[V]) UnmarshalJSON(data []byte) error {
	m.keys, m.values = nil, nil
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	if token == nil {
		return nil
	}
	if token != json.Delim('{') {
		return &json.UnmarshalTypeError{Value: kindOf(token), Type: nil}
	}
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		key, ok := token.(string)
		if !ok {
			return fmt.Errorf("ordered: expected an object key, found %v", token)
		}
		var value V
		if err := decoder.Decode(&value); err != nil {
			return err
		}
		m.Set(key, value)
	}
	_, err = decoder.Token() // the closing brace
	return err
}

func kindOf(token json.Token) string {
	switch token.(type) {
	case json.Delim:
		return "array"
	case string:
		return "string"
	case bool:
		return "bool"
	default:
		return "number"
	}
}

// MarshalJSON writes the object with its keys in JavaScript's order. Like JSON.stringify it does not escape <, >
// and &; an encoder that escapes HTML escapes them again.
func (m Map[V]) MarshalJSON() ([]byte, error) {
	var out bytes.Buffer
	encoder := json.NewEncoder(&out)
	encoder.SetEscapeHTML(false)
	out.WriteByte('{')
	for i, key := range m.Keys() {
		if i > 0 {
			out.WriteByte(',')
		}
		if err := encoder.Encode(key); err != nil {
			return nil, err
		}
		out.Truncate(out.Len() - 1) // Encode ends with a newline
		out.WriteByte(':')
		if err := encoder.Encode(m.values[key]); err != nil {
			return nil, err
		}
		out.Truncate(out.Len() - 1)
	}
	out.WriteByte('}')
	return out.Bytes(), nil
}
