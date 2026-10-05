package editor

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/script"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// folderHint ends a failure to write or remove a file of the view.
const folderHint = "The editor reads .moonwell/; make sure it is a folder you can write, then retry."

// librarySources is the modules of a project with a library: two of the project's own, and of the library a Lua
// module, a YueScript module and an init module.
func librarySources() []script.Source {
	return []script.Source{
		{Name: "main", Path: "src/main.yue", Kind: script.Yue},
		{Name: "tools", Path: "lua/tools.lua", Kind: script.Lua, Text: "return {}"},
		{Name: "example.greet", Path: ".moonwell/libraries/ex/example/greet.lua", Kind: script.Lua, Text: "return 1", Library: "ex"},
		{Name: "example.loud", Path: ".moonwell/libraries/ex/example/loud.yue", Kind: script.Yue, Library: "ex"},
		{Name: "kit.init", Path: ".moonwell/libraries/ex/kit/init.lua", Kind: script.Lua, Text: "return 2", Library: "ex"},
	}
}

// compiled is a way to each module's Lua as a compile gives it: a Lua module's own text, and for a YueScript
// module the Lua that is given here under its name.
func compiled(yue map[string]string) func(script.Source) (string, bool) {
	return func(source script.Source) (string, bool) {
		if source.Kind == script.Lua {
			return source.Text, true
		}
		text, has := yue[source.Name]
		return text, has
	}
}

// viewIn is the files of the view of the project at root, by their paths from .moonwell/lua.
func viewIn(t testing.TB, root string) map[string]string {
	t.Helper()
	return filesIn(t, filepath.Join(root, ".moonwell", "lua"))
}

func TestRefreshLibraryViewWritesLibraryModulesAsLuaByModulePathThenOnlyChanges(t *testing.T) {
	root := t.TempDir()
	sources := librarySources()
	loud := compiled(map[string]string{"example.loud": "return 3"})
	written, err := RefreshLibraryView(root, sources, loud)
	want := []string{".moonwell/lua/example/greet.lua", ".moonwell/lua/example/loud.lua", ".moonwell/lua/kit/init.lua"}
	if err != nil || !slices.Equal(written, want) {
		t.Fatalf("RefreshLibraryView = %q, %v", written, err)
	}
	// The project's own modules have no view: the editor reads them where they are.
	held := map[string]string{"example/greet.lua": "return 1", "example/loud.lua": "return 3", "kit/init.lua": "return 2"}
	if got := viewIn(t, root); !maps.Equal(got, held) {
		t.Errorf("the view holds %q", got)
	}
	if written, err := RefreshLibraryView(root, sources, loud); err != nil || written == nil || len(written) != 0 {
		t.Errorf("unchanged, but RefreshLibraryView = %#v, %v", written, err)
	}
	none := compiled(nil)
	if written, err := RefreshLibraryView(root, sources[:3], none); err != nil || len(written) != 0 {
		t.Errorf("RefreshLibraryView = %q, %v", written, err)
	}
	if got := viewIn(t, root); !maps.Equal(got, map[string]string{"example/greet.lua": "return 1"}) {
		t.Errorf("modules that are gone kept their views: %q", got)
	}
	written, err = RefreshLibraryView(root, sources, none)
	if err != nil || !slices.Equal(written, want[2:]) || fsx.Exists(filepath.Join(root, ".moonwell", "lua", "example", "loud.lua")) {
		t.Errorf("no compiled Lua, but RefreshLibraryView = %q, %v, and the view holds %q", written, err, viewIn(t, root))
	}
}

