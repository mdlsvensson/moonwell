package cli_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/cli"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/models"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// These tests need no other program.

func buildProject(root, folder, manifest string) *project.Project {
	return &project.Project{
		Root:     root,
		Manifest: manifest,
		Map:      project.Map{Folder: "map.w3x", Entry: "src/main.yue"},
		Build:    project.Build{Folder: folder},
	}
}

func TestArchivePathPlacesTheArchiveUnderBuildFolder(t *testing.T) {
	root := t.TempDir()
	path, err := cli.ArchivePath(root, buildProject(root, "dist/bin", "moonwell.pkl"))
	if err != nil || path != filepath.Join(root, "dist", "bin", "map.w3x") {
		t.Errorf("ArchivePath = %q, %v", path, err)
	}
}

func TestArchivePathRefusesADirectoryOrAPathOutsideTheProjectNamingTheEvaluatedManifest(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "maps", "map.w3x"), 0o777); err != nil {
		t.Fatal(err)
	}
	// A directory, such as the source map.
	_, err := cli.ArchivePath(root, buildProject(root, "maps", "moonwell.local.pkl"))
	e := asError(t, err, "the source map")
	if e.Msg != "The build output maps/map.w3x is a directory; refusing to replace it." || e.File != "moonwell.local.pkl" ||
		e.Hint != "Set build.folder to a folder that only holds build output, such as dist/bin." {
		t.Errorf("error = %+v", e)
	}
	for _, folder := range []string{"..", "../other"} {
		_, err := cli.ArchivePath(root, buildProject(root, folder, "moonwell.local.pkl"))
		e := asError(t, err, folder)
		if !strings.HasPrefix(e.Msg, "The build output ") || !strings.HasSuffix(e.Msg, " is outside the project.") || e.File != "moonwell.local.pkl" ||
			e.Hint != "Set build.folder to a folder inside the project, such as dist/bin." {
			t.Errorf("%s: %+v", folder, e)
		}
	}
}

func TestLaunchGameExplainsAMissingOrWrongExecutable(t *testing.T) {
	never := func(string, []string) error { t.Error("the game was started"); return nil }
	e := asError(t, cli.LaunchGame(project.Launch{}, "map", never), "no executable")
	if e.Msg != "launch.gameExecutable is not set." || e.File != "moonwell.local.pkl" ||
		e.Hint != "Run `moonwell setup` to create moonwell.local.pkl, then set launch.gameExecutable there to your Warcraft III.exe." {
		t.Errorf("error = %+v", e)
	}
	const fix = "Fix launch.gameExecutable in moonwell.local.pkl to point at Warcraft III.exe."
	missing := filepath.Join(t.TempDir(), "Warcraft III.exe")
	e = asError(t, cli.LaunchGame(project.Launch{GameExecutable: &missing}, "map", never), "a missing executable")
	if e.Msg != "Game executable not found: "+missing || e.File != "moonwell.local.pkl" || e.Hint != fix {
		t.Errorf("error = %+v", e)
	}
	// A directory is not an executable.
	dir := t.TempDir()
	e = asError(t, cli.LaunchGame(project.Launch{GameExecutable: &dir}, "map", never), "a folder")
	if e.Msg != "Game executable "+dir+" is not a file." || e.File != "moonwell.local.pkl" || e.Hint != fix {
		t.Errorf("error = %+v", e)
	}
}

func TestLaunchGamePassesTheLaunchArgsAndLoadfile(t *testing.T) {
	exe := testkit.WriteFile(t, t.TempDir(), "Warcraft III.exe", nil)
	var calls [][]string
	launch := project.Launch{GameExecutable: &exe, Args: []string{"-launch"}}
	err := cli.LaunchGame(launch, "C:/map.w3x", func(command string, args []string) error {
		calls = append(calls, append([]string{command}, args...))
		return nil
	})
	if err != nil || len(calls) != 1 || !slices.Equal(calls[0], []string{exe, "-launch", "-loadfile", "C:/map.w3x"}) || len(launch.Args) != 1 {
		t.Errorf("calls = %q, %v", calls, err)
	}
}

func TestLaunchGameReportsAGameThatFailsToStart(t *testing.T) {
	exe := testkit.WriteFile(t, t.TempDir(), "Warcraft III.exe", []byte("not a program"))
	e := asError(t, cli.LaunchGame(project.Launch{GameExecutable: &exe}, "map", cli.SpawnDetached), "not a program")
	if !strings.HasPrefix(e.Msg, "Cannot run '"+exe+"': ") || e.File != "moonwell.local.pkl" || !strings.Contains(e.Hint, "moonwell.local.pkl") {
		t.Errorf("error = %+v", e)
	}
}

