package script

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

// collect is the modules of a project that has no fault.
func collect(t *testing.T, p project) []Source {
	t.Helper()
	sources, err := Collect(p.lay(t), p.libraries())
	if err != nil {
		t.Fatal(err)
	}
	return sources
}

// refused is the failure of Collect on a project that has a fault.
func refused(t *testing.T, p project, what string) *diag.Error {
	t.Helper()
	_, err := Collect(p.lay(t), p.libraries())
	return asError(t, err, what)
}

// luaModule is the one Lua module, lua/x.lua, of a project that holds text beside its main file.
func luaModule(t *testing.T, text string) Source {
	t.Helper()
	sources := collect(t, mainOnly.and("lua/x.lua", text))
	if len(sources) != 2 || sources[1].Path != "lua/x.lua" {
		t.Fatalf("Collect = %+v, want src/main.yue and lua/x.lua", sources)
	}
	return sources[1]
}

func namesOf(sources []Source) []string {
	var names []string
	for _, source := range sources {
		names = append(names, source.Name)
	}
	return names
}

func pathsOf(sources []Source) []string {
	var paths []string
	for _, source := range sources {
		paths = append(paths, source.Path)
	}
	return paths
}

// mainOnly is a project of the one file it needs for src/ to exist.
var mainOnly = files("src/main.yue", "x = 1\n")

