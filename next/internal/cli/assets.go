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
// end; the source map, which must be one World Editor saved with a Lua script; the libraries, synced; the plan
// of the import, made as a build makes it; what the plan holds, said; and for a sync the writing.
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
	source, err := build.Source(p)
	if err != nil {
		return err
	}
	if err := savedWithLuaScript(source); err != nil {
		return err
	}
	// Under the lock: a sync writes .moonwell/ and moonwell.lock in several steps, and a build beside it would
	// read them half written.
	synced, err := library.Sync(ctx, e, p.Libraries, p.File)
	if err != nil {
		return err
	}
	// The plan is made on the source folder itself and on no view of it: a sync writes the folder its plan was
	// made from.
	plan, replaced, err := build.PlanAssets(ctx, source, p, synced)
	if err != nil {
		return err
	}
	sayImport(e.Log, source, plan, replaced)
	if !write {
		sayNothingWritten(e.Log, plan)
		return nil
	}
	return writeImport(ctx, e, p, source, plan)
}

// writeImport is the last step of assets:sync: it writes the planned import into the source map, keeps the
// record of the files assets:sync owns there in the file build.StateFile names, and says what it wrote.
func writeImport(
	ctx context.Context, e *env.Env, p *manifest.Project, source *mapdir.Folder, plan *assets.Result,
) error {
	stateFile, err := build.StateFile(p)
	if err != nil {
		return err
	}
	if err := assets.Sync(ctx, source, plan, stateFile); err != nil {
		return err
	}
	e.Log.Info("Synced " + strconv.Itoa(len(plan.Assets)) + " asset(s) into " + source.Label("") + " (" +
		strconv.Itoa(len(plan.Changes)) + " file change(s)). Reopen the map in World Editor.")
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

// sayNothingWritten logs the last line of assets:check: how many assets the plan holds, how many files of the
// map assets:sync would change for them, and that the check changed none.
func sayNothingWritten(log *env.Logger, plan *assets.Result) {
	log.Info("Checked " + strconv.Itoa(len(plan.Assets)) + " asset(s); assets:sync would make " +
		strconv.Itoa(len(plan.Changes)) + " file change(s). Nothing was written.")
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
