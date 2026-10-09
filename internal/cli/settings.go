package cli

import (
	"context"
	"path"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/settings"
)

func runSettingsCheck(ctx context.Context, e *env.Env, _ commandArgs) error {
	project, err := build.Load(ctx, e)
	if err != nil {
		return err
	}
	source, err := build.OpenSource(project)
	if err != nil {
		return err
	}
	changes, err := settings.Plan(source, project)
	if err != nil {
		return err
	}
	logSettingsChanges(e.Log, changes)
	return nil
}

func logSettingsChanges(log *env.Logger, changes []mapdir.Change) {
	for _, change := range changes {
		removed := ""
		if change.Remove {
			removed = " (removed)"
		}
		log.Info("  " + path.Base(change.Path) + removed)
	}
	log.Info("Map settings valid: " + strconv.Itoa(len(changes)) + " internal file(s) would change during build.")
}