func TestCollectListsYueScriptInSrcAndLuaInLuaNamedByPath(t *testing.T) {
	got := collect(t, files(
		"src/main.yue", "x = 1\n",
		"src/game/units.yue", "x = 1\n",
		"src/main.lua", "-- the editor's output, ignored\n",
		"lua/tools/init.lua", "return {}\n",
		"lua/counter.lua", "Count = 0\n",
		"lua/README.md", "ignored\n",
	))
	want := []Source{
		{Name: "game.units", Path: "src/game/units.yue", Kind: Yue},
		{Name: "main", Path: "src/main.yue", Kind: Yue},
		{Name: "counter", Path: "lua/counter.lua", Kind: Lua, Text: "Count = 0\n"},
		{Name: "tools.init", Path: "lua/tools/init.lua", Kind: Lua, Text: "return {}\n"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Collect = %+v", got)
	}
}

func TestCollectReadsALuaFileSavedWithAByteOrderMarkWithoutItSoTheScanSeesItsFirstLine(t *testing.T) {
	module := luaModule(t, mark+"Counter = 0\n")
	if module.Text != "Counter = 0\n" || !slices.Equal(lua.TopLevelGlobals(module.Text), []string{"Counter"}) {
		t.Errorf("text = %q", module.Text)
	}
}

func TestCollectRefusesAModuleThatIsOrClaimsABuiltInModulesName(t *testing.T) {
	for _, path := range []string{"lua/moonwell.lua", "lua/moonwell/init.lua", "src/moonwell.yue"} {
		failure := refused(t, mainOnly.and(path, ""), path)
		if failure.Msg != "Module moonwell is built into Moonwell; rename "+path+"." || failure.File != path ||
			failure.Hint != "`require` of a built-in name always loads the built-in module, never a project file." {
			t.Errorf("%s: %+v", path, failure)
		}
	}
	if got := namesOf(collect(t, mainOnly.and("lua/moonwell/extra.lua", ""))); !slices.Equal(got, []string{"main", "moonwell.extra"}) {
		t.Errorf("names = %q", got)
	}
}

func TestCollectWorksWithoutLuaAndRequiresSrc(t *testing.T) {
	if got := collect(t, mainOnly); len(got) != 1 || got[0].Path != "src/main.yue" {
		t.Errorf("Collect = %+v", got)
	}
	for what, p := range map[string]project{
		"no src":         files("lua/x.lua", ""),
		"a file for src": files("src", "a file, not a folder"),
		"no file at all": files(),
	} {
		root := p.lay(t)
		_, err := Collect(root, nil)
		if failure := asError(t, err, what); failure.Msg != "The src/ folder is missing." || failure.File != root {
			t.Errorf("%s: %+v", what, failure)
		}
	}
}

func TestCollectRefusesDottedNamesInEitherFolder(t *testing.T) {
	for _, path := range []string{"src/a.b.yue", "lua/x.y/z.lua", "src/.hidden.yue", "lua/a/b.c/d/e.lua"} {
		failure := refused(t, mainOnly.and(path, ""), path)
		if failure.Msg != "Module file and folder names cannot contain dots." || failure.File != path ||
			failure.Hint != "Dots separate module names in `import`; rename the file or folder." {
			t.Errorf("%s: %+v", path, failure)
		}
	}
	// A project whose only file has a dotted name fails here, before anything is compiled.
	if failure := refused(t, files("src/a.b.yue", "export x = 1\n"), "a dotted name"); !strings.Contains(failure.Msg, "dots") {
		t.Errorf("error = %+v", failure)
	}
	// A dot in the name of a file that is no module is nobody's business.
	others := mainOnly.and("src/notes.v2.txt", "", "lua/a.b/readme.md", "", "lua/x.lua.bak", "")
	if got := pathsOf(collect(t, others)); !slices.Equal(got, []string{"src/main.yue"}) {
		t.Errorf("paths = %q", got)
	}
}

func TestCollectRefusesANameTwoFilesDefineNamingBoth(t *testing.T) {
	failure := refused(t, mainOnly.and("src/tools.yue", "x = 1\n", "lua/tools.lua", ""), "two files")
	if failure.Msg != "Module tools is defined by src/tools.yue and lua/tools.lua." || failure.File != "lua/tools.lua" ||
		failure.Hint != "Rename one of them: module names are shared by src/ and lua/." {
		t.Errorf("error = %+v", failure)
	}
}

func TestCollectRefusesAnInitModuleNextToAModuleOfItsParentsName(t *testing.T) {
	for _, first := range []string{"src/tools.yue", "lua/tools.lua"} {
		failure := refused(t, mainOnly.and(first, "", "lua/tools/init.lua", ""), first)
		if failure.Msg != "Module tools is defined by "+first+" and lua/tools/init.lua." || failure.File != "lua/tools/init.lua" {
			t.Errorf("%s: %+v", first, failure)
		}
	}
}

func TestCollectLetsATopLevelInitModuleClaimOnlyItsOwnName(t *testing.T) {
	if got := namesOf(collect(t, mainOnly.and("lua/init.lua", ""))); !slices.Equal(got, []string{"main", "init"}) {
		t.Errorf("names = %q", got)
	}
}

func TestCollectTakesTheLibrariesInTheOrderGivenEachYueScriptBeforeItsLua(t *testing.T) {
	root := mainOnly.and(
		"lua/own.lua", "",
		inLibrary("a", "one.lua"), "", inLibrary("a", "two.yue"), "",
		inLibrary("b", "three.lua"), "", inLibrary("b", "four.yue"), "",
	).lay(t)
	a, b := Library{Key: "a", Dir: librariesDir + "/a"}, Library{Key: "b", Dir: librariesDir + "/b"}
	for _, c := range []struct {
		libraries []Library
		want      []string
	}{
		{[]Library{a, b}, []string{"main", "own", "two", "one", "four", "three"}},
		{[]Library{b, a}, []string{"main", "own", "four", "three", "two", "one"}},
		{[]Library{b}, []string{"main", "own", "four", "three"}},
		{nil, []string{"main", "own"}},
	} {
		sources, err := Collect(root, c.libraries)
		if err != nil || !slices.Equal(namesOf(sources), c.want) {
			t.Errorf("Collect with %+v = %q, %v, want %q", c.libraries, namesOf(sources), err, c.want)
		}
	}
}

func TestAClashWithALibraryModuleNamesBothFilesAndSuggestsNarrowingDir(t *testing.T) {
	failure := refused(t, mainOnly.with("ex").and(
		"lua/example/greet.lua", "return {}\n",
		inLibrary("ex", "example/greet.lua"), "return {}\n",
	), "a clash")
	if failure.Msg != "Module example.greet is defined by lua/example/greet.lua and .moonwell/libraries/ex/example/greet.lua." ||
		failure.File != ".moonwell/libraries/ex/example/greet.lua" ||
		failure.Hint != "Rename one of them, or narrow the library's `dir`: module names are shared by src/, lua/ and libraries." {
		t.Errorf("error = %+v", failure)
	}
}

func TestInALibraryALuaFileBesideAYueFileOfTheSameStemIsItsCompiledOutputNotAModule(t *testing.T) {
	got := collect(t, mainOnly.with("ex").and(
		inLibrary("ex", "example/loud.yue"), "x = 1\n",
		inLibrary("ex", "example/loud.lua"), "-- compiled\n",
		inLibrary("ex", "kit/init.yue"), "x = 1\n",
		inLibrary("ex", "kit/init.lua"), "-- compiled\n",
		inLibrary("ex", "example/greet.lua"), "return {}\n",
	))
	want := []Source{
		{Name: "main", Path: "src/main.yue", Kind: Yue},
		{Name: "example.loud", Path: ".moonwell/libraries/ex/example/loud.yue", Kind: Yue, Library: "ex"},
		{Name: "kit.init", Path: ".moonwell/libraries/ex/kit/init.yue", Kind: Yue, Library: "ex"},
		{Name: "example.greet", Path: ".moonwell/libraries/ex/example/greet.lua", Kind: Lua, Library: "ex", Text: "return {}\n"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Collect = %+v", got)
	}
	// In the project's own folders the two kinds lie apart, and a Lua file of a YueScript module's name is a second
	// module of that name.
	failure := refused(t, mainOnly.and("src/loud.yue", "", "lua/loud.lua", ""), "the project's own")
	if failure.Msg != "Module loud is defined by src/loud.yue and lua/loud.lua." {
		t.Errorf("error = %+v", failure)
	}
}

func TestADottedOrBuiltInNameInALibrarySuggestsNarrowingTheLibrarysDir(t *testing.T) {
	for path, message := range map[string]string{
		inLibrary("ex", "a.b.lua"):      "cannot contain dots",
		inLibrary("ex", "moonwell.lua"): "Module moonwell is built into Moonwell; .moonwell/libraries/ex/moonwell.lua takes its name.",
	} {
		failure := refused(t, mainOnly.with("ex").and(path, ""), path)
		if !strings.Contains(failure.Msg, message) || failure.File != path || strings.Contains(failure.Hint, "rename") ||
			!strings.Contains(failure.Hint, "narrow the library's `dir` in moonwell.pkl") {
			t.Errorf("%s: %+v", path, failure)
		}
	}
}

// TestAModuleFileWhoseNameIsNotUTF8IsRefused names module files that are given as strings, so it runs on every
// system, whatever names its files can have.
func TestAModuleFileWhoseNameIsNotUTF8IsRefused(t *testing.T) {
	src, lua, library := folder{dir: "src", kind: Yue}, folder{dir: "lua", kind: Lua}, folder{dir: librariesDir + "/ex", kind: Lua, library: "ex"}
	for _, c := range []struct {
		of   folder
		file string // from the folder, with "/"
	}{
		{src, "a\xffb.yue"},
		{src, "game\xe9/units.yue"}, // a folder's name, in another encoding
		{src, "a/b/\xe2\x82.yue"},   // a character that is cut short
		{lua, "x/" + halfPair + ".lua"},
		{lua, halfPair + "/init.lua"},
		{lua, eAcute + "\xc3.lua"},
	} {
		path := c.of.dir + "/" + c.file
		_, err := c.of.source(c.file)
		failure := asError(t, err, path)
		if failure.Msg != "Module file and folder names must be valid UTF-8." || failure.File != path ||
			!strings.Contains(failure.Hint, "rename the file or folder") {
			t.Errorf("%q: %+v", path, failure)
		}
	}
	// A library's file is not the project's to rename.
	_, err := library.source("kit/\xff.lua")
	failure := asError(t, err, "a library's file")
	if !strings.Contains(failure.Msg, "must be valid UTF-8") || failure.File != librariesDir+"/ex/kit/\xff.lua" ||
		strings.Contains(failure.Hint, "rename") || !strings.Contains(failure.Hint, "narrow the library's `dir` in moonwell.pkl") {
		t.Errorf("a library's file: %+v", failure)
	}
	// A name with a dot is refused for the dot, whatever its bytes.
	_, err = src.source("a.b\xff.yue")
	if failure := asError(t, err, "a dotted name"); !strings.Contains(failure.Msg, "cannot contain dots") {
		t.Errorf("a dotted name: %+v", failure)
	}
	// Every name of valid UTF-8 is a module's, whatever its characters.
	for file, name := range map[string]string{
		eAcute + ".yue": eAcute, beyond + "/" + fullWidthA + ".yue": beyond + "." + fullWidthA, replacement + ".yue": replacement,
		"a\x7fb.yue": "a\x7fb", "a b.yue": "a b", "a" + noBreakSpace + "b/c" + lineSeparator + ".yue": "a" + noBreakSpace + "b.c" + lineSeparator,
	} {
		if source, err := src.source(file); err != nil || source.Name != name || source.Path != "src/"+file {
			t.Errorf("%q: %+v, %v", file, source, err)
		}
	}
}

// TestCollectRefusesAModuleFileWhoseNameIsNotUTF8 lays files whose names are not UTF-8, and is skipped on a
// system that holds no such name.
func TestCollectRefusesAModuleFileWhoseNameIsNotUTF8(t *testing.T) {
	for _, path := range []string{"src/a" + halfPair + ".yue", "lua/" + halfPair + "/x.lua", "src/game/" + halfPair + "/units.yue"} {
		p := mainOnly.and(path, "")
		_, err := Collect(p.layAsNamed(t), p.libraries())
		failure := asError(t, err, path)
		if failure.Msg != "Module file and folder names must be valid UTF-8." || failure.File != path ||
			!strings.Contains(failure.Hint, "rename the file or folder") {
			t.Errorf("%q: %+v", path, failure)
		}
	}
	inALibrary := mainOnly.with("ex").and(inLibrary("ex", "kit/"+halfPair+".yue"), "")
	_, err := Collect(inALibrary.layAsNamed(t), inALibrary.libraries())
	if failure := asError(t, err, "a library's file"); failure.File != inLibrary("ex", "kit/"+halfPair+".yue") || !strings.Contains(failure.Hint, "narrow the library's `dir`") {
		t.Errorf("a library's file: %+v", failure)
	}
	// A file that is no module of its folder is not looked at, whatever its name.
	others := mainOnly.and("src/"+halfPair+".txt", "", "lua/"+halfPair+"/readme.md", "", "src/"+halfPair+".lua", "", "lua/x"+halfPair+".yue", "")
	sources, err := Collect(others.layAsNamed(t), others.libraries())
	if got := pathsOf(sources); err != nil || !slices.Equal(got, []string{"src/main.yue"}) {
		t.Errorf("files that are no modules: paths = %q, %v", got, err)
	}
}

func TestALibraryWithoutAFolderHasNoModules(t *testing.T) {
	for what, p := range map[string]project{
		"no folder":             mainOnly.with("ex"),
		"a file for the folder": mainOnly.with("ex").and(librariesDir+"/ex", "a file, not a folder"),
		"a file for .moonwell":  mainOnly.with("ex").and(".moonwell", "a file, not a folder"),
		"no module in it":       mainOnly.with("ex").and(inLibrary("ex", "notes.txt"), "no module"),
	} {
		if got := pathsOf(collect(t, p)); !slices.Equal(got, []string{"src/main.yue"}) {
			t.Errorf("%s: paths = %q", what, got)
		}
	}
}

func TestCollectListsTheModulesOfAFolderInByteOrder(t *testing.T) {
	// By bytes a character from U+E000 to U+FFFF comes before one beyond the basic plane; by UTF-16 units it comes
	// after.
	got := pathsOf(collect(t, files(
		"src/"+beyond+".yue", "", "src/"+fullWidthA+".yue", "", "src/z.yue", "", "src/B.yue", "", "src/a/b.yue", "",
		"src/a-b.yue", "", "src/a.yue", "", "src/a0.yue", "", "src/"+eAcute+".yue", "",
		"lua/"+beyond+"/x.lua", "", "lua/"+fullWidthA+"/x.lua", "",
	)))
	want := []string{
		"src/B.yue", "src/a-b.yue", "src/a.yue", "src/a/b.yue", "src/a0.yue", "src/z.yue", "src/" + eAcute + ".yue",
		"src/" + fullWidthA + ".yue", "src/" + beyond + ".yue",
		"lua/" + fullWidthA + "/x.lua", "lua/" + beyond + "/x.lua",
	}
	if !slices.Equal(got, want) {
		t.Errorf("paths = %q, want %q", got, want)
	}
}

func TestALuaModulesTextIsTheBytesOfItsFile(t *testing.T) {
	for what, written := range map[string]string{
		"a byte that is not UTF-8":       "a = 1 \xff\n",
		"a run of such bytes":            "a = 1 \xff\xfe\xc0\xc1\n",
		"a character that is cut short":  "a = '\xe2\x80' .. '\xf0\x9f\x98'\n",
		"such a byte inside a string":    "a = 'caf\xe9' .. \"\xe5\xe4\xf6\"\n",
		"such a byte inside a comment":   "-- caf\xe9\nreturn 1 --[[ \xff\xff ]]\n",
		"a written replacement beside":   "a = '" + replacement + "\xff" + replacement + "'\n",
		"characters outside ASCII":       "a = '" + eAcute + beyond + "'\n",
		"a byte order mark further in":   "a = '" + mark + "'\n",
		"line ends of every kind":        "a = 1\r\nb = 2\rc = 3\n\n",
		"a byte of zero and a form feed": "a = '\x00'\f\n",
	} {
		if module := luaModule(t, written); module.Text != written {
			t.Errorf("%s: the text of %q is %q", what, written, module.Text)
		}
	}
	// A library's Lua module is read the same way.
	sources := collect(t, mainOnly.with("ex").and(inLibrary("ex", "x.lua"), "return '\xff\xfe'\n"))
	if len(sources) != 2 || sources[1].Text != "return '\xff\xfe'\n" {
		t.Errorf("Collect = %+v", sources)
	}
}

func TestInsideAFolderOfModulesALinkToAFileIsReadAndALinkToAFolderIsNotEntered(t *testing.T) {
	root := mainOnly.and("lua/own.lua", "return 1\n").lay(t)
	linkTo(t, files("deep.lua", "return 2\n", "more/deeper.lua", "return 3\n"), root, "lua/linked")
	linkTo(t, files("other.yue", "x = 2\n"), root, "src/linked")
	sources, err := Collect(root, nil)
	if err != nil || !slices.Equal(pathsOf(sources), []string{"src/main.yue", "lua/own.lua"}) {
		t.Errorf("with links to folders: Collect = %q, %v, want the modules that are not behind them", pathsOf(sources), err)
	}

	elsewhere := files("real.lua", "return 'through the link'\n", "real.yue", "x = 3\n").lay(t)
	for link, target := range map[string]string{"lua/through.lua": "real.lua", "src/also.yue": "real.yue"} {
		// Skipped where Windows keeps the right to make such a link from this account, and for that alone.
		testkit.LinkFile(t, filepath.Join(elsewhere, target), filepath.Join(root, filepath.FromSlash(link)))
	}
	want := []Source{
		{Name: "also", Path: "src/also.yue", Kind: Yue},
		{Name: "main", Path: "src/main.yue", Kind: Yue},
		{Name: "own", Path: "lua/own.lua", Kind: Lua, Text: "return 1\n"},
		{Name: "through", Path: "lua/through.lua", Kind: Lua, Text: "return 'through the link'\n"},
	}
	if sources, err = Collect(root, nil); err != nil || !reflect.DeepEqual(sources, want) {
		t.Errorf("with links to files: Collect = %+v, %v, want %+v", sources, err, want)
	}
}

func TestALinkAtAFolderOfModulesIsRefused(t *testing.T) {
	for _, link := range []string{"src", "lua", ".moonwell", librariesDir, librariesDir + "/ex"} {
		p := mainOnly.with("ex")
		if link == "src" {
			p = files().with("ex")
		}
		root := p.lay(t)
		at := linkTo(t, files("main.yue", "x = 1\n", "libraries/ex/x.lua", "", "ex/x.lua", ""), root, link)
		_, err := Collect(root, p.libraries())
		failure := asError(t, err, link)
		if failure.Msg != "Symlinks are not supported: "+at || !strings.Contains(failure.Hint, "real files") {
			t.Errorf("a link at %s: %+v", link, failure)
		}
	}
}

func TestALibraryThatNamesNoFolderOfTheProjectIsAMistakeOfTheCaller(t *testing.T) {
	root := mainOnly.lay(t)
	for _, library := range []Library{
		{Key: "", Dir: librariesDir + "/ex"}, {Key: "ex", Dir: ""}, {Key: "ex", Dir: "../ex"}, {Key: "ex", Dir: "/ex"},
		{Key: "ex", Dir: "a//b"}, {Key: "ex", Dir: "ex/"}, {Key: "ex", Dir: "."},
	} {
		_, err := Collect(root, []Library{library})
		var expected *diag.Error
		if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "script.Collect") {
			t.Errorf("Collect with %+v = %v, want an error that is no user's mistake", library, err)
		}
	}
	// A folder written with backslashes is the same folder.
	p := mainOnly.with("ex").and(inLibrary("ex", "x.lua"), "")
	sources, err := Collect(p.lay(t), []Library{{Key: "ex", Dir: `.moonwell\libraries\ex`}})
	if err != nil || !slices.Equal(pathsOf(sources), []string{"src/main.yue", ".moonwell/libraries/ex/x.lua"}) {
		t.Errorf("Collect = %q, %v", pathsOf(sources), err)
	}
}

func TestALuaModuleThatCannotBeReadIsRefusedByItsPath(t *testing.T) {
	root := mainOnly.and("lua/held.lua", "return {}\n").lay(t)
	testkit.MakeUnreadable(t, filepath.Join(root, "lua", "held.lua"))
	_, err := Collect(root, nil)
	failure := asError(t, err, "a held file")
	if !strings.HasPrefix(failure.Msg, "Reading lua/held.lua failed: ") || failure.File != "lua/held.lua" || failure.Hint == "" {
		t.Errorf("error = %+v", failure)
	}
}

func TestAFolderOfModulesThatCannotBeListedIsRefusedByItsName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows lists a folder whatever its permissions")
	}
	if os.Geteuid() == 0 {
		t.Skip("root may list a folder without permissions, so the listing would not fail")
	}
	root := mainOnly.and("lua/closed/x.lua", "").lay(t)
	closed := filepath.Join(root, "lua", "closed")
	if err := os.Chmod(closed, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(closed, 0o777) })
	_, err := Collect(root, nil)
	failure := asError(t, err, "a closed folder")
	if !strings.HasPrefix(failure.Msg, "Reading lua/closed/ failed: ") || failure.File != "lua/closed" ||
		!strings.Contains(failure.Hint, "can be read") {
		t.Errorf("error = %+v", failure)
	}
}

