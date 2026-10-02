package main

import (
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// A hand-written miniature of the game's files, in the export's folder layout. Ids and labels are real where the
// test is about them (Holy Light, Footman); the values are made up.

// sylk is SYLK text with a header row and one row per record. A cell is a string, an int, or nil to leave it out.
func sylk(columns []string, rows ...[]any) string {
	lines := []string{"ID;PWXL;N;E"}
	header := make([]any, len(columns))
	for i, column := range columns {
		header[i] = column
	}
	for y, row := range append([][]any{header}, rows...) {
		first := true
		for x, cell := range row {
			if cell == nil {
				continue
			}
			value := ""
			switch cell := cell.(type) {
			case int:
				value = strconv.Itoa(cell)
			case string:
				value = `"` + cell + `"`
			}
			line := "C;X" + strconv.Itoa(x+1) + ";"
			if first {
				line += "Y" + strconv.Itoa(y+1) + ";"
			}
			lines = append(lines, line+"K"+value)
			first = false
		}
	}
	return strings.Join(append(lines, "E", ""), "\r\n")
}

var (
	unitMeta = []string{
		"ID", "field", "slk", "index", "category", "displayName", "type", "useHero", "useUnit", "useBuilding", "useItem",
		"useSpecific", "netsafe",
	}
	abilityMeta = []string{
		"ID", "field", "slk", "index", "repeat", "data", "category", "displayName", "type", "useUnit", "useHero", "useItem",
		"useSpecific", "notSpecific", "netsafe",
	}
	buffMeta    = []string{"ID", "field", "category", "displayName", "type", "netsafe"}
	upgradeMeta = []string{"ID", "field", "repeat", "effectType", "category", "displayName", "type", "netsafe"}
	balanceMeta = []string{"unitBalanceID", "isbldg", "Primary"}
)

const labelsFile = "_locales/enus.w3mod/ui/worldeditstrings.txt"

func gameFilesFixture() map[string]string {
	return map[string]string{
		"units/unitmetadata.slk": sylk(unitMeta,
			[]any{"uhpm", "HP", "UnitBalance", -1, "stats", "WESTRING_UHPM", "int", 1, 1, 1, 0, nil, 0},
			[]any{"unam", "Name", "Profile", 0, "text", "WESTRING_UNAM", "string", 1, 1, 1, 1, nil, 1},
			[]any{"umdl", "file", "Profile", 0, "art", "WESTRING_UMDL", "model", 1, 1, 0, 0, nil, 1},
			[]any{"ifil", "file", "ItemData", 0, "art", "WESTRING_IFIL", "model", 0, 0, 0, 1, nil, 1},
			[]any{"ushr", "shadowOnWater", "Profile", -1, "art", "WESTRING_USHR", "bool", 0, 0, 1, 0, nil, 11},
			[]any{"uabi", "abilList", "UnitAbilities", -1, "abil", "WESTRING_UABI", "abilityList", 1, 1, 1, 0, nil, 0},
			[]any{"udea", "deathType", "UnitData", -1, "stats", "WESTRING_UDEA", "deathType", 1, 1, 1, 0, nil, 0},
			[]any{"upro", "Propernames", "Profile", -1, "text", "WESTRING_UPRO", "stringList", 1, 0, 0, 0, nil, 1},
			[]any{"ucls", "class", "Profile", -1, "stats", "WESTRING_UCLS", "string", 1, 1, 1, 0, nil, 0},
			[]any{"uver", "fileVerFlags", "Profile", -1, "art", "WESTRING_UVER", "versionFlags", 1, 1, 1, 0, nil, 1},
			[]any{nil, "orphan", "Profile", -1, "stats", "WESTRING_UHPM", "int", 1, 1, 1, 0, nil, 0},
		),
		"units/abilitymetadata.slk": sylk(abilityMeta,
			[]any{"anam", "Name", "Profile", 0, 0, 0, "text", "WESTRING_ANAM", "string", 1, 1, 1, nil, nil, 1},
			[]any{"alev", "levels", "AbilityData", -1, 0, 0, "stats", "WESTRING_ALEV", "int", 1, 1, 1, nil, nil, 0},
			[]any{"acdn", "Cool", "AbilityData", -1, 4, 0, "stats", "WESTRING_ACDN", "unreal", 1, 1, 1, nil, nil, 0},
			[]any{"aare", "Area", "AbilityData", -1, 4, 0, "stats", "WESTRING_AARE", "unreal", 1, 1, 1, nil, "AHhb", 0},
			[]any{"Hhb2", "Data", "AbilityData", -1, 4, 2, "data", "WESTRING_HHB2", "unreal", 1, 1, 1, "AHhb", nil, 0},
			[]any{"Hhb1", "Data", "AbilityData", -1, 4, 1, "data", "WESTRING_HHB1", "unreal", 1, 1, 1, "AHhb", nil, ""},
			[]any{"Htb1", "Data", "AbilityData", -1, 4, 1, "data", "WESTRING_HTB1", "unreal", 1, 1, 1, "AHtb", nil, 0},
			[]any{"Hdc1", "Data", "AbilityData", -1, 4, 12, "data", "WESTRING_HDC1", "int", 1, 1, 1, "AHtb,AHhb", nil, 0},
			[]any{"atp1", "Tip", "Profile", 0, 3, 0, "text", "WESTRING_ATP1", "string", 1, 1, 0, nil, nil, 1},
		),
		"units/abilitybuffmetadata.slk": sylk(buffMeta,
			[]any{"fnam", "EditorName", "text", "WESTRING_FNAM", "string", 1},
			[]any{"fart", "Buffart", "art", "WESTRING_FART", "icon", 1},
		),
		"units/upgrademetadata.slk": sylk(upgradeMeta,
			[]any{"gnam", "Name", 1, nil, "text", "WESTRING_GNAM", "string", 1},
			[]any{"gef1", "effect1", 0, "EffectID", "data", "WESTRING_GEF1", "upgradeEffect", 0},
			[]any{"gba1", "base1", 0, "Base", "data", "WESTRING_GBA1", "unreal", 0},
			[]any{"gmo1", "mod1", 0, "Mod", "data", "WESTRING_GMO1", "unreal", 0},
			[]any{"gpct", "pct", 0, nil, "data", "WESTRING_GPCT", "unreal", 0},
		),
		"units/unitdata.slk": sylk([]string{"unitID", "comment(s)"},
			[]any{"hfoo", "footman"}, []any{"Hpal", "paladin"}, []any{"hbar", "barracks"}, []any{"nzzz", "unnamed critter"},
		),
		"units/unitbalance.slk": sylk(balanceMeta,
			[]any{"hfoo", 0, "_"}, []any{"Hpal", 0, "STR"}, []any{"hbar", 1, "_"}, []any{"nzzz", 0, "_"},
		),
		"units/itemdata.slk": sylk([]string{"itemID", "comment"}, []any{"ratf", "claws"}),
		"units/abilitydata.slk": sylk([]string{"alias", "comments", "levels"},
			[]any{"AHhb", "holy light", 3}, []any{"AHtb", "storm bolt", 3}, []any{nil, "row without an id", 1},
		),
		"units/abilitybuffdata.slk": sylk([]string{"alias", "comments"}, []any{"Binf", "inner fire"}, []any{"BHbd", "blizzard"}),
		"units/upgradedata.slk":     sylk([]string{"upgradeid", "comments", "maxlevel"}, []any{"Rhme", "swords", 3}),
		labelsFile: strings.Join([]string{
			"[WorldEditStrings]",
			"WESTRING_UHPM=Hit Points Maximum (Base)",
			"WESTRING_UNAM=Name",
			"WESTRING_UMDL=WESTRING_MODELFILE",
			"WESTRING_MODELFILE=Model File",
			"WESTRING_IFIL=Model File",
			"WESTRING_USHR=Shadow on Water",
			"WESTRING_UABI=Abilities - Normal",
			"WESTRING_UDEA=Death Type",
			"WESTRING_UPRO=Proper Names (Hero's +1.)",
			"WESTRING_UCLS=Class",
			"WESTRING_UVER=Model File - Extra Versions",
			"WESTRING_ANAM=Name",
			"WESTRING_ALEV=Levels",
			"WESTRING_ACDN=Cooldown",
			"WESTRING_AARE=Area of Effect",
			"WESTRING_HHB2=Area of Effect",
			"WESTRING_HHB1=Amount Healed/Damaged",
			"WESTRING_HTB1=Cooldown",
			"WESTRING_HDC1=Damage Dealt (%)",
			"WESTRING_ATP1=Tooltip - Normal",
			"WESTRING_FNAM=Name",
			"WESTRING_FART=Icon",
			"WESTRING_GNAM=Name",
			"WESTRING_GEF1=Effect 1",
			"WESTRING_GBA1=Effect 1 - %s",
			"WESTRING_GMO1=Effect 1 - %s",
			"WESTRING_GPCT=% Bonus & More",
			"",
		}, "\r\n"),
		"_locales/enus.w3mod/units/humanunitstrings.txt": strings.Join([]string{
			"[hfoo]\t",
			"Name=Footman",
			"[Hpal]",
			`Name="|cffffcc00Paladin|r"`,
			"[hbar]",
			"Name=Barracks",
		}, "\r\n"),
		"_locales/enus.w3mod/units/humanabilitystrings.txt": strings.Join([]string{
			"[AHhb]",
			"Name=Holy Light",
			"[AHtb]",
			"Name=Storm Bolt",
			"[Binf]",
			"Bufftip=Inner Fire",
			"[BHbd]",
			"EditorName=Blizzard (Caster)",
			"Bufftip=Blizzard",
		}, "\n"),
		"_locales/enus.w3mod/units/humanupgradestrings.txt": "[Rhme]\nName=Iron Forged Swords,Steel Forged Swords\n",
		"_locales/enus.w3mod/units/itemstrings.txt":         "[ratf]\nName=Claws of Attack +15\n",
	}
}

// game writes the miniature export, after change has adjusted its files, and returns its folder and the path the
// metadata goes to.
func game(t *testing.T, change func(files map[string]string)) (folder, target string) {
	t.Helper()
	dir := t.TempDir()
	files := gameFilesFixture()
	if change != nil {
		change(files)
	}
	for path, content := range files {
		testkit.WriteFile(t, dir, "game/war3.w3mod/"+path, []byte(content))
	}
	return filepath.Join(dir, "game"), filepath.Join(dir, "metadata.json")
}

var unitClass = Overrides{Names: map[string]map[string]string{"units": {"ucls": "unitClass"}}}

func generate(t *testing.T, folder, version, target string, overrides Overrides) MetadataResult {
	t.Helper()
	result, err := GenerateMetadata(folder, version, target, overrides)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func generated(t *testing.T, target string) *objects.Metadata {
	t.Helper()
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var metadata objects.Metadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatal(err)
	}
	return &metadata
}

func byID(fields []objects.FieldMeta) map[string]objects.FieldMeta {
	out := map[string]objects.FieldMeta{}
	for _, field := range fields {
		out[field.ID] = field
	}
	return out
}

func ids(fields []objects.FieldMeta) []string {
	var out []string
	for _, field := range fields {
		out = append(out, field.ID)
	}
	return out
}

func names(fields []objects.FieldMeta) map[string]string {
	out := map[string]string{}
	for _, field := range fields {
		out[field.ID] = field.Name
	}
	return out
}

func equal[T any](t *testing.T, what string, got, want T) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s: got %v, want %v", what, got, want)
	}
}

