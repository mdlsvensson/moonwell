package script

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestMacrosOfPointsYueAtItsFolderAndHashesTheModule(t *testing.T) {
	root, wantPath, wantFile := "/project", "/project/.moonwell/yue/?.lua", "/project/.moonwell/yue/moonwell/macros.yue"
	if runtime.GOOS == "windows" {
		root, wantPath, wantFile = `C:\project`, `C:\project\.moonwell\yue\?.lua`, `C:\project\.moonwell\yue\moonwell\macros.yue`
	}
	search, err := macrosOf(root)
	if err != nil {
		t.Fatal(err)
	}
	if search.path != wantPath || search.hash != fsx.SHA256Hex([]byte(moonwell.MacrosYue)) {
		t.Errorf("macrosOf = %+v, want the path %s", search, wantPath)
	}
	if MacrosFile != ".moonwell/yue/moonwell/macros.yue" {
		t.Errorf("MacrosFile = %s", MacrosFile)
	}
	// The compiler puts the module's name in place of the "?", with the system's separator for its dot, and tries
	// the pattern with .yue: that is the file RefreshMacros writes.
	found := strings.Replace(search.path, "?.lua", "moonwell"+string(filepath.Separator)+"macros.yue", 1)
	if written := filepath.Join(root, filepath.FromSlash(MacrosFile)); found != wantFile || written != wantFile {
		t.Errorf("the search path %s finds %s, and the macro module is written to %s, want %s for both", search.path, found, written, wantFile)
	}
}

func TestMacrosOfRefusesAProjectFolderWhosePathHasASemicolonOrAQuestionMark(t *testing.T) {
	for _, root := range []string{"/pro;ject", "/pro?ject"} {
		search, err := macrosOf(root)
		failure := asError(t, err, root)
		if failure.Msg != `The project folder's path contains ";" or "?", which YueScript's module search cannot handle.` ||
			failure.File != root || failure.Hint != "Move the project to a folder whose path has neither character." || search != (macros{}) {
			t.Errorf("%s: %+v, %+v", root, search, failure)
		}
	}
}

func TestRefreshMacrosWritesTheMacroModuleWhenItsContentDiffers(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, filepath.FromSlash(MacrosFile))
	holds := func(what string) {
		t.Helper()
		if text, err := os.ReadFile(file); err != nil || string(text) != moonwell.MacrosYue {
			t.Errorf("%s: the file holds %q (%v)", what, text, err)
		}
	}
	if wrote, err := RefreshMacros(root); err != nil || !wrote {
		t.Fatalf("in a new project: wrote %v, %v", wrote, err)
	}
	holds("in a new project")
	written := testkit.Snapshot(t, root)
	if len(written) != 4 {
		t.Errorf("a refresh made %d files and folders, want the file and its three folders", len(written))
	}
	if wrote, err := RefreshMacros(root); err != nil || wrote {
		t.Errorf("with the file as it must be: wrote %v, %v", wrote, err)
	}
	testkit.WriteFile(t, root, MacrosFile, []byte("-- changed by hand\n"))
	if wrote, err := RefreshMacros(root); err != nil || !wrote {
		t.Errorf("with a changed file: wrote %v, %v", wrote, err)
	}
	holds("with a changed file")
}

func TestRefreshMacrosNamesTheFileWhenItCannotBeWritten(t *testing.T) {
	for what, p := range map[string]project{
		"a file for .moonwell":  files(".moonwell", "a file, not a folder"),
		"a file for the folder": files(".moonwell/yue/moonwell", "a file, not a folder"),
		"a folder for the file": files(MacrosFile+"/kept.txt", ""),
	} {
		wrote, err := RefreshMacros(p.lay(t))
		failure := asError(t, err, what)
		if wrote || !strings.HasPrefix(failure.Msg, "Writing .moonwell/yue/moonwell/macros.yue failed: ") || failure.File != MacrosFile ||
			failure.Hint != "Moonwell's compiler and the editor read .moonwell/; make sure it is a folder you can write, then retry." ||
			failure.Cause == nil {
			t.Errorf("%s: wrote %v, %+v", what, wrote, failure)
		}
	}
}

func TestRefreshMacrosRefusesALinkOnTheWayToTheFile(t *testing.T) {
	for _, link := range []string{".moonwell", ".moonwell/yue", ".moonwell/yue/moonwell"} {
		root, elsewhere := t.TempDir(), t.TempDir()
		at := filepath.Join(root, filepath.FromSlash(link))
		if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
			t.Fatal(err)
		}
		// The test keeps the folder the link leads to, to see that nothing was written into it.
		testkit.LinkDir(t, elsewhere, at)
		wrote, err := RefreshMacros(root)
		failure := asError(t, err, link)
		if wrote || failure.Msg != "Symlinks are not supported: "+at || !strings.Contains(failure.Hint, "real files") {
			t.Errorf("a link at %s: wrote %v, %+v", link, wrote, failure)
		}
		if through := testkit.Snapshot(t, elsewhere); len(through) != 0 {
			t.Errorf("a link at %s: %d files were written through it", link, len(through))
		}
	}
}
