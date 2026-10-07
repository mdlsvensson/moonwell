// Package assets imports files into a map: those under a project's assets/ folder and those its libraries ship.
// It collects the files and gives each its in-map path, plans the changes that bring a map folder and its
// war3map.imp into line with them, and for assets:sync writes those changes into the source map and keeps the
// ownership state: the file that says which of the map's files it wrote, and so may replace or remove. For
// assets:paths it reports the files that models reference: which of them the game ships, by the list of the
// game's paths that the program carries, and which a build imports.
//
// Collect takes the project folder, the manifest's assets block and the libraries' folders, and returns the
// assets. Plan takes a map folder, the assets and the ownership state as ReadState read it, and returns the
// changes and the state after them. Sync takes a plan, the map folder it was made from and the path of the state
// file, and writes both: nothing else in the package writes. For the report the package takes the models, which
// are those among the assets or the one a command line names and ReadModel reads, the game's paths and the paths
// a build imports, and returns the references with their statuses, and the report as lines.
//
// It must not know where a library's files come from, how a map is built, nor the settings and the object data
// of a map. It prints nothing: the command prints a report, and decides what a model that cannot be read means
// for it.
//
// Of Moonwell it imports manifest, mapdir, diag, fsx, war3/imp and war3/model, and the root package for the
// embedded list of the game's paths.
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

// indexName is World Editor's index of imported files, by the name World Editor gives it.
const indexName = "war3map.imp"

// Result is what importing the assets changes in a map folder.
type Result struct {
	Assets  []Asset // with each Target in the spelling it is written under
	Changes []mapdir.Change
	State   State // the ownership after the changes

	planned *mapdir.Folder // the folder the plan was made from, which is the one Sync writes into
}

// Plan checks every asset, import and owned file of the map folder, and returns the changes: the assets are
// written, the owned files that no asset wants are removed, and war3map.imp lists the assets. It writes
// nothing, and stops between files once ctx is cancelled.
//
// The changes replace and remove only files that owned lists and that hold what it says they hold. A file of
// the map that is not owned, an import World Editor made, and an owned file with other bytes are refused. Every
// file a change replaces or removes is read through folder, so that ApplyInPlace refuses one that changes after
// the plan.
//
// assets is what Collect returned and owned what ReadState read. Plan checks their in-map paths again with
// targetPath: a path that is one of the map's own files would have a change write or remove that file. Such a
// path is the caller's bug and fails with a plain error, and so do two assets with one path. An asset inside
// another is not looked for: the folder refuses to write such a plan, before its first write.
//
// With nothing to import and nothing owned it returns no change and reads nothing.
func Plan(ctx context.Context, folder *mapdir.Folder, assets []Asset, owned State) (*Result, error) {
	if err := stopIfInterrupted(ctx); err != nil {
		return nil, err
	}
	if len(assets) == 0 && len(owned.Files) == 0 {
		return &Result{Assets: []Asset{}, Changes: []mapdir.Change{}, planned: folder}, nil
	}
	p := &planner{ctx: ctx, folder: folder, assets: assets, order: owned.Files, owned: ownedByKey(owned)}
	if err := p.checkPaths(); err != nil {
		return nil, err
	}
	if err := p.readIndex(); err != nil {
		return nil, err
	}
	if err := p.ownedUnchanged(); err != nil {
		return nil, err
	}
	if err := p.roomForAssets(); err != nil {
		return nil, err
	}
	changes, spelled := p.writes()
	changes = append(changes, p.removals(spelled)...)
	changes = append(changes, p.indexChange(spelled)...)
	return &Result{Assets: spelled, Changes: changes, State: ownershipOf(spelled), planned: folder}, nil
}

// planner is one plan in the making: what it was given, and the index of imports as the map has it.
type planner struct {
	ctx    context.Context
	folder *mapdir.Folder
	assets []Asset
	order  []Owned          // the owned files in the order of the state
	owned  map[string]Owned // by the key of its path, each owned file

	index    []byte          // war3map.imp as the map has it
	hasIndex bool            // whether the map has the file
	imports  []imp.Entry     // its entries
	imported map[string]bool // by key, the in-map paths the entries name
}

// ownedByKey is the owned files by the key of their paths. Of two entries for one path the later counts.
func ownedByKey(owned State) map[string]Owned {
	files := map[string]Owned{}
	for _, file := range owned.Files {
		files[mapdir.Key(file.Path)] = file
	}
	return files
}

