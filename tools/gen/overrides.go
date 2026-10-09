package main

import (
	"maps"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
)

const overridesPath = "tools/metadata/overrides.json"

type nameOverrides struct {
	Names   map[string]map[string]string `json:"names"`
	Removed map[string][]string          `json:"removed"`
}

func readOverrides(checkout string) (nameOverrides, error) {
	var overrides nameOverrides
	if err := readHandWritten(checkout, overridesPath, &overrides); err != nil {
		return nameOverrides{}, err
	}
	return overrides, nil
}

func (o nameOverrides) pinnedName(list, id string) (string, bool) {
	for _, sharedList := range listsSharingTable(list) {
		if name, ok := o.Names[sharedList][displayRawcode(id)]; ok {
			return name, true
		}
	}
	return "", false
}

func listsSharingTable(list string) []string {
	if list == "units" || list == "items" {
		return []string{"units", "items"}
	}
	return []string{list}
}

func (o nameOverrides) unusedPins(fields map[string][]objects.FieldMeta) []string {
	var problems []string
	for _, list := range slices.Sorted(maps.Keys(o.Names)) {
		if !slices.Contains(objects.FieldLists, list) {
			problems = append(problems, describeUnknownList("names", list))
			continue
		}
		for _, id := range slices.Sorted(maps.Keys(o.Names[list])) {
			if !hasField(fields, listsSharingTable(list), id) {
				problems = append(problems, describeUnusedPin(list, id))
			}
		}
	}
	for _, list := range slices.Sorted(maps.Keys(o.Removed)) {
		if !slices.Contains(objects.FieldLists, list) {
			problems = append(problems, describeUnknownList("removed", list))
		}
	}
	return problems
}

func hasField(fields map[string][]objects.FieldMeta, lists []string, id string) bool {
	for _, list := range lists {
		for _, field := range fields[list] {
			if displayRawcode(field.ID) == id {
				return true
			}
		}
	}
	return false
}

func describeUnknownList(key, list string) string {
	return key + "." + list + " is none of the lists of fields (" + joinNames(objects.FieldLists) +
		"): correct its name in " + overridesPath
}

func describeUnusedPin(list, id string) string {
	return "the pin of " + fsx.QuoteJSON(id) + " under names." + list + " names no field: no field of " +
		strings.Join(listsSharingTable(list), " or ") + " has that id; correct the id or take the pin out of " + overridesPath
}