func TestSpawnDetachedKeepsTheChildRunningAfterTheProgramExits(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "marker.txt")
	// The test program is the parent, which spawns itself as the child and exits (TestMain).
	parent := exec.Command(os.Args[0])
	parent.Env = append(os.Environ(), "MOONWELL_TEST_ROLE=parent", "MOONWELL_TEST_MARKER="+marker)
	if output, err := parent.CombinedOutput(); err != nil {
		t.Fatalf("the parent failed: %v\n%s", err, output)
	}
	if fsx.Exists(marker) {
		t.Fatal("the child finished before its parent exited, so the test shows nothing")
	}
	for deadline := time.Now().Add(10 * time.Second); !fsx.Exists(marker); time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the detached child died with its parent")
		}
	}
}

func TestIsRelevantChangeWatchesSourcesModulesAssetsObjectFilesAndManifestsOnly(t *testing.T) {
	root := filepath.Join(t.TempDir(), "proj")
	for path, want := range map[string]bool{
		"src/main.yue":                     true,
		"src/game/units.yue":               true,
		"src/notes.txt":                    false,
		"src/generated/objects.yue":        false,
		"moonwell.pkl":                     true,
		"moonwell.local.pkl":               true,
		"PklProject":                       true,
		"PklProject.deps.json":             true,
		"assets/icons/a.blp":               true,
		"objects/units.pkl":                true,
		"objects/human/barracks/units.pkl": true,
		"objects/notes.txt":                false,
		"lua/tools/init.lua":               true,
		"lua/notes.txt":                    false,
		"dist/stage/lua/main.lua":          false,
		"README.md":                        false,
	} {
		if got := cli.IsRelevantChange(root, filepath.Join(root, filepath.FromSlash(path))); got != want {
			t.Errorf("IsRelevantChange(%s) = %v", path, got)
		}
	}
}

func libraries(pairs ...string) *project.Project {
	p := &project.Project{}
	for i := 0; i < len(pairs); i += 3 {
		entry := project.Library{Dir: pairs[i+2]}
		if path := pairs[i+1]; path != "" {
			entry.Path = &path
		} else {
			repository, tag := "o/r", "v1"
			entry.GitHub, entry.Tag = &repository, &tag
		}
		p.Libraries.Set(pairs[i], entry)
	}
	return p
}

func TestLocalLibraryFoldersListsTheFoldersOfLocalLibrariesOnly(t *testing.T) {
	root := t.TempDir()
	folders := cli.LocalLibraryFolders(root, libraries("mine", "../mine", "src", "remote", "", ""))
	if want := []string{filepath.Join(filepath.Dir(root), "mine", "src")}; !slices.Equal(folders, want) {
		t.Errorf("LocalLibraryFolders = %q", folders)
	}
}

func TestLocalLibraryFoldersTakesTheModuleAndAssetsFoldersFromALibrarysOwnFile(t *testing.T) {
	root := t.TempDir()
	for name, file := range map[string]string{"described": `{"dir":"src","assets":"art"}`, "broken": "{", "rooted": "{}"} {
		testkit.WriteFile(t, root, name+"/moonwell-library.json", []byte(file))
	}
	folders := cli.LocalLibraryFolders(root, libraries("a", "described", "", "b", "described", "lua", "c", "broken", "", "d", "rooted", ""))
	want := []string{
		filepath.Join(root, "described", "src"),
		filepath.Join(root, "described", "art"),
		filepath.Join(root, "described", "lua"),
		filepath.Join(root, "described", "art"),
		filepath.Join(root, "broken"),
		filepath.Join(root, "rooted"),
	}
	if !slices.Equal(folders, want) {
		t.Errorf("LocalLibraryFolders = %q", folders)
	}
}

func TestIsLibraryChangeIgnoresChangesUnderALocalLibrarysDotFolders(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "lib", "src")
	for path, want := range map[string]bool{"example/greet.lua": true, "example": true, ".git/index": false, "example/.cache/x.lua": false} {
		if got := cli.IsLibraryChange(folder, filepath.Join(folder, filepath.FromSlash(path))); got != want {
			t.Errorf("IsLibraryChange(%s) = %v", path, got)
		}
	}
}

