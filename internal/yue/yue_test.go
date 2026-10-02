package yue_test

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/bundle"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/yue"
	"github.com/mdlsvensson/moonwell/internal/yuetest"
)

// These tests run the real compiler.

func project(t *testing.T, files ...string) string {
	t.Helper()
	root := t.TempDir()
	for i := 0; i < len(files); i += 2 {
		testkit.WriteFile(t, root, files[i], []byte(files[i+1]))
	}
	return root
}

// counting runs the real compiler and records the source of each run.
type counting struct {
	lock     sync.Mutex
	compiled []string
}

func (c *counting) run(ctx context.Context, command string, args []string, options proc.Options) (proc.Result, error) {
	c.lock.Lock()
	c.compiled = append(c.compiled, args[len(args)-1])
	c.lock.Unlock()
	return proc.Run(ctx, command, args, options)
}

func compile(t *testing.T, options yue.CompileOptions) *yue.Output {
	t.Helper()
	output, err := yue.Compile(background, options)
	if err != nil {
		t.Fatal(err)
	}
	return output
}

func load(t *testing.T, output *yue.Output, name string) *bundle.CompiledModule {
	t.Helper()
	module, err := output.Load(name)
	if err != nil {
		t.Fatal(err)
	}
	return module
}

func keysOf(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return keys
}

func macrosOf(t *testing.T, root string) *yue.MacroSearch {
	t.Helper()
	macros, err := yue.Macros(root)
	if err != nil {
		t.Fatal(err)
	}
	return macros
}

