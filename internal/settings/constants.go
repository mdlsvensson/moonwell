package settings

import (
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/txt"
)

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

func textSections(s manifest.Settings, manifestFile string) (misc, skin []txt.Section, err error) {
	raw, err := sections(s.GameplayConstants, "settings.gameplayConstants", manifestFile)
	if err != nil {
		return nil, nil, err
	}
	if skin, err = sections(s.GameInterface, "settings.gameInterface", manifestFile); err != nil {
		return nil, nil, err
	}
	if misc, err = withTyped(raw, s.Gameplay, manifestFile); err != nil {
		return nil, nil, err
	}
	return misc, skin, nil
}

func withTyped(raw []txt.Section, gameplay manifest.Gameplay, manifestFile string) ([]txt.Section, error) {
	merged := raw
	for _, constant := range typedConstants(gameplay) {
		if constant.value == nil {
			continue
		}
		var err error
		if merged, err = withConstant(merged, constant, manifestFile); err != nil {
			return nil, err
		}
	}
	return merged, nil
}

func withConstant(merged []txt.Section, constant typedConstant, manifestFile string) ([]txt.Section, error) {
	field := txt.Field{Key: constant.key, Value: strconv.Itoa(*constant.value)}
	at := slices.IndexFunc(merged, func(section txt.Section) bool { return sameName(section.Name, miscSection) })
	if at < 0 {
		return append(merged, txt.Section{Name: miscSection, Fields: []txt.Field{field}}), nil
	}
	fields := merged[at].Fields
	raw := slices.IndexFunc(fields, func(held txt.Field) bool { return sameName(held.Key, field.Key) })
	switch {
	case raw < 0:
		merged[at].Fields = append(fields, field)
	case fields[raw].Value != field.Value:
		return nil, errConflict(manifestFile, constant)
	}
	return merged, nil
}

func sections(raw manifest.Ordered[manifest.Ordered[string]], path, manifestFile string) ([]txt.Section, error) {
	var result []txt.Section
	for name, entries := range raw.All() {
		if slices.ContainsFunc(result, func(section txt.Section) bool { return sameName(section.Name, name) }) {
			return nil, errDuplicateSection(manifestFile, path, name)
		}
		fields, err := fieldsOf(entries, path+"["+strconv.Quote(name)+"]", manifestFile)
		if err != nil {
			return nil, err
		}
		result = append(result, txt.Section{Name: name, Fields: fields})
	}
	return result, nil
}

func fieldsOf(entries manifest.Ordered[string], path, manifestFile string) ([]txt.Field, error) {
	var fields []txt.Field
	for key, value := range entries.All() {
		if slices.ContainsFunc(fields, func(field txt.Field) bool { return sameName(field.Key, key) }) {
			return nil, errDuplicateKey(manifestFile, path, key)
		}
		fields = append(fields, txt.Field{Key: key, Value: value})
	}
	return fields, nil
}

func sameName(a, b string) bool { return strings.EqualFold(a, b) }

const schemaHint = "Check this field in settings against @moonwell/MapSettings.pkl."

func errDuplicateSection(manifestFile, path, name string) error {
	return &diag.Error{Msg: "Invalid or duplicate " + path + " section: " + name, File: manifestFile, Hint: schemaHint}
}

func errDuplicateKey(manifestFile, path, key string) error {
	return &diag.Error{Msg: "Invalid or duplicate " + path + " key: " + key, File: manifestFile, Hint: schemaHint}
}

func errConflict(manifestFile string, constant typedConstant) error {
	return &diag.Error{
		Msg:  "Conflicting typed and raw gameplay constant: " + constant.key + ".",
		File: manifestFile,
		Hint: "Remove the raw " + constant.key + " override or make it equal to settings.gameplay." + constant.setting + ".",
	}
}
