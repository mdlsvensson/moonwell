package script

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// These tests run no compiler: the compile is given one that is a function (bench.fake), and the listing of
// the globals a source uses one that prints what the test says (listing, in unknown_test.go).

// hashesText is what the hashes file holds; "" when there is none.
func (b *bench) hashesText() string {
	text, err := os.ReadFile(b.staged(".hashes.json"))
	if err != nil {
		return ""
	}
	return string(text)
}

func TestTheHashesFileHoldsWhatTheOutputsDependOnAndEachSourcesHash(t *testing.T) {
	loud := inLibrary("ex", "kit/loud.yue")
	b := benchOf(t, files("src/main.yue", "x = 1\n", "src/util/math.yue", "y = 2\n").with("ex").and(loud, "z = 3\n"))
	b.fake(nil)
	b.compiles(fakeYue, false)
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
	if got := b.hashesText(); got != want {
		t.Errorf(".hashes.json is\n%s\nwant\n%s", got, want)
	}
	b.compiles(fakeYue, true)
	if got, wantMinified := b.hashesText(), strings.Replace(want, `"-r"`, `"-m"`, 1); got != wantMinified {
		t.Errorf("minified, .hashes.json is\n%s\nwant\n%s", got, wantMinified)
	}
}

func TestEverySourceIsCompiledAgainWhenTheCompilerTheModeOrTheMacroModuleChanged(t *testing.T) {
	b := benchOf(t, files("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n"))
	b.fake(nil)
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
		b.search.hash = step.macros
		b.compiles(step.yue, step.minify)
		if ran := b.ran(); !slices.Equal(ran, step.want) {
			t.Errorf("%s: the compiler ran on %q, want %q", step.what, ran, step.want)
		}
	}
}

func TestASourceIsCompiledAgainWhenItChangedOrItsOutputIsGone(t *testing.T) {
	b := benchOf(t, files("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n", "src/c.yue", "z = 3\n"))
	b.fake(nil)
	b.compiles(fakeYue, false)
	b.ran()

	b.remove("dist/stage/lua/b.lua")
	b.compiles(fakeYue, false)
	if ran := b.ran(); !slices.Equal(ran, []string{"src/b.yue"}) || !fsx.Exists(b.staged("b.lua")) {
		t.Errorf("with the output of b.yue gone, the compiler ran on %q", ran)
	}
	b.write("src/c.yue", "z = 4\n")
	b.compiles(fakeYue, false)
	if ran := b.ran(); !slices.Equal(ran, []string{"src/c.yue"}) {
		t.Errorf("with c.yue changed, the compiler ran on %q", ran)
	}
	// A change that is undone is a change too: the output is of the text between.
	b.write("src/c.yue", "z = 3\n")
	b.compiles(fakeYue, false)
	if ran := b.ran(); !slices.Equal(ran, []string{"src/c.yue"}) {
		t.Errorf("with c.yue as it was at first, the compiler ran on %q", ran)
	}
}

func TestTheHashesFileIsWrittenWhenSomeFilesFailedHoldingThoseThatCompiled(t *testing.T) {
	b := benchOf(t, files("src/a.yue", "x = 1\n", "src/bad.yue", "x = \n", "src/c.yue", "z = 3\n"))
	b.fake(map[string]answer{"src/bad.yue": {code: 1, stdout: "Failed to compile: bad.yue\n1: boom\n"}})
	b.refuses(fakeYue, false, "a failed file")
	kept, err := readHashes(b.root)
	if paths := slices.Sorted(maps.Keys(kept.Sources)); err != nil || !slices.Equal(paths, []string{"src/a.yue", "src/c.yue"}) {
		t.Errorf("the hashes file keeps %q, %v", paths, err)
	}
	// So the next compile is of the failed file alone.
	b.ran()
	b.fake(nil)
	b.compiles(fakeYue, false)
	if ran := b.ran(); !slices.Equal(ran, []string{"src/bad.yue"}) {
		t.Errorf("the next compile ran on %q", ran)
	}
}

