package build

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

type spawnCall []string

func recordSpawns(e *env.Env) *[]spawnCall {
	starts := &[]spawnCall{}
	e.Spawn = func(program string, args []string) error {
		*starts = append(*starts, append(spawnCall{program}, args...))
		return nil
	}
	return starts
}

func TestLaunchExplainsAMissingOrWrongExecutable(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	const set = "Set launch.gameExecutable there to your Warcraft III.exe. Moonwell's install script makes that file; " +
		"if it is missing, create it as \"Your machine\" in Moonwell's README shows."
	const fix = "Fix launch.gameExecutable in that file to point at Warcraft III.exe."
	missing := filepath.Join(t.TempDir(), "Warcraft III.exe")
	dir := t.TempDir()
	tests := []struct {
		name   string
		launch manifest.Launch
		msg    string
		hint   string
	}{
		{"no executable", manifest.Launch{}, "launch.gameExecutable is not set.", set},
		{"a missing executable", manifest.Launch{GameExecutable: &missing},
			"Game executable not found: " + missing, fix},
		{"a folder", manifest.Launch{GameExecutable: &dir}, "Game executable " + dir + " is not a file.", fix},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagErr := asDiagError(t, launch(e, tt.launch, "map"), tt.name)
			if diagErr.Msg != tt.msg || diagErr.File != manifest.UserFilePath(e) || diagErr.Hint != tt.hint {
				t.Errorf("error = %+v", diagErr)
			}
		})
	}
}

func TestLaunchPassesTheLaunchArgsAndLoadfile(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	starts := recordSpawns(e)
	game := testkit.WriteFile(t, t.TempDir(), "Warcraft III.exe", nil)
	args := make([]string, 1, 4)
	args[0] = "-launch"
	how := manifest.Launch{GameExecutable: &game, Args: args}
	err := launch(e, how, "C:/map.w3x")
	want := spawnCall{game, "-launch", "-loadfile", "C:/map.w3x"}
	if err != nil || len(*starts) != 1 || !slices.Equal((*starts)[0], want) {
		t.Errorf("started %q, %v, want %q", *starts, err, want)
	}
	if whole := args[:cap(args)]; !slices.Equal(whole, []string{"-launch", "", "", ""}) || len(how.Args) != 1 {
		t.Errorf("the manifest's arguments are %q with %q behind them", how.Args, whole[1:])
	}
}

func TestLaunchReportsAGameThatFailsToStart(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	e.Spawn = env.SpawnDetached
	game := testkit.WriteFile(t, t.TempDir(), "Warcraft III.exe", []byte("not a program"))
	diagErr := asDiagError(t, launch(e, manifest.Launch{GameExecutable: &game}, "map"), "not a program")
	if !strings.HasPrefix(diagErr.Msg, "Cannot run '"+game+"': ") || diagErr.File != manifest.UserFilePath(e) ||
		!strings.Contains(diagErr.Hint, "launch.gameExecutable") || diagErr.Cause == nil {
		t.Errorf("error = %+v", diagErr)
	}
}
