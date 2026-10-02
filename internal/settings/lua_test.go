package settings_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/luasrc"
	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func fixtureLua(t *testing.T) string {
	return string(testkit.Fixture(t, "map-settings-v39/war3map.lua"))
}

// patchLua applies the settings of a JSON document to source, with the fixture's map info patched the same way.
func patchLua(t *testing.T, document, source string) string {
	t.Helper()
	s := validated(t, document)
	info, err := settings.PatchMapInfo(testkit.Fixture(t, "map-settings-v39/war3map.w3i"), s, "war3map.w3i")
	if err != nil {
		t.Fatalf("PatchMapInfo(%s): %v", document, err)
	}
	patched, err := settings.PatchLua(source, s, info, "war3map.lua", "war3map.w3i")
	if err != nil {
		t.Fatalf("PatchLua(%s): %v", document, err)
	}
	return patched
}

// refusesLua checks that the settings cannot be applied to source, with an error naming the Lua file.
func refusesLua(t *testing.T, document, source string) *diag.Error {
	t.Helper()
	s := validated(t, document)
	info, err := settings.PatchMapInfo(testkit.Fixture(t, "map-settings-v39/war3map.w3i"), s, "war3map.w3i")
	if err != nil {
		t.Fatalf("PatchMapInfo(%s): %v", document, err)
	}
	_, err = settings.PatchLua(source, s, info, "map/war3map.lua", "map/war3map.w3i")
	e := asError(t, err, document)
	if e.File != "map/war3map.lua" {
		t.Errorf("%s: the error names %q: %s", document, e.File, e.Msg)
	}
	return e
}

func replaced(t *testing.T, source, old, new string) string {
	t.Helper()
	if !strings.Contains(source, old) {
		t.Fatalf("the source has no %q", old)
	}
	return strings.Replace(source, old, new, 1)
}

// teamsTail is the whole InitCustomTeams body after the editor's last call.
func teamsTail(lua string) string {
	const last = "SetPlayerTeam(Player(11), 1)\r\n"
	start := strings.Index(lua, last) + len(last)
	return lua[start : start+strings.Index(lua[start:], "end")]
}

func mainCalls(t *testing.T, lua string) []string {
	t.Helper()
	functions, err := luasrc.Functions(lua, "war3map.lua")
	if err != nil {
		t.Fatal(err)
	}
	for _, function := range functions {
		if function.Name == "main" {
			var names []string
			for _, call := range function.Calls {
				names = append(names, call.Name)
			}
			return names
		}
	}
	t.Fatal("no main")
	return nil
}

func TestPlayerAndEnvironmentEditsAgreeWithPatchedMapInfo(t *testing.T) {
	lua := patchLua(t, `{
		"info":{"name":"A \"quoted\" map\n雪"},
		"players":{"0":{"name":"Hero","controller":"computer","race":"orc","fixedStart":false,"x":256}},
		"environment":{"waterColor":[10,20,30,255],"fog":{"enabled":true,"start":100,"end":1000}}}`, fixtureLua(t))
	for _, line := range []string{
		`SetMapName("A \"quoted\" map\010雪")`,
		"SetPlayerController(Player(0), MAP_CONTROL_COMPUTER)",
		"SetPlayerRacePreference(Player(0), RACE_PREF_ORC)",
		"DefineStartLocation(0, 256, -896)",
		"SetWaterBaseColor(10, 20, 30, 255)",
		"ForcePlayerStartLocation(Player(1), 1)",
		`BlzCreateUnitWithSkin(p, FourCC("Hblm")`,
	} {
		if !strings.Contains(lua, line) {
			t.Errorf("the patched Lua lacks %s", line)
		}
	}
	if strings.Contains(lua, "ForcePlayerStartLocation(Player(0)") {
		t.Error("player 0 is still forced to its start location")
	}
	if got := settings.LuaString("\n123"); got != `"\010123"` {
		t.Errorf("LuaString = %s", got)
	}
}

func TestMissingOrDuplicateEditorCallsRefuseAnEdit(t *testing.T) {
	source := fixtureLua(t)
	name := `{"info":{"name":"Name"}}`
	refusesLua(t, name, replaced(t, source, "SetMapName(", "Other("))
	refusesLua(t, name, source+"\nfunction config() SetMapName(\"x\") end")
	refusesLua(t, name, replaced(t, source, "SetMapName(", "object.SetMapName("))
}