func TestTheOutputOfASourceThatIsGoneIsRemoved(t *testing.T) {
	loud := inLibrary("ex", "kit/loud.yue")
	b := benchOf(t, files("src/main.yue", "x = 1\n", "src/util/math.yue", "y = 2\n", "src/gone.yue", "z = 3\n").with("ex").and(loud, "w = 4\n"))
	b.fake(nil)
	b.compiles(fakeYue, false)
	outputs := []string{"main.lua", "util/math.lua", "gone.lua", ".libraries/ex/kit/loud.lua"}
	there := func() (found []string) {
		for _, output := range outputs {
			if fsx.Exists(b.staged(output)) {
				found = append(found, output)
			}
		}
		return found
	}
	if !slices.Equal(there(), outputs) {
		t.Fatalf("the first compile left %q", there())
	}
	b.ran()

	// A source that is gone, one in a folder, and one whose output is gone already.
	b.remove("src/util/math.yue")
	b.remove("src/gone.yue")
	b.remove("dist/stage/lua/gone.lua")
	b.compiles(fakeYue, false)
	if ran, left := b.ran(), there(); len(ran) != 0 || !slices.Equal(left, []string{"main.lua", ".libraries/ex/kit/loud.lua"}) {
		t.Errorf("with two sources gone, the compiler ran on %q and the outputs are %q", ran, left)
	}

	// A library of another key in the same folder: its module compiles below the new key, and the output below
	// the key that is gone is removed.
	b.libraries = []Library{{Key: "renamed", Dir: librariesDir + "/ex"}}
	outputs = append(outputs, ".libraries/renamed/kit/loud.lua")
	b.compiles(fakeYue, false)
	if ran, left := b.ran(), there(); !slices.Equal(ran, []string{loud}) || !slices.Equal(left, []string{"main.lua", ".libraries/renamed/kit/loud.lua"}) {
		t.Errorf("with the library's key changed, the compiler ran on %q and the outputs are %q", ran, left)
	}

	// A library that is gone.
	b.libraries = nil
	b.compiles(fakeYue, false)
	if ran, left := b.ran(), there(); len(ran) != 0 || !slices.Equal(left, []string{"main.lua"}) {
		t.Errorf("with the library gone, the compiler ran on %q and the outputs are %q", ran, left)
	}
}

func TestAHashesFileInAnotherShapeCountsAsAbsent(t *testing.T) {
	// The file of a compile of a.yue and b.yue, which the cases are made of.
	first := benchOf(t, files("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n"))
	first.fake(nil)
	first.compiles(fakeYue, false)
	good := first.hashesText()
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
		b := benchOf(t, files("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n"))
		b.fake(nil)
		b.compiles(fakeYue, false)
		b.ran()
		b.write("dist/stage/lua/.hashes.json", text)
		b.remove("src/b.yue")
		b.compiles(fakeYue, false)
		// Every source is compiled again, and no output is removed: the file says nothing of the outputs.
		if ran := b.ran(); !slices.Equal(ran, []string{"src/a.yue"}) || !fsx.Exists(b.staged("b.lua")) {
			t.Errorf("%s: the compiler ran on %q, and b.lua is there: %v", what, ran, fsx.Exists(b.staged("b.lua")))
		}
		kept, err := readHashes(b.root)
		want := map[string]keptSource{"src/a.yue": {Hash: hashOfA, Output: "a.lua"}}
		if err != nil || !reflect.DeepEqual(kept.Sources, want) || kept.Compiler != fakeYue {
			t.Errorf("%s: the hashes file written over it keeps %+v, %v", what, kept, err)
		}
	}
	// The file as it was written is trusted: nothing is compiled, and the output of the source that is gone is
	// removed.
	b := benchOf(t, files("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n"))
	b.fake(nil)
	b.compiles(fakeYue, false)
	b.ran()
	b.write("dist/stage/lua/.hashes.json", good)
	b.remove("src/b.yue")
	b.compiles(fakeYue, false)
	if ran := b.ran(); len(ran) != 0 || fsx.Exists(b.staged("b.lua")) {
		t.Errorf("with the file as it was written, the compiler ran on %q, and b.lua is there: %v", ran, fsx.Exists(b.staged("b.lua")))
	}
}

// stopAt gives the bench a compiler that is a function and that a run on one source stops: that run ends with
// an error that is no failure of a compile, and leaves nothing. Every other run ends well and leaves its Lua.
func (b *bench) stopAt(source string, stopped error) {
	b.use(func(ctx context.Context, program string, args []string, options env.RunOptions) (env.RunResult, error) {
		if b.sourceOf(args) == source {
			return env.RunResult{}, stopped
		}
		return env.RunResult{}, os.WriteFile(outputIn(args), []byte("-- "+b.sourceOf(args)+"\n"), 0o666)
	})
}

