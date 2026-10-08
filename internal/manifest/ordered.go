package manifest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"iter"
	"slices"
)

type OrderedMap[V any] struct {
	keys   []string
	values map[string]V
}

func (o OrderedMap[V]) Len() int { return len(o.keys) }

func (o OrderedMap[V]) Keys() []string { return slices.Clone(o.keys) }

func (o OrderedMap[V]) Get(key string) (V, bool) {
	value, ok := o.values[key]
	return value, ok
}

func (o OrderedMap[V]) All() iter.Seq2[string, V] {
	return func(yield func(string, V) bool) {
		for _, key := range o.keys {
			if !yield(key, o.values[key]) {
				return
			}
		}
	}
}

func (o *OrderedMap[V]) Set(key string, value V) {
	if o.values == nil {
		o.values = map[string]V{}
	}
	if _, held := o.values[key]; !held {
		o.keys = append(o.keys, key)
	}
	o.values[key] = value
}

func (o *OrderedMap[V]) UnmarshalJSON(data []byte) error {
	*o = OrderedMap[V]{}
	decoder := json.NewDecoder(bytes.NewReader(data))
	opening, err := decoder.Token()
	if err != nil || opening == nil {
		return err
	}
	if opening != json.Delim('{') {
		return fmt.Errorf("the value is of the wrong kind (%s, not a mapping)", tokenKind(opening))
	}
	for decoder.More() {
		if err := o.decodeEntry(decoder); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

func (o *OrderedMap[V]) decodeEntry(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	key, _ := token.(string)
	var value V
	if err := decoder.Decode(&value); err != nil {
		return wrapWithKey(key, err)
	}
	o.Set(key, value)
	return nil
}

func wrapWithKey(key string, err error) error {
	return fmt.Errorf("%s: %w", key, err)
}

func tokenKind(token json.Token) string {
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
