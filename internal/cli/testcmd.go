package cli

import (
	"context"
	"path/filepath"

	"github.com/mdlsvensson/moonwell/internal/pipeline"
)

// Test stages the map as a folder and starts Warcraft III on it.
func Test(ctx context.Context, env *pipeline.Env, options pipeline.StageOptions) error {
	// Loading only reads; doing it before taking the lock creates nothing outside a project.
	p, err := pipeline.LoadProject(ctx, env)
	if err != nil {
		return err
	}
	release, err := pipeline.AcquireLock(filepath.Join(env.Root, "dist"))
	if err != nil {
		return err
	}
	defer release()
	mapDir, _, err := pipeline.PrepareStage(ctx, env, p, options)
	if err != nil {
		return err
	}
	if err := LaunchGame(p.Launch, mapDir, env.Spawn); err != nil {
		return err
	}
	env.Log.Info("Launched Warcraft III with " + relative(env.Root, mapDir) + ".")
	return nil
}
