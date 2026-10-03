package cli

import (
	"context"
	"path/filepath"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/settings"
)

// SettingsCheck lists the internal map files the manifest's settings would change during a build. It reads the
// source map only: no compiler, no staging, no build lock.
func SettingsCheck(ctx context.Context, env *pipeline.Env) ([]mapdir.Change, error) {
	p, err := pipeline.LoadProject(ctx, env)
	if err != nil {
		return nil, err
	}
	mapDir, err := settings.MapDir(env.Root, p.Map.Folder, p.Manifest)
	if err != nil {
		return nil, err
	}
	options := settings.PlanOptions{ManifestFile: p.Manifest, SourceLabel: "maps/" + p.Map.Folder, Root: env.Root}
	changes, err := settings.Plan(mapDir, p.Settings, options)
	if err != nil {
		return nil, err
	}
	for _, change := range changes {
		removed := ""
		if change.Remove {
			removed = " (removed)"
		}
		env.Log.Info("  " + filepath.Base(filepath.FromSlash(change.Name)) + removed)
	}
	env.Log.Info("Map settings valid: " + strconv.Itoa(len(changes)) + " internal file(s) would change during build.")
	return changes, nil
}
