package settings

import (
	"errors"
	"fmt"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

func patchLuaAfter(source string, s manifest.Settings, patchedInfo []byte, displayPath, infoLabel string) (string, error) {
	if !changesLua(s) {
		return source, nil
	}
	info, err := w3i.Read(patchedInfo, infoLabel, readDepthFor(s))
	if err != nil {
		return "", err
	}
	return patchLua(source, s, info, displayPath)
}

func patchLua(source string, s manifest.Settings, info *w3i.Info, displayPath string) (string, error) {
	if info == nil || info.Details == nil && needsDetails(s) {
		return "", errNoMapInfo(displayPath)
	}
	p, err := newPatcher(source, displayPath)
	if err != nil {
		return "", err
	}
	p.patchNameAndDescription(s.Info, info)
	p.patchPlayers(s.Players, info.Details)
	p.patchForces(s.Forces, info.Details)
	p.patchEnvironment(s.Environment, info)
	if p.err != nil {
		return "", p.err
	}
	return p.result()
}

func changesLua(s manifest.Settings) bool {
	return s.Info.Name != nil || s.Info.Description != nil || hasOverrides(s.Players) || len(forcesWithFlags(s.Forces)) > 0 ||
		s.Environment != (manifest.Environment{})
}

func (p *luaPatcher) patchNameAndDescription(set manifest.Info, info *w3i.Info) {
	if set.Name != nil {
		p.setTextCall("SetMapName", info.Name.Value)
	}
	if set.Description != nil {
		p.setTextCall("SetMapDescription", info.Description.Value)
	}
}

func (p *luaPatcher) setTextCall(native, value string) {
	p.replace(p.findOneCall(p.findFunction("config"), native, 1), native+"("+lua.QuoteString(value)+")")
}

func (p *luaPatcher) result() (string, error) {
	result, err := lua.ApplyEdits(p.source, p.edits)
	if err != nil {
		return "", fmt.Errorf("patching %s: %w", p.displayPath, err)
	}
	if _, err := lua.ParseFunctions(result, p.displayPath); err != nil {
		return "", errUnsafeEdit(p.displayPath, err)
	}
	return result, nil
}

func patchMinimap(source, displayPath string) (string, error) {
	p, err := newPatcher(source, displayPath)
	if err != nil {
		return "", err
	}
	p.insertBefore(p.findFunction("main").EndStart, []string{"BlzChangeMinimapTerrainTex(" + lua.QuoteString(keptMinimap) + ")"})
	if p.err != nil {
		return "", p.err
	}
	return p.result()
}

func errUnsafeEdit(displayPath string, cause error) error {
	return &diag.Error{
		Msg:   "Cannot apply map settings to Lua: the edited script could not be read back safely.",
		File:  displayPath,
		Hint:  resaveLua,
		Cause: cause,
	}
}

func errNoMapInfo(displayPath string) error {
	return errors.New("patching " + displayPath + ": the map info was not read as far as the settings need")
}
