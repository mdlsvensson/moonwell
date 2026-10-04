package objects_test

import (
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/war3/objmod"
)

// noObjectsJSON is what objects:eval prints for a project without objects.
const noObjectsJSON = `{
  "heroes": {},
  "units": {},
  "buildings": {},
  "items": {},
  "abilities": {},
  "buffs": {},
  "upgrades": {}
}`

func TestEvalJSONPrintsEveryCategoryInOrderEvenWithoutObjects(t *testing.T) {
	for name, resolved := range map[string][]objects.Resolved{"nil": nil, "an empty list": {}} {
		if got := string(objects.EvalJSON(resolved)); got != noObjectsJSON {
			t.Errorf("%s: printed\n%s\nwant\n%s", name, got, noObjectsJSON)
		}
	}
}

func TestEvalJSONPrintsEachObjectByItsKeyWithItsIDBaseSourceAndFields(t *testing.T) {
	resolved := resolve(t, `{
		"abilities":{"holy":{"id":"A000","base":"AHhb","source":"objects/abilities.pkl","name":"Holier","manaCost":[75,80],
			"properties":{"Hhb1":[200.5]}}},
		"units":{"knight":{"id":"h001","base":"hkni"},
			"captain":{"id":"h000","base":"hfoo","name":"Captain","scalingValue":1.25,"hitPointsMaximumBase":500}}}`)
	want := `{
  "heroes": {},
  "units": {
    "knight": {
      "id": "h001",
      "base": "hkni",
      "source": "objects/a.pkl",
      "fields": []
    },
    "captain": {
      "id": "h000",
      "base": "hfoo",
      "source": "objects/a.pkl",
      "fields": [
        {
          "rawcode": "uhpm",
          "name": "hitPointsMaximumBase",
          "level": 0,
          "column": 0,
          "skin": false,
          "type": "int",
          "value": 500
        },
        {
          "rawcode": "unam",
          "name": "name",
          "level": 0,
          "column": 0,
          "skin": true,
          "type": "string",
          "value": "Captain"
        },
        {
          "rawcode": "usca",
          "name": "scalingValue",
          "level": 0,
          "column": 0,
          "skin": true,
          "type": "real",
          "value": 1.25
        }
      ]
    }
  },
  "buildings": {},
  "items": {},
  "abilities": {
    "holy": {
      "id": "A000",
      "base": "AHhb",
      "source": "objects/abilities.pkl",
      "fields": [
        {
          "rawcode": "Hhb1",
          "name": "amountHealedOrDamaged",
          "level": 1,
          "column": 1,
          "skin": false,
          "type": "unreal",
          "value": 200.5
        },
        {
          "rawcode": "amcs",
          "name": "manaCost",
          "level": 1,
          "column": 0,
          "skin": false,
          "type": "int",
          "value": 75
        },
        {
          "rawcode": "amcs",
          "name": "manaCost",
          "level": 2,
          "column": 0,
          "skin": false,
          "type": "int",
          "value": 80
        },
        {
          "rawcode": "anam",
          "name": "name",
          "level": 0,
          "column": 0,
          "skin": true,
          "type": "string",
          "value": "Holier"
        }
      ]
    }
  },
  "buffs": {},
  "upgrades": {}
}`
	if got := string(objects.EvalJSON(resolved)); got != want {
		t.Errorf("printed\n%s\nwant\n%s", got, want)
	}
}

func TestEvalJSONIsJSONThatAScriptReadsBack(t *testing.T) {
	resolved := resolve(t, `{
		"heroes":{"paladin":{"id":"H000","base":"Hpal","source":"objects/heroes.pkl","properties":{"uhpm":900}}},
		"units":{"captain":{"id":"h000","base":"hfoo","source":"objects/human/barracks/units.pkl","name":"Captain"}}}`)
	var read map[string]map[string]any
	if err := json.Unmarshal(objects.EvalJSON(resolved), &read); err != nil {
		t.Fatal(err)
	}
	if len(read) != len(manifest.Categories) {
		t.Fatalf("%d categories, want %d", len(read), len(manifest.Categories))
	}
	var want any
	if err := json.Unmarshal([]byte(`{"id":"h000","base":"hfoo","source":"objects/human/barracks/units.pkl","fields":[
		{"rawcode":"unam","name":"name","level":0,"column":0,"skin":true,"type":"string","value":"Captain"}]}`), &want); err != nil {
		t.Fatal(err)
	}
	if unit := read["units"]["captain"]; !reflect.DeepEqual(unit, want) {
		t.Errorf("the unit = %v, want %v", unit, want)
	}
	hero, _ := read["heroes"]["paladin"].(map[string]any)
	fields, _ := hero["fields"].([]any)
	if hero["source"] != "objects/heroes.pkl" || len(fields) != 1 || fields[0].(map[string]any)["rawcode"] != "uhpm" {
		t.Errorf("the hero = %v", hero)
	}
}

// oneValue is a unit with one field of this value, as Resolve gives it.
func oneValue(key string, value objects.Value) []objects.Resolved {
	return []objects.Resolved{{
		Category: "units", Key: key, ID: "h000", Base: "hfoo", Source: "objects/a.pkl",
		Fields: []objects.Field{{ID: "uacq", Name: "acquisitionRange", Value: value}},
	}}
}