func TestRefreshLibraryViewWithoutAWayToTheLuaKeepsTheViewsOfYueScriptLibraryModules(t *testing.T) {
	root := t.TempDir()
	sources := librarySources()
	loud, greet := sources[3], sources[2]
	if _, err := RefreshLibraryView(root, []script.Source{loud, greet}, compiled(map[string]string{"example.loud": "return 3"})); err != nil {
		t.Fatal(err)
	}
	// Setup, after a check: it compiles nothing.
	written, err := RefreshLibraryView(root, []script.Source{loud, greet}, nil)
	if got := viewIn(t, root); err != nil || written == nil || len(written) != 0 || got["example/loud.lua"] != "return 3" {
		t.Errorf("RefreshLibraryView = %#v, %v; the view holds %q", written, err, got)
	}
	// A Lua module is its own text, which no compile is needed for.
	greet.Text = "return 10"
	written, err = RefreshLibraryView(root, []script.Source{loud, greet}, nil)
	held := map[string]string{"example/greet.lua": "return 10", "example/loud.lua": "return 3"}
	if got := viewIn(t, root); err != nil || !slices.Equal(written, []string{".moonwell/lua/example/greet.lua"}) || !maps.Equal(got, held) {
		t.Errorf("after a change of a Lua module, RefreshLibraryView = %q, %v; the view holds %q", written, err, got)
	}
	if _, err := RefreshLibraryView(root, []script.Source{greet}, nil); err != nil || len(viewIn(t, root)) != 1 {
		t.Errorf("a module that went away kept its view (%v): %q", err, viewIn(t, root))
	}
	// A YueScript module without a view gets none: there is no Lua to write.
	if written, err := RefreshLibraryView(root, []script.Source{loud, greet}, nil); err != nil || len(written) != 0 || len(viewIn(t, root)) != 1 {
		t.Errorf("RefreshLibraryView = %q, %v; the view holds %q", written, err, viewIn(t, root))
	}
}

func TestRefreshLibraryViewAsksForTheLuaOfEachLibraryModuleAndOfNoOther(t *testing.T) {
	root := t.TempDir()
	if _, err := RefreshLibraryView(root, librarySources(), compiled(map[string]string{"example.loud": "return 3"})); err != nil {
		t.Fatal(err)
	}
	var asked []string
	onlyKit := func(source script.Source) (string, bool) {
		asked = append(asked, source.Name)
		return "return 'kit'", source.Name == "kit.init"
	}
	// A module that has no Lua has no view, whatever its kind, and the Lua is what the function gives.
	written, err := RefreshLibraryView(root, librarySources(), onlyKit)
	if err != nil || !slices.Equal(written, []string{".moonwell/lua/kit/init.lua"}) {
		t.Errorf("RefreshLibraryView = %q, %v", written, err)
	}
	if !slices.Equal(asked, []string{"example.greet", "example.loud", "kit.init"}) {
		t.Errorf("the Lua was asked for of %q", asked)
	}
	if got := viewIn(t, root); !maps.Equal(got, map[string]string{"kit/init.lua": "return 'kit'"}) {
		t.Errorf("the view holds %q", got)
	}
}

func TestRefreshLibraryViewWritesAModulesBytesAsTheyAre(t *testing.T) {
	// A byte order mark that is no start of the file, a byte that is no UTF-8, half of a pair, a NUL and a
	// carriage return: none is decoded or put right.
	const faulty = "return '\xff\xfe \xed\xa0\x80 \xc3 \x00'\r\n-- \xef\xbb\xbf\n"
	root := t.TempDir()
	sources := []script.Source{
		{Name: "raw.own", Path: ".moonwell/libraries/ex/raw/own.lua", Kind: script.Lua, Text: faulty, Library: "ex"},
		{Name: "raw.made", Path: ".moonwell/libraries/ex/raw/made.yue", Kind: script.Yue, Library: "ex"},
	}
	want := map[string]string{"raw/own.lua": faulty, "raw/made.lua": "-- made\n" + faulty}
	if _, err := RefreshLibraryView(root, sources, compiled(map[string]string{"raw.made": "-- made\n" + faulty})); err != nil {
		t.Fatal(err)
	}
	if got := viewIn(t, root); !maps.Equal(got, want) {
		t.Errorf("the view holds %q", got)
	}
	for _, lua := range []func(script.Source) (string, bool){nil, compiled(map[string]string{"raw.made": "-- made\n" + faulty})} {
		if written, err := RefreshLibraryView(root, sources, lua); err != nil || len(written) != 0 || !maps.Equal(viewIn(t, root), want) {
			t.Errorf("unchanged, but RefreshLibraryView = %q, %v; the view holds %q", written, err, viewIn(t, root))
		}
	}
}

