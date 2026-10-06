package jass_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/tools/gen/jass"
)

// The scripts below are hand-written miniatures in the shape of common.j and blizzard.j; none is copied from the
// game's files.

const common = `// a leading comment
type agent extends handle
type widget   extends agent  // trailing comment
type unit extends widget

globals
    constant integer MAX_THINGS = 24
    constant string SLASHES = "http://example"   // the // inside the string is not a comment
    integer array counts
endglobals

native CreateThing takes player id, integer unitid, real x, real y, real face returns unit
constant native GetThing takes nothing returns unit
native DoNothing takes code func returns nothing
`

const blizzard = `globals
    real bj_ANGLE = 0.0
endglobals

function HelperBJ takes unit whichUnit, boolean flag returns nothing
    local integer i = 0
    // not a declaration
    call DoNothing(null)
endfunction

constant function ConstantBJ takes nothing returns integer
    return 1
endfunction
`

// corners has what the two scripts above have not: lines that end with a carriage return and a line feed, a tab
// and a carriage return alone between two words, a line in a body that starts with endfunction and does not end
// the body, and a body that holds a line like a declaration. Its string with a quote after a backslash is read
// the same however a comment is cut from that line: the script refused as m.j is what holds that.
var corners = strings.Join([]string{
	"type\tagent\textends\thandle\t",
	"globals",
	`  string QUOTED = "a\"//b" // "c`,
	"  integer\rarray\rcounts",
	"  real unset // = 1",
	"endglobals",
	"native F takes nothing\rreturns nothing",
	"function G takes integer a , real b returns integer",
	"  native Inside takes nothing returns nothing",
	"  endfunctions",
	"  endfunction_",
	"endfunction // done",
	"constant function H takes nothing returns nothing",
	"endfunction",
}, "\r\n")

// indented has a comment after globals, white space before and after endglobals, and an endfunction that is
// indented: each ends its block all the same, so the native after them is read.
const indented = "globals // g\n  integer a\n\tendglobals \nfunction F takes nothing returns nothing\n" +
	"\tendfunction\nnative N takes nothing returns nothing\n"

// noDeclaration is a script of white space and a comment.
const noDeclaration = " \n// nothing\n"

func parse(t *testing.T, text, source string) jass.File {
	t.Helper()
	file, err := jass.Parse(text, source)
	if err != nil {
		t.Fatal(err)
	}
	return file
}

func TestParseReadsTypesNativesAndGlobalsFromCommonJ(t *testing.T) {
	file := parse(t, common, "common.j")
	types := []jass.Type{
		{Name: "agent", Extends: "handle"}, {Name: "widget", Extends: "agent"}, {Name: "unit", Extends: "widget"},
	}
	if !reflect.DeepEqual(file.Types, types) {
		t.Errorf("types: %+v", file.Types)
	}
	globals := []jass.Global{
		{Name: "MAX_THINGS", Source: "common.j", Type: "integer", Constant: true},
		{Name: "SLASHES", Source: "common.j", Type: "string", Constant: true},
		{Name: "counts", Source: "common.j", Type: "integer", Array: true},
	}
	if !reflect.DeepEqual(file.Globals, globals) {
		t.Errorf("globals: %+v", file.Globals)
	}
	functions := []jass.Function{
		{Name: "CreateThing", Source: "common.j", Returns: "unit", Params: []jass.Param{
			{Type: "player", Name: "id"}, {Type: "integer", Name: "unitid"}, {Type: "real", Name: "x"},
			{Type: "real", Name: "y"}, {Type: "real", Name: "face"},
		}},
		{Name: "GetThing", Source: "common.j", Constant: true, Params: []jass.Param{}, Returns: "unit"},
		{Name: "DoNothing", Source: "common.j", Params: []jass.Param{{Type: "code", Name: "func"}}, Returns: "nothing"},
	}
	if !reflect.DeepEqual(file.Functions, functions) {
		t.Errorf("functions: %+v", file.Functions)
	}
}

func TestParseReadsBlizzardJFunctionHeadersAndSkipsTheirBodies(t *testing.T) {
	file := parse(t, blizzard, "blizzard.j")
	if !reflect.DeepEqual(file.Globals, []jass.Global{{Name: "bj_ANGLE", Source: "blizzard.j", Type: "real"}}) {
		t.Errorf("globals: %+v", file.Globals)
	}
	want := []jass.Function{
		{Name: "HelperBJ", Source: "blizzard.j", Returns: "nothing", Params: []jass.Param{
			{Type: "unit", Name: "whichUnit"}, {Type: "boolean", Name: "flag"},
		}},
		{Name: "ConstantBJ", Source: "blizzard.j", Constant: true, Params: []jass.Param{}, Returns: "integer"},
	}
	if !reflect.DeepEqual(file.Functions, want) {
		t.Errorf("functions: %+v", file.Functions)
	}
	if file.Types == nil || len(file.Types) != 0 {
		t.Errorf("types: %#v, want a list that is empty and not nil", file.Types)
	}
}