func TestAStoppedRunLeavesNothingItWasToCompileUpToDate(t *testing.T) {
	stopped := errors.New("stopped")
	b := benchOf(t, files("src/a.yue", "x = 1\n", "src/kept.yue", "y = 2\n", "src/stop.yue", "z = 3\n"))
	b.fake(nil)
	b.compiles(fakeYue, false)
	b.ran()

	// An edit, a new file, and a run that is stopped once it has compiled both.
	b.write("src/a.yue", "x = 2\n")
	b.write("src/new.yue", "w = 5\n")
	b.write("src/stop.yue", "z = 4\n")
	b.stopAt("src/stop.yue", stopped)
	if result, err := b.compile(fakeYue, false); result != nil || err != stopped {
		t.Fatalf("the stopped run: %+v, %v", result, err)
	}
	if ran := b.ran(); !slices.Equal(ran, []string{"src/a.yue", "src/new.yue", "src/stop.yue"}) || !fsx.Exists(b.staged("new.lua")) {
		t.Fatalf("the stopped run ran the compiler on %q, and new.lua is there: %v", ran, fsx.Exists(b.staged("new.lua")))
	}
	// The hashes file vouches for the source the run did not touch, and for no other: those it was to compile
	// are kept with their outputs and without a hash.
	kept, err := readHashes(b.root)
	want := map[string]keptSource{
		"src/a.yue": {Output: "a.lua"}, "src/new.yue": {Output: "new.lua"}, "src/stop.yue": {Output: "stop.lua"},
		"src/kept.yue": {Hash: fsx.SHA256Hex([]byte("y = 2\n")), Output: "kept.lua"},
	}
	if err != nil || !reflect.DeepEqual(kept.Sources, want) {
		t.Errorf("after the stopped run the hashes file keeps %+v, %v, want %+v", kept.Sources, err, want)
	}

	// The edit undone and the new file deleted: the Lua at a.lua is of the edit, so a.yue is compiled again, and
	// the output of the file that is gone is removed.
	b.write("src/a.yue", "x = 1\n")
	b.remove("src/new.yue")
	b.fake(nil)
	b.compiles(fakeYue, false)
	if ran := b.ran(); !slices.Equal(ran, []string{"src/a.yue", "src/stop.yue"}) || fsx.Exists(b.staged("new.lua")) {
		t.Errorf("after the stopped run the compiler ran on %q, and new.lua is there: %v", ran, fsx.Exists(b.staged("new.lua")))
	}
	b.compiles(fakeYue, false)
	if ran := b.ran(); len(ran) != 0 {
		t.Errorf("with nothing changed after that, the compiler ran on %q", ran)
	}
}

func TestAStoppedRunInAnotherModeLeavesNothingUpToDate(t *testing.T) {
	stopped := errors.New("stopped")
	b := benchOf(t, files("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n"))
	b.fake(nil)
	b.compiles(fakeYue, false)
	b.ran()
	// A minified run that is stopped once a.lua is minified.
	b.stopAt("src/b.yue", stopped)
	if _, err := b.compile(fakeYue, true); err != stopped {
		t.Fatalf("the stopped run: %v", err)
	}
	b.ran()
	b.fake(nil)
	b.compiles(fakeYue, false)
	if ran := b.ran(); !slices.Equal(ran, []string{"src/a.yue", "src/b.yue"}) {
		t.Errorf("normal again after the stopped minified run, the compiler ran on %q", ran)
	}
}

func TestAProjectWithoutYueScriptCompilesNothingAndKeepsNoSources(t *testing.T) {
	b := benchOf(t, files("src/notes.txt", "", "lua/tools.lua", "return {}\n"))
	result := b.compiles(fakeYue, false)
	if len(result.texts) != 0 || len(result.hashes) != 0 || len(result.lua) != 0 || result.texts == nil {
		t.Errorf("compileAll = %+v, want nothing compiled", result)
	}
	if text := b.hashesText(); !strings.Contains(text, `"sources": {}`) {
		t.Errorf(".hashes.json is\n%s", text)
	}
}

