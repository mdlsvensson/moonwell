package cli

import (
	"context"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
)

func runAssetsCheck(ctx context.Context, e *env.Env, _ call) error {
	return importAssets(ctx, e, false)
}

func runAssetsSync(ctx context.Context, e *env.Env, _ call) error {
	return importAssets(ctx, e, true)
}

func importAssets(ctx context.Context, e *env.Env, write bool) error {
	p, err := build.Load(ctx, e)
	if err != nil {
		return err
	}
	release, err := build.TakeLock(e.Root)
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
	synced, err := library.Sync(ctx, e, p.Libraries, p.File)
	if err != nil {
		return err
	}
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

func writeImport(
	ctx context.Context, e *env.Env, p *manifest.Project, source *mapdir.Folder, plan *assets.Result,
) error {
	stateFile, err := build.OwnershipFile(p)
	if err != nil {
		return err
	}
	if err := assets.Sync(ctx, source, plan, p.Root, stateFile); err != nil {
		return err
	}
	e.Log.Info("Synced " + strconv.Itoa(len(plan.Assets)) + " asset(s) into " + source.DisplayPath("") + " (" +
		strconv.Itoa(len(plan.Changes)) + " file change(s)). Reopen the map in World Editor.")
	return nil
}

func savedWithLuaScript(source *mapdir.Folder) error {
	for _, name := range []string{"war3map.lua", "war3map.w3i"} {
		if !source.HasFile(name) {
			return errMapLacks(name, source.DisplayPath(""))
		}
	}
	return nil
}

func syncedAssets(
	ctx context.Context, e *env.Env, p *manifest.Project,
) (found []assets.Asset, replaced []string, err error) {
	synced, err := library.Sync(ctx, e, p.Libraries, p.File)
	if err != nil {
		return nil, nil, err
	}
	return build.Assets(p, synced)
}

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
		log.Info(action + " " + source.DisplayPath(change.Path))
	}
}

func sayNothingWritten(log *env.Logger, plan *assets.Result) {
	log.Info("Checked " + strconv.Itoa(len(plan.Assets)) + " asset(s); assets:sync would make " +
		strconv.Itoa(len(plan.Changes)) + " file change(s). Nothing was written.")
}

func errMapLacks(name, folder string) error {
	return &diag.Error{
		Msg:  "The source map has no " + name + ".",
		File: folder,
		Hint: "Save the map in World Editor in folder format with Lua as the script language.",
	}
}
