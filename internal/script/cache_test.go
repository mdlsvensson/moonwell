package script

import (
	"context"
	"errors"
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

func (b *compileFixture) readHashesFile() string {
	text, err := os.ReadFile(b.readStaged(".hashes.json"))
	if err != nil {
		return ""
	}
	return string(text)
}

func TestTheHashesFileHoldsWhatTheOutputsDependOnAndEachSourcesHash(t *testing.T) {
	loud := inLibrary("ex", "kit/loud.yue")
	b := newCompileFixture(t, newSourceTree("src/main.yue", "x = 1\n", "src/util/math.yue", "y = 2\n").withLibraries("ex").withFiles(loud, "z = 3\n"))
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	want := "{\n" +
		"  \"compiler\": \"yue-of-the-test\",\n" +
		"  \"mode\": \"-r\",\n" +
		"  \"macros\": \"" + fsx.SHA256Hex([]byte(moonwell.MacrosYue)) + "\",\n" +
		"  \"macroSources\": \"\",\n" +
		"  \"sources\": {\n" +
		"    \".moonwell/libraries/ex/kit/loud.yue\": {\n" +
		"      \"hash\": \"" + fsx.SHA256Hex([]byte("z = 3\n")) + "\",\n" +
		"      \"output\": \".libraries/ex/kit/loud.lua\"\n" +
		"    },\n" +
		"    \"src/main.yue\": {\n" +
		"      \"hash\": \"" + fsx.SHA256Hex([]byte("x = 1\n")) + "\",\n" +
		"      \"output\": \"main.lua\"\n" +
		"    },\n" +
		"    \"src/util/math.yue\": {\n" +
		"      \"hash\": \"" + fsx.SHA256Hex([]byte("y = 2\n")) + "\",\n" +
		"      \"output\": \"util/math.lua\"\n" +
		"    }\n" +
		"  }\n" +
		"}\n"
	if got := b.readHashesFile(); got != want {
		t.Errorf(".hashes.json is\n%s\nwant\n%s", got, want)
	}
	b.mustCompile(fakeYue, true)
	if got, wantMinified := b.readHashesFile(), strings.Replace(want, `"-r"`, `"-m"`, 1); got != wantMinified {
		t.Errorf("minified, .hashes.json is\n%s\nwant\n%s", got, wantMinified)
	}
}

func TestEverySourceIsCompiledAgainWhenTheCompilerTheModeOrTheMacroModuleChanged(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n"))
	b.fakeCompiler(nil)
	all := []string{"src/a.yue", "src/b.yue"}
	steps := []struct {
		what   string
		yue    string
		minify bool
		macros string
		want   []string
	}{
		{"the first compile", fakeYue, false, "one", all},
		{"nothing changed", fakeYue, false, "one", []string{}},
		{"another compiler", "another-yue", false, "one", all},
		{"that compiler again", "another-yue", false, "one", []string{}},
		{"minified", "another-yue", true, "one", all},
		{"minified again", "another-yue", true, "one", []string{}},
		{"another macro module", "another-yue", true, "two", all},
		{"that macro module again", "another-yue", true, "two", []string{}},
		{"all three as at first", fakeYue, false, "one", all},
	}
	for _, step := range steps {
		b.macros.hash = step.macros
		b.mustCompile(step.yue, step.minify)
		if ran := b.ranSources(); !slices.Equal(ran, step.want) {
			t.Errorf("%s: the compiler ran on %q, want %q", step.what, ran, step.want)
		}
	}
}

func TestASourceIsCompiledAgainWhenItChangedOrItsOutputIsGone(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n", "src/c.yue", "z = 3\n"))
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	b.ranSources()

	b.removeFile("dist/stage/lua/b.lua")
	b.mustCompile(fakeYue, false)
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/b.yue"}) || !fsx.Exists(b.readStaged("b.lua")) {
		t.Errorf("with the output of b.yue gone, the compiler ran on %q", ran)
	}
	b.writeFile("src/c.yue", "z = 4\n")
	b.mustCompile(fakeYue, false)
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/c.yue"}) {
		t.Errorf("with c.yue changed, the compiler ran on %q", ran)
	}
	b.writeFile("src/c.yue", "z = 3\n")
	b.mustCompile(fakeYue, false)
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/c.yue"}) {
		t.Errorf("with c.yue as it was at first, the compiler ran on %q", ran)
	}
}

