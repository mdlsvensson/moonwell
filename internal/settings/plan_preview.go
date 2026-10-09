package settings

import (
	"errors"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/war3/picture"
)

const (
	minimapCopyName = "war3mapMinimap.blp"
	minimapName     = "war3mapMap.blp"
	previewTGAName  = "war3mapMap.tga"
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
	for _, name := range []string{minimapCopyName, previewTGAName} {
		if p.source.HasFile(name) {
			return errNameTaken(p.source.CanonicalPath(name), p.source.DisplayPath(name))
		}
		if _, err := p.source.ResolveNewPath(name); err != nil {
			return err
		}
	}
	return nil
}

func (p *planner) requireMinimap() error {
	switch {
	case p.source.HasFile(minimapName):
		return nil
	case p.source.IsDir(minimapName):
		return errIsDir(p.source.CanonicalPath(minimapName), p.source.DisplayPath(minimapName))
	}
	return errNoMinimap(p.source.DisplayPath(minimapName))
}

func (p *planner) planPreview(preview *picture.Picture) error {
	if preview == nil {
		return nil
	}
	minimap, err := p.readRequired(minimapName)
	if err != nil {
		return err
	}
	if err := p.addWrite(minimapCopyName, minimap); err != nil {
		return err
	}
	if preview.Extension == "blp" {
		return p.addWrite(minimapName, preview.Data)
	}
	if err := p.addChange(mapdir.Change{Path: minimapName, Remove: true}); err != nil {
		return err
	}
	return p.addWrite(previewTGAName, preview.Data)
}

func errNoMinimap(displayPath string) error {
	return &diag.Error{
		Msg:  "The map has no " + minimapName + ", the minimap whose place the preview picture takes.",
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