func TestALinkOnTheWayToTheHashesFileIsRefused(t *testing.T) {
	for _, link := range []string{"dist", "dist/stage", "dist/stage/lua"} {
		b := benchOf(t, files("src/notes.txt", ""))
		at := linkTo(t, files(".hashes.json", "{}"), b.root, link)
		_, err := b.compile(fakeYue, false)
		if failure := asError(t, err, "a link at "+link); failure.Msg != "Symlinks are not supported: "+at ||
			failure.File != "dist/stage/lua" {
			t.Errorf("a link at %s: %+v", link, failure)
		}
	}
}

func TestAHashesFileThatCannotBeWrittenIsRefusedByItsPath(t *testing.T) {
	b := benchOf(t, mainOnly.and("dist/stage/lua/.hashes.json/kept.txt", ""))
	b.fake(nil)
	failure := b.refuses(fakeYue, false, "a folder for the hashes file")
	if !strings.HasPrefix(failure.Msg, "Writing dist/stage/lua/.hashes.json failed: ") || failure.File != "dist/stage/lua/.hashes.json" ||
		!strings.Contains(failure.Hint, "dist/") || failure.Cause == nil {
		t.Errorf("error = %+v", failure)
	}
}

func TestACacheFileIsReadBackAsItWasWrittenAndNotAsAnotherShape(t *testing.T) {
	type counted struct {
		Words map[string]int `json:"words"`
		Of    string         `json:"of"`
	}
	root := t.TempDir()
	if kept, found, err := readCache[counted](root, ".counted.json"); found || err != nil || kept.Words != nil {
		t.Errorf("without a file: %+v, %v, %v", kept, found, err)
	}
	want := counted{Words: map[string]int{"a<b>&c": 2, eAcute + beyond: 1}, Of: `C:\a "b"`}
	if err := writeCache(root, ".counted.json", want); err != nil {
		t.Fatal(err)
	}
	if kept, found, err := readCache[counted](root, ".counted.json"); !found || err != nil || !reflect.DeepEqual(kept, want) {
		t.Errorf("read back: %+v, %v, %v, want %+v", kept, found, err, want)
	}
	// Each file has its own shape, and is nothing in the shape of the other.
	if kept, found, err := readCache[hashes](root, ".counted.json"); found || err != nil || kept.Sources != nil {
		t.Errorf("read as hashes: %+v, %v, %v", kept, found, err)
	}
	if text, err := os.ReadFile(root + "/dist/stage/lua/.counted.json"); err != nil || !strings.HasSuffix(string(text), "}\n") {
		t.Errorf("the file is %q, %v", text, err)
	}
}

// ---- the sources that may define macros ----

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
		eAcute + "macro" + eAcute:         true, // only a letter, a digit or "_" of ASCII joins the word
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
		if got := holdsMacroWord(text); got != want {
			t.Errorf("holdsMacroWord(%q) = %v, want %v", text, got, want)
		}
	}
}

func TestTheSourcesThatMayDefineMacrosAreAmongWhatEveryOutputDependsOn(t *testing.T) {
	// member is what a compile of a project keeps, and returns, for the sources that may define macros.
	member := func(p project) string {
		t.Helper()
		b := benchOf(t, p)
		b.fake(nil)
		result := b.compiles(fakeYue, false)
		kept, err := readHashes(b.root)
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
		of   project
		want string // the bytes that are hashed; "" for no source with the word
	}{
		{"no source with the word", mainOnly.and("src/tools.yue", macroImport), ""},
		{"a Lua module with the word", mainOnly.and("lua/x.lua", comment), ""},
		{"a file of src that is no module", mainOnly.and("src/notes.txt", one, "src/x.lua", comment), ""},
		{"a source with the word", mainOnly.and("src/m.yue", one), "src/m.yue\x00" + hashed(one) + "\n"},
		{"that source with another text", mainOnly.and("src/m.yue", edited), "src/m.yue\x00" + hashed(edited) + "\n"},
		{"that source and another text beside it", files("src/main.yue", "x = 2\n", "src/m.yue", one), "src/m.yue\x00" + hashed(one) + "\n"},
		{"two sources with the word", mainOnly.and("src/m.yue", one, "src/a/b.yue", comment),
			"src/a/b.yue\x00" + hashed(comment) + "\nsrc/m.yue\x00" + hashed(one) + "\n"},
		// In the order of the paths' bytes, which puts a library's before the project's own.
		{"a library's source with the word", mainOnly.with("ex").and("src/m.yue", one, inLib, comment, inLibrary("ex", "kit/x.lua"), comment),
			inLib + "\x00" + hashed(comment) + "\nsrc/m.yue\x00" + hashed(one) + "\n"},
		{"a source with the word after a byte order mark", mainOnly.and("src/m.yue", mark+one), "src/m.yue\x00" + hashed(mark+one) + "\n"},
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
	b := benchOf(t, files("src/a.yue", "x = 1\n", "src/b.yue", "y = 2\n", "src/m.yue", "export macro N = -> 1\n"))
	b.fake(nil)
	b.compiles(fakeYue, false)
	b.ran()
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
		b.write(c.path, c.text)
		b.compiles(fakeYue, false)
		if ran := b.ran(); !slices.Equal(ran, c.want) {
			t.Errorf("after %s the compiler ran on %q, want %q", c.what, ran, c.want)
		}
	}
}

