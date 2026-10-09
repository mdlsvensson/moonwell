package manifest

import (
	"encoding/json"
)

type Category string

var Categories = []Category{"heroes", "units", "buildings", "items", "abilities", "buffs", "upgrades"}

type Objects struct {
	Heroes    OrderedMap[Object] `json:"heroes"`
	Units     OrderedMap[Object] `json:"units"`
	Buildings OrderedMap[Object] `json:"buildings"`
	Items     OrderedMap[Object] `json:"items"`
	Abilities OrderedMap[Object] `json:"abilities"`
	Buffs     OrderedMap[Object] `json:"buffs"`
	Upgrades  OrderedMap[Object] `json:"upgrades"`
}

func (o Objects) ByCategory(category Category) OrderedMap[Object] {
	if found := o.pointerTo(category); found != nil {
		return *found
	}
	return OrderedMap[Object]{}
}

func (o Objects) IsEmpty() bool {
	for _, category := range Categories {
		if o.ByCategory(category).Len() > 0 {
			return false
		}
	}
	return true
}

func (o *Objects) MapSources(rewrite func(source string) string) {
	for _, category := range Categories {
		objects := o.pointerTo(category)
		for key, object := range objects.All() {
			object.Source = rewrite(object.Source)
			objects.Set(key, object)
		}
	}
}

func (o *Objects) pointerTo(category Category) *OrderedMap[Object] {
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

type Object struct {
	ID, Base   string
	Source     string
	Typed      OrderedMap[any]
	Properties OrderedMap[any]
}

func (o *Object) UnmarshalJSON(data []byte) error {
	var keys OrderedMap[json.RawMessage]
	if err := keys.UnmarshalJSON(data); err != nil {
		return err
	}
	*o = Object{}
	for key, value := range keys.All() {
		if err := o.decodeField(key, value); err != nil {
			return wrapWithKey(key, err)
		}
	}
	return nil
}

func (o *Object) decodeField(key string, value json.RawMessage) error {
	switch key {
	case "id":
		return json.Unmarshal(value, &o.ID)
	case "base":
		return json.Unmarshal(value, &o.Base)
	case "source":
		return json.Unmarshal(value, &o.Source)
	case "properties":
		var properties OrderedMap[any]
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

func setUnlessNull(properties *OrderedMap[any], name string, value any) {
	if value != nil {
		properties.Set(name, value)
	}
}