func TestAListingThatFailsBelowAFolderNamesTheFolderItFailedAt(t *testing.T) {
	lua := folder{dir: "lua", path: filepath.Join(t.TempDir(), "lua"), kind: Lua}
	denied := func(path string) error { return &fs.PathError{Op: "open", Path: path, Err: fs.ErrPermission} }
	for what, c := range map[string]struct {
		cause error
		want  string
	}{
		"below the folder":    {denied(filepath.Join(lua.path, "a", "closed")), "lua/a/closed"},
		"wrapped once more":   {fmt.Errorf("listing: %w", denied(filepath.Join(lua.path, "closed"))), "lua/closed"},
		"at the folder":       {denied(lua.path), "lua"},
		"outside the folder":  {denied(filepath.Dir(lua.path)), "lua"},
		"beside the folder":   {denied(lua.path + "x"), "lua"},
		"without a path":      {errors.New("the disk is gone"), "lua"},
		"with a path of none": {denied(""), "lua"},
	} {
		failure := asError(t, errUnreadableFolder(lua.failedAt(c.cause), c.cause), what)
		if !strings.HasPrefix(failure.Msg, "Reading "+c.want+"/ failed: ") || failure.File != c.want || failure.Cause != c.cause {
			t.Errorf("%s: %+v, want the folder %s", what, failure, c.want)
		}
	}
}