func TestTheHashesFileIsWrittenWhenSomeFilesFailedHoldingThoseThatCompiled(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/bad.yue", "x = \n", "src/c.yue", "z = 3\n"))
	b.fakeCompiler(map[string]fakeResult{"src/bad.yue": {code: 1, stdout: "Failed to compile: bad.yue\n1: boom\n"}})
	b.mustFailCompile(fakeYue, false, "a failed file")
	kept, err := readCompileCache(b.root)
	if paths := slices.Sorted(maps.Keys(kept.Sources)); err != nil || !slices.Equal(paths, []string{"src/a.yue", "src/c.yue"}) {
		t.Errorf("the hashes file keeps %q, %v", paths, err)
	}
	b.ranSources()
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/bad.yue"}) {
		t.Errorf("the next compile ran on %q", ran)
	}
}

func TestTheOutputOfASourceThatIsGoneIsRemoved(t *testing.T) {
	loud := inLibrary("ex", "kit/loud.yue")
	b := newCompileFixture(t, newSourceTree("src/main.yue", "x = 1\n", "src/util/math.yue", "y = 2\n", "src/gone.yue", "z = 3\n").withLibraries("ex").withFiles(loud, "w = 4\n"))
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	outputs := []string{"main.lua", "util/math.lua", "gone.lua", ".libraries/ex/kit/loud.lua"}
	there := func() (found []string) {
		for _, output := range outputs {
			if fsx.Exists(b.readStaged(output)) {
				found = append(found, output)
			}
		}
		return found
	}
	if !slices.Equal(there(), outputs) {
		t.Fatalf("the first compile left %q", there())
	}
	b.ranSources()

	b.removeFile("src/util/math.yue")
	b.removeFile("src/gone.yue")
	b.removeFile("dist/stage/lua/gone.lua")
	b.mustCompile(fakeYue, false)
	if ran, left := b.ranSources(), there(); len(ran) != 0 || !slices.Equal(left, []string{"main.lua", ".libraries/ex/kit/loud.lua"}) {
		t.Errorf("with two sources gone, the compiler ran on %q and the outputs are %q", ran, left)
	}

	b.libraries = []Library{{Key: "renamed", Dir: librariesDir + "/ex"}}
	outputs = append(outputs, ".libraries/renamed/kit/loud.lua")
	b.mustCompile(fakeYue, false)
	if ran, left := b.ranSources(), there(); !slices.Equal(ran, []string{loud}) || !slices.Equal(left, []string{"main.lua", ".libraries/renamed/kit/loud.lua"}) {
		t.Errorf("with the library's key changed, the compiler ran on %q and the outputs are %q", ran, left)
	}

	b.libraries = nil
	b.mustCompile(fakeYue, false)
	if ran, left := b.ranSources(), there(); len(ran) != 0 || !slices.Equal(left, []string{"main.lua"}) {
		t.Errorf("with the library gone, the compiler ran on %q and the outputs are %q", ran, left)
	}
}

