package main

import (
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/objects"
)

func TestKeepsReleasedNamesRefusesANameThatWouldChangeOrDisappear(t *testing.T) {
	game := readMiniExport(t, nil)
	metadataOf := func(pins nameOverrides) *objects.Metadata { return buildMetadata(t, game, pins) }
	current := metadataOf(unitClass)
	released := renderMetadata(current)
	renamed := strings.Replace(released, `"name":"hitPointsMaximumBase"`, `"name":"hitPoints"`, 1)
	renamed = strings.Replace(renamed, `{"id":"gpct"`, `{"id":"gold"`, 1)
	renamed = strings.Replace(renamed, `"name":"name"`, `"name":"unitName"`, 2)
	c := newFakeCheckout(t)

	if err := checkReleasedNamesKept(current, c.root, unitClass); err != nil {
		t.Errorf("a checkout without a metadata: %v", err)
	}
	c.writeFile(metadataPath, released)
	if err := checkReleasedNamesKept(current, c.root, unitClass); err != nil {
		t.Errorf("the names that are released: %v", err)
	}
	c.writeFile(metadataPath, renamed)
	const lines = ":\n" +
		"  units uhpm \"hitPoints\" would become \"hitPointsMaximumBase\"\n" +
		"  units unam \"unitName\" would become \"name\"\n" +
		"  items unam \"unitName\" would become \"name\"\n" +
		"  upgrades gold \"percentBonusAndMore\" would disappear"
	err := checkReleasedNamesKept(current, c.root, unitClass)
	if err == nil || !strings.HasSuffix(err.Error(), lines) {
		t.Fatalf("got %v, want the lines %q", err, lines)
	}
	checkContains(t, err.Error(), "released friendly names would change. In tools/metadata/overrides.json, ",
		`under "names"`, `under "removed"`)
	if strings.Contains(err.Error(), "no longer") {
		t.Errorf("the refusal tells of what was: %v", err)
	}
	if kept := string(c.readAll()[metadataPath]); kept != renamed {
		t.Error("the check wrote the metadata")
	}

	pins := nameOverrides{
		Names:   map[string]map[string]string{"units": {"ucls": "unitClass", "uhpm": "hitPoints", "unam": "unitName"}},
		Removed: map[string][]string{"upgrades": {"gold"}, "units": {"gold"}},
	}
	pinned := metadataOf(pins)
	if err := checkReleasedNamesKept(pinned, c.root, pins); err != nil {
		t.Errorf("with the names pinned and the field removed: %v", err)
	}
	checkEqual(t, "the name of uhpm", namesOf(pinned.Fields["units"])["uhpm"], "hitPoints")
	pins.Removed = map[string][]string{"units": {"gold"}}
	if err := checkReleasedNamesKept(pinned, c.root, pins); err == nil || !strings.Contains(err.Error(), "upgrades gold") {
		t.Errorf("a field removed under another list: got %v", err)
	}
}

func TestKeepsReleasedNamesLetsAPinGiveAReleasedFieldAnotherName(t *testing.T) {
	game := readMiniExport(t, nil)
	c := newFakeCheckout(t)
	c.writeFile(metadataPath, renderMetadata(buildMetadata(t, game, unitClass)))
	for under, names := range map[string]map[string]map[string]string{
		"units": {"units": {"ucls": "unitClass", "uhpm": "health", "unam": "title"}},
		"items": {"units": {"ucls": "unitClass", "uhpm": "health"}, "items": {"unam": "title"}},
	} {
		pins := nameOverrides{Names: names}
		pinned := buildMetadata(t, game, pins)
		units, items := namesOf(pinned.Fields["units"]), namesOf(pinned.Fields["items"])
		checkEqual(t, "the names of uhpm, and of unam in both lists, with unam pinned under "+under,
			[]string{units["uhpm"], units["unam"], items["unam"]}, []string{"health", "title", "title"})
		if err := checkReleasedNamesKept(pinned, c.root, pins); err != nil {
			t.Errorf("unam pinned under %s: the pins of other names than the released ones are refused: %v", under, err)
		}
	}
}

func TestKeepsReleasedNamesTakesAndShowsTheIDOfThreeLettersAsItIsWritten(t *testing.T) {
	scratch := newFakeCheckout(t)
	scratch.writeFile(metadataPath, `{"fields": {"abilities": [{"id": "Crs\u0000", "name": "missChance"}]}}`)
	gone := &objects.Metadata{}
	renamed := &objects.Metadata{Fields: map[string][]objects.FieldMeta{
		"abilities": {{ID: "Crs\x00", Name: "chanceToMiss"}},
	}}
	for _, c := range []struct {
		current *objects.Metadata
		pins    nameOverrides
		want    string
	}{
		{gone, nameOverrides{}, `abilities Crs "missChance" would disappear`},
		{gone, nameOverrides{Removed: map[string][]string{"abilities": {"Crs"}}}, ""},
		{renamed, nameOverrides{}, `abilities Crs "missChance" would become "chanceToMiss"`},
		{renamed, nameOverrides{Names: map[string]map[string]string{"abilities": {"Crs": "chanceToMiss"}}}, ""},
	} {
		err := checkReleasedNamesKept(c.current, scratch.root, c.pins)
		switch {
		case c.want == "" && err != nil:
			t.Errorf("with the overrides %+v: %v", c.pins, err)
		case c.want != "" && (err == nil || !strings.Contains(err.Error(), "\n  "+c.want)):
			t.Errorf("with the overrides %+v: got %v, want the line %q", c.pins, err, c.want)
		}
	}
}

func TestKeepsReleasedNamesNamesAMetadataItCannotRead(t *testing.T) {
	current := &objects.Metadata{Format: 1}
	asFolder := newFakeCheckout(t)
	asFolder.makeDir(metadataPath)
	err := checkReleasedNamesKept(current, asFolder.root, nameOverrides{})
	const starts = "data/metadata.json: "
	if err == nil || !strings.HasPrefix(err.Error(), starts) || strings.Contains(err.Error(), asFolder.root) {
		t.Errorf("a folder at the place of the metadata: got %v", err)
	}
	for text, words := range map[string]string{
		`{"format": 1,`: "data/metadata.json: unexpected end of JSON input",
		"":              "data/metadata.json: unexpected end of JSON input",
		"not JSON\n":    "data/metadata.json: invalid character 'o'",
		`{"fields": 1}`: "data/metadata.json: fields is of the wrong kind (number)",
	} {
		c := newFakeCheckout(t)
		c.writeFile(metadataPath, text)
		if err := checkReleasedNamesKept(current, c.root, nameOverrides{}); err == nil || !strings.HasPrefix(err.Error(), words) {
			t.Errorf("the metadata %q: got %v, want it to start with %q", text, err, words)
		}
	}
	for _, text := range []string{`{}`, `null`, `{"fields": {"elsewhere": [{"id": "gone", "name": "gone"}]}}`} {
		c := newFakeCheckout(t)
		c.writeFile(metadataPath, text)
		if err := checkReleasedNamesKept(current, c.root, nameOverrides{}); err != nil {
			t.Errorf("the metadata %q: %v", text, err)
		}
	}
}
