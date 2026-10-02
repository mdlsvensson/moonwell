package settings

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/luasrc"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/internal/w3i"
)

type luaEdit struct {
	start, end int
	text       string
}

const resaveLua = "Re-save the map in World Editor to restore its generated Lua initialization."

// LuaString writes value as a Lua 5.3 string literal; control characters use three-digit decimal escapes so that
// digits after them stay separate.
func LuaString(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value {
		switch {
		case r == '\\' || r == '"':
			b.WriteByte('\\')
			b.WriteRune(r)
		case r < 32 || r == 127:
			fmt.Fprintf(&b, `\%03d`, r)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// applyLuaEdits applies edits that do not overlap, from the end; insertions at one offset keep their creation
// order.
func applyLuaEdits(source string, edits []luaEdit) (string, error) {
	var combined []*luaEdit
	insertions := map[int]*luaEdit{}
	for _, edit := range edits {
		if edit.start != edit.end {
			combined = append(combined, &edit)
		} else if existing, ok := insertions[edit.start]; ok {
			existing.text += edit.text
		} else {
			insertions[edit.start] = &edit
			combined = append(combined, &edit)
		}
	}
	slices.SortStableFunc(combined, func(a, b *luaEdit) int {
		if a.start != b.start {
			return b.start - a.start
		}
		return b.end - a.end
	})
	boundary := len(source)
	for _, edit := range combined {
		if edit.start < 0 || edit.end < edit.start || edit.end > boundary {
			return "", errors.New("Overlapping or invalid Lua settings edits.")
		}
		source = source[:edit.start] + edit.text + source[edit.end:]
		boundary = edit.start
	}
	return source, nil
}

var (
	blank          = regexp.MustCompile(`^[ \t]*$`)
	restOfLine     = regexp.MustCompile(`^[ \t]*\r?\n`)
	startsWithName = regexp.MustCompile(`^` + text.SpaceClass + `*[A-Za-z_]`)
)

// luaFailure carries a refusal out of the nested helpers of PatchLua.
type luaFailure struct{ err error }

func lineStart(source string, at int) int {
	return strings.LastIndex(source[:at], "\n") + 1
}

func eolOf(source string) string {
	if strings.Contains(source, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

// PatchLua brings the Lua World Editor generated into line with the already patched map info. It returns the
// source unchanged when no setting has a Lua counterpart. file names the Lua file and w3iFile the map info in
// errors.
func PatchLua(source string, s *Settings, patchedW3i []byte, file, w3iFile string) (patched string, err error) {
	var infoKeys []string
	if s.Info.Name != nil {
		infoKeys = append(infoKeys, "name")
	}
	if s.Info.Description != nil {
		infoKeys = append(infoKeys, "description")
	}
	var flagged []string
	for key, values := range s.Forces.All() {
		if slices.ContainsFunc(forceBits, func(flag forceBit) bool { return flag.get(&values) != nil }) {
			flagged = append(flagged, key)
		}
	}
	environment := s.Environment
	if len(infoKeys) == 0 && s.Players.Len() == 0 && len(flagged) == 0 && environment.empty() {
		return source, nil
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			failure, ok := recovered.(luaFailure)
			if !ok {
				panic(recovered)
			}
			patched, err = "", failure.err
		}
	}()
	failHint := func(message, hint string) {
		panic(luaFailure{&diag.Error{Msg: "Cannot apply map settings to Lua: " + message, File: file, Hint: hint}})
	}
	fail := func(message string) { failHint(message, resaveLua) }

	info, err := w3i.Read(patchedW3i, s.HasExtended(), w3iFile)
	if err != nil {
		return "", err
	}
	functions, err := luasrc.Functions(source, file)
	if err != nil {
		return "", err
	}
	var edits []luaEdit
	eol := eolOf(source)

	global := func(name string) luasrc.Function {
		var found []luasrc.Function
		for _, function := range functions {
			if function.Name == name {
				found = append(found, function)
			}
		}
		if len(found) != 1 {
			fail(fmt.Sprintf("expected exactly one global function %s(), found %d.", name, len(found)))
		}
		return found[0]
	}
	callsNamed := func(fn luasrc.Function, name string, arity int) []luasrc.Call {
		var calls []luasrc.Call
		for _, call := range fn.Calls {
			if call.Name != name {
				continue
			}
			if len(call.Args) != arity {
				fail(fmt.Sprintf("%s in %s() must have %d argument(s).", name, fn.Name, arity))
			}
			calls = append(calls, call)
		}
		return calls
	}
	one := func(calls []luasrc.Call, label string) luasrc.Call {
		if len(calls) != 1 {
			fail(fmt.Sprintf("expected exactly one direct %s call, found %d.", label, len(calls)))
		}
		return calls[0]
	}
	unique := func(fn luasrc.Function, name string, arity int) luasrc.Call {
		return one(callsNamed(fn, name, arity), name+" in "+fn.Name+"()")
	}
	// Every call of this name must name a literal player, or the target cannot be established.
	forPlayer := func(fn luasrc.Function, name string, arity, id int) []luasrc.Call {
		var calls []luasrc.Call
		for _, call := range callsNamed(fn, name, arity) {
			target, ok := luasrc.PlayerID(call.Args[0])
			if !ok {
				fail(fmt.Sprintf("cannot identify the player in a %s call in %s().", name, fn.Name))
			}
			if target == id {
				calls = append(calls, call)
			}
		}
		return calls
	}
	optional := func(calls []luasrc.Call, label string) *luasrc.Call {
		if len(calls) > 1 {
			fail(fmt.Sprintf("expected at most one %s call, found %d.", label, len(calls)))
		}
		if len(calls) == 0 {
			return nil
		}
		return &calls[0]
	}
	literalIs := func(tokens []luasrc.Token, want int) bool {
		value, ok := luasrc.LiteralNumber(tokens)
		return ok && value == float64(want)
	}

	indent := func(at int) (string, bool) {
		prefix := source[lineStart(source, at):at]
		return prefix, blank.MatchString(prefix)
	}
	separator := func(at int) string {
		if prefix, ok := indent(at); ok {
			return eol + prefix
		}
		return " "
	}
	// A call range may include its `;`; keep one so a following `(` statement cannot join a replacement.
	semicolon := func(call luasrc.Call) string {
		if source[call.End-1] == ';' {
			return ";"
		}
		return ""
	}
	replace := func(call luasrc.Call, replacement string) {
		edits = append(edits, luaEdit{call.Start, call.End, replacement + semicolon(call)})
	}
	insertAfter := func(call luasrc.Call, lines []string) {
		var inserted strings.Builder
		for _, line := range lines {
			inserted.WriteString(separator(call.Start) + line + semicolon(call))
		}
		edits = append(edits, luaEdit{call.End, call.End, inserted.String()})
	}
	insertBefore := func(at int, lines []string) {
		var inserted strings.Builder
		for _, line := range lines {
			inserted.WriteString(line + separator(at))
		}
		edits = append(edits, luaEdit{at, at, inserted.String()})
	}
	remove := func(call luasrc.Call) {
		rest := restOfLine.FindString(source[call.End:])
		_, wholeLine := indent(call.Start)
		// Drop a whole line only when the next statement starts with a name, which cannot continue an expression.
		if wholeLine && rest != "" && startsWithName.MatchString(source[call.End+len(rest):]) {
			edits = append(edits, luaEdit{lineStart(source, call.Start), call.End + len(rest), ""})
		} else {
			edits = append(edits, luaEdit{call.Start, call.End, ";"})
		}
	}

	if len(infoKeys) > 0 {
		config := global("config")
		for _, key := range infoKeys {
			native, value := "SetMapName", info.Name.Value
			if key == "description" {
				native, value = "SetMapDescription", info.Description.Value
			}
			replace(unique(config, native, 1), native+"("+LuaString(value)+")")
		}
	}

	details := info.Details
	if s.Players.Len() > 0 {
		config := global("config")
		unique(config, "InitCustomPlayerSlots", 0)
		slots := global("InitCustomPlayerSlots")
		for key, values := range s.Players.All() {
			id, _ := strconv.Atoi(key)
			index := slices.IndexFunc(details.Players, func(entry w3i.Player) bool { return int(entry.ID.Value) == id })
			if index < 0 {
				failHint(fmt.Sprintf("player %d does not exist in war3map.w3i.", id),
					"Create this player slot in World Editor first.")
			}
			record := details.Players[index]
			player := fmt.Sprintf("Player(%d)", id)
			start := optional(forPlayer(slots, "SetPlayerStartLocation", 2, id), "SetPlayerStartLocation("+player+")")
			if start == nil || !literalIs(start.Args[1], index) {
				fail(fmt.Sprintf("InitCustomPlayerSlots() must call SetPlayerStartLocation(%s, %d) to match war3map.w3i.",
					player, index))
			}
			var inserted []string
			// A player's name lives only in war3map.w3i: World Editor writes no SetPlayerName, and calling it from
			// config() crashes Warcraft III 3.0.0.24268 when the lobby is created.
			if values.Controller != nil {
				replace(
					one(forPlayer(slots, "SetPlayerController", 2, id), "SetPlayerController("+player+")"),
					"SetPlayerController("+player+", MAP_CONTROL_"+strings.ToUpper(Controllers[record.Controller.Value])+")",
				)
			}
			if values.Race != nil {
				race := Races[record.Race.Value]
				preference := strings.ToUpper(race)
				if race == "selectable" {
					preference = "USER_SELECTABLE"
				}
				replace(
					one(forPlayer(slots, "SetPlayerRacePreference", 2, id), "SetPlayerRacePreference("+player+")"),
					"SetPlayerRacePreference("+player+", RACE_PREF_"+preference+")",
				)
				replace(
					one(forPlayer(slots, "SetPlayerRaceSelectable", 2, id), "SetPlayerRaceSelectable("+player+")"),
					fmt.Sprintf("SetPlayerRaceSelectable(%s, %t)", player, race == "selectable"),
				)
			}
			if values.FixedStart != nil {
				existing := optional(forPlayer(slots, "ForcePlayerStartLocation", 2, id), "ForcePlayerStartLocation("+player+")")
				if existing != nil && !literalIs(existing.Args[1], index) {
					fail(fmt.Sprintf("ForcePlayerStartLocation(%s) must use start location %d to match war3map.w3i.",
						player, index))
				}
				fixed := record.FixedStart.Value != 0
				if fixed && existing == nil {
					inserted = append(inserted, fmt.Sprintf("ForcePlayerStartLocation(%s, %d)", player, index))
				}
				if !fixed && existing != nil {
					remove(*existing)
				}
			}
			if values.X != nil || values.Y != nil {
				var locations []luasrc.Call
				for _, call := range callsNamed(config, "DefineStartLocation", 3) {
					target, ok := luasrc.LiteralNumber(call.Args[0])
					if !ok {
						fail("cannot identify a DefineStartLocation index in config().")
					}
					if target == float64(index) {
						locations = append(locations, call)
					}
				}
				replace(
					one(locations, fmt.Sprintf("DefineStartLocation(%d) in config()", index)),
					fmt.Sprintf("DefineStartLocation(%d, %s, %s)", index,
						text.Number(float64(record.X.Value)), text.Number(float64(record.Y.Value))),
				)
			}
			if len(inserted) > 0 {
				insertAfter(*start, inserted)
			}
		}
	}

	if len(flagged) > 0 {
		unique(global("config"), "InitCustomTeams", 0)
		teams := global("InitCustomTeams")
		type assignment struct {
			player int
			team   float64
		}
		var assignments []assignment
		for _, call := range callsNamed(teams, "SetPlayerTeam", 2) {
			player, okPlayer := luasrc.PlayerID(call.Args[0])
			team, okTeam := luasrc.LiteralNumber(call.Args[1])
			if !okPlayer || !okTeam {
				fail("cannot identify the player and team of a SetPlayerTeam call in InitCustomTeams().")
			}
			assignments = append(assignments, assignment{player, team})
		}
		var appended []string
		for _, key := range flagged {
			index, _ := strconv.Atoi(key)
			if index >= len(details.Forces) {
				failHint(fmt.Sprintf("force %d does not exist in war3map.w3i.", index),
					"Create this force in World Editor first.")
			}
			force := details.Forces[index]
			var members []int
			for _, entry := range details.Players {
				if id := entry.ID.Value; force.Players.Value&(1<<(uint(id)&31)) != 0 {
					members = append(members, int(id))
				}
			}
			for _, assigned := range assignments {
				if (assigned.team == float64(index)) != slices.Contains(members, assigned.player) {
					fail(fmt.Sprintf("SetPlayerTeam(Player(%d), %s) disagrees with force %d in war3map.w3i.",
						assigned.player, text.Number(assigned.team), index))
				}
			}
			for _, id := range members {
				count := 0
				for _, assigned := range assignments {
					if assigned.player == id {
						count++
					}
				}
				if count != 1 {
					fail(fmt.Sprintf("InitCustomTeams() must call SetPlayerTeam(Player(%d), %d) exactly once.", id, index))
				}
			}
			flags := force.Flags.Value
			victory := 0
			if flags&2 != 0 {
				victory = 1
			}
			for _, id := range members {
				appended = append(appended,
					fmt.Sprintf("SetPlayerState(Player(%d), PLAYER_STATE_ALLIED_VICTORY, %d)", id, victory))
			}
			for _, alliance := range []struct {
				native string
				bit    int32
			}{
				{"SetPlayerAllianceStateAllyBJ", 1},
				{"SetPlayerAllianceStateVisionBJ", 8},
				{"SetPlayerAllianceStateControlBJ", 16},
				{"SetPlayerAllianceStateFullControlBJ", 32},
			} {
				for _, a := range members {
					for _, b := range members {
						if a != b {
							appended = append(appended,
								fmt.Sprintf("%s(Player(%d), Player(%d), %t)", alliance.native, a, b, flags&alliance.bit != 0))
						}
					}
				}
			}
		}
		insertBefore(teams.EndStart, appended)
	}

	if !environment.empty() {
		main := global("main")
		anchorIndex := slices.IndexFunc(main.Calls, func(call luasrc.Call) bool {
			return call.Name == "CreateAllUnits" || call.Name == "InitBlizzard"
		})
		if anchorIndex < 0 {
			fail("main() must call CreateAllUnits() or InitBlizzard() directly.")
		}
		anchor := main.Calls[anchorIndex]
		if len(anchor.Args) != 0 {
			fail(anchor.Name + " in main() must have 0 argument(s).")
		}
		var lines []string
		replaced := func(native string, arity int) {
			if existing := optional(callsNamed(main, native, arity), native+" in main()"); existing != nil {
				remove(*existing)
			}
		}
		if environment.SoundEnvironment != nil {
			replaced("NewSoundEnvironment", 1)
			sound := details.SoundEnvironment.Value
			if sound == "" {
				sound = "Default"
			}
			lines = append(lines, "NewSoundEnvironment("+LuaString(sound)+")")
		}
		if environment.WaterColor != nil {
			replaced("SetWaterBaseColor", 4)
			water := details.WaterColor
			lines = append(lines, fmt.Sprintf("SetWaterBaseColor(%d, %d, %d, %d)",
				water[0].Value, water[1].Value, water[2].Value, water[3].Value))
		}
		if environment.Fog != nil {
			// Both are fog initialization: a surviving old reset or setter would undo the override.
			replaced("SetTerrainFogEx", 7)
			replaced("ResetTerrainFog", 0)
			fog := details.Fog
			if info.Flags.Value&0x2000 != 0 {
				channel := func(i int) string { return text.Number(float64(fog.Color[i].Value) / 255) }
				lines = append(lines, fmt.Sprintf("SetTerrainFogEx(%d, %s, %s, %s, %s, %s, %s)", fog.Style.Value,
					text.Number(float64(fog.Start.Value)), text.Number(float64(fog.End.Value)),
					text.Number(float64(fog.Density.Value)), channel(0), channel(1), channel(2)))
			} else {
				lines = append(lines, "ResetTerrainFog()")
			}
		}
		insertBefore(anchor.Start, lines)
	}

	patched, err = applyLuaEdits(source, edits)
	if err != nil {
		return "", err
	}
	if _, err := luasrc.Functions(patched, file); err != nil {
		return "", &diag.Error{
			Msg:   "Cannot apply map settings to Lua: the edited script could not be read back safely.",
			File:  file,
			Hint:  resaveLua,
			Cause: err,
		}
	}
	return patched, nil
}

// KeptMinimap is the name under which a build with a preview picture keeps World Editor's minimap in the map.
const KeptMinimap = "war3mapMinimap.blp"

// PatchMinimapLua adds the call that gives the game World Editor's minimap back, as the last statement of main():
// a build with a preview picture has put that picture in the minimap's place. The call has no effect before World
// Editor's main body has run (probe of 2026-10-01), and gameplay hooks run after main(), so a minimap they set
// still wins.
func PatchMinimapLua(source, file string) (string, error) {
	functions, err := luasrc.Functions(source, file)
	if err != nil {
		return "", err
	}
	var found []luasrc.Function
	for _, function := range functions {
		if function.Name == "main" {
			found = append(found, function)
		}
	}
	if len(found) != 1 {
		return "", &diag.Error{
			Msg:  fmt.Sprintf("Cannot apply map settings to Lua: expected exactly one global function main(), found %d.", len(found)),
			File: file,
			Hint: resaveLua,
		}
	}
	at := found[0].EndStart
	prefix := source[lineStart(source, at):at]
	// On a line of its own when `end` starts its line; otherwise a space keeps it apart from the statement before.
	call := "BlzChangeMinimapTerrainTex(" + LuaString(KeptMinimap) + ")"
	if blank.MatchString(prefix) {
		call += eolOf(source) + prefix
	} else {
		call += " "
	}
	return applyLuaEdits(source, []luaEdit{{at, at, call}})
}
