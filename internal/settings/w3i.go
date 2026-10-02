package settings

import (
	"fmt"
	"math"
	"slices"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/internal/w3i"
)

// PatchMapInfo replaces the fields of a war3map.w3i that the settings set, and keeps every other byte. file is the
// name errors give. Without settings stored in the map info, source is returned as it is.
func PatchMapInfo(source []byte, s *Settings, file string) ([]byte, error) {
	extended := s.HasExtended()
	if !extended && s.Info.empty() && s.Loading.empty() {
		return source, nil
	}
	fail := func(message, hint string) ([]byte, error) {
		return nil, &diag.Error{Msg: message, File: file, Hint: hint}
	}
	info, err := w3i.Read(source, extended, file)
	if err != nil {
		return nil, err
	}
	var edits []w3i.Edit
	setText := func(field w3i.Field[string], value *string) {
		if value != nil {
			edits = append(edits, w3i.TextEdit(field, *value))
		}
	}
	setText(info.Name, s.Info.Name)
	setText(info.Author, s.Info.Author)
	setText(info.Description, s.Info.Description)
	setText(info.RecommendedPlayers, s.Info.RecommendedPlayers)

	if s.Loading.Background != nil {
		edits = append(edits, w3i.IntEdit(info.Loading.Background, *s.Loading.Background))
	}
	if s.Loading.Model != nil {
		if info.Loading.Model == nil {
			return fail("settings.loadingScreen.model: custom loading-screen models require w3i version 25 or later.",
				"Save the map in a newer World Editor.")
		}
		setText(*info.Loading.Model, s.Loading.Model)
	}
	setText(info.Loading.Text, s.Loading.Text)
	setText(info.Loading.Title, s.Loading.Title)
	setText(info.Loading.Subtitle, s.Loading.Subtitle)

	if d := info.Details; d != nil {
		flags := info.Flags.Value
		for id, values := range s.Players.All() {
			number, _ := strconv.Atoi(id)
			index := slices.IndexFunc(d.Players, func(player w3i.Player) bool { return int(player.ID.Value) == number })
			if index < 0 {
				return fail("settings.players["+text.Quote(id)+"]: player "+id+" does not exist in the source map.",
					"Create this player slot in World Editor first.")
			}
			player := d.Players[index]
			setText(player.Name, values.Name)
			if values.Controller != nil {
				edits = append(edits, w3i.IntEdit(player.Controller, int32(slices.Index(Controllers, *values.Controller))))
			}
			if values.Race != nil {
				edits = append(edits, w3i.IntEdit(player.Race, int32(slices.Index(Races, *values.Race))))
			}
			if values.FixedStart != nil {
				fixed := int32(0)
				if *values.FixedStart {
					fixed = 1
				}
				edits = append(edits, w3i.IntEdit(player.FixedStart, fixed))
			}
			if values.X != nil {
				edits = append(edits, w3i.FloatEdit(player.X, float32(*values.X)))
			}
			if values.Y != nil {
				edits = append(edits, w3i.FloatEdit(player.Y, float32(*values.Y)))
			}
		}
		for index, values := range s.Forces.All() {
			path := "settings.forces[" + text.Quote(index) + "]"
			number, _ := strconv.Atoi(index)
			if number >= len(d.Forces) {
				return fail(path+": force "+index+" does not exist in the source map.",
					"Create this force in World Editor first.")
			}
			if flags&0x40 == 0 {
				return fail(path+": force overrides require custom forces enabled in the source map.",
					"Enable custom forces in World Editor first.")
			}
			force := d.Forces[number]
			setText(force.Name, values.Name)
			bits := force.Flags.Value
			for _, flag := range forceBits {
				if value := flag.get(&values); value != nil {
					if *value {
						bits |= flag.bit
					} else {
						bits &^= flag.bit
					}
				}
			}
			edits = append(edits, w3i.IntEdit(force.Flags, bits))
		}
		environment := s.Environment
		setText(d.SoundEnvironment, environment.SoundEnvironment)
		if environment.WaterColor != nil {
			flags |= 0x10000
			for i, channel := range environment.WaterColor {
				edits = append(edits, w3i.ByteEdit(d.WaterColor[i], channel))
			}
		}
		if fog := environment.Fog; fog != nil {
			if fog.Enabled != nil {
				if *fog.Enabled {
					flags |= 0x2000
				} else {
					flags &^= 0x2000
				}
			}
			or := func(value *float64, inherited float32) float64 {
				if value != nil {
					return *value
				}
				return float64(inherited)
			}
			start, end, density := or(fog.Start, d.Fog.Start.Value), or(fog.End, d.Fog.End.Value), or(fog.Density, d.Fog.Density.Value)
			finite := func(v float64) bool { return !math.IsInf(v, 0) && !math.IsNaN(v) }
			if !finite(start) || !finite(end) || !finite(density) || start > end {
				return fail("settings.environment.fog: start, end and density must be finite, and start must not exceed end.",
					"Check fog values in World Editor or set valid fog values in settings.")
			}
			if fog.Style != nil {
				edits = append(edits, w3i.IntEdit(d.Fog.Style, *fog.Style))
			}
			for _, field := range []struct {
				value *float64
				field w3i.Field[float32]
			}{{fog.Start, d.Fog.Start}, {fog.End, d.Fog.End}, {fog.Density, d.Fog.Density}} {
				if field.value != nil {
					edits = append(edits, w3i.FloatEdit(field.field, float32(*field.value)))
				}
			}
			if fog.Color != nil {
				for i, channel := range fog.Color {
					edits = append(edits, w3i.ByteEdit(d.Fog.Color[i], channel))
				}
			}
		}
		if flags != info.Flags.Value {
			edits = append(edits, w3i.IntEdit(info.Flags, flags))
		}
	}
	patched, err := w3i.ApplyEdits(source, edits)
	if err != nil {
		return nil, fmt.Errorf("patching %s: %w", file, err)
	}
	return patched, nil
}
