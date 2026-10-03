package cli

import (
	"context"
	"path/filepath"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/editor"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/settings"
)

// CheckResult is what a passed check found.
type CheckResult struct {
	// Modules is the number of modules the entry reaches.
	Modules int
	Entry   string
	Assets  int
}

// Check compiles every module and resolves the graph; no map is staged or packed. refreshObjectIDs rewrites
// src/generated/objects.yue when it is stale instead of failing, which is what dev does.
func Check(ctx context.Context, env *pipeline.Env, refreshObjectIDs bool) (CheckResult, error) {
	// Loading only reads; doing it before taking the lock creates nothing outside a project.
	p, err := pipeline.LoadProject(ctx, env)
	if err != nil {
		return CheckResult{}, err
	}
	release, err := pipeline.AcquireLock(filepath.Join(env.Root, "dist"))
	if err != nil {
		return CheckResult{}, err
	}
	defer release()
	return check(ctx, env, p, refreshObjectIDs)
}

func check(ctx context.Context, env *pipeline.Env, p *project.Project, refreshObjectIDs bool) (CheckResult, error) {
	none := CheckResult{}
	// Objects are planned against the source map in build order (before compiling) and never applied. check never
	// writes the generated module; it fails when it is stale.
	plan, err := pipeline.PlanObjects(env, p)
	if err != nil {
		return none, err
	}
	if refreshObjectIDs {
		if _, err := objects.RefreshIDs(env.Root, plan.Generated); err != nil {
			return none, err
		}
	}
	if err := objects.AssertIDsCurrent(env.Root, plan.Generated); err != nil {
		return none, err
	}
	sourceLabel := "maps/" + p.Map.Folder
	if _, err := editor.Refresh(env.Root, editor.Inputs{Objects: plan.Objects, MapFolder: sourceLabel}); err != nil {
		return none, err
	}
	modules, entry, err := pipeline.CompileProject(ctx, env, p, pipeline.StageOptions{})
	if err != nil {
		return none, err
	}
	// Settings are planned against the source map, in build order, and never written. Without active settings, check
	// still passes when the source map is missing, as it always has.
	if p.Settings.Has() {
		source, err := settings.MapDir(env.Root, p.Map.Folder, p.Manifest)
		if err != nil {
			return none, err
		}
		options := settings.PlanOptions{ManifestFile: p.Manifest, SourceLabel: sourceLabel, Root: env.Root}
		if _, err := settings.Plan(source, p.Settings, options); err != nil {
			return none, err
		}
	}
	mapDir, stateFile, err := assets.Locations(env.Root, p.Map.Folder)
	if err != nil {
		return none, err
	}
	// After compiling, which syncs the libraries: the files they ship are assets too.
	config := assets.Config{Paths: p.Assets.Paths, Exclude: p.Assets.Exclude}
	var imported []*assets.Asset
	var replaced []string
	if fsx.Exists(mapDir) {
		planned, err := assets.PlanAssets(ctx, env.Root, mapDir, stateFile, config, p.LibraryKeys())
		if err != nil {
			return none, err
		}
		imported, replaced = planned.Assets, planned.Replaced
	} else if imported, replaced, err = assets.CollectProject(env.Root, config, p.LibraryKeys()); err != nil {
		return none, err
	}
	for _, line := range replaced {
		env.Log.Info(line)
	}
	env.Log.Info("Check passed: " + strconv.Itoa(len(modules)) + " module(s) reachable from " + entry + ", " +
		strconv.Itoa(len(imported)) + " asset(s).")
	return CheckResult{Modules: len(modules), Entry: entry, Assets: len(imported)}, nil
}