// ofLibraries is the modules among sources that are a library's.
func ofLibraries(sources []Source) []Source {
	var listed []Source
	for _, source := range sources {
		if source.Library != "" {
			listed = append(listed, source)
		}
	}
	return listed
}

func TestCollectLibrariesListsTheLibrariesModulesAsCollectListsThem(t *testing.T) {
	root := mainOnly.and(
		"lua/own.lua", "return 'own'\n",
		inLibrary("a", "one.lua"), "return 1\n", inLibrary("a", "two.yue"), "x = 2\n",
		inLibrary("a", "kit/init.yue"), "x = 3\n", inLibrary("a", "kit/init.lua"), "-- compiled\n",
		inLibrary("b", "three.lua"), mark+"return 3\n", inLibrary("b", "four.yue"), "x = 4\n",
		inLibrary("b", "notes.txt"), "no module\n",
	).lay(t)
	a, b := Library{Key: "a", Dir: librariesDir + "/a"}, Library{Key: "b", Dir: librariesDir + "/b"}
	missing := Library{Key: "none", Dir: librariesDir + "/none"}
	for _, c := range []struct {
		libraries []Library
		want      []string
	}{
		{[]Library{a, b}, []string{"kit.init", "two", "one", "four", "three"}},
		{[]Library{b, a}, []string{"four", "three", "kit.init", "two", "one"}},
		{[]Library{b, missing}, []string{"four", "three"}},
		{[]Library{missing}, nil},
		{nil, nil},
	} {
		whole, err := Collect(root, c.libraries)
		if err != nil {
			t.Fatal(err)
		}
		alone, err := CollectLibraries(root, c.libraries)
		if err != nil || !slices.Equal(namesOf(alone), c.want) || !reflect.DeepEqual(alone, ofLibraries(whole)) {
			t.Errorf("CollectLibraries with %+v = %+v, %v, want the modules %q, as Collect lists them: %+v",
				c.libraries, alone, err, c.want, ofLibraries(whole))
		}
	}
}

