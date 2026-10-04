package oracle

import (
	"cmp"
	"fmt"
	"reflect"
	"slices"
	"strconv"
)

// stringDiff is the first pair of strings that differ in bytes, with the path that leads to them.
type stringDiff struct {
	path      string
	want, got string // quoted, so that every byte shows
}

// firstStringDifference walks want and got together and returns the first pair of strings whose bytes differ.
// encoding/json turns every invalid UTF-8 byte into U+FFFD, so values whose JSON is equal can still differ here.
// The two values may have different types: struct fields are matched by name, and only exported ones are looked at.
func firstStringDifference(path string, want, got reflect.Value) (stringDiff, bool) {
	want, got = unwrap(want), unwrap(got)
	if !want.IsValid() || !got.IsValid() {
		return stringDiff{}, false
	}
	switch want.Kind() {
	case reflect.String:
		if got.Kind() == reflect.String && want.String() != got.String() {
			return stringDiff{path, strconv.Quote(want.String()), strconv.Quote(got.String())}, true
		}
	case reflect.Struct:
		return structStringDifference(path, want, got)
	case reflect.Slice, reflect.Array:
		return elementStringDifference(path, want, got)
	case reflect.Map:
		return mapStringDifference(path, want, got)
	}
	return stringDiff{}, false
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

func structStringDifference(path string, want, got reflect.Value) (stringDiff, bool) {
	if got.Kind() != reflect.Struct {
		return stringDiff{}, false
	}
	for _, field := range reflect.VisibleFields(want.Type()) {
		other, ok := got.Type().FieldByName(field.Name)
		if !field.IsExported() || field.Tag.Get("json") == "-" || !ok || !other.IsExported() {
			continue
		}
		wantField, gotField := want.FieldByIndex(field.Index), got.FieldByIndex(other.Index)
		if diff, found := firstStringDifference(fieldPath(path, field.Name), wantField, gotField); found {
			return diff, true
		}
	}
	return stringDiff{}, false
}

func elementStringDifference(path string, want, got reflect.Value) (stringDiff, bool) {
	if got.Kind() != reflect.Slice && got.Kind() != reflect.Array {
		return stringDiff{}, false
	}
	for i := range min(want.Len(), got.Len()) {
		if diff, found := firstStringDifference(path+"["+strconv.Itoa(i)+"]", want.Index(i), got.Index(i)); found {
			return diff, true
		}
	}
	return stringDiff{}, false
}

// mapStringDifference compares the values under equal keys, and the keys themselves byte for byte: a key of want
// that got does not hold is a difference.
func mapStringDifference(path string, want, got reflect.Value) (stringDiff, bool) {
	if got.Kind() != reflect.Map || !want.Type().Key().ConvertibleTo(got.Type().Key()) {
		return stringDiff{}, false
	}
	keys := want.MapKeys()
	slices.SortFunc(keys, func(a, b reflect.Value) int { return cmp.Compare(fmt.Sprint(a), fmt.Sprint(b)) })
	for _, key := range keys {
		keyPath := path + "[" + quoteKey(key) + "]"
		other := got.MapIndex(key.Convert(got.Type().Key()))
		if !other.IsValid() {
			return stringDiff{keyPath, quoteKey(key), "(no such key)"}, true
		}
		if diff, found := firstStringDifference(keyPath, want.MapIndex(key), other); found {
			return diff, true
		}
	}
	return stringDiff{}, false
}

func quoteKey(key reflect.Value) string {
	if key.Kind() == reflect.String {
		return strconv.Quote(key.String())
	}
	return fmt.Sprint(key)
}

// fieldPath appends a field name to a path; the root has the empty path.
func fieldPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}
