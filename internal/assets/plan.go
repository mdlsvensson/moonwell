package assets

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/war3/imp"
)

const indexName = "war3map.imp"

type Result struct {
	Assets  []Asset
	Changes []mapdir.Change
	State   State

	view *mapdir.Folder
}

func Plan(ctx context.Context, source *mapdir.Folder, assets []Asset, owned State) (*Result, error) {
	if err := checkNotCancelled(ctx); err != nil {
		return nil, err
	}
	if len(assets) == 0 && len(owned.Files) == 0 {
		return &Result{Assets: []Asset{}, Changes: []mapdir.Change{}, view: source}, nil
	}
	p := &planner{ctx: ctx, source: source, assets: assets, order: owned.Files, owned: ownedByKey(owned)}
	if err := p.checkPaths(); err != nil {
		return nil, err
	}
	if err := p.readIndex(); err != nil {
		return nil, err
	}
	if err := p.checkOwnedUnchanged(); err != nil {
		return nil, err
	}
	if err := p.checkAssetPaths(); err != nil {
		return nil, err
	}
	changes, spelled := p.planWrites()
	changes = append(changes, p.planRemovals(spelled)...)
	changes = append(changes, p.planIndexChange(spelled)...)
	return &Result{Assets: spelled, Changes: changes, State: newState(spelled), view: source}, nil
}

type planner struct {
	ctx    context.Context
	source *mapdir.Folder
	assets []Asset
	order  []Owned
	owned  map[string]Owned

	index    []byte
	hasIndex bool
	imports  []imp.Entry
	imported map[string]bool
}

