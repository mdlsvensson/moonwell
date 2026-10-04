package settings

import (
	"errors"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/war3/picture"
)

// The files of a preview picture, beside KeptMinimap. minimapName is the minimap World Editor writes, which the
// game's map list shows; tgaName is the name a picture that is not a BLP takes its place under.
const (
	minimapName = "war3mapMap.blp"
	tgaName     = "war3mapMap.tga"
)

// previewOf reads the picture the project's settings name as the map's preview. Without the setting it is nil.
func previewOf(p *manifest.Project) (*picture.Picture, error) {
	preview := p.Settings.Info.Preview
	switch {
	case preview == nil:
		return nil, nil
	case p.Root == "":
		return nil, errNoProjectFolder()
	}
	return loadPreview(p.Root, *preview, p.File)
}

// roomFor checks that the map can take a preview picture, before a file of the map is read. The picture takes
// the minimap's place, so the map must have a minimap, and the two names the preview adds must be free.
func (p *planner) roomFor(preview *picture.Picture) {
	if preview == nil {
		return
	}
	if !p.folder.Has(minimapName) {
		p.failure = errNoMinimap(p.folder.Label(minimapName))
		return
	}
	for _, added := range []string{KeptMinimap, tgaName} {
		if p.folder.Has(added) {
			p.failure = errNameTaken(p.folder.Name(added), p.folder.Label(added))
			return
		}
	}
}

// preview puts the picture in the minimap's place and keeps the minimap under KeptMinimap. A BLP takes the
// minimap's file; any other picture is a TGA, which goes in beside the minimap's file, and that file is removed.
func (p *planner) preview(preview *picture.Picture) {
	if p.failure != nil || preview == nil {
		return
	}
	kept, err := p.required(minimapName)
	if err != nil {
		p.failure = err
		return
	}
	p.write(KeptMinimap, kept)
	if preview.Extension == "blp" {
		p.write(minimapName, preview.Bytes)
		return
	}
	p.change(mapdir.Change{Name: minimapName, Remove: true})
	p.write(tgaName, preview.Bytes)
}

// ---- errors ----

func errNoMinimap(file string) error {
	return &diag.Error{
		Msg:  "The map has no " + minimapName + ", the minimap whose place the preview picture takes.",
		File: file,
		Hint: "Open and save the map in World Editor, which writes the minimap.",
	}
}

func errNameTaken(taken, file string) error {
	return &diag.Error{
		Msg:  "The map already has " + taken + ", a name the preview picture needs.",
		File: file,
		Hint: "Remove that file from the map: a build with settings.info.preview writes it.",
	}
}

// errNoProjectFolder is not a diag error: a project that was loaded knows its folder, so a caller that plans a
// preview for one without a folder has made the project itself.
func errNoProjectFolder() error {
	return errors.New("settings.Plan needs the project folder to read settings.info.preview.")
}
