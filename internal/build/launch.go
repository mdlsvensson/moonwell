package build

import (
	"os"
	"slices"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

// machineManifest is the manifest that holds what is this machine's, the game among it: the file a failure to
// start the game names, whichever manifest was evaluated.
const machineManifest = "moonwell.local.pkl"

// launch starts Warcraft III on a map, a folder or an archive, and does not wait for it: the game is given the
// manifest's launch.args, then -loadfile and the map's path on disk. The game must be a file that is there; what
// the system says when it cannot start it is worded for the user.
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
		return env.SpawnError(game, err, fixGame, machineManifest)
	}
	return nil
}

// ---- errors ----

// fixGame ends a failure of the game the manifest names.
const fixGame = "Fix launch.gameExecutable in " + machineManifest + " to point at Warcraft III.exe."

func errNoGame() error {
	return &diag.Error{
		Msg:  "launch.gameExecutable is not set.",
		File: machineManifest,
		Hint: "Run `moonwell setup` to create " + machineManifest + ", then set launch.gameExecutable there to your " +
			"Warcraft III.exe.",
	}
}

func errGameNotFound(game string) error {
	return &diag.Error{Msg: "Game executable not found: " + game, File: machineManifest, Hint: fixGame}
}

func errGameNotAFile(game string) error {
	return &diag.Error{Msg: "Game executable " + game + " is not a file.", File: machineManifest, Hint: fixGame}
}