func TestCompileCompilesModulesAndLoadsThemByDottedName(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t,
		"src/main.yue", "import \"util.math\" as M\nexport answer = M.double 21\n",
		"src/util/math.yue", "export double = (x) -> x * 2\n",
	)
	output := compile(t, yue.CompileOptions{Yue: compiler, Root: root})
	if !slices.Equal(keysOf(output.Hashes), []string{"main.yue", "util/math.yue"}) || len(output.Hashes["main.yue"]) != 64 ||
		output.Texts["src/util/math.yue"] != "export double = (x) -> x * 2\n" || output.OutDir != filepath.Join(root, "dist", "stage", "lua") {
		t.Errorf("output = %+v", output)
	}
	main := load(t, output, "main")
	if main == nil || main.Name != "main" || main.SourcePath != "src/main.yue" || !strings.Contains(main.Source, `require("util.math")`) {
		t.Errorf("main = %+v", main)
	}
	if math := load(t, output, "util.math"); math == nil || math.SourcePath != "src/util/math.yue" {
		t.Errorf("util.math = %+v", math)
	}
	for _, name := range []string{"missing", "util/math", "Util.Math"} {
		if module := load(t, output, name); module != nil {
			t.Errorf("Load(%s) = %+v", name, module)
		}
	}
	hashes, _ := os.ReadFile(filepath.Join(output.OutDir, ".hashes.json"))
	want := "{\n  \"settings\": " + strings.ReplaceAll(`"paths|`+compiler+`|rewrite|no macros"`, `\`, `\\`) + ",\n  \"files\": {\n" +
		"    \"src/main.yue\": \"" + output.Hashes["main.yue"] + "\",\n" +
		"    \"src/util/math.yue\": \"" + output.Hashes["util/math.yue"] + "\"\n  }\n}"
	if string(hashes) != want {
		t.Errorf(".hashes.json is\n%s\nwant\n%s", hashes, want)
	}
}

func TestCompileCompilesLibraryYueScriptIntoItsOwnFolder(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t,
		"src/main.yue", "import \"example.loud\"\n",
		".moonwell/libraries/ex/example/loud.yue", "export shout = (name) -> name\\upper!\n",
	)
	modules, err := bundle.CollectModules(root, append(slices.Clone(bundle.ProjectRoots), bundle.LibraryRoots([]string{"ex"})...))
	if err != nil {
		t.Fatal(err)
	}
	output := compile(t, yue.CompileOptions{Yue: compiler, Root: root, Modules: modules})
	library := modules[slices.IndexFunc(modules, func(module bundle.SourceModule) bool { return module.Name == "example.loud" })]
	compiled, err := output.LoadModule(library)
	if err != nil || library.Library != "ex" || compiled.SourcePath != ".moonwell/libraries/ex/example/loud.yue" ||
		!strings.Contains(compiled.Source, "upper") {
		t.Errorf("LoadModule = %+v, %v", compiled, err)
	}
	if !fsx.Exists(filepath.Join(root, "dist", "stage", "lua", ".libraries", "ex", "example", "loud.lua")) {
		t.Error("the library's output is not under .libraries/ex")
	}
	// The unknown-global check still sees src/ only.
	if !slices.Equal(keysOf(output.Hashes), []string{"main.yue"}) ||
		!slices.Equal(keysOf(output.Texts), []string{".moonwell/libraries/ex/example/loud.yue", "src/main.yue"}) {
		t.Errorf("hashes %q, texts %q", keysOf(output.Hashes), keysOf(output.Texts))
	}
	if module, err := output.Load("example.loud"); module != nil || err != nil {
		t.Errorf("Load of a library's module = %+v, %v", module, err)
	}
}

func TestCompileOnlyRecompilesChangedFilesAndRemovesDeletedOutputs(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t, "src/a.yue", "export x = 1\n", "src/b.yue", "export y = 2\n", "src/c.yue", "export z = 4\n")
	compile(t, yue.CompileOptions{Yue: compiler, Root: root})

	testkit.WriteFile(t, root, "src/a.yue", []byte("export x = 3\n"))
	os.Remove(filepath.Join(root, "src", "b.yue"))
	counted := &counting{}
	output := compile(t, yue.CompileOptions{Yue: compiler, Root: root, Run: counted.run})
	if len(counted.compiled) != 1 || !strings.HasSuffix(counted.compiled[0], "a.yue") {
		t.Errorf("compiled %q", counted.compiled)
	}
	if !strings.Contains(load(t, output, "a").Source, "3") || fsx.Exists(filepath.Join(root, "dist", "stage", "lua", "b.lua")) {
		t.Error("a.lua is stale, or b.lua is still there")
	}

	unchanged := &counting{}
	compile(t, yue.CompileOptions{Yue: compiler, Root: root, Run: unchanged.run})
	if len(unchanged.compiled) != 0 {
		t.Errorf("unchanged, but compiled %q", unchanged.compiled)
	}
	minified := &counting{}
	compile(t, yue.CompileOptions{Yue: compiler, Root: root, Minify: true, Run: minified.run})
	if len(minified.compiled) != 2 {
		t.Errorf("minified compiled %q", minified.compiled)
	}
}

func TestCompileReportsSyntaxErrorsWithFileAndLine(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t, "src/ok.yue", "export x = 1\n", "src/bad.yue", "x = 1\ny = \n  if then\n")
	_, err := yue.Compile(background, yue.CompileOptions{Yue: compiler, Root: root})
	if e := asError(t, err, "a syntax error"); e.File != "src/bad.yue" || e.Line != 2 {
		t.Errorf("error = %+v", e)
	}
	// The file that compiled keeps its place in the hashes; the failed one has none, so it compiles again.
	hashes, _ := os.ReadFile(filepath.Join(root, "dist", "stage", "lua", ".hashes.json"))
	if !strings.Contains(string(hashes), `"src/ok.yue"`) || strings.Contains(string(hashes), `"src/bad.yue"`) {
		t.Errorf(".hashes.json is\n%s", hashes)
	}
}

func TestCompileReportsTheFirstOfSeveralFailedFilesAndCountsTheRest(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t, "src/b.yue", "y = \n  if then\n", "src/A.yue", "\ny = \n  if then\n", "src/c.yue", "y = \n  if then\n")
	_, err := yue.Compile(background, yue.CompileOptions{Yue: compiler, Root: root})
	e := asError(t, err, "three syntax errors")
	if e.File != "src/A.yue" || e.Line != 2 || !strings.HasSuffix(e.Msg, "\n(2 more file(s) failed to compile)") || e.Hint != "" {
		t.Errorf("error = %+v", e)
	}
}

func TestAnEmptyCompileOutputForAFileWithCodeFailsInsteadOfDroppingTheModule(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t, "src/main.yue", "-- a comment\nexport x = 1\n", "src/notes.yue", "-- only comments\n\n")
	// yue reports success but writes nothing, as 0.34.2 does for `//`; the output file follows `-o`.
	run := func(ctx context.Context, command string, args []string, options proc.Options) (proc.Result, error) {
		result, err := proc.Run(ctx, command, args, options)
		if strings.HasSuffix(args[len(args)-1], "main.yue") {
			os.WriteFile(args[slices.Index(args, "-o")+1], nil, 0o666)
		}
		return result, err
	}
	_, err := yue.Compile(background, yue.CompileOptions{Yue: compiler, Root: root, Run: run})
	e := asError(t, err, "an empty output")
	if e.Msg != "YueScript reported success but wrote no Lua for src/main.yue, although the file has code." || e.File != "src/main.yue" ||
		!strings.Contains(e.Hint, "//") {
		t.Errorf("error = %+v", e)
	}
}