func TestDevFailsBeforeAnyWorkWhenSrcIsMissing(t *testing.T) {
	root := t.TempDir()
	env, _ := newEnv(root)
	env.Run = func(_ context.Context, command string, _ []string, _ proc.Options) (proc.Result, error) {
		t.Errorf("unexpected run: %s", command)
		return proc.Result{}, errors.New("unexpected run")
	}
	e := asError(t, cli.Dev(background, env, cli.DevOptions{}), "no src")
	if e.Msg != "The src/ folder is missing." || e.File != root ||
		e.Hint != "Run dev from a Moonwell project folder, or create one with `moonwell init <dir>`." {
		t.Errorf("error = %+v", e)
	}
}

func TestDevWatchesAssetsObjectsAndLuaWhenTheyExist(t *testing.T) {
	for message, folders := range map[string][]string{
		"Watching src/ and the project manifests. Press Ctrl+C to stop.":                    {},
		"Watching src/, objects/ and the project manifests. Press Ctrl+C to stop.":          {"objects"},
		"Watching src/, assets/, objects/ and the project manifests. Press Ctrl+C to stop.": {"assets", "objects"},
		"Watching src/, lua/ and the project manifests. Press Ctrl+C to stop.":              {"lua"},
	} {
		root := t.TempDir()
		for _, folder := range append([]string{"src"}, folders...) {
			if err := os.Mkdir(filepath.Join(root, folder), 0o777); err != nil {
				t.Fatal(err)
			}
		}
		env, log := newEnv(root)
		env.Run = func(context.Context, string, []string, proc.Options) (proc.Result, error) {
			return proc.Result{}, &diag.Error{Msg: "no Pkl in this test"}
		}
		// The first check fails and is reported; a cancelled context then stops the watching at once.
		stopped, stop := context.WithCancel(background)
		stop()
		if err := cli.Dev(stopped, env, cli.DevOptions{}); err != nil || !slices.Equal(log.Lines, []string{"error: no Pkl in this test", message}) {
			t.Errorf("Dev = %v; log %q", err, log.Lines)
		}
	}
}

// initEnv is an environment whose pkl is a stand-in that reports a version and resolves with a given exit code.
func initEnv(t *testing.T, root, pklVersion string, resolveCode int) *pipeline.Env {
	t.Helper()
	env, _ := newEnv(root)
	env.Run = func(_ context.Context, command string, args []string, _ proc.Options) (proc.Result, error) {
		switch line := strings.Join(append([]string{command}, args...), " "); line {
		case "pkl --version":
			return proc.Result{Stdout: pklVersion}, nil
		case "pkl project resolve":
			return proc.Result{Code: resolveCode, Stderr: "cannot reach the package server"}, nil
		default:
			t.Errorf("unexpected command: %s", line)
			return proc.Result{}, errors.New("unexpected command")
		}
	}
	return env
}

func linked(t *testing.T) cli.InitOptions {
	return cli.InitOptions{Link: true, Checkout: testkit.RepoRoot(t)}
}

func TestInitChecksThePklVersionBeforeWritingAnything(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "my-map")
	_, err := cli.Init(background, initEnv(t, parent, "Pkl 0.31.0", 0), target, linked(t))
	if e := asError(t, err, "an old Pkl"); !strings.Contains(e.Msg, "0.32") || fsx.Exists(target) {
		t.Errorf("error = %+v", e)
	}
}

func TestInitRemovesTheDirectoryItCreatedWhenResolvingFails(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "my-map")
	_, err := cli.Init(background, initEnv(t, parent, "Pkl 0.32.1", 1), target, linked(t))
	e := asError(t, err, "a failed resolve")
	if e.Msg != "pkl project resolve failed:\ncannot reach the package server" || e.File != filepath.Join(target, "PklProject") || fsx.Exists(target) {
		t.Errorf("error = %+v", e)
	}
}

func TestInitEmptiesAPreExistingDirectoryAgainWhenItFails(t *testing.T) {
	target := t.TempDir()
	_, err := cli.Init(background, initEnv(t, target, "Pkl 0.32.1", 1), target, linked(t))
	asError(t, err, "a failed resolve")
	if entries, readErr := os.ReadDir(target); readErr != nil || len(entries) != 0 {
		t.Errorf("the directory holds %v (%v)", entries, readErr)
	}
}

