package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/objects"
)

// The tests of the oracle itself: what oracle_test.go says of its classes, its places and its guard, each held on
// readings and on runs that break a rule.

// The readings that decide a class, each on lines and endings of its kind and of other kinds.
func TestOracleReadsTheLinesAndTheEndingsThatDecideItsClasses(t *testing.T) {
	root := filepath.Join(t.TempDir(), "checkout")
	file := filepath.Join(root, "data", "metadata.json")
	for _, c := range []struct {
		line, want string
		shaped     bool
	}{
		{"error: open " + file + ": the reason\n", "error: data/metadata.json: the reason\n", true},
		{"error: mkdir " + filepath.Join(root, "schema") + ": the reason\n", "error: schema: the reason\n", true},
		{"error: data/metadata.json: the reason\n", "", false},
		{"error: cannot open " + file + ": the reason\n", "", false},
		// The second form: a file that the decoder refused, without an operation.
		{"error: " + file + ": the reason\n", "error: data/metadata.json: the reason\n", true},
		{"error: in " + file + ": the reason\n", "error: data/metadata.json: the reason\n", true},
		{"error:  " + file + ": the reason\n", "", false},
		{"error: open" + file + ": the reason\n", "", false},
		{"error: " + file + "\n", "", false},
		{"open " + file + ": the reason\n", "", false},
		{"error: open " + file + "\n", "", false},
	} {
		if got, shaped := lineFromTheCheckout(c.line, root); got != c.want || shaped != c.shaped {
			t.Errorf("lineFromTheCheckout(%q) = %q, %v, want %q, %v", c.line, got, shaped, c.want, c.shaped)
		}
	}
	for _, c := range []struct {
		line, want string
		shaped     bool
	}{
		{"error: open list.txt: the reason\n", "error: list.txt: the reason\n", true},
		{"error: read list.txt: the reason\n", "error: list.txt: the reason\n", true},
		{"error: stat list.txt: the reason\n", "", false},
		{"error: open other.txt: the reason\n", "", false},
		{"error: list.txt: the reason\n", "", false},
	} {
		if got, shaped := lineWithoutTheOperation(c.line, "list.txt"); got != c.want || shaped != c.shaped {
			t.Errorf("lineWithoutTheOperation(%q) = %q, %v, want %q, %v", c.line, got, shaped, c.want, c.shaped)
		}
	}
	// The path that a run names: a file that the line names is the argument as it stands, and a file below a
	// folder that the line names is the two joined as the system joins them, without what a path need not have.
	script := func(steps ...string) string { return filepath.Join(append(steps, "scripts", "common.j")...) }
	natives := func(folder string) []string { return []string{"natives", folder, "1"} }
	for _, c := range []struct {
		n    named
		args []string
		in   string
		has  bool
	}{
		{named{1, ""}, []string{"game-paths", "lists//list.txt", "1"}, "lists//list.txt", true},
		{named{1, "scripts/common.j"}, natives(filepath.Join("an", "export")), script("an", "export"), true},
		{named{1, "scripts/common.j"}, natives("an/export/"), script("an", "export"), true},
		{named{1, "scripts/common.j"}, natives("an//export//"), script("an", "export"), true},
		{named{1, "scripts/common.j"}, natives(""), script(), true},
		{named{1, "scripts/common.j"}, natives("."), script(), true},
		{named{1, "scripts/common.j"}, natives(".."), script(".."), true},
		{named{1, "scripts/common.j"}, natives("an/../export"), script("export"), true},
		{named{3, "scripts/common.j"}, natives("export"), "", false},
	} {
		if in, has := c.n.in(c.args); in != c.in || has != c.has {
			t.Errorf("%+v of %q: a program makes %q, %v of it; want %q, %v", c.n, c.args, in, has, c.in, c.has)
		}
	}
	const stack = "panic: a function without a name\n\ngoroutine 1 [running]:\nmain.main()\n"
	for _, c := range []struct {
		made              outcome
		carried, panicked bool
	}{
		{outcome{}, true, false},
		{outcome{stdout: "wrote a file\n"}, true, false},
		{outcome{stderr: "a warning\n"}, false, false},
		{outcome{code: 1, stderr: "error: a refusal\n"}, false, false},
		{outcome{code: 2, stderr: stack}, false, true},
		{outcome{code: 1, stderr: stack}, false, false},
		{outcome{code: 2, stderr: "error: a refusal\n"}, false, false},
	} {
		if carried, ended := carriedOut(c.made), panicked(c.made); carried != c.carried || ended != c.panicked {
			t.Errorf("an end with %d and %q: carried out %v, panicked %v, want %v, %v",
				c.made.code, c.made.stderr, carried, ended, c.carried, c.panicked)
		}
	}
	const text = "one\ntwo and more\nthree"
	for _, c := range []struct {
		a, b   string
		at     int
		ofEach [2]string
	}{
		{text, text, len(text), [2]string{"three", "three"}},
		{text, "one\ntwo or less\nthree", 8, [2]string{"two and more", "two or less"}},
		{text, "one\n", 4, [2]string{"two and more", ""}},
		{strings.Repeat("a", 200) + "b\n", strings.Repeat("a", 200) + "c\n", 200,
			[2]string{strings.Repeat("a", 60) + "b", strings.Repeat("a", 60) + "c"}},
	} {
		at := partingOffset(c.a, c.b)
		if got := [2]string{lineAt(c.a, at), lineAt(c.b, at)}; at != c.at || got != c.ofEach {
			t.Errorf("%q and %q part at %d, with the lines %q; want %d and %q", c.a, c.b, at, got, c.at, c.ofEach)
		}
	}
	const written = "one\ntwo\nthree\ntwo and two\n"
	for _, c := range []struct {
		places  []place
		changed string
		wrong   string // words of what is wrong with the places; "" for places that are sound
	}{
		{[]place{{"one\n", "ONE\n"}}, "ONE\ntwo\nthree\ntwo and two\n", ""},
		{[]place{{"three\n", ""}, {"one\n", "1\n"}}, "1\ntwo\ntwo and two\n", ""},
		{[]place{{"\ntwo\n", "\ntwo\nmore\n"}}, "one\ntwo\nmore\nthree\ntwo and two\n", ""},
		{nil, "", "names no place"},
		{[]place{{"three", "three"}}, "", "alike"},
		{[]place{{"two", "2"}}, "", "3 times"},
		{[]place{{"four", "4"}}, "", "0 times"},
		{[]place{{"one\ntwo", "1"}, {"ne\ntwo\nthree", "2"}}, "", "lies in the place before it"},
	} {
		changed, wrong := withPlaces(written, c.places)
		if changed != c.changed || (wrong == "") != (c.wrong == "") || !strings.Contains(wrong, c.wrong) {
			t.Errorf("withPlaces(%q) = %q, %q; want %q and the words %q", c.places, changed, wrong, c.changed, c.wrong)
		}
	}
	// The path of AsGiven is the argument, or a file below the folder that the argument names; the predicate
	// asks that the input has no file there, and that the other tree's line is Go's own of that path.
	missing := filepath.Join(root, "export")
	below := filepath.Join(missing, "scripts", "common.j")
	line := []string{"natives", missing, "1"}
	goSaid, thisSaid := "error: open "+below+": the reason\n", "error: "+below+": the reason\n"
	for name, c := range map[string]struct {
		carried *named
		said    string // the other tree's standard error
		is      bool
	}{
		"a file below a folder that is not there":     {&named{1, "scripts/common.j"}, goSaid, true},
		"the folder itself, of which the line is not": {&named{1, ""}, goSaid, false},
		"another argument":                            {&named{2, "scripts/common.j"}, goSaid, false},
		"an argument the line has not":                {&named{3, ""}, goSaid, false},
		"nothing carried":                             {nil, goSaid, false},
		"a line of this tree's shape":                 {&named{1, "scripts/common.j"}, thisSaid, false},
	} {
		made := comparison{r: oracleRun{cannotGive: c.carried}, in: input{args: line}, want: outcome{stderr: c.said}}
		if is := cannotGiveWhatTheLineNames(made); is != c.is {
			t.Errorf("%s: the predicate of AsGiven is %v, want %v", name, is, c.is)
		}
	}
}

