package settings

import (
	"errors"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/war3/picture"
)

const (
	keptMinimap  = "war3mapMinimap.blp"
	savedMinimap = "war3mapMap.blp"
	tgaName      = "war3mapMap.tga"
)

func previewOf(p *manifest.Project) (*picture.Picture, error) {
	preview := p.Settings.Info.Preview
	switch {
	case preview == nil:
		return nil, nil
	case p.Root == "":
		return nil, errNoProjectFolder()
	}
	return loadPreview(p.Root, *preview, p.ManifestName)
}

func (p *planner) roomFor(preview *picture.Picture) error {
	if preview == nil {
		return nil
	}
	if err := p.hasMinimap(); err != nil {
		return err
	}
	for _, added := range []string{keptMinimap, tgaName} {
		if p.folder.HasFile(added) {
			return errNameTaken(p.folder.CanonicalPath(added), p.folder.DisplayPath(added))
		}
		if _, err := p.folder.ResolveNewPath(added); err != nil {
			return err
		}
	}
	return nil
}

func (p *planner) hasMinimap() error {
	switch {
	case p.folder.HasFile(savedMinimap):
		return nil
	case p.folder.IsDir(savedMinimap):
		return errFolderForFile(p.folder.CanonicalPath(savedMinimap), p.folder.DisplayPath(savedMinimap))
	}
	return errNoMinimap(p.folder.DisplayPath(savedMinimap))
}

func (p *planner) preview(preview *picture.Picture) error {
	if preview == nil {
		return nil
	}
	kept, err := p.required(savedMinimap)
	if err != nil {
		return err
	}
	if err := p.write(keptMinimap, kept); err != nil {
		return err
	}
	if preview.Extension == "blp" {
		return p.write(savedMinimap, preview.Bytes)
	}
	if err := p.change(mapdir.Change{Path: savedMinimap, Remove: true}); err != nil {
		return err
	}
	return p.write(tgaName, preview.Bytes)
}

func errNoMinimap(file string) error {
	return &diag.Error{
		Msg:  "The map has no " + savedMinimap + ", the minimap whose place the preview picture takes.",
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

func errNoProjectFolder() error {
	return errors.New("settings.Plan needs the project folder to read settings.info.preview.")
}
