package cli

import (
	"path/filepath"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/manifest"
)

// The tests of this file run whole command lines in a project that init made, in the real world: each runs the
// real pkl and the real YueScript compiler, and takes the seconds those take.

func TestE2ESetupLocalManifestCreatesAndKeeps(t *testing.T) {
	root := compiling(t)
	remove(t, root, "moonwell.local.pkl")
	ok(t, root, "setup")
	if read(t, root, "moonwell.local.pkl") != manifest.LocalPkl() {
		t.Fatal("wrong local manifest")
	}
	mine := "amends \"moonwell.pkl\"\nlaunch { gameExecutable = \"/games/wc3.exe\" }\n"
	write(t, root, "moonwell.local.pkl", mine)
	ok(t, root, "setup")
	if read(t, root, "moonwell.local.pkl") != mine {
		t.Fatal("overwritten")
	}
}

func TestE2ESetupUpgradesEditorFilesAndKeepsExisting(t *testing.T) {
	root := compiling(t)
	for _, path := range []string{"yueconfig.yue", ".luarc.json", ".vscode"} {
		remove(t, root, path)
	}
	write(t, root, ".gitignore", "dist/\nmoonwell.local.pkl\n.pkl-lsp/\n")
	r := ok(t, root, "setup")
	for _, path := range []string{
		"yueconfig.yue", ".luarc.json", ".vscode/extensions.json", ".moonwell/types/natives.d.lua",
	} {
		if !exists(root, path) {
			t.Error(path)
		}
	}
	contains(t, read(t, root, ".gitignore"), ".moonwell/\nsrc/**/*.lua\n")
	contains(t, r.output, "Added yueconfig.yue for the editor.")
	mine := "return { build: false }\n"
	write(t, root, "yueconfig.yue", mine)
	ok(t, root, "setup")
	if read(t, root, "yueconfig.yue") != mine {
		t.Fatal("overwritten")
	}
}

func TestE2ESetupLibraryViewDespiteCollision(t *testing.T) {
	root := compiling(t)
	useLibrary(t, root, exampleLibrary(t))
	write(t, root, "lua/example/greet.lua", "return {}\n")
	ok(t, root, "setup")
	contains(t, read(t, root, ".moonwell/lua/example/greet.lua"), "Hello, ")
	if !exists(root, ".moonwell/types/natives.d.lua") {
		t.Fatal("no declarations")
	}
}

func TestE2ESetupEditorFilesBeforeFailedLibrarySync(t *testing.T) {
	root := compiling(t)
	useLibrary(t, root, filepath.Join(t.TempDir(), "missing"))
	fails(t, root, []string{"is not a folder"}, "setup")
	for _, path := range []string{".moonwell/types/natives.d.lua", ".moonwell/yue/moonwell/macros.yue"} {
		if !exists(root, path) {
			t.Error(path)
		}
	}
}
