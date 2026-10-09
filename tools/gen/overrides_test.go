package main

import (
	"reflect"
	"testing"
)

func TestTheOverridesAreThePinsAndTheFieldsThatAreRemoved(t *testing.T) {
	const text = `{
		"names": {"abilities": {"Tau1": "preferHostiles", "Crs": "missChance"}, "upgrades": {"gcls": "upgradeClass"}},
		"removed": {"units": ["uold", "uolder"]}
	}`
	want := nameOverrides{
		Names: map[string]map[string]string{
			"abilities": {"Tau1": "preferHostiles", "Crs": "missChance"}, "upgrades": {"gcls": "upgradeClass"}},
		Removed: map[string][]string{"units": {"uold", "uolder"}},
	}
	if got, err := decodeOverrides([]byte(text)); err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("the overrides are %+v, %v; want %+v", got, err, want)
	}
	for text, want := range map[string]nameOverrides{
		`{}`:                               {},
		`null`:                             {},
		`{"names": null, "removed": null}`: {},
		`{"removed": {}}`:                  {Removed: map[string][]string{}},
		`{"names": {"units": {}}}`:         {Names: map[string]map[string]string{"units": {}}},
	} {
		if got, err := decodeOverrides([]byte(text)); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: got %+v, %v; want %+v", text, got, err, want)
		}
	}
	committed, err := decodeOverrides(realFile(t, overridesPath))
	if err != nil || len(committed.Names) == 0 {
		t.Errorf("the committed overrides are read as %d lists of pins, %v", len(committed.Names), err)
	}
}

func decodeOverrides(data []byte) (nameOverrides, error) {
	var pins nameOverrides
	if err := decodeHandWritten(overridesPath, data, &pins); err != nil {
		return nameOverrides{}, err
	}
	return pins, nil
}