func TestInitRefusesATargetThatIsAFileOrIsNotEmpty(t *testing.T) {
	parent := t.TempDir()
	target := testkit.WriteFile(t, parent, "my-map", nil)
	_, err := cli.Init(background, initEnv(t, parent, "Pkl 0.32.1", 0), target, linked(t))
	if e := asError(t, err, "a file"); e.Msg != target+" is not a directory." || e.Hint != "Choose a new or empty directory." {
		t.Errorf("error = %+v", e)
	}
	_, err = cli.Init(background, initEnv(t, parent, "Pkl 0.32.1", 0), ".", linked(t))
	if e := asError(t, err, "a folder with a file"); e.Msg != ". is not empty." || e.Hint != "Choose a new or empty directory." {
		t.Errorf("error = %+v", e)
	}
}

func TestInitWritesTheTemplateWithoutADenoJsonAndEndsWithTheNextCommand(t *testing.T) {
	parent := t.TempDir()
	env := initEnv(t, parent, "Pkl 0.32.1", 0)
	log := testkit.NewRecorder()
	env.Log = log.Logger
	target, err := cli.Init(background, env, "my-map", cli.InitOptions{})
	if err != nil || target != filepath.Join(parent, "my-map") {
		t.Fatalf("Init = %q, %v", target, err)
	}
	for _, file := range []string{"moonwell.pkl", "moonwell.local.pkl", "src/main.yue", "maps/map.w3x/war3map.lua", ".gitignore", ".luarc.json"} {
		if !exists(target, file) {
			t.Errorf("%s is missing", file)
		}
	}
	if exists(target, "deno.json") || exists(target, "PklProject.deps.json") {
		t.Error("init wrote a deno.json, or the template's resolved dependencies")
	}
	if got := read(t, target, "PklProject"); got != project.PklProject("0.7.0", "") && !strings.Contains(got, "moonwell/moonwell@") {
		t.Errorf("PklProject =\n%s", got)
	}
	if got := read(t, target, "moonwell.local.pkl"); got != project.LocalPkl() {
		t.Errorf("moonwell.local.pkl =\n%s", got)
	}
	want := "Created my-map. Check launch.gameExecutable in moonwell.local.pkl, then: cd my-map && moonwell build"
	if !slices.Equal(log.Lines, []string{want}) {
		t.Errorf("log = %q", log.Lines)
	}
}

func TestInitWithLinkNeedsACheckout(t *testing.T) {
	parent := t.TempDir()
	_, err := cli.Init(background, initEnv(t, parent, "Pkl 0.32.1", 0), "my-map", cli.InitOptions{Link: true})
	if e := asError(t, err, "no checkout"); e.Msg != "--link only works when Moonwell runs from a local checkout." || exists(parent, "my-map") {
		t.Errorf("error = %+v", e)
	}
	// Below a checkout, the checkout is found: a folder whose go.mod names this module.
	checkout := t.TempDir()
	testkit.WriteFile(t, checkout, "go.mod", []byte("module github.com/mdlsvensson/moonwell\n\ngo 1.27\n"))
	below := filepath.Join(checkout, "internal", "cli")
	if err := os.MkdirAll(below, 0o777); err != nil {
		t.Fatal(err)
	}
	target, err := cli.Init(background, initEnv(t, below, "Pkl 0.32.1", 0), "../../maps/my-map", cli.InitOptions{Link: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := read(t, target, "PklProject"), project.PklProject("", "../../schema"); got != want {
		t.Errorf("PklProject =\n%s", got)
	}
}

func TestLinkPathIsRelativeWhenTheTargetSharesARootWithTheCheckout(t *testing.T) {
	base := t.TempDir()
	path, err := cli.LinkPath(filepath.Join(base, "maps", "my-map"), filepath.Join(base, "moonwell", "schema"))
	if err != nil || path != "../../moonwell/schema" {
		t.Errorf("LinkPath = %q, %v", path, err)
	}
}

func TestLinkPathRefusesATargetOnAnotherDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows has drives")
	}
	// Pkl cannot load a local dependency from another drive: PklProject.deps.json has no way to express the path.
	_, err := cli.LinkPath(`C:\Temp\my-map`, `D:\a\moonwell\schema`)
	e := asError(t, err, "another drive")
	if e.Msg != "--link needs the project on the same drive as this Moonwell checkout." || !strings.HasPrefix(e.Hint, `Create the project on D:\ (`) {
		t.Errorf("error = %+v", e)
	}
}

