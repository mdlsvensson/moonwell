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

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const (
	folderHint  = "The editor reads .moonwell/; make sure it is a folder you can write, then retry."
	removalHint = "The editor reads .moonwell/lua/: close the file there if a program has it open, " +
		"and make sure the folder is one you can write, then retry."
	readingHint = "The editor reads .moonwell/; make sure it is a folder you can read, then retry."
)

func librarySources() []script.Source {
	return []script.Source{
		{Name: "main", Path: "src/main.yue", Kind: script.Yue},
		{Name: "tools", Path: "lua/tools.lua", Kind: script.Lua, Text: "return {}"},
		{Name: "example.greet", Path: ".moonwell/libraries/ex/example/greet.lua", Kind: script.Lua, Text: "return 1", Library: "ex"},
		{Name: "example.loud", Path: ".moonwell/libraries/ex/example/loud.yue", Kind: script.Yue, Library: "ex"},
		{Name: "kit.init", Path: ".moonwell/libraries/ex/kit/init.lua", Kind: script.Lua, Text: "return 2", Library: "ex"},
	}
}

func compiled(yue map[string]string) func(script.Source) (string, bool) {
	return func(source script.Source) (string, bool) {
		if source.Kind == script.Lua {
			return source.Text, true
		}
		text, has := yue[source.Name]
		return text, has
	}
}

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
	written, err := RefreshLibraryView(root, []script.Source{loud, greet}, nil)
	if got := viewIn(t, root); err != nil || written == nil || len(written) != 0 || got["example/loud.lua"] != "return 3" {
		t.Errorf("RefreshLibraryView = %#v, %v; the view holds %q", written, err, got)
	}
	greet.Text = "return 10"
	written, err = RefreshLibraryView(root, []script.Source{loud, greet}, nil)
	held := map[string]string{"example/greet.lua": "return 10", "example/loud.lua": "return 3"}
	if got := viewIn(t, root); err != nil || !slices.Equal(written, []string{".moonwell/lua/example/greet.lua"}) || !maps.Equal(got, held) {
		t.Errorf("after a change of a Lua module, RefreshLibraryView = %q, %v; the view holds %q", written, err, got)
	}
	if _, err := RefreshLibraryView(root, []script.Source{greet}, nil); err != nil || len(viewIn(t, root)) != 1 {
		t.Errorf("a module that went away kept its view (%v): %q", err, viewIn(t, root))
	}
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

func TestRefreshLibraryViewRemovesEveryOtherFileAndEveryFolderThatHoldsNothingAndNothingOutsideTheFolder(t *testing.T) {
	root := lay(t,
		".moonwell/lua/gone.lua", "return 0", ".moonwell/lua/example/gone/deep.lua", "return 0", ".moonwell/lua/notes.txt", "",
		".moonwell/lua/example/greet.lua", "return 1", ".moonwell/lua/EXAMPLE.lua", "", ".moonwell/lua/far/down/below/gone.lua", "",
		".moonwell/libraries/ex/example/greet.lua", "return 1", ".moonwell/types/natives.d.lua", "---@meta\n",
		".moonwell/lua.txt", "beside", ".moonwell/luau/kept.lua", "beside", "lua/tools.lua", "return {}", "src/main.yue", "",
	)
	for _, hollow := range []string{".moonwell/lua/hollow/er/est", ".moonwell/lua/example/hollow", ".moonwell/luau/hollow", "lua/hollow"} {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(hollow)), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	before := testkit.Snapshot(t, root)
	written, err := RefreshLibraryView(root, librarySources()[:3], nil)
	if err != nil || len(written) != 0 {
		t.Fatalf("RefreshLibraryView = %q, %v", written, err)
	}
	maps.DeleteFunc(before, func(path string, _ []byte) bool { return strings.HasPrefix(path, ".moonwell/lua/") })
	before[".moonwell/lua/example"], before[".moonwell/lua/example/greet.lua"] = nil, []byte("return 1")
	if after := testkit.Snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Errorf("the project holds %q, want %q", slices.Sorted(maps.Keys(after)), slices.Sorted(maps.Keys(before)))
	}
	written, err = RefreshLibraryView(root, nil, nil)
	if got := entriesIn(t, filepath.Join(root, ".moonwell")); err != nil || len(written) != 0 ||
		!slices.Equal(got, []string{"libraries", "libraries/ex", "libraries/ex/example", "libraries/ex/example/greet.lua",
			"lua", "lua.txt", "luau", "luau/hollow", "luau/kept.lua", "types", "types/natives.d.lua"}) {
		t.Errorf("RefreshLibraryView = %q, %v; .moonwell holds %q", written, err, got)
	}
}

