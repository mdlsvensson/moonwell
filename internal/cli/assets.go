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

func runAssetsCheck(ctx context.Context, e *env.Env, _ commandArgs) error {
	return syncOrCheckAssets(ctx, e, false)
}

func runAssetsSync(ctx context.Context, e *env.Env, _ commandArgs) error {
	return syncOrCheckAssets(ctx, e, true)
}

func syncOrCheckAssets(ctx context.Context, e *env.Env, write bool) error {
	project, err := build.Load(ctx, e)
	if err != nil {
		return err
	}
	release, err := build.AcquireLock(e.Root)
	if err != nil {
		return err
	}
	defer release()
	source, err := build.OpenSource(project)
	if err != nil {
		return err
	}
	if err := checkLuaScriptMap(source); err != nil {
		return err
	}
	synced, err := library.Sync(ctx, e, project.Libraries, project.ManifestName)
	if err != nil {
		return err
	}
	plan, replaced, err := build.PlanAssets(ctx, source, project, synced)
	if err != nil {
		return err
	}
	logAssetPlan(e.Log, source, plan, replaced)
	if !write {
		logNothingWritten(e.Log, plan)
		return nil
	}
	return writeAssets(ctx, e, project, source, plan)
}

func writeAssets(
	ctx context.Context, e *env.Env, project *manifest.Project, source *mapdir.Folder, plan *assets.Result,
) error {
	stateFile, err := build.AssetStatePath(project)
	if err != nil {
		return err
	}
	if err := assets.Sync(ctx, source, plan, project.Root, stateFile); err != nil {
		return err
	}
	e.Log.Info("Synced " + strconv.Itoa(len(plan.Assets)) + " asset(s) into " + source.DisplayPath("") + " (" +
		strconv.Itoa(len(plan.Changes)) + " file change(s)). Reopen the map in World Editor.")
	return nil
}

func checkLuaScriptMap(source *mapdir.Folder) error {
	for _, name := range []string{"war3map.lua", "war3map.w3i"} {
		if !source.HasFile(name) {
			return errMapLacks(name, source.DisplayPath(""))
		}
	}
	return nil
}

func collectSyncedAssets(
	ctx context.Context, e *env.Env, project *manifest.Project,
) (collected []assets.Asset, replaced []string, err error) {
	synced, err := library.Sync(ctx, e, project.Libraries, project.ManifestName)
	if err != nil {
		return nil, nil, err
	}
	return build.CollectAssets(project, synced)
}

func logAssetPlan(log *env.Logger, source *mapdir.Folder, plan *assets.Result, replaced []string) {
	for _, asset := range plan.Assets {
		origin := asset.Source
		if asset.Library != "" {
			origin = "library " + asset.Library + ": " + asset.Source
		}
		log.Info(origin + " -> " + strings.ReplaceAll(asset.Target, "/", `\`))
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

func logNothingWritten(log *env.Logger, plan *assets.Result) {
	log.Info("Checked " + strconv.Itoa(len(plan.Assets)) + " asset(s); assets:sync would make " +
		strconv.Itoa(len(plan.Changes)) + " file change(s). Nothing was written.")
}

func errMapLacks(name, displayPath string) error {
	return &diag.Error{
		Msg:  "The source map has no " + name + ".",
		File: displayPath,
		Hint: "Save the map in World Editor in folder format with Lua as the script language.",
	}
}
