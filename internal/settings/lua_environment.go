package settings

import (
	"fmt"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

func (p *luaPatcher) patchEnvironment(set manifest.Environment, info *w3i.Info) {
	if set == (manifest.Environment{}) {
		return
	}
	main := p.findFunction("main")
	before := p.findAnchorCall(main)
	var calls []string
	if set.SoundEnvironment != nil {
		calls = append(calls, p.soundStatement(main, info.Details))
	}
	if set.WaterColor != nil {
		calls = append(calls, p.waterStatement(main, info.Details))
	}
	if set.Fog != (manifest.Fog{}) {
		calls = append(calls, p.fogStatement(main, info))
	}
	p.insertBefore(before.Start, calls)
}

func (p *luaPatcher) findAnchorCall(main lua.Function) lua.Call {
	at := slices.IndexFunc(main.Calls, func(call lua.Call) bool {
		return call.Name == "CreateAllUnits" || call.Name == "InitBlizzard"
	})
	switch {
	case p.hasFailed():
		return lua.Call{}
	case at < 0:
		p.fail(errNoAnchor(p.displayPath))
		return lua.Call{}
	case len(main.Calls[at].Args) != 0:
		p.fail(errArity(p.displayPath, main.Calls[at].Name, main.Name, 0))
		return lua.Call{}
	}
	return main.Calls[at]
}

func (p *luaPatcher) removeCalls(main lua.Function, native string, arity int) {
	if call, found := p.atMostOne(p.findCalls(main, native, arity), native+" in main()"); found {
		p.remove(call)
	}
}

func (p *luaPatcher) soundStatement(main lua.Function, details *w3i.Details) string {
	p.removeCalls(main, "NewSoundEnvironment", 1)
	name := details.SoundEnvironment.Value
	if name == "" {
		name = "Default"
	}
	return "NewSoundEnvironment(" + lua.QuoteString(name) + ")"
}

func (p *luaPatcher) waterStatement(main lua.Function, details *w3i.Details) string {
	p.removeCalls(main, "SetWaterBaseColor", 4)
	color := details.WaterColor
	return fmt.Sprintf("SetWaterBaseColor(%d, %d, %d, %d)", color[0].Value, color[1].Value, color[2].Value, color[3].Value)
}

func (p *luaPatcher) fogStatement(main lua.Function, info *w3i.Info) string {
	p.removeCalls(main, "SetTerrainFogEx", 7)
	p.removeCalls(main, "ResetTerrainFog", 0)
	if info.Flags.Value&fogOn == 0 {
		return "ResetTerrainFog()"
	}
	fog := info.Details.Fog
	if !areFinite(fog.Start.Value, fog.End.Value, fog.Density.Value) {
		p.fail(errNoFog(p.displayPath))
		return ""
	}
	number := func(value float32) string { return lua.FormatNumber(float64(value)) }
	channel := func(i int) string { return lua.FormatNumber(float64(fog.Color[i].Value) / 255) }
	return fmt.Sprintf("SetTerrainFogEx(%d, %s, %s, %s, %s, %s, %s)", fog.Style.Value,
		number(fog.Start.Value), number(fog.End.Value), number(fog.Density.Value), channel(0), channel(1), channel(2))
}

func errNoAnchor(displayPath string) error {
	return errLua(displayPath, "main() must call CreateAllUnits() or InitBlizzard() directly.")
}

func errNoFog(displayPath string) error {
	return errLuaHint(displayPath, "the fog of the map info has a start, an end or a density that is not a number.", resaveInfo)
}
