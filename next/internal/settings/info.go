// Package settings applies the manifest's map settings to a map folder: it patches war3map.w3i, brings
// war3map.lua into line with it, merges the two settings text files, and puts a preview picture in the
// minimap's place. It takes a project and a map folder and returns changes; it writes nothing.
//
// The shape of the settings is Pkl's to check. What is checked here is what Pkl cannot see: what a setting needs
// of the map it is for, a typed gameplay constant against a raw one, and names that differ only in letter case.
// The package must not know how a map is built or launched, nor the object data and the imported files of a map.
package settings

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/war3/w3i"
)

// The player controllers and races by their number in war3map.w3i; controller 0 is none.
var controllers = []string{"", "user", "computer", "neutral", "rescuable"}
var races = []string{"selectable", "human", "orc", "undead", "nightelf"}

// The map flags that a setting reads or turns.
const (
	customForces = 0x40    // the map has forces of its own, which a force override needs
	fogOn        = 0x2000  // the terrain fog is shown
	waterTinted  = 0x10000 // the water has a colour of its own
)

// The bits of a force's flags.
const (
	allied                = 1
	alliedVictory         = 2
	sharedVision          = 8
	sharedControl         = 16
	sharedAdvancedControl = 32
)

// patchInfo replaces the fields of a war3map.w3i that the settings set, and keeps every other byte. file is the
// name errors give. Without a setting that is stored in the map info, data is returned as it is and is not read.
func patchInfo(data []byte, s manifest.Settings, file string) ([]byte, error) {
	if !setsInfo(s) {
		return data, nil
	}
	info, err := w3i.Read(data, file, depthFor(s))
	if err != nil {
		return nil, err
	}
	p := &infoPatch{info: info, file: file, flags: info.Flags.Value}
	p.description(s.Info)
	p.loadingScreen(s.LoadingScreen)
	p.players(s.Players)
	p.forces(s.Forces)
	p.environment(s.Environment)
	p.mapFlags()
	if p.refusal != nil {
		return nil, p.refusal
	}
	patched, err := w3i.ApplyEdits(data, p.edits)
	if err != nil {
		return nil, fmt.Errorf("patching %s: %w", file, err)
	}
	return patched, nil
}

// setsInfo reports whether a setting is stored in war3map.w3i.
func setsInfo(s manifest.Settings) bool {
	return needsDetails(s) || described(s.Info) || s.LoadingScreen != (manifest.LoadingScreen{})
}

// described reports whether a part of the map's description is set. The preview is not one: it is a file of its
// own in the map.
func described(info manifest.Info) bool {
	info.Preview = nil
	return info != (manifest.Info{})
}

// needsDetails reports whether a setting is stored among the players, the forces or the environment of
// war3map.w3i, which only a file of version 28 or later in Lua mode has.
func needsDetails(s manifest.Settings) bool {
	return anySet(s.Players) || anySet(s.Forces) || s.Environment != (manifest.Environment{})
}

// anySet reports whether one of the overrides sets something. An override with nothing set is skipped everywhere,
// so its slot need not exist in the map.
func anySet[V comparable](overrides map[string]V) bool {
	var nothing V
	for _, override := range overrides {
		if override != nothing {
			return true
		}
	}
	return false
}

// depthFor is how much of the file the settings need read.
func depthFor(s manifest.Settings) w3i.Depth {
	if needsDetails(s) {
		return w3i.Extended
	}
	return w3i.Basic
}

// infoPatch gathers the edits of one map info. It keeps the first refusal: the steps after it go on and add edits
// that are never made.
type infoPatch struct {
	info    *w3i.Info
	file    string
	flags   int32 // the map flags as the settings leave them
	edits   []w3i.Edit
	refusal error
}

// refuse records why the settings do not go into this map, unless an earlier reason is recorded.
func (p *infoPatch) refuse(reason error) {
	if p.refusal == nil {
		p.refusal = reason
	}
}

// ---- the steps, in the order of the settings ----

func (p *infoPatch) description(info manifest.Info) {
	p.text(p.info.Name, info.Name)
	p.text(p.info.Author, info.Author)
	p.text(p.info.Description, info.Description)
	p.text(p.info.RecommendedPlayers, info.RecommendedPlayers)
}

