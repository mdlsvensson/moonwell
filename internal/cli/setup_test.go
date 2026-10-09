package cli

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/build"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/toolchain"
)

const createdLocal = "Created moonwell.local.pkl. Check that launch.gameExecutable points at your Warcraft III.exe."

func compilerLine(compiler string) string {
	return "YueScript " + toolchain.YueVersion + ": " + compiler
}

func isPathWarning(line, binDir string) bool {
	return strings.HasPrefix(line, "warning: yue is not on PATH; VS Code's YueScript extension needs YueScript "+
		toolchain.YueVersion+" there. Run this once in ") &&
		strings.HasSuffix(line, ":\n  "+toolchain.AddToPathCommand(binDir, runtime.GOOS))
}

func TestSetupSaysItsStepsInTheirOrderAndCopiesTheCompilerForTheEditorOnce(t *testing.T) {
	world, root := newFakeWorld(t), newProject(t, "my-map")
	removeFile(t, root, "moonwell.local.pkl")
	removeFile(t, root, "yueconfig.yue")
	e, log := world.envAt(root)
	if err := runSetup(background, e, commandArgs{}); err != nil {
		t.Fatal(diag.Format(err))
	}
	copied := filepath.Join(world.binDir(), filepath.Base(world.compiler))
	lines := log.Lines()
	if len(lines) != 5 || lines[0] != createdLocal || lines[1] != compilerLine(world.compiler) ||
		lines[2] != "Copied YueScript for the editor to "+copied+"." || !isPathWarning(lines[3], world.binDir()) ||
		lines[4] != "Added yueconfig.yue for the editor." {
		t.Errorf("the first setup logged %q", lines)
	}
	if !fsx.Exists(copied) || !exists(root, ".moonwell/types/natives.d.lua") || exists(root, "dist/.lock") {
		t.Error("setup kept no copy of the compiler, wrote no declarations, or left the build lock behind")
	}

	e, log = world.envAt(root)
	if err := runSetup(background, e, commandArgs{}); err != nil {
		t.Fatal(diag.Format(err))
	}
	lines = log.Lines()
	if len(lines) != 2 || lines[0] != compilerLine(world.compiler) || !isPathWarning(lines[1], world.binDir()) {
		t.Errorf("the second setup logged %q", lines)
	}
}

func TestSetupWithAYuePathCopiesNothingAndNamesItsFolderAsTheManifestWritesIt(t *testing.T) {
	tools := filepath.ToSlash(t.TempDir())
	written := tools + "/kept/yue-of-mine"
	testkit.WriteFile(t, tools, "kept/yue-of-mine", nil)
	world, root := newFakeWorld(t, written), newProject(t, "my-map")
	appendToFile(t, root, "moonwell.local.pkl", "\nyue { path = \""+written+"\" }\n")
	e, log := world.envAt(root)
	if err := runSetup(background, e, commandArgs{}); err != nil {
		t.Fatal(diag.Format(err))
	}
	lines := log.Lines()
	if len(lines) != 2 || lines[0] != compilerLine(written) || !isPathWarning(lines[1], tools+"/kept") {
		t.Errorf("setup logged %q", lines)
	}
	if fsx.Exists(world.binDir()) {
		t.Error("setup copied a compiler that is the user's own")
	}
}

func TestSetupMakesTheLocalManifestBeforeItLooksForTheCompilerAndTheEditorsFilesAfter(t *testing.T) {
	root := newProject(t, "my-map")
	gone := filepath.ToSlash(filepath.Join(t.TempDir(), "no-such-yue"))
	appendToFile(t, root, "moonwell.pkl", "\nyue { path = \""+gone+"\" }\n")
	removeFile(t, root, "moonwell.local.pkl")
	removeFile(t, root, "yueconfig.yue")
	e, log, ran := newPklOnlyEnv(t, root)
	diagErr := asDiagError(t, runSetup(background, e, commandArgs{}), "a compiler that is not there")
	if !strings.Contains(diagErr.Msg, "yue.path does not exist") {
		t.Errorf("error = %+v", diagErr)
	}
	if lines := log.Lines(); !slices.Equal(lines, []string{createdLocal}) {
		t.Errorf("log = %q", lines)
	}
	if !exists(root, "moonwell.local.pkl") || exists(root, "yueconfig.yue") || exists(root, ".moonwell") {
		t.Error("setup made no moonwell.local.pkl before the compiler, or an editor's file ahead of it")
	}
	notPkl := func(program string) bool { return program != "pkl" }
	if programs := ran(); len(programs) == 0 || slices.ContainsFunc(programs, notPkl) {
		t.Errorf("setup ran %q before the compiler, want pkl alone", programs)
	}
}

