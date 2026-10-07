package settings

import (
	"errors"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/war3/picture"
)

// The files of a preview picture.
const (
	// keptMinimap is the file a build with a preview picture keeps World Editor's minimap in: a copy of what the
	// map has as savedMinimap, which the picture replaces. The call patchMinimap adds to war3map.lua names it.
	keptMinimap = "war3mapMinimap.blp"
	// savedMinimap is the file World Editor saves the minimap as, which the game's map list shows: the place a
	// preview picture takes.
	savedMinimap = "war3mapMap.blp"
	// tgaName is the file a preview picture that is not a BLP goes in as, in place of savedMinimap.
	tgaName = "war3mapMap.tga"
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
// the minimap's place, so the map must have a minimap, and the two names the preview adds must be free: of a
// file, and of a folder, which mapdir refuses a file's place to. Both names are checked for a picture of either
// kind, the name the minimap is kept under first.
func (p *planner) roomFor(preview *picture.Picture) error {
	if preview == nil {
		return nil
	}
	if err := p.hasMinimap(); err != nil {
		return err
	}
	for _, added := range []string{keptMinimap, tgaName} {
		if p.folder.Has(added) {
			return errNameTaken(p.folder.Name(added), p.folder.Label(added))
		}
		if _, err := p.folder.Place(added); err != nil {
			return err
		}
	}
	return nil
}

// hasMinimap fails unless the map has the minimap World Editor saves. A folder under the minimap's name is not
// the minimap, and is refused as a folder where the file belongs, not as a minimap the map lacks.
func (p *planner) hasMinimap() error {
	switch {
	case p.folder.Has(savedMinimap):
		return nil
	case p.folder.IsFolder(savedMinimap):
		return errFolderForFile(p.folder.Name(savedMinimap), p.folder.Label(savedMinimap))
	}
	return errNoMinimap(p.folder.Label(savedMinimap))
}

// preview puts the picture in the minimap's place and keeps the minimap under keptMinimap. A BLP takes the
// minimap's file; any other picture is a TGA, which goes in beside the minimap's file, and that file is removed.
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
	if err := p.change(mapdir.Change{Name: savedMinimap, Remove: true}); err != nil {
		return err
	}
	return p.write(tgaName, preview.Bytes)
}

// ---- errors ----

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

// errNoProjectFolder is not a diag error: a project that was loaded knows its folder, so a caller that plans a
// preview for one without a folder has made the project itself.
func errNoProjectFolder() error {
	return errors.New("settings.Plan needs the project folder to read settings.info.preview.")
}