// valueLine is the line of the printed JSON that holds the value of the one field.
func valueLine(t *testing.T, printed []byte) string {
	t.Helper()
	for _, line := range strings.Split(string(printed), "\n") {
		if text, found := strings.CutPrefix(line, `          "value": `); found {
			return text
		}
	}
	t.Fatalf("no value in\n%s", printed)
	return ""
}

func TestEvalJSONWritesANumberAsJSONDoesAndHasOneZero(t *testing.T) {
	cases := []struct {
		number float64
		want   string
	}{
		{0, "0"},
		{math.Copysign(0, -1), "0"}, // a real keeps the sign of the zero below 0; what is printed has one zero
		{600, "600"},
		{600.5, "600.5"},
		{-0.25, "-0.25"},
		{0.1, "0.1"},
		{-2147483648, "-2147483648"},
		{0.000001, "0.000001"},
		{0.0000001, "1e-7"},
		{0.00000015, "1.5e-7"},
		{1e-300, "1e-300"},
		{123456789012345680000, "123456789012345680000"},
		{1e21, "1e+21"},
		{-1.5e25, "-1.5e+25"},
		{math.MaxFloat32, "3.4028234663852886e+38"},
	}
	for _, c := range cases {
		for _, kind := range []objmod.ValueType{objmod.Real, objmod.Unreal} {
			printed := objects.EvalJSON(oneValue("captain", objects.Value{Type: kind, Number: c.number}))
			if got := valueLine(t, printed); got != c.want {
				t.Errorf("%v is printed as %s, want %s", c.number, got, c.want)
			}
		}
	}
}

func TestEvalJSONWritesNullForANumberThatIsNotFinite(t *testing.T) {
	// Resolve returns no such number; JSON has no text for one, and what is printed must stay JSON.
	for _, number := range []float64{math.Inf(1), math.Inf(-1), math.NaN()} {
		printed := objects.EvalJSON(oneValue("captain", objects.Value{Type: objmod.Unreal, Number: number}))
		if got := valueLine(t, printed); got != "null" {
			t.Errorf("%v is printed as %s, want null", number, got)
		}
		if !json.Valid(printed) {
			t.Errorf("%v: what is printed is not JSON:\n%s", number, printed)
		}
	}
}

func TestEvalJSONEscapesOnlyTheQuoteTheBackslashAndControlCharacters(t *testing.T) {
	cases := []struct{ name, text, want string }{
		{"markup", "<b>Tom & Jerry</b>", `"<b>Tom & Jerry</b>"`},
		{"a quote and a backslash", `say "hi" \ bye`, `"say \"hi\" \\ bye"`},
		{"a line break and a tab", "one\ntwo\tthree\r", `"one\ntwo\tthree\r"`},
		{"a backspace and a form feed", "one\btwo\fthree", `"one\btwo\fthree"`},
		{"a control character", "a\x01b\x1fc", "\"a\x5cu0001b\x5cu001fc\""},
		{"a letter outside ASCII", "caf\xc3\xa9", "\"caf\xc3\xa9\""},
		{"a line separator", "a\xe2\x80\xa8b", "\"a\xe2\x80\xa8b\""},
		{"a delete character", "a\x7fb", "\"a\x7fb\""},
		{"an empty text", "", `""`},
	}
	for _, c := range cases {
		printed := objects.EvalJSON(oneValue(c.text, objects.Value{Type: objmod.String, Text: c.text}))
		if got := valueLine(t, printed); got != c.want {
			t.Errorf("%s: the value is printed as %s, want %s", c.name, got, c.want)
		}
		// The object's key is the same text, and is written the same way.
		if key := "    " + c.want + ": {"; !strings.Contains(string(printed), "\n"+key+"\n") {
			t.Errorf("%s: no line %s in\n%s", c.name, key, printed)
		}
	}
}

func TestEvalJSONWritesTheRawcodeOfThreeLettersWithItsNUL(t *testing.T) {
	resolved := resolve(t, `{"abilities":{"curse":{"id":"A000","base":"Acrs","properties":{"Crs":0.25}}}}`)
	want := "          \"rawcode\": \"Crs\x5cu0000\",\n"
	if printed := string(objects.EvalJSON(resolved)); !strings.Contains(printed, want) {
		t.Errorf("no line %q in\n%s", want, printed)
	}
}

func TestEvalJSONKeepsObjectsInResolvedOrderAlsoUnderKeysThatLookLikeNumbers(t *testing.T) {
	resolved := resolve(t, `{"units":{
		"b":{"id":"h000","base":"hfoo"},"10":{"id":"h001","base":"hfoo"},"2":{"id":"h002","base":"hfoo"},
		"a":{"id":"h003","base":"hfoo"}}}`)
	var keys []string
	for _, line := range strings.Split(string(objects.EvalJSON(resolved)), "\n") {
		if key, found := strings.CutSuffix(line, ": {"); found && strings.HasPrefix(key, `    "`) {
			keys = append(keys, strings.TrimSpace(key))
		}
	}
	if want := []string{`"b"`, `"10"`, `"2"`, `"a"`}; !reflect.DeepEqual(keys, want) {
		t.Errorf("the units come as %q, want %q", keys, want)
	}
}