func ownedByKey(owned State) map[string]Owned {
	files := map[string]Owned{}
	for _, file := range owned.Files {
		files[mapdir.Key(file.Path)] = file
	}
	return files
}

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
	for _, file := range p.order {
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
	file := p.source.DisplayPath(indexName)
	if p.imports, err = imp.Read(data, file); err != nil {
		return err
	}
	p.index, p.hasIndex = data, true
	for _, entry := range p.imports {
		path, ok := fsx.CleanRelPath(entry.MapPath())
		if !ok {
			return errImportPath(entry.MapPath(), file)
		}
		if p.imported[mapdir.Key(path)] {
			return errImportedTwice(entry.MapPath(), file)
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
	for _, file := range p.order {
		if err := checkNotCancelled(p.ctx); err != nil {
			return err
		}
		data, found, err := p.source.Read(file.Path)
		if err != nil {
			return err
		}
		if found && fsx.SHA256Hex(data) != p.owned[mapdir.Key(file.Path)].Hash {
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
	if _, owned := p.owned[key]; !owned && (p.source.HasFile(asset.Target) || p.imported[key]) {
		return errConflict(asset, p.source.DisplayPath(asset.Target))
	}
	_, err := p.source.ResolveNewPath(asset.Target)
	return p.wrapPathError(asset, err)
}

func (p *planner) wrapPathError(asset Asset, refusal error) error {
	var failure *diag.Error
	switch {
	case !errors.As(refusal, &failure):
		return refusal
	case p.source.IsDir(asset.Target):
		return errOntoAFolder(asset, p.source.DisplayPath(asset.Target))
	}
	inTheWay := strings.TrimPrefix(failure.File, p.source.DisplayPath("")+"/")
	return errThroughAFile(inTheWay, asset, failure.File)
}

func (p *planner) planWrites() (writes []mapdir.Change, spelled []Asset) {
	writes = []mapdir.Change{}
	for _, asset := range p.assets {
		held := p.source.HasFile(asset.Target) && p.owned[mapdir.Key(asset.Target)].Hash == asset.Hash
		if !held {
			writes = append(writes, mapdir.Change{Path: asset.Target, Data: asset.Data})
		}
	}
	view := p.source.WithChanges(writes)
	for i := range writes {
		writes[i].Path = view.CanonicalPath(writes[i].Path)
	}
	spelled = make([]Asset, len(p.assets))
	for i, asset := range p.assets {
		spelled[i] = asset
		spelled[i].Target = view.CanonicalPath(asset.Target)
	}
	return writes, spelled
}

func (p *planner) planRemovals(assets []Asset) []mapdir.Change {
	wanted := map[string]bool{}
	for _, asset := range assets {
		wanted[mapdir.Key(asset.Target)] = true
	}
	var removals []mapdir.Change
	seen := map[string]bool{}
	for _, file := range p.order {
		key := mapdir.Key(file.Path)
		if seen[key] || wanted[key] || !p.source.HasFile(file.Path) {
			continue
		}
		seen[key] = true
		removals = append(removals, mapdir.Change{Path: p.source.CanonicalPath(file.Path), Remove: true})
	}
	return removals
}

func (p *planner) planIndexChange(assets []Asset) []mapdir.Change {
	flags := map[string]uint8{}
	entries := []imp.Entry{}
	for _, entry := range p.imports {
		key := mapdir.Key(entry.MapPath())
		if _, owned := p.owned[key]; owned {
			flags[key] = entry.Flag
			continue
		}
		entries = append(entries, entry)
	}
	for _, asset := range assets {
		flag, kept := flags[mapdir.Key(asset.Target)]
		if !kept {
			flag = imp.CustomPath
		}
		entries = append(entries, imp.Entry{Flag: flag, Path: strings.ReplaceAll(asset.Target, "/", `\`)})
	}
	written := imp.Write(entries)
	if p.hasIndex && bytes.Equal(written, p.index) {
		return nil
	}
	return []mapdir.Change{{Path: p.source.CanonicalPath(indexName), Data: written}}
}

func newState(assets []Asset) State {
	var state State
	for _, asset := range assets {
		state.Files = append(state.Files, Owned{Path: asset.Target, Hash: asset.Hash})
	}
	return state
}

func errInterruptedBeforeWriting() error {
	return &diag.Error{Msg: "Interrupted; nothing was written."}
}

func errNotAnAssetPath(what, path string, refusal error) error {
	return fmt.Errorf("Cannot plan the assets: %s has the in-map path %q, which no asset may have (%v).",
		what, path, refusal)
}

func errSamePath(path string) error {
	return fmt.Errorf("Cannot plan the assets: two assets have the in-map path %q.", path)
}

func errIndexIsAFolder(folder, file string) error {
	return &diag.Error{
		Msg:  folder + " in the map is a folder, not a file.",
		File: file,
		Hint: "The map has a folder where its index of imports belongs. Remove that folder from the source map, " +
			"or open and re-save the map in World Editor.",
	}
}

func errImportPath(path, file string) error {
	return &diag.Error{
		Msg:  "Invalid asset path: " + path,
		File: file,
		Hint: "Use a relative path such as icons/BTNSword.blp, without .., drive letters or characters Windows forbids.",
	}
}

func errImportedTwice(path, file string) error {
	return &diag.Error{
		Msg:  "war3map.imp lists " + path + " twice.",
		File: file,
		Hint: "Remove the duplicate import in World Editor's Import Manager.",
	}
}

func errModified(name, file string) error {
	return &diag.Error{
		Msg:  name + " was modified in the map after assets:sync wrote it.",
		File: file,
		Hint: "assets:sync owns this file in the source map (a build stages a copy of it). " +
			"Move your edited copy into assets/, or restore the file in the source map, then run assets:sync.",
	}
}

func errConflict(asset Asset, file string) error {
	owner, hint := "", "Import it under another path with assets.paths, or remove the map's own copy."
	if asset.Library != "" {
		owner, hint = " of library "+asset.Library, "Remove the map's own copy in World Editor's Import Manager."
	}
	return &diag.Error{
		Msg:  "Asset " + asset.Target + owner + " conflicts with a file or import already in the map.",
		File: file,
		Hint: hint,
	}
}

func errThroughAFile(inTheWay string, asset Asset, file string) error {
	return &diag.Error{
		Msg:  inTheWay + " in the map is a file, not a directory, so " + asset.Target + " cannot go there.",
		File: file,
		Hint: makeRoomHint(asset, inTheWay),
	}
}

func errOntoAFolder(asset Asset, file string) error {
	return &diag.Error{
		Msg:  "Asset " + asset.Target + " would replace a folder in the map.",
		File: file,
		Hint: makeRoomHint(asset, "that folder"),
	}
}

func makeRoomHint(asset Asset, inTheWay string) string {
	if asset.Library != "" {
		return "Remove " + inTheWay + " from the source map."
	}
	return "Import it under another path with assets.paths, or remove " + inTheWay + " from the source map."
}
