package library

import (
	"strings"
	"testing"
)

const (
	where      = "https://github.com/owner/lib/blob/v1/moonwell-library.json"
	reportIt   = "Report it to the library's author, or use another tag of the library."
	needsNewer = "This Moonwell knows dir and assets; the library may need a newer Moonwell."
)

func TestALibraryWithoutTheFileShipsModulesFromItsRootAndNoAssets(t *testing.T) {
	// What is given as the content of a file that is not there is not read.
	for _, data := range [][]byte{nil, []byte("{"), []byte(`{"dir":"src"}`)} {
		described, err := ParseFile("ex", data, false, where)
		if err != nil || described.Dir != nil || described.Assets != nil {
			t.Errorf("ParseFile(%q) of a library without the file = %+v, %v", data, described, err)
		}
	}
	if File != "moonwell-library.json" {
		t.Errorf("File = %q", File)
	}
}

func TestTheFileNamesTheModuleFolderAndTheAssetsFolderEachOptional(t *testing.T) {
	cases := []struct{ document, dir, assets string }{
		{`{ "dir": "src", "assets": "assets" }`, "src", "assets"},
		{`{"assets":"files/for the map"}`, "<nil>", "files/for the map"},
		{`{"dir":"lua/lib"}`, "lua/lib", "<nil>"},
		{`{}`, "<nil>", "<nil>"},
		{" \n{}\n", "<nil>", "<nil>"},
		{"\xEF\xBB\xBF" + `{"dir":"src"}`, "src", "<nil>"},
		{`{"dir":7,"dir":"written/twice"}`, "written/twice", "<nil>"},
		{`{"dir":"..src/a.b/...","assets":"\u00e9/\ud83d\ude00"}`, "..src/a.b/...", "\xc3\xa9/\xf0\x9f\x98\x80"},
	}
	for _, c := range cases {
		described, err := ParseFile("ex", []byte(c.document), true, where)
		if err != nil || shown(described.Dir) != c.dir || shown(described.Assets) != c.assets {
			t.Errorf("ParseFile(%q) = %q, %q, %v", c.document, shown(described.Dir), shown(described.Assets), err)
		}
	}
}

func TestAFileThatIsNotAJSONObjectIsRefusedNamingTheLibraryAndTheFile(t *testing.T) {
	cases := []struct{ document, problem string }{
		{"{", "is not valid JSON."},
		{"", "is not valid JSON."},
		{`{"dir":"src"} {}`, "is not valid JSON."},
		{`{"dir":"src",}`, "is not valid JSON."},
		{"{\"dir\":\"s\xff\"}", "is not valid JSON."},
		{"\xEF\xBB\xBF\xEF\xBB\xBF{}", "is not valid JSON."},
		{"[]", "is not a JSON object."},
		{"null", "is not a JSON object."},
		{`"dir"`, "is not a JSON object."},
		{"7", "is not a JSON object."},
		{"\xEF\xBB\xBF[]", "is not a JSON object."},
	}
	for _, c := range cases {
		_, err := ParseFile("ex", []byte(c.document), true, where)
		failure := asError(t, err, c.document)
		if failure.Msg != "Library ex: moonwell-library.json "+c.problem || failure.File != where || failure.Hint != reportIt {
			t.Errorf("ParseFile(%q): %+v", c.document, failure)
		}
	}
}

func TestAnUnknownKeyIsRefusedTheFirstInSortedOrder(t *testing.T) {
	cases := []struct{ document, unknown string }{
		{`{"objects":"objects","dir":"src","extra":1}`, "extra"},
		{`{"dir":7,"Dir":"src"}`, "Dir"},
		{`{"assets ":"a"}`, "assets "},
		// U+FFFD is before U+1F600 by bytes.
		{"{\"\xf0\x9f\x98\x80\":1,\"\xef\xbf\xbd\":1}", "\xef\xbf\xbd"},
	}
	for _, c := range cases {
		_, err := ParseFile("ex", []byte(c.document), true, where)
		failure := asError(t, err, c.document)
		if !strings.Contains(failure.Msg, `has an unknown key "`+c.unknown+`".`) || failure.File != where ||
			failure.Hint != needsNewer {
			t.Errorf("ParseFile(%q): %+v", c.document, failure)
		}
	}
}

// folderNames is the two keys of the file, each of which names a folder.
var folderNames = []string{"dir", "assets"}

func TestAFolderMustBeARelativePathOfPlainNames(t *testing.T) {
	values := []string{
		`""`, `"."`, `".."`, `"a/../b"`, `"/abs"`, `"a//b"`, `"a/"`, `"a\\b"`, `"C:/x"`, `7`, `null`, `["src"]`,
		`true`, `{"dir":"src"}`, `"a/./b"`, `"a:b"`,
	}
	for _, value := range values {
		for _, name := range folderNames {
			_, err := ParseFile("ex", []byte(`{"`+name+`":`+value+`}`), true, where)
			failure := asError(t, err, value)
			want := "has " + name + " = " + value + ", which is not a folder inside the library."
			if !strings.Contains(failure.Msg, want) || failure.File != where || failure.Hint != reportIt {
				t.Errorf("%s = %s: %+v", name, value, failure)
			}
		}
	}
}

// A value that is no folder is shown in the characters the file has it in, without the white space around its
// parts: a number is not printed again, and neither is a string's escape.
func TestAValueThatIsNoFolderIsShownAsItIsWritten(t *testing.T) {
	cases := []struct{ written, shown string }{
		{`1.0`, `1.0`},
		{`1e21`, `1e21`},
		{`-0`, `-0`},
		{`1E2`, `1E2`},
		{`1e999`, `1e999`},
		{`"\u002e\u002E"`, `"\u002e\u002E"`},
		{`"a\/\/b"`, `"a\/\/b"`},
		{`[ "src" , 1.50 ]`, `["src",1.50]`},
		{"{ \"2\" : 1,\n\t\"a\" : [ ],  \"1\" : \"x y\" }", `{"2":1,"a":[],"1":"x y"}`},
		{"\"a\\\\<&>\xe2\x80\xa8\"", "\"a\\\\<&>\xe2\x80\xa8\""},
	}
	for _, c := range cases {
		for _, name := range folderNames {
			_, err := ParseFile("ex", []byte(`{"`+name+`": `+c.written+` }`), true, where)
			failure := asError(t, err, c.written)
			if want := "has " + name + " = " + c.shown + ", which is not"; !strings.Contains(failure.Msg, want) {
				t.Errorf("%s = %s: %q, want it to say %q", name, c.written, failure.Msg, want)
			}
		}
	}
}

func TestTheFirstProblemOfAFileIsAnUnknownKeyThenTheModuleFolderThenTheAssetsFolder(t *testing.T) {
	cases := []struct{ document, says string }{
		{`{"assets":7,"dir":8,"more":9}`, `unknown key "more"`},
		{`{"assets":7,"dir":8}`, "has dir = 8"},
		{`{"assets":7,"dir":"src"}`, "has assets = 7"},
	}
	for _, c := range cases {
		_, err := ParseFile("ex", []byte(c.document), true, where)
		if failure := asError(t, err, c.document); !strings.Contains(failure.Msg, c.says) {
			t.Errorf("ParseFile(%s): %q, want it to say %q", c.document, failure.Msg, c.says)
		}
	}
}