func renamed(name string) []script.Source {
	path := ".moonwell/libraries/ex/" + strings.ReplaceAll(name, ".", "/") + ".lua"
	return []script.Source{{Name: name, Path: path, Kind: script.Lua, Text: "return 2", Library: "ex"}}
}

func TestAViewIsUnderTheSpellingOfItsModulesNameAfterOneRun(t *testing.T) {
	cases := []struct {
		what, before, after string
		held                []string
	}{
		{"a folder in another letter case", "Kit.init", "kit.init", []string{"kit", "kit/init.lua"}},
		{"a file in another letter case", "kit.Greet", "kit.greet", []string{"kit", "kit/greet.lua"}},
		{"a folder and a file below it", "kit.Deep.Mod", "kit.deep.mod", []string{"kit", "kit/deep", "kit/deep/mod.lua"}},
		{"another name", "old.init", "new.init", []string{"new", "new/init.lua"}},
	}
	for _, c := range cases {
		for mode, lua := range map[string]func(script.Source) (string, bool){"without a compile": nil, "after a compile": compiled(nil)} {
			root, what := t.TempDir(), c.what+", "+mode
			view := filepath.Join(root, ".moonwell", "lua")
			if _, err := RefreshLibraryView(root, renamed(c.before), lua); err != nil {
				t.Fatal(err)
			}
			written, err := RefreshLibraryView(root, renamed(c.after), lua)
			want := []string{".moonwell/lua/" + c.held[len(c.held)-1]}
			if got := entriesIn(t, view); err != nil || !slices.Equal(written, want) || !slices.Equal(got, c.held) {
				t.Errorf("%s: RefreshLibraryView = %q, %v; the view holds %q, want %q", what, written, err, got, c.held)
			}
			written, err = RefreshLibraryView(root, renamed(c.after), lua)
			if got := entriesIn(t, view); err != nil || len(written) != 0 || !slices.Equal(got, c.held) {
				t.Errorf("%s, again: RefreshLibraryView = %q, %v; the view holds %q", what, written, err, got)
			}
		}
	}
}

func TestAnUnchangedLibraryViewIsLeftAsItIs(t *testing.T) {
	root := t.TempDir()
	view := filepath.Join(root, ".moonwell", "lua")
	loud := compiled(map[string]string{"example.loud": "return 3"})
	if _, err := RefreshLibraryView(root, librarySources(), loud); err != nil {
		t.Fatal(err)
	}
	before := testkit.Snapshot(t, view)
	if len(before) != 5 {
		t.Fatalf("the view holds %q", slices.Sorted(maps.Keys(before)))
	}
	for i, lua := range []func(script.Source) (string, bool){loud, nil, loud} {
		written, err := RefreshLibraryView(root, librarySources(), lua)
		if err != nil || written == nil || len(written) != 0 || !reflect.DeepEqual(before, testkit.Snapshot(t, view)) {
			t.Errorf("run %d: RefreshLibraryView = %#v, %v; the view holds %q", i, written, err, viewIn(t, root))
		}
	}
}

