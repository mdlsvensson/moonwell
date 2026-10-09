package build

import (
	"os"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

func launch(e *env.Env, config manifest.Launch, mapPath string) error {
	userFilePath := manifest.UserFilePath(e)
	if config.GameExecutable == nil {
		return errNoGame(userFilePath)
	}
	executable := *config.GameExecutable
	info, err := os.Stat(executable)
	switch {
	case err != nil:
		return errGameNotFound(executable, userFilePath)
	case !info.Mode().IsRegular():
		return errGameNotAFile(executable, userFilePath)
	}
	args := append(slices.Clone(config.Args), "-loadfile", mapPath)
	if err := e.Spawn(executable, args); err != nil {
		return env.NewSpawnError(executable, err, fixGame, userFilePath)
	}
	return nil
}

const fixGame = "Fix launch.gameExecutable in that file to point at Warcraft III.exe."

func errNoGame(userFilePath string) error {
	return &diag.Error{
		Msg:  "launch.gameExecutable is not set.",
		File: userFilePath,
		Hint: "Set launch.gameExecutable there to your Warcraft III.exe. Moonwell's install script makes that file; " +
			"if it is missing, create it as \"Your machine\" in Moonwell's README shows.",
	}
}

func errGameNotFound(executable, userFilePath string) error {
	return &diag.Error{Msg: "Game executable not found: " + executable, File: userFilePath, Hint: fixGame}
}

func errGameNotAFile(executable, userFilePath string) error {
	return &diag.Error{Msg: "Game executable " + executable + " is not a file.", File: userFilePath, Hint: fixGame}
}