func TestAHashesFileInAnotherShapeCountsAsAbsent(t *testing.T) {
	first := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n"))
	first.fakeCompiler(nil)
	first.mustCompile(fakeYue, false)
	good := first.readHashesFile()
	if !strings.Contains(good, `"output": "b.lua"`) || !strings.Contains(good, `"mode": "-r",`) {
		t.Fatalf(".hashes.json is\n%s", good)
	}
	hashOfA := fsx.SHA256Hex([]byte("x = 1\n"))
	for what, text := range map[string]string{
		"the shape of another version": `{"settings": "paths|yue-of-the-test|rewrite|x", "files": {"src/a.yue": "` + hashOfA + `", "src/b.yue": "y"}}`,
		"an empty file":                "",
		"no JSON":                      "not JSON",
		"JSON that is cut short":       good[:len(good)/2],
		"null":                         "null",
		"a list":                       "[]",
		"a string":                     `"text"`,
		"an empty object":              "{}",
		"a member it has not":          strings.Replace(good, `"mode"`, `"more": 1, "mode"`, 1),
		"text after it":                good + "x",
		"an object after it":           good + "{}",
		"a bracket after it":           good + "]",
		"a brace after it":             good + "}",
		"sources that are a list":      `{"compiler": "yue-of-the-test", "mode": "-r", "macros": "x", "sources": []}`,
		"a source that is a string":    `{"compiler": "yue-of-the-test", "mode": "-r", "macros": "x", "sources": {"src/a.yue": "` + hashOfA + `"}}`,
		"a hash that is a number":      strings.Replace(good, `"hash": "`+hashOfA+`"`, `"hash": 1`, 1),
		"an output above the folder":   strings.Replace(good, `"output": "b.lua"`, `"output": "../b.lua"`, 1),
		"an output above, by a detour": strings.Replace(good, `"output": "b.lua"`, `"output": "a/../../b.lua"`, 1),
		"an output that is no Lua":     strings.Replace(good, `"output": "b.lua"`, `"output": ".hashes.json"`, 1),
		"an output without a name":     strings.Replace(good, `"output": "b.lua"`, `"output": ""`, 1),
		"an output from the root":      strings.Replace(good, `"output": "b.lua"`, `"output": "/b.lua"`, 1),
	} {
		b := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n"))
		b.fakeCompiler(nil)
		b.mustCompile(fakeYue, false)
		b.ranSources()
		b.writeFile("dist/stage/lua/.hashes.json", text)
		b.removeFile("src/b.yue")
		b.mustCompile(fakeYue, false)
		if ran := b.ranSources(); !slices.Equal(ran, []string{"src/a.yue"}) || !fsx.Exists(b.readStaged("b.lua")) {
			t.Errorf("%s: the compiler ran on %q, and b.lua is there: %v", what, ran, fsx.Exists(b.readStaged("b.lua")))
		}
		kept, err := readCompileCache(b.root)
		want := map[string]cachedSource{"src/a.yue": {Hash: hashOfA, Output: "a.lua"}}
		if err != nil || !reflect.DeepEqual(kept.Sources, want) || kept.Compiler != fakeYue {
			t.Errorf("%s: the hashes file written over it keeps %+v, %v", what, kept, err)
		}
	}
	b := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n"))
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	b.ranSources()
	b.writeFile("dist/stage/lua/.hashes.json", good)
	b.removeFile("src/b.yue")
	b.mustCompile(fakeYue, false)
	if ran := b.ranSources(); len(ran) != 0 || fsx.Exists(b.readStaged("b.lua")) {
		t.Errorf("with the file as it was written, the compiler ran on %q, and b.lua is there: %v", ran, fsx.Exists(b.readStaged("b.lua")))
	}
}

func (b *compileFixture) failCompileOf(source string, stopped error) {
	b.setCompiler(func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		if b.sourceOf(args) == source {
			return env.RunResult{}, stopped
		}
		return env.RunResult{}, os.WriteFile(outputIn(args), []byte("-- "+b.sourceOf(args)+"\n"), 0o666)
	})
}

func TestAStoppedRunLeavesNothingItWasToCompileUpToDate(t *testing.T) {
	stopped := errors.New("stopped")
	b := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/kept.yue", "y = 2\n", "src/stop.yue", "z = 3\n"))
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	b.ranSources()

	b.writeFile("src/a.yue", "x = 2\n")
	b.writeFile("src/new.yue", "w = 5\n")
	b.writeFile("src/stop.yue", "z = 4\n")
	b.failCompileOf("src/stop.yue", stopped)
	if result, err := b.tryCompile(fakeYue, false); result != nil || err != stopped {
		t.Fatalf("the stopped run: %+v, %v", result, err)
	}
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/a.yue", "src/new.yue", "src/stop.yue"}) || !fsx.Exists(b.readStaged("new.lua")) {
		t.Fatalf("the stopped run ran the compiler on %q, and new.lua is there: %v", ran, fsx.Exists(b.readStaged("new.lua")))
	}
	kept, err := readCompileCache(b.root)
	want := map[string]cachedSource{
		"src/a.yue": {Output: "a.lua"}, "src/new.yue": {Output: "new.lua"}, "src/stop.yue": {Output: "stop.lua"},
		"src/kept.yue": {Hash: fsx.SHA256Hex([]byte("y = 2\n")), Output: "kept.lua"},
	}
	if err != nil || !reflect.DeepEqual(kept.Sources, want) {
		t.Errorf("after the stopped run the hashes file keeps %+v, %v, want %+v", kept.Sources, err, want)
	}

	b.writeFile("src/a.yue", "x = 1\n")
	b.removeFile("src/new.yue")
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/a.yue", "src/stop.yue"}) || fsx.Exists(b.readStaged("new.lua")) {
		t.Errorf("after the stopped run the compiler ran on %q, and new.lua is there: %v", ran, fsx.Exists(b.readStaged("new.lua")))
	}
	b.mustCompile(fakeYue, false)
	if ran := b.ranSources(); len(ran) != 0 {
		t.Errorf("with nothing changed after that, the compiler ran on %q", ran)
	}
}