func TestGenerateMetadataWritesFieldRecordsFromTheMetadataSLKs(t *testing.T) {
	folder, target := game(t, nil)
	generate(t, folder, "3.0.0.1", target, unitClass)
	metadata := generated(t, target)
	equal(t, "format", metadata.Format, 1)
	equal(t, "game", metadata.Game, "3.0.0.1")
	units := byID(metadata.Fields["units"])
	equal(t, "uhpm", units["uhpm"], objects.FieldMeta{
		ID: "uhpm", Name: "hitPointsMaximumBase", Label: "Hit Points Maximum (Base)", Category: "stats", Type: "int",
		Storage: "int", Use: []string{"unit", "hero", "building"}, Specific: []string{}, NotSpecific: []string{},
	})
	// Rows without an id are skipped; fields only items use are not unit fields.
	equal(t, "the unit fields", ids(metadata.Fields["units"]),
		[]string{"uabi", "ucls", "udea", "uhpm", "umdl", "unam", "upro", "ushr", "uver"})
	// Labels resolve through nested WESTRING references; only netsafe 1 marks a skin field (11 does not).
	equal(t, "umdl's label", units["umdl"].Label, "Model File")
	equal(t, "the skin marks", []bool{units["umdl"].Skin, units["ushr"].Skin, units["unam"].Skin}, []bool{true, false, true})
	// bool, flag and enumeration types are stored as int; lists and other types as strings.
	equal(t, "the storage",
		[]string{units["ushr"].Storage, units["udea"].Storage, units["uver"].Storage, units["uabi"].Storage, units["umdl"].Storage},
		[]string{"int", "int", "int", "string", "string"})
	equal(t, "the lists", []bool{units["uabi"].List, units["upro"].List, units["unam"].List}, []bool{true, true, false})

	items := byID(metadata.Fields["items"])
	equal(t, "the item fields", ids(metadata.Fields["items"]), []string{"ifil", "unam"})
	equal(t, "unam of items", items["unam"], units["unam"])
	equal(t, "unam's use", items["unam"].Use, []string{"unit", "hero", "building", "item"})

	abilities := byID(metadata.Fields["abilities"])
	equal(t, "per level", []bool{abilities["anam"].PerLevel, abilities["atp1"].PerLevel, abilities["acdn"].PerLevel},
		[]bool{false, true, true})
	equal(t, "the columns", []int{abilities["Hhb1"].Column, abilities["Hdc1"].Column, abilities["acdn"].Column}, []int{1, 12, 0})
	equal(t, "Hdc1's bases", abilities["Hdc1"].Specific, []string{"AHtb", "AHhb"})
	equal(t, "aare's exceptions", abilities["aare"].NotSpecific, []string{"AHhb"})
	equal(t, "Hhb1's skin mark", abilities["Hhb1"].Skin, false)
	equal(t, "Hhb1's use", abilities["Hhb1"].Use, []string{})

	upgrades := byID(metadata.Fields["upgrades"])
	equal(t, "gnam per level", upgrades["gnam"].PerLevel, true)
	// "%s" stands for the effect's own label in World Editor; the effect type names the field instead.
	equal(t, "the effect labels", []string{upgrades["gba1"].Label, upgrades["gmo1"].Label}, []string{"Effect 1 - Base", "Effect 1 - Mod"})
	equal(t, "gef1's storage", upgrades["gef1"].Storage, "string")
}

