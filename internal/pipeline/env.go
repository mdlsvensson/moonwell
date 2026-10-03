// Package pipeline composes the planners and the compiler into the steps commands share: compiling a project,
// planning its objects and preparing the staged map.
package pipeline

import (
	"context"

	"github.com/mdlsvensson/moonwell/internal/logging"
	"github.com/mdlsvensson/moonwell/internal/pkl"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

// Env is everything a command needs from the outside world; tests substitute parts of it.
type Env struct {
	// Root is the project folder.
	Root string
	Log  *logging.Logger
	// Run runs pkl and yue.
	Run proc.RunFunc
	// Install downloads the compiler and Pkl, and libraries through its Fetch.
	Install yue.InstallDeps
	// PklDownloads are the Pkl executables Moonwell may download when PATH has no Pkl it can use, by platform
	// (pkl.Known); nil downloads none.
	PklDownloads map[string]pkl.Asset
	// Spawn starts the game and does not wait for it.
	Spawn func(command string, args []string) error

	// pkl is the pkl program, once Pkl has found it.
	pkl string
}

// NewEnv is the real world for the project at root: programs are run, and downloads go over HTTP into the user's
// cache. Spawn is left for the command line to set.
func NewEnv(root string, log *logging.Logger) *Env {
	return &Env{
		Root: root, Log: log, Run: proc.Run, Install: yue.DefaultInstallDeps(log, proc.Run), PklDownloads: pkl.Known,
	}
}

// Pkl returns the pkl program to run (pkl.Ensure), finding it on the first call only, so that `dev` checks and warns
// once.
func (env *Env) Pkl(ctx context.Context) (string, error) {
	if env.pkl != "" {
		return env.pkl, nil
	}
	program, err := pkl.Ensure(ctx, env.PklDeps())
	if err != nil {
		return "", err
	}
	env.pkl = program
	return program, nil
}

// PklDeps is what finding and installing Pkl needs, from this Env.
func (env *Env) PklDeps() pkl.Deps {
	return pkl.Deps{
		Fetch:     env.Install.Fetch,
		Run:       env.Run,
		CacheRoot: env.Install.CacheRoot,
		Platform:  env.Install.Platform,
		Known:     env.PklDownloads,
		Log:       env.Log,
	}
}

// LoadProject evaluates the project's manifest with the pkl program Pkl finds.
func LoadProject(ctx context.Context, env *Env) (*project.Project, error) {
	program, err := env.Pkl(ctx)
	if err != nil {
		return nil, err
	}
	return project.Load(ctx, env.Root, program, env.Run)
}