// The guard stands before every run: a run whose folder it refuses is carried out by no tree.
func TestOracleCarriesOutNoRunThatItsGuardRefuses(t *testing.T) {
	goMod := func(text string) func(checkout) { return func(c checkout) { c.write("go.mod", text) } }
	for name, c := range map[string]struct {
		r       oracleRun
		refused string // words of the refusal
	}{
		"a run in the folder above its checkout": {oracleRun{below: ".."}, "nor below it"},
		"a run of a checkout that has lost its go.mod": {
			oracleRun{lay: goMod(anotherModule)}, "names this module: false"},
		"a run of no checkout that is laid one": {
			oracleRun{noCheckout: true, lay: goMod(moduleFile)}, "names this module: true"},
	} {
		started := false
		heard := listenTo(t, func(tb testing.TB) {
			c.r.madeBy(tb, func(checkout) (int, string, string) {
				started = true
				return 0, "", ""
			})
		})
		if started || !strings.Contains(heard, c.refused) {
			t.Errorf("%s: carried out: %v; the guard said %q, want the words %q", name, started, heard, c.refused)
		}
	}
	started := false
	heard := listenTo(t, func(tb testing.TB) {
		oracleRun{below: "tools/gen"}.madeBy(tb, func(checkout) (int, string, string) {
			started = true
			return 0, "", ""
		})
	})
	if !started || heard != "" {
		t.Errorf("a run in a folder of its checkout: carried out: %v; the guard said %q, want nothing", started, heard)
	}
}