func TestGenerateMetadataDerivesFriendlyNamesFromLabelsAndRenamesClashes(t *testing.T) {
	folder, target := game(t, nil)
	result := generate(t, folder, "3.0.0.1", target, unitClass)
	metadata := generated(t, target)
	equal(t, "units", names(metadata.Fields["units"]), map[string]string{
		"uabi": "abilitiesNormal",
		"ucls": "unitClass",
		"udea": "deathType",
		"uhpm": "hitPointsMaximumBase",
		"umdl": "modelFile",
		"unam": "name",
		"upro": "properNamesHerosPlus1",
		"ushr": "shadowOnWater",
		"uver": "modelFileExtraVersions",
	})
	// "Model File" appears once per class: umdl is not an item field, ifil is not a unit field.
	equal(t, "items", names(metadata.Fields["items"]), map[string]string{"ifil": "modelFile", "unam": "name"})
	// Storm Bolt's own "Cooldown" clashes with the common one, so both take their category. Holy Light's own "Area
	// of Effect" does not clash with the common one, which does not apply to Holy Light (notSpecific).
	equal(t, "abilities", names(metadata.Fields["abilities"]), map[string]string{
		"Hdc1": "damageDealtPercent",
		"Hhb1": "amountHealedOrDamaged",
		"Hhb2": "areaOfEffect",
		"aare": "areaOfEffect",
		"Htb1": "dataCooldown",
		"acdn": "statsCooldown",
		"alev": "levels",
		"anam": "name",
		"atp1": "tooltipNormal",
	})
	equal(t, "upgrades", names(metadata.Fields["upgrades"]), map[string]string{
		"gba1": "effect1Base",
		"gef1": "effect1",
		"gmo1": "effect1Mod",
		"gnam": "name",
		"gpct": "percentBonusAndMore",
	})
	equal(t, "renames", result.Renames, []string{
		`units ucls "class" -> "unitClass" (override)`,
		`abilities Htb1 "cooldown" -> "dataCooldown" (category prefix)`,
		`abilities acdn "cooldown" -> "statsCooldown" (category prefix)`,
	})
}