func TestAStoppedRunInAnotherModeLeavesNothingUpToDate(t *testing.T) {
	stopped := errors.New("stopped")
	b := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n"))
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	b.ranSources()
	b.failCompileOf("src/b.yue", stopped)
	if _, err := b.tryCompile(fakeYue, true); err != stopped {
		t.Fatalf("the stopped run: %v", err)
	}
	b.ranSources()
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	if ran := b.ranSources(); !slices.Equal(ran, []string{"src/a.yue", "src/b.yue"}) {
		t.Errorf("normal again after the stopped minified run, the compiler ran on %q", ran)
	}
}

func TestAProjectWithoutYueScriptCompilesNothingAndKeepsNoSources(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/notes.txt", "", "lua/tools.lua", "return {}\n"))
	result := b.mustCompile(fakeYue, false)
	if len(result.sourceTexts) != 0 || len(result.sourceHashes) != 0 || len(result.outputFiles) != 0 || result.sourceTexts == nil {
		t.Errorf("compileAll = %+v, want nothing compiled", result)
	}
	if text := b.readHashesFile(); !strings.Contains(text, `"sources": {}`) {
		t.Errorf(".hashes.json is\n%s", text)
	}
}

func TestALinkOnTheWayToTheHashesFileIsRefused(t *testing.T) {
	for _, symlink := range []string{"dist", "dist/stage", "dist/stage/lua"} {
		b := newCompileFixture(t, newSourceTree("src/notes.txt", ""))
		at := symlinkTree(t, newSourceTree(".hashes.json", "{}"), b.root, symlink)
		_, err := b.tryCompile(fakeYue, false)
		if diagErr := asDiagError(t, err, "a link at "+symlink); diagErr.Msg != "Symlinks are not supported: "+at ||
			diagErr.File != "dist/stage/lua" {
			t.Errorf("a link at %s: %+v", symlink, diagErr)
		}
	}
}

func TestAHashesFileThatCannotBeWrittenIsRefusedByItsPath(t *testing.T) {
	b := newCompileFixture(t, mainOnly.withFiles("dist/stage/lua/.hashes.json/kept.txt", ""))
	b.fakeCompiler(nil)
	diagErr := b.mustFailCompile(fakeYue, false, "a folder for the hashes file")
	if !strings.HasPrefix(diagErr.Msg, "Writing dist/stage/lua/.hashes.json failed: ") || diagErr.File != "dist/stage/lua/.hashes.json" ||
		!strings.Contains(diagErr.Hint, "dist/") || diagErr.Cause == nil {
		t.Errorf("error = %+v", diagErr)
	}
}

func TestACacheFileIsReadBackAsItWasWrittenAndNotAsAnotherShape(t *testing.T) {
	type counted struct {
		Words map[string]int `json:"words"`
		Of    string         `json:"of"`
	}
	root := t.TempDir()
	if kept, found, err := readCacheFile[counted](root, ".counted.json"); found || err != nil || kept.Words != nil {
		t.Errorf("without a file: %+v, %v, %v", kept, found, err)
	}
	want := counted{Words: map[string]int{"a<b>&c": 2, eAcute + beyond: 1}, Of: `C:\a "b"`}
	if err := writeCacheFile(root, ".counted.json", want); err != nil {
		t.Fatal(err)
	}
	if kept, found, err := readCacheFile[counted](root, ".counted.json"); !found || err != nil || !reflect.DeepEqual(kept, want) {
		t.Errorf("read back: %+v, %v, %v, want %+v", kept, found, err, want)
	}
	if kept, found, err := readCacheFile[compileCache](root, ".counted.json"); found || err != nil || kept.Sources != nil {
		t.Errorf("read as hashes: %+v, %v, %v", kept, found, err)
	}
	if text, err := os.ReadFile(root + "/dist/stage/lua/.counted.json"); err != nil || !strings.HasSuffix(string(text), "}\n") {
		t.Errorf("the file is %q, %v", text, err)
	}
}

