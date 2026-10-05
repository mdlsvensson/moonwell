package cli

import (
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
)

// The tests of this file run setup in a project that init made. Each runs the real pkl, and takes the time that
// takes; none runs the compiler, which setup asks for and never starts. They run in a seeded world, so that what
// setup keeps for the editor goes into a cache of the test's own. The carried cases of setup are in e2e_test.go.

// createdLocal is the line setup logs for a moonwell.local.pkl it made.
const createdLocal = "Created moonwell.local.pkl. Check that launch.gameExecutable points at your Warcraft III.exe."

// saysWhichCompiler is the line setup logs for the compiler it found.
func saysWhichCompiler(compiler string) string {
	return "YueScript " + toolchain.YueVersion + ": " + compiler
}

// warnsOfPath reports whether a line is the warning for a PATH without `yue`, with the command that puts binDir
// there.
func warnsOfPath(line, binDir string) bool {
	return strings.HasPrefix(line, "warning: yue is not on PATH; VS Code's YueScript extension needs YueScript "+
		toolchain.YueVersion+" there. Run this once in ") &&
		strings.HasSuffix(line, ":\n  "+toolchain.PathCommand(binDir, runtime.GOOS))
}

// ---- the order of the steps, and what each says ----

// A whole setup says its steps in their order. The compiler is copied for the editor once: a second setup finds
// the copy, and says nothing of it.
func TestSetupSaysItsStepsInTheirOrderAndCopiesTheCompilerForTheEditorOnce(t *testing.T) {
	world, root := seeded(t), newProject(t, "my-map")
	remove(t, root, "moonwell.local.pkl")
	remove(t, root, "yueconfig.yue")
	e, log := world.at(root)
	if err := runSetup(background, e, call{}); err != nil {
		t.Fatal(diag.Format(err))
	}
	copied := filepath.Join(world.binDir(), filepath.Base(world.compiler))
	lines := log.Lines()
	if len(lines) != 5 || lines[0] != createdLocal || lines[1] != saysWhichCompiler(world.compiler) ||
		lines[2] != "Copied YueScript for the editor to "+copied+"." || !warnsOfPath(lines[3], world.binDir()) ||
		lines[4] != "Added yueconfig.yue for the editor." {
		t.Errorf("the first setup logged %q", lines)
	}
	if !fsx.Exists(copied) || !exists(root, ".moonwell/types/natives.d.lua") || exists(root, "dist/.lock") {
		t.Error("setup kept no copy of the compiler, wrote no declarations, or left the build lock behind")
	}

	e, log = world.at(root)
	if err := runSetup(background, e, call{}); err != nil {
		t.Fatal(diag.Format(err))
	}
	lines = log.Lines()
	if len(lines) != 2 || lines[0] != saysWhichCompiler(world.compiler) || !warnsOfPath(lines[1], world.binDir()) {
		t.Errorf("the second setup logged %q", lines)
	}
}

// A yue.path is the user's own program: it is asked for its version, not copied, and the folder to put on PATH
// is its own, with the separators the manifest writes.
func TestSetupWithAYuePathCopiesNothingAndNamesItsFolderAsTheManifestWritesIt(t *testing.T) {
	tools := filepath.ToSlash(t.TempDir())
	written := tools + "/kept/yue-of-mine"
	testkit.WriteFile(t, tools, "kept/yue-of-mine", nil)
	world, root := seeded(t, written), newProject(t, "my-map")
	appendTo(t, root, "moonwell.local.pkl", "\nyue { path = \""+written+"\" }\n")
	e, log := world.at(root)
	if err := runSetup(background, e, call{}); err != nil {
		t.Fatal(diag.Format(err))
	}
	lines := log.Lines()
	if len(lines) != 2 || lines[0] != saysWhichCompiler(written) || !warnsOfPath(lines[1], tools+"/kept") {
		t.Errorf("setup logged %q", lines)
	}
	if fsx.Exists(world.binDir()) {
		t.Error("setup copied a compiler that is the user's own")
	}
}

