package settings

import (
	"bytes"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/war3/txt"
)

// The files of a map that settings change, by the names World Editor gives them. A map may spell them in another
// letter case, and is changed under the spelling it has. The files of a preview picture are named beside its
// steps.
const (
	infoName   = "war3map.w3i"
	scriptName = "war3map.lua"
	miscName   = "war3mapMisc.txt"
	skinName   = "war3mapSkin.txt"
)

// Plan computes every file the project's settings change in the map folder, and writes nothing. It returns the
// changed files only, in the order war3map.w3i, war3map.lua, war3mapMisc.txt, war3mapSkin.txt, and then the
// files of a preview picture: war3mapMinimap.blp (World Editor's minimap, kept), war3mapMap.blp (replaced by a
// BLP picture, removed for a TGA one) and war3mapMap.tga.
//
// What does not depend on the map is refused before a map file is read: constants that cannot be written, and a
// preview that is no usable picture. A file is read only when a setting needs it, so settings that set nothing
// read nothing.
func Plan(folder *mapdir.Folder, p *manifest.Project) ([]mapdir.Change, error) {
	s := p.Settings
	misc, skin, err := textSections(s, p.File)
	if err != nil {
		return nil, err
	}
	preview, err := previewOf(p)
	if err != nil {
		return nil, err
	}
	plan := &planner{folder: folder}
	plan.roomFor(preview)
	info := plan.mapInfo(s)
	plan.script(s, info, preview != nil)
	plan.text(miscName, misc)
	plan.text(skinName, skin)
	plan.preview(preview)
	if plan.failure != nil {
		return nil, plan.failure
	}
	return plan.changes, nil
}

// planner gathers the changes of one plan. It keeps the first failure: a step after it does nothing.
type planner struct {
	folder  *mapdir.Folder
	changes []mapdir.Change
	failure error
}

// ---- the steps, in the order of their changes; those of a preview are in plan_preview.go ----

// mapInfo patches war3map.w3i and returns the file as the settings leave it, which the script is brought into
// line with. Without a setting that is stored in the file, it is not read and nothing is returned.
func (p *planner) mapInfo(s manifest.Settings) []byte {
	if p.failure != nil || !setsInfo(s) {
		return nil
	}
	data, err := p.required(infoName)
	if err != nil {
		p.failure = err
		return nil
	}
	patched, err := patchInfo(data, s, p.folder.Label(infoName))
	if err != nil {
		p.failure = err
		return nil
	}
	if !bytes.Equal(patched, data) {
		p.write(infoName, patched)
	}
	return patched
}

// script brings war3map.lua into line with the patched map info and, for a map that gets a preview picture, adds
// the call that gives the game World Editor's minimap back. Both edits go into one change. A byte order mark is
// kept aside and put back in front of what is written.
func (p *planner) script(s manifest.Settings, patchedInfo []byte, withPreview bool) {
	if p.failure != nil || !needsScript(s) && !withPreview {
		return
	}
	mark, source, err := p.requiredText(scriptName)
	if err != nil {
		p.failure = err
		return
	}
	patched, err := p.patchedScript(source, s, patchedInfo, withPreview)
	if err != nil {
		p.failure = err
		return
	}
	if patched != source {
		p.write(scriptName, []byte(mark+patched))
	}
}

// needsScript reports whether the settings are of a kind the script takes part in: the map's name and
// description, and the players, the forces and the environment, which only a map whose script is Lua has. The
// script of such a map must be there and be text, also for a setting with no call of its own, as a force's name.
func needsScript(s manifest.Settings) bool {
	return needsDetails(s) || s.Info.Name != nil || s.Info.Description != nil
}

// patchedScript is the text of the script with the settings in it, and with the minimap call for a preview.
func (p *planner) patchedScript(source string, s manifest.Settings, patchedInfo []byte, withPreview bool) (string, error) {
	file := p.folder.Label(scriptName)
	patched, err := patchLuaAfter(source, s, patchedInfo, file, p.folder.Label(infoName))
	if err != nil || !withPreview {
		return patched, err
	}
	return patchMinimap(patched, file)
}

// text merges the sections into the text file under name. A map without the file is merged into as one whose
// file holds nothing, and so gets a file with the sections alone; a file that holds every value already is not
// changed. Without a key to set, the file is not read. A byte order mark is kept aside and put back in front of
// what is written.
func (p *planner) text(name string, sections []txt.Section) {
	if p.failure != nil || !setsKeys(sections) {
		return
	}
	data, _, err := p.folder.Read(name)
	if err != nil {
		p.failure = err
		return
	}
	mark, source, err := p.textOf(name, data)
	if err != nil {
		p.failure = err
		return
	}
	if merged := txt.Merge(source, sections); merged != source {
		p.write(name, []byte(mark+merged))
	}
}

// setsKeys reports whether a section has a key to set. Sections without keys write nothing.
func setsKeys(sections []txt.Section) bool {
	for _, section := range sections {
		if len(section.Fields) > 0 {
			return true
		}
	}
	return false
}

// ---- reading and changing the map's files ----

// required is the bytes of a file the settings need the map to have, in any letter case.
func (p *planner) required(name string) ([]byte, error) {
	data, found, err := p.folder.Read(name)
	switch {
	case err != nil:
		return nil, err
	case !found:
		return nil, errMissing(p.folder.Label(name))
	}
	return data, nil
}

// requiredText is required for a file that is text: its byte order mark, and the text after it.
func (p *planner) requiredText(name string) (mark, text string, err error) {
	data, err := p.required(name)
	if err != nil {
		return "", "", err
	}
	return p.textOf(name, data)
}

// textOf is the bytes of the file under name as text, which is UTF-8: the byte order mark the file starts with,
// or "", and the text after it.
func (p *planner) textOf(name string, data []byte) (mark, text string, err error) {
	mark, text, ok := fsx.TextWithMark(data)
	if !ok {
		return "", "", errNotText(p.folder.Label(name))
	}
	return mark, text, nil
}

// write plans the file under name to hold data.
func (p *planner) write(name string, data []byte) {
	p.change(mapdir.Change{Name: name, Bytes: data})
}

// change adds a change, under the spelling the map has for the file. A file the map does not have keeps the name
// given; where the map has a folder under that name, the plan is refused. After a failure nothing is added, so a
// step that makes several changes stops at the first that fails.
func (p *planner) change(change mapdir.Change) {
	if p.failure != nil {
		return
	}
	placed, err := p.folder.Place(change.Name)
	if err != nil {
		p.failure = err
		return
	}
	change.Name = placed
	p.changes = append(p.changes, change)
}

// ---- errors ----

// resaveMap is the hint of a map that lacks a file World Editor writes, or has one World Editor did not write.
const resaveMap = "Open and re-save the map in World Editor in folder format with Lua as the script language."

func errMissing(file string) error {
	return &diag.Error{Msg: "A map file needed by the configured settings is missing.", File: file, Hint: resaveMap}
}

func errNotText(file string) error {
	return &diag.Error{Msg: "This map file is not valid UTF-8 text.", File: file, Hint: resaveMap}
}
