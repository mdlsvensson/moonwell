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
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// withPins makes a scratch checkout with an overrides file of this text and a data folder.
func withPins(t testing.TB, pins string) checkout {
	t.Helper()
	c := newCheckout(t)
	c.write(overridesPath, pins)
	c.folder("data")
	return c
}

// written reads the data/metadata.json among the outputs of a checkout.
func written(t testing.TB, files map[string][]byte) *objects.Metadata {
	t.Helper()
	metadata, err := decodeMetadata(files[metadataPath])
	if err != nil {
		t.Fatal(err)
	}
	return metadata
}

// The report of the miniature export: the counts of the lists and of the categories in their order, the renames
// in the order of the lists and then of the ids, and the command that is to be run next.
const miniReport = `fields: {"units":9,"items":2,"abilities":9,"buffs":2,"upgrades":5}
bases: {"heroes":1,"units":2,"buildings":1,"items":1,"abilities":2,"buffs":2,"upgrades":1}
renamed (3):
  units ucls "class" -> "unitClass" (override)
  abilities Htb1 "cooldown" -> "dataCooldown" (category prefix)
  abilities acdn "cooldown" -> "statsCooldown" (category prefix)
` + "wrote data/metadata.json. Now run `go run ./tools/gen`.\n"

func TestTheModeMetadataWritesTheSameFileTwiceAndReportsTheCountsOfEachCategory(t *testing.T) {
	folder := exportedGame(t, nil)
	c := withPins(t, unitClassPins)
	printed, files, err := c.run("metadata", folder, "3.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if printed != miniReport {
		t.Errorf("the run printed\n%s\nwant\n%s", printed, miniReport)
	}
	first := texts(files)
	if len(first) != 1 || first[metadataPath] == "" {
		t.Fatalf("the run left %q, want the metadata alone", slices.Sorted(maps.Keys(first)))
	}
	// One field and one standard object on a line keep the file small and its changes readable. "&" stands as it
	// is: the file escapes no more than a JSON text must.
	contains(t, first[metadataPath],
		"{\n  \"format\": 1,\n  \"game\": \"3.0.0.1\",\n  \"fields\": {\n    \"units\": [\n      {\"id\":\"uabi\",",
		"\n      {\"id\":\"uhpm\",\"name\":\"hitPointsMaximumBase\",",
		"\n      \"AHhb\": {\"name\":\"Holy Light\",\"levels\":3},",
		`"label":"% Bonus & More"`,
	)
	// The second run finds the file of the first as the names that are released, and writes it again.
	again, files, err := c.run("metadata", folder, "3.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if again != printed || !maps.Equal(texts(files), first) {
		t.Errorf("the second run printed %q and wrote another file than the first: %s",
			again, parting(first[metadataPath], texts(files)[metadataPath]))
	}
	if metadata := written(t, files); metadata.Format != 1 || metadata.Game != "3.0.0.1" {
		t.Errorf("the file states the format %d and the game %q", metadata.Format, metadata.Game)
	}
}

