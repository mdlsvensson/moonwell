package objects_test

import (
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
)

// mini is the metadata the tests resolve against.
var mini = miniMetadata()

// metaField is a field of the miniature metadata; the defaults describe an unleveled int field with no
// restrictions, and change overrides them.
func metaField(id, name string, change func(*objects.FieldMeta)) objects.FieldMeta {
	field := objects.FieldMeta{
		ID: id, Name: name, Label: name, Category: "stats", Type: "int", Storage: "int",
		Use: []string{}, Specific: []string{}, NotSpecific: []string{},
	}
	if change != nil {
		change(&field)
	}
	return field
}

// miniMetadata is hand-written miniature metadata, shaped like metadata.json, so that resolution tests do not
// change when the game data is regenerated. Labels and rawcodes follow the game's where they exist.
func miniMetadata() *objects.Metadata {
	unitUses := []string{"unit", "hero", "building"}
	levels := func(n int) *int { return &n }
	text := func(label string, more func(*objects.FieldMeta)) func(*objects.FieldMeta) {
		return func(f *objects.FieldMeta) {
			f.Label, f.Category, f.Type, f.Storage, f.Skin = label, "text", "string", "string", true
			if more != nil {
				more(f)
			}
		}
	}
	data := func(label string, column int, specific string) func(*objects.FieldMeta) {
		return func(f *objects.FieldMeta) {
			f.Label, f.Category, f.Type, f.Storage, f.PerLevel = label, "data", "unreal", "unreal", true
			f.Column, f.Specific = column, []string{specific}
		}
	}
	name := text("Name", func(f *objects.FieldMeta) { f.Use = []string{"unit", "hero", "building", "item"} })
	return &objects.Metadata{
		Format: 1,
		Game:   "1.2.3.4",
		Fields: map[string][]objects.FieldMeta{
			"units": {
				metaField("uacq", "acquisitionRange", func(f *objects.FieldMeta) {
					f.Label, f.Type, f.Storage, f.Use = "Acquisition Range", "unreal", "unreal", unitUses
				}),
				metaField("ubui", "structuresBuilt", func(f *objects.FieldMeta) {
					f.Label, f.Category, f.Type, f.Storage, f.List = "Structures Built", "techtree", "unitList", "string", true
					f.Use = []string{"unit", "hero"}
				}),
				metaField("uhpm", "hitPointsMaximumBase", func(f *objects.FieldMeta) {
					f.Label, f.Use = "Hit Points Maximum (Base)", unitUses
				}),
				metaField("unam", "name", name),
				metaField("usca", "scalingValue", func(f *objects.FieldMeta) {
					f.Label, f.Category, f.Type, f.Storage, f.Skin, f.Use = "Scaling Value", "art", "real", "real", true, unitUses
				}),
				metaField("ustr", "startingStrength", func(f *objects.FieldMeta) {
					f.Label, f.Use = "Starting Strength", []string{"hero"}
				}),
			},
			"items": {
				metaField("iper", "perishable", func(f *objects.FieldMeta) {
					f.Label, f.Type, f.Use = "Perishable", "bool", []string{"item"}
				}),
				metaField("unam", "name", name),
			},
			"abilities": {
				metaField("Crs\x00", "chanceToMiss", data("Chance to Miss", 1, "Acrs")),
				metaField("Hhb1", "amountHealedOrDamaged", data("Amount Healed/Damaged", 1, "AHhb")),
				// Two base-specific fields sharing a friendly name, as the game data has (the names are unique per
				// base).
				metaField("Hbz2", "damage", data("Damage", 2, "AHbz")),
				metaField("Ucs1", "damage", data("Damage", 1, "AUcs")),
				metaField("abuf", "buffs", func(f *objects.FieldMeta) {
					f.Label, f.Type, f.Storage, f.List, f.PerLevel = "Buffs", "buffList", "string", true, true
				}),
				metaField("aher", "heroAbility", func(f *objects.FieldMeta) { f.Label, f.Type = "Hero Ability", "bool" }),
				metaField("alev", "levels", func(f *objects.FieldMeta) { f.Label = "Levels" }),
				metaField("amcs", "manaCost", func(f *objects.FieldMeta) { f.Label, f.PerLevel = "Mana Cost", true }),
				metaField("anam", "name", text("Name", nil)),
				metaField("aran", "castRange", func(f *objects.FieldMeta) {
					f.Label, f.Type, f.Storage, f.PerLevel = "Cast Range", "unreal", "unreal", true
				}),
				metaField("aret", "tooltipLearn", text("Tooltip - Learn", func(f *objects.FieldMeta) {
					f.NotSpecific = []string{"Aatk"}
				})),
			},
			"buffs": {
				metaField("feff", "isAnEffect", func(f *objects.FieldMeta) { f.Label, f.Type = "Is an Effect", "bool" }),
				metaField("ftip", "tooltip", text("Tooltip", nil)),
			},
			"upgrades": {
				metaField("gba1", "effect1Base", func(f *objects.FieldMeta) {
					f.Label, f.Type, f.Storage = "Effect 1 - Base", "unreal", "unreal"
				}),
				metaField("glvl", "levels", func(f *objects.FieldMeta) { f.Label = "Levels" }),
				metaField("gnam", "name", text("Name", func(f *objects.FieldMeta) { f.PerLevel = true })),
			},
		},
		Bases: map[manifest.Category]map[string]objects.BaseMeta{
			"heroes":    {"Hamg": {Name: "Archmage"}, "Hmkg": {Name: "Mountain King"}, "Hpal": {Name: "Paladin"}},
			"units":     {"hfoo": {Name: "Footman"}, "hkni": {Name: "Knight"}, "hpea": {Name: "Peasant"}},
			"buildings": {"hbar": {Name: "Barracks"}, "htow": {Name: "Town Hall"}},
			"items":     {"ckng": {Name: "Crown of Kings +5"}, "ratf": {Name: "Claws of Attack +15"}},
			"abilities": {
				"AHbu": {Name: "Build (Human)", Levels: levels(0)},
				"AHhb": {Name: "Holy Light", Levels: levels(3)},
				"Aatk": {Name: "Attack", Levels: levels(0)},
				"Acrs": {Name: "Curse", Levels: levels(1)},
			},
			"buffs":    {"BHbd": {Name: "Blizzard"}, "Bcrs": {Name: "Curse"}},
			"upgrades": {"Rhar": {Name: "Iron Plating", Levels: levels(3)}, "Rhme": {Name: "Iron Forged Swords", Levels: levels(3)}},
		},
	}
}
