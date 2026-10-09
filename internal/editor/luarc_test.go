package editor

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestMergeLuarcTakesALinkToNothingForNoFile(t *testing.T) {
	for _, kind := range brokenSymlinks {
		t.Run(kind.name, func(t *testing.T) {
			above, root, _ := newProjectWithBrokenSymlink(t, ".luarc.json", kind.make)
			before := readTree(t, above)
			added, merged, err := MergeLuarc(root, templateFiles(t))
			if err != nil || !merged || added == nil || len(added) != 0 {
				t.Errorf("MergeLuarc = %#v, %v, %v", added, merged, err)
			}
			if after := readTree(t, above); !maps.Equal(after, before) {
				t.Errorf("the folder above the project holds %q, and held %q", after, before)
			}
		})
	}
}

func TestMergeLuarcAddsTheTemplatesMissingEntries(t *testing.T) {
	root := newProjectDir(t, ".luarc.json", `{"runtime.path":["src/?.lua"],"workspace.library":[".moonwell/types","extra"],`+
		`"workspace.ignoreDir":["dist","maps"],"diagnostics.globals":["X"]}`)
	added, merged, err := MergeLuarc(root, templateFiles(t))
	want := []string{"src/?/init.lua", "lua/?.lua", "lua/?/init.lua", ".moonwell/lua", ".moonwell/libraries"}
	if err != nil || !merged || !slices.Equal(added, want) {
		t.Fatalf("MergeLuarc = %q, %v, %v", added, merged, err)
	}
	written := `{
  "runtime.path": [
    "src/?.lua",
    "src/?/init.lua",
    "lua/?.lua",
    "lua/?/init.lua"
  ],
  "workspace.library": [
    ".moonwell/types",
    "extra",
    ".moonwell/lua"
  ],
  "workspace.ignoreDir": [
    "dist",
    "maps",
    ".moonwell/libraries"
  ],
  "diagnostics.globals": [
    "X"
  ]
}
`
	if got := readFile(t, root, ".luarc.json"); got != written {
		t.Errorf(".luarc.json =\n%s", got)
	}
	if added, merged, err := MergeLuarc(root, templateFiles(t)); err != nil || !merged || added == nil || len(added) != 0 {
		t.Errorf("MergeLuarc again = %#v, %v, %v", added, merged, err)
	}
	if got := readFile(t, root, ".luarc.json"); got != written {
		t.Errorf("after a merge that added nothing, .luarc.json =\n%s", got)
	}
}

func TestMergeLuarcGivesAnObjectWithoutTheArraysEveryEntryOfTheCarriedTemplate(t *testing.T) {
	for _, held := range []string{"{}", "{} \r\n\t", " \n{\n}\n"} {
		root := newProjectDir(t, ".luarc.json", held)
		added, merged, err := MergeLuarc(root, templateFiles(t))
		if want := slices.Concat(carriedPaths, carriedLibrary, carriedIgnored); err != nil || !merged || !slices.Equal(added, want) {
			t.Fatalf("%q: MergeLuarc = %q, %v, %v", held, added, merged, err)
		}
		if got := readFile(t, root, ".luarc.json"); got != "{\n"+afterOne+"\n}\n" {
			t.Errorf("%q: .luarc.json =\n%s", held, got)
		}
	}
}

func TestMergeLuarcLeavesAFileThatIsNotAJSONObjectAlone(t *testing.T) {
	contents := []string{
		"// a comment\n{ \"runtime.path\": [] }\n", "[]", "null", "3",
		"{ \"runtime.path\": [], }", "{ \"runtime.path\": [\"src/?.lua\",] }", "{ /* a comment */ }", "{} {}", "{}}",
		"", " \n", "true", `"text"`, "{", mark + mark + "{}", "{'runtime.path': []}",
	}
	for _, content := range contents {
		root := newProjectDir(t, ".luarc.json", content)
		added, merged, err := MergeLuarc(root, templateFiles(t))
		if err != nil || merged || added != nil || readFile(t, root, ".luarc.json") != content {
			t.Errorf("%q: MergeLuarc = %q, %v, %v", content, added, merged, err)
		}
	}
	root := t.TempDir()
	if added, merged, err := MergeLuarc(root, templateFiles(t)); err != nil || !merged || added == nil || len(added) != 0 {
		t.Errorf("MergeLuarc without a file = %#v, %v, %v", added, merged, err)
	}
	if got := listEntries(t, root); len(got) != 0 {
		t.Errorf("without a file, the project holds %q", got)
	}
}