func TestWithoutAWayToTheLuaAYueScriptModulesViewIsKeptByItsExactNameAlone(t *testing.T) {
	root := lay(t, ".moonwell/lua/example/loud.lua", "return 3", ".moonwell/lua/Kit/Shout.lua", "return 4")
	view := filepath.Join(root, ".moonwell", "lua")
	sources := []script.Source{
		{Name: "example.loud", Path: ".moonwell/libraries/ex/example/loud.yue", Kind: script.Yue, Library: "ex"},
		{Name: "kit.shout", Path: ".moonwell/libraries/ex/kit/shout.yue", Kind: script.Yue, Library: "ex"},
	}
	written, err := RefreshLibraryView(root, sources, nil)
	if got := entriesIn(t, view); err != nil || len(written) != 0 || !slices.Equal(got, []string{"example", "example/loud.lua"}) ||
		read(t, root, ".moonwell/lua/example/loud.lua") != "return 3" {
		t.Errorf("RefreshLibraryView = %q, %v; the view holds %q", written, err, got)
	}
	lua := compiled(map[string]string{"example.loud": "return 3", "kit.shout": "return 4"})
	written, err = RefreshLibraryView(root, sources, lua)
	held := []string{"example", "example/loud.lua", "kit", "kit/shout.lua"}
	if got := entriesIn(t, view); err != nil || !slices.Equal(written, []string{".moonwell/lua/kit/shout.lua"}) || !slices.Equal(got, held) {
		t.Errorf("after a compile, RefreshLibraryView = %q, %v; the view holds %q", written, err, got)
	}
	for _, lua := range []func(script.Source) (string, bool){lua, nil} {
		written, err = RefreshLibraryView(root, sources, lua)
		if got := entriesIn(t, view); err != nil || len(written) != 0 || !slices.Equal(got, held) {
			t.Errorf("unchanged, but RefreshLibraryView = %q, %v; the view holds %q", written, err, got)
		}
	}
}

func TestAStrayFileOrFolderInTheWayOfAViewIsRemovedInOneRun(t *testing.T) {
	root := lay(t, ".moonwell/lua/kit", "a file", ".moonwell/lua/tool.lua/inside.txt", "a file of a folder")
	view := filepath.Join(root, ".moonwell", "lua")
	if err := os.MkdirAll(filepath.Join(view, "bare.lua", "deeper"), 0o777); err != nil {
		t.Fatal(err)
	}
	sources := slices.Concat(renamed("kit.init"), renamed("tool"), renamed("bare"))
	written, err := RefreshLibraryView(root, sources, nil)
	want := []string{".moonwell/lua/kit/init.lua", ".moonwell/lua/tool.lua", ".moonwell/lua/bare.lua"}
	held := map[string]string{"kit/init.lua": "return 2", "tool.lua": "return 2", "bare.lua": "return 2"}
	if got := viewIn(t, root); err != nil || !slices.Equal(written, want) || !maps.Equal(got, held) {
		t.Errorf("RefreshLibraryView = %q, %v; the view holds %q", written, err, got)
	}
	if written, err := RefreshLibraryView(root, sources, nil); err != nil || len(written) != 0 || !maps.Equal(viewIn(t, root), held) {
		t.Errorf("again: RefreshLibraryView = %q, %v; the view holds %q", written, err, viewIn(t, root))
	}
}

func TestAModuleThatBecomesAFolderOfModulesAndAModuleAgain(t *testing.T) {
	root := t.TempDir()
	view := filepath.Join(root, ".moonwell", "lua")
	steps := []struct {
		name string
		held []string
	}{{"a", []string{"a.lua"}}, {"a.b", []string{"a", "a/b.lua"}}, {"a", []string{"a.lua"}}, {"a.b.c", []string{"a", "a/b", "a/b/c.lua"}}, {"a.b", []string{"a", "a/b.lua"}}}
	for i, step := range steps {
		written, err := RefreshLibraryView(root, renamed(step.name), nil)
		want := []string{".moonwell/lua/" + step.held[len(step.held)-1]}
		if got := entriesIn(t, view); err != nil || !slices.Equal(written, want) || !slices.Equal(got, step.held) {
			t.Errorf("step %d, the module %s: RefreshLibraryView = %q, %v; the view holds %q", i, step.name, written, err, got)
		}
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
		if got := filesIn(t, above); len(got) != 1 || got["project/.moonwell/lua/kept.lua"] != "return 0" {
			t.Errorf("a module named %q: the folder above the project holds %q", name, slices.Sorted(maps.Keys(got)))
		}
	}
	own := []script.Source{{Name: "..", Path: "lua/odd.lua", Kind: script.Lua, Text: "return 1"}}
	if written, err := RefreshLibraryView(t.TempDir(), own, nil); err != nil || len(written) != 0 {
		t.Errorf("a module of the project's own: RefreshLibraryView = %q, %v", written, err)
	}
}

