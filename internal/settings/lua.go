package settings

import (
	"errors"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

func patchLuaFromInfo(source string, settings manifest.Settings, patchedInfo []byte, displayPath, infoDisplayPath string) (string, error) {
	if !changesLua(settings) {
		return source, nil
	}
	info, err := w3i.Read(patchedInfo, infoDisplayPath, readDepthFor(settings))
	if err != nil {
		return "", err
	}
	return patchLua(source, settings, info, displayPath)
}

func patchLua(source string, settings manifest.Settings, info *w3i.Info, displayPath string) (string, error) {
	if info == nil || info.Details == nil && needsDetails(settings) {
		return "", errNoMapInfo(displayPath)
	}
	p, err := newPatcher(source, displayPath)
	if err != nil {
		return "", err
	}
	p.patchNameAndDescription(settings.Info, info)
	p.patchPlayers(settings.Players, info.Details)
	p.patchForces(settings.Forces, info.Details)
	p.patchEnvironment(settings.Environment, info)
	if p.err != nil {
		return "", p.err
	}
	return p.patchedSource()
}

func changesLua(settings manifest.Settings) bool {
	return settings.Info.Name != nil || settings.Info.Description != nil || hasOverrides(settings.Players) || len(forcesWithFlags(settings.Forces)) > 0 ||
		settings.Environment != (manifest.Environment{})
}

func (p *luaPatcher) patchNameAndDescription(override manifest.Info, info *w3i.Info) {
	if override.Name != nil {
		p.setTextCall("SetMapName", info.Name.Value)
	}
	if override.Description != nil {
		p.setTextCall("SetMapDescription", info.Description.Value)
	}
}

func (p *luaPatcher) setTextCall(native, value string) {
	p.replace(p.findOneCall(p.findFunction("config"), native, 1), native+"("+lua.QuoteString(value)+")")
}

func patchMinimap(source, displayPath string) (string, error) {
	p, err := newPatcher(source, displayPath)
	if err != nil {
		return "", err
	}
	p.insertBefore(p.findFunction("main").EndStart, []string{"BlzChangeMinimapTerrainTex(" + lua.QuoteString(minimapCopyName) + ")"})
	if p.err != nil {
		return "", p.err
	}
	return p.patchedSource()
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