func TestCollectLibrariesDoesNotLookAtTheProjectsOwnModules(t *testing.T) {
	greet := inLibrary("ex", "example/greet.lua")
	library := files(greet, "return {}\n").with("ex")
	withSrc := library.and("src/main.yue", "x = 1\n")
	const clash = "Module example.greet is defined by"
	cases := []struct {
		name    string
		project project
		refusal string // what Collect says of the project
	}{
		{"a module of src/ with the name", library.and("src/example/greet.yue", "x = 1\n"), clash},
		{"a module of lua/ with the name", withSrc.and("lua/example/greet.lua", ""), clash},
		{"a module of src/ that claims the name", library.and("src/example/greet/init.yue", ""), clash},
		{"no src/", library, "The src/ folder is missing."},
		{"a file for src/", library.and("src", "a file, not a folder"), "The src/ folder is missing."},
		{"a dotted name in src/", library.and("src/a.b.yue", ""), "cannot contain dots"},
		{"a built-in module's name in lua/", withSrc.and("lua/moonwell.lua", ""), "is built into"},
	}
	want := []Source{{Name: "example.greet", Path: greet, Kind: Lua, Library: "ex", Text: "return {}\n"}}
	for _, c := range cases {
		root := c.project.lay(t)
		_, err := Collect(root, c.project.libraries())
		if failure := asError(t, err, c.name); !strings.Contains(failure.Msg, c.refusal) {
			t.Errorf("%s: Collect refuses with %+v, want %q", c.name, failure, c.refusal)
		}
		if got, err := CollectLibraries(root, c.project.libraries()); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: CollectLibraries = %+v, %v, want %+v", c.name, got, err, want)
		}
	}
}