func TestASourceMayDefineMacrosWhenItHoldsTheWordMacro(t *testing.T) {
	for text, want := range map[string]bool{
		"macro N = -> 1":                  true,
		"export macro N = -> 1":           true,
		"macro\tN = -> 1":                 true,
		"x = 1\n-- a macro, in a comment": true,
		"macro":                           true,
		"x = 1\nmacro":                    true,
		"(macro)":                         true,
		"$macro":                          true,
		"x.macro":                         true,
		"macro\r\n":                       true,
		"macros and a macro":              true,
		eAcute + "macro" + eAcute:         true,
		"":                                false,
		"macros":                          false,
		"mymacro":                         false,
		"macro_x":                         false,
		"_macro":                          false,
		"macro1":                          false,
		"1macro":                          false,
		"Macro N = -> 1":                  false,
		"MACRO":                           false,
		"mac ro":                          false,
		macroImport + "print $FourCC 'x'": false,
	} {
		if got := containsMacroWord(text); got != want {
			t.Errorf("holdsMacroWord(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestTheSourcesThatMayDefineMacrosAreAmongWhatEveryOutputDependsOn(t *testing.T) {
	member := func(p sourceTree) string {
		t.Helper()
		b := newCompileFixture(t, p)
		b.fakeCompiler(nil)
		result := b.mustCompile(fakeYue, false)
		kept, err := readCompileCache(b.root)
		if err != nil || kept.MacroSources != result.macroSources {
			t.Fatalf("the hashes file keeps %q and the compile returns %q, %v", kept.MacroSources, result.macroSources, err)
		}
		return kept.MacroSources
	}
	hashed := func(text string) string { return fsx.SHA256Hex([]byte(text)) }
	const one, edited, comment = "export macro N = -> 1\n", "export macro N = -> 2\n", "-- a macro\n"
	inLib := inLibrary("ex", "kit/m.yue")
	for _, c := range []struct {
		what string
		of   sourceTree
		want string
	}{
		{"no source with the word", mainOnly.withFiles("src/tools.yue", macroImport), ""},
		{"a Lua module with the word", mainOnly.withFiles("lua/x.lua", comment), ""},
		{"a file of src that is no module", mainOnly.withFiles("src/notes.txt", one, "src/x.lua", comment), ""},
		{"a source with the word", mainOnly.withFiles("src/m.yue", one), "src/m.yue\x00" + hashed(one) + "\n"},
		{"that source with another text", mainOnly.withFiles("src/m.yue", edited), "src/m.yue\x00" + hashed(edited) + "\n"},
		{"that source and another text beside it", newSourceTree("src/main.yue", "x = 2\n", "src/m.yue", one), "src/m.yue\x00" + hashed(one) + "\n"},
		{"two sources with the word", mainOnly.withFiles("src/m.yue", one, "src/a/b.yue", comment),
			"src/a/b.yue\x00" + hashed(comment) + "\nsrc/m.yue\x00" + hashed(one) + "\n"},
		{"a library's source with the word", mainOnly.withLibraries("ex").withFiles("src/m.yue", one, inLib, comment, inLibrary("ex", "kit/x.lua"), comment),
			inLib + "\x00" + hashed(comment) + "\nsrc/m.yue\x00" + hashed(one) + "\n"},
		{"a source with the word after a byte order mark", mainOnly.withFiles("src/m.yue", mark+one), "src/m.yue\x00" + hashed(mark+one) + "\n"},
	} {
		want := ""
		if c.want != "" {
			want = hashed(c.want)
		}
		if got := member(c.of); got != want {
			t.Errorf("%s: the sources that may define macros hash to %q, want %q", c.what, got, want)
		}
	}
}

func TestAnEditOfASourceThatMayDefineMacrosCompilesEverySourceAgain(t *testing.T) {
	b := newCompileFixture(t, newSourceTree("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n", "src/m.yue", "export macro N = -> 1\n"))
	b.fakeCompiler(nil)
	b.mustCompile(fakeYue, false)
	b.ranSources()
	all := []string{"src/a.yue", "src/b.yue", "src/m.yue"}
	for _, c := range []struct {
		what, path, text string
		want             []string
	}{
		{"an edit of a source without the word", "src/a.yue", "x = 2\n", []string{"src/a.yue"}},
		{"an edit of the source with the word", "src/m.yue", "export macro N = -> 2\n", all},
		{"the word in a source that had none", "src/b.yue", "y = 2 -- no macro\n", all},
		{"that source without the word", "src/b.yue", "y = 3\n", all},
		{"the word gone from the last source that has it", "src/m.yue", "z = 3\n", all},
		{"an edit of that source", "src/m.yue", "z = 4\n", []string{"src/m.yue"}},
	} {
		b.writeFile(c.path, c.text)
		b.mustCompile(fakeYue, false)
		if ran := b.ranSources(); !slices.Equal(ran, c.want) {
			t.Errorf("after %s the compiler ran on %q, want %q", c.what, ran, c.want)
		}
	}
}
