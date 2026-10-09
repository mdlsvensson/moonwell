package assets

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/war3/imp"
)

func checkNotCancelled(ctx context.Context) error {
	if ctx.Err() != nil {
		return errInterruptedBeforeWriting()
	}
	return nil
}

func (p *planner) checkPaths() error {
	taken := map[string]bool{}
	for _, asset := range p.assets {
		if _, err := parseTargetPath(asset.Target); err != nil {
			return errNotAnAssetPath("an asset", asset.Target, err)
		}
		if taken[mapdir.Key(asset.Target)] {
			return errSamePath(asset.Target)
		}
		taken[mapdir.Key(asset.Target)] = true
	}
	for _, file := range p.ownedFiles {
		if _, err := parseTargetPath(file.Path); err != nil {
			return errNotAnAssetPath("an owned file", file.Path, err)
		}
	}
	return nil
}

func (p *planner) readIndex() error {
	data, found, err := p.source.Read(indexName)
	if err != nil {
		return err
	}
	p.imported = map[string]bool{}
	if !found {
		return p.checkIndexPath()
	}
	displayPath := p.source.DisplayPath(indexName)
	if p.imports, err = imp.Read(data, displayPath); err != nil {
		return err
	}
	p.index, p.hasIndex = data, true
	for _, entry := range p.imports {
		path, ok := fsx.CleanRelPath(entry.MapPath())
		if !ok {
			return errImportPath(entry.MapPath(), displayPath)
		}
		if p.imported[mapdir.Key(path)] {
			return errImportedTwice(entry.MapPath(), displayPath)
		}
		p.imported[mapdir.Key(path)] = true
	}
	return nil
}

func (p *planner) checkIndexPath() error {
	if p.source.IsDir(indexName) {
		return errIndexIsAFolder(p.source.CanonicalPath(indexName), p.source.DisplayPath(indexName))
	}
	return nil
}

func (p *planner) checkOwnedUnchanged() error {
	for _, file := range p.ownedFiles {
		if err := checkNotCancelled(p.ctx); err != nil {
			return err
		}
		data, found, err := p.source.Read(file.Path)
		if err != nil {
			return err
		}
		if found && fsx.SHA256Hex(data) != p.ownedByKey[mapdir.Key(file.Path)].Hash {
			return errModified(p.source.CanonicalPath(file.Path), p.source.DisplayPath(file.Path))
		}
	}
	return nil
}

func (p *planner) checkAssetPaths() error {
	for _, asset := range p.assets {
		if err := checkNotCancelled(p.ctx); err != nil {
			return err
		}
		if err := p.checkAssetPath(asset); err != nil {
			return err
		}
	}
	return nil
}

func (p *planner) checkAssetPath(asset Asset) error {
	key := mapdir.Key(asset.Target)
	if _, owned := p.ownedByKey[key]; !owned && (p.source.HasFile(asset.Target) || p.imported[key]) {
		return errConflict(asset, p.source.DisplayPath(asset.Target))
	}
	_, err := p.source.ResolveNewPath(asset.Target)
	return p.wrapPathError(asset, err)
}

func (p *planner) wrapPathError(asset Asset, err error) error {
	var diagErr *diag.Error
	switch {
	case !errors.As(err, &diagErr):
		return err
	case p.source.IsDir(asset.Target):
		return errOntoAFolder(asset, p.source.DisplayPath(asset.Target))
	}
	blockingPath := strings.TrimPrefix(diagErr.File, p.source.DisplayPath("")+"/")
	return errThroughAFile(blockingPath, asset, diagErr.File)
}

func errInterruptedBeforeWriting() error {
	return &diag.Error{Msg: "Interrupted; nothing was written."}
}

func errNotAnAssetPath(what, path string, cause error) error {
	return fmt.Errorf("Cannot plan the assets: %s has the in-map path %q, which no asset may have (%v).",
		what, path, cause)
}

func errSamePath(path string) error {
	return fmt.Errorf("Cannot plan the assets: two assets have the in-map path %q.", path)
}

func errIndexIsAFolder(dir, displayPath string) error {
	return &diag.Error{
		Msg:  dir + " in the map is a folder, not a file.",
		File: displayPath,
		Hint: "The map has a folder where its index of imports belongs. Remove that folder from the source map, " +
			"or open and re-save the map in World Editor.",
	}
}

func errImportPath(path, displayPath string) error {
	return &diag.Error{
		Msg:  "Invalid asset path: " + path,
		File: displayPath,
		Hint: "Use a relative path such as icons/BTNSword.blp, without .., drive letters or characters Windows forbids.",
	}
}

func errImportedTwice(path, displayPath string) error {
	return &diag.Error{
		Msg:  "war3map.imp lists " + path + " twice.",
		File: displayPath,
		Hint: "Remove the duplicate import in World Editor's Import Manager.",
	}
}

func errModified(path, displayPath string) error {
	return &diag.Error{
		Msg:  path + " was modified in the map after assets:sync wrote it.",
		File: displayPath,
		Hint: "assets:sync owns this file in the source map (a build stages a copy of it). " +
			"Move your edited copy into assets/, or restore the file in the source map, then run assets:sync.",
	}
}

func errConflict(asset Asset, displayPath string) error {
	owner, hint := "", "Import it under another path with assets.paths, or remove the map's own copy."
	if asset.Library != "" {
		owner, hint = " of library "+asset.Library, "Remove the map's own copy in World Editor's Import Manager."
	}
	return &diag.Error{
		Msg:  "Asset " + asset.Target + owner + " conflicts with a file or import already in the map.",
		File: displayPath,
		Hint: hint,
	}
}

func errThroughAFile(blockingPath string, asset Asset, displayPath string) error {
	return &diag.Error{
		Msg:  blockingPath + " in the map is a file, not a directory, so " + asset.Target + " cannot go there.",
		File: displayPath,
		Hint: makeRoomHint(asset, blockingPath),
	}
}

func errOntoAFolder(asset Asset, displayPath string) error {
	return &diag.Error{
		Msg:  "Asset " + asset.Target + " would replace a folder in the map.",
		File: displayPath,
		Hint: makeRoomHint(asset, "that folder"),
	}
}

func makeRoomHint(asset Asset, blocking string) string {
	if asset.Library != "" {
		return "Remove " + blocking + " from the source map."
	}
	return "Import it under another path with assets.paths, or remove " + blocking + " from the source map."
}