func TestRefreshLibraryViewRemovesEveryOtherFileOfTheFolderAndNothingOutsideIt(t *testing.T) {
	root := lay(t,
		".moonwell/lua/gone.lua", "return 0", ".moonwell/lua/example/gone/deep.lua", "return 0", ".moonwell/lua/notes.txt", "",
		".moonwell/lua/example/greet.lua", "return 1", ".moonwell/lua/EXAMPLE.lua", "",
		".moonwell/libraries/ex/example/greet.lua", "return 1", ".moonwell/types/natives.d.lua", "---@meta\n",
		".moonwell/lua.txt", "beside", ".moonwell/luau/kept.lua", "beside", "lua/tools.lua", "return {}", "src/main.yue", "",
	)
	before := testkit.Snapshot(t, root)
	written, err := RefreshLibraryView(root, librarySources()[:3], nil)
	if err != nil || len(written) != 0 {
		t.Fatalf("RefreshLibraryView = %q, %v", written, err)
	}
	after := testkit.Snapshot(t, root)
	for _, removed := range []string{".moonwell/lua/gone.lua", ".moonwell/lua/example/gone/deep.lua", ".moonwell/lua/notes.txt", ".moonwell/lua/EXAMPLE.lua"} {
		if _, there := after[removed]; there {
			t.Errorf("%s is no view of a module and is still there", removed)
		}
		delete(before, removed)
	}
	// Files are removed, and a folder that holds nothing more stays.
	if !reflect.DeepEqual(before, after) {
		t.Errorf("more changed than the files that are no views:\nbefore %q\nafter  %q", slices.Sorted(maps.Keys(before)), slices.Sorted(maps.Keys(after)))
	}
}

func TestRefreshLibraryViewOfAProjectWithoutLibraryModulesMakesNoFolder(t *testing.T) {
	root := t.TempDir()
	for _, lua := range []func(script.Source) (string, bool){nil, compiled(nil)} {
		written, err := RefreshLibraryView(root, librarySources()[:2], lua)
		if err != nil || written == nil || len(written) != 0 {
			t.Errorf("RefreshLibraryView = %#v, %v", written, err)
		}
	}
	if left := testkit.Snapshot(t, root); len(left) != 0 {
		t.Errorf("written without a library module: %q", slices.Sorted(maps.Keys(left)))
	}
}

func TestRefreshLibraryViewRefusesAModuleWhoseNameNamesNoFileOfTheFolder(t *testing.T) {
	// No listing of modules gives such a name: a part of a name is a file's or a folder's own name.
	for _, name := range []string{"", ".", "..", "...", "a..b", ".a", "a.", "a/", "/a", "a//b", "../../outside", "a/../../../outside"} {
		above := lay(t, "project/.moonwell/lua/kept.lua", "return 0")
		root := filepath.Join(above, "project")
		sources := []script.Source{
			{Name: "fine", Path: ".moonwell/libraries/ex/fine.lua", Kind: script.Lua, Text: "return 1", Library: "ex"},
			{Name: name, Path: ".moonwell/libraries/ex/odd.lua", Kind: script.Lua, Text: "return 1", Library: "ex"},
		}
		for _, lua := range []func(script.Source) (string, bool){nil, compiled(nil)} {
			written, err := RefreshLibraryView(root, sources, lua)
			var expected *diag.Error
			if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "editor.RefreshLibraryView") || written != nil {
				t.Errorf("a module named %q: RefreshLibraryView = %q, %v, want an error that is no expected failure", name, written, err)
			}
		}
		// The refusal comes before anything is written or removed.
		if got := filesIn(t, above); len(got) != 1 || got["project/.moonwell/lua/kept.lua"] != "return 0" {
			t.Errorf("a module named %q: the folder above the project holds %q", name, slices.Sorted(maps.Keys(got)))
		}
	}
	// A module of the project's own has no view, so its name is not looked at.
	own := []script.Source{{Name: "..", Path: "lua/odd.lua", Kind: script.Lua, Text: "return 1"}}
	if written, err := RefreshLibraryView(t.TempDir(), own, nil); err != nil || len(written) != 0 {
		t.Errorf("a module of the project's own: RefreshLibraryView = %q, %v", written, err)
	}
}

