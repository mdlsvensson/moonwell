package settings

import (
	"bytes"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/war3/txt"
)

const (
	infoName = "war3map.w3i"
	luaName  = "war3map.lua"
	miscName = "war3mapMisc.txt"
	skinName = "war3mapSkin.txt"
)

func Plan(source *mapdir.Folder, project *manifest.Project) ([]mapdir.Change, error) {
	settings := project.Settings
	misc, skin, err := textSections(settings, project.ManifestName)
	if err != nil {
		return nil, err
	}
	preview, err := loadProjectPreview(project)
	if err != nil {
		return nil, err
	}
	plan := &planner{source: source}
	if err := plan.checkPreviewFits(preview); err != nil {
		return nil, err
	}
	info, err := plan.planMapInfo(settings)
	if err != nil {
		return nil, err
	}
	if err := plan.planLua(settings, info, preview != nil); err != nil {
		return nil, err
	}
	if err := plan.planTextFile(miscName, misc); err != nil {
		return nil, err
	}
	if err := plan.planTextFile(skinName, skin); err != nil {
		return nil, err
	}
	if err := plan.planPreview(preview); err != nil {
		return nil, err
	}
	return plan.changes, nil
}

type planner struct {
	source  *mapdir.Folder
	changes []mapdir.Change
}

func (p *planner) planMapInfo(settings manifest.Settings) ([]byte, error) {
	if !changesInfo(settings) {
		return nil, nil
	}
	data, err := p.readRequired(infoName)
	if err != nil {
		return nil, err
	}
	patched, err := patchInfo(data, settings, p.source.DisplayPath(infoName))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(patched, data) {
		if err := p.addWrite(infoName, patched); err != nil {
			return nil, err
		}
	}
	return patched, nil
}

func (p *planner) planLua(settings manifest.Settings, patchedInfo []byte, withPreview bool) error {
	if !needsLua(settings) && !withPreview {
		return nil
	}
	bom, source, err := p.readRequiredText(luaName)
	if err != nil {
		return err
	}
	patched, err := p.patchLuaSource(source, settings, patchedInfo, withPreview)
	if err != nil || patched == source {
		return err
	}
	return p.addWrite(luaName, []byte(bom+patched))
}

func needsLua(settings manifest.Settings) bool {
	return needsDetails(settings) || settings.Info.Name != nil || settings.Info.Description != nil
}

func (p *planner) patchLuaSource(source string, settings manifest.Settings, patchedInfo []byte, withPreview bool) (string, error) {
	displayPath := p.source.DisplayPath(luaName)
	patched, err := patchLuaFromInfo(source, settings, patchedInfo, displayPath, p.source.DisplayPath(infoName))
	if err != nil || !withPreview {
		return patched, err
	}
	return patchMinimap(patched, displayPath)
}

func (p *planner) planTextFile(name string, sections []txt.Section) error {
	if !hasFields(sections) {
		return nil
	}
	data, _, err := p.source.Read(name)
	if err != nil {
		return err
	}
	bom, source, err := p.decodeText(name, data)
	if err != nil {
		return err
	}
	merged := txt.Merge(source, sections)
	if merged == source {
		return nil
	}
	return p.addWrite(name, []byte(bom+merged))
}

func hasFields(sections []txt.Section) bool {
	for _, section := range sections {
		if len(section.Fields) > 0 {
			return true
		}
	}
	return false
}

func (p *planner) readRequired(name string) ([]byte, error) {
	data, found, err := p.source.Read(name)
	switch {
	case err != nil:
		return nil, err
	case found:
		return data, nil
	case p.source.IsDir(name):
		return nil, errIsDir(p.source.CanonicalPath(name), p.source.DisplayPath(name))
	}
	return nil, errMissing(p.source.DisplayPath(name))
}

func (p *planner) readRequiredText(name string) (bom, text string, err error) {
	data, err := p.readRequired(name)
	if err != nil {
		return "", "", err
	}
	return p.decodeText(name, data)
}

func (p *planner) decodeText(name string, data []byte) (bom, text string, err error) {
	bom, text, ok := fsx.SplitBOM(data)
	if !ok {
		return "", "", errNotText(p.source.DisplayPath(name))
	}
	return bom, text, nil
}

func (p *planner) addWrite(name string, data []byte) error {
	return p.addChange(mapdir.Change{Path: name, Data: data})
}

func (p *planner) addChange(change mapdir.Change) error {
	path, err := p.source.ResolveNewPath(change.Path)
	if err != nil {
		return err
	}
	change.Path = path
	p.changes = append(p.changes, change)
	return nil
}

const resaveMap = "Open and re-save the map in World Editor in folder format with Lua as the script language."

func errMissing(displayPath string) error {
	return &diag.Error{Msg: "A map file needed by the configured settings is missing.", File: displayPath, Hint: resaveMap}
}

func errIsDir(folder, displayPath string) error {
	return &diag.Error{
		Msg:  folder + " in the map is a folder, not a file.",
		File: displayPath,
		Hint: "The map has a folder where a file the configured settings need belongs. Remove that folder from the " +
			"source map, or open and re-save the map in World Editor.",
	}
}

func errNotText(displayPath string) error {
	return &diag.Error{Msg: "This map file is not valid UTF-8 text.", File: displayPath, Hint: resaveMap}
}