func TestAFileUsingFloorDivisionCompilesNormalAndMinified(t *testing.T) {
	compiler := yuetest.Need(t)
	// yue 0.34.2 emptied such a file; 0.34.3, the pinned version, compiles it.
	for _, minify := range []bool{false, true} {
		root := project(t, "src/main.yue", "x = 7 // 2\nprint x\n")
		output := compile(t, yue.CompileOptions{Yue: compiler, Root: root, Minify: minify})
		if main := load(t, output, "main"); main == nil || !strings.Contains(main.Source, "//") {
			t.Errorf("minify %v: main = %+v", minify, main)
		}
	}
}

func TestAFileUsingABitwiseOperatorFailsAtItsLineWithAHint(t *testing.T) {
	compiler := yuetest.Need(t)
	// yue compiles the operators, but the step that rewrites or minifies the Lua does not read them: it fails and
	// leaves the Lua it could not rewrite, which the build must not use.
	const source = "x = 1\n\n\nflags = x & 3\nprint flags\n"
	root := project(t, "src/main.yue", source)
	_, err := yue.Compile(background, yue.CompileOptions{Yue: compiler, Root: root})
	e := asError(t, err, "a bitwise operator")
	if e.Msg != "YueScript compiled this file but could not rewrite its Lua: Unexpected Symbol `&` in source." ||
		e.File != "src/main.yue" || e.Line != 4 || !strings.Contains(e.Hint, "bitwise operators") || !strings.Contains(e.Hint, "lua/") {
		t.Errorf("error = %+v", e)
	}
	if fsx.Exists(filepath.Join(root, "dist", "stage", "lua", "main.lua")) {
		t.Error("the Lua that could not be rewritten was left behind")
	}

	// A minified build has no line to give: the Lua it fails on carries no line marks.
	root = project(t, "src/main.yue", source)
	_, err = yue.Compile(background, yue.CompileOptions{Yue: compiler, Root: root, Minify: true})
	e = asError(t, err, "a bitwise operator, minified")
	if e.Msg != "YueScript compiled this file but could not minify its Lua: Unexpected Symbol `&` in source." ||
		e.File != "src/main.yue" || e.Line != 0 || !strings.Contains(e.Hint, "bitwise operators") {
		t.Errorf("minified: error = %+v", e)
	}
}

func TestCompileRejectsDotsInFileNames(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t, "src/a.b.yue", "export x = 1\n")
	_, err := yue.Compile(background, yue.CompileOptions{Yue: compiler, Root: root})
	if e := asError(t, err, "a dotted name"); !strings.Contains(e.Msg, "dots") {
		t.Errorf("error = %+v", e)
	}
}

