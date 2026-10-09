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

func mustCollect(t *testing.T, p sourceTree) []Source {
	t.Helper()
	sources, err := CollectSources(p.writeToTempDir(t), p.libraries())
	if err != nil {
		t.Fatal(err)
	}
	return sources
}

func mustFailCollect(t *testing.T, p sourceTree, what string) *diag.Error {
	t.Helper()
	_, err := CollectSources(p.writeToTempDir(t), p.libraries())
	return asDiagError(t, err, what)
}

func luaModule(t *testing.T, text string) Source {
	t.Helper()
	sources := mustCollect(t, mainOnly.withFiles("lua/x.lua", text))
	if len(sources) != 2 || sources[1].Path != "lua/x.lua" {
		t.Fatalf("Collect = %+v, want src/main.yue and lua/x.lua", sources)
	}
	return sources[1]
}

func sourceNames(sources []Source) []string {
	var names []string
	for _, source := range sources {
		names = append(names, source.Name)
	}
	return names
}

func sourcePaths(sources []Source) []string {
	var paths []string
	for _, source := range sources {
		paths = append(paths, source.Path)
	}
	return paths
}

var mainOnly = newSourceTree("src/main.yue", "x = 1\n")

func TestCollectListsYueScriptInSrcAndLuaInLuaNamedByPath(t *testing.T) {
	got := mustCollect(t, newSourceTree(
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
	if module.Text != "Counter = 0\n" || !slices.Equal(lua.FindTopLevelGlobals(module.Text), []string{"Counter"}) {
		t.Errorf("text = %q", module.Text)
	}
}

func TestCollectRefusesAModuleThatIsOrClaimsABuiltInModulesName(t *testing.T) {
	for _, path := range []string{"lua/moonwell.lua", "lua/moonwell/init.lua", "src/moonwell.yue"} {
		diagErr := mustFailCollect(t, mainOnly.withFiles(path, ""), path)
		if diagErr.Msg != "Module moonwell is built into Moonwell; rename "+path+"." || diagErr.File != path ||
			diagErr.Hint != "`require` of a built-in name always loads the built-in module, never a project file." {
			t.Errorf("%s: %+v", path, diagErr)
		}
	}
	if got := sourceNames(mustCollect(t, mainOnly.withFiles("lua/moonwell/extra.lua", ""))); !slices.Equal(got, []string{"main", "moonwell.extra"}) {
		t.Errorf("names = %q", got)
	}
}

func TestCollectWorksWithoutLuaAndRequiresSrc(t *testing.T) {
	if got := mustCollect(t, mainOnly); len(got) != 1 || got[0].Path != "src/main.yue" {
		t.Errorf("Collect = %+v", got)
	}
	for what, p := range map[string]sourceTree{
		"no src":         newSourceTree("lua/x.lua", ""),
		"a file for src": newSourceTree("src", "a file, not a folder"),
		"no file at all": newSourceTree(),
	} {
		root := p.writeToTempDir(t)
		_, err := CollectSources(root, nil)
		if diagErr := asDiagError(t, err, what); diagErr.Msg != "The src/ folder is missing." || diagErr.File != root {
			t.Errorf("%s: %+v", what, diagErr)
		}
	}
}

func TestCollectRefusesDottedNamesInEitherFolder(t *testing.T) {
	for _, path := range []string{"src/a.b.yue", "lua/x.y/z.lua", "src/.hidden.yue", "lua/a/b.c/d/e.lua"} {
		diagErr := mustFailCollect(t, mainOnly.withFiles(path, ""), path)
		if diagErr.Msg != "Module file and folder names cannot contain dots." || diagErr.File != path ||
			diagErr.Hint != "Dots separate module names in `import`; rename the file or folder." {
			t.Errorf("%s: %+v", path, diagErr)
		}
	}
	if diagErr := mustFailCollect(t, newSourceTree("src/a.b.yue", "export x = 1\n"), "a dotted name"); !strings.Contains(diagErr.Msg, "dots") {
		t.Errorf("error = %+v", diagErr)
	}
	others := mainOnly.withFiles("src/notes.v2.txt", "", "lua/a.b/readme.md", "", "lua/x.lua.bak", "")
	if got := sourcePaths(mustCollect(t, others)); !slices.Equal(got, []string{"src/main.yue"}) {
		t.Errorf("paths = %q", got)
	}
}

func TestCollectRefusesANameTwoFilesDefineNamingBoth(t *testing.T) {
	diagErr := mustFailCollect(t, mainOnly.withFiles("src/tools.yue", "x = 1\n", "lua/tools.lua", ""), "two files")
	if diagErr.Msg != "Module tools is defined by src/tools.yue and lua/tools.lua." || diagErr.File != "lua/tools.lua" ||
		diagErr.Hint != "Rename one of them: module names are shared by src/ and lua/." {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestCollectRefusesAnInitModuleNextToAModuleOfItsParentsName(t *testing.T) {
	for _, first := range []string{"src/tools.yue", "lua/tools.lua"} {
		diagErr := mustFailCollect(t, mainOnly.withFiles(first, "", "lua/tools/init.lua", ""), first)
		if diagErr.Msg != "Module tools is defined by "+first+" and lua/tools/init.lua." || diagErr.File != "lua/tools/init.lua" {
			t.Errorf("%s: %+v", first, diagErr)
		}
	}
}

func TestCollectLetsATopLevelInitModuleClaimOnlyItsOwnName(t *testing.T) {
	if got := sourceNames(mustCollect(t, mainOnly.withFiles("lua/init.lua", ""))); !slices.Equal(got, []string{"main", "init"}) {
		t.Errorf("names = %q", got)
	}
}

func TestCollectTakesTheLibrariesInTheOrderGivenEachYueScriptBeforeItsLua(t *testing.T) {
	root := mainOnly.withFiles(
		"lua/own.lua", "",
		inLibrary("a", "one.lua"), "", inLibrary("a", "two.yue"), "",
		inLibrary("b", "three.lua"), "", inLibrary("b", "four.yue"), "",
	).writeToTempDir(t)
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
		sources, err := CollectSources(root, c.libraries)
		if err != nil || !slices.Equal(sourceNames(sources), c.want) {
			t.Errorf("Collect with %+v = %q, %v, want %q", c.libraries, sourceNames(sources), err, c.want)
		}
	}
}

func TestAClashWithALibraryModuleNamesBothFilesAndSuggestsNarrowingDir(t *testing.T) {
	diagErr := mustFailCollect(t, mainOnly.withLibraries("ex").withFiles(
		"lua/example/greet.lua", "return {}\n",
		inLibrary("ex", "example/greet.lua"), "return {}\n",
	), "a clash")
	if diagErr.Msg != "Module example.greet is defined by lua/example/greet.lua and .moonwell/libraries/ex/example/greet.lua." ||
		diagErr.File != ".moonwell/libraries/ex/example/greet.lua" ||
		diagErr.Hint != "Rename one of them, or narrow the library's `dir`: module names are shared by src/, lua/ and libraries." {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestInALibraryALuaFileBesideAYueFileOfTheSameStemIsItsCompiledOutputNotAModule(t *testing.T) {
	got := mustCollect(t, mainOnly.withLibraries("ex").withFiles(
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
	diagErr := mustFailCollect(t, mainOnly.withFiles("src/loud.yue", "", "lua/loud.lua", ""), "the project's own")
	if diagErr.Msg != "Module loud is defined by src/loud.yue and lua/loud.lua." {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestADottedOrBuiltInNameInALibrarySuggestsNarrowingTheLibrarysDir(t *testing.T) {
	for path, message := range map[string]string{
		inLibrary("ex", "a.b.lua"):      "cannot contain dots",
		inLibrary("ex", "moonwell.lua"): "Module moonwell is built into Moonwell; .moonwell/libraries/ex/moonwell.lua takes its name.",
	} {
		diagErr := mustFailCollect(t, mainOnly.withLibraries("ex").withFiles(path, ""), path)
		if !strings.Contains(diagErr.Msg, message) || diagErr.File != path || strings.Contains(diagErr.Hint, "rename") ||
			!strings.Contains(diagErr.Hint, "narrow the library's `dir` in moonwell.toml") {
			t.Errorf("%s: %+v", path, diagErr)
		}
	}
}

func TestAModuleFileWhoseNameIsNotUTF8IsRefused(t *testing.T) {
	src, lua, library := moduleDir{dir: "src", kind: Yue}, moduleDir{dir: "lua", kind: Lua}, moduleDir{dir: librariesDir + "/ex", kind: Lua, library: "ex"}
	for _, c := range []struct {
		of   moduleDir
		file string
	}{
		{src, "a\xffb.yue"},
		{src, "game\xe9/units.yue"},
		{src, "a/b/\xe2\x82.yue"},
		{lua, "x/" + halfPair + ".lua"},
		{lua, halfPair + "/init.lua"},
		{lua, eAcute + "\xc3.lua"},
	} {
		path := c.of.dir + "/" + c.file
		_, err := c.of.readSource(c.file)
		diagErr := asDiagError(t, err, path)
		if diagErr.Msg != "Module file and folder names must be valid UTF-8." || diagErr.File != path ||
			!strings.Contains(diagErr.Hint, "rename the file or folder") {
			t.Errorf("%q: %+v", path, diagErr)
		}
	}
	_, err := library.readSource("kit/\xff.lua")
	diagErr := asDiagError(t, err, "a library's file")
	if !strings.Contains(diagErr.Msg, "must be valid UTF-8") || diagErr.File != librariesDir+"/ex/kit/\xff.lua" ||
		strings.Contains(diagErr.Hint, "rename") || !strings.Contains(diagErr.Hint, "narrow the library's `dir` in moonwell.toml") {
		t.Errorf("a library's file: %+v", diagErr)
	}
	_, err = src.readSource("a.b\xff.yue")
	if diagErr := asDiagError(t, err, "a dotted name"); !strings.Contains(diagErr.Msg, "cannot contain dots") {
		t.Errorf("a dotted name: %+v", diagErr)
	}
	for file, name := range map[string]string{
		eAcute + ".yue": eAcute, beyond + "/" + fullWidthA + ".yue": beyond + "." + fullWidthA, replacement + ".yue": replacement,
		"a\x7fb.yue": "a\x7fb", "a b.yue": "a b", "a" + noBreakSpace + "b/c" + lineSeparator + ".yue": "a" + noBreakSpace + "b.c" + lineSeparator,
	} {
		if source, err := src.readSource(file); err != nil || source.Name != name || source.Path != "src/"+file {
			t.Errorf("%q: %+v, %v", file, source, err)
		}
	}
}

func TestCollectRefusesAModuleFileWhoseNameIsNotUTF8(t *testing.T) {
	for _, path := range []string{"src/a" + halfPair + ".yue", "lua/" + halfPair + "/x.lua", "src/game/" + halfPair + "/units.yue"} {
		p := mainOnly.withFiles(path, "")
		_, err := CollectSources(p.writeToTempDirOrSkip(t), p.libraries())
		diagErr := asDiagError(t, err, path)
		if diagErr.Msg != "Module file and folder names must be valid UTF-8." || diagErr.File != path ||
			!strings.Contains(diagErr.Hint, "rename the file or folder") {
			t.Errorf("%q: %+v", path, diagErr)
		}
	}
	inALibrary := mainOnly.withLibraries("ex").withFiles(inLibrary("ex", "kit/"+halfPair+".yue"), "")
	_, err := CollectSources(inALibrary.writeToTempDirOrSkip(t), inALibrary.libraries())
	if diagErr := asDiagError(t, err, "a library's file"); diagErr.File != inLibrary("ex", "kit/"+halfPair+".yue") || !strings.Contains(diagErr.Hint, "narrow the library's `dir`") {
		t.Errorf("a library's file: %+v", diagErr)
	}
	others := mainOnly.withFiles("src/"+halfPair+".txt", "", "lua/"+halfPair+"/readme.md", "", "src/"+halfPair+".lua", "", "lua/x"+halfPair+".yue", "")
	sources, err := CollectSources(others.writeToTempDirOrSkip(t), others.libraries())
	if got := sourcePaths(sources); err != nil || !slices.Equal(got, []string{"src/main.yue"}) {
		t.Errorf("files that are no modules: paths = %q, %v", got, err)
	}
}

func TestALibraryWithoutAFolderHasNoModules(t *testing.T) {
	for what, p := range map[string]sourceTree{
		"no folder":             mainOnly.withLibraries("ex"),
		"a file for the folder": mainOnly.withLibraries("ex").withFiles(librariesDir+"/ex", "a file, not a folder"),
		"a file for .moonwell":  mainOnly.withLibraries("ex").withFiles(".moonwell", "a file, not a folder"),
		"no module in it":       mainOnly.withLibraries("ex").withFiles(inLibrary("ex", "notes.txt"), "no module"),
	} {
		if got := sourcePaths(mustCollect(t, p)); !slices.Equal(got, []string{"src/main.yue"}) {
			t.Errorf("%s: paths = %q", what, got)
		}
	}
}

func TestCollectListsTheModulesOfAFolderInByteOrder(t *testing.T) {
	got := sourcePaths(mustCollect(t, newSourceTree(
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
	sources := mustCollect(t, mainOnly.withLibraries("ex").withFiles(inLibrary("ex", "x.lua"), "return '\xff\xfe'\n"))
	if len(sources) != 2 || sources[1].Text != "return '\xff\xfe'\n" {
		t.Errorf("Collect = %+v", sources)
	}
}

func TestInsideAFolderOfModulesALinkToAFileIsReadAndALinkToAFolderIsNotEntered(t *testing.T) {
	root := mainOnly.withFiles("lua/own.lua", "return 1\n").writeToTempDir(t)
	symlinkTree(t, newSourceTree("deep.lua", "return 2\n", "more/deeper.lua", "return 3\n"), root, "lua/linked")
	symlinkTree(t, newSourceTree("other.yue", "x = 2\n"), root, "src/linked")
	sources, err := CollectSources(root, nil)
	if err != nil || !slices.Equal(sourcePaths(sources), []string{"src/main.yue", "lua/own.lua"}) {
		t.Errorf("with links to folders: Collect = %q, %v, want the modules that are not behind them", sourcePaths(sources), err)
	}

	elsewhere := newSourceTree("real.lua", "return 'through the link'\n", "real.yue", "x = 3\n").writeToTempDir(t)
	for symlink, target := range map[string]string{"lua/through.lua": "real.lua", "src/also.yue": "real.yue"} {
		testkit.LinkFile(t, filepath.Join(elsewhere, target), filepath.Join(root, filepath.FromSlash(symlink)))
	}
	want := []Source{
		{Name: "also", Path: "src/also.yue", Kind: Yue},
		{Name: "main", Path: "src/main.yue", Kind: Yue},
		{Name: "own", Path: "lua/own.lua", Kind: Lua, Text: "return 1\n"},
		{Name: "through", Path: "lua/through.lua", Kind: Lua, Text: "return 'through the link'\n"},
	}
	if sources, err = CollectSources(root, nil); err != nil || !reflect.DeepEqual(sources, want) {
		t.Errorf("with links to files: Collect = %+v, %v, want %+v", sources, err, want)
	}
}

func TestALinkAtAFolderOfModulesIsRefused(t *testing.T) {
	for _, symlink := range []string{"src", "lua", ".moonwell", librariesDir, librariesDir + "/ex"} {
		p := mainOnly.withLibraries("ex")
		if symlink == "src" {
			p = newSourceTree().withLibraries("ex")
		}
		root := p.writeToTempDir(t)
		at := symlinkTree(t, newSourceTree("main.yue", "x = 1\n", "libraries/ex/x.lua", "", "ex/x.lua", ""), root, symlink)
		_, err := CollectSources(root, p.libraries())
		diagErr := asDiagError(t, err, symlink)
		if diagErr.Msg != "Symlinks are not supported: "+at || !strings.Contains(diagErr.Hint, "real files") {
			t.Errorf("a link at %s: %+v", symlink, diagErr)
		}
		if !slices.Contains([]string{"src", "lua", librariesDir + "/ex"}, diagErr.File) ||
			!strings.HasPrefix(diagErr.File, symlink) {
			t.Errorf("a link at %s: the refusal names the file %q", symlink, diagErr.File)
		}
	}
}

func TestALibraryThatNamesNoFolderOfTheProjectIsAMistakeOfTheCaller(t *testing.T) {
	root := mainOnly.writeToTempDir(t)
	for _, library := range []Library{
		{Key: "", Dir: librariesDir + "/ex"}, {Key: "ex", Dir: ""}, {Key: "ex", Dir: "../ex"}, {Key: "ex", Dir: "/ex"},
		{Key: "ex", Dir: "a//b"}, {Key: "ex", Dir: "ex/"}, {Key: "ex", Dir: "."},
	} {
		_, err := CollectSources(root, []Library{library})
		var expected *diag.Error
		if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "script.Collect") {
			t.Errorf("Collect with %+v = %v, want an error that is no user's mistake", library, err)
		}
	}
	p := mainOnly.withLibraries("ex").withFiles(inLibrary("ex", "x.lua"), "")
	sources, err := CollectSources(p.writeToTempDir(t), []Library{{Key: "ex", Dir: `.moonwell\libraries\ex`}})
	if err != nil || !slices.Equal(sourcePaths(sources), []string{"src/main.yue", ".moonwell/libraries/ex/x.lua"}) {
		t.Errorf("Collect = %q, %v", sourcePaths(sources), err)
	}
}

func TestALuaModuleThatCannotBeReadIsRefusedByItsPath(t *testing.T) {
	root := mainOnly.withFiles("lua/held.lua", "return {}\n").writeToTempDir(t)
	testkit.MakeUnreadable(t, filepath.Join(root, "lua", "held.lua"))
	_, err := CollectSources(root, nil)
	diagErr := asDiagError(t, err, "a held file")
	if !strings.HasPrefix(diagErr.Msg, "Reading lua/held.lua failed: ") || diagErr.File != "lua/held.lua" || diagErr.Hint == "" {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestAFolderOfModulesThatCannotBeListedIsRefusedByItsName(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows lists a folder whatever its permissions")
	}
	if os.Geteuid() == 0 {
		t.Skip("root may list a folder without permissions, so the listing would not fail")
	}
	root := mainOnly.withFiles("lua/closed/x.lua", "").writeToTempDir(t)
	closed := filepath.Join(root, "lua", "closed")
	if err := os.Chmod(closed, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(closed, 0o777) })
	_, err := CollectSources(root, nil)
	diagErr := asDiagError(t, err, "a closed folder")
	if !strings.HasPrefix(diagErr.Msg, "Reading lua/closed/ failed: ") || diagErr.File != "lua/closed" ||
		!strings.Contains(diagErr.Hint, "can be read") {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestAListingThatFailsBelowAFolderNamesTheFolderItFailedAt(t *testing.T) {
	lua := moduleDir{dir: "lua", fullPath: filepath.Join(t.TempDir(), "lua"), kind: Lua}
	denied := func(path string) error { return &fs.PathError{Op: "open", Path: path, Err: fs.ErrPermission} }
	for what, c := range map[string]struct {
		cause error
		want  string
	}{
		"below the folder":    {denied(filepath.Join(lua.fullPath, "a", "closed")), "lua/a/closed"},
		"wrapped once more":   {fmt.Errorf("listing: %w", denied(filepath.Join(lua.fullPath, "closed"))), "lua/closed"},
		"at the folder":       {denied(lua.fullPath), "lua"},
		"outside the folder":  {denied(filepath.Dir(lua.fullPath)), "lua"},
		"beside the folder":   {denied(lua.fullPath + "x"), "lua"},
		"without a path":      {errors.New("the disk is gone"), "lua"},
		"with a path of none": {denied(""), "lua"},
	} {
		diagErr := asDiagError(t, errUnreadableFolder(lua.failedPath(c.cause), c.cause), what)
		if !strings.HasPrefix(diagErr.Msg, "Reading "+c.want+"/ failed: ") || diagErr.File != c.want || diagErr.Cause != c.cause {
			t.Errorf("%s: %+v, want the folder %s", what, diagErr, c.want)
		}
	}
}

func librarySources(sources []Source) []Source {
	var listed []Source
	for _, source := range sources {
		if source.Library != "" {
			listed = append(listed, source)
		}
	}
	return listed
}

func TestCollectLibrariesListsTheLibrariesModulesAsCollectListsThem(t *testing.T) {
	root := mainOnly.withFiles(
		"lua/own.lua", "return 'own'\n",
		inLibrary("a", "one.lua"), "return 1\n", inLibrary("a", "two.yue"), "x = 2\n",
		inLibrary("a", "kit/init.yue"), "x = 3\n", inLibrary("a", "kit/init.lua"), "-- compiled\n",
		inLibrary("b", "three.lua"), mark+"return 3\n", inLibrary("b", "four.yue"), "x = 4\n",
		inLibrary("b", "notes.txt"), "no module\n",
	).writeToTempDir(t)
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
		whole, err := CollectSources(root, c.libraries)
		if err != nil {
			t.Fatal(err)
		}
		alone, err := CollectLibrarySources(root, c.libraries)
		if err != nil || !slices.Equal(sourceNames(alone), c.want) || !reflect.DeepEqual(alone, librarySources(whole)) {
			t.Errorf("CollectLibraries with %+v = %+v, %v, want the modules %q, as Collect lists them: %+v",
				c.libraries, alone, err, c.want, librarySources(whole))
		}
	}
}

func TestCollectLibrariesDoesNotLookAtTheProjectsOwnModules(t *testing.T) {
	greet := inLibrary("ex", "example/greet.lua")
	library := newSourceTree(greet, "return {}\n").withLibraries("ex")
	withSrc := library.withFiles("src/main.yue", "x = 1\n")
	const clash = "Module example.greet is defined by"
	cases := []struct {
		name    string
		project sourceTree
		refusal string
	}{
		{"a module of src/ with the name", library.withFiles("src/example/greet.yue", "x = 1\n"), clash},
		{"a module of lua/ with the name", withSrc.withFiles("lua/example/greet.lua", ""), clash},
		{"a module of src/ that claims the name", library.withFiles("src/example/greet/init.yue", ""), clash},
		{"no src/", library, "The src/ folder is missing."},
		{"a file for src/", library.withFiles("src", "a file, not a folder"), "The src/ folder is missing."},
		{"a dotted name in src/", library.withFiles("src/a.b.yue", ""), "cannot contain dots"},
		{"a built-in module's name in lua/", withSrc.withFiles("lua/moonwell.lua", ""), "is built into"},
	}
	want := []Source{{Name: "example.greet", Path: greet, Kind: Lua, Library: "ex", Text: "return {}\n"}}
	for _, c := range cases {
		root := c.project.writeToTempDir(t)
		_, err := CollectSources(root, c.project.libraries())
		if diagErr := asDiagError(t, err, c.name); !strings.Contains(diagErr.Msg, c.refusal) {
			t.Errorf("%s: Collect refuses with %+v, want %q", c.name, diagErr, c.refusal)
		}
		if got, err := CollectLibrarySources(root, c.project.libraries()); err != nil || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: CollectLibraries = %+v, %v, want %+v", c.name, got, err, want)
		}
	}
}

func TestCollectLibrariesDoesNotGoThroughALinkAtAFolderOfTheProjectsOwn(t *testing.T) {
	for _, symlink := range []string{"src", "lua"} {
		p := newSourceTree(inLibrary("ex", "x.lua"), "").withLibraries("ex")
		if symlink == "lua" {
			p = p.withFiles("src/main.yue", "")
		}
		root := p.writeToTempDir(t)
		at := symlinkTree(t, newSourceTree("main.yue", "", "x.lua", ""), root, symlink)
		_, err := CollectSources(root, p.libraries())
		if diagErr := asDiagError(t, err, symlink); diagErr.Msg != "Symlinks are not supported: "+at {
			t.Errorf("a link at %s: Collect refuses with %+v", symlink, diagErr)
		}
		got, err := CollectLibrarySources(root, p.libraries())
		if err != nil || !slices.Equal(sourcePaths(got), []string{inLibrary("ex", "x.lua")}) {
			t.Errorf("a link at %s: CollectLibraries = %q, %v", symlink, sourcePaths(got), err)
		}
	}
}

func TestCollectLibrariesRefusesWhatCollectRefusesOfALibrary(t *testing.T) {
	one, two := mainOnly.withLibraries("ex"), mainOnly.withLibraries("b", "a")
	for what, p := range map[string]sourceTree{
		"a dotted name":              one.withFiles(inLibrary("ex", "a.b.lua"), ""),
		"a dotted folder":            one.withFiles(inLibrary("ex", "a.b/c.yue"), ""),
		"a built-in module's name":   one.withFiles(inLibrary("ex", "moonwell.lua"), ""),
		"a name of two libraries":    two.withFiles(inLibrary("a", "kit.lua"), "", inLibrary("b", "kit.yue"), ""),
		"a name that an init claims": two.withFiles(inLibrary("a", "kit/init.yue"), "", inLibrary("b", "kit.lua"), ""),
	} {
		root := p.writeToTempDir(t)
		_, whole := CollectSources(root, p.libraries())
		got, alone := CollectLibrarySources(root, p.libraries())
		want := asDiagError(t, whole, what)
		if diagErr := asDiagError(t, alone, what); got != nil || !reflect.DeepEqual(diagErr, want) || want.File == "" {
			t.Errorf("%s: CollectLibraries = %+v, %+v, want the refusal of Collect: %+v", what, got, diagErr, want)
		}
	}
}

func TestCollectLibrariesRefusesALinkAtAFolderOfALibrary(t *testing.T) {
	for _, symlink := range []string{".moonwell", librariesDir, librariesDir + "/ex"} {
		p := mainOnly.withLibraries("ex")
		root := p.writeToTempDir(t)
		at := symlinkTree(t, newSourceTree("libraries/ex/x.lua", "", "ex/x.lua", "", "x.lua", ""), root, symlink)
		got, err := CollectLibrarySources(root, p.libraries())
		diagErr := asDiagError(t, err, symlink)
		if got != nil || diagErr.Msg != "Symlinks are not supported: "+at ||
			!strings.Contains(diagErr.Hint, "real files") {
			t.Errorf("a link at %s: CollectLibraries = %+v, %+v", symlink, got, diagErr)
		}
	}
}

func TestCollectLibrariesTakesALibraryThatNamesNoFolderOfTheProjectForAMistakeOfTheCaller(t *testing.T) {
	root := newSourceTree().writeToTempDir(t)
	for _, library := range []Library{
		{Key: "", Dir: librariesDir + "/ex"}, {Key: "ex", Dir: ""}, {Key: "ex", Dir: "../ex"}, {Key: "ex", Dir: "/ex"},
	} {
		_, err := CollectLibrarySources(root, []Library{library})
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
		diagErr := asDiagError(t, err, entry)
		if name != "" || diagErr.Msg != "Entry '"+entry+"' must be a .yue file under src/." || diagErr.File != "" ||
			diagErr.Hint != "For example: src/main.yue" {
			t.Errorf("EntryName(%q) = %q, %+v", entry, name, diagErr)
		}
	}
}