func TestGenerateMetadataAppendsTheRawcodeWhenTheCategoryPrefixLeavesAClash(t *testing.T) {
	folder, target := game(t, func(files map[string]string) {
		files[labelsFile] += "WESTRING_HHB1=Damage\r\nWESTRING_HDC1=Damage\r\n"
	})
	result := generate(t, folder, "3.0.0.1", target, unitClass)
	abilities := names(generated(t, target).Fields["abilities"])
	equal(t, "names", []string{abilities["Hhb1"], abilities["Hdc1"]}, []string{"dataDamageHhb1", "dataDamageHdc1"})
	if !slices.Contains(result.Renames, `abilities Hhb1 "damage" -> "dataDamageHhb1" (category prefix and rawcode)`) {
		t.Errorf("renames: %q", result.Renames)
	}
}

func TestGenerateMetadataClassifiesBasesAndReadsNamesAndLevelCounts(t *testing.T) {
	folder, target := game(t, nil)
	generate(t, folder, "3.0.0.1", target, unitClass)
	three := 3
	equal(t, "bases", generated(t, target).Bases, map[objects.Category]map[string]objects.BaseMeta{
		"heroes":    {"Hpal": {Name: "Paladin"}},
		"units":     {"hfoo": {Name: "Footman"}, "nzzz": {Name: "unnamed critter"}},
		"buildings": {"hbar": {Name: "Barracks"}},
		"items":     {"ratf": {Name: "Claws of Attack +15"}},
		"abilities": {"AHhb": {Name: "Holy Light", Levels: &three}, "AHtb": {Name: "Storm Bolt", Levels: &three}},
		"buffs":     {"BHbd": {Name: "Blizzard (Caster)"}, "Binf": {Name: "Inner Fire"}},
		"upgrades":  {"Rhme": {Name: "Iron Forged Swords", Levels: &three}},
	})
}