// ---- the globals each source uses ----

// usesBench is a project folder without files, a compiler that is a listing, and what a listing of uses is
// given beside its sources: the macro search, and the hash of the sources that may define macros.
type usesBench struct {
	t            *testing.T
	root         string
	world        *env.Env
	yue          *listing
	search       macros
	macroSources string
}

// usesBenchOf is a bench whose compiler prints what printed holds for each source, by its path.
func usesBenchOf(t *testing.T, printed map[string]env.RunResult) *usesBench {
	t.Helper()
	root := t.TempDir()
	b := &usesBench{t: t, root: root, yue: &listing{t: t, root: root, printed: printed}, macroSources: "s1"}
	b.world, _ = testkit.Env(t, root)
	b.world.Run = b.yue.run
	b.search = macros{path: filepath.Join(root, ".moonwell", "yue", "?.lua"), hash: "m1"}
	return b
}

// list lists the uses of sources, given as pairs of a path and a hash, with the compiler named yue.
func (b *usesBench) list(yue string, pairs ...string) (map[string][]globalUse, error) {
	var sources []checked
	for i := 0; i+1 < len(pairs); i += 2 {
		sources = append(sources, checked{path: pairs[i], hash: pairs[i+1]})
	}
	return listUses(background, b.world, yue, b.search, b.macroSources, sources)
}

// usesText is what the uses file holds; "" when there is none.
func (b *usesBench) usesText() string {
	text, err := os.ReadFile(filepath.Join(b.root, "dist", "stage", "lua", ".globals.json"))
	if err != nil {
		return ""
	}
	return string(text)
}

// sameUses reports whether two results of a listing hold the same uses for the same sources.
func sameUses(got, want map[string][]globalUse) bool { return maps.EqualFunc(got, want, slices.Equal) }

