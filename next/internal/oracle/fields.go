package oracle

import (
	"reflect"
	"slices"
	"strings"
)

// jsonField is a struct field as encoding/json sees it: under its JSON name.
type jsonField struct {
	name  string
	value reflect.Value
}

// structStringDifference pairs the fields of two structs by JSON name, which is what made their JSON equal. A field
// only one side has, other than one it leaves out of its JSON, cannot be paired.
func structStringDifference(path string, want, got reflect.Value) (stringDiff, bool) {
	wantFields, gotFields := jsonFields(want), jsonFields(got)
	for _, field := range wantFields {
		at := fieldPath(path, field.name)
		i := slices.IndexFunc(gotFields, func(other jsonField) bool { return other.name == field.name })
		if i < 0 {
			return unpairable(at, "the field is only in the wanted value")
		}
		if diff, found := firstStringDifference(at, field.value, gotFields[i].value); found {
			return diff, true
		}
	}
	for _, field := range gotFields {
		if !slices.ContainsFunc(wantFields, func(other jsonField) bool { return other.name == field.name }) {
			return unpairable(fieldPath(path, field.name), "the field is only in the actual value")
		}
	}
	return stringDiff{}, false
}

// jsonFields lists the fields of a struct that reach its JSON, promoted ones included. A field behind a nil
// embedded pointer holds nothing and is left out.
func jsonFields(value reflect.Value) []jsonField {
	var fields []jsonField
	for _, field := range reflect.VisibleFields(value.Type()) {
		name, ok := jsonName(field)
		if !ok {
			continue
		}
		if held, err := value.FieldByIndexErr(field.Index); err == nil {
			fields = append(fields, jsonField{name, held})
		}
	}
	return fields
}

// jsonName is the name a field has in JSON: the tag's, else the Go name. It is false for a field that is not
// exported, is tagged "-", or is an embedded struct whose fields are listed as promoted ones.
func jsonName(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("json")
	if tag == "-" || !field.IsExported() {
		return "", false
	}
	name, _, _ := strings.Cut(tag, ",")
	if name == "" && field.Anonymous && isStructOrPointerToOne(field.Type) {
		return "", false
	}
	if name == "" {
		name = field.Name
	}
	return name, true
}

func isStructOrPointerToOne(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Kind() == reflect.Struct
}

// fieldPath appends a field name to a path; the root has the empty path.
func fieldPath(path, name string) string {
	if path == "" {
		return name
	}
	return path + "." + name
}
