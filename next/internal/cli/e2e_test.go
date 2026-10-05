package cli

import (
	"path/filepath"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/manifest"
)

// The tests of this file run whole command lines in a project that init made. Each runs the real pkl, and takes
// the time that takes. Setup runs in a seeded world: it asks for the compiler and never starts it, and what it
// keeps for the editor goes into a cache of the test's own.

func TestE2ESetupLocalManifestCreatesAndKeeps(t *testing.T) {
	world, root := seeded(t), newProject(t, "my-map")
	remove(t, root, "moonwell.local.pkl")
	world.ok(t, root, "setup")
	if read(t, root, "moonwell.local.pkl") != manifest.LocalPkl() {
		t.Fatal("wrong local manifest")
	}
	mine := "amends \"moonwell.pkl\"\nlaunch { gameExecutable = \"/games/wc3.exe\" }\n"
	write(t, root, "moonwell.local.pkl", mine)
	world.ok(t, root, "setup")
	if read(t, root, "moonwell.local.pkl") != mine {
		t.Fatal("overwritten")
	}
}

func TestE2ESetupUpgradesEditorFilesAndKeepsExisting(t *testing.T) {
	world, root := seeded(t), newProject(t, "my-map")
	for _, path := range []string{"yueconfig.yue", ".luarc.json", ".vscode"} {
		remove(t, root, path)
	}
	write(t, root, ".gitignore", "dist/\nmoonwell.local.pkl\n.pkl-lsp/\n")
	r := world.ok(t, root, "setup")
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
	world.ok(t, root, "setup")
	if read(t, root, "yueconfig.yue") != mine {
		t.Fatal("overwritten")
	}
}

func TestE2ESetupLibraryViewDespiteCollision(t *testing.T) {
	world, root := seeded(t), newProject(t, "my-map")
	useLibrary(t, root, exampleLibrary(t))
	write(t, root, "lua/example/greet.lua", "return {}\n")
	world.ok(t, root, "setup")
	contains(t, read(t, root, ".moonwell/lua/example/greet.lua"), "Hello, ")
	if !exists(root, ".moonwell/types/natives.d.lua") {
		t.Fatal("no declarations")
	}
}

func TestE2ESetupEditorFilesBeforeFailedLibrarySync(t *testing.T) {
	world, root := seeded(t), newProject(t, "my-map")
	useLibrary(t, root, filepath.Join(t.TempDir(), "missing"))
	world.fails(t, root, []string{"is not a folder"}, "setup")
	for _, path := range []string{".moonwell/types/natives.d.lua", ".moonwell/yue/moonwell/macros.yue"} {
		if !exists(root, path) {
			t.Error(path)
		}
	}
}