func TestALinkOnTheWayToTheLibraryViewIsRefused(t *testing.T) {
	for _, symlink := range []string{".moonwell", ".moonwell/lua"} {
		root := t.TempDir()
		at, target := linkAt(t, root, symlink)
		write(t, target, "lua/behind.lua", "return 0", "behind.lua", "return 0")
		behind := testkit.Snapshot(t, target)
		for _, lua := range []func(script.Source) (string, bool){nil, compiled(nil)} {
			written, err := RefreshLibraryView(root, librarySources(), lua)
			diagErr := asError(t, err, "a link at "+symlink)
			if diagErr.Msg != "Symlinks are not supported: "+at || diagErr.File != LibraryViewDir ||
				!strings.Contains(diagErr.Hint, "real files") || written != nil {
				t.Errorf("a link at %s: RefreshLibraryView = %q, %+v", symlink, written, diagErr)
			}
		}
		if !reflect.DeepEqual(behind, testkit.Snapshot(t, target)) {
			t.Errorf("a link at %s: what is behind the link changed", symlink)
		}
	}
}

func isPlain(t testing.TB, path string) bool {
	t.Helper()
	info, err := fsx.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info != nil && !fsx.IsSymlink(info)
}

func TestALinkToAFolderBelowTheLibraryViewIsRemovedAsTheLinkBeforeAnythingIsWritten(t *testing.T) {
	root := t.TempDir()
	atFolder, behindFolder := linkAt(t, root, ".moonwell/lua/example")
	atFile, behindFile := linkAt(t, root, ".moonwell/lua/kit/init.lua")
	atOther, behindOther := linkAt(t, root, ".moonwell/lua/stale")
	write(t, behindFolder, "beside.lua", "return 0", "greet.lua", "behind the link")
	write(t, behindFile, "inside.lua", "return 0")
	write(t, behindOther, "kept.lua", "return 0", "deep/kept.lua", "return 0")
	behind := []map[string][]byte{testkit.Snapshot(t, behindFolder), testkit.Snapshot(t, behindFile), testkit.Snapshot(t, behindOther)}
	sources := slices.Concat(librarySources()[:3], librarySources()[4:])
	written, err := RefreshLibraryView(root, sources, nil)
	if err != nil || !slices.Equal(written, []string{".moonwell/lua/example/greet.lua", ".moonwell/lua/kit/init.lua"}) {
		t.Fatalf("RefreshLibraryView = %q, %v", written, err)
	}
	for i, target := range []string{behindFolder, behindFile, behindOther} {
		if !reflect.DeepEqual(behind[i], testkit.Snapshot(t, target)) {
			t.Errorf("what is behind link %d changed: %q", i, filesIn(t, target))
		}
	}
	if !isPlain(t, atFolder) || !isPlain(t, atFile) || fsx.Exists(atOther) {
		t.Errorf("of the links, there is still one: %v, %v, %v", !isPlain(t, atFolder), !isPlain(t, atFile), fsx.Exists(atOther))
	}
	held := map[string]string{"example/greet.lua": "return 1", "kit/init.lua": "return 2"}
	if got := viewIn(t, root); !maps.Equal(got, held) || !isPlain(t, filepath.Join(root, filepath.FromSlash(written[0]))) {
		t.Errorf("the view holds %q", got)
	}
	if written, err := RefreshLibraryView(root, sources, nil); err != nil || len(written) != 0 || !maps.Equal(viewIn(t, root), held) {
		t.Errorf("again: RefreshLibraryView = %q, %v; the view holds %q", written, err, viewIn(t, root))
	}
}