// A link at src/ or at lua/ is the project's own: Collect refuses it, and CollectLibraries does not go there.
func TestCollectLibrariesDoesNotGoThroughALinkAtAFolderOfTheProjectsOwn(t *testing.T) {
	for _, link := range []string{"src", "lua"} {
		p := files(inLibrary("ex", "x.lua"), "").with("ex")
		if link == "lua" {
			p = p.and("src/main.yue", "")
		}
		root := p.lay(t)
		at := linkTo(t, files("main.yue", "", "x.lua", ""), root, link)
		_, err := Collect(root, p.libraries())
		if failure := asError(t, err, link); failure.Msg != "Symlinks are not supported: "+at {
			t.Errorf("a link at %s: Collect refuses with %+v", link, failure)
		}
		got, err := CollectLibraries(root, p.libraries())
		if err != nil || !slices.Equal(pathsOf(got), []string{inLibrary("ex", "x.lua")}) {
			t.Errorf("a link at %s: CollectLibraries = %q, %v", link, pathsOf(got), err)
		}
	}
}

func TestCollectLibrariesRefusesWhatCollectRefusesOfALibrary(t *testing.T) {
	one, two := mainOnly.with("ex"), mainOnly.with("b", "a")
	for what, p := range map[string]project{
		"a dotted name":              one.and(inLibrary("ex", "a.b.lua"), ""),
		"a dotted folder":            one.and(inLibrary("ex", "a.b/c.yue"), ""),
		"a built-in module's name":   one.and(inLibrary("ex", "moonwell.lua"), ""),
		"a name of two libraries":    two.and(inLibrary("a", "kit.lua"), "", inLibrary("b", "kit.yue"), ""),
		"a name that an init claims": two.and(inLibrary("a", "kit/init.yue"), "", inLibrary("b", "kit.lua"), ""),
	} {
		root := p.lay(t)
		_, whole := Collect(root, p.libraries())
		got, alone := CollectLibraries(root, p.libraries())
		want := asError(t, whole, what)
		if failure := asError(t, alone, what); got != nil || !reflect.DeepEqual(failure, want) || want.File == "" {
			t.Errorf("%s: CollectLibraries = %+v, %+v, want the refusal of Collect: %+v", what, got, failure, want)
		}
	}
}

