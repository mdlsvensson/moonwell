package settings

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

const luaFile = "maps/map.w3x/war3map.lua"

const minimapCall = `BlzChangeMinimapTerrainTex("war3mapMinimap.blp")`

func fixtureLua(t testing.TB) string {
	t.Helper()
	return string(testkit.Fixture(t, "map-settings-v39/war3map.lua"))
}

func fixtureInfo(t testing.TB) []byte {
	t.Helper()
	return testkit.Fixture(t, "map-settings-v39/war3map.w3i")
}

func afterInfo(source string, s manifest.Settings, patchedInfo []byte) (string, error) {
	return patchLuaFromInfo(source, s, patchedInfo, luaFile, infoFile)
}

func withSettings(t testing.TB, document, source string) (string, error) {
	t.Helper()
	s := settingsOf(t, document)
	info, err := patchInfo(fixtureInfo(t), s, infoFile)
	if err != nil {
		t.Fatalf("patchInfo(%s): %v", document, err)
	}
	return afterInfo(source, s, info)
}

func inLine(t testing.TB, document, source string) string {
	t.Helper()
	result, err := withSettings(t, document, source)
	if err != nil {
		t.Fatalf("patchLua(%s): %v", document, err)
	}
	return result
}

func refusedLua(t testing.TB, document, source string) *diag.Error {
	t.Helper()
	_, err := withSettings(t, document, source)
	diagErr := asError(t, err, document)
	if diagErr.File != luaFile || diagErr.Hint == "" || !strings.HasPrefix(diagErr.Msg, "Cannot apply map settings to Lua: ") {
		t.Errorf("%s: the error names %q, hints %q and says %q", document, diagErr.File, diagErr.Hint, diagErr.Msg)
	}
	return diagErr
}

func swapped(t testing.TB, source, old, new string) string {
	t.Helper()
	if !strings.Contains(source, old) {
		t.Fatalf("the source has no %q", old)
	}
	return strings.Replace(source, old, new, 1)
}

func teamsTail(script string) string {
	const last = "SetPlayerTeam(Player(11), 1)\r\n"
	start := strings.Index(script, last) + len(last)
	return script[start : start+strings.Index(script[start:], "end")]
}

func mainCalls(t testing.TB, script string) []string {
	t.Helper()
	functions, err := lua.ParseFunctions(script, luaFile)
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
	script := inLine(t, `{
		"info":{"name":"A \"quoted\" map\n`+"\xe9\x9b\xaa"+`"},
		"players":{"0":{"name":"Hero","controller":"computer","race":"orc","fixedStart":false,"x":256}},
		"environment":{"waterColor":[10,20,30,255],"fog":{"enabled":true,"start":100,"end":1000}}}`, fixtureLua(t))
	for _, line := range []string{
		`SetMapName("A \"quoted\" map\010` + "\xe9\x9b\xaa" + `")`,
		"SetPlayerController(Player(0), MAP_CONTROL_COMPUTER)",
		"SetPlayerRacePreference(Player(0), RACE_PREF_ORC)",
		"DefineStartLocation(0, 256, -896)",
		"SetWaterBaseColor(10, 20, 30, 255)",
		"ForcePlayerStartLocation(Player(1), 1)",
		`BlzCreateUnitWithSkin(p, FourCC("Hblm")`,
	} {
		if !strings.Contains(script, line) {
			t.Errorf("the patched Lua lacks %s", line)
		}
	}
	if strings.Contains(script, "ForcePlayerStartLocation(Player(0)") {
		t.Error("player 0 is still forced to its start location")
	}
}

func TestMissingOrDuplicateEditorCallsRefuseAnEdit(t *testing.T) {
	source := fixtureLua(t)
	name := `{"info":{"name":"Name"}}`
	for _, c := range []struct{ source, words string }{
		{swapped(t, source, "SetMapName(", "Other("), "exactly one direct SetMapName in config() call, found 0"},
		{source + "\nfunction config() SetMapName(\"x\") end", "exactly one global function config(), found 2"},
		{swapped(t, source, "SetMapName(", "object.SetMapName("), "exactly one direct SetMapName in config() call, found 0"},
	} {
		if diagErr := refusedLua(t, name, c.source); !strings.Contains(diagErr.Msg, c.words) {
			t.Errorf("the error says %q, want %q in it", diagErr.Msg, c.words)
		}
	}
}

func TestSettingsWithoutLuaCounterpartsReturnTheSourceUnchangedWithoutReadingIt(t *testing.T) {
	source := fixtureLua(t)
	if inLine(t, `{}`, source) != source {
		t.Error("no settings changed the Lua")
	}
	metadataOnly := `{"info":{"author":"Author","recommendedPlayers":""},"loadingScreen":{"title":"Title"},
		"forces":{"0":{"name":"Allies"}},"gameplay":{"heroMaxLevel":20}}`
	if inLine(t, metadataOnly, source) != source {
		t.Error("settings stored only in the map info changed the Lua")
	}
	if got := inLine(t, `{}`, "function (((unreadable"); got != "function (((unreadable" {
		t.Errorf("unreadable Lua was touched: %q", got)
	}
	if got, err := afterInfo("function (((unreadable", settingsOf(t, metadataOnly), nil); err != nil || got != "function (((unreadable" {
		t.Errorf("patchLuaAfter without a map info = %q, %v", got, err)
	}
}