func TestSettingsWithoutLuaCounterpartsReturnTheSourceUnchangedWithoutReadingIt(t *testing.T) {
	source := fixtureLua(t)
	if patchLua(t, `{}`, source) != source {
		t.Error("no settings changed the Lua")
	}
	metadataOnly := `{"info":{"author":"Author","recommendedPlayers":""},"loadingScreen":{"title":"Title"},
		"forces":{"0":{"name":"Allies"}},"gameplay":{"heroMaxLevel":20}}`
	if patchLua(t, metadataOnly, source) != source {
		t.Error("settings stored only in the map info changed the Lua")
	}
	if got := patchLua(t, `{}`, "function (((unreadable"); got != "function (((unreadable" {
		t.Errorf("unreadable Lua was touched: %q", got)
	}
}

func TestAMapNameEditChangesOnlyThatCall(t *testing.T) {
	source := fixtureLua(t)
	want := replaced(t, source, `SetMapName("TRIGSTR_001")`, `SetMapName("Name")`)
	want = replaced(t, want, `SetMapDescription("TRIGSTR_003")`, `SetMapDescription("")`)
	if got := patchLua(t, `{"info":{"name":"Name","description":""}}`, source); got != want {
		t.Error("a name and description edit changed something else")
	}
}

func TestPlayerEditsReplaceInsertAndRemoveOnlyThatPlayersCalls(t *testing.T) {
	source := fixtureLua(t)
	want := replaced(t, source,
		"SetPlayerStartLocation(Player(0), 0)\r\nForcePlayerStartLocation(Player(0), 0)\r\n",
		"SetPlayerStartLocation(Player(0), 0)\r\n")
	want = replaced(t, want, "SetPlayerRacePreference(Player(0), RACE_PREF_HUMAN)",
		"SetPlayerRacePreference(Player(0), RACE_PREF_USER_SELECTABLE)")
	want = replaced(t, want, "SetPlayerRaceSelectable(Player(0), false)", "SetPlayerRaceSelectable(Player(0), true)")
	if got := patchLua(t, `{"players":{"0":{"name":"Hero","race":"selectable","fixedStart":false}}}`, source); got != want {
		t.Error("player 0's edits differ")
	}
	unforced := replaced(t, source, "ForcePlayerStartLocation(Player(1), 1)\r\n", "")
	if got := patchLua(t, `{"players":{"1":{"fixedStart":true}}}`, unforced); got != source {
		t.Error("forcing player 1's start location did not restore the editor's call")
	}
	// Player 11 is the fifth record, so its start location is 4; an existing matching call is kept.
	if got := patchLua(t, `{"players":{"11":{"fixedStart":true,"controller":"computer"}}}`, source); got != source {
		t.Error("settings equal to the map's changed the Lua")
	}
	// A name changes only war3map.w3i; an existing SetPlayerName call is left alone.
	named := replaced(t, source, "SetPlayerColor(Player(1), ConvertPlayerColor(1))",
		"SetPlayerColor(Player(1), ConvertPlayerColor(1))\r\nSetPlayerName(Player(1), \"TRIGSTR_006\")")
	want = replaced(t, named, "SetPlayerController(Player(1), MAP_CONTROL_USER)",
		"SetPlayerController(Player(1), MAP_CONTROL_RESCUABLE)")
	if got := patchLua(t, `{"players":{"1":{"name":"Tab\there ✓","controller":"rescuable"}}}`, named); got != want {
		t.Error("a name and controller edit differ")
	}
	if got := patchLua(t, `{"players":{"0":{"name":"Hero"}}}`, source); got != source {
		t.Error("a player name changed the Lua")
	}
}

