package settings

import (
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/txt"
)

func textSections(settings manifest.Settings, manifestName string) (misc, skin []txt.Section, err error) {
	rawSections, err := toSections(settings.GameplayConstants, "settings.gameplayConstants", manifestName)
	if err != nil {
		return nil, nil, err
	}
	if skin, err = toSections(settings.GameInterface, "settings.gameInterface", manifestName); err != nil {
		return nil, nil, err
	}
	if misc, err = mergeTypedConstants(rawSections, settings.Gameplay, manifestName); err != nil {
		return nil, nil, err
	}
	return misc, skin, nil
}

func toSections(constants manifest.OrderedMap[manifest.OrderedMap[string]], path, manifestName string) ([]txt.Section, error) {
	var result []txt.Section
	for name, entries := range constants.All() {
		if slices.ContainsFunc(result, func(section txt.Section) bool { return equalFold(section.Name, name) }) {
			return nil, errDuplicateSection(manifestName, path, name)
		}
		fields, err := toFields(entries, path+"["+strconv.Quote(name)+"]", manifestName)
		if err != nil {
			return nil, err
		}
		result = append(result, txt.Section{Name: name, Fields: fields})
	}
	return result, nil
}

func toFields(entries manifest.OrderedMap[string], path, manifestName string) ([]txt.Field, error) {
	var fields []txt.Field
	for key, value := range entries.All() {
		if slices.ContainsFunc(fields, func(field txt.Field) bool { return equalFold(field.Key, key) }) {
			return nil, errDuplicateKey(manifestName, path, key)
		}
		fields = append(fields, txt.Field{Key: key, Value: value})
	}
	return fields, nil
}

const miscSection = "Misc"

type typedConstant struct {
	setting, key string
	value        *int
}

func typedConstants(gameplay manifest.Gameplay) []typedConstant {
	return []typedConstant{
		{"heroMaxLevel", "MaxHeroLevel", gameplay.HeroMaxLevel},
		{"foodLimit", "FoodCeiling", gameplay.FoodLimit},
	}
}

func mergeTypedConstants(sections []txt.Section, gameplay manifest.Gameplay, manifestName string) ([]txt.Section, error) {
	merged := sections
	for _, constant := range typedConstants(gameplay) {
		if constant.value == nil {
			continue
		}
		var err error
		if merged, err = mergeConstant(merged, constant, manifestName); err != nil {
			return nil, err
		}
	}
	return merged, nil
}

func mergeConstant(merged []txt.Section, constant typedConstant, manifestName string) ([]txt.Section, error) {
	field := txt.Field{Key: constant.key, Value: strconv.Itoa(*constant.value)}
	sectionIndex := slices.IndexFunc(merged, func(section txt.Section) bool { return equalFold(section.Name, miscSection) })
	if sectionIndex < 0 {
		return append(merged, txt.Section{Name: miscSection, Fields: []txt.Field{field}}), nil
	}
	fields := merged[sectionIndex].Fields
	fieldIndex := slices.IndexFunc(fields, func(existing txt.Field) bool { return equalFold(existing.Key, field.Key) })
	switch {
	case fieldIndex < 0:
		merged[sectionIndex].Fields = append(fields, field)
	case fields[fieldIndex].Value != field.Value:
		return nil, errConflict(manifestName, constant)
	}
	return merged, nil
}

func equalFold(a, b string) bool { return strings.EqualFold(a, b) }

const schemaHint = "Check this setting against the list under \"Settings\" in Moonwell's README."

func errDuplicateSection(manifestName, path, name string) error {
	return &diag.Error{Msg: "Invalid or duplicate " + path + " section: " + name, File: manifestName, Hint: schemaHint}
}

func errDuplicateKey(manifestName, path, key string) error {
	return &diag.Error{Msg: "Invalid or duplicate " + path + " key: " + key, File: manifestName, Hint: schemaHint}
}

func errConflict(manifestName string, constant typedConstant) error {
	return &diag.Error{
		Msg:  "Conflicting typed and raw gameplay constant: " + constant.key + ".",
		File: manifestName,
		Hint: "Remove the raw " + constant.key + " override or make it equal to settings.gameplay." + constant.setting + ".",
	}
}