func TestSetsLuaIsTrueForTheSettingsWithACounterpartInTheScript(t *testing.T) {
	for document, want := range map[string]bool{
		`{}`: false,
		`{"info":{"author":"A","recommendedPlayers":"2","preview":"p.tga"}}`:                                  false,
		`{"loadingScreen":{"background":1,"model":"m","text":"t","title":"t","subtitle":"s"}}`:                false,
		`{"forces":{"0":{"name":"Allies"},"1":{}}}`:                                                           false,
		`{"players":{"0":{},"5":{"name":null}}}`:                                                              false,
		`{"environment":{"fog":{}}}`:                                                                          false,
		`{"gameplay":{"heroMaxLevel":20},"gameplayConstants":{"A":{"B":"c"}},"gameInterface":{"A":{}}}`:       false,
		`{"info":{"name":""}}`:                                                                                true,
		`{"info":{"description":"D"}}`:                                                                        true,
		`{"players":{"0":{"name":"Hero"}}}`:                                                                   true,
		`{"players":{"3":{},"0":{"fixedStart":false}}}`:                                                       true,
		`{"forces":{"0":{"name":"Allies"},"1":{"allied":false}}}`:                                             true,
		`{"forces":{"0":{"sharedAdvancedControl":true}}}`:                                                     true,
		`{"environment":{"soundEnvironment":""}}`:                                                             true,
		`{"environment":{"waterColor":[0,0,0,0]}}`:                                                            true,
		`{"environment":{"fog":{"density":0}}}`:                                                               true,
		`{"info":{"author":"A"},"loadingScreen":{"title":"T"},"environment":{"fog":{"enabled":false}}}`:       true,
		`{"info":{"name":null,"description":null},"players":{"0":{"x":null}},"forces":{"0":{"allied":null}}}`: false,
	} {
		if got := changesLua(settingsOf(t, document)); got != want {
			t.Errorf("setsLua(%s) = %v, want %v", document, got, want)
		}
	}
}

func TestAMapNameEditChangesOnlyThatCall(t *testing.T) {
	source := fixtureLua(t)
	want := swapped(t, source, `SetMapName("TRIGSTR_001")`, `SetMapName("Name")`)
	want = swapped(t, want, `SetMapDescription("TRIGSTR_003")`, `SetMapDescription("")`)
	if got := inLine(t, `{"info":{"name":"Name","description":""}}`, source); got != want {
		t.Error("a name and description edit changed something else")
	}
}

func TestPlayerEditsReplaceInsertAndRemoveOnlyThatPlayersCalls(t *testing.T) {
	source := fixtureLua(t)
	want := swapped(t, source,
		"SetPlayerStartLocation(Player(0), 0)\r\nForcePlayerStartLocation(Player(0), 0)\r\n",
		"SetPlayerStartLocation(Player(0), 0)\r\n")
	want = swapped(t, want, "SetPlayerRacePreference(Player(0), RACE_PREF_HUMAN)",
		"SetPlayerRacePreference(Player(0), RACE_PREF_USER_SELECTABLE)")
	want = swapped(t, want, "SetPlayerRaceSelectable(Player(0), false)", "SetPlayerRaceSelectable(Player(0), true)")
	if got := inLine(t, `{"players":{"0":{"name":"Hero","race":"selectable","fixedStart":false}}}`, source); got != want {
		t.Error("player 0's edits differ")
	}
	unforced := swapped(t, source, "ForcePlayerStartLocation(Player(1), 1)\r\n", "")
	if got := inLine(t, `{"players":{"1":{"fixedStart":true}}}`, unforced); got != source {
		t.Error("forcing player 1's start location did not restore the editor's call")
	}
	if got := inLine(t, `{"players":{"11":{"fixedStart":true,"controller":"computer"}}}`, source); got != source {
		t.Error("settings equal to the map's changed the Lua")
	}
	named := swapped(t, source, "SetPlayerColor(Player(1), ConvertPlayerColor(1))",
		"SetPlayerColor(Player(1), ConvertPlayerColor(1))\r\nSetPlayerName(Player(1), \"TRIGSTR_006\")")
	want = swapped(t, named, "SetPlayerController(Player(1), MAP_CONTROL_USER)",
		"SetPlayerController(Player(1), MAP_CONTROL_RESCUABLE)")
	document := `{"players":{"1":{"name":"Tab\there ` + "\xe2\x9c\x93" + `","controller":"rescuable"}}}`
	if got := inLine(t, document, named); got != want {
		t.Error("a name and controller edit differ")
	}
	if got := inLine(t, `{"players":{"0":{"name":"Hero"}}}`, source); got != source {
		t.Error("a player name changed the Lua")
	}
}

