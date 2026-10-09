package settings

import (
	"fmt"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

func (p *luaPatcher) patchForces(overrides map[int]manifest.Force, details *w3i.Details) {
	slots := forcesWithFlags(overrides)
	if len(slots) == 0 {
		return
	}
	p.findOneCall(p.findFunction("config"), "InitCustomTeams", 0)
	initTeams := p.findFunction("InitCustomTeams")
	teamCalls := p.findTeamCalls(initTeams)
	var statements []string
	for _, slot := range slots {
		statements = append(statements, p.forceStatements(slot, details, teamCalls)...)
	}
	p.insertBefore(initTeams.EndStart, statements)
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

type teamCall struct {
	player int
	team   float64
}

func (p *luaPatcher) findTeamCalls(initTeams lua.Function) []teamCall {
	var calls []teamCall
	for _, call := range p.findCalls(initTeams, "SetPlayerTeam", 2) {
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

func (p *luaPatcher) forceStatements(slot int, details *w3i.Details, calls []teamCall) []string {
	if slot >= len(details.Forces) {
		p.fail(errForceNotInInfo(p.displayPath, slot))
		return nil
	}
	force := details.Forces[slot]
	members := membersOf(force, details.Players)
	if !p.isTeamConsistent(slot, members, calls) {
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

func (p *luaPatcher) isTeamConsistent(slot int, members []int, calls []teamCall) bool {
	for _, call := range calls {
		if (call.team == float64(slot)) != slices.Contains(members, call.player) {
			p.fail(errTeamDisagrees(p.displayPath, call, slot))
			return false
		}
	}
	for _, member := range members {
		ofMember := func(call teamCall) bool { return call.player == member }
		if first := slices.IndexFunc(calls, ofMember); first < 0 || slices.ContainsFunc(calls[first+1:], ofMember) {
			p.fail(errTeamCount(p.displayPath, member, slot))
			return false
		}
	}
	return !p.hasFailed()
}

var allianceNatives = []struct {
	native string
	bit    int32
}{
	{"SetPlayerAllianceStateAllyBJ", allied},
	{"SetPlayerAllianceStateVisionBJ", sharedVision},
	{"SetPlayerAllianceStateControlBJ", sharedControl},
	{"SetPlayerAllianceStateFullControlBJ", sharedAdvancedControl},
}

func allianceStatements(members []int, flags int32) []string {
	victory := 0
	if flags&alliedVictory != 0 {
		victory = 1
	}
	var statements []string
	for _, member := range members {
		statements = append(statements, fmt.Sprintf("SetPlayerState(Player(%d), PLAYER_STATE_ALLIED_VICTORY, %d)", member, victory))
	}
	for _, alliance := range allianceNatives {
		enabled := flags&alliance.bit != 0
		for _, pair := range memberPairs(members) {
			statements = append(statements,
				fmt.Sprintf("%s(Player(%d), Player(%d), %t)", alliance.native, pair.from, pair.to, enabled))
		}
	}
	return statements
}

type memberPair struct{ from, to int }

func memberPairs(members []int) []memberPair {
	var pairs []memberPair
	for _, from := range members {
		for _, to := range members {
			if from != to {
				pairs = append(pairs, memberPair{from, to})
			}
		}
	}
	return pairs
}

func errForceNotInInfo(displayPath string, slot int) error {
	return errLuaHint(displayPath, fmt.Sprintf("force %d does not exist in war3map.w3i.", slot),
		"Create this force in World Editor first.")
}

func errUnknownTeam(displayPath string) error {
	return errLua(displayPath, "cannot identify the player and team of a SetPlayerTeam call in InitCustomTeams().")
}

func errTeamDisagrees(displayPath string, call teamCall, slot int) error {
	return errLua(displayPath, fmt.Sprintf("SetPlayerTeam(Player(%d), %s) disagrees with force %d in war3map.w3i.",
		call.player, lua.FormatNumber(call.team), slot))
}

func errTeamCount(displayPath string, member, slot int) error {
	return errLua(displayPath, fmt.Sprintf("InitCustomTeams() must call SetPlayerTeam(Player(%d), %d) exactly once.", member, slot))
}