func TestGenerateMetadataIsDeterministicAndReportsCountsPerCategory(t *testing.T) {
	folder, target := game(t, nil)
	first := generate(t, folder, "3.0.0.1", target, unitClass)
	written, _ := os.ReadFile(target)
	second := generate(t, folder, "3.0.0.1", target, unitClass)
	again, _ := os.ReadFile(target)
	equal(t, "the second file", string(again), string(written))
	equal(t, "the second result", second, first)
	equal(t, "field counts", first.Fields, []Count{{"units", 9}, {"items", 2}, {"abilities", 9}, {"buffs", 2}, {"upgrades", 5}})
	equal(t, "base counts", first.Bases, []Count{
		{"heroes", 1}, {"units", 2}, {"buildings", 1}, {"items", 1}, {"abilities", 2}, {"buffs", 2}, {"upgrades", 1},
	})
	// One record per line keeps the file small and its diffs readable.
	contains(t, string(written),
		"\n      {\"id\":\"uhpm\",\"name\":\"hitPointsMaximumBase\",",
		"\n      \"AHhb\": {\"name\":\"Holy Light\",\"levels\":3},",
		// JSON.stringify leaves "&" as it is, which Go's encoder would escape.
		`"label":"% Bonus & More"`,
	)
}

func TestGenerateMetadataFailsWithoutWritingOnAPklKeywordOrReservedNameWithoutAnOverride(t *testing.T) {
	folder, target := game(t, nil)
	_, err := GenerateMetadata(folder, "3.0.0.1", target, Overrides{})
	if err == nil {
		t.Fatal("the name class was accepted")
	}
	contains(t, err.Error(), `units ucls "class" (Class)`, "overrides.json")
	if _, err := os.Stat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Error("the metadata was written")
	}

	folder, target = game(t, func(files map[string]string) { files[labelsFile] += "WESTRING_FART=Base\r\n" })
	if _, err = GenerateMetadata(folder, "3.0.0.1", target, unitClass); err == nil {
		t.Fatal("the name base was accepted")
	}
	contains(t, err.Error(), `buffs fart "base" (Base)`)
}