func TestSetupWithoutItsSourceMapFailsAtTheDeclarations(t *testing.T) {
	world, root := newFakeWorld(t), newProject(t, "my-map")
	removeFile(t, root, "maps/map.w3x")
	removeFile(t, root, "yueconfig.yue")
	useLibrary(t, root, exampleLibrary(t))
	e, log := world.envAt(root)
	diagErr := asDiagError(t, runSetup(background, e, commandArgs{}), "a project without its source map")
	if diagErr.File != "moonwell.local.pkl" || diagErr.Hint == "" ||
		!strings.Contains(diagErr.Msg, "Source map folder maps/map.w3x not found") {
		t.Errorf("error = %+v", diagErr)
	}
	logged := strings.Join(log.Lines(), "\n")
	checkContains(t, logged, compilerLine(world.compiler), "Added yueconfig.yue for the editor.")
	if !exists(root, "yueconfig.yue") || !fsx.Exists(world.binDir()) {
		t.Error("setup did not copy the compiler and add the editor's files before it opened the map")
	}
	for _, path := range []string{".moonwell/types", ".moonwell/yue", ".moonwell/libraries", "dist/.lock"} {
		if exists(root, path) {
			t.Errorf("setup made %s though the map is not there", path)
		}
	}
}

func TestSetupIsRefusedAtTheLibrariesByAHeldBuildLockAndByALinkAtDist(t *testing.T) {
	for _, c := range []struct {
		what    string
		arrange func(t *testing.T, root string)
		msg     string
		file    string
	}{
		{"a running build", func(t *testing.T, root string) {
			release, err := build.AcquireLock(root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(release)
		}, "Another Moonwell build is running", "dist/.lock"},
		{"a link at dist", func(t *testing.T, root string) {
			testkit.LinkDir(t, t.TempDir(), filepath.Join(root, "dist"))
		}, "dist is a link", "dist"},
	} {
		t.Run(c.what, func(t *testing.T) {
			world, root := newFakeWorld(t), newProject(t, "my-map")
			removeFile(t, root, "yueconfig.yue")
			useLibrary(t, root, exampleLibrary(t))
			c.arrange(t, root)
			held := exists(root, "dist/.lock")
			e, log := world.envAt(root)
			diagErr := asDiagError(t, runSetup(background, e, commandArgs{}), c.what)
			if diagErr.File != c.file || diagErr.Hint == "" || !strings.Contains(diagErr.Msg, c.msg) {
				t.Errorf("error = %+v", diagErr)
			}
			checkContains(t, strings.Join(log.Lines(), "\n"), compilerLine(world.compiler))
			for _, path := range []string{
				"yueconfig.yue", ".moonwell/types/natives.d.lua", ".moonwell/yue/moonwell/macros.yue",
			} {
				if !exists(root, path) {
					t.Errorf("setup did not write %s before it was refused", path)
				}
			}
			if exists(root, ".moonwell/libraries") || exists(root, ".moonwell/lua") {
				t.Error("setup synced the libraries without the build lock")
			}
			if exists(root, "dist/.lock") != held {
				t.Error("setup removed the lock of the build beside it, or left one of its own")
			}
		})
	}
}

func TestSetupDeclaresTheObjectsAndLeavesTheIDsModuleToABuild(t *testing.T) {
	world, root := newFakeWorld(t), newProject(t, "my-map")
	removeFile(t, root, "src/generated/objects.yue")
	world.mustSucceed(t, root, "setup")
	checkContains(t, readFile(t, root, ".moonwell/types/objects.d.lua"), "captain")
	if exists(root, "src/generated/objects.yue") {
		t.Error("setup wrote the ids module")
	}
}

func TestSetupSaysWhatItAddsToLuarcAndNamesTheEntriesOfOneItLeavesAlone(t *testing.T) {
	world, root := newFakeWorld(t), newProject(t, "my-map")
	writeFile(t, root, ".luarc.json", "{\n  \"runtime.version\": \"Lua 5.3\",\n  \"runtime.path\": [\"src/?.lua\"]\n}\n")
	result := world.mustSucceed(t, root, "setup")
	added := "Added src/?/init.lua, lua/?.lua, lua/?/init.lua, .moonwell/types, .moonwell/lua, dist, maps, " +
		".moonwell/libraries to .luarc.json."
	checkContains(t, result.output, added)
	checkContains(t, readFile(t, root, "dist/moonwell.log"), "] info: "+added+"\n")
	if exists(root, "dist/.lock") {
		t.Error("setup left the build lock behind")
	}
	if again := world.mustSucceed(t, root, "setup"); strings.Contains(again.output, ".luarc.json") {
		t.Errorf("a second setup spoke of a .luarc.json that lacks nothing:\n%s", again.output)
	}

	mine := "// Mine.\n" + readFile(t, root, ".luarc.json")
	writeFile(t, root, ".luarc.json", mine)
	replaceInFile(t, root, ".luarc.json", `"dist"`, `"build"`)
	mine = readFile(t, root, ".luarc.json")
	result = world.mustSucceed(t, root, "setup")
	checkContains(t, result.output, "warning: .luarc.json is not plain JSON, so setup left it alone. Make sure its "+
		"runtime.path has src/?.lua, src/?/init.lua, lua/?.lua, lua/?/init.lua, its workspace.library has "+
		".moonwell/types, .moonwell/lua and its workspace.ignoreDir has dist, maps, .moonwell/libraries.")
	if readFile(t, root, ".luarc.json") != mine {
		t.Error("setup changed a .luarc.json that is not plain JSON")
	}
}
