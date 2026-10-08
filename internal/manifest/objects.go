package manifest

import "encoding/json"

type Category string

var Categories = []Category{"heroes", "units", "buildings", "items", "abilities", "buffs", "upgrades"}

type Objects struct {
	Heroes    Ordered[Object] `json:"heroes"`
	Units     Ordered[Object] `json:"units"`
	Buildings Ordered[Object] `json:"buildings"`
	Items     Ordered[Object] `json:"items"`
	Abilities Ordered[Object] `json:"abilities"`
	Buffs     Ordered[Object] `json:"buffs"`
	Upgrades  Ordered[Object] `json:"upgrades"`
}

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

func (o Objects) Of(category Category) Ordered[Object] {
	if held := o.of(category); held != nil {
		return *held
	}
	return Ordered[Object]{}
}

func (o Objects) Empty() bool {
	for _, category := range Categories {
		if o.Of(category).Len() > 0 {
			return false
		}
	}
	return true
}

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

type Object struct {
	ID, Base   string
	Source     string
	Typed      Ordered[any]
	Properties Ordered[any]
}

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

func setUnlessNull(properties *Ordered[any], name string, value any) {
	if value != nil {
		properties.Set(name, value)
	}
}
