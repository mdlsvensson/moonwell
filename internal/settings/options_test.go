package settings_test

import (
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/settings"
)

func TestSettingsInheritByDefaultAndRetainExplicitFalseZeroAndEmptyText(t *testing.T) {
	inherit := validated(t, `{"info":{"name":null},"players":{"23":{"name":null}},"environment":{"fog":{}}}`)
	if inherit.Has() {
		t.Error("settings of nulls and empty objects count as set")
	}
	s := validated(t, `{"info":{"name":""},"players":{"0":{"fixedStart":false,"x":0}},"gameplay":{"foodLimit":0}}`)
	if s.Info.Name == nil || *s.Info.Name != "" || s.Info.Author != nil {
		t.Errorf("info = %+v", s.Info)
	}
	player, ok := s.Players.Get("0")
	if !ok || player.FixedStart == nil || *player.FixedStart || player.X == nil || *player.X != 0 || player.Y != nil ||
		player.Name != nil {
		t.Errorf("player 0 = %+v", player)
	}
	if s.Gameplay.FoodLimit == nil || *s.Gameplay.FoodLimit != 0 || !s.HasExtended() || !s.Has() {
		t.Errorf("gameplay = %+v", s.Gameplay)
	}
}

func TestInvalidSettingsIdentifyTheManifestAndField(t *testing.T) {
	for _, document := range []string{
		`null`,
		`[]`,
		`{"typo":null}`,
		`{"info":{"typo":null}}`,
		`{"info":{"name":"bad\u0000text"}}`,
		`{"loadingScreen":{"background":-2}}`,
		`{"loadingScreen":{"background":1.5}}`,
		`{"players":{"00":{}}}`,
		`{"players":{"24":{}}}`,
		`{"players":{"0":{"race":"elf"}}}`,
		`{"players":{"0":{"x":1e999}}}`,
		`{"environment":{"waterColor":[1,2,3]}}`,
		`{"environment":{"fog":{"density":1.01}}}`,
		`{"gameplay":{"heroMaxLevel":0}}`,
		`{"gameplay":{"foodLimit":301}}`,
		`{"gameplayConstants":{"Misc":{},"misc":{}}}`,
		`{"gameInterface":{"Frame":{"X":"a","x":"b"}}}`,
		`{"gameplayConstants":{"Misc":{"X":"x\nY=z"}}}`,
		`{"players":{"0":{"constructor":null}}}`,
	} {
		_, err := settings.Validate(tree(t, document), "moonwell.local.pkl")
		if e := asError(t, err, document); e.File != "moonwell.local.pkl" || e.Hint == "" {
			t.Errorf("%s: %+v", document, e)
		}
	}
	raw := validated(t, `{"gameInterface":{"constructor":{"constructor":"ok"}}}`)
	section, _ := raw.GameInterface.Get("constructor")
	if value, _ := section.Get("constructor"); value != "ok" {
		t.Errorf("a section named constructor = %q", value)
	}
}

func TestSettingsEnforceNumericBoundariesAndAcceptedNames(t *testing.T) {
	for _, document := range []string{
		`{"loadingScreen":{"background":-1}}`,
		`{"loadingScreen":{"background":2147483647}}`,
		`{"gameplay":{"heroMaxLevel":1,"foodLimit":0}}`,
		`{"gameplay":{"heroMaxLevel":10000,"foodLimit":300}}`,
		`{"players":{"23":{"x":-10000000,"y":10000000}}}`,
		`{"environment":{"fog":{"style":0,"density":0,"start":-10000000,"end":10000000}}}`,
		`{"environment":{"fog":{"style":2,"density":1},"waterColor":[0,255,0,255]}}`,
	} {
		validated(t, document)
	}
	for _, document := range []string{
		`{"loadingScreen":{"background":2147483648}}`,
		`{"gameplay":{"heroMaxLevel":10001}}`,
		`{"gameplay":{"foodLimit":-1}}`,
		`{"players":{"0":{"x":-10000000.01}}}`,
		`{"players":{"0":{"y":10000000.01}}}`,
		`{"environment":{"fog":{"style":3}}}`,
		`{"environment":{"fog":{"density":-0.01}}}`,
		`{"environment":{"fog":{"start":10000000.01}}}`,
		`{"environment":{"waterColor":[1,2,3]}}`,
		`{"environment":{"waterColor":[1,2,3,4.5]}}`,
		`{"environment":{"fog":{"color":[0,0,0,256]}}}`,
	} {
		_, err := settings.Validate(tree(t, document), "")
		asError(t, err, document)
	}
	for _, controller := range []string{"user", "computer", "neutral", "rescuable"} {
		player, _ := validated(t, `{"players":{"0":{"controller":"`+controller+`"}}}`).Players.Get("0")
		if player.Controller == nil || *player.Controller != controller {
			t.Errorf("controller %s = %+v", controller, player)
		}
	}
	for _, race := range []string{"selectable", "human", "orc", "undead", "nightelf"} {
		player, _ := validated(t, `{"players":{"0":{"race":"`+race+`"}}}`).Players.Get("0")
		if player.Race == nil || *player.Race != race {
			t.Errorf("race %s = %+v", race, player)
		}
	}
	flags := map[string]func(settings.Force) *bool{
		"allied":                func(f settings.Force) *bool { return f.Allied },
		"alliedVictory":         func(f settings.Force) *bool { return f.AlliedVictory },
		"sharedVision":          func(f settings.Force) *bool { return f.SharedVision },
		"sharedControl":         func(f settings.Force) *bool { return f.SharedControl },
		"sharedAdvancedControl": func(f settings.Force) *bool { return f.SharedAdvancedControl },
	}
	for key, get := range flags {
		force, ok := validated(t, `{"forces":{"0":{"`+key+`":false}}}`).Forces.Get("0")
		if value := get(force); !ok || value == nil || *value {
			t.Errorf("%s = %+v", key, force)
		}
		_, err := settings.Validate(tree(t, `{"forces":{"0":{"`+key+`":0}}}`), "")
		asError(t, err, key+" as a number")
	}
}

