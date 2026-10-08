package build

import (
	"os"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

func launch(e *env.Env, how manifest.Launch, mapPath string) error {
	if how.GameExecutable == nil {
		return errNoGame()
	}
	game := *how.GameExecutable
	info, err := os.Stat(game)
	switch {
	case err != nil:
		return errGameNotFound(game)
	case !info.Mode().IsRegular():
		return errGameNotAFile(game)
	}
	args := append(slices.Clone(how.Args), "-loadfile", mapPath)
	if err := e.Spawn(game, args); err != nil {
		return env.NewSpawnError(game, err, fixGame, manifest.LocalFile)
	}
	return nil
}

const fixGame = "Fix launch.gameExecutable in " + manifest.LocalFile + " to point at Warcraft III.exe."

func errNoGame() error {
	return &diag.Error{
		Msg:  "launch.gameExecutable is not set.",
		File: manifest.LocalFile,
		Hint: "Run `moonwell setup` to create " + manifest.LocalFile + ", then set launch.gameExecutable there to " +
			"your Warcraft III.exe.",
	}
}

func errGameNotFound(game string) error {
	return &diag.Error{Msg: "Game executable not found: " + game, File: manifest.LocalFile, Hint: fixGame}
}

func errGameNotAFile(game string) error {
	return &diag.Error{Msg: "Game executable " + game + " is not a file.", File: manifest.LocalFile, Hint: fixGame}
}
