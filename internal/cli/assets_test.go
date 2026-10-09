package cli

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/imp"
)

func checked(assets, changes string) string {
	return "Checked " + assets + " asset(s); assets:sync would make " + changes + " file change(s). Nothing was written."
}

func synced(assets, changes string) string {
	return "Synced " + assets + " asset(s) into maps/map.w3x (" + changes + " file change(s)). Reopen the map in " +
		"World Editor."
}

func importsOf(t *testing.T, root string) []string {
	t.Helper()
	entries, err := imp.Read([]byte(read(t, root, "maps/map.w3x/war3map.imp")), "war3map.imp")
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, entry := range entries {
		paths = append(paths, entry.Path)
	}
	return paths
}

func TestPklAssetsCheckAndSync(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "assets/icons/a.blp", "icon")
	planned := []string{
		`icons/a.blp -> icons\a.blp`, "write maps/map.w3x/icons/a.blp", "write maps/map.w3x/war3map.imp",
	}
	e, log, ran := pklOnly(t, root)
	if lines := logged(t, e, log, "assets:check"); !slices.Equal(lines, append(slices.Clone(planned), checked("1", "2"))) {
		t.Fatalf("assets:check logged %q", lines)
	}
	for _, written := range []string{"maps/map.w3x/icons/a.blp", "maps/map.w3x/war3map.imp", ".asset-state"} {
		if exists(root, written) {
			t.Fatalf("assets:check wrote %s", written)
		}
	}

	e, log, _ = pklOnly(t, root)
	if lines := logged(t, e, log, "assets:sync"); !slices.Equal(lines, append(slices.Clone(planned), synced("1", "2"))) {
		t.Fatalf("assets:sync logged %q", lines)
	}
	if read(t, root, "maps/map.w3x/icons/a.blp") != "icon" || !slices.Equal(importsOf(t, root), []string{`icons\a.blp`}) {
		t.Fatalf("the map holds another file than the asset, or its index lists %q", importsOf(t, root))
	}
	contains(t, read(t, root, ".asset-state/map.w3x.json"), "icons/a.blp")

	e, log, _ = pklOnly(t, root)
	if lines := logged(t, e, log, "assets:check"); !slices.Equal(lines, []string{planned[0], checked("1", "0")}) {
		t.Fatalf("assets:check after a sync logged %q", lines)
	}
	onlyPkl(t, ran)
	if exists(root, "dist/.lock") {
		t.Error("a command left the build lock behind")
	}
}

func TestPklAssetsAreListedByThePathsTheyAreWrittenUnder(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "assets/Icons/a.blp", "a")
	write(t, root, "assets/b.blp", "b")
	edit(t, root, "moonwell.pkl", "paths {}", `paths { ["b.blp"] = #"icons\b.blp"# }`)
	e, log, _ := pklOnly(t, root)
	lines := logged(t, e, log, "assets:check")
	want := []string{
		`Icons/a.blp -> Icons\a.blp`, `b.blp -> Icons\b.blp`, "write maps/map.w3x/Icons/a.blp",
		"write maps/map.w3x/Icons/b.blp", "write maps/map.w3x/war3map.imp", checked("2", "3"),
	}
	if !slices.Equal(lines, want) {
		t.Fatalf("assets:check logged:\n%s", strings.Join(lines, "\n"))
	}
}

func stoppingAfterTheManifest(t *testing.T, stop context.CancelFunc) envFactory {
	return func(root string, log *env.Logger) *env.Env {
		e, _, _ := pklOnly(t, root)
		e.Log = log
		runs := e.Run
		e.Run = func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
			result, err := runs(ctx, program, args, options)
			if slices.Contains(args, "eval") {
				stop()
			}
			return result, err
		}
		return e
	}
}

func TestPklAssetsInterruptedSyncWritesNothing(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "assets/icons/a.blp", "icon")
	before := testkit.Snapshot(t, filepath.Join(root, "maps"))

	stopped, stop := context.WithCancel(background)
	stop()
	if result := carriedIn(stopped, stoppingAfterTheManifest(t, stop), root, "assets:sync"); result.code != 130 ||
		result.output != "" {
		t.Errorf("a sync that was told to stop before it started: %+v", result)
	}

	planning, stop := context.WithCancel(background)
	defer stop()
	result := carriedIn(planning, stoppingAfterTheManifest(t, stop), root, "assets:sync")
	if result.code != 130 || result.output != "error: Interrupted; nothing was written." {
		t.Errorf("a sync that was told to stop while it planned: %+v", result)
	}

	sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), "interrupted sync")
	if exists(root, ".asset-state") || exists(root, "dist/.lock") {
		t.Fatal("an interrupted sync wrote the ownership state, or left the build lock behind")
	}
}