// A place of the oracle is a literal: no test file of the package converts a text to the type that the two
// texts of a place have, so what a place holds stands written in a file, where a reader sees it.
func TestAPlaceOfTheOracleIsALiteral(t *testing.T) {
	const converting = "package main\n\n" +
		"var p = place{other: \"a\", this: literal(made())}\n" +
		"var q = place{\"a\", (literal)(\"b\")}\n"
	if found := conversionsToLiteral(t, "converting.go", converting); len(found) != 2 {
		t.Fatalf("in a file with two conversions to literal the test finds %q", found)
	}
	files, err := filepath.Glob("*_test.go")
	if err != nil || !slices.Contains(files, "oracle_test.go") {
		t.Fatalf("the test files of the package are %q, and the oracle is not among them: %v", files, err)
	}
	for _, name := range files {
		source, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, at := range conversionsToLiteral(t, name, string(source)) {
			t.Errorf("%s converts a text to literal: a place holds what stands written in the file", at)
		}
	}
}

// conversionsToLiteral is where the source of a Go file converts a value to the type literal, each as the file,
// the line and the column.
func conversionsToLiteral(t testing.TB, name, source string) []string {
	t.Helper()
	positions := token.NewFileSet()
	file, err := parser.ParseFile(positions, name, source, parser.SkipObjectResolution)
	if err != nil {
		t.Fatal(err)
		return nil
	}
	var found []string
	ast.Inspect(file, func(node ast.Node) bool {
		if call, isCall := node.(*ast.CallExpr); isCall {
			if converted, isName := ast.Unparen(call.Fun).(*ast.Ident); isName && converted.Name == "literal" {
				found = append(found, positions.Position(call.Pos()).String())
			}
		}
		return true
	})
	return found
}

