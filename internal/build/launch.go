package build

import (
	"os"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

func launch(e *env.Env, config manifest.Launch, mapPath string) error {
	if config.GameExecutable == nil {
		return errNoGame()
	}
	executable := *config.GameExecutable
	info, err := os.Stat(executable)
	switch {
	case err != nil:
		return errGameNotFound(executable)
	case !info.Mode().IsRegular():
		return errGameNotAFile(executable)
	}
	args := append(slices.Clone(config.Args), "-loadfile", mapPath)
	if err := e.Spawn(executable, args); err != nil {
		return env.NewSpawnError(executable, err, fixGame, manifest.LocalManifest)
	}
	return nil
}

const fixGame = "Fix launch.gameExecutable in " + manifest.LocalManifest + " to point at Warcraft III.exe."

func errNoGame() error {
	return &diag.Error{
		Msg:  "launch.gameExecutable is not set.",
		File: manifest.LocalManifest,
		Hint: "Run `moonwell setup` to create " + manifest.LocalManifest + ", then set launch.gameExecutable there to " +
			"your Warcraft III.exe.",
	}
}

func errGameNotFound(executable string) error {
	return &diag.Error{Msg: "Game executable not found: " + executable, File: manifest.LocalManifest, Hint: fixGame}
}

func errGameNotAFile(executable string) error {
	return &diag.Error{Msg: "Game executable " + executable + " is not a file.", File: manifest.LocalManifest, Hint: fixGame}
}