func TestOutsideAProjectAssetsPathsTellsInGamePathsFromCustomOnesShownWithBackslashes(t *testing.T) {
	root := t.TempDir()
	env, log := newEnv(root)
	testkit.WriteFile(t, root, "knight.mdx", testkit.MDX(
		testkit.Chunk("TEXS", testkit.Concat(testkit.Texture("Textures/Knight.blp", 0), testkit.Texture("", 1))),
		testkit.Chunk("PREM", testkit.Emitter(`Abilities\Heal.mdx`, 0)),
	))
	reports, err := cli.AssetsPaths(background, env, "knight.mdx", models.ParseGamePaths("# test\ntextures/knight.dds\n"))
	if err != nil || len(reports) != 1 || reports[0].Heading != "knight.mdx" {
		t.Fatalf("AssetsPaths = %+v, %v", reports, err)
	}
	var statuses []cli.PathStatus
	for _, ref := range reports[0].Refs {
		statuses = append(statuses, ref.Status)
	}
	if !slices.Equal(statuses, []cli.PathStatus{cli.InGame, "", cli.Custom}) {
		t.Errorf("statuses = %q", statuses)
	}
	want := []string{
		"knight.mdx",
		`  texture         Textures\Knight.blp   in-game path`,
		`  texture         team colour (slot 1)`,
		`  particle model  Abilities\Heal.mdx    custom path`,
		"1 model, 3 paths: 1 in-game, 1 custom.",
	}
	if !slices.Equal(log.Lines, want) {
		t.Errorf("log =\n%s", strings.Join(log.Lines, "\n"))
	}
}

func TestAReforgedTifReferenceMatchesTheGamesDds(t *testing.T) {
	root := t.TempDir()
	env, _ := newEnv(root)
	testkit.WriteFile(t, root, "grass.mdx", testkit.MDX(testkit.Chunk("TEXS", testkit.Texture("Doodads/Corn/plant1_Normal.tif", 0))))
	reports, err := cli.AssetsPaths(background, env, "grass.mdx", models.ParseGamePaths("doodads/corn/plant1_normal.dds\n"))
	if err != nil || reports[0].Refs[0].Status != cli.InGame {
		t.Errorf("AssetsPaths = %+v, %v", reports, err)
	}
}

func TestAnEmptyInGamePathListIsAnnounced(t *testing.T) {
	root := t.TempDir()
	env, log := newEnv(root)
	testkit.WriteFile(t, root, "a.mdx", testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\A.blp`, 0))))
	if _, err := cli.AssetsPaths(background, env, "a.mdx", map[string]bool{}); err != nil {
		t.Fatal(err)
	}
	if log.Lines[0] != "warning: Moonwell's in-game path list is empty, so every path shows as custom." {
		t.Errorf("log = %q", log.Lines)
	}
}

func TestOutsideAProjectAssetsPathsNeedsAFileThatExistsAndIsNotAFolder(t *testing.T) {
	root := t.TempDir()
	env, _ := newEnv(root)
	_, err := cli.AssetsPaths(background, env, "", nil)
	if e := asError(t, err, "no file"); e.Msg != "assets:paths needs a model file outside a Moonwell project." ||
		e.Hint != "moonwell assets:paths assets/Models/Knight.mdx" {
		t.Errorf("error = %+v", e)
	}
	_, err = cli.AssetsPaths(background, env, "missing.mdx", nil)
	if e := asError(t, err, "a missing file"); e.Msg != "missing.mdx does not exist." ||
		e.Hint != "Model paths are relative to the project folder, e.g. assets/Models/Knight.mdx." {
		t.Errorf("error = %+v", e)
	}
	_, err = cli.AssetsPaths(background, env, ".", nil)
	if e := asError(t, err, "a folder"); e.Msg != ". is a folder, not a model file." {
		t.Errorf("error = %+v", e)
	}
}

func TestEvaluatedListsEveryCategoryAndEachFieldInItsOrder(t *testing.T) {
	result := cli.Evaluated(nil)
	if got := ordered.Stringify(result, 0); got != `{"heroes":{},"units":{},"buildings":{},"items":{},"abilities":{},"buffs":{},"upgrades":{}}` {
		t.Errorf("Evaluated(nil) = %s", got)
	}
	if reflect.TypeOf(result) != reflect.TypeOf(&ordered.Map[any]{}) {
		t.Errorf("Evaluated returns %T", result)
	}
}
