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

type started []string

func recordingSpawn(e *env.Env) *[]started {
	starts := &[]started{}
	e.Spawn = func(program string, args []string) error {
		*starts = append(*starts, append(started{program}, args...))
		return nil
	}
	return starts
}

func TestLaunchExplainsAMissingOrWrongExecutable(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	const set = "Run `moonwell setup` to create moonwell.local.pkl, then set launch.gameExecutable there to your " +
		"Warcraft III.exe."
	const fix = "Fix launch.gameExecutable in moonwell.local.pkl to point at Warcraft III.exe."
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
			diagErr := asError(t, launch(e, tt.launch, "map"), tt.name)
			if diagErr.Msg != tt.msg || diagErr.File != "moonwell.local.pkl" || diagErr.Hint != tt.hint {
				t.Errorf("error = %+v", diagErr)
			}
		})
	}
}

func TestLaunchPassesTheLaunchArgsAndLoadfile(t *testing.T) {
	e, _ := testkit.Env(t, t.TempDir())
	starts := recordingSpawn(e)
	game := testkit.WriteFile(t, t.TempDir(), "Warcraft III.exe", nil)
	args := make([]string, 1, 4)
	args[0] = "-launch"
	how := manifest.Launch{GameExecutable: &game, Args: args}
	err := launch(e, how, "C:/map.w3x")
	want := started{game, "-launch", "-loadfile", "C:/map.w3x"}
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
	diagErr := asError(t, launch(e, manifest.Launch{GameExecutable: &game}, "map"), "not a program")
	if !strings.HasPrefix(diagErr.Msg, "Cannot run '"+game+"': ") || diagErr.File != "moonwell.local.pkl" ||
		!strings.Contains(diagErr.Hint, "moonwell.local.pkl") || diagErr.Cause == nil {
		t.Errorf("error = %+v", diagErr)
	}
}