// stopIfInterrupted fails once ctx is cancelled.
func stopIfInterrupted(ctx context.Context) error {
	if ctx.Err() != nil {
		return errInterruptedBeforeWriting()
	}
	return nil
}

// ---- the checks ----

// checkPaths fails when an asset or an owned file has an in-map path that targetPath refuses, and when two
// assets have one path: what Collect and ReadState never return. Two owned entries with one path pass: the
// later one counts (ownedByKey), and the file is checked and removed once.
func (p *planner) checkPaths() error {
	taken := map[string]bool{}
	for _, asset := range p.assets {
		if _, err := targetPath(asset.Target); err != nil {
			return errNotAnAssetPath("an asset", asset.Target, err)
		}
		if taken[mapdir.Key(asset.Target)] {
			return errSamePath(asset.Target)
		}
		taken[mapdir.Key(asset.Target)] = true
	}
	for _, file := range p.order {
		if _, err := targetPath(file.Path); err != nil {
			return errNotAnAssetPath("an owned file", file.Path, err)
		}
	}
	return nil
}

// readIndex reads war3map.imp and notes the in-map path of each entry. A map without the file imports nothing.
// Every entry must name a path a file can have, and no two the same one.
func (p *planner) readIndex() error {
	data, found, err := p.folder.Read(indexName)
	if err != nil {
		return err
	}
	p.imported = map[string]bool{}
	if !found {
		return p.placeForIndex()
	}
	file := p.folder.Label(indexName)
	if p.imports, err = imp.Read(data, file); err != nil {
		return err
	}
	p.index, p.hasIndex = data, true
	for _, entry := range p.imports {
		path, ok := fsx.RelPath(entry.MapPath())
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

// placeForIndex fails when a map without war3map.imp has no place for the one the plan writes: the map has a
// folder under that name. The name has no folder on its way, so nothing else can be in its place.
func (p *planner) placeForIndex() error {
	if p.folder.IsFolder(indexName) {
		return errIndexIsAFolder(p.folder.Name(indexName), p.folder.Label(indexName))
	}
	return nil
}

// ownedUnchanged fails unless every owned file the map has holds what the state says, the files the plan
// removes among them. An owned file the map does not have is not checked: the plan writes it again, or drops it.
func (p *planner) ownedUnchanged() error {
	for _, file := range p.order {
		if err := stopIfInterrupted(p.ctx); err != nil {
			return err
		}
		data, found, err := p.folder.Read(file.Path)
		if err != nil {
			return err
		}
		if found && fsx.SHA256Hex(data) != p.owned[mapdir.Key(file.Path)].Hash {
			return errModified(p.folder.Name(file.Path), p.folder.Label(file.Path))
		}
	}
	return nil
}

// roomForAssets fails at the first asset the map has no room for.
func (p *planner) roomForAssets() error {
	for _, asset := range p.assets {
		if err := stopIfInterrupted(p.ctx); err != nil {
			return err
		}
		if err := p.roomFor(asset); err != nil {
			return err
		}
	}
	return nil
}

// roomFor fails when the asset would land on a file or an import that the map has and assets:sync does not own,
// on a folder of the map, or below a file of the map. An owned file is in the way of an asset below it too: no
// file of the map becomes a folder.
func (p *planner) roomFor(asset Asset) error {
	key := mapdir.Key(asset.Target)
	if _, owned := p.owned[key]; !owned && (p.folder.Has(asset.Target) || p.imported[key]) {
		return errConflict(asset, p.folder.Label(asset.Target))
	}
	_, err := p.folder.Place(asset.Target)
	return p.noPlace(asset, err)
}

// noPlace is the refusal of an asset that the folder has no place for. Place refuses a name for one of two
// reasons, and the folder is asked which: the map has a folder under the asset's own path, which the folder
// names, or a file on the way to it. That file is named by the file of Place's error alone: a file the map has on
// disk is on the way in a view that removes it too, and the folder has no other answer that tells such a file
// from a folder the asset would make.
func (p *planner) noPlace(asset Asset, refusal error) error {
	var failure *diag.Error
	switch {
	case !errors.As(refusal, &failure):
		return refusal
	case p.folder.IsFolder(asset.Target):
		return errOntoAFolder(asset, p.folder.Label(asset.Target))
	}
	inTheWay := strings.TrimPrefix(failure.File, p.folder.Label("")+"/")
	return errThroughAFile(inTheWay, asset, failure.File)
}

// ---- the changes ----

// writes is a change for each asset that the map does not have, or has with other bytes, and the assets with
// each Target as it is written. A file the map has at an asset's path is owned and holds what the state says:
// roomFor and ownedUnchanged saw to that. The spelling is the folder's: all writes are laid over it at once, so
// that a file the map has keeps its name, and a new one goes below folders as the map, or the first asset to
// name them, spells them.
func (p *planner) writes() (writes []mapdir.Change, spelled []Asset) {
	writes = []mapdir.Change{}
	for _, asset := range p.assets {
		held := p.folder.Has(asset.Target) && p.owned[mapdir.Key(asset.Target)].Hash == asset.Hash
		if !held {
			writes = append(writes, mapdir.Change{Name: asset.Target, Bytes: asset.Bytes})
		}
	}
	view := p.folder.With(writes)
	for i := range writes {
		writes[i].Name = view.Name(writes[i].Name)
	}
	spelled = make([]Asset, len(p.assets))
	for i, asset := range p.assets {
		spelled[i] = asset
		spelled[i].Target = view.Name(asset.Target)
	}
	return writes, spelled
}

// removals is a change for each owned file that the map has and no asset is imported as, in the order of the
// state.
func (p *planner) removals(assets []Asset) []mapdir.Change {
	wanted := map[string]bool{}
	for _, asset := range assets {
		wanted[mapdir.Key(asset.Target)] = true
	}
	var removals []mapdir.Change
	seen := map[string]bool{}
	for _, file := range p.order {
		key := mapdir.Key(file.Path)
		if seen[key] || wanted[key] || !p.folder.Has(file.Path) {
			continue
		}
		seen[key] = true
		removals = append(removals, mapdir.Change{Name: p.folder.Name(file.Path), Remove: true})
	}
	return removals
}

// indexChange is the change that brings war3map.imp into line with the assets, or none when the file holds
// that already: the entries assets:sync does not own in their order, then one for each asset. An owned entry
// keeps the flag World Editor saved it with, which is 29 where Moonwell wrote 13, so that a map World Editor
// saved is not changed.
func (p *planner) indexChange(assets []Asset) []mapdir.Change {
	flags := map[string]uint8{}
	entries := []imp.Entry{}
	for _, entry := range p.imports {
		key := mapdir.Key(entry.MapPath())
		if _, owned := p.owned[key]; owned {
			// Whatever the flag is, it is kept, and the asset's entry below is written with the whole in-map path.
			// Under a flag without a custom path (0, 5 or 8) World Editor reads that path below war3mapImported\,
			// so such an entry names war3mapImported\war3mapImported\..., a file the map does not have, and the
			// next plan takes it for an import of the editor's own and adds a second entry for the asset.
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
	return []mapdir.Change{{Name: p.folder.Name(indexName), Bytes: written}}
}

// ownershipOf is the state after the changes: every asset by the path it is written under, with its hash.
func ownershipOf(assets []Asset) State {
	var state State
	for _, asset := range assets {
		state.Files = append(state.Files, Owned{Path: asset.Target, Hash: asset.Hash})
	}
	return state
}

// ---- errors ----

// errInterruptedBeforeWriting is raised by Sync too, when it is stopped before its first write.
func errInterruptedBeforeWriting() error {
	return &diag.Error{Msg: "Interrupted; nothing was written."}
}

// errNotAnAssetPath and errSamePath are not diag errors: Collect and ReadState refuse such paths, so one that
// reaches a plan must be reported as Moonwell's own fault. The refusal of the path is a diag error, and is not
// wrapped.
func errNotAnAssetPath(what, path string, refusal error) error {
	return fmt.Errorf("Cannot plan the assets: %s has the in-map path %q, which no asset may have (%v).",
		what, path, refusal)
}

func errSamePath(path string) error {
	return fmt.Errorf("Cannot plan the assets: two assets have the in-map path %q.", path)
}

// errIndexIsAFolder names the folder as the map spells it, as every refusal of a folder where a file of the map
// belongs does.
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
		Hint: makeRoom(asset, inTheWay),
	}
}

func errOntoAFolder(asset Asset, file string) error {
	return &diag.Error{
		Msg:  "Asset " + asset.Target + " would replace a folder in the map.",
		File: file,
		Hint: makeRoom(asset, "that folder"),
	}
}

// makeRoom is the hint of an asset the map has no place for, with what is in the way: one of the map's own assets
// can be imported under another path, and a library's cannot.
func makeRoom(asset Asset, inTheWay string) string {
	if asset.Library != "" {
		return "Remove " + inTheWay + " from the source map."
	}
	return "Import it under another path with assets.paths, or remove " + inTheWay + " from the source map."
}
