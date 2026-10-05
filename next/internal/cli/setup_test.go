package cli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/build"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/toolchain"
)

// The tests of this file run setup in a project that init made. Each runs the real pkl, and all but the first
// the real YueScript compiler too, and takes the seconds those take. The carried cases of setup are in
// e2e_test.go.

// createdLocal is the line setup logs for a moonwell.local.pkl it made.
const createdLocal = "Created moonwell.local.pkl. Check that launch.gameExecutable points at your Warcraft III.exe."

// The world lets pkl alone run, and has no compiler in its cache: setup gets as far as the compiler, and what it
// has done by then is what comes before it.
func TestSetupMakesTheLocalManifestBeforeItLooksForTheCompilerAndTheEditorsFilesAfter(t *testing.T) {
	root := newProject(t, "my-map")
	remove(t, root, "moonwell.local.pkl")
	remove(t, root, "yueconfig.yue")
	e, log, ran := pklOnly(t, root)
	failure := asError(t, runSetup(background, e, call{}), "a world without a compiler")
	// The compiler is not in the cache of that world, and its download is refused.
	refused := asError(t, failure.Cause, "the refused download")
	if !strings.Contains(refused.Msg, "tried to download") {
		t.Errorf("error = %+v, caused by %+v", failure, refused)
	}
	lines := log.Lines()
	if len(lines) == 0 || lines[0] != createdLocal || strings.Contains(strings.Join(lines, "\n"), "Added ") {
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
	root := compiling(t)
	remove(t, root, "maps/map.w3x")
	remove(t, root, "yueconfig.yue")
	useLibrary(t, root, exampleLibrary(t))
	e, log := realWorld(root)
	failure := asError(t, runSetup(background, e, call{}), "a project without its source map")
	if failure.File != "moonwell.local.pkl" || failure.Hint == "" ||
		!strings.Contains(failure.Msg, "Source map folder maps/map.w3x not found") {
		t.Errorf("error = %+v", failure)
	}
	// The tools and the editor's files are in place by then.
	logged := strings.Join(log.Lines(), "\n")
	contains(t, logged, "YueScript "+toolchain.YueVersion+": ", "Added yueconfig.yue for the editor.")
	if !exists(root, "yueconfig.yue") {
		t.Error("setup did not add the editor's files before it opened the map")
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
			root := compiling(t)
			remove(t, root, "yueconfig.yue")
			useLibrary(t, root, exampleLibrary(t))
			c.arrange(t, root)
			held := exists(root, "dist/.lock")
			e, log := realWorld(root)
			failure := asError(t, runSetup(background, e, call{}), c.what)
			if failure.File != c.file || failure.Hint == "" || !strings.Contains(failure.Msg, c.msg) {
				t.Errorf("error = %+v", failure)
			}
			contains(t, strings.Join(log.Lines(), "\n"), "YueScript "+toolchain.YueVersion+": ")
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

// A setup that ends well gives the build lock back, and keeps what it said in the project's log.
func TestSetupSaysWhatItAddsToLuarcAndNamesTheEntriesOfOneItLeavesAlone(t *testing.T) {
	root := compiling(t)
	write(t, root, ".luarc.json", "{\n  \"runtime.version\": \"Lua 5.3\",\n  \"runtime.path\": [\"src/?.lua\"]\n}\n")
	result := ok(t, root, "setup")
	added := "Added src/?/init.lua, lua/?.lua, lua/?/init.lua, .moonwell/types, .moonwell/lua, dist, maps, " +
		".moonwell/libraries to .luarc.json."
	contains(t, result.output, added)
	contains(t, read(t, root, "dist/moonwell.log"), "] info: "+added+"\n")
	if exists(root, "dist/.lock") {
		t.Error("setup left the build lock behind")
	}
	if again := ok(t, root, "setup"); strings.Contains(again.output, ".luarc.json") {
		t.Errorf("a second setup spoke of a .luarc.json that lacks nothing:\n%s", again.output)
	}

	// A comment makes the file one that only lua-language-server reads: it is left as it is.
	mine := "// Mine.\n" + read(t, root, ".luarc.json")
	write(t, root, ".luarc.json", mine)
	edit(t, root, ".luarc.json", `"dist"`, `"build"`)
	mine = read(t, root, ".luarc.json")
	result = ok(t, root, "setup")
	contains(t, result.output, "warning: .luarc.json is not plain JSON, so setup left it alone. Make sure its "+
		"runtime.path has src/?.lua, src/?/init.lua, lua/?.lua, lua/?/init.lua, its workspace.library has "+
		".moonwell/types, .moonwell/lua and its workspace.ignoreDir has dist, maps, .moonwell/libraries.")
	if read(t, root, ".luarc.json") != mine {
		t.Error("setup changed a .luarc.json that is not plain JSON")
	}
}
