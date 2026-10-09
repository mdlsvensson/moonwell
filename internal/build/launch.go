package build

import (
	"os"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

func launch(e *env.Env, config manifest.Launch, userFile, mapPath string) error {
	if config.GameExecutable == nil {
		return errNoGame(userFile)
	}
	executable := *config.GameExecutable
	info, err := os.Stat(executable)
	switch {
	case err != nil:
		return errGameNotFound(executable, userFile)
	case !info.Mode().IsRegular():
		return errGameNotAFile(executable, userFile)
	}
	args := append(slices.Clone(config.Args), "-loadfile", mapPath)
	if err := e.Spawn(executable, args); err != nil {
		return env.NewSpawnError(executable, err, fixGame, userFile)
	}
	return nil
}

const fixGame = "Fix launch.gameExecutable in that file to point at Warcraft III.exe."

func errNoGame(userFile string) error {
	return &diag.Error{
		Msg:  "launch.gameExecutable is not set.",
		File: userFile,
		Hint: "Run `moonwell setup` to create that file if it is missing, then set launch.gameExecutable there to " +
			"your Warcraft III.exe.",
	}
}

func errGameNotFound(executable, userFile string) error {
	return &diag.Error{Msg: "Game executable not found: " + executable, File: userFile, Hint: fixGame}
}

func errGameNotAFile(executable, userFile string) error {
	return &diag.Error{Msg: "Game executable " + executable + " is not a file.", File: userFile, Hint: fixGame}
}
