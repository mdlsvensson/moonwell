package main

import (
	"cmp"
	"encoding/json"
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func newCheckoutWithPins(t testing.TB, pins string) fakeCheckout {
	t.Helper()
	c := newFakeCheckout(t)
	c.writeFile(overridesPath, pins)
	c.makeDir("data")
	return c
}

func mustReadMetadata(t testing.TB, files map[string][]byte) *objects.Metadata {
	t.Helper()
	metadata, err := decodeMetadata(files[metadataPath])
	if err != nil {
		t.Fatal(err)
	}
	return metadata
}

const miniReport = `fields: {"units":9,"items":2,"abilities":9,"buffs":2,"upgrades":5}
bases: {"heroes":1,"units":2,"buildings":1,"items":1,"abilities":2,"buffs":2,"upgrades":1}
renamed (3):
  units ucls "class" -> "unitClass" (override)
  abilities Htb1 "cooldown" -> "dataCooldown" (category prefix)
  abilities acdn "cooldown" -> "statsCooldown" (category prefix)
` + "wrote data/metadata.json. Now run `go run ./tools/gen`.\n"

func TestTheModeMetadataWritesTheSameFileTwiceAndReportsTheCountsOfEachCategory(t *testing.T) {
	dir := newExportDir(t, nil)
	c := newCheckoutWithPins(t, unitClassPins)
	output, files, err := c.runGen("metadata", dir, "3.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if output != miniReport {
		t.Errorf("the run printed\n%s\nwant\n%s", output, miniReport)
	}
	first := toTexts(files)
	beside := withGoMod(map[string]string{overridesPath: unitClassPins, metadataPath: first[metadataPath]})
	if first[metadataPath] == "" || !maps.Equal(first, beside) {
		t.Fatalf("the run left %q, want the metadata beside what the checkout held", slices.Sorted(maps.Keys(first)))
	}
	checkContains(t, first[metadataPath],
		"{\n  \"format\": 1,\n  \"game\": \"3.0.0.1\",\n  \"fields\": {\n    \"units\": [\n      {\"id\":\"uabi\",",
		"\n      {\"id\":\"uhpm\",\"name\":\"hitPointsMaximumBase\",",
		"\n      \"AHhb\": {\"name\":\"Holy Light\",\"levels\":3},",
		`"label":"% Bonus & More"`,
	)
	again, files, err := c.runGen("metadata", dir, "3.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if again != output || !maps.Equal(toTexts(files), first) {
		t.Errorf("the second run printed %q and wrote another file than the first: %s",
			again, describeDifference(first[metadataPath], toTexts(files)[metadataPath]))
	}
	if metadata := mustReadMetadata(t, files); metadata.Format != 1 || metadata.Game != "3.0.0.1" {
		t.Errorf("the file states the format %d and the game %q", metadata.Format, metadata.Game)
	}
}

func TestTheModeMetadataReportsTheRenamesInTheOrderOfTheListsAndTheIds(t *testing.T) {
	dir := newExportDir(t, func(files map[string]string) {
		files[abilityFieldsTable] = withRow(files[abilityFieldsTable],
			`C;X1;Y11;K"Crs"`, `C;X7;K"data"`, `C;X8;K"WESTRING_CRS"`, `C;X9;K"unreal"`)
		files[labelsFile] += "WESTRING_GPCT=Name\r\nWESTRING_FART=Name\r\nWESTRING_CRS=Chance to Miss\r\n"
	})
	c := newCheckoutWithPins(t, `{"names": {"items": {"unam": "unitName", "ifil": "itemModel"}, "units": {"ucls": "unitClass"},
		"abilities": {"Crs": "missChance"}}}`)
	output, files, err := c.runGen("metadata", dir, "3.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	checkContains(t, string(files[metadataPath]), `{"id":"Crs\u0000","name":"missChance",`)
	checkContains(t, output, `renamed (10):
  units ucls "class" -> "unitClass" (override)
  units unam "name" -> "unitName" (override)
  items ifil "modelFile" -> "itemModel" (override)
  abilities Crs "chanceToMiss" -> "missChance" (override)
  abilities Htb1 "cooldown" -> "dataCooldown" (category prefix)
  abilities acdn "cooldown" -> "statsCooldown" (category prefix)
  buffs fart "name" -> "artName" (category prefix)
  buffs fnam "name" -> "textName" (category prefix)
  upgrades gnam "name" -> "textName" (category prefix)
  upgrades gpct "name" -> "dataName" (category prefix)
`)
}

func TestTheModeMetadataWritesNothingWhenItRefuses(t *testing.T) {
	const released = `{"format":1,"game":"1.0","fields":{"units":[{"id":"uhpm","name":"hitPoints"}]}}`
	whole := newExportDir(t, nil)
	for name, c := range map[string]struct {
		dir    string
		pins   string
		kept   string
		starts string
		words  []string
	}{
		"a name that needs a pin": {dir: whole, pins: "{}", starts: "cannot derive friendly names:\n  ",
			words: []string{`units ucls "class" (Class)`, "tools/metadata/overrides.json"}},
		"a file that the export lacks": {
			dir:    newExportDir(t, func(files map[string]string) { delete(files, upgradesTable) }),
			starts: "war3.w3mod/units/upgradedata.slk is missing from "},
		"a table whose column netsafe has another name": {
			dir: newExportDir(t, func(files map[string]string) {
				files[unitFieldsTable] = strings.Replace(files[unitFieldsTable], `K"netsafe"`, `K"netSafe"`, 1)
			}),
			starts: `war3.w3mod/units/unitmetadata.slk has no column "netsafe"`,
			words:  []string{"tools/gen/export.go"}},
		"a name that needs a pin, in an export with a table that lacks a column": {pins: "{}",
			dir: newExportDir(t, func(files map[string]string) {
				files[upgradesTable] = strings.Replace(files[upgradesTable], `K"maxlevel"`, `K"levels"`, 1)
			}),
			starts: `war3.w3mod/units/upgradedata.slk has no column "maxlevel"`},
		"a name that needs a pin, in an export with a table that does not parse": {pins: "{}",
			dir: newExportDir(t, func(files map[string]string) {
				files[upgradesTable] = "ID;PWXL;N;E\r\nC;X1;Y1;K\"upgradeid\"\r\nC;X1;Y2;K\"Rhme\r\nE\r\n"
			}),
			starts: "war3.w3mod/units/upgradedata.slk:3: unterminated quoted string"},
		"a unit that breaks the rule for heroes": {
			dir: newExportDir(t, func(files map[string]string) {
				files[unitsTable] = withRow(files[unitsTable], `C;X1;Y6;K"Nhro"`)
				files[balanceTable] = withRow(files[balanceTable], `C;X1;Y6;K"Nhro"`, `C;X2;K0`, `C;X3;K"_"`)
			}),
			starts: "standard units break the rule for heroes: Nhro (uppercase, "},
		"a released name that would change": {dir: whole, kept: released,
			starts: "released friendly names would change.",
			words:  []string{`units uhpm "hitPoints" would become "hitPointsMaximumBase"`}},
		"a released file that is no JSON": {dir: whole, kept: `{"format":`, starts: "data/metadata.json: "},
		"overrides that are no JSON":      {dir: whole, pins: `{"names":`, starts: "tools/metadata/overrides.json: "},
		"overrides with a key too many": {dir: whole, pins: `{"names": {}, "renamed": {}}`,
			starts: `tools/metadata/overrides.json: unknown field "renamed"`},
		"a pin under a list that is none": {dir: whole,
			pins:   `{"names": {"units": {"ucls": "unitClass"}, "ability": {"anam": "title"}}}`,
			starts: "cannot derive friendly names:\n  names.ability is none of the lists of fields"},
		"a pin of an id that no field has": {dir: whole,
			pins:   `{"names": {"units": {"ucls": "unitClass", "uhpn": "health"}}}`,
			starts: "cannot derive friendly names:\n  the pin of \"uhpn\" under names.units names no field"},
	} {
		scratch := newCheckoutWithPins(t, cmp.Or(c.pins, unitClassPins))
		if c.kept != "" {
			scratch.writeFile(metadataPath, c.kept)
		}
		before := scratch.readAll()
		output, files, err := scratch.runGen("metadata", c.dir, "3.0.0.2")
		if err == nil {
			t.Errorf("%s: the run wrote the metadata", name)
			continue
		}
		if !strings.HasPrefix(err.Error(), c.starts) {
			t.Errorf("%s: the error is %q, want it to start with %q", name, err, c.starts)
		}
		checkContains(t, err.Error(), c.words...)
		if strings.Contains(err.Error(), scratch.root) {
			t.Errorf("%s: the error holds the full path of the checkout: %q", name, err)
		}
		if output != "" || !reflect.DeepEqual(files, before) {
			t.Errorf("%s: the refused run printed %q and left %q, want what the checkout held",
				name, output, slices.Sorted(maps.Keys(files)))
		}
	}
}

func TestTheModeMetadataNamesTheFileOfTheCheckoutItFailsOn(t *testing.T) {
	dir := newExportDir(t, nil)
	for name, c := range map[string]struct {
		lay    func(c fakeCheckout)
		starts string
	}{
		"no overrides": {func(c fakeCheckout) { c.makeDir("data") }, "tools/metadata/overrides.json: "},
		"a folder at the place of the released metadata": {func(c fakeCheckout) {
			c.writeFile(overridesPath, unitClassPins)
			c.makeDir(metadataPath)
		}, "data/metadata.json: "},
		"no data folder": {func(c fakeCheckout) { c.writeFile(overridesPath, unitClassPins) }, "data/metadata.json: "},
	} {
		scratch := newFakeCheckout(t)
		c.lay(scratch)
		before := scratch.readAll()
		output, files, err := scratch.runGen("metadata", dir, "3.0.0.1")
		if err == nil || !strings.HasPrefix(err.Error(), c.starts) || strings.Contains(err.Error(), scratch.root) {
			t.Errorf("%s: got %v, want a failure that starts with %q and holds no path of the checkout",
				name, err, c.starts)
		}
		if output != "" || !reflect.DeepEqual(files, before) {
			t.Errorf("%s: the run printed %q and left %q", name, output, slices.Sorted(maps.Keys(files)))
		}
	}
}

func TestRenderMetadataWritesTheTextOfTheFile(t *testing.T) {
	none, three := 0, 3
	metadata := &objects.Metadata{
		Format: 1,
		Game:   "1.0 \"beta\"",
		Fields: map[string][]objects.FieldMeta{
			"units": {
				{ID: "uabi", Name: "abilities", Label: "Abilities <&>", Category: "abil", Type: "abilityList",
					Storage: "string", List: true, PerLevel: true, Column: 12, Skin: true,
					Use: []string{"unit", "hero"}, Specific: []string{"AHhb", "AHtb"}, NotSpecific: []string{"Aloc"}},
				{ID: "Crs\x00", Name: "tab\tAndQuote\"", Use: []string{}},
			},
			"upgrades":  {},
			"elsewhere": {{ID: "none", Name: "notWritten"}},
		},
		Bases: map[manifest.Category]map[string]objects.BaseMeta{
			"heroes":    {"Hpal": {Name: "Paladin"}, "Hamg": {Name: "Archmage"}, "\xEE\x80\x80": {}, "\xF0\x90\x80\x80": {}},
			"abilities": {"AHhb": {Name: "Holy\nLight", Levels: &three}, "Aloc": {Name: "Locust", Levels: &none}},
		},
	}
	const want = `{
  "format": 1,
  "game": "1.0 \"beta\"",
  "fields": {
    "units": [
      {"id":"uabi","name":"abilities","label":"Abilities <&>","category":"abil","type":"abilityList",` +
		`"storage":"string","list":true,"perLevel":true,"column":12,"skin":true,"use":["unit","hero"],` +
		`"specific":["AHhb","AHtb"],"notSpecific":["Aloc"]},
      {"id":"Crs\u0000","name":"tab\tAndQuote\"","label":"","category":"","type":"","storage":"","list":false,` +
		`"perLevel":false,"column":0,"skin":false,"use":[],"specific":[],"notSpecific":[]}
    ],
    "items": [],
    "abilities": [],
    "buffs": [],
    "upgrades": []
  },
  "bases": {
    "heroes": {
      "Hamg": {"name":"Archmage"},
      "Hpal": {"name":"Paladin"},
      "` + "\xEE\x80\x80" + `": {"name":""},
      "` + "\xF0\x90\x80\x80" + `": {"name":""}
    },
    "units": {},
    "buildings": {},
    "items": {},
    "abilities": {
      "AHhb": {"name":"Holy\nLight","levels":3},
      "Aloc": {"name":"Locust","levels":0}
    },
    "buffs": {},
    "upgrades": {}
  }
}
`
	if got := renderMetadata(metadata); got != want {
		t.Errorf("the metadata is not written as the file has it: %s", describeDifference(want, got))
	}
}

func TestTheCommittedMetadataRendersToItself(t *testing.T) {
	committed := string(moonwell.Metadata)
	metadata := objects.LoadMetadata()
	if rendered := renderMetadata(metadata); rendered != committed {
		t.Errorf("%s does not render to itself: %s", metadataPath, describeDifference(committed, rendered))
	}
	if string(realFile(t, metadataPath)) != committed {
		t.Errorf("the program does not carry %s as the checkout has it", metadataPath)
	}
	lists, five := slices.Sorted(maps.Keys(metadata.Fields)), slices.Sorted(slices.Values(objects.FieldLists))
	if !slices.Equal(lists, five) {
		t.Errorf("the lists of fields of %s are %q, want the five that are written", metadataPath, lists)
	}
}

func TestTheModeMetadataWritesTheCommittedMetadataFromTheGamesFiles(t *testing.T) {
	export := testkit.NeedExport(t, "MOONWELL_GAME_DATA").Path()
	committed := objects.LoadMetadata()
	want := string(realFile(t, metadataPath))
	var lists, categories []string
	for _, list := range objects.FieldLists {
		lists = append(lists, fmt.Sprintf("%q:%d", list, len(committed.Fields[list])))
	}
	for _, category := range manifest.Categories {
		categories = append(categories, fmt.Sprintf("%q:%d", category, len(committed.Bases[category])))
	}
	counts := "fields: {" + strings.Join(lists, ",") + "}\nbases: {" + strings.Join(categories, ",") + "}\nrenamed ("
	for name, carried := range map[string][]string{
		"over the committed metadata": {overridesPath, metadataPath},
		"into an empty data folder":   {overridesPath},
	} {
		c := newFakeCheckout(t)
		c.makeDir("data")
		c.copyRealFiles(carried...)
		output, files, err := c.runGen("metadata", export, committed.Game)
		if err != nil {
			t.Errorf("%s: the run failed, and the first line of its error is %q", name, firstLine(err.Error()))
			continue
		}
		if got := string(files[metadataPath]); got != want {
			t.Errorf("%s: the game's files do not give the committed %s: the two part at offset %d",
				name, metadataPath, firstDifferenceOffset(want, got))
		}
		const last = "\nwrote data/metadata.json. Now run `go run ./tools/gen`.\n"
		if !strings.HasPrefix(output, counts) || !strings.HasSuffix(output, last) {
			t.Errorf("%s: the run printed %d bytes, and not the counts of the committed file first and the "+
				"command to run next last", name, len(output))
		}
	}
}

func TestTheModeMetadataWritesAFileThatReadsBackAsItWasMade(t *testing.T) {
	game := readMiniExport(t, nil)
	fields, _, err := buildNamedFields(game, unitClass)
	if err != nil {
		t.Fatal(err)
	}
	bases, err := buildBases(game)
	if err != nil {
		t.Fatal(err)
	}
	made := &objects.Metadata{Format: 1, Game: "3.0.0.1", Fields: fields, Bases: bases}
	var read objects.Metadata
	if err := json.Unmarshal([]byte(renderMetadata(made)), &read); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(&read, made) {
		t.Errorf("the metadata reads back as %+v, want %+v", read, made)
	}
}
