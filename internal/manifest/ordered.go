package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"
	"slices"
)

// Ordered is a Pkl Mapping in the order it was written. The zero value is an empty mapping.
//
// A copy of an Ordered shares its storage with the original, as a copy of a map does: what Set does through one of
// them may show in the other, or in a part of it. So only the owner of an Ordered calls Set, through the variable or
// the field that holds it; a copy, such as what Objects.Of returns or a value passed to a function, is for reading.
type Ordered[V any] struct {
	keys   []string // in the order written
	values map[string]V
}

// Len is the number of keys.
func (o Ordered[V]) Len() int { return len(o.keys) }

// Keys returns the keys in the order written.
func (o Ordered[V]) Keys() []string { return slices.Clone(o.keys) }

// Get returns the value of key, and whether the key is there.
func (o Ordered[V]) Get(key string) (V, bool) {
	value, ok := o.values[key]
	return value, ok
}

// All ranges over the keys and their values in the order written.
func (o Ordered[V]) All() iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		for _, key := range o.keys {
			if !yield(key, o.values[key]) {
				return
			}
		}
	}
}

// Set adds a key at the end, or replaces the value of a key already there, which keeps its place.
func (o *Ordered[V]) Set(key string, value V) {
	if o.values == nil {
		o.values = map[string]V{}
	}
	if _, held := o.values[key]; !held {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

// UnmarshalJSON reads a JSON object in the order of its text, or null as an empty mapping. A key that comes twice
// keeps its first place and its last value.
func (o *Ordered[V]) UnmarshalJSON(data []byte) error {
	*o = Ordered[V]{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil || opening == nil {
		return err
	}
	if opening != json.Delim('{') {
		return fmt.Errorf("the value is of the wrong kind (%s, not a mapping)", kindOf(opening))
	}
	for decoder.More() {
		if err := o.readEntry(decoder); err != nil {
			return err
		}
	}
	_, err = decoder.Token() // the closing brace
	return err
}

// readEntry reads the next key and its value.
func (o *Ordered[V]) readEntry(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	key, _ := token.(string) // the decoder gives nothing else where an object has a key
	var value V
	if err := decoder.Decode(&value); err != nil {
		return under(key, err)
	}
	o.Set(key, value)
	return nil
}

// under puts the key a value was read for before the failure of reading it. The decoder names the fields of
// structs in its reasons and passes on the failure of a type that reads itself as it is, so the keys of a mapping
// are named here.
func under(key string, err error) error {
	return fmt.Errorf("%s: %w", key, err)
}

// kindOf names the JSON value a token starts, as the decoder names it in its reasons.
func kindOf(token json.Token) string {
	switch token.(type) {
	case json.Delim:
		return "array"
	case string:
		return "string"
	case bool:
		return "bool"
	}
	return "number"
}