func projectWithAssetLibrary(t *testing.T) (root, plain string) {
	t.Helper()
	root = newProject(t, "my-map")
	library := filepath.Join(filepath.Dir(root), "golems")
	write(t, library, "moonwell-library.json", `{"dir":"src","assets":"assets"}`)
	write(t, library, "src/golems/spawn.lua", "return {}\n")
	testkit.WriteFile(t, library, "assets/Models/Golem.mdx",
		testkit.MDX(testkit.Chunk("TEXS", testkit.Texture(`Textures\Golem.blp`, 0))))
	testkit.WriteFile(t, library, "assets/Textures/Golem.blp", []byte{1})
	write(t, library, "assets/war3mapImported/golems/frames.toc", "war3mapImported\\golems\\frames.fdf\n")
	plain = read(t, root, "moonwell.pkl")
	edit(t, root, "moonwell.pkl", "libraries {\n",
		"libraries {\n  [\"golems\"] { path = \""+filepath.ToSlash(library)+"\" }\n")
	return root, plain
}

func TestPklLibraryAssetsSyncAndRemoval(t *testing.T) {
	root, plain := projectWithAssetLibrary(t)
	testkit.WriteFile(t, root, "assets/Textures/golem.blp", []byte{2})
	planned := []string{
		`library golems: Models/Golem.mdx -> Models\Golem.mdx`,
		`Textures/golem.blp -> Textures\golem.blp`,
		`library golems: war3mapImported/golems/frames.toc -> war3mapImported\golems\frames.toc`,
		"assets/Textures/golem.blp replaces library golems's Textures/Golem.blp",
		"write maps/map.w3x/Models/Golem.mdx",
		"write maps/map.w3x/Textures/golem.blp",
		"write maps/map.w3x/war3mapImported/golems/frames.toc",
		"write maps/map.w3x/war3map.imp",
	}
	e, log, ran := pklOnly(t, root)
	if lines := logged(t, e, log, "assets:check"); !slices.Equal(lines, append(slices.Clone(planned), checked("3", "4"))) {
		t.Fatalf("assets:check logged:\n%s", strings.Join(lines, "\n"))
	}
	if exists(root, "maps/map.w3x/Models") || !exists(root, ".moonwell/library-assets/golems/Models/Golem.mdx") {
		t.Fatal("assets:check wrote into the map, or did not sync the library")
	}

	e, log, _ = pklOnly(t, root)
	if lines := logged(t, e, log, "assets:sync"); !slices.Equal(lines, append(slices.Clone(planned), synced("3", "4"))) {
		t.Fatalf("assets:sync logged:\n%s", strings.Join(lines, "\n"))
	}
	if read(t, root, "maps/map.w3x/Textures/golem.blp") != string([]byte{2}) ||
		read(t, root, "maps/map.w3x/war3mapImported/golems/frames.toc") != "war3mapImported\\golems\\frames.fdf\n" {
		t.Fatal("the map holds other bytes than the assets")
	}

	write(t, root, "moonwell.pkl", plain)
	e, log, _ = pklOnly(t, root)
	lines := logged(t, e, log, "assets:sync")
	if !slices.Equal(lines, []string{
		`Textures/golem.blp -> Textures\golem.blp`,
		"delete maps/map.w3x/Models/Golem.mdx",
		"delete maps/map.w3x/war3mapImported/golems/frames.toc",
		"write maps/map.w3x/war3map.imp",
		synced("1", "3"),
	}) {
		t.Fatalf("assets:sync without the library logged:\n%s", strings.Join(lines, "\n"))
	}
	for _, name := range []string{
		"maps/map.w3x/Models/Golem.mdx", "maps/map.w3x/war3mapImported/golems/frames.toc",
		".moonwell/library-assets/golems",
	} {
		if exists(root, name) {
			t.Errorf("%s is there still", name)
		}
	}
	if imports := importsOf(t, root); !slices.Equal(imports, []string{`Textures\golem.blp`}) {
		t.Fatalf("the index lists %q", imports)
	}
	onlyPkl(t, ran)
}

func TestPklAssetsCommandsNeedTheMapsScriptAndItsInfoFile(t *testing.T) {
	for _, name := range []string{"war3map.lua", "war3map.w3i"} {
		for what, arrange := range map[string]func(t *testing.T, root string){
			"is not there": func(t *testing.T, root string) {},
			"is a folder":  func(t *testing.T, root string) { write(t, root, "maps/map.w3x/"+name+"/kept.txt", "") },
		} {
			root, _ := projectWithAssetLibrary(t)
			write(t, root, "assets/icons/a.blp", "icon")
			remove(t, root, "maps/map.w3x/"+name)
			arrange(t, root)
			before := testkit.Snapshot(t, filepath.Join(root, "maps"))
			for _, command := range []string{"assets:check", "assets:sync"} {
				e, log, _ := pklOnly(t, root)
				_, err := commandIn(t, background, e, command)
				diagErr := asError(t, err, command+" in a map whose "+name+" "+what)
				if !strings.Contains(diagErr.Msg, "source map has no "+name) || diagErr.File != "maps/map.w3x" ||
					!strings.Contains(diagErr.Hint, "Lua as the script language") {
					t.Errorf("%s, %s %s: error = %+v", command, name, what, diagErr)
				}
				if lines := log.Lines(); len(lines) != 0 {
					t.Errorf("%s, %s %s: logged %q", command, name, what, lines)
				}
			}
			sameFiles(t, before, testkit.Snapshot(t, filepath.Join(root, "maps")), name+" "+what)
			if exists(root, ".moonwell") || exists(root, ".asset-state") || exists(root, "dist/.lock") {
				t.Errorf("%s %s: a refused command synced the libraries, wrote the state, or kept the lock", name, what)
			}
		}
	}
}

