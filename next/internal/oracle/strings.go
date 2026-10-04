package oracle

import (
	"cmp"
	"encoding"
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strconv"
)

// stringDiff is where a walk of two values stopped: either two strings that differ in bytes (want and got, quoted so
// that every byte shows), or two sides it could not pair (cannot says why).
type stringDiff struct {
	path      string
	want, got string
	cannot    string
}

var (
	jsonMarshaler = reflect.TypeFor[json.Marshaler]()
	textMarshaler = reflect.TypeFor[encoding.TextMarshaler]()
)

// firstStringDifference walks want and got together and returns the first pair of strings whose bytes differ.
// encoding/json writes every invalid UTF-8 byte as U+FFFD, so values whose JSON is equal can still differ here. It
// never passes what it cannot compare: where the two sides cannot be paired it returns a diff with cannot set.
func firstStringDifference(path string, want, got reflect.Value) (stringDiff, bool) {
	want, got = unwrap(want), unwrap(got)
	if !want.IsValid() || !got.IsValid() {
		return nilSideDifference(path, want, got)
	}
	if hasMarshaler(want.Type()) || hasMarshaler(got.Type()) {
		return marshalerDifference(path, want, got)
	}
	switch {
	case want.Kind() == reflect.String && got.Kind() == reflect.String:
		if want.String() != got.String() {
			return stringDiff{path: path, want: strconv.Quote(want.String()), got: strconv.Quote(got.String())}, true
		}
		return stringDiff{}, false
	case want.Kind() == reflect.Struct && got.Kind() == reflect.Struct:
		return structStringDifference(path, want, got)
	case isList(want) && isList(got):
		return elementStringDifference(path, want, got)
	case want.Kind() == reflect.Map && got.Kind() == reflect.Map:
		return mapStringDifference(path, want, got)
	case isScalar(want) && isScalar(got):
		return stringDiff{}, false
	}
	return unpairable(path, fmt.Sprintf("a %s against a %s", want.Type(), got.Type()))
}

func unpairable(path, reason string) (stringDiff, bool) {
	return stringDiff{path: path, cannot: reason}, true
}

// unwrap follows pointers and interfaces to the value they hold; it returns the zero Value for a nil one.
func unwrap(value reflect.Value) reflect.Value {
	for value.IsValid() && (value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface) {
		if value.IsNil() {
			return reflect.Value{}
		}
		value = value.Elem()
	}
	return value
}

// hasMarshaler reports whether values of the type write their own JSON, by value or through a pointer.
func hasMarshaler(t reflect.Type) bool {
	for _, marshaler := range []reflect.Type{jsonMarshaler, textMarshaler} {
		if t.Implements(marshaler) || reflect.PointerTo(t).Implements(marshaler) {
			return true
		}
	}
	return false
}

// nilSideDifference handles a side that holds nothing (JSON null): it holds no string, unless the other side writes
// its own JSON, which cannot be looked into.
func nilSideDifference(path string, want, got reflect.Value) (stringDiff, bool) {
	for _, side := range []reflect.Value{want, got} {
		if side.IsValid() && hasMarshaler(side.Type()) {
			return unpairable(path, fmt.Sprintf("type %s writes its own JSON", side.Type()))
		}
	}
	return stringDiff{}, false
}

// marshalerDifference handles a side that writes its own JSON: what it holds cannot be looked into, so it is
// unpairable unless both sides have one type and hold equal values.
func marshalerDifference(path string, want, got reflect.Value) (stringDiff, bool) {
	if want.Type() == got.Type() && want.CanInterface() && got.CanInterface() &&
		reflect.DeepEqual(want.Interface(), got.Interface()) {
		return stringDiff{}, false
	}
	return unpairable(path, fmt.Sprintf("the types %s and %s write their own JSON", want.Type(), got.Type()))
}

func isList(value reflect.Value) bool {
	return value.Kind() == reflect.Slice || value.Kind() == reflect.Array
}

// isScalar reports whether the value is a bool or a number: it holds no string, so equal JSON is all there is.
func isScalar(value reflect.Value) bool {
	return value.Kind() >= reflect.Bool && value.Kind() <= reflect.Float64
}

func elementStringDifference(path string, want, got reflect.Value) (stringDiff, bool) {
	for i := range min(want.Len(), got.Len()) {
		if diff, found := firstStringDifference(path+"["+strconv.Itoa(i)+"]", want.Index(i), got.Index(i)); found {
			return diff, true
		}
	}
	return stringDiff{}, false
}

// mapStringDifference compares the values under equal keys, and the keys themselves byte for byte: a key of want
// that got does not hold is a difference. Keys must be strings on both sides or integers on both sides.
func mapStringDifference(path string, want, got reflect.Value) (stringDiff, bool) {
	wantKey, gotKey := want.Type().Key(), got.Type().Key()
	if !comparableKeys(wantKey, gotKey) {
		return unpairable(path, fmt.Sprintf("the map keys of types %s and %s", wantKey, gotKey))
	}
	keys := want.MapKeys()
	slices.SortFunc(keys, func(a, b reflect.Value) int { return cmp.Compare(fmt.Sprint(a), fmt.Sprint(b)) })
	for _, key := range keys {
		keyPath := path + "[" + quoteKey(key) + "]"
		other := got.MapIndex(key.Convert(gotKey))
		if !other.IsValid() {
			return stringDiff{path: keyPath, want: quoteKey(key), got: "(no such key)"}, true
		}
		if diff, found := firstStringDifference(keyPath, want.MapIndex(key), other); found {
			return diff, true
		}
	}
	return stringDiff{}, false
}

// comparableKeys reports whether keys of the two types can be looked up in each other's map as they are: both
// strings, or both integers, and neither writing its own text.
func comparableKeys(a, b reflect.Type) bool {
	if hasMarshaler(a) || hasMarshaler(b) {
		return false
	}
	return a.Kind() == reflect.String && b.Kind() == reflect.String || isIntegerKind(a.Kind()) && isIntegerKind(b.Kind())
}

func isIntegerKind(kind reflect.Kind) bool { return kind >= reflect.Int && kind <= reflect.Uintptr }

func quoteKey(key reflect.Value) string {
	if key.Kind() == reflect.String {
		return strconv.Quote(key.String())
	}
	return fmt.Sprint(key)
}