func TestStartCoordinatesUseEffectiveFloat32MapInfoValues(t *testing.T) {
	lua := patchLua(t, `{"players":{"11":{"x":0.1}}}`, fixtureLua(t))
	for _, line := range []string{
		"DefineStartLocation(4, 0.10000000149011612, -896)\r\n",
		"DefineStartLocation(0, 128.0, -896.0)\r\n",
	} {
		if !strings.Contains(lua, line) {
			t.Errorf("the patched Lua lacks %q", line)
		}
	}
	fog := patchLua(t, `{"environment":{"fog":{"enabled":true,"start":0,"density":0.3,"color":[255,0,51,128]}}}`, fixtureLua(t))
	if want := "SetTerrainFogEx(0, 0, 5000, 0.30000001192092896, 1, 0, 0.2)\r\nCreateAllUnits()"; !strings.Contains(fog, want) {
		t.Errorf("the patched Lua lacks %q", want)
	}
}

func TestUnsafePlayerTeamAndEnvironmentShapesAreRefused(t *testing.T) {
	source := fixtureLua(t)
	computer0 := `{"players":{"0":{"controller":"computer"}}}`
	computer1 := `{"players":{"1":{"controller":"computer"}}}`
	allied0 := `{"forces":{"0":{"allied":true}}}`
	water := `{"environment":{"waterColor":[1,2,3,4]}}`
	units := "CreateAllUnits()\r\nInitBlizzard()"
	for _, c := range []struct{ document, old, new string }{
		{`{"info":{"name":"x"}}`, `SetMapName("TRIGSTR_001")`, `SetMapName("TRIGSTR_001", 1)`},
		{computer1, "SetPlayerStartLocation(Player(1), 1)", "SetPlayerStartLocation(Player(1), 2)"},
		{computer1, "SetPlayerStartLocation(Player(1), 1)\r\n", ""},
		{computer0, "SetPlayerController(Player(0), MAP_CONTROL_USER)", "SetPlayerController(Player(0 + 0), MAP_CONTROL_USER)"},
		{computer0, "SetPlayerController(Player(1), MAP_CONTROL_USER)", "SetPlayerController(p, MAP_CONTROL_USER)"},
		{`{"players":{"0":{"x":1}}}`, "DefineStartLocation(0,", "DefineStartLocation(zero,"},
		{`{"players":{"0":{"x":1}}}`, "DefineStartLocation(1,", "DefineStartLocation(0,"},
		{`{"players":{"0":{"fixedStart":false}}}`, "ForcePlayerStartLocation(Player(0), 0)",
			"ForcePlayerStartLocation(Player(0), 0)\r\nForcePlayerStartLocation(Player(0), 0)"},
		{`{"players":{"0":{"fixedStart":true}}}`, "ForcePlayerStartLocation(Player(0), 0)", "ForcePlayerStartLocation(Player(0), 1)"},
		{`{"players":{"0":{"race":"orc"}}}`, "SetPlayerRaceSelectable(Player(0), false)\r\n", ""},
		{`{"players":{"0":{"name":"x"}}}`, "InitCustomPlayerSlots()\r\nInitCustomTeams()", "InitCustomTeams()"},
		{allied0, "SetPlayerTeam(Player(3), 0)", "SetPlayerTeam(Player(3), 1)"},
		{`{"forces":{"1":{"allied":true}}}`, "SetPlayerTeam(Player(3), 0)", "SetPlayerTeam(Player(3), 1)"},
		{allied0, "SetPlayerTeam(Player(3), 0)\r\n", ""},
		{allied0, "SetPlayerTeam(Player(3), 0)", "SetPlayerTeam(Player(3), team)"},
		{allied0, "InitCustomTeams()\r\nInitAllyPriorities()", "InitAllyPriorities()"},
		{water, "CreateAllUnits()\r\nInitBlizzard()\r\n", ""},
		{water, units, "CreateAllUnits(1)\r\nInitBlizzard()"},
		{water, units, "SetWaterBaseColor(1, 2, 3)\r\n" + units},
		{`{"environment":{"soundEnvironment":"Cave"}}`, units, "NewSoundEnvironment(\"Second\")\r\n" + units},
		{`{"environment":{"fog":{"enabled":false}}}`, units, "ResetTerrainFog(1)\r\n" + units},
	} {
		e := refusesLua(t, c.document, replaced(t, source, c.old, c.new))
		if !strings.HasPrefix(e.Msg, "Cannot apply map settings to Lua: ") || e.Hint == "" {
			t.Errorf("%s with %q: %+v", c.document, c.new, e)
		}
	}
}

