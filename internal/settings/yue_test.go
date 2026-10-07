package settings

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/tooltest"
)

// The patched script, run in the Lua inside the real compiler on stand-ins for the game's natives. Each test
// starts the compiler once, and is skipped on a machine without one (tooltest.Yue).

// printedBy is what a Lua script prints when it runs. A script that fails, fails the test.
func printedBy(t *testing.T, script string) string {
	t.Helper()
	return tooltest.RunLua(t, testkit.WriteFile(t, t.TempDir(), "settings.lua", []byte(script)))
}

func TestSettingsLuaFogResetAndUnambiguousEscapes(t *testing.T) {
	lua := inLine(t, `{"info":{"name":"\n123"},"environment":{"fog":{"enabled":true,"start":100,"end":1000}}}`, `
function config() SetMapName("old") end
function main() SetTerrainFogEx(0, 1, 2, 0.5, 1, 1, 1) ResetTerrainFog() InitBlizzard() end`)
	out := printedBy(t, `local events = {}
function SetMapName(value) assert(value == "\010123") end
function SetTerrainFogEx(style, first, last) assert(first == 100 and last == 1000) events[#events+1] = "fog" end
function ResetTerrainFog() error("old fog reset survived") end
function InitBlizzard() events[#events+1] = "init" end
`+lua+`
config(); main(); assert(table.concat(events, ",") == "fog,init"); io.write("settings-ok")`)
	if out != "settings-ok" {
		t.Fatal(out)
	}
}

func TestSettingsLuaRemovedCallSeparatesStatements(t *testing.T) {
	lua := inLine(t, `{"environment":{"fog":{"enabled":false}}}`, "function main()\nx = b\nResetTerrainFog();(f)()\nInitBlizzard()\nend")
	out := printedBy(t, `local events = {}
b = function() error("statements were joined") end
f = function() events[#events+1] = "f" end
function ResetTerrainFog() events[#events+1] = "reset" end
function InitBlizzard() events[#events+1] = "init" end
`+lua+` main(); io.write(table.concat(events, ","))`)
	if out != "f,reset,init" {
		t.Fatal(out)
	}
}

// gameNatives are the natives and the functions of Blizzard's own script that the fixture's script, or a setting
// put into it, calls.
var gameNatives = strings.Fields(`BlzCreateUnitWithSkin ConditionalTriggerExecute ConvertPlayerColor CreateTrigger
DefineStartLocation ForcePlayerStartLocation FourCC GetCameraMargin InitBlizzard NewSoundEnvironment
SelectUnitForPlayerSingle SetAmbientDaySound SetAmbientNightSound SetCameraBounds SetDayNightModels SetGamePlacement
SetHDWaterParamsEx SetMapDescription SetMapMusic SetMapName SetPlayerAllianceStateAllyBJ SetPlayerAllianceStateVisionBJ
SetPlayerColor SetPlayerController SetPlayerRacePreference SetPlayerRaceSelectable SetPlayerRaceSkin SetPlayerStartLocation
SetPlayerTeam SetPlayers SetStartLocPrio SetStartLocPrioCount SetTeams TriggerAddAction ResetTerrainFog
SetPlayerAllianceStateControlBJ SetPlayerAllianceStateFullControlBJ SetPlayerName SetPlayerState SetTerrainFogEx SetWaterBaseColor`)

// gameConstant finds the names of the game's constants in a script: capitals and digits in parts joined by "_".
var gameConstant = regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`)

// callsOf runs config() and main() of a map script outside the game and returns the calls they made of the
// game's natives, in order, each as its name and its arguments: a number in full, anything else as Lua prints
// it. A constant of the game stands for its own name, a native returns 0, and Player gives the player's number.
// A script that reads a global that is none of these fails the test.
func callsOf(t *testing.T, lua string) []string {
	t.Helper()
	constants := gameConstant.FindAllString(fixtureLua(t), -1)
	constants = append(constants, "PLAYER_STATE_ALLIED_VICTORY", "MAP_CONTROL_RESCUABLE")
	script := `local calls = {}
local function record(name, ...)
 local parts = {}
 for i = 1, select("#", ...) do
  local value = select(i, ...)
  parts[#parts+1] = type(value) == "number" and string.format("%.17g", value) or tostring(value)
 end
 calls[#calls+1] = name .. "(" .. table.concat(parts, ",") .. ")"
end
`
	for _, name := range constants {
		script += name + ` = "` + name + "\"\n"
	}
	for _, name := range gameNatives {
		script += `function ` + name + `(...) record("` + name + `", ...) return 0 end` + "\n"
	}
	script += `function Player(id) return id end
local stderr, exit = io.stderr, os.exit
setmetatable(_G, { __index = function(_, name) stderr:write("undefined global " .. tostring(name)) exit(1) end })
` + lua + ` config(); main(); io.write(table.concat(calls, "\n"))`
	return strings.Split(printedBy(t, script), "\n")
}