func TestSettingsKeepNestedValuesAndRawKeys(t *testing.T) {
	s := validated(t, `{"environment":{"waterColor":[1,2,3,4]},"gameplayConstants":{"Misc":{"FoodCeiling":"0"}}}`)
	if s.Environment.WaterColor == nil || *s.Environment.WaterColor != [4]uint8{1, 2, 3, 4} {
		t.Errorf("water colour = %v", s.Environment.WaterColor)
	}
	misc, _ := s.GameplayConstants.Get("Misc")
	if value, _ := misc.Get("FoodCeiling"); value != "0" {
		t.Errorf("FoodCeiling = %q", value)
	}
}

func TestSettingsValidationErrorsNameTheSettingsPathOnce(t *testing.T) {
	for _, c := range []struct{ document, message string }{
		{`[]`, "settings must be an object."},
		{`{"other":{}}`, "Unknown map setting: settings.other"},
		{`{"info":[]}`, "settings.info must be an object."},
		{`{"info":{"title":"x"}}`, "Unknown map setting: settings.info.title"},
		{`{"info":{"name":1}}`, "Invalid map setting: settings.info.name"},
		{`{"players":{"24":{}}}`, "settings.players keys must be IDs from 0 to 23: 24"},
		{`{"players":{"7":{"x":"1"}}}`, `Invalid map setting: settings.players["7"].x`},
		{`{"forces":{"0":{"team":1}}}`, `Unknown map setting: settings.forces["0"].team`},
		{`{"environment":{"fog":{"density":2}}}`, "Invalid map setting: settings.environment.fog.density"},
		{`{"gameplayConstants":{"Misc":{},"misc":{}}}`, "Invalid or duplicate settings.gameplayConstants section: misc"},
		{`{"gameInterface":{"Frame":{"X":"a","x":"b"}}}`, `Invalid or duplicate settings.gameInterface["Frame"] key: x`},
		{`{"gameplayConstants":{"Misc":{"X":"a\nb"}}}`, `settings.gameplayConstants["Misc"]["X"] must be a single-line string.`},
		{`{"gameInterface":{"Frame":[]}}`, `settings.gameInterface["Frame"] must be an object.`},
		// The first problem in document order is the one reported.
		{`{"info":{"name":1,"typo":2},"other":3}`, "Unknown map setting: settings.other"},
		{`{"info":{"name":1,"typo":2}}`, "Invalid map setting: settings.info.name"},
		{`{"info":{"typo":2,"name":1}}`, "Unknown map setting: settings.info.typo"},
		{`{"info":{"name":1},"players":{"0":{"x":"1"}}}`, `Invalid map setting: settings.players["0"].x`},
	} {
		_, err := settings.Validate(tree(t, c.document), "moonwell.local.pkl")
		if e := asError(t, err, c.document); e.Msg != c.message || strings.Contains(e.Msg, "settings.settings") {
			t.Errorf("%s: %q, want %q", c.document, e.Msg, c.message)
		}
	}
}

func TestWholeNumberCoordinatesAndFogValuesAreAcceptedAsNumbers(t *testing.T) {
	s := validated(t, `{"players":{"0":{"x":256,"y":-896}},"environment":{"fog":{"start":100,"end":1000,"density":1}}}`)
	player, _ := s.Players.Get("0")
	if *player.X != 256 || *player.Y != -896 {
		t.Errorf("player = %v, %v", *player.X, *player.Y)
	}
	fog := s.Environment.Fog
	if fog == nil || *fog.Start != 100 || *fog.End != 1000 || *fog.Density != 1 || fog.Enabled != nil || fog.Style != nil {
		t.Errorf("fog = %+v", fog)
	}
}

func TestThePreviewIsKeptApartFromTheFieldsStoredInTheMapInfo(t *testing.T) {
	s := validated(t, `{"info":{"name":"N","preview":"art/preview.tga"}}`)
	if *s.Info.Name != "N" || s.Preview == nil || *s.Preview != "art/preview.tga" {
		t.Errorf("settings = %+v", s)
	}
	only := validated(t, `{"info":{"preview":"p.blp"}}`)
	if !only.Has() || only.HasExtended() {
		t.Error("a preview alone is a setting, and not an extended one")
	}
	none := validated(t, `{"info":{"preview":null}}`)
	if none.Preview != nil || none.Has() {
		t.Error("a null preview counts as set")
	}
	for _, bad := range []string{`""`, `5`, `"a\u0000b"`} {
		_, err := settings.Validate(tree(t, `{"info":{"preview":`+bad+`}}`), "moonwell.pkl")
		e := asError(t, err, bad)
		if e.Msg != "Invalid map setting: settings.info.preview" || e.File != "moonwell.pkl" {
			t.Errorf("preview %s: %+v", bad, e)
		}
	}
}

func TestPlayersAndForcesComeInJavaScriptsKeyOrder(t *testing.T) {
	s := validated(t, `{"players":{"10":{"name":"k"},"2":{"name":"c"},"0":{"name":"a"}}}`)
	if got := strings.Join(s.Players.Keys(), ","); got != "0,2,10" {
		t.Errorf("players = %s", got)
	}
}