func TestCompileExpandsFourCCThroughTheMacroModule(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t,
		yue.MacrosFile, moonwell.MacrosYue,
		"src/main.yue", "import \"moonwell.macros\" as {:$FourCC}\nexport footman = $FourCC \"hfoo\"\n",
	)
	output := compile(t, yue.CompileOptions{Yue: compiler, Root: root, Macros: macrosOf(t, root)})
	if source := load(t, output, "main").Source; !strings.Contains(source, "1751543663") {
		t.Errorf("main.lua is\n%s", source)
	}
}

func TestAChangedMacroModuleRecompilesEveryFile(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t, yue.MacrosFile, moonwell.MacrosYue, "src/a.yue", "export x = 1\n", "src/b.yue", "export y = 2\n")
	macros := macrosOf(t, root)
	compile(t, yue.CompileOptions{Yue: compiler, Root: root, Macros: macros})
	unchanged := &counting{}
	compile(t, yue.CompileOptions{Yue: compiler, Root: root, Macros: macros, Run: unchanged.run})
	changed := &counting{}
	compile(t, yue.CompileOptions{Yue: compiler, Root: root, Macros: &yue.MacroSearch{Path: macros.Path, Hash: "another"}, Run: changed.run})
	if len(unchanged.compiled) != 0 || len(changed.compiled) != 2 {
		t.Errorf("unchanged compiled %q, changed compiled %q", unchanged.compiled, changed.compiled)
	}
}

const fourCCMessage = `$FourCC needs a string literal of exactly 4 characters, such as "hfoo".`

func TestAFailedMacroNamesTheFileAndLineWithTheMacrosOwnMessage(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t,
		yue.MacrosFile, moonwell.MacrosYue,
		"src/main.yue", "import \"moonwell.macros\" as {:$FourCC}\nx = 1\ny = $FourCC \"hfo\"\n",
	)
	_, err := yue.Compile(background, yue.CompileOptions{Yue: compiler, Root: root})
	if e := asError(t, err, "no macro path"); !strings.Contains(e.Msg, "moonwell.macros") {
		t.Errorf("without --path the module is not found, but: %+v", e)
	}
	_, err = yue.Compile(background, yue.CompileOptions{Yue: compiler, Root: root, Macros: macrosOf(t, root)})
	if e := asError(t, err, "a failed macro"); e.File != "src/main.yue" || e.Line != 3 || !strings.HasPrefix(e.Msg, fourCCMessage+"\n") {
		t.Errorf("error = %+v", e)
	}
}

