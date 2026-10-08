package settings

import (
	"fmt"
	"math"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

var controllers = []string{"", "user", "computer", "neutral", "rescuable"}
var races = []string{"selectable", "human", "orc", "undead", "nightelf"}

const (
	customForces = 0x40
	fogOn        = 0x2000
	waterTinted  = 0x10000
)

const (
	allied                = 1
	alliedVictory         = 2
	sharedVision          = 8
	sharedControl         = 16
	sharedAdvancedControl = 32
)

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
	if p.failure != nil {
		return nil, p.failure
	}
	patched, err := w3i.ApplyEdits(data, p.edits)
	if err != nil {
		return nil, fmt.Errorf("patching %s: %w", file, err)
	}
	return patched, nil
}

func setsInfo(s manifest.Settings) bool {
	return needsDetails(s) || described(s.Info) || s.LoadingScreen != (manifest.LoadingScreen{})
}

func described(info manifest.Info) bool {
	info.Preview = nil
	return info != (manifest.Info{})
}

func needsDetails(s manifest.Settings) bool {
	return anySet(s.Players) || anySet(s.Forces) || s.Environment != (manifest.Environment{})
}

func anySet[V comparable](overrides map[int]V) bool {
	var nothing V
	for _, override := range overrides {
		if override != nothing {
			return true
		}
	}
	return false
}

func depthFor(s manifest.Settings) w3i.Depth {
	if needsDetails(s) {
		return w3i.Extended
	}
	return w3i.Basic
}

type infoPatch struct {
	info    *w3i.Info
	file    string
	flags   int32
	edits   []w3i.Edit
	failure error
}

func (p *infoPatch) refuse(reason error) {
	if p.failure == nil {
		p.failure = reason
	}
}

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

func (p *infoPatch) players(overrides map[int]manifest.Player) {
	for _, slot := range manifest.SortedSlots(overrides) {
		override := overrides[slot]
		if override == (manifest.Player{}) {
			continue
		}
		player, found := p.player(slot)
		if !found {
			p.refuse(errNoPlayer(p.file, slot))
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

func (p *infoPatch) player(slot int) (w3i.Player, bool) {
	players := p.info.Details.Players
	at := slices.IndexFunc(players, func(player w3i.Player) bool { return int(player.ID.Value) == slot })
	if at < 0 {
		return w3i.Player{}, false
	}
	return players[at], true
}

func (p *infoPatch) forces(overrides map[int]manifest.Force) {
	for _, slot := range manifest.SortedSlots(overrides) {
		override := overrides[slot]
		if override == (manifest.Force{}) {
			continue
		}
		force, found := p.force(slot)
		switch {
		case !found:
			p.refuse(errNoForce(p.file, slot))
		case p.info.Flags.Value&customForces == 0:
			p.refuse(errNoCustomForces(p.file, slot))
		default:
			p.text(force.Name, override.Name)
			p.edits = append(p.edits, w3i.IntEdit(force.Flags, alliances(force.Flags.Value, override)))
		}
	}
}

func (p *infoPatch) force(slot int) (w3i.Force, bool) {
	forces := p.info.Details.Forces
	if slot < 0 || slot >= len(forces) {
		return w3i.Force{}, false
	}
	return forces[slot], true
}

func alliances(flags int32, override manifest.Force) int32 {
	flags = turned(flags, allied, override.Allied)
	flags = turned(flags, alliedVictory, override.AlliedVictory)
	flags = turned(flags, sharedVision, override.SharedVision)
	flags = turned(flags, sharedControl, override.SharedControl)
	return turned(flags, sharedAdvancedControl, override.SharedAdvancedControl)
}

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

func inOrder(fog manifest.Fog, current w3i.Fog) bool {
	start := orInherited(fog.Start, current.Start.Value)
	end := orInherited(fog.End, current.End.Value)
	density := orInherited(fog.Density, current.Density.Value)
	isNumber := func(value float64) bool { return !math.IsInf(value, 0) && !math.IsNaN(value) }
	return isNumber(start) && isNumber(end) && isNumber(density) && start <= end
}

func orInherited(setting *float64, inherited float32) float64 {
	if setting != nil {
		return *setting
	}
	return float64(inherited)
}

func (p *infoPatch) mapFlags() {
	if p.flags != p.info.Flags.Value {
		p.edits = append(p.edits, w3i.IntEdit(p.info.Flags, p.flags))
	}
}

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

func (p *infoPatch) yesNo(field w3i.Field[int32], setting *bool) {
	switch {
	case setting == nil:
	case *setting:
		p.edits = append(p.edits, w3i.IntEdit(field, 1))
	default:
		p.edits = append(p.edits, w3i.IntEdit(field, 0))
	}
}

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

func (p *infoPatch) color(field w3i.Color, setting *[4]uint8) {
	if setting == nil {
		return
	}
	for channel, value := range setting {
		p.edits = append(p.edits, w3i.ByteEdit(field[channel], value))
	}
}

func errNoLoadingModel(file string) error {
	return &diag.Error{
		Msg:  "settings.loadingScreen.model: custom loading-screen models require w3i version 25 or later.",
		File: file,
		Hint: "Save the map in a newer World Editor.",
	}
}

func errNoPlayer(file string, slot int) error {
	return &diag.Error{
		Msg:  fmt.Sprintf(`settings.players["%d"]: player %d does not exist in the source map.`, slot, slot),
		File: file,
		Hint: "Create this player slot in World Editor first.",
	}
}

func errNoForce(file string, slot int) error {
	return &diag.Error{
		Msg:  fmt.Sprintf(`settings.forces["%d"]: force %d does not exist in the source map.`, slot, slot),
		File: file,
		Hint: "Create this force in World Editor first.",
	}
}

func errNoCustomForces(file string, slot int) error {
	return &diag.Error{
		Msg:  fmt.Sprintf(`settings.forces["%d"]: force overrides require custom forces enabled in the source map.`, slot),
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

func errNoNumber(name string, names []string) error {
	return fmt.Errorf("settings: %q has no number in war3map.w3i; the names with one are %q", name, names)
}