func TestForceFlagEditsAppendEffectiveStatesAfterTheEditorsCalls(t *testing.T) {
	lua := patchLua(t,
		`{"forces":{"0":{"allied":false,"alliedVictory":true,"sharedVision":false,"sharedControl":true},"1":{}}}`,
		fixtureLua(t))
	var want strings.Builder
	for id := range 4 {
		fmt.Fprintf(&want, "SetPlayerState(Player(%d), PLAYER_STATE_ALLIED_VICTORY, 1)\r\n", id)
	}
	for _, alliance := range []struct {
		native string
		value  bool
	}{
		{"SetPlayerAllianceStateAllyBJ", false},
		{"SetPlayerAllianceStateVisionBJ", false},
		{"SetPlayerAllianceStateControlBJ", true},
		{"SetPlayerAllianceStateFullControlBJ", false},
	} {
		for a := range 4 {
			for b := range 4 {
				if a != b {
					fmt.Fprintf(&want, "%s(Player(%d), Player(%d), %t)\r\n", alliance.native, a, b, alliance.value)
				}
			}
		}
	}
	if got := teamsTail(lua); got != want.String() {
		t.Errorf("the appended team states are\n%q\nwant\n%q", got, want.String())
	}
	if !strings.Contains(lua, "SetPlayerAllianceStateAllyBJ(Player(0), Player(1), true)\r\n") {
		t.Error("the editor's own alliance calls are gone")
	}
	// Force 1 has one member: only its allied-victory state (inherited false) is written.
	one := teamsTail(patchLua(t, `{"forces":{"1":{"sharedVision":true}}}`, fixtureLua(t)))
	if one != "SetPlayerState(Player(11), PLAYER_STATE_ALLIED_VICTORY, 0)\r\n" {
		t.Errorf("force 1's states are %q", one)
	}
}

func TestEnvironmentEditsReplaceOldInitializationImmediatelyBeforeTheAnchor(t *testing.T) {
	source := fixtureLua(t)
	want := replaced(t, source, "NewSoundEnvironment(\"Default\")\r\n", "")
	want = replaced(t, want, "CreateAllUnits()\r\nInitBlizzard()",
		"NewSoundEnvironment(\"Default\")\r\nResetTerrainFog()\r\nCreateAllUnits()\r\nInitBlizzard()")
	if got := patchLua(t, `{"environment":{"soundEnvironment":"","fog":{"enabled":false}}}`, source); got != want {
		t.Error("the environment edit differs")
	}
	custom := "function main()\n  SetTerrainFogEx(0, 1, 2, 0.5, 1, 1, 1) ResetTerrainFog()\n  NewSoundEnvironment(\"Old\")\n  InitBlizzard()\nend\n"
	got := patchLua(t, `{"environment":{"soundEnvironment":"Cave","fog":{"enabled":true}}}`, custom)
	wantCustom := "function main()\n  ; ;\n  NewSoundEnvironment(\"Cave\")\n  SetTerrainFogEx(0, 3000, 5000, 0.5, 0, 0, 0)\n  InitBlizzard()\nend\n"
	if got != wantCustom {
		t.Errorf("patched = %q, want %q", got, wantCustom)
	}
}

func TestRemovingACallNeverJoinsTheStatementsAroundIt(t *testing.T) {
	for _, c := range []struct{ source, want string }{
		{"function main()\nx = b\nResetTerrainFog();(f)()\nInitBlizzard()\nend", "function main()\nx = b\n;(f)()\n"},
		{"function main()\nx = b\nResetTerrainFog();\n(f)()\nInitBlizzard()\nend", "function main()\nx = b\n;\n(f)()\n"},
		{"function main()\nx = b\nResetTerrainFog() -- c\ny()\nInitBlizzard()\nend", "function main()\nx = b\n; -- c\ny()\n"},
	} {
		lua := patchLua(t, `{"environment":{"fog":{"enabled":false}}}`, c.source)
		if want := c.want + "ResetTerrainFog()\nInitBlizzard()\nend"; lua != want {
			t.Errorf("patched = %q, want %q", lua, want)
		}
		mainCalls(t, lua)
	}
}

