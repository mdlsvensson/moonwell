package settings

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

// slot is one player of the map as the Lua must show it.
type slot struct {
	id       int
	player   string // the player as the Lua writes it: `Player(3)`
	location int    // its start location, which is the place of its record among the map info's players
	record   w3i.Player
}

// players brings InitCustomPlayerSlots() and the start locations of config() into line with the map info, for
// the player of each override that sets something, in slot order.
//
// A player's name is stored in the map info alone: World Editor writes no SetPlayerName, and a call of it from
// config() crashes Warcraft III 3.0.0.24268 when the lobby is created. An override that sets only a name still
// asks that the Lua has the player as the map info has it.
func (p *patcher) players(overrides map[int]manifest.Player, details *w3i.Details) {
	if !anySet(overrides) {
		return
	}
	config := p.function("config")
	p.unique(config, "InitCustomPlayerSlots", 0)
	slots := p.function("InitCustomPlayerSlots")
	for _, id := range manifest.Slots(overrides) {
		if override := overrides[id]; override != (manifest.Player{}) {
			p.player(config, slots, override, id, details)
		}
	}
}

// player edits the calls of one player: those the override has a setting for.
func (p *patcher) player(config, slots lua.Function, override manifest.Player, id int, details *w3i.Details) {
	s, found := p.slot(id, details)
	if !found {
		return
	}
	start := p.startLocation(slots, s)
	if override.Controller != nil {
		p.controller(slots, s)
	}
	if override.Race != nil {
		p.race(slots, s)
	}
	if override.FixedStart != nil {
		p.fixedStart(slots, s, start)
	}
	if override.X != nil || override.Y != nil {
		p.position(config, s)
	}
}

// slot is the map's player in the slot of the id. A setting for a player the map info lacks is refused.
func (p *patcher) slot(id int, details *w3i.Details) (slot, bool) {
	at := slices.IndexFunc(details.Players, func(player w3i.Player) bool { return int(player.ID.Value) == id })
	if at < 0 {
		p.refuse(errPlayerNotInInfo(p.file, id))
		return slot{}, false
	}
	return slot{id: id, player: fmt.Sprintf("Player(%d)", id), location: at, record: details.Players[at]}, true
}

// startLocation is the call that gives the player its start location, which must be the one the map info gives
// it: every other call of the player is found by the same id, and a start location is edited by its number.
func (p *patcher) startLocation(slots lua.Function, s slot) lua.Call {
	start, found := p.optional(p.forPlayer(slots, "SetPlayerStartLocation", 2, s.id), "SetPlayerStartLocation("+s.player+")")
	if !found || !literalIs(start.Args[1], s.location) {
		p.refuse(errStartLocation(p.file, s))
		return lua.Call{}
	}
	return start
}

// controller writes who controls the player.
func (p *patcher) controller(slots lua.Function, s slot) {
	controller, known := named(controllers, s.record.Controller.Value)
	if !known {
		p.refuse(errNoController(p.file, s))
		return
	}
	p.replace(
		p.one(p.forPlayer(slots, "SetPlayerController", 2, s.id), "SetPlayerController("+s.player+")"),
		"SetPlayerController("+s.player+", MAP_CONTROL_"+strings.ToUpper(controller)+")",
	)
}

// named is the name a number of the map info has, which is the name in its place among names, and whether it has
// one. The empty name stands where a number has none.
func named(names []string, number int32) (string, bool) {
	if number < 0 || int(number) >= len(names) || names[number] == "" {
		return "", false
	}
	return names[number], true
}

// race writes the race the player prefers, and whether the player may choose another.
func (p *patcher) race(slots lua.Function, s slot) {
	race, known := named(races, s.record.Race.Value)
	if !known {
		p.refuse(errNoRace(p.file, s))
		return
	}
	preference := strings.ToUpper(race)
	if race == "selectable" {
		preference = "USER_SELECTABLE"
	}
	p.replace(
		p.one(p.forPlayer(slots, "SetPlayerRacePreference", 2, s.id), "SetPlayerRacePreference("+s.player+")"),
		"SetPlayerRacePreference("+s.player+", RACE_PREF_"+preference+")",
	)
	p.replace(
		p.one(p.forPlayer(slots, "SetPlayerRaceSelectable", 2, s.id), "SetPlayerRaceSelectable("+s.player+")"),
		fmt.Sprintf("SetPlayerRaceSelectable(%s, %t)", s.player, race == "selectable"),
	)
}

// fixedStart adds the call that holds the player to its start location, after the call that gives it one, or
// takes the call out, as the map info says. A call that is there and right is kept.
func (p *patcher) fixedStart(slots lua.Function, s slot, start lua.Call) {
	label := "ForcePlayerStartLocation(" + s.player + ")"
	forced, found := p.optional(p.forPlayer(slots, "ForcePlayerStartLocation", 2, s.id), label)
	if found && !literalIs(forced.Args[1], s.location) {
		p.refuse(errForcedElsewhere(p.file, s))
		return
	}
	switch fixed := s.record.FixedStart.Value != 0; {
	case fixed && !found:
		p.insertAfter(start, []string{fmt.Sprintf("ForcePlayerStartLocation(%s, %d)", s.player, s.location)})
	case !fixed && found:
		p.remove(forced)
	}
}

// position writes where the player's start location is. Both coordinates are the map info's, also the one the
// override does not set, so both must be numbers.
func (p *patcher) position(config lua.Function, s slot) {
	if !finite(s.record.X.Value, s.record.Y.Value) {
		p.refuse(errNoPosition(p.file, s))
		return
	}
	var locations []lua.Call
	for _, call := range p.callsNamed(config, "DefineStartLocation", 3) {
		number, ok := lua.LiteralNumber(call.Args[0])
		if !ok {
			p.refuse(errUnknownLocation(p.file))
			return
		}
		if number == float64(s.location) {
			locations = append(locations, call)
		}
	}
	p.replace(
		p.one(locations, fmt.Sprintf("DefineStartLocation(%d) in config()", s.location)),
		fmt.Sprintf("DefineStartLocation(%d, %s, %s)", s.location,
			lua.Number(float64(s.record.X.Value)), lua.Number(float64(s.record.Y.Value))),
	)
}

// ---- errors ----

func errPlayerNotInInfo(file string, id int) error {
	return errLuaHint(file, fmt.Sprintf("player %d does not exist in war3map.w3i.", id),
		"Create this player slot in World Editor first.")
}

func errStartLocation(file string, s slot) error {
	return errLua(file, fmt.Sprintf("InitCustomPlayerSlots() must call SetPlayerStartLocation(%s, %d) to match war3map.w3i.",
		s.player, s.location))
}

func errForcedElsewhere(file string, s slot) error {
	return errLua(file, fmt.Sprintf("ForcePlayerStartLocation(%s) must use start location %d to match war3map.w3i.",
		s.player, s.location))
}

func errUnknownLocation(file string) error {
	return errLua(file, "cannot identify a DefineStartLocation index in config().")
}

func errNoController(file string, s slot) error {
	return errLuaHint(file, fmt.Sprintf("player %d has controller %d in the map info, which is not a controller.",
		s.id, s.record.Controller.Value), resaveInfo)
}

func errNoRace(file string, s slot) error {
	return errLuaHint(file, fmt.Sprintf("player %d has race %d in the map info, which is not a race.",
		s.id, s.record.Race.Value), resaveInfo)
}

func errNoPosition(file string, s slot) error {
	return errLuaHint(file, fmt.Sprintf("player %d has a start position in the map info that is not a number.", s.id),
		resaveInfo)
}
