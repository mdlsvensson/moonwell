package settings

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/war3/txt"
)

// written is raw sections as JSON, in their order.
func written(t *testing.T, raw manifest.Ordered[manifest.Ordered[string]]) string {
	t.Helper()
	text, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

func section(name string, fields ...txt.Field) txt.Section {
	return txt.Section{Name: name, Fields: fields}
}

func TestTypedGameplayConstantsMergeIntoTheRawOnesWithoutRegardToLetterCase(t *testing.T) {
	tests := []struct {
		name, document string
		want           []txt.Section
	}{
		{"a raw value equal to the typed one, spelled in lower case",
			`{"gameplay":{"foodLimit":200},"gameplayConstants":{"misc":{"foodceiling":"200"}}}`,
			[]txt.Section{section("misc", txt.Field{Key: "foodceiling", Value: "200"})}},
		{"typed values without a Misc section get one after the raw sections",
			`{"gameplay":{"heroMaxLevel":25,"foodLimit":150},"gameplayConstants":{"Other":{"A":"1"}}}`,
			[]txt.Section{
				section("Other", txt.Field{Key: "A", Value: "1"}),
				section("Misc", txt.Field{Key: "MaxHeroLevel", Value: "25"}, txt.Field{Key: "FoodCeiling", Value: "150"}),
			}},
		{"a typed value joins the keys of the raw section, whatever its spelling",
			`{"gameplay":{"heroMaxLevel":25},"gameplayConstants":{"MISC":{"A":"1"},"Other":{}}}`,
			[]txt.Section{
				section("MISC", txt.Field{Key: "A", Value: "1"}, txt.Field{Key: "MaxHeroLevel", Value: "25"}),
				section("Other"),
			}},
		{"a zero is a value", `{"gameplay":{"foodLimit":0}}`,
			[]txt.Section{section("Misc", txt.Field{Key: "FoodCeiling", Value: "0"})}},
		{"raw sections alone, in the order written", `{"gameplayConstants":{"B":{"z":"1","a":""},"A":{"k":"v"}}}`,
			[]txt.Section{
				section("B", txt.Field{Key: "z", Value: "1"}, txt.Field{Key: "a", Value: ""}),
				section("A", txt.Field{Key: "k", Value: "v"}),
			}},
		{"nothing set", `{}`, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := settingsOf(t, tt.document)
			before := written(t, s.GameplayConstants)
			merged, _, err := textSections(s, manifestName)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(merged, tt.want) {
				t.Errorf("merged = %+v\nwant     %+v", merged, tt.want)
			}
			// The merge is a copy: neither making it nor changing it changes the settings.
			for i := range merged {
				for j := range merged[i].Fields {
					merged[i].Fields[j].Value = "changed"
				}
				merged[i].Fields = append(merged[i].Fields, txt.Field{Key: "Added", Value: "x"})
			}
			if after := written(t, s.GameplayConstants); after != before {
				t.Errorf("the settings were changed: %s, were %s", after, before)
			}
		})
	}
}

func TestATypedGameplayConstantThatDisagreesWithARawOneIsRefusedByTheManifest(t *testing.T) {
	tests := []struct{ name, document, constant, setting string }{
		{"the food limit, by its text and not its number",
			`{"gameplay":{"foodLimit":200},"gameplayConstants":{"misc":{"foodceiling":"0200"}}}`,
			"FoodCeiling", "settings.gameplay.foodLimit"},
		{"the hero level", `{"gameplay":{"heroMaxLevel":20},"gameplayConstants":{"MISC":{"maxHeroLevel":"10"}}}`,
			"MaxHeroLevel", "settings.gameplay.heroMaxLevel"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := settingsOf(t, tt.document)
			before := written(t, s.GameplayConstants)
			_, _, err := textSections(s, manifestName)
			failure := asError(t, err, tt.document)
			if failure.File != manifestName || !strings.Contains(failure.Msg, "Conflicting typed and raw gameplay constant: "+tt.constant) ||
				!strings.Contains(failure.Hint, tt.setting) {
				t.Errorf("error = %+v", failure)
			}
			if after := written(t, s.GameplayConstants); after != before {
				t.Errorf("the settings were changed: %s, were %s", after, before)
			}
		})
	}
}

