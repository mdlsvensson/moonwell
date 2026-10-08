package settings

import (
	"errors"
	"fmt"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

func patchLuaAfter(source string, s manifest.Settings, patchedInfo []byte, file, infoLabel string) (string, error) {
	if !setsLua(s) {
		return source, nil
	}
	info, err := w3i.Read(patchedInfo, infoLabel, depthFor(s))
	if err != nil {
		return "", err
	}
	return patchLua(source, s, info, file)
}

func patchLua(source string, s manifest.Settings, info *w3i.Info, file string) (string, error) {
	if info == nil || info.Details == nil && needsDetails(s) {
		return "", errNoMapInfo(file)
	}
	p, err := newPatcher(source, file)
	if err != nil {
		return "", err
	}
	p.nameAndDescription(s.Info, info)
	p.players(s.Players, info.Details)
	p.forces(s.Forces, info.Details)
	p.environment(s.Environment, info)
	if p.failure != nil {
		return "", p.failure
	}
	return p.edited()
}

func setsLua(s manifest.Settings) bool {
	return s.Info.Name != nil || s.Info.Description != nil || anySet(s.Players) || len(flagged(s.Forces)) > 0 ||
		s.Environment != (manifest.Environment{})
}

func (p *patcher) nameAndDescription(set manifest.Info, info *w3i.Info) {
	if set.Name != nil {
		p.text("SetMapName", info.Name.Value)
	}
	if set.Description != nil {
		p.text("SetMapDescription", info.Description.Value)
	}
}

func (p *patcher) text(native, value string) {
	p.replace(p.unique(p.function("config"), native, 1), native+"("+lua.QuoteString(value)+")")
}

func (p *patcher) edited() (string, error) {
	result, err := lua.ApplyEdits(p.source, p.edits)
	if err != nil {
		return "", fmt.Errorf("patching %s: %w", p.file, err)
	}
	if _, err := lua.ParseFunctions(result, p.file); err != nil {
		return "", errUnsafeEdit(p.file, err)
	}
	return result, nil
}

func patchMinimap(source, file string) (string, error) {
	p, err := newPatcher(source, file)
	if err != nil {
		return "", err
	}
	p.insertBefore(p.function("main").EndStart, []string{"BlzChangeMinimapTerrainTex(" + lua.QuoteString(keptMinimap) + ")"})
	if p.failure != nil {
		return "", p.failure
	}
	return p.edited()
}

func errUnsafeEdit(file string, cause error) error {
	return &diag.Error{
		Msg:   "Cannot apply map settings to Lua: the edited script could not be read back safely.",
		File:  file,
		Hint:  resaveLua,
		Cause: cause,
	}
}

func errNoMapInfo(file string) error {
	return errors.New("patching " + file + ": the map info was not read as far as the settings need")
}
