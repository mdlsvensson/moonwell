package settings

import (
	"errors"
	"fmt"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
	"github.com/mdlsvensson/moonwell/internal/war3/w3i"
)

// patchLuaAfter brings the Lua World Editor generated into line with the map info, given as the bytes patchInfo
// returned. It returns the source as it is when no setting has a Lua counterpart: then neither the source nor
// the bytes are read. Otherwise the bytes are read as deep as the settings need; bytes that do not read are
// refused by infoLabel, the name of the map info. file names the Lua file in errors.
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

// patchLua is patchLuaAfter for a map info already read. It is asked only for settings with a Lua counterpart:
// patchLuaAfter, its one caller, returns the source as it is for the others.
//
// The script says what the map info says, also for a value the settings leave alone, so info is the map info as
// w3i.Read returns it after patchInfo: read Extended when a player, a force or the environment is set.
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

// setsLua reports whether a setting has a counterpart in war3map.lua: the map's name and description, a player
// override that sets something, an alliance flag of a force, and the environment. Every other setting is stored
// in the map info alone.
func setsLua(s manifest.Settings) bool {
	return s.Info.Name != nil || s.Info.Description != nil || anySet(s.Players) || len(flagged(s.Forces)) > 0 ||
		s.Environment != (manifest.Environment{})
}

// nameAndDescription writes the map's name and description into config(), each when it is set.
func (p *patcher) nameAndDescription(set manifest.Info, info *w3i.Info) {
	if set.Name != nil {
		p.text("SetMapName", info.Name.Value)
	}
	if set.Description != nil {
		p.text("SetMapDescription", info.Description.Value)
	}
}

// text gives the one call of the native in config() the value as its argument.
func (p *patcher) text(native, value string) {
	p.replace(p.unique(p.function("config"), native, 1), native+"("+lua.Quote(value)+")")
}

// edited is the source with the edits made. It is read once more: a script that does not read after the edits is
// refused.
func (p *patcher) edited() (string, error) {
	result, err := lua.ApplyEdits(p.source, p.edits)
	if err != nil {
		return "", fmt.Errorf("patching %s: %w", p.file, err)
	}
	if _, err := lua.Functions(result, p.file); err != nil {
		return "", errUnsafeEdit(p.file, err)
	}
	return result, nil
}

// patchMinimap adds, as the last statement of main(), the call that gives the game World Editor's minimap back:
// a build with a preview picture puts that picture in the minimap's place. The call has no effect before the body
// World Editor wrote for main() has run, and gameplay hooks run after main(), so a minimap they set still wins.
//
// The call stands on a line of its own when the `end` of main() starts its line; otherwise a space keeps it apart
// from the statement before. Where main() ends in a return that gives a value, no statement can follow: the
// result is read once more, and a script that does not read after the edits is refused.
func patchMinimap(source, file string) (string, error) {
	p, err := newPatcher(source, file)
	if err != nil {
		return "", err
	}
	p.insertBefore(p.function("main").EndStart, []string{"BlzChangeMinimapTerrainTex(" + lua.Quote(KeptMinimap) + ")"})
	if p.failure != nil {
		return "", p.failure
	}
	return p.edited()
}

// ---- errors ----

func errUnsafeEdit(file string, cause error) error {
	return &diag.Error{
		Msg:   "Cannot apply map settings to Lua: the edited script could not be read back safely.",
		File:  file,
		Hint:  resaveLua,
		Cause: cause,
	}
}

// errNoMapInfo is not a diag error: the caller reads the map info as deep as the settings need before it asks
// for the script.
func errNoMapInfo(file string) error {
	return errors.New("patching " + file + ": the map info was not read as far as the settings need")
}
