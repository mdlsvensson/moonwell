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

func patchInfo(data []byte, settings manifest.Settings, displayPath string) ([]byte, error) {
	if !changesInfo(settings) {
		return data, nil
	}
	info, err := w3i.Read(data, displayPath, readDepthFor(settings))
	if err != nil {
		return nil, err
	}
	p := &infoPatch{info: info, displayPath: displayPath, flags: info.Flags.Value}
	p.patchDescription(settings.Info)
	p.patchLoadingScreen(settings.LoadingScreen)
	p.patchPlayers(settings.Players)
	p.patchForces(settings.Forces)
	p.patchEnvironment(settings.Environment)
	p.patchMapFlags()
	if p.err != nil {
		return nil, p.err
	}
	patched, err := w3i.ApplyEdits(data, p.edits)
	if err != nil {
		return nil, fmt.Errorf("patching %s: %w", displayPath, err)
	}
	return patched, nil
}

func changesInfo(settings manifest.Settings) bool {
	return needsDetails(settings) || hasDescription(settings.Info) || settings.LoadingScreen != (manifest.LoadingScreen{})
}

func hasDescription(info manifest.Info) bool {
	info.Preview = nil
	return info != (manifest.Info{})
}

func needsDetails(settings manifest.Settings) bool {
	return hasOverrides(settings.Players) || hasOverrides(settings.Forces) || settings.Environment != (manifest.Environment{})
}

func hasOverrides[V comparable](overrides map[int]V) bool {
	var zero V
	for _, override := range overrides {
		if override != zero {
			return true
		}
	}
	return false
}

func readDepthFor(settings manifest.Settings) w3i.Depth {
	if needsDetails(settings) {
		return w3i.Extended
	}
	return w3i.Basic
}

type infoPatch struct {
	info        *w3i.Info
	displayPath string
	flags       int32
	edits       []w3i.Edit
	err         error
}

func (p *infoPatch) fail(err error) {
	if p.err == nil {
		p.err = err
	}
}

func (p *infoPatch) patchDescription(info manifest.Info) {
	p.setText(p.info.Name, info.Name)
	p.setText(p.info.Author, info.Author)
	p.setText(p.info.Description, info.Description)
	p.setText(p.info.RecommendedPlayers, info.RecommendedPlayers)
}

func (p *infoPatch) patchLoadingScreen(screen manifest.LoadingScreen) {
	loading := p.info.Loading
	p.setInt(loading.Background, screen.Background)
	switch {
	case screen.Model == nil:
	case loading.Model == nil:
		p.fail(errNoLoadingModel(p.displayPath))
	default:
		p.setText(*loading.Model, screen.Model)
	}
	p.setText(loading.Text, screen.Text)
	p.setText(loading.Title, screen.Title)
	p.setText(loading.Subtitle, screen.Subtitle)
}

func (p *infoPatch) patchPlayers(overrides map[int]manifest.Player) {
	for _, slot := range manifest.SortedSlots(overrides) {
		override := overrides[slot]
		if override == (manifest.Player{}) {
			continue
		}
		player, found := p.findPlayer(slot)
		if !found {
			p.fail(errNoPlayer(p.displayPath, slot))
			continue
		}
		p.setText(player.Name, override.Name)
		p.setEnum(player.Controller, override.Controller, controllers)
		p.setEnum(player.Race, override.Race, races)
		p.setBool(player.FixedStart, override.FixedStart)
		p.setFloat(player.X, override.X)
		p.setFloat(player.Y, override.Y)
	}
}

func (p *infoPatch) findPlayer(slot int) (w3i.Player, bool) {
	players := p.info.Details.Players
	index := slices.IndexFunc(players, func(player w3i.Player) bool { return int(player.ID.Value) == slot })
	if index < 0 {
		return w3i.Player{}, false
	}
	return players[index], true
}

func (p *infoPatch) patchForces(overrides map[int]manifest.Force) {
	for _, slot := range manifest.SortedSlots(overrides) {
		override := overrides[slot]
		if override == (manifest.Force{}) {
			continue
		}
		force, found := p.findForce(slot)
		switch {
		case !found:
			p.fail(errNoForce(p.displayPath, slot))
		case p.info.Flags.Value&customForces == 0:
			p.fail(errNoCustomForces(p.displayPath, slot))
		default:
			p.setText(force.Name, override.Name)
			p.edits = append(p.edits, w3i.IntEdit(force.Flags, applyAllianceFlags(force.Flags.Value, override)))
		}
	}
}

func (p *infoPatch) findForce(slot int) (w3i.Force, bool) {
	forces := p.info.Details.Forces
	if slot < 0 || slot >= len(forces) {
		return w3i.Force{}, false
	}
	return forces[slot], true
}

func applyAllianceFlags(flags int32, override manifest.Force) int32 {
	flags = withBit(flags, allied, override.Allied)
	flags = withBit(flags, alliedVictory, override.AlliedVictory)
	flags = withBit(flags, sharedVision, override.SharedVision)
	flags = withBit(flags, sharedControl, override.SharedControl)
	return withBit(flags, sharedAdvancedControl, override.SharedAdvancedControl)
}

