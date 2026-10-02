package testkit

import (
	"encoding/binary"
	"math"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/objects"
)

// SyntheticMod is one modification of a synthetic modification file. An empty End is the usual end token of four
// NUL bytes.
type SyntheticMod struct {
	Field  string
	Level  int32
	Column int32
	Value  objects.ModValue
	End    string
}

// SyntheticSet is one set of modifications.
type SyntheticSet struct {
	Flag int32
	Mods []SyntheticMod
}

// SyntheticObject is one object. Mods is shorthand for one set with flag 0; version 1 and 2 files take exactly one
// set and write no set fields.
type SyntheticObject struct {
	Base, ID string
	Sets     []SyntheticSet
	Mods     []SyntheticMod
}

// BuildModFile encodes a modification file independently of the production code, from the layout seen in the
// names fixture.
func BuildModFile(version int32, original, custom []SyntheticObject, kind objects.TableKind) []byte {
	var out []byte
	i32 := func(n int32) { out = binary.LittleEndian.AppendUint32(out, uint32(n)) }
	id := func(s string) {
		count := 0
		for _, r := range s {
			if r > 0xff {
				panic("test id must be 4 Latin-1 characters: " + s)
			}
			out = append(out, byte(r))
			count++
		}
		if count != 4 {
			panic("test id must be 4 Latin-1 characters: " + s)
		}
	}
	varTypes := []string{"int", "real", "unreal", "string"}
	i32(version)
	for _, table := range [][]SyntheticObject{original, custom} {
		i32(int32(len(table)))
		for _, object := range table {
			id(object.Base)
			id(object.ID)
			sets := object.Sets
			if sets == nil {
				sets = []SyntheticSet{{Mods: object.Mods}}
			}
			if version >= 3 {
				i32(int32(len(sets)))
			} else if len(sets) != 1 {
				panic("v1/v2 objects have exactly one set")
			}
			for _, set := range sets {
				if version >= 3 {
					i32(set.Flag)
				}
				i32(int32(len(set.Mods)))
				for _, mod := range set.Mods {
					id(mod.Field)
					i32(int32(slices.Index(varTypes, mod.Value.Type)))
					if kind == objects.Leveled {
						i32(mod.Level)
						i32(mod.Column)
					}
					switch mod.Value.Type {
					case "string":
						out = append(append(out, mod.Value.Text...), 0)
					case "int":
						i32(int32(mod.Value.Number))
					default:
						out = binary.LittleEndian.AppendUint32(out, math.Float32bits(float32(mod.Value.Number)))
					}
					if mod.End == "" {
						out = append(out, 0, 0, 0, 0)
					} else {
						id(mod.End)
					}
				}
			}
		}
	}
	return out
}

// MetaField is a field of the miniature metadata; the defaults describe an unleveled int field with no
// restrictions, and change overrides them.
func MetaField(id, name string, change func(*objects.FieldMeta)) objects.FieldMeta {
	field := objects.FieldMeta{
		ID: id, Name: name, Label: name, Category: "stats", Type: "int", Storage: "int",
		Use: []string{}, Specific: []string{}, NotSpecific: []string{},
	}
	if change != nil {
		change(&field)
	}
	return field
}

// MiniMetadata is hand-written miniature metadata, shaped like metadata.json, so that resolution tests do not
// change when the game data is regenerated. Labels and rawcodes follow the game's where they exist.
func MiniMetadata() *objects.Metadata {
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
				MetaField("uacq", "acquisitionRange", func(f *objects.FieldMeta) {
					f.Label, f.Type, f.Storage, f.Use = "Acquisition Range", "unreal", "unreal", unitUses
				}),
				MetaField("ubui", "structuresBuilt", func(f *objects.FieldMeta) {
					f.Label, f.Category, f.Type, f.Storage, f.List = "Structures Built", "techtree", "unitList", "string", true
					f.Use = []string{"unit", "hero"}
				}),
				MetaField("uhpm", "hitPointsMaximumBase", func(f *objects.FieldMeta) {
					f.Label, f.Use = "Hit Points Maximum (Base)", unitUses
				}),
				MetaField("unam", "name", name),
				MetaField("usca", "scalingValue", func(f *objects.FieldMeta) {
					f.Label, f.Category, f.Type, f.Storage, f.Skin, f.Use = "Scaling Value", "art", "real", "real", true, unitUses
				}),
				MetaField("ustr", "startingStrength", func(f *objects.FieldMeta) {
					f.Label, f.Use = "Starting Strength", []string{"hero"}
				}),
			},
			"items": {
				MetaField("iper", "perishable", func(f *objects.FieldMeta) {
					f.Label, f.Type, f.Use = "Perishable", "bool", []string{"item"}
				}),
				MetaField("unam", "name", name),
			},
			"abilities": {
				MetaField("Crs\x00", "chanceToMiss", data("Chance to Miss", 1, "Acrs")),
				MetaField("Hhb1", "amountHealedOrDamaged", data("Amount Healed/Damaged", 1, "AHhb")),
				// Two base-specific fields sharing a friendly name, as the game data has (the names are unique per
				// base).
				MetaField("Hbz2", "damage", data("Damage", 2, "AHbz")),
				MetaField("Ucs1", "damage", data("Damage", 1, "AUcs")),
				MetaField("abuf", "buffs", func(f *objects.FieldMeta) {
					f.Label, f.Type, f.Storage, f.List, f.PerLevel = "Buffs", "buffList", "string", true, true
				}),
				MetaField("aher", "heroAbility", func(f *objects.FieldMeta) { f.Label, f.Type = "Hero Ability", "bool" }),
				MetaField("alev", "levels", func(f *objects.FieldMeta) { f.Label = "Levels" }),
				MetaField("amcs", "manaCost", func(f *objects.FieldMeta) { f.Label, f.PerLevel = "Mana Cost", true }),
				MetaField("anam", "name", text("Name", nil)),
				MetaField("aran", "castRange", func(f *objects.FieldMeta) {
					f.Label, f.Type, f.Storage, f.PerLevel = "Cast Range", "unreal", "unreal", true
				}),
				MetaField("aret", "tooltipLearn", text("Tooltip - Learn", func(f *objects.FieldMeta) {
					f.NotSpecific = []string{"Aatk"}
				})),
			},
			"buffs": {
				MetaField("feff", "isAnEffect", func(f *objects.FieldMeta) { f.Label, f.Type = "Is an Effect", "bool" }),
				MetaField("ftip", "tooltip", text("Tooltip", nil)),
			},
			"upgrades": {
				MetaField("gba1", "effect1Base", func(f *objects.FieldMeta) {
					f.Label, f.Type, f.Storage = "Effect 1 - Base", "unreal", "unreal"
				}),
				MetaField("glvl", "levels", func(f *objects.FieldMeta) { f.Label = "Levels" }),
				MetaField("gnam", "name", text("Name", func(f *objects.FieldMeta) { f.PerLevel = true })),
			},
		},
		Bases: map[objects.Category]map[string]objects.BaseMeta{
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