func TestAnAddedCallStandsAndEndsAsTheCallItFollows(t *testing.T) {
	notHeld := swapped(t, fixtureLua(t), "ForcePlayerStartLocation(Player(1), 1)\r\n", "")
	oneLine := strings.ReplaceAll(swapped(t, notHeld, "--\r\n", ""), "\r\n", " ")
	for _, c := range []struct{ source, want string }{
		{strings.ReplaceAll(notHeld, "\r\n", "\n"),
			"\nSetPlayerStartLocation(Player(1), 1)\nForcePlayerStartLocation(Player(1), 1)\nSetPlayerColor"},
		{strings.ReplaceAll(notHeld, ")\r\n", ");\r\n"),
			"\r\nSetPlayerStartLocation(Player(1), 1);\r\nForcePlayerStartLocation(Player(1), 1);\r\nSetPlayerColor"},
		{strings.ReplaceAll(notHeld, "\r\n", "\r\n\t  "),
			"\r\n\t  SetPlayerStartLocation(Player(1), 1)\r\n\t  ForcePlayerStartLocation(Player(1), 1)\r\n\t  SetPlayerColor"},
		{oneLine, " SetPlayerStartLocation(Player(1), 1) ForcePlayerStartLocation(Player(1), 1) SetPlayerColor"},
		{strings.ReplaceAll(oneLine, ") ", "); "),
			" SetPlayerStartLocation(Player(1), 1); ForcePlayerStartLocation(Player(1), 1); SetPlayerColor"},
	} {
		if script := inLine(t, `{"players":{"1":{"fixedStart":true}}}`, c.source); !strings.Contains(script, c.want) {
			t.Errorf("the patched Lua lacks %q", c.want)
		}
	}
}

func TestStartCoordinatesUseEffectiveFloat32MapInfoValues(t *testing.T) {
	script := inLine(t, `{"players":{"11":{"x":0.1}}}`, fixtureLua(t))
	for _, line := range []string{
		"DefineStartLocation(4, 0.10000000149011612, -896)\r\n",
		"DefineStartLocation(0, 128.0, -896.0)\r\n",
	} {
		if !strings.Contains(script, line) {
			t.Errorf("the patched Lua lacks %q", line)
		}
	}
	fog := inLine(t, `{"environment":{"fog":{"enabled":true,"start":0,"density":0.3,"color":[255,0,51,128]}}}`, fixtureLua(t))
	if want := "SetTerrainFogEx(0, 0, 5000, 0.30000001192092896, 1, 0, 0.2)\r\nCreateAllUnits()"; !strings.Contains(fog, want) {
		t.Errorf("the patched Lua lacks %q", want)
	}
}

func withFogEnd(t *testing.T, end float32) []byte {
	t.Helper()
	data := fixtureInfo(t)
	info := readInfo(t, data, w3i.Extended)
	binary.LittleEndian.PutUint32(data[info.Details.Fog.End.Start:], math.Float32bits(end))
	binary.LittleEndian.PutUint32(data[info.Flags.Start:], uint32(info.Flags.Value|fogOn))
	return data
}

func TestANumberIsWrittenInPlainDecimalHoweverSmallOrLargeAndAZeroAsZero(t *testing.T) {
	for _, c := range []struct{ document, want string }{
		{`{"players":{"0":{"x":-0.0000001,"y":3e-7}}}`,
			"DefineStartLocation(0, -0.00000010000000116860974, 0.0000003000000106112566)\r\n"},
		{`{"players":{"0":{"x":0.000001,"y":0.0000011}}}`,
			"DefineStartLocation(0, 0.0000009999999974752427, 0.0000010999999631167157)\r\n"},
		{`{"environment":{"fog":{"enabled":true,"start":-0.000001,"density":1e-7}}}`,
			"SetTerrainFogEx(0, -0.0000009999999974752427, 5000, 0.00000010000000116860974, 0, 0, 0)\r\n"},
		{`{"players":{"0":{"x":-0.0,"y":0}}}`, "DefineStartLocation(0, 0, 0)\r\n"},
		{`{"players":{"0":{"x":1e-46,"y":-1e-46}}}`, "DefineStartLocation(0, 0, 0)\r\n"},
		{`{"environment":{"fog":{"enabled":true,"start":-0.0,"end":-0.0,"density":-0.0,"color":[1,2,3,4]}}}`,
			"SetTerrainFogEx(0, 0, 0, 0, 0.00392156862745098, 0.00784313725490196, 0.011764705882352941)\r\n"},
	} {
		if script := inLine(t, c.document, fixtureLua(t)); !strings.Contains(script, c.want) {
			t.Errorf("%s: the patched Lua lacks %q", c.document, c.want)
		}
	}
	s := settingsOf(t, `{"environment":{"fog":{"start":0}}}`)
	info, err := patchInfo(withFogEnd(t, 1e30), s, infoFile)
	if err != nil {
		t.Fatal(err)
	}
	script, err := afterInfo(fixtureLua(t), s, info)
	if want := "SetTerrainFogEx(0, 0, 1000000015047466200000000000000, 0.5, 0, 0, 0)\r\n"; err != nil || !strings.Contains(script, want) {
		t.Errorf("the patched Lua lacks %q: %v", want, err)
	}
}

