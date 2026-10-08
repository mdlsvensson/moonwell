package settings

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

type playerSlot struct {
	id       int
	player   string
	location int
	record   w3i.Player
}

func (p *luaPatcher) patchPlayers(overrides map[int]manifest.Player, details *w3i.Details) {
	if !hasOverrides(overrides) {
		return
	}
	config := p.findFunction("config")
	p.findOneCall(config, "InitCustomPlayerSlots", 0)
	slots := p.findFunction("InitCustomPlayerSlots")
	for _, id := range manifest.SortedSlots(overrides) {
		if override := overrides[id]; override != (manifest.Player{}) {
			p.patchPlayer(config, slots, override, id, details)
		}
	}
}

func (p *luaPatcher) patchPlayer(config, slots lua.Function, override manifest.Player, id int, details *w3i.Details) {
	s, found := p.findSlot(id, details)
	if !found {
		return
	}
	start := p.findStartLocationCall(slots, s)
	if override.Controller != nil {
		p.patchController(slots, s)
	}
	if override.Race != nil {
		p.patchRace(slots, s)
	}
	if override.FixedStart != nil {
		p.patchFixedStart(slots, s, start)
	}
	if override.X != nil || override.Y != nil {
		p.patchPosition(config, s)
	}
}

func (p *luaPatcher) findSlot(id int, details *w3i.Details) (playerSlot, bool) {
	at := slices.IndexFunc(details.Players, func(player w3i.Player) bool { return int(player.ID.Value) == id })
	if at < 0 {
		p.fail(errPlayerNotInInfo(p.displayPath, id))
		return playerSlot{}, false
	}
	return playerSlot{id: id, player: fmt.Sprintf("Player(%d)", id), location: at, record: details.Players[at]}, true
}

func (p *luaPatcher) findStartLocationCall(slots lua.Function, s playerSlot) lua.Call {
	start, found := p.atMostOne(p.findPlayerCalls(slots, "SetPlayerStartLocation", 2, s.id), "SetPlayerStartLocation("+s.player+")")
	if !found || !isIntLiteral(start.Args[1], s.location) {
		p.fail(errStartLocation(p.displayPath, s))
		return lua.Call{}
	}
	return start
}

func (p *luaPatcher) patchController(slots lua.Function, s playerSlot) {
	controller, known := nameOf(controllers, s.record.Controller.Value)
	if !known {
		p.fail(errNoController(p.displayPath, s))
		return
	}
	p.replace(
		p.requireOne(p.findPlayerCalls(slots, "SetPlayerController", 2, s.id), "SetPlayerController("+s.player+")"),
		"SetPlayerController("+s.player+", MAP_CONTROL_"+strings.ToUpper(controller)+")",
	)
}

func nameOf(names []string, number int32) (string, bool) {
	if number < 0 || int(number) >= len(names) || names[number] == "" {
		return "", false
	}
	return names[number], true
}

func (p *luaPatcher) patchRace(slots lua.Function, s playerSlot) {
	race, known := nameOf(races, s.record.Race.Value)
	if !known {
		p.fail(errNoRace(p.displayPath, s))
		return
	}
	preference := strings.ToUpper(race)
	if race == "selectable" {
		preference = "USER_SELECTABLE"
	}
	p.replace(
		p.requireOne(p.findPlayerCalls(slots, "SetPlayerRacePreference", 2, s.id), "SetPlayerRacePreference("+s.player+")"),
		"SetPlayerRacePreference("+s.player+", RACE_PREF_"+preference+")",
	)
	p.replace(
		p.requireOne(p.findPlayerCalls(slots, "SetPlayerRaceSelectable", 2, s.id), "SetPlayerRaceSelectable("+s.player+")"),
		fmt.Sprintf("SetPlayerRaceSelectable(%s, %t)", s.player, race == "selectable"),
	)
}

func (p *luaPatcher) patchFixedStart(slots lua.Function, s playerSlot, start lua.Call) {
	label := "ForcePlayerStartLocation(" + s.player + ")"
	forced, found := p.atMostOne(p.findPlayerCalls(slots, "ForcePlayerStartLocation", 2, s.id), label)
	if found && !isIntLiteral(forced.Args[1], s.location) {
		p.fail(errForcedElsewhere(p.displayPath, s))
		return
	}
	switch fixed := s.record.FixedStart.Value != 0; {
	case fixed && !found:
		p.insertAfter(start, []string{fmt.Sprintf("ForcePlayerStartLocation(%s, %d)", s.player, s.location)})
	case !fixed && found:
		p.remove(forced)
	}
}

func (p *luaPatcher) patchPosition(config lua.Function, s playerSlot) {
	if !areFinite(s.record.X.Value, s.record.Y.Value) {
		p.fail(errNoPosition(p.displayPath, s))
		return
	}
	var locations []lua.Call
	for _, call := range p.findCalls(config, "DefineStartLocation", 3) {
		number, ok := lua.ParseNumberLiteral(call.Args[0])
		if !ok {
			p.fail(errUnknownLocation(p.displayPath))
			return
		}
		if number == float64(s.location) {
			locations = append(locations, call)
		}
	}
	p.replace(
		p.requireOne(locations, fmt.Sprintf("DefineStartLocation(%d) in config()", s.location)),
		fmt.Sprintf("DefineStartLocation(%d, %s, %s)", s.location,
			lua.FormatNumber(float64(s.record.X.Value)), lua.FormatNumber(float64(s.record.Y.Value))),
	)
}

func errPlayerNotInInfo(displayPath string, id int) error {
	return errLuaHint(displayPath, fmt.Sprintf("player %d does not exist in war3map.w3i.", id),
		"Create this player slot in World Editor first.")
}

func errStartLocation(displayPath string, s playerSlot) error {
	return errLua(displayPath, fmt.Sprintf("InitCustomPlayerSlots() must call SetPlayerStartLocation(%s, %d) to match war3map.w3i.",
		s.player, s.location))
}

func errForcedElsewhere(displayPath string, s playerSlot) error {
	return errLua(displayPath, fmt.Sprintf("ForcePlayerStartLocation(%s) must use start location %d to match war3map.w3i.",
		s.player, s.location))
}

func errUnknownLocation(displayPath string) error {
	return errLua(displayPath, "cannot identify a DefineStartLocation index in config().")
}

func errNoController(displayPath string, s playerSlot) error {
	return errLuaHint(displayPath, fmt.Sprintf("player %d has controller %d in the map info, which is not a controller.",
		s.id, s.record.Controller.Value), resaveInfo)
}

func errNoRace(displayPath string, s playerSlot) error {
	return errLuaHint(displayPath, fmt.Sprintf("player %d has race %d in the map info, which is not a race.",
		s.id, s.record.Race.Value), resaveInfo)
}

func errNoPosition(displayPath string, s playerSlot) error {
	return errLuaHint(displayPath, fmt.Sprintf("player %d has a start position in the map info that is not a number.", s.id),
		resaveInfo)
}