func TestCollectLibrariesRefusesALinkAtAFolderOfALibrary(t *testing.T) {
	for _, link := range []string{".moonwell", librariesDir, librariesDir + "/ex"} {
		p := mainOnly.with("ex")
		root := p.lay(t)
		at := linkTo(t, files("libraries/ex/x.lua", "", "ex/x.lua", "", "x.lua", ""), root, link)
		got, err := CollectLibraries(root, p.libraries())
		failure := asError(t, err, link)
		if got != nil || failure.Msg != "Symlinks are not supported: "+at ||
			!strings.Contains(failure.Hint, "real files") {
			t.Errorf("a link at %s: CollectLibraries = %+v, %+v", link, got, failure)
		}
	}
}

func TestCollectLibrariesTakesALibraryThatNamesNoFolderOfTheProjectForAMistakeOfTheCaller(t *testing.T) {
	root := files().lay(t)
	for _, library := range []Library{
		{Key: "", Dir: librariesDir + "/ex"}, {Key: "ex", Dir: ""}, {Key: "ex", Dir: "../ex"}, {Key: "ex", Dir: "/ex"},
	} {
		_, err := CollectLibraries(root, []Library{library})
		var expected *diag.Error
		if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "script.CollectLibraries: ") {
			t.Errorf("CollectLibraries with %+v = %v, want an error that is no user's mistake", library, err)
		}
	}
}

func TestEntryNameIsTheDottedNameOfAYueScriptFileUnderSrc(t *testing.T) {
	for entry, want := range map[string]string{
		"src/main.yue":         "main",
		"src/game/init.yue":    "game.init",
		`src\game\init.yue`:    "game.init",
		"./src/main.yue":       "main",
		`.\src\game\units.yue`: "game.units",
	} {
		if got, err := EntryName(entry); err != nil || got != want {
			t.Errorf("EntryName(%q) = %q, %v, want %q", entry, got, err, want)
		}
	}
	for _, entry := range []string{
		"", "main.yue", "src", "src/main.lua", "src/main", "lua/main.yue", "SRC/main.yue", "src/main.YUE", "/src/main.yue",
		"././src/main.yue", "game/src/main.yue",
	} {
		name, err := EntryName(entry)
		failure := asError(t, err, entry)
		if name != "" || failure.Msg != "Entry '"+entry+"' must be a .yue file under src/." || failure.File != "" ||
			failure.Hint != "For example: src/main.yue" {
			t.Errorf("EntryName(%q) = %q, %+v", entry, name, failure)
		}
	}
}