// The rules of a class, each on a run that breaks it: the oracle must report the run, in the words that stand
// beside it. The runs go through compare as every run does, the other tree's generator a program; the test
// builds it, and is skipped with -short.
func TestOracleReportsARunThatIsNotOfTheClassItNames(t *testing.T) {
	if testing.Short() {
		t.Skip("the test builds the other tree's generator and starts it for its runs: not with -short")
	}
	o := genOracle{other: builtProgram(t, "./tools/gen")}
	ordinary := gamePathsOf("Units/A.mdx\n", "2.0.0")
	noBreak := gamePathsOf("Units/A.mdx\n\xC2\xA0Units/B.mdx\n", "2.0.0")
	notThere := func(_ testing.TB, outside string) []string {
		return []string{"game-paths", filepath.Join(outside, "no-listfile.txt"), "2.0.0"}
	}
	// The reports of a run that is compared whole, and whose list or standard error the trees make apart.
	const listDiffers, errorDiffers = gamePathsPath + ": differs at offset", standardError + ": differs at offset"
	laidApart := 0
	for _, probe := range []struct {
		r       oracleRun
		reports []string
	}{
		{oracleRun{name: "a class whose predicate does not hold", class: "FromCheckout", lay: oneBuff("fnam", "name")},
			[]string{"names the class FromCheckout, whose predicate does not hold of it"}},
		{oracleRun{name: "a class that the oracle has not", class: "NoSuchClass", lay: oneBuff("fnam", "name")},
			[]string{`names the class "NoSuchClass", and the oracle has none of that name`}},
		{oracleRun{name: "a no-break space named as a dotted I", class: "DottedI", lay: noList, line: noBreak,
			apart: inTheListWritten("\nunits/b.mdx\n", "\n\xC2\xA0units/b.mdx\n")},
			[]string{"names the class DottedI, whose predicate does not hold of it", listDiffers}},
		{oracleRun{name: "a list without the difference of its class", class: "ByBytes", lay: noList, line: ordinary,
			apart: inTheListWritten("\nunits/a.mdx\n", "\nunits/A.mdx\n")},
			[]string{"names the class ByBytes, whose predicate does not hold of it"}},
		{oracleRun{name: "a difference of the metadata named in a line of game-paths", class: "WiderSpace",
			line: ordinary,
			lay: func(c checkout) {
				noList(c)
				withFields(map[string][]objects.FieldMeta{"buffs": {field("fnbs", "noBreak", noBreakSpaces)}})(c)
			}, apart: inTheListWritten("\nunits/a.mdx\n", "\nunits/A.mdx\n")},
			[]string{"names the class WiderSpace, whose predicate does not hold of it"}},
		{oracleRun{name: "a difference without a file", class: "WiderSpace", lay: noList, line: noBreak},
			[]string{"names nothing that the two trees write apart", gamePathsPath + ": differs at offset"}},
		{oracleRun{name: "a place of another difference than the class's", class: "WiderSpace", lay: noList,
			line:  gamePathsOf("\xC2\xA0Sound/Hit.wav\nUnits/\xC4\xB0.MDX\n", "2.0.0"),
			apart: inTheListWritten("\nunits/i\xCC\x87.mdx\n", "\nunits/i.mdx\n")},
			[]string{"does not show the difference of the class WiderSpace"}},
		{oracleRun{name: "a place without the bytes of its class", class: "DottedI", lay: noList,
			line:  gamePathsOf("Units/\xC4\xB0.MDX\n", "2.0.0"),
			apart: inTheListWritten("\xCC\x87.mdx\n", ".mdx\n")},
			[]string{"does not show the difference of the class DottedI"}},
		{oracleRun{name: "a difference in what is printed, and the list alone named", class: "WiderSpace",
			lay: noList, line: gamePathsOf(aNameThenWiderSpace, "2.0.0"), apart: inTheListWritten("units/b.mdx\n", "")},
			[]string{standardOutput + ": differs at offset"}},
		{oracleRun{name: "a stream named, and a place that it does not hold", class: "WiderSpace", lay: noList,
			line: gamePathsOf(aNameThenWiderSpace, "2.0.0"),
			apart: map[string][]place{
				gamePathsPath:  {{"units/b.mdx\n", ""}},
				standardOutput: {{": 3 paths.", ": 1 paths."}},
			}},
			[]string{standardOutput + `: the other tree wrote ": 3 paths." 0 times, want once`}},
		{oracleRun{name: "a stream named, and a place that this tree prints otherwise", class: "WiderSpace",
			lay: noList, line: gamePathsOf(aNameThenWiderSpace, "2.0.0"),
			apart: map[string][]place{
				gamePathsPath:  {{"units/b.mdx\n", ""}},
				standardOutput: {{": 2 paths.", ": 0 paths."}},
			}},
			[]string{standardOutput + ": this tree does not write what it must"}},
		{oracleRun{name: "a stream named that the trees write alike", class: "WiderSpace", lay: noList,
			line: gamePathsOf(aNameThenWiderSpace, "2.0.0"),
			apart: map[string][]place{
				gamePathsPath:  {{"units/b.mdx\n", ""}},
				standardOutput: {{": 2 paths.", ": 1 paths."}},
				standardError:  {{"error: ", "error: no "}},
			}},
			[]string{standardError + `: the other tree wrote "error: " 0 times, want once`}},
		{oracleRun{name: "a difference in how the trees end", class: "WiderSpace", lay: noList,
			line: gamePathsOf("Units/B.mdx\xC2\xA0\n", "2.0.0"), apart: inTheListWritten("units/b.mdx\n", "")},
			[]string{"the exit code: values differ", standardError + ": length differs",
				standardOutput + ": length differs", "what the checkout holds: values differ"}},
		{oracleRun{name: "a difference, and no class named", lay: noList, line: noBreak},
			[]string{gamePathsPath + ": differs at offset"}},
		{oracleRun{name: "a class about a difference, and the trees write alike", class: "NoUTF8", lay: noList,
			line:  gamePathsOf("Units/B\xE4\xB8.mdx\n", "2.0.0"),
			apart: inTheListWritten("\nunits/b\xEF\xBF\xBD.mdx\n", "\nunits/b.mdx\n")},
			[]string{"the class is about a difference, and there is none"}},
		{oracleRun{name: "a place that the other tree did not write", class: "WiderSpace", lay: noList, line: noBreak,
			apart: inTheListWritten("\nunits/c.mdx\n", "\n\xC2\xA0units/c.mdx\n")},
			[]string{`the other tree wrote "\nunits/c.mdx\n" 0 times, want once`}},
		{oracleRun{name: "a place that is written alike", class: "WiderSpace", lay: noList, line: noBreak,
			apart: inTheListWritten("\nunits/b.mdx\n", "\nunits/b.mdx\n")},
			[]string{"which is no place apart"}},
		{oracleRun{name: "a place that this tree writes otherwise", class: "WiderSpace", lay: noList, line: noBreak,
			apart: inTheListWritten("\nunits/b.mdx\n", "\n units/b.mdx\n")},
			[]string{gamePathsPath + ": this tree does not write what it must: the two part at offset 33"}},
		{oracleRun{name: "another file named than the one written apart", class: "WiderSpace", lay: noList,
			line: noBreak, apart: map[string][]place{"go.mod": {{"module ", "modul "}}}},
			[]string{listDiffers, "go.mod: the two trees write it alike"}},
		{oracleRun{name: "a refusal without its words", class: "CountRefused", line: words("", "more"),
			lay: oneBuff("fnam", "name")},
			[]string{"holds nothing that this tree must say"}},
		{oracleRun{name: "a refusal of a line that the other tree refuses too", class: "CountRefused",
			refusal: "error: Usage: go run ./tools/gen game-paths <listfile> <game version, e.g. 3.0.0.24268>\n",
			line:    words("game-paths")},
			[]string{"names the class CountRefused, whose predicate does not hold of it"}},
		{oracleRun{name: "a file the system cannot give, and no argument named", class: "AsGiven", lay: anotherList,
			line: notThere},
			[]string{"names the class AsGiven, whose predicate does not hold of it", errorDiffers}},
		{oracleRun{name: "an argument named that names a file", class: "AsGiven", cannotGive: &named{argument: 1},
			lay: noList, line: ordinary},
			[]string{"names the class AsGiven, whose predicate does not hold of it"}},
		{oracleRun{name: "a file the system cannot give, and no class named", lay: anotherList, line: notThere},
			[]string{"standard error: differs at offset"}},
		{oracleRun{name: "a script the system cannot give, and the folder named that has it not", class: "AsGiven",
			cannotGive: &named{argument: 1}, lay: luaExtras(miniExtrasText), line: withoutBlizzard},
			[]string{"names the class AsGiven, whose predicate does not hold of it", errorDiffers}},
		{oracleRun{name: "a script the system cannot give, and the other script named", class: "AsGiven",
			cannotGive: &named{1, commonOfAnExport}, lay: luaExtras(miniExtrasText), line: withoutBlizzard},
			[]string{"names the class AsGiven, whose predicate does not hold of it", errorDiffers}},
		{oracleRun{name: "a script the system cannot give, and no class named", lay: luaExtras(miniExtrasText),
			line: withoutBlizzard},
			[]string{errorDiffers}},
		{oracleRun{name: "extras that this tree reads, named as extras with a key too many", class: "UnknownKey",
			lay: luaExtras(miniExtrasText), line: ofTheMiniatures,
			refusal: keyNotRead("the file", "more", "functions, globals and removed")},
			[]string{"names the class UnknownKey, whose predicate does not hold of it"}},
		{oracleRun{name: "a key too many, in a line of another mode", class: "UnknownKey", line: ordinary,
			lay:     luaExtras(`{"more": 1}`),
			refusal: keyNotRead("the file", "more", "functions, globals and removed")},
			[]string{"names the class UnknownKey, whose predicate does not hold of it"}},
		{oracleRun{name: "a key too many, and no class named", lay: luaExtras(`{"more": 1}`), line: ofTheMiniatures},
			[]string{"the exit code: values differ", standardError + ": length differs"}},
		{oracleRun{name: "a function without params, named as one without a name", class: "NoName",
			lay:     luaExtras(functionsOfTheExtras(`{"name": "Bare", "returns": "nothing"}`)),
			line:    ofTheMiniatures,
			refusal: keyLeftOut("the function Bare", "params")},
			[]string{"names the class NoName, whose predicate does not hold of it", "the exit code: values differ"}},
		{oracleRun{name: "a function without a name, named as one without params", class: "LacksAKey",
			lay:     luaExtras(functionsOfTheExtras(`{"returns": "nothing"}`)),
			line:    ofTheMiniatures,
			refusal: keyLeftOut("function 1 of the list", "name")},
			[]string{"names the class LacksAKey, whose predicate does not hold of it", "the exit code: values differ"}},
		{oracleRun{name: "extras without null in a list, named as extras with one", class: "NullEntry",
			lay: luaExtras(`{"globals": ["print"], "removed": null}`), line: ofTheMiniatures,
			refusal: entryIsNull("1", "removed")},
			[]string{"names the class NullEntry, whose predicate does not hold of it"}},
		{oracleRun{name: "null among the globals, and no class named", lay: luaExtras(`{"globals": ["print", null]}`),
			line: ofTheMiniatures},
			[]string{"the exit code: values differ", standardError + ": length differs"}},
		{oracleRun{name: "extras cut short inside a text, named as cut where the readers end apart", class: "EndOfJSON",
			lay: luaExtras(`{"functions": [{ "name": "Fo`), line: ofTheMiniatures, apart: endsTooSoon},
			[]string{"names the class EndOfJSON, whose predicate does not hold of it"}},
		{oracleRun{name: "extras cut short after a bracket, and no class named", lay: luaExtras(`{"functions": [`),
			line: ofTheMiniatures},
			[]string{errorDiffers}},
		{oracleRun{name: "keys in the order of the file, named as keys in another order", class: "KeyOrder",
			lay: luaExtras(miniExtrasText), line: ofTheMiniatures,
			apart: inTheNativesWritten("      \"name\": \"FourCC\",\n", "      \"name\": \"fourCC\",\n")},
			[]string{"names the class KeyOrder, whose predicate does not hold of it"}},
		{oracleRun{name: "keys in another order, and a place that is no other order", class: "KeyOrder",
			lay:   luaExtras(functionsOfTheExtras(`{"returns": "integer", "params": [], "name": "Odd"}`)),
			line:  ofTheMiniatures,
			apart: inTheNativesWritten("      \"returns\": \"integer\",\n", "      \"name\": \"Odd\",\n")},
			[]string{"does not show the difference of the class KeyOrder"}},
		{oracleRun{name: "keys in another order, and no class named",
			lay:  luaExtras(functionsOfTheExtras(`{"returns": "integer", "params": [], "name": "Odd"}`)),
			line: ofTheMiniatures},
			[]string{nativesPath + ": differs at offset"}},
		{oracleRun{name: "a refusal of the extras in other words than this tree's", class: "LacksAKey",
			lay:     luaExtras(functionsOfTheExtras(`{"name": "Bare", "returns": "nothing"}`)),
			line:    ofTheMiniatures,
			refusal: keyLeftOut("the function Bare", "returns")},
			[]string{standardError + ": this tree does not say what it must"}},
		{oracleRun{name: "overrides that this tree reads, named as overrides with a key too many", class: "OverridesKey",
			lay: miniPins, line: ofTheMiniExport, refusal: overridesKeyNotRead("renamed")},
			[]string{"names the class OverridesKey, whose predicate does not hold of it"}},
		{oracleRun{name: "a key too many in the overrides, and no class named", line: ofTheMiniExport,
			lay: pinned(`{"names": {"units": {"ucls": "unitClass"}}, "renamed": {}}`)},
			[]string{"the exit code: values differ", standardError + ": length differs"}},
		{oracleRun{name: "a label that the export has not, named as one that gives a name that is taken",
			class: "NameTaken", lay: miniPins, line: ofTheMiniExport,
			holds:   []laid{{labelsFile, "WESTRING_FART=Private"}},
			refusal: nameNotAllowed("buffs", "fart", "private", "Private")},
			[]string{"names the class NameTaken, whose predicate does not hold of it"}},
		{oracleRun{name: "a label of another name, named as one that gives a name that is taken", class: "NameTaken",
			lay: miniPins, line: ofTheMiniExport, holds: []laid{{labelsFile, "WESTRING_FART=Icon"}},
			refusal: nameNotAllowed("buffs", "fart", "icon", "Icon")},
			[]string{"names the class NameTaken, whose predicate does not hold of it"}},
		{oracleRun{name: "a pin of another name, named as one that gives a name that is taken", class: "NameTaken",
			lay: pinned(`{"names": {"units": {"ucls": "unitClass"}, "buffs": {"fart": "outputs"}}}`), line: ofTheMiniExport,
			refusal: nameNotAllowed("buffs", "fart", "outputs", "Icon")},
			[]string{"names the class NameTaken, whose predicate does not hold of it"}},
		{oracleRun{name: "a pin of the name output, and no class named", line: ofTheMiniExport,
			lay: pinned(`{"names": {"units": {"ucls": "unitClass"}, "buffs": {"fart": "output"}}}`)},
			[]string{"the exit code: values differ", standardError + ": length differs"}},
		{oracleRun{name: "an id of ASCII, named as one that the trees pad apart", class: "PaddedID", lay: miniPins,
			holds: []laid{{buffFieldsTable, `C;X1;Y3;K"fa"`}},
			line:  metadataOf("3.0.0.1", swapped(buffFieldsTable, `K"fart"`, `K"fa"`)),
			apart: inTheMetadataWritten(place{"\"id\":\"fa\\u0000\\u0000\"", "\"id\":\"fa\\u0000\""})},
			[]string{"names the class PaddedID, whose predicate does not hold of it"}},
		{oracleRun{name: "an id that the trees pad apart, and no class named", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(buffFieldsTable, `K"fart"`, "K\"\xC3\xA9a\""))},
			[]string{metadataPath + ": differs at offset"}},
		{oracleRun{name: "a cell that is a number, named as one that this tree alone refuses", class: "NumberRefused",
			lay: miniPins, holds: []laid{{abilityFieldsTable, `C;X5;K"4"`}},
			line:    metadataOf("3.0.0.1", swapped(abilityFieldsTable, "C;X5;K4", `C;X5;K"4"`)),
			refusal: noNumber("acdn", "repeat", "4")},
			[]string{"names the class NumberRefused, whose predicate does not hold of it"}},
		{oracleRun{name: "a data cell that is NaN, named as a cell that the other tree reads", class: "NumberRefused",
			lay: miniPins, holds: []laid{{abilityFieldsTable, `C;X6;K"NaN"`}},
			line:    metadataOf("3.0.0.1", swapped(abilityFieldsTable, "C;X6;K12", `C;X6;K"NaN"`)),
			refusal: noNumber("Hdc1", "data", "NaN")},
			[]string{"names the class NumberRefused, whose predicate does not hold of it", errorDiffers}},
		{oracleRun{name: "a repeat cell that is NaN, named as a cell that both trees refuse", class: "OtherWords",
			lay: miniPins, holds: []laid{{abilityFieldsTable, `C;X5;K"NaN"`}},
			line:  metadataOf("3.0.0.1", swapped(abilityFieldsTable, "C;X5;K4", `C;X5;K"NaN"`)),
			apart: onStandardError("the data column 'NaN' is not a whole number", "the data cell 'NaN' is not a number")},
			[]string{"names the class OtherWords, whose predicate does not hold of it", "the exit code: values differ"}},
		{oracleRun{name: "a repeat cell that is NaN, and no class named", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilityFieldsTable, "C;X5;K4", `C;X5;K"NaN"`))},
			[]string{"the exit code: values differ", standardError + ": length differs"}},
		{oracleRun{name: "a row without its levels, and a place in other words than this tree's", class: "OtherWords",
			lay: miniPins, holds: []laid{{abilitiesTable, "C;X1;Y2;K\"AHhb\"\r\nC;X2;K\"holy light\"\r\nC;X1;Y3;"}},
			line:  metadataOf("3.0.0.1", swapped(abilitiesTable, "C;X3;K3\r\n", "")),
			apart: onStandardError("bad levels 'undefined'", "bad levels: no cell")},
			[]string{standardError + ": this tree does not write what it must"}},
		{oracleRun{name: "a row without its levels, and no class named", lay: miniPins,
			line: metadataOf("3.0.0.1", swapped(abilitiesTable, "C;X3;K3\r\n", ""))},
			[]string{errorDiffers}},
		{oracleRun{name: "one fault, named as two", class: "TwoFaults", lay: pinned("{}"), line: ofTheMiniExport,
			holds:   []laid{{labelsFile, "WESTRING_UCLS=Class"}, {upgradesTable, `C;X1;Y2;K"Rhme"`}},
			refusal: doesNotParse(upgradesTable)},
			[]string{standardError + ": this tree does not say what it must"}},
		{oracleRun{name: "two faults, the second in a table that both trees read before they name a field",
			class: "TwoFaults", lay: pinned("{}"),
			holds:   []laid{{labelsFile, "WESTRING_UCLS=Class"}, {unitFieldsTable, openQuote("uhpm")}},
			line:    metadataOf("3.0.0.1", set(unitFieldsTable, brokenTable("ID", "uhpm"))),
			refusal: doesNotParse(unitFieldsTable)},
			[]string{"names the class TwoFaults, whose predicate does not hold of it"}},
		{oracleRun{name: "two faults, and no class named", lay: pinned("{}"),
			line: metadataOf("3.0.0.1", set(upgradesTable, brokenTable("upgradeid", "Rhme")))},
			[]string{errorDiffers}},
		{oracleRun{name: "a released metadata that is cut short, and no class named", line: ofTheMiniExport,
			lay: released(unitClassPins, `{"format": 1,`)},
			[]string{errorDiffers}},
		{oracleRun{name: "a value of the class WiderSpace that the export does not hold", class: "WiderSpace",
			lay: miniPins, line: ofTheMiniExport, holds: []laid{{labelsFile, "WESTRING_FART=\xC2\xA0Icon"}},
			apart: inTheMetadataWritten(place{"\"label\":\"Icon\"", "\"label\":\"\xC2\xA0Icon\""})},
			[]string{"names the class WiderSpace, whose predicate does not hold of it"}},
		{oracleRun{name: "a path from the working folder, given to run in the process of the test", below: "work",
			line: words("game-paths", "listfile.txt", "2.0.0"),
			lay: func(c checkout) {
				noList(c)
				c.write("work/listfile.txt", "Units/A.mdx\n")
			}},
			[]string{"the exit code: values differ"}},
		{oracleRun{name: "two checkouts that are not laid alike", lay: func(c checkout) {
			laidApart++
			c.write(metadataPath, strings.Repeat(" ", laidApart)+"{}")
		}},
			[]string{"the run lays two checkouts that are not the same"}},
	} {
		heard := listenTo(t, func(tb testing.TB) { o.compare(tb, probe.r, newTally()) })
		for _, words := range probe.reports {
			if !strings.Contains(heard, words) {
				t.Errorf("%s: the oracle reported\n%s\nwant a report with the words %q", probe.r.name, heard, words)
			}
		}
	}
}