var unsafeShapes = []struct{ document, old, new, words string }{
	{`{"info":{"name":"x"}}`, `SetMapName("TRIGSTR_001")`, `SetMapName("TRIGSTR_001", 1)`,
		"SetMapName in config() must have 1 argument(s)"},
	{`{"players":{"1":{"controller":"computer"}}}`, "SetPlayerStartLocation(Player(1), 1)", "SetPlayerStartLocation(Player(1), 2)",
		"must call SetPlayerStartLocation(Player(1), 1) to match"},
	{`{"players":{"1":{"controller":"computer"}}}`, "SetPlayerStartLocation(Player(1), 1)\r\n", "",
		"must call SetPlayerStartLocation(Player(1), 1) to match"},
	{`{"players":{"0":{"controller":"computer"}}}`, "SetPlayerController(Player(0), MAP_CONTROL_USER)",
		"SetPlayerController(Player(0 + 0), MAP_CONTROL_USER)",
		"cannot identify the player in a SetPlayerController call in InitCustomPlayerSlots()"},
	{`{"players":{"0":{"controller":"computer"}}}`, "SetPlayerController(Player(1), MAP_CONTROL_USER)",
		"SetPlayerController(p, MAP_CONTROL_USER)",
		"cannot identify the player in a SetPlayerController call in InitCustomPlayerSlots()"},
	{`{"players":{"0":{"x":1}}}`, "DefineStartLocation(0,", "DefineStartLocation(zero,",
		"cannot identify a DefineStartLocation index in config()"},
	{`{"players":{"0":{"x":1}}}`, "DefineStartLocation(1,", "DefineStartLocation(0,",
		"exactly one direct DefineStartLocation(0) in config() call, found 2"},
	{`{"players":{"0":{"fixedStart":false}}}`, "ForcePlayerStartLocation(Player(0), 0)",
		"ForcePlayerStartLocation(Player(0), 0)\r\nForcePlayerStartLocation(Player(0), 0)",
		"at most one ForcePlayerStartLocation(Player(0)) call, found 2"},
	{`{"players":{"0":{"fixedStart":true}}}`, "ForcePlayerStartLocation(Player(0), 0)", "ForcePlayerStartLocation(Player(0), 1)",
		"ForcePlayerStartLocation(Player(0)) must use start location 0"},
	{`{"players":{"0":{"race":"orc"}}}`, "SetPlayerRaceSelectable(Player(0), false)\r\n", "",
		"exactly one direct SetPlayerRaceSelectable(Player(0)) call, found 0"},
	{`{"players":{"0":{"name":"x"}}}`, "InitCustomPlayerSlots()\r\nInitCustomTeams()", "InitCustomTeams()",
		"exactly one direct InitCustomPlayerSlots in config() call, found 0"},
	{`{"forces":{"0":{"allied":true}}}`, "SetPlayerTeam(Player(3), 0)", "SetPlayerTeam(Player(3), 1)",
		"SetPlayerTeam(Player(3), 1) disagrees with force 0"},
	{`{"forces":{"1":{"allied":true}}}`, "SetPlayerTeam(Player(3), 0)", "SetPlayerTeam(Player(3), 1)",
		"SetPlayerTeam(Player(3), 1) disagrees with force 1"},
	{`{"forces":{"0":{"allied":true}}}`, "SetPlayerTeam(Player(3), 0)\r\n", "",
		"must call SetPlayerTeam(Player(3), 0) exactly once"},
	{`{"forces":{"0":{"allied":true}}}`, "SetPlayerTeam(Player(3), 0)", "SetPlayerTeam(Player(3), team)",
		"cannot identify the player and team of a SetPlayerTeam call"},
	{`{"forces":{"0":{"allied":true}}}`, "InitCustomTeams()\r\nInitAllyPriorities()", "InitAllyPriorities()",
		"exactly one direct InitCustomTeams in config() call, found 0"},
	{`{"environment":{"waterColor":[1,2,3,4]}}`, "CreateAllUnits()\r\nInitBlizzard()\r\n", "",
		"main() must call CreateAllUnits() or InitBlizzard() directly"},
	{`{"environment":{"waterColor":[1,2,3,4]}}`, "CreateAllUnits()\r\nInitBlizzard()", "CreateAllUnits(1)\r\nInitBlizzard()",
		"CreateAllUnits in main() must have 0 argument(s)"},
	{`{"environment":{"waterColor":[1,2,3,4]}}`, "CreateAllUnits()\r\nInitBlizzard()",
		"SetWaterBaseColor(1, 2, 3)\r\nCreateAllUnits()\r\nInitBlizzard()",
		"SetWaterBaseColor in main() must have 4 argument(s)"},
	{`{"environment":{"soundEnvironment":"Cave"}}`, "CreateAllUnits()\r\nInitBlizzard()",
		"NewSoundEnvironment(\"Second\")\r\nCreateAllUnits()\r\nInitBlizzard()",
		"at most one NewSoundEnvironment in main() call, found 2"},
	{`{"environment":{"fog":{"enabled":false}}}`, "CreateAllUnits()\r\nInitBlizzard()",
		"ResetTerrainFog(1)\r\nCreateAllUnits()\r\nInitBlizzard()",
		"ResetTerrainFog in main() must have 0 argument(s)"},
	{`{"forces":{"1":{"allied":true}}}`, "SetPlayerTeam(Player(11), 1)", "SetPlayerTeam(Player(11), 0.5)",
		"SetPlayerTeam(Player(11), 0.5) disagrees with force 1"},
	{`{"forces":{"1":{"allied":true}}}`, "SetPlayerTeam(Player(11), 1)", "SetPlayerTeam(Player(11), -0.0)",
		"SetPlayerTeam(Player(11), 0) disagrees with force 1"},
	{`{"forces":{"0":{"allied":true}}}`, "SetPlayerTeam(Player(11), 1)\r\nend", "SetPlayerTeam(Player(11), 1)\r\nreturn\r\nend",
		"the edited script could not be read back safely"},
	{`{"info":{"name":"x"}}`, "function InitGlobals()", "function InitGlobals(((",
		"Cannot safely read map Lua"},
}