func TestTheCompilerListsEachChangedSourceOnceAndItsUsesAreKeptByItsHash(t *testing.T) {
	const main, captain = "src/main.yue", "src/heroes/captain.yue"
	b := usesBenchOf(t, map[string]env.RunResult{main: prints("print 1 1\n"), captain: prints("CreatUnit 3 5\nprint 4 1\n")})
	expected := map[string][]globalUse{
		captain: {{Name: "CreatUnit", Line: 3, Column: 5}, {Name: "print", Line: 4, Column: 1}},
		main:    {{Name: "print", Line: 1, Column: 1}},
	}
	both := []string{captain, main}

	uses, err := b.list("yue", main, "h1", captain, "h2")
	if err != nil || !sameUses(uses, expected) || !slices.Equal(b.yue.ran(), both) {
		t.Fatalf("listUses = %+v, %v", uses, err)
	}
	want := `{
  "compiler": "yue",
  "macros": "m1",
  "macroSources": "s1",
  "sources": {
    "src/heroes/captain.yue": {
      "hash": "h2",
      "uses": [
        {
          "name": "CreatUnit",
          "line": 3,
          "column": 5
        },
        {
          "name": "print",
          "line": 4,
          "column": 1
        }
      ]
    },
    "src/main.yue": {
      "hash": "h1",
      "uses": [
        {
          "name": "print",
          "line": 1,
          "column": 1
        }
      ]
    }
  }
}
`
	if got := b.usesText(); got != want {
		t.Errorf(".globals.json is\n%s\nwant\n%s", got, want)
	}

	uses, err = b.list("yue", main, "h1", captain, "h2")
	if ran := b.yue.ran(); err != nil || !sameUses(uses, expected) || len(ran) != 0 {
		t.Errorf("unchanged: %+v, %v after listing %q", uses, err, ran)
	}

	// An edited source is listed again, and a source that uses no global has a list without uses.
	b.yue.printed[main] = prints("")
	uses, err = b.list("yue", main, "h3", captain, "h2")
	if ran := b.yue.ran(); err != nil || !slices.Equal(ran, []string{main}) || uses[main] == nil || len(uses[main]) != 0 || len(uses) != 2 {
		t.Errorf("after an edit: %+v, %v after listing %q", uses, err, ran)
	}
	if text := b.usesText(); !strings.Contains(text, "\"hash\": \"h3\",\n      \"uses\": []\n") {
		t.Errorf("after an edit, .globals.json is\n%s", text)
	}
	uses, err = b.list("yue", main, "h3", captain, "h2")
	if ran := b.yue.ran(); err != nil || len(ran) != 0 || uses[main] == nil || len(uses[main]) != 0 {
		t.Errorf("a source without uses, unchanged: %+v, %v after listing %q", uses, err, ran)
	}

	// Another compiler lists every source again.
	if _, err := b.list("other-yue", main, "h3", captain, "h2"); err != nil || !slices.Equal(b.yue.ran(), both) {
		t.Errorf("another compiler: %v", err)
	}

	// A source that is listed no more leaves the file.
	if _, err := b.list("other-yue", main, "h3"); err != nil || len(b.yue.ran()) != 0 {
		t.Fatalf("one source of the two: %v", err)
	}
	kept, err := readUses(b.root, listedWith{Compiler: "other-yue", Macros: "m1", MacroSources: "s1"})
	if err != nil || len(kept) != 1 || kept[main].Hash != "h3" {
		t.Errorf("the uses file keeps %+v, %v", kept, err)
	}
}

func TestAUsesFileInAnotherShapeCountsAsAbsent(t *testing.T) {
	const with = `{"compiler": "yue", "macros": "m1", "macroSources": "s1", `
	const good = with + `"sources": {"src/main.yue": {"hash": "h1", "uses": [{"name": "print", "line": 1, "column": 1}]}}}`
	want := map[string][]globalUse{"src/main.yue": {{Name: "Zzz", Line: 1, Column: 1}}}
	for what, text := range map[string]string{
		"the shape of another version": `{"settings": "yue|m1", "files": {"main.yue": {"hash": "h1", "uses": [["print", 1, 1]]}}}`,
		"no JSON":                      "not json",
		"an empty file":                "",
		"null":                         "null",
		"a member it has not":          strings.Replace(good, `"macros"`, `"mode": "-r", "macros"`, 1),
		"text after it":                good + "x",
		"sources that are a list":      with + `"sources": []}`,
		"a source without uses":        with + `"sources": {"src/main.yue": {"hash": "h1"}}}`,
		"uses that are null":           with + `"sources": {"src/main.yue": {"hash": "h1", "uses": null}}}`,
		"a use that is a list":         strings.Replace(good, `{"name": "print", "line": 1, "column": 1}`, `["print", 1, 1]`, 1),
		"a line that is a string":      strings.Replace(good, `"line": 1`, `"line": "1"`, 1),
		"a line that is no integer":    strings.Replace(good, `"line": 1`, `"line": 1.5`, 1),
		"a use without a name":         strings.Replace(good, `"name": "print", `, ``, 1),
		"a use with a member it lacks": strings.Replace(good, `"column": 1`, `"column": 1, "end": 2`, 1),
		"another compiler":             strings.Replace(good, `"compiler": "yue"`, `"compiler": "another"`, 1),
		"another macro module":         strings.Replace(good, `"macros": "m1"`, `"macros": "m2"`, 1),
		"other sources with macros":    strings.Replace(good, `"macroSources": "s1"`, `"macroSources": "s2"`, 1),
		"no sources with macros":       strings.Replace(good, `"macroSources": "s1", `, ``, 1),
		"another hash":                 strings.Replace(good, `"hash": "h1"`, `"hash": "h0"`, 1),
		"another source":               strings.Replace(good, `"src/main.yue"`, `"src/other.yue"`, 1),
	} {
		b := usesBenchOf(t, map[string]env.RunResult{"src/main.yue": prints("Zzz 1 1\n")})
		testkit.WriteFile(t, b.root, "dist/stage/lua/.globals.json", []byte(text))
		uses, err := b.list("yue", "src/main.yue", "h1")
		if ran := b.yue.ran(); err != nil || !sameUses(uses, want) || !slices.Equal(ran, []string{"src/main.yue"}) {
			t.Errorf("%s: %+v, %v after listing %q", what, uses, err, ran)
		}
	}
	// The file as a listing writes it is trusted: the compiler is not run.
	b := usesBenchOf(t, map[string]env.RunResult{"src/main.yue": prints("Zzz 1 1\n")})
	testkit.WriteFile(t, b.root, "dist/stage/lua/.globals.json", []byte(good))
	uses, err := b.list("yue", "src/main.yue", "h1")
	if ran := b.yue.ran(); err != nil || !sameUses(uses, map[string][]globalUse{"src/main.yue": {{Name: "print", Line: 1, Column: 1}}}) || len(ran) != 0 {
		t.Errorf("the file as it is written: %+v, %v after listing %q", uses, err, ran)
	}
}