func TestALinkToAFileAtAViewsPlaceIsRemovedAsTheLink(t *testing.T) {
	root := lay(t, "elsewhere/mine.lua", "mine", "elsewhere/same.lua", "return 2", "elsewhere/loud.lua", "return 3")
	view := filepath.Join(root, ".moonwell", "lua")
	linkToFile(t, filepath.Join(root, "elsewhere", "mine.lua"), filepath.Join(view, "example", "greet.lua"))
	linkToFile(t, filepath.Join(root, "elsewhere", "same.lua"), filepath.Join(view, "kit", "init.lua"))
	linkToFile(t, filepath.Join(root, "elsewhere", "loud.lua"), filepath.Join(view, "example", "loud.lua"))
	elsewhere := testkit.Snapshot(t, filepath.Join(root, "elsewhere"))
	written, err := RefreshLibraryView(root, librarySources(), nil)
	if err != nil || !slices.Equal(written, []string{".moonwell/lua/example/greet.lua", ".moonwell/lua/kit/init.lua"}) {
		t.Fatalf("RefreshLibraryView = %q, %v", written, err)
	}
	if !reflect.DeepEqual(elsewhere, testkit.Snapshot(t, filepath.Join(root, "elsewhere"))) {
		t.Errorf("what is behind the links changed: %q", filesIn(t, filepath.Join(root, "elsewhere")))
	}
	held := map[string]string{"example/greet.lua": "return 1", "kit/init.lua": "return 2"}
	if got := viewIn(t, root); !maps.Equal(got, held) || !isPlain(t, filepath.Join(view, "example", "greet.lua")) || !isPlain(t, filepath.Join(view, "kit", "init.lua")) {
		t.Errorf("the view holds %q, or a view is a link", got)
	}
	if written, err := RefreshLibraryView(root, librarySources(), nil); err != nil || len(written) != 0 || !maps.Equal(viewIn(t, root), held) {
		t.Errorf("again: RefreshLibraryView = %q, %v; the view holds %q", written, err, viewIn(t, root))
	}
}

func TestAFileInThePlaceOfTheLibraryViewIsRemovedAndTheViewsAreWritten(t *testing.T) {
	root := lay(t, ".moonwell/lua", "a file, not a folder")
	written, err := RefreshLibraryView(root, librarySources()[:3], nil)
	if got := viewIn(t, root); err != nil || !slices.Equal(written, []string{".moonwell/lua/example/greet.lua"}) ||
		!maps.Equal(got, map[string]string{"example/greet.lua": "return 1"}) {
		t.Errorf("RefreshLibraryView = %q, %v; the view holds %q", written, err, got)
	}
	root = lay(t, ".moonwell/lua", "a file, not a folder")
	if written, err := RefreshLibraryView(root, nil, nil); err != nil || len(written) != 0 || fsx.Exists(filepath.Join(root, ".moonwell", "lua")) {
		t.Errorf("RefreshLibraryView = %q, %v, and the file is there: %v", written, err, fsx.Exists(filepath.Join(root, ".moonwell", "lua")))
	}
}

func TestAMoonwellFolderThatIsAFileFailsTheLibraryView(t *testing.T) {
	root := lay(t, ".moonwell", "a file, not a folder")
	written, err := RefreshLibraryView(root, librarySources(), compiled(nil))
	diagErr := asError(t, err, "a file for .moonwell")
	atFolder := diagErr.File == ".moonwell/lua" && strings.HasPrefix(diagErr.Msg, "Writing .moonwell/lua failed: ")
	atView := diagErr.File == ".moonwell/lua/example/greet.lua" && strings.HasPrefix(diagErr.Msg, "Writing .moonwell/lua/example/greet.lua failed: ")
	if !(atFolder || atView) || diagErr.Hint != folderHint || written != nil {
		t.Errorf("RefreshLibraryView = %q, %+v", written, diagErr)
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
	if written, err := RefreshLibraryView(root, sources, nil); err != nil || len(written) != 0 {
		t.Errorf("unchanged, but RefreshLibraryView = %q, %v", written, err)
	}
	sources[2].Text, sources[4].Text = "return 'greet'", "return 'kit'"
	written, err := RefreshLibraryView(root, sources, nil)
	diagErr := asError(t, err, "a view that cannot be written")
	if !strings.HasPrefix(diagErr.Msg, "Writing .moonwell/lua/kit/init.lua failed: ") || diagErr.File != ".moonwell/lua/kit/init.lua" ||
		diagErr.Hint != folderHint || diagErr.Cause == nil || written != nil {
		t.Errorf("RefreshLibraryView = %q, %+v", written, diagErr)
	}
	if got := viewIn(t, root); got["example/greet.lua"] != "return 'greet'" || got["kit/init.lua"] != "return 2" {
		t.Errorf("the view holds %q", got)
	}
}

func TestAFileOfTheLibraryViewThatCannotBeRemovedIsReported(t *testing.T) {
	root := lay(t, ".moonwell/lua/old/gone.lua", "return 0")
	gone := filepath.Join(root, ".moonwell", "lua", "old", "gone.lua")
	switch {
	case runtime.GOOS == "windows":
		testkit.MakeUnwritable(t, gone)
	case os.Geteuid() == 0:
		t.Skip("root may remove a file from a folder without the permission, so the removal would not fail")
	default:
		if err := os.Chmod(filepath.Dir(gone), 0o555); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(filepath.Dir(gone), 0o777) })
	}
	written, err := RefreshLibraryView(root, nil, nil)
	diagErr := asError(t, err, "a file that cannot be removed")
	if !strings.HasPrefix(diagErr.Msg, "Removing .moonwell/lua/old/gone.lua failed: ") || diagErr.File != ".moonwell/lua/old/gone.lua" ||
		diagErr.Hint != removalHint || diagErr.Cause == nil || written != nil {
		t.Errorf("RefreshLibraryView = %q, %+v", written, diagErr)
	}
	if strings.Contains(diagErr.Msg, root) || strings.Contains(diagErr.Msg+diagErr.Hint, "Warcraft") {
		t.Errorf("the failure is worded for another file: %+v", diagErr)
	}
}