func TestLuaStringsEscapeQuotesBackslashesAndControlCharactersAsDecimalEscapes(t *testing.T) {
	for input, want := range map[string]string{
		"Back\\slash \"q\" \t\x7f ✓ 雪 😀": `"Back\\slash \"q\" \009\127 ✓ 雪 😀"`,
		"\r\n1\x001":                     `"\013\0101\0001"`,
		"":                               `""`,
	} {
		if got := settings.LuaString(input); got != want {
			t.Errorf("LuaString(%q) = %s, want %s", input, got, want)
		}
	}
}

func TestUnreadablePatchedMapInfoIsReportedAgainstTheMapInfoFile(t *testing.T) {
	s := validated(t, `{"info":{"name":"X"}}`)
	_, err := settings.PatchLua(fixtureLua(t), s, []byte{1, 2}, "maps/m/war3map.lua", "maps/m/war3map.w3i")
	if e := asError(t, err, "a cut-off map info"); e.File != "maps/m/war3map.w3i" {
		t.Errorf("the error names %q", e.File)
	}
}

const minimapCall = `BlzChangeMinimapTerrainTex("war3mapMinimap.blp")`

func TestTheMinimapCallBecomesTheLastStatementOfMainOnALineOfItsOwn(t *testing.T) {
	lua := fixtureLua(t)
	patched, err := settings.PatchMinimapLua(lua, "war3map.lua")
	if err != nil {
		t.Fatal(err)
	}
	if len(patched) != len(lua)+len(minimapCall)+2 {
		t.Errorf("the patch added %d bytes", len(patched)-len(lua))
	}
	if !strings.Contains(patched, "RunInitializationTriggers()\r\n"+minimapCall+"\r\nend\r\n") {
		t.Error("the call is not the last line of main")
	}
	calls := mainCalls(t, patched)
	if n := len(calls); calls[n-1] != "BlzChangeMinimapTerrainTex" || calls[n-2] != "RunInitializationTriggers" {
		t.Errorf("main calls %q", calls)
	}
	// The other functions are untouched: the call is in main alone.
	if strings.Count(patched, minimapCall) != 1 {
		t.Error("the call was added more than once")
	}
}

func TestTheMinimapCallKeepsTheScriptsLineEndingAndTheIndentationOfMainsEnd(t *testing.T) {
	for source, want := range map[string]string{
		"function main()\r\n  InitBlizzard()\r\n  end\r\n": "function main()\r\n  InitBlizzard()\r\n  " + minimapCall + "\r\n  end\r\n",
		"function main()\nend\n":                           "function main()\n" + minimapCall + "\nend\n",
		"function main() InitBlizzard() end":               "function main() InitBlizzard() " + minimapCall + " end",
	} {
		if got, err := settings.PatchMinimapLua(source, "war3map.lua"); err != nil || got != want {
			t.Errorf("PatchMinimapLua(%q) = %q, %v", source, got, err)
		}
	}
}

func TestTheMinimapCallNeedsExactlyOneGlobalMain(t *testing.T) {
	for source, count := range map[string]int{
		"function config()\nend\n":                     0,
		"function main()\nend\nfunction main()\nend\n": 2,
	} {
		_, err := settings.PatchMinimapLua(source, "map/war3map.lua")
		e := asError(t, err, source)
		want := fmt.Sprintf("expected exactly one global function main(), found %d.", count)
		if !strings.Contains(e.Msg, want) || e.File != "map/war3map.lua" || e.Hint == "" {
			t.Errorf("error = %+v", e)
		}
	}
}

func TestTheMinimapCallGoesInBesideTheOtherLuaSettings(t *testing.T) {
	lua := patchLua(t, `{"info":{"name":"Both"},"environment":{"soundEnvironment":"Dungeon"}}`, fixtureLua(t))
	patched, err := settings.PatchMinimapLua(lua, "war3map.lua")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(patched, `SetMapName("Both")`) || !strings.Contains(patched, `NewSoundEnvironment("Dungeon")`) {
		t.Error("the other settings are gone")
	}
	if calls := mainCalls(t, patched); calls[len(calls)-1] != "BlzChangeMinimapTerrainTex" {
		t.Errorf("main calls %q", calls)
	}
}