func TestAFailedListingIsReportedLikeAFileThatFailedToCompile(t *testing.T) {
	failed := env.RunResult{Code: 1, Stdout: "Failed to compile: main.yue\n2: unexpected expression\n"}
	b := usesBenchOf(t, map[string]env.RunResult{"src/main.yue": failed})
	uses, err := b.list("yue", "src/main.yue", "h1")
	failure := asError(t, err, "a failed run")
	if uses != nil || failure.Msg != "unexpected expression\n2: unexpected expression" || failure.File != "src/main.yue" || failure.Line != 2 {
		t.Errorf("listUses = %+v, %+v", uses, failure)
	}
	// What the run printed on its other stream is read too.
	b = usesBenchOf(t, map[string]env.RunResult{"src/main.yue": {Code: 1, Stderr: "7: on the error stream\n"}})
	_, err = b.list("yue", "src/main.yue", "h1")
	if failure := asError(t, err, "a failed run"); failure.Line != 7 || failure.File != "src/main.yue" {
		t.Errorf("error = %+v", failure)
	}
}

func TestOutputThatCannotBeReadIsReportedAndTheOtherSourcesAreStillKept(t *testing.T) {
	const bad, good, worse = "src/bad.yue", "src/good.yue", "src/Worse.yue"
	b := usesBenchOf(t, map[string]env.RunResult{bad: prints("Score one 8\n"), good: prints("print 1 1\n"), worse: prints("x\n")})
	_, err := b.list("yue", bad, "h1", good, "h2", worse, "h3")
	// Of two failures the one whose file is first by bytes is reported: a capital letter before every small one.
	if failure := asError(t, err, "unreadable output"); failure.Msg != "yue -g printed a line Moonwell cannot read: x" || failure.File != worse {
		t.Errorf("error = %+v", failure)
	}
	if ran := b.yue.ran(); !slices.Equal(ran, []string{worse, bad, good}) {
		t.Errorf("the compiler listed %q", ran)
	}

	b.yue.printed[bad], b.yue.printed[worse] = prints("Score 1 8\n"), prints("")
	uses, err := b.list("yue", bad, "h1", good, "h2", worse, "h3")
	want := map[string][]globalUse{
		bad:   {{Name: "Score", Line: 1, Column: 8}},
		good:  {{Name: "print", Line: 1, Column: 1}},
		worse: {},
	}
	if ran := b.yue.ran(); err != nil || !sameUses(uses, want) || !slices.Equal(ran, []string{worse, bad}) {
		t.Errorf("listUses = %+v, %v after listing %q", uses, err, ran)
	}
}

