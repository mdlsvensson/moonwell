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

func runSettingsCheck(ctx context.Context, e *env.Env, _ call) error {
	p, err := build.Load(ctx, e)
	if err != nil {
		return err
	}
	source, err := build.Source(p)
	if err != nil {
		return err
	}
	changes, err := settings.Plan(source, p)
	if err != nil {
		return err
	}
	sayChangedBySettings(e.Log, changes)
	return nil
}

func sayChangedBySettings(log *env.Logger, changes []mapdir.Change) {
	for _, change := range changes {
		removed := ""
		if change.Remove {
			removed = " (removed)"
		}
		log.Info("  " + path.Base(change.Name) + removed)
	}
	log.Info("Map settings valid: " + strconv.Itoa(len(changes)) + " internal file(s) would change during build.")
}