func TestSettingsLuaUnpatchedFixtureRuns(t *testing.T) {
	source := fixtureLua(t)
	if inLine(t, `{}`, source) != source {
		t.Fatal("empty settings changed source")
	}
	calls := callsOf(t, source)
	for _, wanted := range []string{"SetPlayerController(11,MAP_CONTROL_COMPUTER)", "ForcePlayerStartLocation(0,0)"} {
		if !slices.Contains(calls, wanted) {
			t.Fatal(wanted)
		}
	}
	for _, call := range calls {
		if strings.HasPrefix(call, "SetPlayerState(") {
			t.Fatal(call)
		}
	}
}

// overrides sets something of every kind the script takes: the map's name and description, two players, a
// force's flags, and the environment.
const overrides = `{"info":{"name":"Moonwell","description":""},` +
	`"players":{"0":{"name":"","controller":"computer","race":"selectable","fixedStart":false,"x":0,"y":0.1},` +
	`"11":{"controller":"rescuable","race":"undead"}},` +
	`"forces":{"0":{"allied":false,"alliedVictory":true,"sharedVision":false,"sharedControl":true}},` +
	`"environment":{"soundEnvironment":"","waterColor":[0,0,0,0],"fog":{"enabled":true,"start":0,"color":[255,0,0,0]}}}`

// overrideCalls are calls the script makes once the overrides are in it.
var overrideCalls = []string{
	"SetMapName(Moonwell)", "SetMapDescription()", "DefineStartLocation(0,0,0.10000000149011612)",
	"SetPlayerRacePreference(0,RACE_PREF_USER_SELECTABLE)", "SetPlayerRaceSelectable(0,true)",
	"SetPlayerController(0,MAP_CONTROL_COMPUTER)", "SetPlayerRacePreference(11,RACE_PREF_UNDEAD)",
	"SetPlayerController(11,MAP_CONTROL_RESCUABLE)", "SetPlayerState(3,PLAYER_STATE_ALLIED_VICTORY,1)",
	"SetPlayerAllianceStateAllyBJ(0,1,false)", "SetPlayerAllianceStateVisionBJ(3,2,false)",
	"SetPlayerAllianceStateControlBJ(1,0,true)", "SetPlayerAllianceStateFullControlBJ(2,3,false)",
	"SetPlayerTeam(11,1)", "NewSoundEnvironment(Default)", "SetWaterBaseColor(0,0,0,0)",
	"SetTerrainFogEx(0,0,5000,0.5,1,0,0)",
}

func TestSettingsLuaOverridesPrecedeUnitCreation(t *testing.T) {
	calls := callsOf(t, inLine(t, overrides, fixtureLua(t)))
	for _, wanted := range overrideCalls {
		if !slices.Contains(calls, wanted) {
			t.Errorf("missing %s", wanted)
		}
	}
	firstUnit := slices.IndexFunc(calls, func(s string) bool { return strings.HasPrefix(s, "BlzCreateUnitWithSkin(") })
	for _, prefix := range []string{"NewSoundEnvironment(", "SetWaterBaseColor(", "SetTerrainFogEx("} {
		at := slices.IndexFunc(calls, func(s string) bool { return strings.HasPrefix(s, prefix) })
		if at < 0 || at >= firstUnit {
			t.Error(prefix + " must precede unit creation")
		}
	}
	if firstUnit >= slices.Index(calls, "InitBlizzard()") {
		t.Fatal("units must precede InitBlizzard")
	}
	countSound, lastAlly := 0, -1
	for i, call := range calls {
		if strings.HasPrefix(call, "ForcePlayerStartLocation(0,") {
			t.Error(call)
		}
		if strings.HasPrefix(call, "NewSoundEnvironment(") {
			countSound++
		}
		if call == "SetPlayerAllianceStateAllyBJ(0,1,true)" {
			lastAlly = i
		}
	}
	if countSound != 1 || lastAlly >= slices.Index(calls, "SetPlayerAllianceStateAllyBJ(0,1,false)") {
		t.Fatal("override order", calls)
	}
}

func TestSettingsLuaDisabledFogResetsBeforeUnits(t *testing.T) {
	calls := callsOf(t, inLine(t, `{"environment":{"fog":{"enabled":false,"density":0}}}`, fixtureLua(t)))
	reset := slices.Index(calls, "ResetTerrainFog()")
	unit := slices.IndexFunc(calls, func(s string) bool { return strings.HasPrefix(s, "BlzCreateUnitWithSkin(") })
	if reset < 0 || reset >= unit {
		t.Fatal(calls)
	}
	for _, call := range calls {
		if strings.HasPrefix(call, "SetTerrainFogEx(") {
			t.Fatal(call)
		}
	}
}
