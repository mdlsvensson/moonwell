package settings

import (
	"fmt"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

var allianceNatives = []struct {
	native string
	bit    int32
}{
	{"SetPlayerAllianceStateAllyBJ", allied},
	{"SetPlayerAllianceStateVisionBJ", sharedVision},
	{"SetPlayerAllianceStateControlBJ", sharedControl},
	{"SetPlayerAllianceStateFullControlBJ", sharedAdvancedControl},
}

type teamCall struct {
	player int
	team   float64
}

func forcesWithFlags(overrides map[int]manifest.Force) []int {
	var slots []int
	for _, slot := range manifest.SortedSlots(overrides) {
		override := overrides[slot]
		override.Name = nil
		if override != (manifest.Force{}) {
			slots = append(slots, slot)
		}
	}
	return slots
}

func (p *luaPatcher) patchForces(overrides map[int]manifest.Force, details *w3i.Details) {
	indexes := forcesWithFlags(overrides)
	if len(indexes) == 0 {
		return
	}
	p.findOneCall(p.findFunction("config"), "InitCustomTeams", 0)
	teams := p.findFunction("InitCustomTeams")
	calls := p.findTeamCalls(teams)
	var states []string
	for _, index := range indexes {
		states = append(states, p.forceStatements(index, details, calls)...)
	}
	p.insertBefore(teams.EndStart, states)
}

func (p *luaPatcher) findTeamCalls(teams lua.Function) []teamCall {
	var calls []teamCall
	for _, call := range p.findCalls(teams, "SetPlayerTeam", 2) {
		player, isPlayer := lua.ParsePlayerID(call.Args[0])
		team, isTeam := lua.ParseNumberLiteral(call.Args[1])
		if !isPlayer || !isTeam {
			p.fail(errUnknownTeam(p.displayPath))
			return nil
		}
		calls = append(calls, teamCall{player, team})
	}
	return calls
}

func (p *luaPatcher) forceStatements(index int, details *w3i.Details, calls []teamCall) []string {
	if index >= len(details.Forces) {
		p.fail(errForceNotInInfo(p.displayPath, index))
		return nil
	}
	force := details.Forces[index]
	members := membersOf(force, details.Players)
	if !p.isTeamConsistent(index, members, calls) {
		return nil
	}
	return allianceStatements(members, force.Flags.Value)
}

func membersOf(force w3i.Force, players []w3i.Player) []int {
	var members []int
	for _, player := range players {
		if id := player.ID.Value; force.Players.Value&(1<<uint(id)) != 0 {
			members = append(members, int(id))
		}
	}
	return members
}

func (p *luaPatcher) isTeamConsistent(index int, members []int, calls []teamCall) bool {
	for _, call := range calls {
		if (call.team == float64(index)) != slices.Contains(members, call.player) {
			p.fail(errTeamDisagrees(p.displayPath, call, index))
			return false
		}
	}
	for _, member := range members {
		ofMember := func(call teamCall) bool { return call.player == member }
		if first := slices.IndexFunc(calls, ofMember); first < 0 || slices.ContainsFunc(calls[first+1:], ofMember) {
			p.fail(errTeamCount(p.displayPath, member, index))
			return false
		}
	}
	return !p.hasFailed()
}

func allianceStatements(members []int, flags int32) []string {
	victory := 0
	if flags&alliedVictory != 0 {
		victory = 1
	}
	var calls []string
	for _, member := range members {
		calls = append(calls, fmt.Sprintf("SetPlayerState(Player(%d), PLAYER_STATE_ALLIED_VICTORY, %d)", member, victory))
	}
	for _, alliance := range allianceNatives {
		for _, a := range members {
			for _, b := range members {
				if a != b {
					calls = append(calls, fmt.Sprintf("%s(Player(%d), Player(%d), %t)", alliance.native, a, b, flags&alliance.bit != 0))
				}
			}
		}
	}
	return calls
}

func errForceNotInInfo(displayPath string, index int) error {
	return errLuaHint(displayPath, fmt.Sprintf("force %d does not exist in war3map.w3i.", index),
		"Create this force in World Editor first.")
}

func errUnknownTeam(displayPath string) error {
	return errLua(displayPath, "cannot identify the player and team of a SetPlayerTeam call in InitCustomTeams().")
}

func errTeamDisagrees(displayPath string, call teamCall, index int) error {
	return errLua(displayPath, fmt.Sprintf("SetPlayerTeam(Player(%d), %s) disagrees with force %d in war3map.w3i.",
		call.player, lua.FormatNumber(call.team), index))
}

func errTeamCount(displayPath string, member, index int) error {
	return errLua(displayPath, fmt.Sprintf("InitCustomTeams() must call SetPlayerTeam(Player(%d), %d) exactly once.", member, index))
}
