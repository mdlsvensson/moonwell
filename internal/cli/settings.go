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

// runSettingsCheck is `moonwell settings:check`: it lists the files of the map that the manifest's settings
// change in a build. It reads the manifest, the source map and the preview picture the settings name, runs no
// compiler, takes no build lock and writes nothing.
func runSettingsCheck(ctx context.Context, e *env.Env, _ call) error {
	p, err := build.Load(ctx, e)
	if err != nil {
		return err
	}
	source, err := build.Source(p) // maps/<map.folder>, opened as a build opens it
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

// sayChangedBySettings logs each file the settings change, by its name without the folders it is in, a file that
// a build takes out of the map as removed, and then how many files there are.
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