func TestUnsafePlayerTeamAndEnvironmentShapesAreRefused(t *testing.T) {
	source := fixtureLua(t)
	for _, c := range unsafeShapes {
		_, err := withSettings(t, c.document, swapped(t, source, c.old, c.new))
		diagErr := asError(t, err, c.document)
		if diagErr.File != luaFile || diagErr.Hint == "" || !strings.Contains(diagErr.Msg, c.words) {
			t.Errorf("%s with %q: the error names %q, hints %q and says %q, want %q in it",
				c.document, c.new, diagErr.File, diagErr.Hint, diagErr.Msg, c.words)
		}
	}
}

func TestAScriptThatDoesNotReadIsRefusedWithItsLineAndColumn(t *testing.T) {
	source := swapped(t, fixtureLua(t), "function InitGlobals()", "function InitGlobals(((")
	_, err := withSettings(t, `{"info":{"name":"x"}}`, source)
	diagErr := asError(t, err, "a script that does not read")
	if diagErr.File != luaFile || diagErr.Line != 4 || diagErr.Column != 22 || diagErr.Msg != "Cannot safely read map Lua: expected a name" {
		t.Errorf("the error is %+v", diagErr)
	}
	_, err = patchMinimap(source, luaFile)
	if again := asError(t, err, "a script that does not read"); *again != *diagErr {
		t.Errorf("the minimap call is refused with %+v, the settings with %+v", again, diagErr)
	}
}

func TestATeamInARefusalIsWrittenInPlainDecimal(t *testing.T) {
	for team, want := range map[string]string{
		"0.0000005": "SetPlayerTeam(Player(11), 0.0000005) disagrees with force 1",
		"1e21":      "SetPlayerTeam(Player(11), 1000000000000000000000) disagrees with force 1",
	} {
		source := swapped(t, fixtureLua(t), "SetPlayerTeam(Player(11), 1)", "SetPlayerTeam(Player(11), "+team+")")
		if diagErr := refusedLua(t, `{"forces":{"1":{"allied":true}}}`, source); !strings.Contains(diagErr.Msg, want) {
			t.Errorf("the error says %q, want %q in it", diagErr.Msg, want)
		}
	}
}

func TestAScriptThatNoLongerReadsAfterTheEditsIsRefusedWithWhatIsWrongWithIt(t *testing.T) {
	source := swapped(t, fixtureLua(t), "SetPlayerTeam(Player(11), 1)\r\nend", "SetPlayerTeam(Player(11), 1)\r\nreturn\r\nend")
	diagErr := refusedLua(t, `{"forces":{"0":{"allied":true}}}`, source)
	var cause *diag.Error
	if !errors.As(diagErr.Cause, &cause) || cause.File != luaFile || !strings.Contains(cause.Msg, "return must end its block") {
		t.Errorf("the cause is %v", diagErr.Cause)
	}
}

func TestTheFirstOfSeveralRefusalsIsTheOneReported(t *testing.T) {
	source := swapped(t, fixtureLua(t), `SetMapName("TRIGSTR_001")`, `SetMapName("TRIGSTR_001", 1)`)
	source = swapped(t, source, "SetPlayerStartLocation(Player(1), 1)\r\n", "")
	source = swapped(t, source, "CreateAllUnits()\r\nInitBlizzard()\r\n", "")
	everything := `{"info":{"name":"x","description":"y"},"players":{"1":{"controller":"computer"}},
		"forces":{"7":{"allied":true}},"environment":{"waterColor":[1,2,3,4]}}`
	s := settingsOf(t, everything)
	_, err := patchLua(source, s, readInfo(t, fixtureInfo(t), w3i.Extended), luaFile)
	if diagErr := asError(t, err, everything); !strings.Contains(diagErr.Msg, "SetMapName in config() must have 1 argument(s)") {
		t.Errorf("the error says %q", diagErr.Msg)
	}
	s.Info = manifest.Info{}
	_, err = patchLua(source, s, readInfo(t, fixtureInfo(t), w3i.Extended), luaFile)
	if diagErr := asError(t, err, everything); !strings.Contains(diagErr.Msg, "must call SetPlayerStartLocation(Player(1), 1)") {
		t.Errorf("without the name, the error says %q", diagErr.Msg)
	}
}