func TestALinkOnTheWayToTheLibraryViewIsRefused(t *testing.T) {
	for _, link := range []string{".moonwell", ".moonwell/lua"} {
		root := t.TempDir()
		at, target := linkAt(t, root, link)
		write(t, target, "lua/behind.lua", "return 0", "behind.lua", "return 0")
		behind := testkit.Snapshot(t, target)
		for _, lua := range []func(script.Source) (string, bool){nil, compiled(nil)} {
			written, err := RefreshLibraryView(root, librarySources(), lua)
			failure := asError(t, err, "a link at "+link)
			if failure.Msg != "Symlinks are not supported: "+at || !strings.Contains(failure.Hint, "real files") || written != nil {
				t.Errorf("a link at %s: RefreshLibraryView = %q, %+v", link, written, failure)
			}
		}
		if !reflect.DeepEqual(behind, testkit.Snapshot(t, target)) {
			t.Errorf("a link at %s: what is behind the link changed", link)
		}
	}
}

func TestBelowTheLibraryViewALinkIsWrittenThroughAndRemovedAsALink(t *testing.T) {
	// The folder is Moonwell's own: what is below it is not looked at for links. A link there is a file of the
	// folder that is no view, and is removed as the link it is: what it leads to is not listed.
	root := t.TempDir()
	at, target := linkAt(t, root, ".moonwell/lua/example")
	stale, kept := linkAt(t, root, ".moonwell/lua/stale")
	write(t, target, "beside.lua", "return 0")
	write(t, kept, "kept.lua", "return 0", "deep/kept.lua", "return 0")
	written, err := RefreshLibraryView(root, librarySources()[:3], nil)
	if err != nil || !slices.Equal(written, []string{".moonwell/lua/example/greet.lua"}) {
		t.Fatalf("RefreshLibraryView = %q, %v", written, err)
	}
	if got := filesIn(t, target); !maps.Equal(got, map[string]string{"beside.lua": "return 0", "greet.lua": "return 1"}) {
		t.Errorf("behind the link at the place of a module's folder: %q", got)
	}
	if got := filesIn(t, kept); len(got) != 2 {
		t.Errorf("behind the link that is no view: %q", got)
	}
	for _, link := range []string{at, stale} {
		if info, err := fsx.Lstat(link); err != nil || info != nil {
			t.Errorf("the link at %s is still there (%v)", link, err)
		}
	}
	// The next refresh writes the view into a folder of the view's own.
	written, err = RefreshLibraryView(root, librarySources()[:3], nil)
	if got := viewIn(t, root); err != nil || len(written) != 1 || !maps.Equal(got, map[string]string{"example/greet.lua": "return 1"}) {
		t.Errorf("after the link was removed, RefreshLibraryView = %q, %v; the view holds %q", written, err, got)
	}
}

func TestAFileInThePlaceOfTheLibraryViewIsRemovedOrNamedWhereAViewIsWritten(t *testing.T) {
	root := lay(t, ".moonwell/lua", "a file, not a folder")
	written, err := RefreshLibraryView(root, librarySources()[:3], nil)
	failure := asError(t, err, "a file for .moonwell/lua")
	if !strings.HasPrefix(failure.Msg, "Writing .moonwell/lua/example/greet.lua failed: ") ||
		failure.File != ".moonwell/lua/example/greet.lua" || failure.Hint != folderHint || failure.Cause == nil || written != nil {
		t.Errorf("RefreshLibraryView = %q, %+v", written, failure)
	}
	// Without a view to write, the file is one that is no view.
	if written, err := RefreshLibraryView(root, nil, nil); err != nil || len(written) != 0 || fsx.Exists(filepath.Join(root, ".moonwell", "lua")) {
		t.Errorf("RefreshLibraryView = %q, %v, and the file is there: %v", written, err, fsx.Exists(filepath.Join(root, ".moonwell", "lua")))
	}
}