func (p *infoPatch) loadingScreen(screen manifest.LoadingScreen) {
	loading := p.info.Loading
	p.number(loading.Background, screen.Background)
	switch {
	case screen.Model == nil:
	case loading.Model == nil:
		p.refuse(errNoLoadingModel(p.file))
	default:
		p.text(*loading.Model, screen.Model)
	}
	p.text(loading.Text, screen.Text)
	p.text(loading.Title, screen.Title)
	p.text(loading.Subtitle, screen.Subtitle)
}

// players edits the player of each override that sets something, in slot order.
func (p *infoPatch) players(overrides map[string]manifest.Player) {
	for _, id := range manifest.Slots(overrides) {
		override := overrides[id]
		if override == (manifest.Player{}) {
			continue
		}
		player, found := p.player(id)
		if !found {
			p.refuse(errNoPlayer(p.file, id))
			continue
		}
		p.text(player.Name, override.Name)
		p.named(player.Controller, override.Controller, controllers)
		p.named(player.Race, override.Race, races)
		p.yesNo(player.FixedStart, override.FixedStart)
		p.real(player.X, override.X)
		p.real(player.Y, override.Y)
	}
}

// player is the map's player in the slot of the id. A map has a record only for the slots it uses, in any order.
func (p *infoPatch) player(id string) (w3i.Player, bool) {
	slot, err := strconv.Atoi(id)
	if err != nil {
		return w3i.Player{}, false
	}
	players := p.info.Details.Players
	at := slices.IndexFunc(players, func(player w3i.Player) bool { return int(player.ID.Value) == slot })
	if at < 0 {
		return w3i.Player{}, false
	}
	return players[at], true
}

// forces edits the force of each override that sets something, in slot order.
func (p *infoPatch) forces(overrides map[string]manifest.Force) {
	for _, id := range manifest.Slots(overrides) {
		override := overrides[id]
		if override == (manifest.Force{}) {
			continue
		}
		force, found := p.force(id)
		switch {
		case !found:
			p.refuse(errNoForce(p.file, id))
		case p.info.Flags.Value&customForces == 0:
			p.refuse(errNoCustomForces(p.file, id))
		default:
			p.text(force.Name, override.Name)
			p.edits = append(p.edits, w3i.IntEdit(force.Flags, alliances(force.Flags.Value, override)))
		}
	}
}

// force is the map's force of the id, which is its place among the forces.
func (p *infoPatch) force(id string) (w3i.Force, bool) {
	forces := p.info.Details.Forces
	at, err := strconv.Atoi(id)
	if err != nil || at < 0 || at >= len(forces) {
		return w3i.Force{}, false
	}
	return forces[at], true
}

// alliances is a force's flags with each bit the override sets turned on or off.
func alliances(flags int32, override manifest.Force) int32 {
	flags = turned(flags, allied, override.Allied)
	flags = turned(flags, alliedVictory, override.AlliedVictory)
	flags = turned(flags, sharedVision, override.SharedVision)
	flags = turned(flags, sharedControl, override.SharedControl)
	return turned(flags, sharedAdvancedControl, override.SharedAdvancedControl)
}

// turned is flags with the bit on or off as the setting says, or as it was when the setting is not set.
func turned(flags, bit int32, setting *bool) int32 {
	switch {
	case setting == nil:
		return flags
	case *setting:
		return flags | bit
	}
	return flags &^ bit
}

func (p *infoPatch) environment(environment manifest.Environment) {
	if environment == (manifest.Environment{}) {
		return
	}
	details := p.info.Details
	p.text(details.SoundEnvironment, environment.SoundEnvironment)
	if environment.WaterColor != nil {
		p.flags |= waterTinted
		p.color(details.WaterColor, environment.WaterColor)
	}
	p.fog(environment.Fog)
}

// fog edits the terrain fog. The fog the map has is looked at only here, so a map whose own fog is not in order
// takes every other setting.
func (p *infoPatch) fog(fog manifest.Fog) {
	if fog == (manifest.Fog{}) {
		return
	}
	current := p.info.Details.Fog
	if !inOrder(fog, current) {
		p.refuse(errFog(p.file))
		return
	}
	p.flags = turned(p.flags, fogOn, fog.Enabled)
	p.number(current.Style, fog.Style)
	p.real(current.Start, fog.Start)
	p.real(current.End, fog.End)
	p.real(current.Density, fog.Density)
	p.color(current.Color, fog.Color)
}