func TestAFolderOfTheLibraryViewThatCannotBeListedOrAViewThatCannotBeLookedAtIsAFailureToRead(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows lists a folder and looks at a file in it whatever its permissions; the case is covered on the other system's run")
	}
	if os.Geteuid() == 0 {
		t.Skip("root may list a folder without permissions, so the listing would not fail")
	}
	for _, c := range []struct {
		what  string
		mode  os.FileMode
		files []string
	}{
		{"a folder that cannot be listed", 0, []string{".moonwell/lua"}},
		{"a view that cannot be looked at", 0o444, []string{".moonwell/lua/kit/init.lua", ".moonwell/lua"}},
	} {
		root := t.TempDir()
		if _, err := RefreshLibraryView(root, renamed("kit.init"), nil); err != nil {
			t.Fatal(err)
		}
		kit := filepath.Join(root, ".moonwell", "lua", "kit")
		if err := os.Chmod(kit, c.mode); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { os.Chmod(kit, 0o777) })
		written, err := RefreshLibraryView(root, renamed("kit.init"), nil)
		diagErr := asError(t, err, c.what)
		if !slices.Contains(c.files, diagErr.File) || !strings.HasPrefix(diagErr.Msg, "Reading "+diagErr.File+" failed: ") ||
			diagErr.Hint != readingHint || diagErr.Cause == nil || written != nil {
			t.Errorf("%s: RefreshLibraryView = %q, %+v", c.what, written, diagErr)
		}
	}
}

func TestAFolderOfTheLibraryViewThatCannotBeRemovedIsReported(t *testing.T) {
	root := t.TempDir()
	hollow := filepath.Join(root, ".moonwell", "lua", "held", "hollow")
	if err := os.MkdirAll(hollow, 0o777); err != nil {
		t.Fatal(err)
	}
	guarded := filepath.Dir(hollow)
	if runtime.GOOS == "windows" {
		guarded = hollow
	} else if os.Geteuid() == 0 {
		t.Skip("root may remove a folder from a folder without the permission, so the removal would not fail")
	}
	if err := os.Chmod(guarded, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(guarded, 0o777) })
	written, err := RefreshLibraryView(root, renamed("kit.init"), nil)
	diagErr := asError(t, err, "a folder that cannot be removed")
	if !strings.HasPrefix(diagErr.Msg, "Removing .moonwell/lua/held/hollow failed: ") || diagErr.File != ".moonwell/lua/held/hollow" ||
		diagErr.Hint != removalHint || diagErr.Cause == nil || written != nil {
		t.Errorf("RefreshLibraryView = %q, %+v", written, diagErr)
	}
	if got := viewIn(t, root); len(got) != 0 {
		t.Errorf("a view was written before the folder was cleared: %q", got)
	}
}