func TestPklAssetsCommandsNeedTheSourceMap(t *testing.T) {
	root := newProject(t, "my-map")
	remove(t, root, "maps/map.w3x")
	for _, command := range []string{"assets:check", "assets:sync"} {
		e, _, _ := pklOnly(t, root)
		_, err := commandIn(t, background, e, command)
		diagErr := asError(t, err, command+" without the map")
		if diagErr.File != "moonwell.local.pkl" || diagErr.Hint == "" ||
			!strings.Contains(diagErr.Msg, "Source map folder maps/map.w3x not found") {
			t.Errorf("%s: error = %+v", command, diagErr)
		}
	}
	if exists(root, "maps/map.w3x") || exists(root, ".asset-state") {
		t.Error("a command made the map folder, or wrote the ownership state")
	}
}

func TestPklAssetsCommandsAreRefusedBesideARunningBuild(t *testing.T) {
	root, _ := projectWithAssetLibrary(t)
	holdBuildLock(t, root)
	for _, command := range []string{"assets:check", "assets:sync"} {
		e, log, _ := pklOnly(t, root)
		_, err := commandIn(t, background, e, command)
		diagErr := asError(t, err, command+" beside a build")
		if diagErr.File != "dist/.lock" || diagErr.Hint == "" ||
			!strings.Contains(diagErr.Msg, "Another Moonwell build is running") {
			t.Errorf("%s: error = %+v", command, diagErr)
		}
		if lines := log.Lines(); len(lines) != 0 {
			t.Errorf("%s logged %q", command, lines)
		}
	}
	if exists(root, ".moonwell") || exists(root, "maps/map.w3x/Models") || !exists(root, "dist/.lock") {
		t.Error("a refused command synced the libraries, wrote into the map, or removed the lock of the build")
	}
}

func TestPklTheAssetsCommandsHoldTheBuildLockWhileTheySyncTheLibraries(t *testing.T) {
	for _, name := range []string{"assets:check", "assets:sync", "assets:paths"} {
		root := newProject(t, "my-map")
		edit(t, root, "moonwell.pkl", "libraries {\n",
			"libraries {\n  [\"kit\"] { github = \"owner/kit\"; tag = \"v1.0.0\" }\n")
		e, _, _ := pklOnly(t, root)
		var asked, locked atomic.Int32
		e.Fetch = func(_ context.Context, url string) (int, []byte, error) {
			asked.Add(1)
			if exists(root, "dist/.lock") {
				locked.Add(1)
			}
			return 0, nil, &diag.Error{Msg: "tried to download " + url}
		}
		_, err := commandIn(t, background, e, name)
		if err == nil || !strings.Contains(diag.Format(err), "owner/kit") {
			t.Errorf("%s: the command ended with %v, want the failure of the library's download", name, err)
		}
		if asked.Load() == 0 || locked.Load() != asked.Load() {
			t.Errorf("%s: the lock was held at %d of the %d downloads the sync asked for, want it at each, and "+
				"one at least", name, locked.Load(), asked.Load())
		}
		if exists(root, "dist/.lock") {
			t.Errorf("%s left the build lock behind", name)
		}
	}
}

func TestPklAssetsCommandLinesPrintAndKeepWhatTheySay(t *testing.T) {
	root := newProject(t, "my-map")
	write(t, root, "assets/icons/a.blp", "icon")
	r := okWithPklAlone(t, root, "assets:check")
	if r.stdout != "" || r.output != "icons/a.blp -> icons\\a.blp\nwrite maps/map.w3x/icons/a.blp\n"+
		"write maps/map.w3x/war3map.imp\n"+checked("1", "2") {
		t.Fatalf("assets:check: %+v", r)
	}
	r = okWithPklAlone(t, root, "assets:sync")
	if r.stdout != "" || !strings.HasSuffix(r.output, "\n"+synced("1", "2")) {
		t.Fatalf("assets:sync: %+v", r)
	}
	contains(t, read(t, root, "dist/moonwell.log"), "] info: "+checked("1", "2")+"\n", "] info: "+synced("1", "2")+"\n")
}