func TestNamesThatDifferOnlyInLetterCaseAreRefusedByTheManifest(t *testing.T) {
	tests := []struct{ name, document, words string }{
		{"two sections of the gameplay constants", `{"gameplayConstants":{"Misc":{},"misc":{}}}`,
			"duplicate settings.gameplayConstants section: misc"},
		{"two keys of a section of the game interface", `{"gameInterface":{"Frame":{"X":"a","x":"b"}}}`,
			`duplicate settings.gameInterface["Frame"] key: x`},
		{"two sections of the game interface", `{"gameInterface":{"A":{"k":"v"},"B":{},"a":{}}}`,
			"duplicate settings.gameInterface section: a"},
		{"two keys of a section of the gameplay constants", `{"gameplayConstants":{"Misc":{"Key":"1","Other":"2","KEY":"3"}}}`,
			`duplicate settings.gameplayConstants["Misc"] key: KEY`},
		{"a key before the section after it", `{"gameInterface":{"A":{"k":"1","K":"2"},"a":{}}}`,
			`duplicate settings.gameInterface["A"] key: K`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := settingsOf(t, tt.document)
			_, constantsErr := sections(s.GameplayConstants, "settings.gameplayConstants", manifestName)
			_, interfaceErr := sections(s.GameInterface, "settings.gameInterface", manifestName)
			if (constantsErr == nil) == (interfaceErr == nil) {
				t.Fatalf("the constants give %v and the interface %v, want one refusal", constantsErr, interfaceErr)
			}
			err := constantsErr
			if err == nil {
				err = interfaceErr
			}
			failure := asError(t, err, tt.document)
			if failure.File != manifestName || failure.Hint == "" || !strings.Contains(failure.Msg, tt.words) {
				t.Errorf("error = %+v, want %q", failure, tt.words)
			}
		})
	}
}

func TestTwoSpellingsOfANameAreRefusedBeforeATypedConstantIsMergedAndTheConstantsBeforeTheInterface(t *testing.T) {
	tests := []struct{ name, document, words string }{
		{"two spellings of Misc, and a typed constant against a raw one",
			`{"gameplay":{"foodLimit":1},"gameplayConstants":{"Misc":{"FoodCeiling":"2"},"MISC":{}}}`,
			"settings.gameplayConstants section: MISC"},
		{"two spellings in the interface, and a typed constant against a raw one",
			`{"gameplay":{"foodLimit":1},"gameplayConstants":{"Misc":{"FoodCeiling":"2"}},"gameInterface":{"A":{},"a":{}}}`,
			"settings.gameInterface section: a"},
		{"two spellings in both blocks, the interface written first",
			`{"gameInterface":{"A":{},"a":{}},"gameplayConstants":{"Misc":{"Key":"1","KEY":"2"}}}`,
			`settings.gameplayConstants["Misc"] key: KEY`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			misc, skin, err := textSections(settingsOf(t, tt.document), manifestName)
			failure := asError(t, err, tt.document)
			if !strings.Contains(failure.Msg, tt.words) || failure.File != manifestName || misc != nil || skin != nil {
				t.Errorf("error = %+v, want %q", failure, tt.words)
			}
		})
	}
}

func TestTheSectionsOfBothTextFilesComeTogether(t *testing.T) {
	s := settingsOf(t, `{"gameplay":{"foodLimit":7},"gameInterface":{"A":{"B":"c"}},"gameplayConstants":{"Other":{}}}`)
	misc, skin, err := textSections(s, manifestName)
	if err != nil {
		t.Fatal(err)
	}
	wantMisc := []txt.Section{section("Other"), section("Misc", txt.Field{Key: "FoodCeiling", Value: "7"})}
	wantSkin := []txt.Section{section("A", txt.Field{Key: "B", Value: "c"})}
	if !reflect.DeepEqual(misc, wantMisc) || !reflect.DeepEqual(skin, wantSkin) {
		t.Errorf("misc = %+v, skin = %+v", misc, skin)
	}
}

func TestSectionsKeepTheOrderWrittenAndASectionWithoutKeys(t *testing.T) {
	s := settingsOf(t, `{"gameInterface":{"B":{"z":"1","a":"2"},"A":{},"C":{"k":""}}}`)
	got, err := sections(s.GameInterface, "settings.gameInterface", manifestName)
	if err != nil {
		t.Fatal(err)
	}
	want := []txt.Section{
		section("B", txt.Field{Key: "z", Value: "1"}, txt.Field{Key: "a", Value: "2"}),
		section("A"),
		section("C", txt.Field{Key: "k", Value: ""}),
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("sections = %+v\nwant       %+v", got, want)
	}
}