// expand compiles `print <call>` after the macro import in a new project: the Lua, or the compiler's output.
func expand(t *testing.T, compiler, call string) (ok bool, output string) {
	t.Helper()
	root := project(t, yue.MacrosFile, moonwell.MacrosYue, "src/main.yue", "import \"moonwell.macros\" as {:$FourCC}\nprint "+call+"\n")
	target := filepath.Join(root, "main.lua")
	args := append([]string{"--target=5.3", "-r", "-o", target}, macrosOf(t, root).PathArgs()...)
	result, err := proc.Run(background, compiler, append(args, filepath.Join(root, "src", "main.yue")), proc.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if result.Code != 0 {
		return false, result.Stdout + "\n" + result.Stderr
	}
	lua, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	return true, string(lua)
}

func TestFourCCTurnsA4CharacterStringLiteralIntoTheRawcodesInteger(t *testing.T) {
	compiler := yuetest.Need(t)
	for call, want := range map[string]string{
		`$FourCC "hfoo"`:  "1751543663",
		`$FourCC 'hfoo'`:  "1751543663",
		`$FourCC("hfoo")`: "1751543663",
		`$FourCC "Hpal"`:  "1215324524",
		// A single-quoted string does not interpolate, so '#{a}' is its own four characters.
		`$FourCC '#{a}'`: "595288445",
	} {
		ok, lua := expand(t, compiler, call)
		if !ok || !strings.Contains(lua, want) || strings.Contains(lua, "moonwell.macros") {
			t.Errorf("%s: %v\n%s", call, ok, lua)
		}
	}
}

func TestFourCCRefusesAnythingButA4CharacterStringLiteral(t *testing.T) {
	compiler := yuetest.Need(t)
	for _, call := range []string{
		"$FourCC!", "$FourCC x", `$FourCC "hfo"`, `$FourCC "hfooo"`, "$FourCC 1234", `$FourCC "h\oo"`, `$FourCC "héé"`,
		`$FourCC "hé!"`, "$FourCC [[hfoo]]", `$FourCC "hfoo", "x"`, `$FourCC "#{x}"`,
	} {
		if ok, output := expand(t, compiler, call); ok || !strings.Contains(output, fourCCMessage) {
			t.Errorf("%s: %v\n%s", call, ok, output)
		}
	}
}

func TestMacrosPointsYueAtItsFolderAndHashesTheModule(t *testing.T) {
	root := filepath.FromSlash("/project")
	search := macrosOf(t, root)
	if search.Path != filepath.Join(root, ".moonwell", "yue", "?.lua") || search.Hash != fsx.SHA256Hex([]byte(moonwell.MacrosYue)) ||
		!slices.Equal(search.PathArgs(), []string{"--path", search.Path}) {
		t.Errorf("Macros = %+v", search)
	}
	var none *yue.MacroSearch
	if args := none.PathArgs(); len(args) != 0 {
		t.Errorf("PathArgs without macros = %q", args)
	}
	if yue.MacrosFile != ".moonwell/yue/moonwell/macros.yue" {
		t.Errorf("MacrosFile = %s", yue.MacrosFile)
	}
}

func TestMacrosRefusesAProjectFolderWhosePathHasASemicolonOrAQuestionMark(t *testing.T) {
	for _, root := range []string{"/pro;ject", "/pro?ject"} {
		_, err := yue.Macros(root)
		e := asError(t, err, root)
		if e.Msg != `The project folder's path contains ";" or "?", which YueScript's module search cannot handle.` || e.File != root ||
			e.Hint != "Move the project to a folder whose path has neither character." {
			t.Errorf("%s: %+v", root, e)
		}
	}
}

func TestListGlobalUsesReadsTheRealCompilersOutput(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t, "src/main.yue", "global Score = 0\nprint CreatUnit!\nx = math.floor 1.5\nprint Score, x\n")
	uses, err := yue.ListGlobalUses(background, yue.UsesOptions{Yue: compiler, Root: root, Hashes: map[string]string{"main.yue": "h"}})
	want := map[string][]yue.GlobalUse{"main.yue": {
		{Name: "Score", Line: 1, Column: 8},
		{Name: "print", Line: 2, Column: 1},
		{Name: "CreatUnit", Line: 2, Column: 7},
		{Name: "math", Line: 3, Column: 5},
		{Name: "print", Line: 4, Column: 1},
		{Name: "Score", Line: 4, Column: 7},
	}}
	if err != nil || !reflect.DeepEqual(uses, want) {
		t.Errorf("ListGlobalUses = %+v, %v", uses, err)
	}
}

func TestYueWithTheMacroPathListsNoGlobalForAFourCCCall(t *testing.T) {
	compiler := yuetest.Need(t)
	root := project(t, yue.MacrosFile, moonwell.MacrosYue, "src/main.yue", "import \"moonwell.macros\" as {:$FourCC}\nprint $FourCC \"hfoo\"\n")
	uses, err := yue.ListGlobalUses(background, yue.UsesOptions{
		Yue: compiler, Root: root, Hashes: map[string]string{"main.yue": "h"}, Macros: macrosOf(t, root),
	})
	want := map[string][]yue.GlobalUse{"main.yue": {{Name: "print", Line: 2, Column: 1}}}
	if err != nil || !reflect.DeepEqual(uses, want) {
		t.Errorf("ListGlobalUses = %+v, %v", uses, err)
	}
}