func TestParseReadsTheCornersOfALine(t *testing.T) {
	want := jass.File{
		Types: []jass.Type{{Name: "agent", Extends: "handle"}},
		Globals: []jass.Global{
			{Name: "QUOTED", Source: "corners.j", Type: "string"},
			{Name: "counts", Source: "corners.j", Type: "integer", Array: true},
			{Name: "unset", Source: "corners.j", Type: "real"},
		},
		Functions: []jass.Function{
			{Name: "F", Source: "corners.j", Params: []jass.Param{}, Returns: "nothing"},
			{Name: "G", Source: "corners.j", Returns: "integer", Params: []jass.Param{
				{Type: "integer", Name: "a"}, {Type: "real", Name: "b"},
			}},
			{Name: "H", Source: "corners.j", Constant: true, Params: []jass.Param{}, Returns: "nothing"},
		},
	}
	if got := parse(t, corners, "corners.j"); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
	empty := jass.File{Types: []jass.Type{}, Functions: []jass.Function{}, Globals: []jass.Global{}}
	if got := parse(t, noDeclaration, "empty.j"); !reflect.DeepEqual(got, empty) {
		t.Errorf("a script without a declaration: %#v", got)
	}
}

// comments has a comment of two slashes that end their line, and comments with a carriage return in them, which
// no line that is read holds after an equals sign: after a string of one character, and after a string that ends
// with a backslash that a backslash escapes.
const comments = "type agent extends handle//\n" +
	"globals\n" +
	"string ONE = \"a\" // one\rtwo\n" +
	"string BACK = \"a\\\\\" // one\rtwo\n" +
	"endglobals//\n"

