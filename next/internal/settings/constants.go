package settings

import (
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/war3/txt"
)

// miscSection is the section of war3mapMisc.txt that holds the gameplay constants with a setting of their own.
const miscSection = "Misc"

// typedConstant is a gameplay constant with a setting of its own: the setting's name under settings.gameplay, the
// key it has in war3mapMisc.txt, and the value set, or nil.
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

// textSections is what the settings write into the two text files: into war3mapMisc.txt the raw gameplay
// constants with the typed ones merged in, into war3mapSkin.txt the game interface. Names that differ only in
// letter case are refused first, those of the constants before those of the interface, and then a typed constant
// that disagrees with a raw one. Every refusal names the manifest. The result is new: the settings are not
// changed.
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

// withTyped is the raw gameplay constants with the typed ones that are set merged in. A typed value that
// disagrees with a raw one fails.
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

// withConstant is the sections with the typed constant in the Misc section. A raw Misc section and a raw key of
// the constant are found in any letter case and keep their spelling; without them the constant goes into a
// section Misc after the others, or after the section's other keys.
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

// sections turns raw sections into what txt.Merge takes, and refuses two section names, or two keys of one
// section, that differ only in letter case. path is the setting the sections are written under, such as
// settings.gameInterface.
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

// fieldsOf is the keys and values of one raw section, in the order written. path is the setting the section is.
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

// sameName reports whether two names of sections or of keys are one name to the game, which reads them without
// regard to letter case.
func sameName(a, b string) bool { return strings.EqualFold(a, b) }

// ---- errors ----

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