// The renames are reported in the order of the lists, and in a list in the order of the ids' bytes: capitals
// first. A field of units and of items is reported once, under the units, and a field that items alone use under
// the items.
func TestTheModeMetadataReportsTheRenamesInTheOrderOfTheListsAndTheIds(t *testing.T) {
	folder := exportedGame(t, func(files map[string]string) {
		files[labelsFile] += "WESTRING_GPCT=Name\r\nWESTRING_FART=Name\r\n"
	})
	c := withPins(t, `{"names": {"items": {"unam": "unitName", "ifil": "itemModel"}, "units": {"ucls": "unitClass"}}}`)
	printed, _, err := c.run("metadata", folder, "3.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	contains(t, printed, `renamed (9):
  units ucls "class" -> "unitClass" (override)
  units unam "name" -> "unitName" (override)
  items ifil "modelFile" -> "itemModel" (override)
  abilities Htb1 "cooldown" -> "dataCooldown" (category prefix)
  abilities acdn "cooldown" -> "statsCooldown" (category prefix)
  buffs fart "name" -> "artName" (category prefix)
  buffs fnam "name" -> "textName" (category prefix)
  upgrades gnam "name" -> "textName" (category prefix)
  upgrades gpct "name" -> "dataName" (category prefix)
`)
}

// A run that is refused prints nothing and leaves the data folder as it was: the file of another version is
// kept, and no file is made where there was none.
func TestTheModeMetadataWritesNothingWhenItRefuses(t *testing.T) {
	const released = `{"format":1,"game":"1.0","fields":{"units":[{"id":"uhpm","name":"hitPoints"}]}}`
	whole := exportedGame(t, nil)
	for name, c := range map[string]struct {
		folder string
		pins   string // the overrides; "" for those of the miniature
		kept   string // the data/metadata.json that the checkout holds; "" for none
		starts string // what the error starts with
		words  []string
	}{
		"a name that needs a pin": {folder: whole, pins: "{}", starts: "cannot derive friendly names:\n  ",
			words: []string{`units ucls "class" (Class)`, "tools/metadata/overrides.json"}},
		"a file that the export lacks": {
			folder: exportedGame(t, func(files map[string]string) { delete(files, upgradesTable) }),
			starts: "war3.w3mod/units/upgradedata.slk is missing from "},
		// Every file of the export is read before anything is made of it: a table that does not parse is told
		// of before a name that needs a pin.
		"a name that needs a pin, in an export with a table that does not parse": {pins: "{}",
			folder: exportedGame(t, func(files map[string]string) {
				files[upgradesTable] = "ID;PWXL;N;E\r\nC;X1;Y1;K\"upgradeid\"\r\nC;X1;Y2;K\"Rhme\r\nE\r\n"
			}),
			starts: "war3.w3mod/units/upgradedata.slk:3: unterminated quoted string"},
		"a unit that breaks the rule for heroes": {
			folder: exportedGame(t, func(files map[string]string) {
				files[unitsTable] = withRow(files[unitsTable], `C;X1;Y6;K"Nhro"`)
				files[balanceTable] = withRow(files[balanceTable], `C;X1;Y6;K"Nhro"`, `C;X2;K0`, `C;X3;K"_"`)
			}),
			starts: "standard unit ids where the uppercase hero rule disagrees", words: []string{"Nhro (uppercase, "}},
		"a released name that would change": {folder: whole, kept: released,
			starts: "released friendly names would change.",
			words:  []string{`units uhpm "hitPoints" would become "hitPointsMaximumBase"`}},
		"a released file that is no JSON": {folder: whole, kept: `{"format":`, starts: "data/metadata.json: "},
		"overrides that are no JSON":      {folder: whole, pins: `{"names":`, starts: "tools/metadata/overrides.json: "},
		"overrides with a key too many": {folder: whole, pins: `{"names": {}, "renamed": {}}`,
			starts: `tools/metadata/overrides.json: the file has the key "renamed"`},
	} {
		scratch := withPins(t, cmp.Or(c.pins, unitClassPins))
		if c.kept != "" {
			scratch.write(metadataPath, c.kept)
		}
		before := scratch.outputs()
		printed, files, err := scratch.run("metadata", c.folder, "3.0.0.2")
		if err == nil {
			t.Errorf("%s: the run wrote the metadata", name)
			continue
		}
		if !strings.HasPrefix(err.Error(), c.starts) {
			t.Errorf("%s: the error is %q, want it to start with %q", name, err, c.starts)
		}
		contains(t, err.Error(), c.words...)
		if strings.Contains(err.Error(), scratch.root) {
			t.Errorf("%s: the error holds the full path of the checkout: %q", name, err)
		}
		if printed != "" || !reflect.DeepEqual(files, before) {
			t.Errorf("%s: the refused run printed %q and left %q, want what the checkout held",
				name, printed, slices.Sorted(maps.Keys(files)))
		}
	}
}

// A failure of the system on a file of the checkout names the file by its path from the checkout: the overrides
// that are not there, the released metadata that is a folder, and the data folder that is not there to write
// into.
func TestTheModeMetadataNamesTheFileOfTheCheckoutItFailsOn(t *testing.T) {
	folder := exportedGame(t, nil)
	for name, c := range map[string]struct {
		lay    func(c checkout)
		starts string
	}{
		"no overrides": {func(c checkout) { c.folder("data") }, "tools/metadata/overrides.json: "},
		"a folder at the place of the released metadata": {func(c checkout) {
			c.write(overridesPath, unitClassPins)
			c.folder(metadataPath)
		}, "data/metadata.json: "},
		"no data folder": {func(c checkout) { c.write(overridesPath, unitClassPins) }, "data/metadata.json: "},
	} {
		scratch := newCheckout(t)
		c.lay(scratch)
		before := scratch.outputs()
		printed, files, err := scratch.run("metadata", folder, "3.0.0.1")
		if err == nil || !strings.HasPrefix(err.Error(), c.starts) || strings.Contains(err.Error(), scratch.root) {
			t.Errorf("%s: got %v, want a failure that starts with %q and holds no path of the checkout",
				name, err, c.starts)
		}
		if printed != "" || !reflect.DeepEqual(files, before) {
			t.Errorf("%s: the run printed %q and left %q", name, printed, slices.Sorted(maps.Keys(files)))
		}
	}
}

// Every kind of entry, against the text as it stands in the file: a field with every key, the id of three
// letters, a list that holds nothing and a category without an object, an object with levels and one without,
// texts that a JSON string must escape, and the objects of a category in the order of their ids' bytes.
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
		t.Errorf("the metadata is not written as the file has it: %s", parting(want, got))
	}
}

// The committed file, read and rendered again, is itself: byte for byte. Its lists of fields are the five that
// the renderer writes.
func TestTheCommittedMetadataRendersToItself(t *testing.T) {
	committed := string(moonwell.Metadata)
	metadata := objects.LoadMetadata()
	if rendered := renderMetadata(metadata); rendered != committed {
		t.Errorf("%s does not render to itself: %s", metadataPath, parting(committed, rendered))
	}
	if string(realFile(t, metadataPath)) != committed {
		t.Errorf("the program does not carry %s as the checkout has it", metadataPath)
	}
	lists, five := slices.Sorted(maps.Keys(metadata.Fields)), slices.Sorted(slices.Values(objects.FieldLists))
	if !slices.Equal(lists, five) {
		t.Errorf("the lists of fields of %s are %q, want the five that are written", metadataPath, lists)
	}
}

// The game's object data, with the version that the committed metadata state and the committed overrides, give
// the committed metadata byte for byte, over the committed file and into an empty data folder. The test reads
// the export that MOONWELL_GAME_DATA names, and takes a second or two.
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
		c := newCheckout(t)
		c.folder("data")
		c.carry(carried...)
		printed, files, err := c.run("metadata", export, committed.Game)
		if err != nil {
			t.Errorf("%s: the run failed, and the first line of its error is %q", name, firstLine(err.Error()))
			continue
		}
		if got := string(files[metadataPath]); got != want {
			t.Errorf("%s: the game's files do not give the committed %s: the two part at offset %d",
				name, metadataPath, partingOffset(want, got))
		}
		const last = "\nwrote data/metadata.json. Now run `go run ./tools/gen`.\n"
		if !strings.HasPrefix(printed, counts) || !strings.HasSuffix(printed, last) {
			t.Errorf("%s: the run printed %d bytes, and not the counts of the committed file first and the "+
				"command to run next last", name, len(printed))
		}
	}
}

// The file that the mode writes is read back as the program reads it.
func TestTheModeMetadataWritesAFileThatReadsBackAsItWasMade(t *testing.T) {
	game := readMini(t, nil)
	fields, _, err := nameFields(game, unitClass)
	if err != nil {
		t.Fatal(err)
	}
	bases, err := standardObjects(game)
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
