package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/project"
)

// Assets is assets:check and, with sync, assets:sync. The first shows what the second would change; the second
// writes assets/ and the files the libraries ship into the source map for World Editor. It stops before it writes
// when ctx is cancelled (Ctrl+C) while planning, and undoes its writes when it is cancelled later. Both sync the
// libraries first.
func Assets(ctx context.Context, env *pipeline.Env, sync bool) (*assets.Plan, error) {
	// Loading only reads; doing it before taking the lock creates nothing outside a project.
	p, err := project.Load(ctx, env.Root, env.Run)
	if err != nil {
		return nil, err
	}
	release, err := pipeline.AcquireLock(filepath.Join(env.Root, "dist"))
	if err != nil {
		return nil, err
	}
	defer release()

	mapDir, stateFile, err := assets.Locations(env.Root, p.Map.Folder)
	if err != nil {
		return nil, err
	}
	for _, name := range []string{"war3map.lua", "war3map.w3i"} {
		if info, err := os.Stat(filepath.Join(mapDir, name)); err != nil || !info.Mode().IsRegular() {
			return nil, &diag.Error{
				Msg:  "The source map has no " + name + ".",
				File: "maps/" + p.Map.Folder,
				Hint: "Save the map in World Editor in folder format with Lua as the script language.",
			}
		}
	}
	if err := pipeline.SyncLibraries(ctx, env, p); err != nil {
		return nil, err
	}
	config := assets.Config{Paths: p.Assets.Paths, Exclude: p.Assets.Exclude}
	plan, err := assets.PlanAssets(ctx, env.Root, mapDir, stateFile, config, p.LibraryKeys())
	if err != nil {
		return nil, err
	}
	for _, asset := range plan.Assets {
		source := asset.Source
		if asset.Library != "" {
			source = "library " + asset.Library + ": " + asset.Source
		}
		env.Log.Info(source + " -> " + strings.ReplaceAll(asset.Target, "/", `\`))
	}
	for _, line := range plan.Replaced {
		env.Log.Info(line)
	}
	for _, change := range plan.Changes {
		action := "write"
		if change.Remove {
			action = "delete"
		}
		env.Log.Info(action + " " + relative(env.Root, change.File))
	}
	count, changes := strconv.Itoa(len(plan.Assets)), strconv.Itoa(len(plan.Changes))
	if !sync {
		env.Log.Info("Checked " + count + " asset(s); assets:sync would make " + changes + " file change(s). " +
			"Nothing was written.")
		return plan, nil
	}
	if err := assets.ApplyPlan(ctx, plan, stateFile); err != nil {
		return nil, err
	}
	env.Log.Info("Synced " + count + " asset(s) into maps/" + p.Map.Folder + " (" + changes + " file change(s)). " +
		"Reopen the map in World Editor.")
	return plan, nil
}