func TestGenerateMetadataFailsWhenAReleasedFriendlyNameWouldChangeOrDisappear(t *testing.T) {
	folder, target := game(t, nil)
	generate(t, folder, "3.0.0.1", target, unitClass)
	released, _ := os.ReadFile(target)
	renamed := strings.Replace(string(released), `"name":"hitPointsMaximumBase"`, `"name":"hitPoints"`, 1)
	renamed = strings.Replace(renamed, `{"id":"gpct"`, `{"id":"gold"`, 1)
	if err := os.WriteFile(target, []byte(renamed), 0o666); err != nil {
		t.Fatal(err)
	}

	_, err := GenerateMetadata(folder, "3.0.0.2", target, unitClass)
	if err == nil {
		t.Fatal("released names changed without an override")
	}
	contains(t, err.Error(),
		`units uhpm "hitPoints" would become "hitPointsMaximumBase"`,
		`upgrades gold "percentBonusAndMore" would disappear`,
	)
	if kept, _ := os.ReadFile(target); string(kept) != renamed {
		t.Error("the metadata was written")
	}

	// An override pins the released name; removed acknowledges a field the game no longer has.
	generate(t, folder, "3.0.0.2", target, Overrides{
		Names:   map[string]map[string]string{"units": {"ucls": "unitClass", "uhpm": "hitPoints"}},
		Removed: map[string][]string{"upgrades": {"gold"}},
	})
	equal(t, "uhpm's name", byID(generated(t, target).Fields["units"])["uhpm"].Name, "hitPoints")
}

func TestGenerateMetadataFailsWhenAnUppercaseUnitIDIsNotAHeroInTheBalanceData(t *testing.T) {
	folder, target := game(t, func(files map[string]string) {
		files["units/unitbalance.slk"] = sylk(balanceMeta,
			[]any{"hfoo", 0, "_"}, []any{"Hpal", 0, "_"}, []any{"hbar", 1, "_"}, []any{"nzzz", 0, "_"}, []any{"nhro", 0, "AGI"},
		)
		files["units/unitdata.slk"] = sylk([]string{"unitID"},
			[]any{"hfoo"}, []any{"Hpal"}, []any{"hbar"}, []any{"nzzz"}, []any{"nhro"},
		)
	})
	_, err := GenerateMetadata(folder, "3.0.0.1", target, unitClass)
	if err == nil {
		t.Fatal("the exceptions to the hero rule were accepted")
	}
	contains(t, err.Error(), "Hpal", "nhro")
}

func TestGenerateMetadataNamesAMissingGameFile(t *testing.T) {
	folder, target := game(t, func(files map[string]string) { delete(files, "units/upgradedata.slk") })
	_, err := GenerateMetadata(folder, "3.0.0.1", target, unitClass)
	if err == nil {
		t.Fatal("a missing file was accepted")
	}
	contains(t, err.Error(), "war3.w3mod/units/upgradedata.slk")
}

func TestGenerateMetadataPadsAThreeLetterFieldIDWithNULAndReadsADotAsAListSeparator(t *testing.T) {
	folder, target := game(t, func(files map[string]string) {
		files["units/abilitymetadata.slk"] = strings.Replace(files["units/abilitymetadata.slk"], "\r\nE\r\n",
			"\r\nC;X1;Y11;K\"Crs\"\r\nC;X2;K\"Data\"\r\nC;X5;K4\r\nC;X6;K1\r\nC;X7;K\"data\"\r\nC;X8;K\"WESTRING_CRS\"\r\n"+
				"C;X9;K\"unreal\"\r\nC;X13;K\"AHtb.AHhb\"\r\nE\r\n", 1)
		files[labelsFile] += "WESTRING_CRS=Chance to Miss\r\n"
	})
	generate(t, folder, "3.0.0.1", target, unitClass)
	for _, field := range generated(t, target).Fields["abilities"] {
		if field.Label == "Chance to Miss" {
			equal(t, "the id", field.ID, "Crs\x00")
			equal(t, "the bases", field.Specific, []string{"AHtb", "AHhb"})
			return
		}
	}
	t.Error("the field is missing")
}

// The renderer writes the file as the TypeScript generator did: the committed file, read and rendered again, is
// itself.
func TestTheCommittedMetadataRendersToItself(t *testing.T) {
	root := testkit.RepoRoot(t)
	committed, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(metadataPath)))
	if err != nil {
		t.Fatal(err)
	}
	metadata, err := readMetadata(root)
	if err != nil {
		t.Fatal(err)
	}
	if RenderMetadata(metadata) != string(committed) {
		t.Errorf("%s does not render to itself", metadataPath)
	}
	// Every category the file has is one the renderer writes.
	if categories := slices.Sorted(maps.Keys(metadata.Fields)); !slices.Equal(categories, slices.Sorted(slices.Values(objects.FieldCategories))) {
		t.Errorf("field categories: %q", categories)
	}
}
