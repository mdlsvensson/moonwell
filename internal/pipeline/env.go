// Package pipeline composes the planners and the compiler into the steps commands share: compiling a project,
// planning its objects and preparing the staged map.
package pipeline

import (
	"github.com/mdlsvensson/moonwell/internal/logging"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

// Env is everything a command needs from the outside world; tests substitute parts of it.
type Env struct {
	// Root is the project folder.
	Root string
	Log  *logging.Logger
	// Run runs pkl and yue.
	Run proc.RunFunc
	// Install downloads the compiler, and libraries through its Fetch.
	Install yue.InstallDeps
	// Spawn starts the game and does not wait for it.
	Spawn func(command string, args []string) error
}

// NewEnv is the real world for the project at root: programs are run, and downloads go over HTTP into the user's
// cache. Spawn is left for the command line to set.
func NewEnv(root string, log *logging.Logger) *Env {
	return &Env{Root: root, Log: log, Run: proc.Run, Install: yue.DefaultInstallDeps(log, proc.Run)}
}
