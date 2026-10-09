package settings

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

func (p *luaPatcher) patchPlayers(overrides map[int]manifest.Player, details *w3i.Details) {
	if !hasOverrides(overrides) {
		return
	}
	config := p.findFunction("config")
	p.findOneCall(config, "InitCustomPlayerSlots", 0)
	initSlots := p.findFunction("InitCustomPlayerSlots")
	for _, id := range manifest.SortedSlots(overrides) {
		if override := overrides[id]; override != (manifest.Player{}) {
			p.patchPlayer(config, initSlots, override, id, details)
		}
	}
}

func (p *luaPatcher) patchPlayer(config, initSlots lua.Function, override manifest.Player, id int, details *w3i.Details) {
	slot, found := p.findSlot(id, details)
	if !found {
		return
	}
	start := p.findStartLocationCall(initSlots, slot)
	if override.Controller != nil {
		p.patchController(initSlots, slot)
	}
	if override.Race != nil {
		p.patchRace(initSlots, slot)
	}
	if override.FixedStart != nil {
		p.patchFixedStart(initSlots, slot, start)
	}
	if override.X != nil || override.Y != nil {
		p.patchPosition(config, slot)
	}
}

type playerSlot struct {
	id            int
	luaPlayer     string
	startLocation int
	infoPlayer    w3i.Player
}

func (p *luaPatcher) findSlot(id int, details *w3i.Details) (playerSlot, bool) {
	index := slices.IndexFunc(details.Players, func(player w3i.Player) bool { return int(player.ID.Value) == id })
	if index < 0 {
		p.fail(errPlayerNotInInfo(p.displayPath, id))
		return playerSlot{}, false
	}
	return playerSlot{id: id, luaPlayer: fmt.Sprintf("Player(%d)", id), startLocation: index, infoPlayer: details.Players[index]}, true
}

func (p *luaPatcher) findStartLocationCall(initSlots lua.Function, slot playerSlot) lua.Call {
	start, found := p.atMostOne(p.findPlayerCalls(initSlots, "SetPlayerStartLocation", 2, slot.id), "SetPlayerStartLocation("+slot.luaPlayer+")")
	if !found || !isIntLiteral(start.Args[1], slot.startLocation) {
		p.fail(errStartLocation(p.displayPath, slot))
		return lua.Call{}
	}
	return start
}

func (p *luaPatcher) patchController(initSlots lua.Function, slot playerSlot) {
	controller, known := nameOf(controllers, slot.infoPlayer.Controller.Value)
	if !known {
		p.fail(errNoController(p.displayPath, slot))
		return
	}
	p.replace(
		p.requireOne(p.findPlayerCalls(initSlots, "SetPlayerController", 2, slot.id), "SetPlayerController("+slot.luaPlayer+")"),
		"SetPlayerController("+slot.luaPlayer+", MAP_CONTROL_"+strings.ToUpper(controller)+")",
	)
}

func nameOf(names []string, number int32) (string, bool) {
	if number < 0 || int(number) >= len(names) || names[number] == "" {
		return "", false
	}
	return names[number], true
}

func (p *luaPatcher) patchRace(initSlots lua.Function, slot playerSlot) {
	race, known := nameOf(races, slot.infoPlayer.Race.Value)
	if !known {
		p.fail(errNoRace(p.displayPath, slot))
		return
	}
	preference := strings.ToUpper(race)
	if race == "selectable" {
		preference = "USER_SELECTABLE"
	}
	p.replace(
		p.requireOne(p.findPlayerCalls(initSlots, "SetPlayerRacePreference", 2, slot.id), "SetPlayerRacePreference("+slot.luaPlayer+")"),
		"SetPlayerRacePreference("+slot.luaPlayer+", RACE_PREF_"+preference+")",
	)
	p.replace(
		p.requireOne(p.findPlayerCalls(initSlots, "SetPlayerRaceSelectable", 2, slot.id), "SetPlayerRaceSelectable("+slot.luaPlayer+")"),
		fmt.Sprintf("SetPlayerRaceSelectable(%s, %t)", slot.luaPlayer, race == "selectable"),
	)
}

func (p *luaPatcher) patchFixedStart(initSlots lua.Function, slot playerSlot, start lua.Call) {
	description := "ForcePlayerStartLocation(" + slot.luaPlayer + ")"
	forced, found := p.atMostOne(p.findPlayerCalls(initSlots, "ForcePlayerStartLocation", 2, slot.id), description)
	if found && !isIntLiteral(forced.Args[1], slot.startLocation) {
		p.fail(errForcedElsewhere(p.displayPath, slot))
		return
	}
	switch fixed := slot.infoPlayer.FixedStart.Value != 0; {
	case fixed && !found:
		p.insertAfter(start, []string{fmt.Sprintf("ForcePlayerStartLocation(%s, %d)", slot.luaPlayer, slot.startLocation)})
	case !fixed && found:
		p.remove(forced)
	}
}

func (p *luaPatcher) patchPosition(config lua.Function, slot playerSlot) {
	if !areFinite(slot.infoPlayer.X.Value, slot.infoPlayer.Y.Value) {
		p.fail(errNoPosition(p.displayPath, slot))
		return
	}
	var locations []lua.Call
	for _, call := range p.findCalls(config, "DefineStartLocation", 3) {
		number, ok := lua.ParseNumberLiteral(call.Args[0])
		if !ok {
			p.fail(errUnknownLocation(p.displayPath))
			return
		}
		if number == float64(slot.startLocation) {
			locations = append(locations, call)
		}
	}
	p.replace(
		p.requireOne(locations, fmt.Sprintf("DefineStartLocation(%d) in config()", slot.startLocation)),
		fmt.Sprintf("DefineStartLocation(%d, %s, %s)", slot.startLocation,
			lua.FormatNumber(float64(slot.infoPlayer.X.Value)), lua.FormatNumber(float64(slot.infoPlayer.Y.Value))),
	)
}

func errPlayerNotInInfo(displayPath string, id int) error {
	return errLuaHint(displayPath, fmt.Sprintf("player %d does not exist in war3map.w3i.", id),
		"Create this player slot in World Editor first.")
}

func errStartLocation(displayPath string, slot playerSlot) error {
	return errLua(displayPath, fmt.Sprintf("InitCustomPlayerSlots() must call SetPlayerStartLocation(%s, %d) to match war3map.w3i.",
		slot.luaPlayer, slot.startLocation))
}

func errForcedElsewhere(displayPath string, slot playerSlot) error {
	return errLua(displayPath, fmt.Sprintf("ForcePlayerStartLocation(%s) must use start location %d to match war3map.w3i.",
		slot.luaPlayer, slot.startLocation))
}

func errUnknownLocation(displayPath string) error {
	return errLua(displayPath, "cannot identify a DefineStartLocation index in config().")
}

func errNoController(displayPath string, slot playerSlot) error {
	return errLuaHint(displayPath, fmt.Sprintf("player %d has controller %d in the map info, which is not a controller.",
		slot.id, slot.infoPlayer.Controller.Value), resaveInfo)
}

func errNoRace(displayPath string, slot playerSlot) error {
	return errLuaHint(displayPath, fmt.Sprintf("player %d has race %d in the map info, which is not a race.",
		slot.id, slot.infoPlayer.Race.Value), resaveInfo)
}

func errNoPosition(displayPath string, slot playerSlot) error {
	return errLuaHint(displayPath, fmt.Sprintf("player %d has a start position in the map info that is not a number.", slot.id),
		resaveInfo)
}