func TestAMoonwellFolderThatIsAFileFailsTheLibraryView(t *testing.T) {
	root := lay(t, ".moonwell", "a file, not a folder")
	written, err := RefreshLibraryView(root, librarySources(), compiled(nil))
	failure := asError(t, err, "a file for .moonwell")
	// The system decides where the failure shows: on the way to the folder, or at the first view.
	atFolder := failure.File == ".moonwell/lua" && strings.HasPrefix(failure.Msg, "Writing .moonwell/lua failed: ")
	atView := failure.File == ".moonwell/lua/example/greet.lua" && strings.HasPrefix(failure.Msg, "Writing .moonwell/lua/example/greet.lua failed: ")
	if !(atFolder || atView) || failure.Hint != folderHint || written != nil {
		t.Errorf("RefreshLibraryView = %q, %+v", written, failure)
	}
	if got := read(t, root, ".moonwell"); got != "a file, not a folder" {
		t.Errorf(".moonwell holds %q", got)
	}
}

func TestRefreshLibraryViewReportsAViewItCannotWrite(t *testing.T) {
	root := t.TempDir()
	sources := librarySources()
	if _, err := RefreshLibraryView(root, sources, nil); err != nil {
		t.Fatal(err)
	}
	testkit.MakeUnwritable(t, filepath.Join(root, ".moonwell", "lua", "kit", "init.lua"))
	// A view that cannot be written and holds what it is to hold is no failure: nothing is written.
	if written, err := RefreshLibraryView(root, sources, nil); err != nil || len(written) != 0 {
		t.Errorf("unchanged, but RefreshLibraryView = %q, %v", written, err)
	}
	sources[2].Text, sources[4].Text = "return 'greet'", "return 'kit'"
	written, err := RefreshLibraryView(root, sources, nil)
	failure := asError(t, err, "a view that cannot be written")
	if !strings.HasPrefix(failure.Msg, "Writing .moonwell/lua/kit/init.lua failed: ") || failure.File != ".moonwell/lua/kit/init.lua" ||
		failure.Hint != folderHint || failure.Cause == nil || written != nil {
		t.Errorf("RefreshLibraryView = %q, %+v", written, failure)
	}
	// The views are written in the order of the modules, and the first failure ends the refresh.
	if got := viewIn(t, root); got["example/greet.lua"] != "return 'greet'" || got["kit/init.lua"] != "return 2" {
		t.Errorf("the view holds %q", got)
	}
}

func TestAFileOfTheLibraryViewThatCannotBeRemovedIsReported(t *testing.T) {
	root := lay(t, ".moonwell/lua/old/gone.lua", "return 0")
	gone := filepath.Join(root, ".moonwell", "lua", "old", "gone.lua")
	if runtime.GOOS == "windows" {
		// A file another program holds: the failure is the one every removal of a held file gives.
		testkit.MakeUnwritable(t, gone)
		_, err := RefreshLibraryView(root, nil, nil)
		if failure := asError(t, err, "a held file"); failure.Msg != gone+" is in use by another program." || failure.Hint == "" {
			t.Errorf("RefreshLibraryView = %+v", failure)
		}
		return
	}
	if os.Geteuid() == 0 {
		t.Skip("root may remove a file from a folder without the permission, so the removal would not fail")
	}
	if err := os.Chmod(filepath.Dir(gone), 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(filepath.Dir(gone), 0o777) })
	written, err := RefreshLibraryView(root, nil, nil)
	failure := asError(t, err, "a file in a folder that cannot be written")
	if !strings.HasPrefix(failure.Msg, "Removing .moonwell/lua/old/gone.lua failed: ") || failure.File != ".moonwell/lua/old/gone.lua" ||
		failure.Hint != folderHint || failure.Cause == nil || written != nil {
		t.Errorf("RefreshLibraryView = %q, %+v", written, failure)
	}
}
