package settings

import (
	"fmt"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

// allianceNatives are the natives that set what the players of a force share, each with the bit of the force's
// flags it follows.
var allianceNatives = []struct {
	native string
	bit    int32
}{
	{"SetPlayerAllianceStateAllyBJ", allied},
	{"SetPlayerAllianceStateVisionBJ", sharedVision},
	{"SetPlayerAllianceStateControlBJ", sharedControl},
	{"SetPlayerAllianceStateFullControlBJ", sharedAdvancedControl},
}

// teamCall is a SetPlayerTeam call of the script: the player it puts into a team, and the team.
type teamCall struct {
	player int
	team   float64
}

// flagged is the slots, in their order, of the forces whose override sets an alliance flag. A force's name is
// stored in the map info alone.
func flagged(overrides map[int]manifest.Force) []int {
	var slots []int
	for _, slot := range manifest.Slots(overrides) {
		override := overrides[slot]
		override.Name = nil
		if override != (manifest.Force{}) {
			slots = append(slots, slot)
		}
	}
	return slots
}

// forces adds, at the end of InitCustomTeams(), the states of the players of each force whose override sets an
// alliance flag: every state the map info has for the force, also those the override leaves alone. They stand
// after the calls World Editor wrote, so they are the ones that hold.
func (p *patcher) forces(overrides map[int]manifest.Force, details *w3i.Details) {
	indexes := flagged(overrides)
	if len(indexes) == 0 {
		return
	}
	p.unique(p.function("config"), "InitCustomTeams", 0)
	teams := p.function("InitCustomTeams")
	calls := p.teamCalls(teams)
	var states []string
	for _, index := range indexes {
		states = append(states, p.force(index, details, calls)...)
	}
	p.insertBefore(teams.EndStart, states)
}

// teamCalls is the SetPlayerTeam calls of InitCustomTeams(). Each must name its player and its team as literals.
func (p *patcher) teamCalls(teams lua.Function) []teamCall {
	var calls []teamCall
	for _, call := range p.callsNamed(teams, "SetPlayerTeam", 2) {
		player, isPlayer := lua.PlayerID(call.Args[0])
		team, isTeam := lua.LiteralNumber(call.Args[1])
		if !isPlayer || !isTeam {
			p.refuse(errUnknownTeam(p.file))
			return nil
		}
		calls = append(calls, teamCall{player, team})
	}
	return calls
}

// force is the states of the force in a slot, which is its place among the map info's forces, after it is seen
// that the script puts into the force's team the players the map info has in the force, and no other.
func (p *patcher) force(index int, details *w3i.Details, calls []teamCall) []string {
	if index >= len(details.Forces) {
		p.refuse(errForceNotInInfo(p.file, index))
		return nil
	}
	force := details.Forces[index]
	members := membersOf(force, details.Players)
	if !p.teamed(index, members, calls) {
		return nil
	}
	return states(members, force.Flags.Value)
}

// membersOf is the ids of the map's players that the force's mask has, in the order of the map info. The mask has
// a bit for each slot, and a map info that w3i.Read accepts has no player outside the 24 slots.
func membersOf(force w3i.Force, players []w3i.Player) []int {
	var members []int
	for _, player := range players {
		if id := player.ID.Value; force.Players.Value&(1<<uint(id)) != 0 {
			members = append(members, int(id))
		}
	}
	return members
}

// teamed reports whether the script's SetPlayerTeam calls agree with the force: a call puts a player into the
// team of the force's number exactly when the player is a member, and each member has exactly one call.
func (p *patcher) teamed(index int, members []int, calls []teamCall) bool {
	for _, call := range calls {
		if (call.team == float64(index)) != slices.Contains(members, call.player) {
			p.refuse(errTeamDisagrees(p.file, call, index))
			return false
		}
	}
	for _, member := range members {
		ofMember := func(call teamCall) bool { return call.player == member }
		if first := slices.IndexFunc(calls, ofMember); first < 0 || slices.ContainsFunc(calls[first+1:], ofMember) {
			p.refuse(errTeamCount(p.file, member, index))
			return false
		}
	}
	return !p.failed()
}

// states is the calls that give the members of a force the states its flags say: for each member whether an
// allied victory counts, and for each pair of members, both ways, what the two share.
func states(members []int, flags int32) []string {
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

// ---- errors ----

func errForceNotInInfo(file string, index int) error {
	return errLuaHint(file, fmt.Sprintf("force %d does not exist in war3map.w3i.", index),
		"Create this force in World Editor first.")
}

func errUnknownTeam(file string) error {
	return errLua(file, "cannot identify the player and team of a SetPlayerTeam call in InitCustomTeams().")
}

func errTeamDisagrees(file string, call teamCall, index int) error {
	return errLua(file, fmt.Sprintf("SetPlayerTeam(Player(%d), %s) disagrees with force %d in war3map.w3i.",
		call.player, lua.Number(call.team), index))
}

func errTeamCount(file string, member, index int) error {
	return errLua(file, fmt.Sprintf("InitCustomTeams() must call SetPlayerTeam(Player(%d), %d) exactly once.", member, index))
}