func TestTheCompilerIsGivenTheMacroPathAndTheUsesDependOnTheMacroModule(t *testing.T) {
	b := usesBenchOf(t, map[string]env.RunResult{"src/main.yue": prints("print 1 1\n")})
	list := func() {
		t.Helper()
		if _, err := b.list("yue", "src/main.yue", "h1"); err != nil {
			t.Fatal(err)
		}
	}
	list()
	want := []string{"-g", "--path", b.search.path, filepath.Join(b.root, "src", "main.yue")}
	if len(b.yue.runs) != 1 || !slices.Equal(b.yue.runs[0], want) {
		t.Errorf("the compiler was run with %q, want once with %q", b.yue.runs, want)
	}
	list()
	if len(b.yue.runs) != 1 {
		t.Error("unchanged, but listed again")
	}
	b.search.hash = "m2"
	list()
	if len(b.yue.runs) != 2 {
		t.Error("a changed macro module lists every file again")
	}
	// So do other sources that may define macros: the macros a source uses may be theirs.
	b.macroSources = "s2"
	list()
	list()
	if len(b.yue.runs) != 3 {
		t.Errorf("after a change of the sources that may define macros, the compiler ran %d times in all, want 3", len(b.yue.runs))
	}
}

func TestAtMostEightSourcesAreListedAtATime(t *testing.T) {
	var pairs []string
	for i := range 40 {
		pairs = append(pairs, fmt.Sprintf("src/m%02d.yue", i), "h")
	}
	b := usesBenchOf(t, nil)
	var guard sync.Mutex
	running, most := 0, 0
	var once sync.Once
	eightAreIn := make(chan struct{})
	// A listing that never lets eight in is told apart by the count, after a wait that a working one never
	// spends.
	waited, giveUp := context.WithTimeout(background, 5*time.Second)
	defer giveUp()
	b.world.Run = func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
		guard.Lock()
		running++
		most = max(most, running)
		if running == 8 {
			once.Do(func() { close(eightAreIn) })
		}
		guard.Unlock()
		// Each of the first runs stays in until eight are in at once, and every run a moment longer, in which a
		// ninth would come in if it were let.
		select {
		case <-eightAreIn:
		case <-waited.Done():
		}
		time.Sleep(2 * time.Millisecond)
		guard.Lock()
		running--
		guard.Unlock()
		return prints("print 1 1\n"), nil
	}
	uses, err := b.list("yue", pairs...)
	if err != nil || most != 8 || running != 0 || len(uses) != 40 {
		t.Errorf("at most %d sources were listed at a time, %d are still listed, and %d have their uses: %v", most, running, len(uses), err)
	}
}

func TestAListingThatIsStoppedPassesTheErrorOnAndKeepsNothingNew(t *testing.T) {
	b := usesBenchOf(t, map[string]env.RunResult{"src/a.yue": prints("print 1 1\n"), "src/b.yue": prints("print 2 2\n")})
	if _, err := b.list("yue", "src/a.yue", "h1"); err != nil {
		t.Fatal(err)
	}
	before := b.usesText()
	stopped := errors.New("stopped")
	b.world.Run = func(context.Context, string, []string, env.RunOptions) (env.RunResult, error) {
		return env.RunResult{}, stopped
	}
	uses, err := b.list("yue", "src/a.yue", "h1", "src/b.yue", "h2")
	// The error is the one that came, and the file is as the last listing left it.
	if uses != nil || err != stopped || b.usesText() != before || before == "" {
		t.Errorf("listUses = %+v, %v; .globals.json is\n%s\nand was\n%s", uses, err, b.usesText(), before)
	}
}

func TestAUsesFileThatCannotBeReadOrWrittenIsRefusedByItsPath(t *testing.T) {
	b := usesBenchOf(t, map[string]env.RunResult{"src/main.yue": prints("print 1 1\n")})
	testkit.WriteFile(t, b.root, "dist/stage/lua/.globals.json/kept.txt", nil)
	_, err := b.list("yue", "src/main.yue", "h1")
	failure := asError(t, err, "a folder for the uses file")
	if !strings.HasPrefix(failure.Msg, "Writing dist/stage/lua/.globals.json failed: ") || failure.File != "dist/stage/lua/.globals.json" ||
		!strings.Contains(failure.Hint, "dist/") || failure.Cause == nil {
		t.Errorf("error = %+v", failure)
	}
	// A link on the way to the file is refused before the compiler runs.
	b = usesBenchOf(t, nil)
	at := linkTo(t, files(".globals.json", "{}"), b.root, "dist/stage")
	_, err = b.list("yue", "src/main.yue", "h1")
	if failure := asError(t, err, "a link at dist/stage"); failure.Msg != "Symlinks are not supported: "+at ||
		failure.File != "dist/stage/lua/.globals.json" || len(b.yue.ran()) != 0 {
		t.Errorf("a link at dist/stage: %+v", failure)
	}
}