func withBit(flags, bit int32, setting *bool) int32 {
	switch {
	case setting == nil:
		return flags
	case *setting:
		return flags | bit
	}
	return flags &^ bit
}

func (p *infoPatch) patchEnvironment(environment manifest.Environment) {
	if environment == (manifest.Environment{}) {
		return
	}
	details := p.info.Details
	p.setText(details.SoundEnvironment, environment.SoundEnvironment)
	if environment.WaterColor != nil {
		p.flags |= waterTinted
		p.setColor(details.WaterColor, environment.WaterColor)
	}
	p.patchFog(environment.Fog)
}

func (p *infoPatch) patchFog(fog manifest.Fog) {
	if fog == (manifest.Fog{}) {
		return
	}
	current := p.info.Details.Fog
	if !isFogRangeValid(fog, current) {
		p.fail(errFogRange(p.displayPath))
		return
	}
	p.flags = withBit(p.flags, fogOn, fog.Enabled)
	p.setInt(current.Style, fog.Style)
	p.setFloat(current.Start, fog.Start)
	p.setFloat(current.End, fog.End)
	p.setFloat(current.Density, fog.Density)
	p.setColor(current.Color, fog.Color)
}

func isFogRangeValid(fog manifest.Fog, current w3i.Fog) bool {
	start := valueOrInherited(fog.Start, current.Start.Value)
	end := valueOrInherited(fog.End, current.End.Value)
	density := valueOrInherited(fog.Density, current.Density.Value)
	isNumber := func(value float64) bool { return !math.IsInf(value, 0) && !math.IsNaN(value) }
	return isNumber(start) && isNumber(end) && isNumber(density) && start <= end
}

func valueOrInherited(setting *float64, inherited float32) float64 {
	if setting != nil {
		return *setting
	}
	return float64(inherited)
}

func (p *infoPatch) patchMapFlags() {
	if p.flags != p.info.Flags.Value {
		p.edits = append(p.edits, w3i.IntEdit(p.info.Flags, p.flags))
	}
}

func (p *infoPatch) setText(field w3i.Field[string], setting *string) {
	if setting != nil {
		p.edits = append(p.edits, w3i.TextEdit(field, *setting))
	}
}

func (p *infoPatch) setInt(field w3i.Field[int32], setting *int32) {
	if setting != nil {
		p.edits = append(p.edits, w3i.IntEdit(field, *setting))
	}
}

func (p *infoPatch) setFloat(field w3i.Field[float32], setting *float64) {
	if setting != nil {
		p.edits = append(p.edits, w3i.FloatEdit(field, float32(*setting)))
	}
}

func (p *infoPatch) setBool(field w3i.Field[int32], setting *bool) {
	switch {
	case setting == nil:
	case *setting:
		p.edits = append(p.edits, w3i.IntEdit(field, 1))
	default:
		p.edits = append(p.edits, w3i.IntEdit(field, 0))
	}
}

func (p *infoPatch) setEnum(field w3i.Field[int32], setting *string, names []string) {
	if setting == nil {
		return
	}
	number := slices.Index(names, *setting)
	if number < 0 || *setting == "" {
		p.fail(errUnknownName(*setting, names))
		return
	}
	p.edits = append(p.edits, w3i.IntEdit(field, int32(number)))
}

func (p *infoPatch) setColor(field w3i.Color, setting *[4]uint8) {
	if setting == nil {
		return
	}
	for channel, value := range setting {
		p.edits = append(p.edits, w3i.ByteEdit(field[channel], value))
	}
}

func errNoLoadingModel(displayPath string) error {
	return &diag.Error{
		Msg:  "settings.loadingScreen.model: custom loading-screen models require w3i version 25 or later.",
		File: displayPath,
		Hint: "Save the map in a newer World Editor.",
	}
}

func errNoPlayer(displayPath string, slot int) error {
	return &diag.Error{
		Msg:  fmt.Sprintf(`settings.players["%d"]: player %d does not exist in the source map.`, slot, slot),
		File: displayPath,
		Hint: "Create this player slot in World Editor first.",
	}
}

func errNoForce(displayPath string, slot int) error {
	return &diag.Error{
		Msg:  fmt.Sprintf(`settings.forces["%d"]: force %d does not exist in the source map.`, slot, slot),
		File: displayPath,
		Hint: "Create this force in World Editor first.",
	}
}

func errNoCustomForces(displayPath string, slot int) error {
	return &diag.Error{
		Msg:  fmt.Sprintf(`settings.forces["%d"]: force overrides require custom forces enabled in the source map.`, slot),
		File: displayPath,
		Hint: "Enable custom forces in World Editor first.",
	}
}

func errFogRange(displayPath string) error {
	return &diag.Error{
		Msg:  "settings.environment.fog: start, end and density must be finite, and start must not exceed end.",
		File: displayPath,
		Hint: "Check fog values in World Editor or set valid fog values in settings.",
	}
}

func errUnknownName(name string, names []string) error {
	return fmt.Errorf("settings: %q has no number in war3map.w3i; the names with one are %q", name, names)
}
