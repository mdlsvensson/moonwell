package settings_test

import (
	"context"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/yuetest"
)

func runSettingsLua(t *testing.T, script string) string {
	t.Helper()
	dir := t.TempDir()
	file := testkit.WriteFile(t, dir, "settings.lua", []byte(script))
	r, err := proc.Run(context.Background(), yuetest.Need(t), []string{"-e", file}, proc.Options{Dir: dir})
	if err != nil || r.Code != 0 {
		t.Fatalf("Lua: %v, %d\n%s\n%s", err, r.Code, r.Stdout, r.Stderr)
	}
	return strings.ReplaceAll(r.Stdout, "\r\n", "\n")
}

func TestSettingsLuaFogResetAndUnambiguousEscapes(t *testing.T) {
	lua := patchLua(t, `{"info":{"name":"\n123"},"environment":{"fog":{"enabled":true,"start":100,"end":1000}}}`, `
function config() SetMapName("old") end
function main() SetTerrainFogEx(0, 1, 2, 0.5, 1, 1, 1) ResetTerrainFog() InitBlizzard() end`)
	out := runSettingsLua(t, `local events = {}
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
	lua := patchLua(t, `{"environment":{"fog":{"enabled":false}}}`, "function main()\nx = b\nResetTerrainFog();(f)()\nInitBlizzard()\nend")
	out := runSettingsLua(t, `local events = {}
b = function() error("statements were joined") end
f = function() events[#events+1] = "f" end
function ResetTerrainFog() events[#events+1] = "reset" end
function InitBlizzard() events[#events+1] = "init" end
`+lua+` main(); io.write(table.concat(events, ","))`)
	if out != "f,reset,init" {
		t.Fatal(out)
	}
}

func executeSettingsLua(t *testing.T, lua string) []string {
	t.Helper()
	constants := regexp.MustCompile(`\b[A-Z][A-Z0-9]*(?:_[A-Z0-9]+)+\b`).FindAllString(fixtureLua(t), -1)
	constants = append(constants, "PLAYER_STATE_ALLIED_VICTORY", "MAP_CONTROL_RESCUABLE")
	natives := strings.Fields(`BlzCreateUnitWithSkin ConditionalTriggerExecute ConvertPlayerColor CreateTrigger
DefineStartLocation ForcePlayerStartLocation FourCC GetCameraMargin InitBlizzard NewSoundEnvironment
SelectUnitForPlayerSingle SetAmbientDaySound SetAmbientNightSound SetCameraBounds SetDayNightModels SetGamePlacement
SetHDWaterParamsEx SetMapDescription SetMapMusic SetMapName SetPlayerAllianceStateAllyBJ SetPlayerAllianceStateVisionBJ
SetPlayerColor SetPlayerController SetPlayerRacePreference SetPlayerRaceSelectable SetPlayerRaceSkin SetPlayerStartLocation
SetPlayerTeam SetPlayers SetStartLocPrio SetStartLocPrioCount SetTeams TriggerAddAction ResetTerrainFog
SetPlayerAllianceStateControlBJ SetPlayerAllianceStateFullControlBJ SetPlayerName SetPlayerState SetTerrainFogEx SetWaterBaseColor`)
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
	for _, name := range natives {
		script += `function ` + name + `(...) record("` + name + `", ...) return 0 end` + "\n"
	}
	script += `function Player(id) return id end
local stderr, exit = io.stderr, os.exit
setmetatable(_G, { __index = function(_, name) stderr:write("undefined global " .. tostring(name)) exit(1) end })
` + lua + ` config(); main(); io.write(table.concat(calls, "\n"))`
	return strings.Split(runSettingsLua(t, script), "\n")
}

func TestSettingsLuaUnpatchedFixtureRuns(t *testing.T) {
	source := fixtureLua(t)
	if patchLua(t, `{}`, source) != source {
		t.Fatal("empty settings changed source")
	}
	calls := executeSettingsLua(t, source)
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

func TestSettingsLuaOverridesPrecedeUnitCreation(t *testing.T) {
	calls := executeSettingsLua(t, patchLua(t, `{"info":{"name":"Moonwell","description":""},"players":{"0":{"name":"","controller":"computer","race":"selectable","fixedStart":false,"x":0,"y":0.1},"11":{"controller":"rescuable","race":"undead"}},"forces":{"0":{"allied":false,"alliedVictory":true,"sharedVision":false,"sharedControl":true}},"environment":{"soundEnvironment":"","waterColor":[0,0,0,0],"fog":{"enabled":true,"start":0,"color":[255,0,0,0]}}}`, fixtureLua(t)))
	for _, wanted := range []string{"SetMapName(Moonwell)", "SetMapDescription()", "DefineStartLocation(0,0,0.10000000149011612)", "SetPlayerRacePreference(0,RACE_PREF_USER_SELECTABLE)", "SetPlayerRaceSelectable(0,true)", "SetPlayerController(0,MAP_CONTROL_COMPUTER)", "SetPlayerRacePreference(11,RACE_PREF_UNDEAD)", "SetPlayerController(11,MAP_CONTROL_RESCUABLE)", "SetPlayerState(3,PLAYER_STATE_ALLIED_VICTORY,1)", "SetPlayerAllianceStateAllyBJ(0,1,false)", "SetPlayerAllianceStateVisionBJ(3,2,false)", "SetPlayerAllianceStateControlBJ(1,0,true)", "SetPlayerAllianceStateFullControlBJ(2,3,false)", "SetPlayerTeam(11,1)", "NewSoundEnvironment(Default)", "SetWaterBaseColor(0,0,0,0)", "SetTerrainFogEx(0,0,5000,0.5,1,0,0)"} {
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
	calls := executeSettingsLua(t, patchLua(t, `{"environment":{"fog":{"enabled":false,"density":0}}}`, fixtureLua(t)))
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
