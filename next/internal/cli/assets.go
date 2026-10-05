package cli

import (
	"context"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/next/internal/assets"
	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/library"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
)

// runAssetsCheck is `moonwell assets:check`: it says what assets:sync would change in the source map, and
// writes nothing there. It syncs the libraries into the project, as assets:sync does.
func runAssetsCheck(ctx context.Context, e *env.Env, _ call) error {
	return importAssets(ctx, e, false)
}

// runAssetsSync is `moonwell assets:sync`: it writes the project's assets, and the files its libraries ship,
// into the source map for World Editor, with the map's index of imports, and keeps the record of the files it
// owns there.
func runAssetsSync(ctx context.Context, e *env.Env, _ call) error {
	return importAssets(ctx, e, true)
}

// importAssets is assets:check and, with write, assets:sync: the two plan the same import into the source map
// and say the same of it, and the second then writes it. Their steps: the manifest; the build lock, held to the
// end; the source map, which must be one World Editor saved with a Lua script; the libraries, synced, and the
// assets; the record of the files assets:sync owns in the map; the plan; what the plan holds, said; and for a
// sync the writing.
//
// A command that is told to stop while it plans has written nothing, and says so; a sync that is told to stop
// while it writes puts back what it wrote.
func importAssets(ctx context.Context, e *env.Env, write bool) error {
	// Before the lock, so that a command outside a project makes no dist folder there.
	p, err := build.Load(ctx, e)
	if err != nil {
		return err
	}
	release, err := build.Acquire(e.Root)
	if err != nil {
		return err
	}
	defer release()
	// The plan is made on the source folder itself and on no view of it: a sync writes the folder its plan was
	// made from.
	source, err := build.Source(p)
	if err != nil {
		return err
	}
	if err := savedWithLuaScript(source); err != nil {
		return err
	}
	found, replaced, err := syncedAssets(ctx, e, p)
	if err != nil {
		return err
	}
	stateFile, owned, err := ownership(p)
	if err != nil {
		return err
	}
	plan, err := assets.Plan(ctx, source, found, owned)
	if err != nil {
		return err
	}
	sayImport(e.Log, source, plan, replaced)
	count, changes := strconv.Itoa(len(plan.Assets)), strconv.Itoa(len(plan.Changes))
	if !write {
		e.Log.Info("Checked " + count + " asset(s); assets:sync would make " + changes + " file change(s). " +
			"Nothing was written.")
		return nil
	}
	if err := assets.Sync(ctx, source, plan, stateFile); err != nil {
		return err
	}
	e.Log.Info("Synced " + count + " asset(s) into " + source.Label("") + " (" + changes + " file change(s)). " +
		"Reopen the map in World Editor.")
	return nil
}

// savedWithLuaScript refuses a source map that World Editor did not save in folder format with Lua as its script
// language: one without its script or without its info file, in any letter case. A folder under either name is
// no such file.
func savedWithLuaScript(source *mapdir.Folder) error {
	for _, name := range []string{"war3map.lua", "war3map.w3i"} {
		if !source.Has(name) {
			return errMapLacks(name, source.Label(""))
		}
	}
	return nil
}

// syncedAssets syncs the manifest's libraries into the project, and collects what a build imports: the map's own
// assets and the files the libraries ship, with the lines that say which of a library's files the map's own
// replace. Its caller holds the build lock: a sync writes .moonwell/ and moonwell.lock in several steps, and a
// build beside it would read them half written.
func syncedAssets(
	ctx context.Context, e *env.Env, p *manifest.Project,
) (found []assets.Asset, replaced []string, err error) {
	synced, err := library.Sync(ctx, e, p.Libraries, p.File)
	if err != nil {
		return nil, nil, err
	}
	return build.Assets(p, synced)
}

// ownership is the record of the files of the project's source map that assets:sync owns: the file that holds
// it, and what the file says. A project without the file owns nothing.
func ownership(p *manifest.Project) (file string, owned assets.State, err error) {
	file, err = build.StateFile(p)
	if err != nil {
		return "", assets.State{}, err
	}
	owned, err = assets.ReadState(file)
	return file, owned, err
}

// sayImport logs what a planned import holds: each asset with the in-map path it is imported as, written with
// backslashes as World Editor shows one, and a library's asset with the library's key; the lines that say which
// of a library's files the map's own replace; and each file of the map that the import writes or removes, as its
// path from the project folder.
func sayImport(log *env.Logger, source *mapdir.Folder, plan *assets.Result, replaced []string) {
	for _, asset := range plan.Assets {
		from := asset.Source
		if asset.Library != "" {
			from = "library " + asset.Library + ": " + asset.Source
		}
		log.Info(from + " -> " + strings.ReplaceAll(asset.Target, "/", `\`))
	}
	for _, line := range replaced {
		log.Info(line)
	}
	for _, change := range plan.Changes {
		action := "write"
		if change.Remove {
			action = "delete"
		}
		log.Info(action + " " + source.Label(change.Name))
	}
}

// ---- errors ----

// errMapLacks names the map folder, from the project folder: the file is one World Editor writes when it saves
// the map.
func errMapLacks(name, folder string) error {
	return &diag.Error{
		Msg:  "The source map has no " + name + ".",
		File: folder,
		Hint: "Save the map in World Editor in folder format with Lua as the script language.",
	}
}