func TestACommentIsCutFromItsLineWhereTwoSlashesStandOutsideAString(t *testing.T) {
	want := jass.File{
		Types:     []jass.Type{{Name: "agent", Extends: "handle"}},
		Functions: []jass.Function{},
		Globals: []jass.Global{
			{Name: "ONE", Source: "comments.j", Type: "string"},
			{Name: "BACK", Source: "comments.j", Type: "string"},
		},
	}
	if got := parse(t, comments, "comments.j"); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

func TestABlockEndsWhateverStandsAroundTheWordThatEndsIt(t *testing.T) {
	want := jass.File{
		Types:   []jass.Type{},
		Globals: []jass.Global{{Name: "a", Source: "indented.j", Type: "integer"}},
		Functions: []jass.Function{
			{Name: "F", Source: "indented.j", Params: []jass.Param{}, Returns: "nothing"},
			{Name: "N", Source: "indented.j", Params: []jass.Param{}, Returns: "nothing"},
		},
	}
	if got := parse(t, indented, "indented.j"); !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

// escape is a backslash, the letter u and the four digits: how a message writes a character it does not write as
// it is.
func escape(digits string) string { return `\` + "u" + digits }

// refused are the scripts that are refused, each with the place and the distinguishing words of its error.
var refused = []struct{ text, source, place, words string }{
	{"type unit extends widget\nlibrary Foo\n", "common.j", "common.j:2: ", `cannot read "library Foo"`},
	{"function F takes nothing returns nothing\n", "blizzard.j", "blizzard.j:1: ", "never reaches endfunction"},
	{"globals\n    what is this\nendglobals\n", "common.j", "common.j:2: ", `cannot read "what is this"`},
	{"native A takes nothing returns nothing\n\nfunction F takes nothing returns nothing\nendfunctions\n", "b.j",
		"b.j:3: ", "never reaches endfunction"},
	{"globals\ninteger a\n", "c.j", "c.j: ", "never reaches endglobals"},
	{"native F takes integer returns nothing\n", "d.j", "d.j:1: ", "cannot read"},
	{"native F takes integer a, returns nothing\n", "e.j", "e.j:1: ", "cannot read"},
	{"endglobals\n", "f.j", "f.j:1: ", `cannot read "endglobals"`},
	{"globals\nnative F takes nothing returns nothing\nendglobals\n", "g.j", "g.j:2: ", "cannot read"},
	// A carriage return alone is white space between two words, and no character of a parameter list or of what
	// follows an equals sign.
	{"native F takes integer a,\rinteger b returns nothing\n", "h.j", "h.j:1: ", `integer a,\rinteger b returns`},
	{"globals\nstring S = \"a\rb\"\nendglobals\n", "i.j", "i.j:2: ", `cannot read "string S = \"a\rb\""`},
	// Two slashes inside a string start no comment, so the carriage return after them is still in the line.
	{"globals\nstring S = \"//a\rb\"\nendglobals\n", "l.j", "l.j:2: ", `cannot read "string S = \"//a\rb\""`},
	// A quote after a backslash does not end its string, so the two slashes after it are inside the string too.
	{"globals\nstring S = \"a\\\"//\rb\"\nendglobals\n", "m.j", "m.j:2: ", `cannot read "string S = \"a\\\"//\rb\""`},
	// The line is quoted as it is written, comment and all, without the white space at its ends: the quote and
	// the backslash have a backslash before them, a control character is written as an escape, and the markup
	// characters, DEL and a character outside ASCII are written as they are.
	{"\t call F(\"a\\b\")\v\x01\x7F<&>\xC3\xA9  // why \f\r\n", "j.j", "j.j:1: ",
		`cannot read "call F(\"a\\b\")` + escape("000b") + escape("0001") + "\x7F<&>\xC3\xA9  // why" + `"`},
	{"library\bA\fB\tC\n", "k.j", "k.j:1: ", `cannot read "library\bA\fB\tC"`},
}

func TestParseNamesTheFileAndLineOfAnythingItDoesNotUnderstand(t *testing.T) {
	for _, c := range refused {
		file, err := jass.Parse(c.text, c.source)
		if err == nil || !strings.HasPrefix(err.Error(), c.place) || !strings.Contains(err.Error(), c.words) {
			t.Errorf("%s: got %v, want an error at %q with %q", c.source, err, c.place, c.words)
		}
		if file.Types != nil || file.Functions != nil || file.Globals != nil {
			t.Errorf("%s: a refused script is %+v, want none", c.source, file)
		}
	}
}

func TestAnErrorIsThePlaceAndWhatIsWrongThere(t *testing.T) {
	for text, want := range map[string]string{
		"type unit extends widget\n  library Foo \n":           `x.j:2: cannot read "library Foo"`,
		"\nfunction F takes nothing returns nothing\nreturn\n": "x.j:2: the function never reaches endfunction",
		"globals\ninteger a\n":                                 "x.j: the globals block never reaches endglobals",
	} {
		if _, err := jass.Parse(text, "x.j"); err == nil || err.Error() != want {
			t.Errorf("%q: got %v, want %s", text, err, want)
		}
	}
}

// widerSpace are scripts with a character that is white space outside ASCII. Each is text: it parts no two
// words, it is trimmed from no line, and inside a line it is a character like any other. A case with words is a
// script that is refused; one without is read into want.
var widerSpace = []struct {
	name, text   string
	place, words string
	want         jass.File
}{
	{name: "a no-break space between two words",
		text:  "type agent extends handle\ntype\xC2\xA0unit extends agent\n",
		place: "wide.j:2: ", words: "cannot read \"type\xC2\xA0unit extends agent\""},
	{name: "a byte order mark before a declaration that is not the first line",
		text:  "type agent extends handle\n\xEF\xBB\xBFnative F takes nothing returns nothing\n",
		place: "wide.j:2: ", words: "cannot read \"\xEF\xBB\xBFnative F takes nothing returns nothing\""},
	{name: "a line separator in what follows an equals sign",
		text: "globals\nstring S = \"a\xE2\x80\xA8b\"\nendglobals\n",
		want: jass.File{Types: []jass.Type{}, Functions: []jass.Function{},
			Globals: []jass.Global{{Name: "S", Source: "wide.j", Type: "string"}}}},
	{name: "a line and a paragraph separator in a line that is refused, each quoted as an escape",
		text:  "library\xE2\x80\xA8Foo\xE2\x80\xA9\n",
		place: "wide.j:1: ", words: `cannot read "library` + escape("2028") + "Foo" + escape("2029") + `"`},
	{name: "a no-break space in a comment and an ideographic space in a string",
		text: "type agent extends handle //\xC2\xA0note\nglobals\nstring S = \"a\xE3\x80\x80b\"\nendglobals\n",
		want: jass.File{Types: []jass.Type{{Name: "agent", Extends: "handle"}}, Functions: []jass.Function{},
			Globals: []jass.Global{{Name: "S", Source: "wide.j", Type: "string"}}}},
}

func TestWhiteSpaceOutsideASCIIIsText(t *testing.T) {
	for _, c := range widerSpace {
		file, err := jass.Parse(c.text, "wide.j")
		switch {
		case c.words == "" && (err != nil || !reflect.DeepEqual(file, c.want)):
			t.Errorf("%s: got %+v and the error %v, want %+v", c.name, file, err, c.want)
		case c.words == "":
		case err == nil || !strings.HasPrefix(err.Error(), c.place) || !strings.Contains(err.Error(), c.words):
			t.Errorf("%s: got %v, want an error at %q with %q", c.name, err, c.place, c.words)
		}
	}
}
