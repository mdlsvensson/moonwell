package cli

import (
	"os"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/project"
)

const (
	setExecutable = "Run `moonwell setup` to create moonwell.local.pkl, then set launch.gameExecutable there to your " +
		"Warcraft III.exe."
	fixExecutable = "Fix launch.gameExecutable in moonwell.local.pkl to point at Warcraft III.exe."
)

// LaunchGame starts Warcraft III on mapPath, a staged folder map or a .w3x, through spawn.
func LaunchGame(launch project.Launch, mapPath string, spawn func(command string, args []string) error) error {
	const file = "moonwell.local.pkl"
	if launch.GameExecutable == nil {
		return &diag.Error{Msg: "launch.gameExecutable is not set.", File: file, Hint: setExecutable}
	}
	executable := *launch.GameExecutable
	info, err := os.Stat(executable)
	if err != nil {
		return &diag.Error{Msg: "Game executable not found: " + executable, File: file, Hint: fixExecutable}
	}
	if !info.Mode().IsRegular() {
		return &diag.Error{Msg: "Game executable " + executable + " is not a file.", File: file, Hint: fixExecutable}
	}
	args := append(append([]string{}, launch.Args...), "-loadfile", mapPath)
	if err := spawn(executable, args); err != nil {
		return proc.SpawnError(executable, err, fixExecutable, file)
	}
	return nil
}