// inOrder reports whether the fog as the settings leave it has a start, an end and a density that are numbers,
// and does not start after its end. A value that is not set is the map's own.
func inOrder(fog manifest.Fog, current w3i.Fog) bool {
	start := orInherited(fog.Start, current.Start.Value)
	end := orInherited(fog.End, current.End.Value)
	density := orInherited(fog.Density, current.Density.Value)
	isNumber := func(value float64) bool { return !math.IsInf(value, 0) && !math.IsNaN(value) }
	return isNumber(start) && isNumber(end) && isNumber(density) && start <= end
}

// orInherited is the fog value a setting sets, or the one the map has.
func orInherited(setting *float64, inherited float32) float64 {
	if setting != nil {
		return *setting
	}
	return float64(inherited)
}

// mapFlags writes the map flags when a setting turned one.
func (p *infoPatch) mapFlags() {
	if p.flags != p.info.Flags.Value {
		p.edits = append(p.edits, w3i.IntEdit(p.info.Flags, p.flags))
	}
}

// ---- one edit for one setting; a setting that is not set edits nothing ----

func (p *infoPatch) text(field w3i.Field[string], setting *string) {
	if setting != nil {
		p.edits = append(p.edits, w3i.TextEdit(field, *setting))
	}
}

func (p *infoPatch) number(field w3i.Field[int32], setting *int32) {
	if setting != nil {
		p.edits = append(p.edits, w3i.IntEdit(field, *setting))
	}
}

func (p *infoPatch) real(field w3i.Field[float32], setting *float64) {
	if setting != nil {
		p.edits = append(p.edits, w3i.FloatEdit(field, float32(*setting)))
	}
}

// yesNo writes a yes as 1 and a no as 0.
func (p *infoPatch) yesNo(field w3i.Field[int32], setting *bool) {
	switch {
	case setting == nil:
	case *setting:
		p.edits = append(p.edits, w3i.IntEdit(field, 1))
	default:
		p.edits = append(p.edits, w3i.IntEdit(field, 0))
	}
}

// named writes the number a name has in the file, which is its place among names. The empty name stands where
// a number has no name, and is not one.
func (p *infoPatch) named(field w3i.Field[int32], setting *string, names []string) {
	if setting == nil {
		return
	}
	number := slices.Index(names, *setting)
	if number < 0 || *setting == "" {
		p.refuse(errNoNumber(*setting, names))
		return
	}
	p.edits = append(p.edits, w3i.IntEdit(field, int32(number)))
}

// color writes the four channels, each to the byte it has in the file.
func (p *infoPatch) color(field w3i.Color, setting *[4]uint8) {
	if setting == nil {
		return
	}
	for channel, value := range setting {
		p.edits = append(p.edits, w3i.ByteEdit(field[channel], value))
	}
}

// ---- errors ----

func errNoLoadingModel(file string) error {
	return &diag.Error{
		Msg:  "settings.loadingScreen.model: custom loading-screen models require w3i version 25 or later.",
		File: file,
		Hint: "Save the map in a newer World Editor.",
	}
}

func errNoPlayer(file, id string) error {
	return &diag.Error{
		Msg:  "settings.players[" + strconv.Quote(id) + "]: player " + id + " does not exist in the source map.",
		File: file,
		Hint: "Create this player slot in World Editor first.",
	}
}

func errNoForce(file, id string) error {
	return &diag.Error{
		Msg:  "settings.forces[" + strconv.Quote(id) + "]: force " + id + " does not exist in the source map.",
		File: file,
		Hint: "Create this force in World Editor first.",
	}
}

func errNoCustomForces(file, id string) error {
	return &diag.Error{
		Msg:  "settings.forces[" + strconv.Quote(id) + "]: force overrides require custom forces enabled in the source map.",
		File: file,
		Hint: "Enable custom forces in World Editor first.",
	}
}

func errFog(file string) error {
	return &diag.Error{
		Msg:  "settings.environment.fog: start, end and density must be finite, and start must not exceed end.",
		File: file,
		Hint: "Check fog values in World Editor or set valid fog values in settings.",
	}
}

// errNoNumber is not a diag error: Pkl lets through only the names of the schema, so a name without a number is
// a name the schema has and this package lacks.
func errNoNumber(name string, names []string) error {
	return fmt.Errorf("settings: %q has no number in war3map.w3i; the names with one are %q", name, names)
}
