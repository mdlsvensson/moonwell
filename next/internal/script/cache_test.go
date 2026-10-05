package script

import (
	"maps"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// These tests run no compiler: the compile is given one that is a function (bench.fake).

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
		"an output with a backslash":   strings.Replace(good, `"output": "b.lua"`, `"output": "sub\\b.lua"`, 1),
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
		if failure := asError(t, err, "a link at "+link); failure.Msg != "Symlinks are not supported: "+at {
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