// The world lets pkl alone run, and the manifest names a compiler that is not there: setup ends at the compiler,
// and what it has done by then is what comes before it.
func TestSetupMakesTheLocalManifestBeforeItLooksForTheCompilerAndTheEditorsFilesAfter(t *testing.T) {
	root := newProject(t, "my-map")
	gone := filepath.ToSlash(filepath.Join(t.TempDir(), "no-such-yue"))
	appendTo(t, root, "moonwell.pkl", "\nyue { path = \""+gone+"\" }\n")
	remove(t, root, "moonwell.local.pkl")
	remove(t, root, "yueconfig.yue")
	e, log, ran := pklOnly(t, root)
	failure := asError(t, runSetup(background, e, call{}), "a compiler that is not there")
	if !strings.Contains(failure.Msg, "yue.path does not exist") {
		t.Errorf("error = %+v", failure)
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
	world, root := seeded(t), newProject(t, "my-map")
	remove(t, root, "maps/map.w3x")
	remove(t, root, "yueconfig.yue")
	useLibrary(t, root, exampleLibrary(t))
	e, log := world.at(root)
	failure := asError(t, runSetup(background, e, call{}), "a project without its source map")
	if failure.File != "moonwell.local.pkl" || failure.Hint == "" ||
		!strings.Contains(failure.Msg, "Source map folder maps/map.w3x not found") {
		t.Errorf("error = %+v", failure)
	}
	// The tools and the editor's files are in place by then.
	logged := strings.Join(log.Lines(), "\n")
	contains(t, logged, saysWhichCompiler(world.compiler), "Added yueconfig.yue for the editor.")
	if !exists(root, "yueconfig.yue") || !fsx.Exists(world.binDir()) {
		t.Error("setup did not copy the compiler and add the editor's files before it opened the map")
	}
	// The declarations, and the libraries after them, are not.
	for _, path := range []string{".moonwell/types", ".moonwell/yue", ".moonwell/libraries", "dist/.lock"} {
		if exists(root, path) {
			t.Errorf("setup made %s though the map is not there", path)
		}
	}
}

// The build lock is taken for the libraries and no earlier: beside a running build, and in a project whose dist
// is a link, setup has installed the tools and written the editor's files and the declarations by the time it
// is refused.
func TestSetupIsRefusedAtTheLibrariesByAHeldBuildLockAndByALinkAtDist(t *testing.T) {
	for _, c := range []struct {
		what    string
		arrange func(t *testing.T, root string)
		msg     string
		file    string
	}{
		{"a running build", func(t *testing.T, root string) {
			release, err := build.Acquire(root)
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
			world, root := seeded(t), newProject(t, "my-map")
			remove(t, root, "yueconfig.yue")
			useLibrary(t, root, exampleLibrary(t))
			c.arrange(t, root)
			held := exists(root, "dist/.lock")
			e, log := world.at(root)
			failure := asError(t, runSetup(background, e, call{}), c.what)
			if failure.File != c.file || failure.Hint == "" || !strings.Contains(failure.Msg, c.msg) {
				t.Errorf("error = %+v", failure)
			}
			contains(t, strings.Join(log.Lines(), "\n"), saysWhichCompiler(world.compiler))
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

// ---- .luarc.json ----

// A setup that ends well gives the build lock back, and keeps what it said in the project's log.
func TestSetupSaysWhatItAddsToLuarcAndNamesTheEntriesOfOneItLeavesAlone(t *testing.T) {
	world, root := seeded(t), newProject(t, "my-map")
	write(t, root, ".luarc.json", "{\n  \"runtime.version\": \"Lua 5.3\",\n  \"runtime.path\": [\"src/?.lua\"]\n}\n")
	result := world.ok(t, root, "setup")
	added := "Added src/?/init.lua, lua/?.lua, lua/?/init.lua, .moonwell/types, .moonwell/lua, dist, maps, " +
		".moonwell/libraries to .luarc.json."
	contains(t, result.output, added)
	contains(t, read(t, root, "dist/moonwell.log"), "] info: "+added+"\n")
	if exists(root, "dist/.lock") {
		t.Error("setup left the build lock behind")
	}
	if again := world.ok(t, root, "setup"); strings.Contains(again.output, ".luarc.json") {
		t.Errorf("a second setup spoke of a .luarc.json that lacks nothing:\n%s", again.output)
	}

	// A comment makes the file one that only lua-language-server reads: it is left as it is.
	mine := "// Mine.\n" + read(t, root, ".luarc.json")
	write(t, root, ".luarc.json", mine)
	edit(t, root, ".luarc.json", `"dist"`, `"build"`)
	mine = read(t, root, ".luarc.json")
	result = world.ok(t, root, "setup")
	contains(t, result.output, "warning: .luarc.json is not plain JSON, so setup left it alone. Make sure its "+
		"runtime.path has src/?.lua, src/?/init.lua, lua/?.lua, lua/?/init.lua, its workspace.library has "+
		".moonwell/types, .moonwell/lua and its workspace.ignoreDir has dist, maps, .moonwell/libraries.")
	if read(t, root, ".luarc.json") != mine {
		t.Error("setup changed a .luarc.json that is not plain JSON")
	}
}