func TestAPlayerOrAForceTheMapInfoLacksIsRefusedWithWhatToCreate(t *testing.T) {
	info := readInfo(t, fixtureInfo(t), w3i.Extended)
	for _, c := range []struct{ document, words, hint string }{
		{`{"players":{"7":{"controller":"computer"}}}`, "player 7 does not exist in war3map.w3i", "Create this player slot"},
		{`{"forces":{"2":{"allied":true}}}`, "force 2 does not exist in war3map.w3i", "Create this force"},
	} {
		_, err := patchLua(fixtureLua(t), settingsOf(t, c.document), info, luaFile)
		diagErr := asError(t, err, c.document)
		if diagErr.File != luaFile || !strings.Contains(diagErr.Msg, c.words) || !strings.Contains(diagErr.Hint, c.hint) {
			t.Errorf("%s: the error names %q, says %q and hints %q", c.document, diagErr.File, diagErr.Msg, diagErr.Hint)
		}
	}
}

func TestForceFlagEditsAppendEffectiveStatesAfterTheEditorsCalls(t *testing.T) {
	script := inLine(t,
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
	if got := teamsTail(script); got != want.String() {
		t.Errorf("the appended team states are\n%q\nwant\n%q", got, want.String())
	}
	if !strings.Contains(script, "SetPlayerAllianceStateAllyBJ(Player(0), Player(1), true)\r\n") {
		t.Error("the editor's own alliance calls are gone")
	}
	one := teamsTail(inLine(t, `{"forces":{"1":{"sharedVision":true}}}`, fixtureLua(t)))
	if one != "SetPlayerState(Player(11), PLAYER_STATE_ALLIED_VICTORY, 0)\r\n" {
		t.Errorf("force 1's states are %q", one)
	}
}

const indentedMain = "function main()\n  SetTerrainFogEx(0, 1, 2, 0.5, 1, 1, 1) ResetTerrainFog()\n  NewSoundEnvironment(\"Old\")\n  InitBlizzard()\nend\n"

func TestEnvironmentEditsReplaceOldInitializationImmediatelyBeforeTheAnchor(t *testing.T) {
	source := fixtureLua(t)
	want := swapped(t, source, "NewSoundEnvironment(\"Default\")\r\n", "")
	want = swapped(t, want, "CreateAllUnits()\r\nInitBlizzard()",
		"NewSoundEnvironment(\"Default\")\r\nResetTerrainFog()\r\nCreateAllUnits()\r\nInitBlizzard()")
	if got := inLine(t, `{"environment":{"soundEnvironment":"","fog":{"enabled":false}}}`, source); got != want {
		t.Error("the environment edit differs")
	}
	got := inLine(t, `{"environment":{"soundEnvironment":"Cave","fog":{"enabled":true}}}`, indentedMain)
	wantCustom := "function main()\n  ; ;\n  NewSoundEnvironment(\"Cave\")\n  SetTerrainFogEx(0, 3000, 5000, 0.5, 0, 0, 0)\n  InitBlizzard()\nend\n"
	if got != wantCustom {
		t.Errorf("patched = %q, want %q", got, wantCustom)
	}
}

var joinable = []struct{ source, want string }{
	{"function main()\nx = b\nResetTerrainFog();(f)()\nInitBlizzard()\nend", "function main()\nx = b\n;(f)()\n"},
	{"function main()\nx = b\nResetTerrainFog();\n(f)()\nInitBlizzard()\nend", "function main()\nx = b\n;\n(f)()\n"},
	{"function main()\nx = b\nResetTerrainFog() -- c\ny()\nInitBlizzard()\nend", "function main()\nx = b\n; -- c\ny()\n"},
}

func TestRemovingACallNeverJoinsTheStatementsAroundIt(t *testing.T) {
	for _, c := range joinable {
		script := inLine(t, `{"environment":{"fog":{"enabled":false}}}`, c.source)
		if want := c.want + "ResetTerrainFog()\nInitBlizzard()\nend"; script != want {
			t.Errorf("patched = %q, want %q", script, want)
		}
		mainCalls(t, script)
	}
}

func TestTextsAreWrittenWithQuotesBackslashesAndControlCharactersEscaped(t *testing.T) {
	for input, want := range map[string]string{
		"Back\\slash \"q\" \t\x7f \xe2\x9c\x93 \xe9\x9b\xaa \xf0\x9f\x98\x80": `"Back\\slash \"q\" \009\127 ` + "\xe2\x9c\x93 \xe9\x9b\xaa \xf0\x9f\x98\x80" + `"`,
		"\r\n1\x001": `"\013\0101\0001"`,
		"\n123":      `"\010123"`,
		"":           `""`,
	} {
		info := readInfo(t, fixtureInfo(t), w3i.Extended)
		info.Name.Value, info.Description.Value, info.Details.SoundEnvironment.Value = input, input, input
		s := settingsOf(t, `{"info":{"name":"x","description":"x"},"environment":{"soundEnvironment":"x"}}`)
		script, err := patchLua(fixtureLua(t), s, info, luaFile)
		if err != nil {
			t.Fatalf("%q: %v", input, err)
		}
		for _, call := range []string{"SetMapName(" + want + ")\r\n", "SetMapDescription(" + want + ")\r\n"} {
			if !strings.Contains(script, call) {
				t.Errorf("the patched Lua lacks %s", call)
			}
		}
		if input == "" {
			want = `"Default"`
		}
		if call := "NewSoundEnvironment(" + want + ")\r\nCreateAllUnits()"; !strings.Contains(script, call) {
			t.Errorf("the patched Lua lacks %s", call)
		}
	}
}

func TestUnreadablePatchedMapInfoIsReportedAgainstTheMapInfoFile(t *testing.T) {
	_, err := afterInfo(fixtureLua(t), settingsOf(t, `{"info":{"name":"X"}}`), []byte{1, 2})
	if diagErr := asError(t, err, "a cut-off map info"); diagErr.File != infoFile {
		t.Errorf("the error names %q", diagErr.File)
	}
}

func TestAMapInfoWithoutWhatTheSettingsNeedIsTheCallersMistake(t *testing.T) {
	basic := readInfo(t, fixtureInfo(t), w3i.Basic)
	for document, info := range map[string]*w3i.Info{
		`{"info":{"name":"X"}}`:                       nil,
		`{"players":{"0":{"name":"Hero"}}}`:           basic,
		`{"forces":{"0":{"allied":true}}}`:            basic,
		`{"environment":{"soundEnvironment":"Cave"}}`: basic,
	} {
		_, err := patchLua(fixtureLua(t), settingsOf(t, document), info, luaFile)
		var expected *diag.Error
		if err == nil || errors.As(err, &expected) {
			t.Errorf("%s: got %v, want an error that is not a diag error", document, err)
		}
	}
	if _, err := patchLua(fixtureLua(t), settingsOf(t, `{"info":{"name":"X"}}`), basic, luaFile); err != nil {
		t.Error(err)
	}
}

func TestTheMinimapCallBecomesTheLastStatementOfMainOnALineOfItsOwn(t *testing.T) {
	script := fixtureLua(t)
	result, err := patchMinimap(script, luaFile)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != len(script)+len(minimapCall)+2 {
		t.Errorf("the patch added %d bytes", len(result)-len(script))
	}
	if !strings.Contains(result, "RunInitializationTriggers()\r\n"+minimapCall+"\r\nend\r\n") {
		t.Error("the call is not the last line of main")
	}
	calls := mainCalls(t, result)
	if n := len(calls); calls[n-1] != "BlzChangeMinimapTerrainTex" || calls[n-2] != "RunInitializationTriggers" {
		t.Errorf("main calls %q", calls)
	}
	if strings.Count(result, minimapCall) != 1 {
		t.Error("the call was added more than once")
	}
}

var minimapSources = map[string]string{
	"function main()\r\n  InitBlizzard()\r\n  end\r\n": "function main()\r\n  InitBlizzard()\r\n  " + minimapCall + "\r\n  end\r\n",
	"function main()\nend\n":                           "function main()\n" + minimapCall + "\nend\n",
	"function main() InitBlizzard() end":               "function main() InitBlizzard() " + minimapCall + " end",
}

func TestTheMinimapCallKeepsTheScriptsLineEndingAndTheIndentationOfMainsEnd(t *testing.T) {
	for source, want := range minimapSources {
		if got, err := patchMinimap(source, luaFile); err != nil || got != want {
			t.Errorf("patchMinimap(%q) = %q, %v", source, got, err)
		}
	}
}

var withoutOneMain = map[string]int{
	"function config()\nend\n":                     0,
	"function main()\nend\nfunction main()\nend\n": 2,
}

func TestTheMinimapCallNeedsExactlyOneGlobalMain(t *testing.T) {
	for source, count := range withoutOneMain {
		_, err := patchMinimap(source, luaFile)
		diagErr := asError(t, err, source)
		want := fmt.Sprintf("expected exactly one global function main(), found %d.", count)
		if !strings.Contains(diagErr.Msg, want) || diagErr.File != luaFile || diagErr.Hint == "" {
			t.Errorf("error = %+v", diagErr)
		}
	}
	_, err := patchMinimap("function main(", luaFile)
	if diagErr := asError(t, err, "a script that does not read"); diagErr.File != luaFile {
		t.Errorf("the error names %q", diagErr.File)
	}
}

const (
	mainReturnsValue = "function main()\n  InitBlizzard()\n  return 1\nend\n"
	mainReturns      = "function main()\n  InitBlizzard()\n  return\nend\n"
)

func TestTheMinimapCallIsRefusedWhereItWouldStandAfterAReturnedValue(t *testing.T) {
	_, err := patchMinimap(mainReturnsValue, luaFile)
	diagErr := asError(t, err, mainReturnsValue)
	if diagErr.File != luaFile || diagErr.Hint == "" || !strings.Contains(diagErr.Msg, "the edited script could not be read back safely") {
		t.Errorf("the error names %q, hints %q and says %q", diagErr.File, diagErr.Hint, diagErr.Msg)
	}
	var cause *diag.Error
	if !errors.As(diagErr.Cause, &cause) || !strings.Contains(cause.Msg, "return must end its block") {
		t.Errorf("the cause is %v", diagErr.Cause)
	}
	want := "function main()\n  InitBlizzard()\n  return\n" + minimapCall + "\nend\n"
	if got, err := patchMinimap(mainReturns, luaFile); err != nil || got != want {
		t.Errorf("patchMinimap(%q) = %q, %v", mainReturns, got, err)
	}
}

func damaged(t *testing.T, change func(player *w3i.Player, fog *w3i.Fog)) *w3i.Info {
	t.Helper()
	info := readInfo(t, fixtureInfo(t), w3i.Extended)
	info.Flags.Value |= fogOn
	change(&info.Details.Players[0], &info.Details.Fog)
	return info
}

func TestAMapInfoWithAValueTheScriptCannotTakeIsRefused(t *testing.T) {
	nan, endless := float32(math.NaN()), float32(math.Inf(-1))
	for _, c := range []struct {
		document, words string
		change          func(player *w3i.Player, fog *w3i.Fog)
	}{
		{`{"players":{"0":{"controller":"computer"}}}`, "player 0 has controller 9 in the map info",
			func(player *w3i.Player, _ *w3i.Fog) { player.Controller.Value = 9 }},
		{`{"players":{"0":{"controller":"computer"}}}`, "player 0 has controller 0 in the map info",
			func(player *w3i.Player, _ *w3i.Fog) { player.Controller.Value = 0 }},
		{`{"players":{"0":{"controller":"computer"}}}`, "player 0 has controller -1 in the map info",
			func(player *w3i.Player, _ *w3i.Fog) { player.Controller.Value = -1 }},
		{`{"players":{"0":{"race":"orc"}}}`, "player 0 has race 5 in the map info",
			func(player *w3i.Player, _ *w3i.Fog) { player.Race.Value = 5 }},
		{`{"players":{"0":{"race":"orc"}}}`, "player 0 has race -2 in the map info",
			func(player *w3i.Player, _ *w3i.Fog) { player.Race.Value = -2 }},
		{`{"players":{"0":{"x":1}}}`, "player 0 has a start position in the map info that is not a number",
			func(player *w3i.Player, _ *w3i.Fog) { player.Y.Value = nan }},
		{`{"players":{"0":{"y":1}}}`, "player 0 has a start position in the map info that is not a number",
			func(player *w3i.Player, _ *w3i.Fog) { player.X.Value = endless }},
		{`{"environment":{"fog":{"style":1}}}`, "the fog of the map info has a start, an end or a density that is not a number",
			func(_ *w3i.Player, fog *w3i.Fog) { fog.Density.Value = nan }},
		{`{"environment":{"fog":{"style":1}}}`, "the fog of the map info has a start, an end or a density that is not a number",
			func(_ *w3i.Player, fog *w3i.Fog) { fog.Start.Value = endless }},
	} {
		_, err := patchLua(fixtureLua(t), settingsOf(t, c.document), damaged(t, c.change), luaFile)
		diagErr := asError(t, err, c.document)
		if diagErr.File != luaFile || !strings.Contains(diagErr.Msg, c.words) || !strings.Contains(diagErr.Hint, "Re-save the map in World Editor") {
			t.Errorf("%s: the error names %q, says %q and hints %q; want %q in it", c.document, diagErr.File, diagErr.Msg, diagErr.Hint, c.words)
		}
	}
}

func TestAValueOfTheMapInfoThatNoSettingMakesTheScriptTakeIsNotLookedAt(t *testing.T) {
	broken := damaged(t, func(player *w3i.Player, fog *w3i.Fog) {
		player.Controller.Value, player.Race.Value, player.X.Value = 9, 9, float32(math.NaN())
		fog.End.Value = float32(math.Inf(1))
	})
	for _, document := range []string{
		`{"players":{"0":{"name":"Hero","fixedStart":true},"1":{"controller":"computer","race":"orc","x":1}}}`,
		`{"environment":{"soundEnvironment":"Cave","waterColor":[1,2,3,4]}}`,
	} {
		if _, err := patchLua(fixtureLua(t), settingsOf(t, document), broken, luaFile); err != nil {
			t.Errorf("%s: %v", document, err)
		}
	}
	broken.Flags.Value &^= fogOn
	script, err := patchLua(fixtureLua(t), settingsOf(t, `{"environment":{"fog":{"style":1}}}`), broken, luaFile)
	if err != nil || !strings.Contains(script, "ResetTerrainFog()\r\nCreateAllUnits()") {
		t.Errorf("a fog that is not shown: %v", err)
	}
}

func TestTheMinimapCallGoesInBesideTheOtherLuaSettings(t *testing.T) {
	script := inLine(t, `{"info":{"name":"Both"},"environment":{"soundEnvironment":"Dungeon"}}`, fixtureLua(t))
	result, err := patchMinimap(script, luaFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result, `SetMapName("Both")`) || !strings.Contains(result, `NewSoundEnvironment("Dungeon")`) {
		t.Error("the other settings are gone")
	}
	if calls := mainCalls(t, result); calls[len(calls)-1] != "BlzChangeMinimapTerrainTex" {
		t.Errorf("main calls %q", calls)
	}
}