func TestMergeLuarcGivesAMissingKeyTheTemplatesWholeArrayAndLeavesANonArrayValueAlone(t *testing.T) {
	for _, value := range []string{`"not an array"`, "null", `{"0":".moonwell/types"}`, "3", "true"} {
		root := newProjectDir(t, ".luarc.json", `{"workspace.library":`+value+`}`)
		added, merged, err := MergeLuarc(root, templateFiles(t))
		if want := slices.Concat(carriedPaths, carriedIgnored); err != nil || !merged || !slices.Equal(added, want) {
			t.Fatalf("%s: MergeLuarc = %q, %v, %v", value, added, merged, err)
		}
		config := mustParseObject(t, readFile(t, root, ".luarc.json"))
		if library, _ := config.Get("workspace.library"); compactJSON(library) != value ||
			!slices.Equal(stringListAt(t, config, "runtime.path"), carriedPaths) ||
			!slices.Equal(stringListAt(t, config, "workspace.ignoreDir"), carriedIgnored) ||
			!slices.Equal(config.Keys(), []string{"workspace.library", "runtime.path", "workspace.ignoreDir"}) {
			t.Errorf("%s: .luarc.json =\n%s", value, readFile(t, root, ".luarc.json"))
		}
	}
	const held = `{"runtime.path":null,"workspace.library":{"a":1},"workspace.ignoreDir":3}`
	root := newProjectDir(t, ".luarc.json", held)
	if added, merged, err := MergeLuarc(root, templateFiles(t)); err != nil || !merged || added == nil || len(added) != 0 ||
		readFile(t, root, ".luarc.json") != held {
		t.Errorf("no arrays: MergeLuarc = %#v, %v, %v", added, merged, err)
	}
}

func TestMergeLuarcMergesAFileSavedWithABOM(t *testing.T) {
	root := newProjectDir(t, ".luarc.json", mark+`{"runtime.path":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"]}`)
	added, merged, err := MergeLuarc(root, templateFiles(t))
	if want := slices.Concat(carriedLibrary, carriedIgnored); err != nil || !merged || !slices.Equal(added, want) {
		t.Fatalf("MergeLuarc = %q, %v, %v", added, merged, err)
	}
	if got := readFile(t, root, ".luarc.json"); got != "{\n"+afterOne+"\n}\n" {
		t.Errorf(".luarc.json =\n%q", got)
	}
}

func TestMergeLuarcReportsAFileItCannotRead(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".luarc.json"), 0o777); err != nil {
		t.Fatal(err)
	}
	added, merged, err := MergeLuarc(root, templateFiles(t))
	e := asDiagError(t, err, "a folder for .luarc.json")
	if added != nil || merged || !strings.HasPrefix(e.Msg, "Reading .luarc.json failed: ") || e.File != ".luarc.json" ||
		!strings.Contains(e.Hint, "has .luarc.json open") || e.Cause == nil {
		t.Errorf("MergeLuarc = %q, %v, %+v", added, merged, e)
	}
}

func TestMergeLuarcReportsAFileItCannotWrite(t *testing.T) {
	root := newProjectDir(t, ".luarc.json", "{"+lackingOne+"}")
	testkit.MakeUnwritable(t, filepath.Join(root, ".luarc.json"))
	added, merged, err := MergeLuarc(root, templateFiles(t))
	e := asDiagError(t, err, "a .luarc.json that cannot be written")
	if added != nil || merged || !strings.HasPrefix(e.Msg, "Writing .luarc.json failed: ") || e.File != ".luarc.json" || e.Hint == "" {
		t.Errorf("MergeLuarc = %q, %v, %+v", added, merged, e)
	}
	if got := readFile(t, root, ".luarc.json"); got != "{"+lackingOne+"}" {
		t.Errorf(".luarc.json = %q", got)
	}
}

