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

func Plan(folder *mapdir.Folder, p *manifest.Project) ([]mapdir.Change, error) {
	s := p.Settings
	misc, skin, err := textSections(s, p.ManifestName)
	if err != nil {
		return nil, err
	}
	preview, err := previewOf(p)
	if err != nil {
		return nil, err
	}
	plan := &planner{folder: folder}
	if err := plan.roomFor(preview); err != nil {
		return nil, err
	}
	info, err := plan.mapInfo(s)
	if err != nil {
		return nil, err
	}
	if err := plan.lua(s, info, preview != nil); err != nil {
		return nil, err
	}
	if err := plan.text(miscName, misc); err != nil {
		return nil, err
	}
	if err := plan.text(skinName, skin); err != nil {
		return nil, err
	}
	if err := plan.preview(preview); err != nil {
		return nil, err
	}
	return plan.changes, nil
}

type planner struct {
	folder  *mapdir.Folder
	changes []mapdir.Change
}

func (p *planner) mapInfo(s manifest.Settings) ([]byte, error) {
	if !setsInfo(s) {
		return nil, nil
	}
	data, err := p.required(infoName)
	if err != nil {
		return nil, err
	}
	patched, err := patchInfo(data, s, p.folder.DisplayPath(infoName))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(patched, data) {
		if err := p.write(infoName, patched); err != nil {
			return nil, err
		}
	}
	return patched, nil
}

func (p *planner) lua(s manifest.Settings, patchedInfo []byte, withPreview bool) error {
	if !needsLua(s) && !withPreview {
		return nil
	}
	mark, source, err := p.requiredText(luaName)
	if err != nil {
		return err
	}
	patched, err := p.patchedLua(source, s, patchedInfo, withPreview)
	if err != nil || patched == source {
		return err
	}
	return p.write(luaName, []byte(mark+patched))
}

func needsLua(s manifest.Settings) bool {
	return needsDetails(s) || s.Info.Name != nil || s.Info.Description != nil
}

func (p *planner) patchedLua(source string, s manifest.Settings, patchedInfo []byte, withPreview bool) (string, error) {
	file := p.folder.DisplayPath(luaName)
	patched, err := patchLuaAfter(source, s, patchedInfo, file, p.folder.DisplayPath(infoName))
	if err != nil || !withPreview {
		return patched, err
	}
	return patchMinimap(patched, file)
}

func (p *planner) text(name string, sections []txt.Section) error {
	if !setsKeys(sections) {
		return nil
	}
	data, _, err := p.folder.Read(name)
	if err != nil {
		return err
	}
	mark, source, err := p.textOf(name, data)
	if err != nil {
		return err
	}
	merged := txt.Merge(source, sections)
	if merged == source {
		return nil
	}
	return p.write(name, []byte(mark+merged))
}

func setsKeys(sections []txt.Section) bool {
	for _, section := range sections {
		if len(section.Fields) > 0 {
			return true
		}
	}
	return false
}

func (p *planner) required(name string) ([]byte, error) {
	data, found, err := p.folder.Read(name)
	switch {
	case err != nil:
		return nil, err
	case found:
		return data, nil
	case p.folder.IsDir(name):
		return nil, errFolderForFile(p.folder.CanonicalPath(name), p.folder.DisplayPath(name))
	}
	return nil, errMissing(p.folder.DisplayPath(name))
}

func (p *planner) requiredText(name string) (mark, text string, err error) {
	data, err := p.required(name)
	if err != nil {
		return "", "", err
	}
	return p.textOf(name, data)
}

func (p *planner) textOf(name string, data []byte) (mark, text string, err error) {
	mark, text, ok := fsx.SplitBOM(data)
	if !ok {
		return "", "", errNotText(p.folder.DisplayPath(name))
	}
	return mark, text, nil
}

func (p *planner) write(name string, data []byte) error {
	return p.change(mapdir.Change{Path: name, Data: data})
}

func (p *planner) change(change mapdir.Change) error {
	placed, err := p.folder.ResolveNewPath(change.Path)
	if err != nil {
		return err
	}
	change.Path = placed
	p.changes = append(p.changes, change)
	return nil
}

const resaveMap = "Open and re-save the map in World Editor in folder format with Lua as the script language."

func errMissing(file string) error {
	return &diag.Error{Msg: "A map file needed by the configured settings is missing.", File: file, Hint: resaveMap}
}

func errFolderForFile(folder, file string) error {
	return &diag.Error{
		Msg:  folder + " in the map is a folder, not a file.",
		File: file,
		Hint: "The map has a folder where a file the configured settings need belongs. Remove that folder from the " +
			"source map, or open and re-save the map in World Editor.",
	}
}

func errNotText(file string) error {
	return &diag.Error{Msg: "This map file is not valid UTF-8 text.", File: file, Hint: resaveMap}
}
