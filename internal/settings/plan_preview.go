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

func loadProjectPreview(p *manifest.Project) (*picture.Picture, error) {
	preview := p.Settings.Info.Preview
	switch {
	case preview == nil:
		return nil, nil
	case p.Root == "":
		return nil, errNoProjectFolder()
	}
	return loadPreview(p.Root, *preview, p.ManifestName)
}

func (p *planner) checkPreviewFits(preview *picture.Picture) error {
	if preview == nil {
		return nil
	}
	if err := p.requireMinimap(); err != nil {
		return err
	}
	for _, added := range []string{keptMinimap, tgaName} {
		if p.source.HasFile(added) {
			return errNameTaken(p.source.CanonicalPath(added), p.source.DisplayPath(added))
		}
		if _, err := p.source.ResolveNewPath(added); err != nil {
			return err
		}
	}
	return nil
}

func (p *planner) requireMinimap() error {
	switch {
	case p.source.HasFile(savedMinimap):
		return nil
	case p.source.IsDir(savedMinimap):
		return errIsDir(p.source.CanonicalPath(savedMinimap), p.source.DisplayPath(savedMinimap))
	}
	return errNoMinimap(p.source.DisplayPath(savedMinimap))
}

func (p *planner) planPreview(preview *picture.Picture) error {
	if preview == nil {
		return nil
	}
	kept, err := p.readRequired(savedMinimap)
	if err != nil {
		return err
	}
	if err := p.addWrite(keptMinimap, kept); err != nil {
		return err
	}
	if preview.Extension == "blp" {
		return p.addWrite(savedMinimap, preview.Data)
	}
	if err := p.addChange(mapdir.Change{Path: savedMinimap, Remove: true}); err != nil {
		return err
	}
	return p.addWrite(tgaName, preview.Data)
}

func errNoMinimap(displayPath string) error {
	return &diag.Error{
		Msg:  "The map has no " + savedMinimap + ", the minimap whose place the preview picture takes.",
		File: displayPath,
		Hint: "Open and save the map in World Editor, which writes the minimap.",
	}
}

func errNameTaken(taken, displayPath string) error {
	return &diag.Error{
		Msg:  "The map already has " + taken + ", a name the preview picture needs.",
		File: displayPath,
		Hint: "Remove that file from the map: a build with settings.info.preview writes it.",
	}
}

func errNoProjectFolder() error {
	return errors.New("settings.Plan needs the project folder to read settings.info.preview.")
}
