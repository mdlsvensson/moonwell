package manifest

import "encoding/json"

// Category is a kind of custom object, by its name in the manifest.
type Category string

// Categories are the categories in the order Moonwell works through them.
var Categories = []Category{"heroes", "units", "buildings", "items", "abilities", "buffs", "upgrades"}

// Objects is the manifest's custom objects, by category and key: Objects.pkl and ObjectFile.pkl.
type Objects struct {
	Heroes    Ordered[Object] `json:"heroes"`
	Units     Ordered[Object] `json:"units"`
	Buildings Ordered[Object] `json:"buildings"`
	Items     Ordered[Object] `json:"items"`
	Abilities Ordered[Object] `json:"abilities"`
	Buffs     Ordered[Object] `json:"buffs"`
	Upgrades  Ordered[Object] `json:"upgrades"`
}

// of is where the objects of a category are held, or nil for a name that is no category.
func (o *Objects) of(category Category) *Ordered[Object] {
	switch category {
	case "heroes":
		return &o.Heroes
	case "units":
		return &o.Units
	case "buildings":
		return &o.Buildings
	case "items":
		return &o.Items
	case "abilities":
		return &o.Abilities
	case "buffs":
		return &o.Buffs
	case "upgrades":
		return &o.Upgrades
	}
	return nil
}

// Of returns the objects of a category by key; a name that is no category has none.
func (o Objects) Of(category Category) Ordered[Object] {
	if held := o.of(category); held != nil {
		return *held
	}
	return Ordered[Object]{}
}

// Empty reports whether there is no object in any category.
func (o Objects) Empty() bool {
	for _, category := range Categories {
		if o.Of(category).Len() > 0 {
			return false
		}
	}
	return true
}

// nameSources gives every object that names no file of its own the file as its source: an object written in the
// manifest itself is from the manifest that was evaluated.
func (o *Objects) nameSources(file string) {
	for _, category := range Categories {
		objects := o.of(category)
		for key, object := range objects.All() {
			if object.Source == "" {
				object.Source = file
				objects.Set(key, object)
			}
		}
	}
}

// Object is one custom object as the manifest gives it. A property's value is a bool, a float64, a string, or a
// list of those and of lists of strings.
type Object struct {
	ID, Base   string
	Source     string       // the object's file, or the evaluated manifest for an object written inline
	Typed      Ordered[any] // the typed properties by friendly name; a null is left out
	Properties Ordered[any] // the `properties` block, by friendly name or field rawcode; a null is left out
}

// UnmarshalJSON reads an object as pkl prints it: id, base, source and properties are its own, and every other
// key is a typed property, in the order read.
func (o *Object) UnmarshalJSON(data []byte) error {
	var keys Ordered[json.RawMessage]
	if err := keys.UnmarshalJSON(data); err != nil {
		return err
	}
	*o = Object{}
	for key, value := range keys.All() {
		if err := o.readKey(key, value); err != nil {
			return under(key, err)
		}
	}
	return nil
}

// readKey reads the value of one key of an object.
func (o *Object) readKey(key string, value json.RawMessage) error {
	switch key {
	case "id":
		return json.Unmarshal(value, &o.ID)
	case "base":
		return json.Unmarshal(value, &o.Base)
	case "source":
		return json.Unmarshal(value, &o.Source)
	case "properties":
		var properties Ordered[any]
		err := json.Unmarshal(value, &properties)
		for name, property := range properties.All() {
			setUnlessNull(&o.Properties, name, property)
		}
		return err
	}
	var property any
	err := json.Unmarshal(value, &property)
	setUnlessNull(&o.Typed, key, property)
	return err
}

// setUnlessNull sets a property that has a value. A null means that the object keeps what its base has, as a
// property that is left out does.
func setUnlessNull(properties *Ordered[any], name string, value any) {
	if value != nil {
		properties.Set(name, value)
	}
}
