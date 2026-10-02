package objects

import (
	"slices"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// SchemaHint is the hint for evaluated JSON that does not match what this CLI reads: a schema and CLI version
// mismatch.
const SchemaHint = "Is the moonwell Pkl package the version this CLI expects?"

// ManifestObject is one custom object as the manifest gives it. A property value is a scalar (bool, float64 or
// string), or a list of scalars and lists of strings.
type ManifestObject struct {
	ID   string
	Base string
	// Source is the object's file relative to the manifest, or the evaluated manifest for objects that
	// Objects.merge did not set.
	Source string
	// Typed holds the typed properties by friendly name, in evaluation order; null (inherit) is left out.
	Typed ordered.Map[any]
	// Properties is the `properties` escape hatch, keyed by friendly name or field rawcode; null is left out.
	Properties ordered.Map[any]
}

// Manifest is the manifest's custom objects: for every category, its objects by key.
type Manifest map[Category]*ordered.Map[ManifestObject]

var reserved = []string{"id", "base", "source", "properties"}

// EmptyManifest returns a manifest without objects.
func EmptyManifest() Manifest {
	manifest := Manifest{}
	for _, category := range Categories {
		manifest[category] = &ordered.Map[ManifestObject]{}
	}
	return manifest
}

// Empty reports whether the manifest has no objects.
func (m Manifest) Empty() bool {
	for _, category := range Categories {
		if m[category].Len() > 0 {
			return false
		}
	}
	return true
}

func isScalar(input any) bool {
	switch input.(type) {
	case bool, float64, string:
		return true
	}
	return false
}

func isPropertyValue(input any) bool {
	if isScalar(input) {
		return true
	}
	items, ok := input.([]any)
	if !ok {
		return false
	}
	for _, item := range items {
		if isScalar(item) {
			continue
		}
		entries, ok := item.([]any)
		if !ok {
			return false
		}
		for _, entry := range entries {
			if _, isString := entry.(string); !isString {
				return false
			}
		}
	}
	return true
}

// ParseManifest checks the shape of the evaluated `objects` JSON tree. Pkl has already checked the types; a
// mismatch means a schema and CLI version problem. Pkl omits null, and objects written inline or added in
// moonwell.local.pkl have no source, so file, the evaluated manifest, stands in. When the manifest has no objects
// key at all (present is false), the result is an empty manifest, as 0.2 manifests have no objects.
func ParseManifest(value any, present bool, file string) (Manifest, error) {
	result := EmptyManifest()
	if !present {
		return result, nil
	}
	var failure error
	fail := func(path, expected string) {
		if failure == nil {
			failure = &diag.Error{Msg: path + " must be " + expected + ".", File: file, Hint: SchemaHint}
		}
	}
	empty := &ordered.Object{}
	record := func(input any, path string) *ordered.Object {
		if object, ok := input.(*ordered.Object); ok {
			return object
		}
		fail(path, "an object")
		return empty
	}
	stringAt := func(object *ordered.Object, key, path string) string {
		value, _ := object.Get(key)
		if s, ok := value.(string); ok {
			return s
		}
		fail(path, "a string")
		return ""
	}
	values := func(target *ordered.Map[any], key string, entry any, path string) {
		if entry == nil {
			return
		}
		if !isPropertyValue(entry) {
			fail(path, "a Boolean, number, string or List")
		}
		target.Set(key, entry)
	}

	categories := record(value, "objects")
	for _, category := range categories.Keys() {
		if !slices.Contains(Categories, Category(category)) && failure == nil {
			failure = &diag.Error{Msg: "objects." + category + " is not an object category.", File: file, Hint: SchemaHint}
		}
	}
	for _, category := range Categories {
		entries, ok := categories.Get(string(category))
		if !ok {
			continue
		}
		for key, entry := range record(entries, "objects."+string(category)).All() {
			path := "objects." + string(category) + "[" + text.Quote(key) + "]"
			object := record(entry, path)
			parsed := ManifestObject{
				ID:     stringAt(object, "id", path+".id"),
				Base:   stringAt(object, "base", path+".base"),
				Source: file,
			}
			if source, _ := object.Get("source"); source != nil {
				parsed.Source = stringAt(object, "source", path+".source")
			}
			for name, typed := range object.All() {
				if !slices.Contains(reserved, name) {
					values(&parsed.Typed, name, typed, path+"."+name)
				}
			}
			properties, _ := object.Get("properties")
			if properties == nil {
				properties = empty
			}
			for name, property := range record(properties, path+".properties").All() {
				values(&parsed.Properties, name, property, path+".properties["+text.Quote(name)+"]")
			}
			result[category].Set(key, parsed)
		}
	}
	if failure != nil {
		return nil, failure
	}
	return result, nil
}