func TestMergeLuarcKeepsTheTextOfEveryValue(t *testing.T) {
	numbers := []string{
		"1.0", "1e3", "1E+2", "-0", "-0.0", "0.10", "9007199254740993", "0.1234567890123456789", "1e400", "-1e-7",
		"100000000000000000000000",
	}
	texts := []string{
		`"` + escapeU + `00e9"`, `"` + eAcute + `"`, `"\/"`, `"/"`, `"<"`, `"` + escapeU + `003c"`, `"&"`,
		`"` + escapeU + `2028"`, `"` + lineSep + `"`, `"` + escapeU + `d800"`, `"` + escapeU + `007f"`, "\"\x7f\"",
		`"\b"`, `"` + escapeU + `0008"`, `"\\"`, `"\""`, `"` + escapeU + `D83D` + escapeU + `DE00"`,
	}
	const faulty = "\"a\xff\xe2\x82 b\xc3\""
	cases := []struct{ name, held, want string }{
		{"a number",
			"{" + lackingOne + `,"n":[` + strings.Join(numbers, ",") + `]}`,
			"{\n" + afterOne + ",\n  \"n\": " + formatJSONList(numbers) + "\n}\n"},
		{"a string",
			"{" + lackingOne + `,"s":[` + strings.Join(texts, ",") + `]}`,
			"{\n" + afterOne + ",\n  \"s\": " + formatJSONList(texts) + "\n}\n"},
		{"an object in a value: a key twice, keys that look like numbers, a key with an escape",
			"{" + lackingOne + `,"o":{"x":1,"y":3,"x":2,"b":{"10":0,"2":0,"a":0},"\/":[{"1":1,"0":0}]}}`,
			"{\n" + afterOne + ",\n  \"o\": {\n    \"x\": 1,\n    \"y\": 3,\n    \"x\": 2,\n    \"b\": {\n      \"10\": 0,\n" +
				"      \"2\": 0,\n      \"a\": 0\n    },\n    \"\\/\": [\n      {\n        \"1\": 1,\n        \"0\": 0\n      }\n    ]\n  }\n}\n"},
		{"keys of the file that look like numbers",
			`{"b":1,"10":2,"2":3,` + lackingOne + `,"0":4}`,
			"{\n  \"b\": 1,\n  \"10\": 2,\n  \"2\": 3,\n" + afterOne + ",\n  \"0\": 4\n}\n"},
		{"bytes that are not UTF-8",
			"{" + lackingOne + `,"s":` + faulty + `,"t":[` + faulty + `]}`,
			"{\n" + afterOne + ",\n  \"s\": " + faulty + ",\n  \"t\": [\n    " + faulty + "\n  ]\n}\n"},
		{"a value of one of the three arrays",
			`{"runtime.path":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"],"workspace.library":[".moonwell\/types",1.0,` +
				`{"1":1,"0":0},"` + escapeU + "00e9\",\"caf\xe9\"" + `],"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`,
			"{\n" + strings.Replace(afterOne, "    \".moonwell/types\",\n",
				"    \".moonwell\\/types\",\n    1.0,\n    {\n      \"1\": 1,\n      \"0\": 0\n    },\n    \""+escapeU+"00e9\",\n"+
					"    \"caf\xe9\",\n", 1) + "\n}\n"},
	}
	for _, c := range cases {
		if got := mustMergeLuarc(t, c.name, c.held); got != c.want {
			t.Errorf("%s: .luarc.json = %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestMergeLuarcLaysTheWholeFileOutWithTwoSpacesAndAFinalLineBreak(t *testing.T) {
	held := "\r\n{\r\n\t" + lackingOne + ",\r\n\t\"o\" : { \"a\" : [ ] , \"b\" : { } , \"c\":[1 , 2,[\n]] }\r\n}\r\n\r\n"
	want := "{\n" + afterOne + ",\n  \"o\": {\n    \"a\": [],\n    \"b\": {},\n    \"c\": [\n      1,\n      2,\n      []\n    ]\n  }\n}\n"
	if got := mustMergeLuarc(t, "white space of every kind", held); got != want {
		t.Errorf(".luarc.json = %q\nwant %q", got, want)
	}
}

func TestMergeLuarcWritesAKeyOfTheFileOnceAndWithTheEscapesItNeeds(t *testing.T) {
	cases := []struct{ name, held, want string }{
		{"a key twice",
			`{"x":1,"runtime.path":["a"],` + lackingOne + `,"x":2}`,
			"{\n  \"x\": 2,\n" + afterOne + "\n}\n"},
		{"a key with escapes",
			`{"` + escapeU + `00e9\/<` + escapeU + `2028` + lineSep + `\"\\` + escapeU + `0001\b` + escapeU + `007f":1,` + lackingOne + `}`,
			"{\n  \"" + eAcute + "/<" + lineSep + lineSep + `\"\\` + escapeU + "0001\\b\x7f\": 1,\n" + afterOne + "\n}\n"},
		{"a key of one of the arrays with an escape",
			`{"runtime` + escapeU + `002epath":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"],` +
				`"workspace.library":[".moonwell/types"],"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`,
			"{\n" + afterOne + "\n}\n"},
		{"a key with bytes that are not UTF-8",
			"{\"k\xe2\x82\":1," + lackingOne + "}",
			"{\n  \"k" + replaced + replaced + "\": 1,\n" + afterOne + "\n}\n"},
	}
	for _, c := range cases {
		if got := mustMergeLuarc(t, c.name, c.held); got != c.want {
			t.Errorf("%s: .luarc.json = %q\nwant %q", c.name, got, c.want)
		}
	}
}

func TestMergeLuarcTakesAnEntryToBeThereOnlyAsTheSameString(t *testing.T) {
	cases := []struct {
		name, held string
		added      []string
		library    string
	}{
		{"another letter case, a slash at the end, a backslash",
			`[".Moonwell/types",".moonwell/types/",".moonwell\\lua",".moonwell/lua/"]`, carriedLibrary,
			`[".Moonwell/types",".moonwell/types/",".moonwell\\lua",".moonwell/lua/",".moonwell/types",".moonwell/lua"]`},
		{"the entries with escapes",
			`[".moonwell\/types","` + escapeU + `002emoonwell/lua"]`, nil, ""},
		{"elements that are no strings",
			`[1,null,{"a":".moonwell/lua"},[".moonwell/lua"],true,".moonwell/types"]`, carriedLibrary[1:],
			`[1,null,{"a":".moonwell/lua"},[".moonwell/lua"],true,".moonwell/types",".moonwell/lua"]`},
		{"an entry twice",
			`[".moonwell/types",".moonwell/types"]`, carriedLibrary[1:],
			`[".moonwell/types",".moonwell/types",".moonwell/lua"]`},
		{"an empty array", `[]`, carriedLibrary, `[".moonwell/types",".moonwell/lua"]`},
	}
	for _, c := range cases {
		held := `{"runtime.path":["src/?.lua","src/?/init.lua","lua/?.lua","lua/?/init.lua"],"workspace.library":` + c.held +
			`,"workspace.ignoreDir":["dist","maps",".moonwell/libraries"]}`
		root := newProjectDir(t, ".luarc.json", held)
		added, merged, err := MergeLuarc(root, templateFiles(t))
		if err != nil || !merged || added == nil || !slices.Equal(added, c.added) {
			t.Errorf("%s: MergeLuarc = %#v, %v, %v", c.name, added, merged, err)
		}
		got := readFile(t, root, ".luarc.json")
		if len(c.added) == 0 {
			if got != held {
				t.Errorf("%s: nothing was added, and .luarc.json = %q", c.name, got)
			}
			continue
		}
		library, _ := mustParseObject(t, got).Get("workspace.library")
		if got := compactJSON(library); got != c.library {
			t.Errorf("%s: workspace.library = %s", c.name, got)
		}
	}
}

func TestMergeLuarcWithATemplateItCannotReadIsAMistakeOfTheCaller(t *testing.T) {
	const arrays = `"runtime.path":["a"],"workspace.library":["b"],"workspace.ignoreDir":[]`
	cases := []struct {
		name     string
		template []moonwell.TemplateFile
		words    []string
	}{
		{"no template", nil, []string{"template", ".luarc.json"}},
		{"a template without the file", smallTemplate[:1], []string{"template", ".luarc.json"}},
		{"a file that is no JSON", templateWithLuarc("// a comment\n{" + arrays + "}"), []string{"template", "not a JSON object"}},
		{"a file that is no object", templateWithLuarc("[]"), []string{"template", "not a JSON object"}},
		{"a file that is null", templateWithLuarc("null"), []string{"template", "not a JSON object"}},
		{"no array", smallTemplate, []string{"template", "runtime.path"}},
		{"a value that is no array", templateWithLuarc(`{"runtime.path":["a"],"workspace.library":null,"workspace.ignoreDir":[]}`),
			[]string{"template", "workspace.library"}},
		{"an entry that is no string", templateWithLuarc(`{"runtime.path":["a"],"workspace.library":["b"],"workspace.ignoreDir":["c",1.0]}`),
			[]string{"template", "workspace.ignoreDir"}},
		{"an entry that is null", templateWithLuarc(`{"runtime.path":[null],"workspace.library":["b"],"workspace.ignoreDir":[]}`),
			[]string{"template", "runtime.path"}},
	}
	for _, c := range cases {
		root := newProjectDir(t, ".luarc.json", "{}")
		added, merged, err := MergeLuarc(root, c.template)
		checkPlainError(t, err, c.name, c.words...)
		if added != nil || merged || readFile(t, root, ".luarc.json") != "{}" {
			t.Errorf("%s: MergeLuarc = %q, %v", c.name, added, merged)
		}
		entries, err := LuarcTemplateEntries(c.template)
		checkPlainError(t, err, c.name, c.words...)
		if entries != nil {
			t.Errorf("%s: LuarcTemplateEntries = %q", c.name, entries)
		}
	}
	if added, merged, err := MergeLuarc(t.TempDir(), nil); err != nil || !merged || len(added) != 0 {
		t.Errorf("without a file and without a template: MergeLuarc = %q, %v, %v", added, merged, err)
	}
}

func TestLuarcTemplateEntriesListsTheTemplatesArrays(t *testing.T) {
	entries, err := LuarcTemplateEntries(templateFiles(t))
	want := map[string][]string{
		"runtime.path": carriedPaths, "workspace.library": carriedLibrary, "workspace.ignoreDir": carriedIgnored,
	}
	if err != nil || !reflect.DeepEqual(entries, want) {
		t.Errorf("LuarcTemplateEntries = %q, %v", entries, err)
	}
}

func TestATemplateOfItsOwnGivesItsEntriesAndEachOfThemOnce(t *testing.T) {
	template := templateWithLuarc(mark + `{"runtime.path":["a","b","a"],"other":["x"],"workspace.library":["b","b"],"workspace.ignoreDir":[]}`)
	entries, err := LuarcTemplateEntries(template)
	listed := map[string][]string{"runtime.path": {"a", "b", "a"}, "workspace.library": {"b", "b"}, "workspace.ignoreDir": nil}
	if err != nil || !reflect.DeepEqual(entries, listed) {
		t.Errorf("LuarcTemplateEntries = %#v, %v", entries, err)
	}
	cases := []struct {
		name, held string
		added      []string
		want       string
	}{
		{"a file without the arrays", "{}", []string{"a", "b", "b"},
			"{\n  \"runtime.path\": [\n    \"a\",\n    \"b\"\n  ],\n  \"workspace.library\": [\n    \"b\"\n  ],\n  \"workspace.ignoreDir\": []\n}\n"},
		{"a file with some of the entries", `{"workspace.library":[],"runtime.path":["b"]}`, []string{"a", "b"},
			"{\n  \"workspace.library\": [\n    \"b\"\n  ],\n  \"runtime.path\": [\n    \"b\",\n    \"a\"\n  ],\n  \"workspace.ignoreDir\": []\n}\n"},
	}
	for _, c := range cases {
		root := newProjectDir(t, ".luarc.json", c.held)
		added, merged, err := MergeLuarc(root, template)
		if err != nil || !merged || !slices.Equal(added, c.added) {
			t.Fatalf("%s: MergeLuarc = %q, %v, %v", c.name, added, merged, err)
		}
		if got := readFile(t, root, ".luarc.json"); got != c.want {
			t.Errorf("%s: .luarc.json = %q", c.name, got)
		}
	}
}

func mustMergeLuarc(t *testing.T, name, held string) string {
	t.Helper()
	root := newProjectDir(t, ".luarc.json", held)
	added, merged, err := MergeLuarc(root, templateFiles(t))
	if err != nil || !merged || !slices.Equal(added, []string{".moonwell/lua"}) {
		t.Fatalf("%s: MergeLuarc = %q, %v, %v", name, added, merged, err)
	}
	return readFile(t, root, ".luarc.json")
}

func formatJSONList(texts []string) string {
	return "[\n    " + strings.Join(texts, ",\n    ") + "\n  ]"
}
