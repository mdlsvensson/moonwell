package assets

import (
	"bytes"
	"context"
	"strings"

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
	p := &planner{ctx: ctx, source: source, assets: assets, ownedFiles: owned.Files, ownedByKey: indexOwnedByKey(owned)}
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
	changes, canonicalAssets := p.planWrites()
	changes = append(changes, p.planRemovals(canonicalAssets)...)
	changes = append(changes, p.planIndexChange(canonicalAssets)...)
	return &Result{Assets: canonicalAssets, Changes: changes, State: newState(canonicalAssets), view: source}, nil
}

type planner struct {
	ctx        context.Context
	source     *mapdir.Folder
	assets     []Asset
	ownedFiles []Owned
	ownedByKey map[string]Owned

	index    []byte
	hasIndex bool
	imports  []imp.Entry
	imported map[string]bool
}

func indexOwnedByKey(owned State) map[string]Owned {
	files := map[string]Owned{}
	for _, file := range owned.Files {
		files[mapdir.Key(file.Path)] = file
	}
	return files
}

func (p *planner) planWrites() (writes []mapdir.Change, canonicalAssets []Asset) {
	writes = []mapdir.Change{}
	for _, asset := range p.assets {
		isUnchanged := p.source.HasFile(asset.Target) && p.ownedByKey[mapdir.Key(asset.Target)].Hash == asset.Hash
		if !isUnchanged {
			writes = append(writes, mapdir.Change{Path: asset.Target, Data: asset.Data})
		}
	}
	view := p.source.WithChanges(writes)
	for i := range writes {
		writes[i].Path = view.CanonicalPath(writes[i].Path)
	}
	canonicalAssets = make([]Asset, len(p.assets))
	for i, asset := range p.assets {
		canonicalAssets[i] = asset
		canonicalAssets[i].Target = view.CanonicalPath(asset.Target)
	}
	return writes, canonicalAssets
}

func (p *planner) planRemovals(assets []Asset) []mapdir.Change {
	targets := map[string]bool{}
	for _, asset := range assets {
		targets[mapdir.Key(asset.Target)] = true
	}
	var removals []mapdir.Change
	seen := map[string]bool{}
	for _, file := range p.ownedFiles {
		key := mapdir.Key(file.Path)
		if seen[key] || targets[key] || !p.source.HasFile(file.Path) {
			continue
		}
		seen[key] = true
		removals = append(removals, mapdir.Change{Path: p.source.CanonicalPath(file.Path), Remove: true})
	}
	return removals
}

func (p *planner) planIndexChange(assets []Asset) []mapdir.Change {
	ownedFlags := map[string]uint8{}
	entries := []imp.Entry{}
	for _, entry := range p.imports {
		key := mapdir.Key(entry.MapPath())
		if _, owned := p.ownedByKey[key]; owned {
			ownedFlags[key] = entry.Flag
			continue
		}
		entries = append(entries, entry)
	}
	for _, asset := range assets {
		flag, ok := ownedFlags[mapdir.Key(asset.Target)]
		if !ok {
			flag = imp.CustomPath
		}
		entries = append(entries, imp.Entry{Flag: flag, Path: strings.ReplaceAll(asset.Target, "/", `\`)})
	}
	data := imp.Write(entries)
	if p.hasIndex && bytes.Equal(data, p.index) {
		return nil
	}
	return []mapdir.Change{{Path: p.source.CanonicalPath(indexName), Data: data}}
}

func newState(assets []Asset) State {
	var state State
	for _, asset := range assets {
		state.Files = append(state.Files, Owned{Path: asset.Target, Hash: asset.Hash})
	}
	return state
}
