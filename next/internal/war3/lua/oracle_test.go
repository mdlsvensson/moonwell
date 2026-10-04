package lua

import (
	"errors"
	"io/fs"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/luasrc"
	"github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// read is what a literal helper returned.
type read struct {
	Value float64
	OK    bool
}

// compareScanners runs the other tree's package and this one on one source, and compares everything each reads
// from it.
func compareScanners(t *testing.T, what, source string) {
	t.Helper()
	wantTokens, wantFault := luasrc.Tokenize(source)
	gotTokens, gotFault := Tokenize(source)
	oracle.Values(t, what+": tokens", wantTokens, gotTokens)
	oracle.Values(t, what+": fault", wantFault, gotFault)
	oracle.Values(t, what+": requires", luasrc.Requires(source), Requires(source))
	oracle.Values(t, what+": top-level globals", luasrc.TopLevelGlobals(source), TopLevelGlobals(source))
	oracle.Values(t, what+": map globals", luasrc.ReadMapGlobals(source), ReadMapGlobals(source))
	compareLiterals(t, what+": the tokens as a literal", wantTokens, gotTokens)

	wantFunctions, wantErr := luasrc.Functions(source, "war3map.lua")
	gotFunctions, gotErr := Functions(source, "war3map.lua")
	if oracle.Errors(t, what+": functions", wantErr, gotErr) {
		return
	}
	oracle.Values(t, what+": functions", wantFunctions, gotFunctions)
	if t.Failed() {
		return // the two lists may not pair
	}
	for i, function := range wantFunctions {
		for j, call := range function.Calls {
			for k, argument := range call.Args {
				compareLiterals(t, what+": an argument of "+call.Name, argument, gotFunctions[i].Calls[j].Args[k])
			}
		}
	}
}

// compareLiterals compares what the literal helpers of both sides make of the same tokens.
func compareLiterals(t *testing.T, what string, want []luasrc.Token, got []Token) {
	t.Helper()
	wantNumber, wantOK := luasrc.LiteralNumber(want)
	gotNumber, gotOK := LiteralNumber(got)
	oracle.Values(t, what+": LiteralNumber", read{wantNumber, wantOK}, read{gotNumber, gotOK})
	wantID, wantOK := luasrc.PlayerID(want)
	gotID, gotOK := PlayerID(got)
	oracle.Values(t, what+": PlayerID", read{float64(wantID), wantOK}, read{float64(gotID), gotOK})
}

// luaFiles returns every .lua file below dir, and none when there is no such folder.
func luaFiles(t *testing.T, dir string) []string {
	t.Helper()
	var files []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err == nil && !entry.IsDir() && strings.HasSuffix(path, ".lua") {
			files = append(files, path)
		}
		return err
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	return files
}

func TestOracleOnTheLuaFilesOfTheCheckout(t *testing.T) {
	root := testkit.RepoRoot(t)
	for _, source := range []struct {
		dir string
		// required is false for a folder that is beside the checkout on a developer's machine only.
		required bool
	}{
		{filepath.Join(root, "template"), true},
		{filepath.Join(root, "runtime"), true},
		{filepath.Join(root, "internal", "testkit", "testdata"), true},
		{filepath.Join(root, "..", "moonwell-wrappers", "src"), false},
		{filepath.Join(root, "..", "moonwell-systems", "src"), false},
	} {
		files := luaFiles(t, source.dir)
		t.Logf("%d Lua files under %s", len(files), source.dir)
		if source.required && len(files) == 0 {
			t.Errorf("no Lua file under %s", source.dir)
		}
		for _, file := range files {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			compareScanners(t, file, string(data))
		}
	}
}

type namedSource struct{ name, source string }

// oracleSources returns the sources of this package's tests, each with a name for a failure: the ones a scanner
// refuses or finds a fault in, the ones the tests read, and the ones at the corners of the grammar.
func oracleSources() []namedSource {
	sources := []namedSource{
		{"an open string before a require", afterAnOpenString},
		{"escaped strings", escapedStrings},
		{"direct calls", directCalls},
		{"every expression", everyExpression},
		{"token boundaries", tokenBoundaries},
		{"adjacent statements", adjacentStatements},
		{"declaration forms", declarationForms},
		{"every spelling", everySpelling},
		{"a chain of ..", longChain("..")},
		{"a chain of ^", longChain("^")},
	}
	for _, c := range malformedSources {
		sources = append(sources, namedSource{"malformed: " + c.source, c.source})
	}
	for _, list := range [][]refusal{ambiguousStructures, deepNesting, invalidShapes, placedErrors} {
		for _, c := range list {
			sources = append(sources, namedSource{"refused: " + strconv.Quote(c.source[:min(len(c.source), 60)]), c.source})
		}
	}
	for _, c := range literalNumbers {
		sources = append(sources, namedSource{"literal: " + c.source, c.source})
	}
	for _, c := range playerIDs {
		sources = append(sources, namedSource{"player: " + c.source, c.source})
	}
	return append(sources, cornerSources...)
}

// nested is `(((1)))` with the given number of brackets, as the argument of a call statement.
func nested(brackets int) string {
	return "function config() Capture(" + strings.Repeat("(", brackets) + "1" + strings.Repeat(")", brackets) + ") end"
}

// cornerSources are sources at the edges of what the tokenizer, the scanners and the line reader take: line breaks
// of every kind, escapes at the end of a string or of the source, numerals that almost are, bytes that are not
// text, and nesting on both sides of the limit.
var cornerSources = []namedSource{
	{"empty", ""},
	{"only white space", " \t\v\f\r\n"},
	{"a backslash, a line feed, a return", "a = 'x\\\n\ry' b"},
	{"a backslash and a lone return", "a = 'x\\\ry' b\nc"},
	{"a backslash ends the source", "a = 'x\\"},
	{"\\z ends the source", "a = \"x\\z  \n\t "},
	{"\\z before the closing quote", "a = \"x\\z\n\n\" b"},
	{"an escaped quote", `a = "x\"y" b = 'x\'y' c = "\\" d`},
	{"a comment ended by a return", "x = 1 -- one\ry = 2 --[ not long\nz = 3"},
	{"long brackets of several levels", "a = [==[ ]] ]=] ]==] b = [[\r\nfirst]] c = [=[\nsecond]=] d = [[\rthird]]"},
	{"a long string that starts with two line breaks", "x = [[\n\nfirst]] y = [[\r\n\r\nsecond]]"},
	{"a long comment with lines", "--[==[\n\n]==] a --[[\n]] b"},
	{"a bracket that is not long", "a[=1] = b[ [[x]] ] c = [= d"},
	{"numerals", "1 1. .5 1.5e10 1e+5 1E-5 0x1F 0X.8p-2 0x1.fp+2 0xA. 3..4 5...6 7.e2"},
	{"numerals that are not", "1e 0x 0xg 1.2.3 1..2.3 0x1p 1e+ 3a 4_ 0x.p1 1.e 08.e+x"},
	{"every symbol", "+ - * / % ^ # & ~ | < > = ( ) { } [ ] ; : , . .. ... // << >> == ~= <= >= ::"},
	{"symbols without spaces", "a<<=b>>=c~==d...e....f:::g"},
	{"symbols Lua does not have", "a ! b ? c $ d ` e \\ f"},
	{"a letter that is not ASCII", "é = 1 été = 2"},
	{"bytes that are not UTF-8", "a = \xFF\xFE b \xC3"},
	{"text that is not ASCII in a string", "a = 'é\U0001F319' b = [[é]] -- é\nc = \"\xFF\""},
	{"a byte order mark inside a string", "a = '\xEF\xBB\xBF' -- \xEF\xBB\xBF"},
	{"a no-break space inside a comment", "a = 1 --\xC2\xA0x\nb = [[\xC2\xA0]]"},
	{"a line separator inside a string", "local a = '\xE2\x80\xA8' b = 2 -- \xE2\x80\xA9"},
	{"calls without brackets", "f'x' g\"y\" h[[z]] i{} a.b:c 'd' require'm'"},
	{"a require of every form", "require 'a' require(\"b\") require [[c]] require('d', e) require(f) x.require 'g'"},
	{"a require at the end", "require"},
	{"a require before an open bracket", "require ("},
	{"globals after semicolons and breaks", "A = 1; B, C = 2, 3\rD = 4 E = 5\nF, G\n= 6\nH == 7\nfunction I() end"},
	{"globals inside brackets and blocks", "t = { A = 1 }\nf(\nB = 2)\nwhile x do\nC = 3\nend\nrepeat\nD = 4\nuntil E\nF = 5"},
	{"locals of every form", "local A, B\nlocal function C() end\nlocal function\nlocal\nA = 1\nB = 2\nC = 3\nD = 4"},
	// A string that holds `local` or `function` passes for the keyword in both trees.
	{"a string that holds a keyword", "print 'local' function F() end\nlocal 'function' G\nG = 1\nf [[local]] function H() end"},
	{"ends without a block", "end end\nA = 1\n) ]\nB = 2"},
	{"declaration lines", "udg_A   =   5  \r\nudg_B=-3\nudg_C = -.5\nudg_D = 1.\nudg_E = \"x\" -- y\nudg_F = true\nudg_G==1"},
	{"array declarations", "udg_A = __jarray( \"\" )\nudg_B = __jarray(__jarray(0))\nudg_C = __jarray({}) \nudg_D = __jarray()"},
	{"handle declarations", "gg_snd_A = nil\ngg_dest_B = nil\ngg_item_C = 5\ngg_xyz_D = 5\ngg_Trg_E = 5\ngg_cam_ = 1\ngg__F = 2"},
	{"lines that are not declarations", " udg_A = 1\nlocal udg_B = 1\nudg_C.x = 1\n\tudg_D = 1\nudg_E\n= 1\n1udg = 2"},
	{"function lines", "function  Spaced  ()\nend\n function Indented()\nfunction a.b()\nfunction\tTabbed\t(\nudg_Late = 1"},
	{"two functions on a line", "function A()function B()\nend"},
	{"declarations with tabs and returns", "udg_A\t=\t5\t\r\nudg_B =\v1\f\nudg_C = 1\r"},
	{"a declaration with a lone return", "udg_A = 1\rudg_B = 2\nudg_C = 3"},
	{"a function line with a lone return", "udg_A = 1\rfunction main()\nudg_B = 2"},
	{"a call and its semicolon", "function config() A() ; B() --[[ c ]] ; C()\n; D() -- c\n; E();; end"},
	{"functions of every form", "function a() end local function b() end function c.d() end function e:f() end g = function() end"},
	{"a function in a function", "function a() function b() X() end Y() end function a() end"},
	{"a method call of a string", "function a() X():y 'z' X{}.a() X 'a' 'b' X.y() end"},
	{"a return of every form", "function a() return end function b() return; end function c() return 1, 2; end return a"},
	{"a statement after a return", "function a() return 1; X() end"},
	{"a for of every form", "function a() for i = 1, 2 do end for i = 1, 2, 3 do end for k, v in next, t do end end"},
	{"a label and a goto", "function a() ::top:: goto top end"},
	{"unary operators in a row", "function a() local x = - - not # ~ 1 ^ - 2 X(-x) end"},
	{"an operator without an operand", "function a() local x = 1 + end"},
	{"brackets at the limit", nested(197)},
	{"brackets one past the limit", nested(198)},
	{"blocks at the limit", strings.Repeat("do ", 199) + strings.Repeat("end ", 199)},
	{"blocks one past the limit", strings.Repeat("do ", 200) + strings.Repeat("end ", 200)},
	{"tables at the limit", "x = " + strings.Repeat("{", 198) + strings.Repeat("}", 198)},
	{"tables one past the limit", "x = " + strings.Repeat("{", 199) + strings.Repeat("}", 199)},
	{"a negative number with a space", "function a() X(- 1, -1, - -1, -0x10, 0X10, 1E5, -0) end"},
	{"a hexadecimal number too long", "function a() X(0x" + strings.Repeat("F", 300) + ", -0x" + strings.Repeat("f", 300) + ") end"},
	{"a hexadecimal number that rounds", "function a() X(0x20000000000001, 0x20000000000003, 0x1FFFFFFFFFFFFF, 0x0) end"},
	{"a decimal number at the edges", "function a() X(1e308, 1e309, 1e-400, 4.9e-324, 00012, 1.e5, .0) end"},
	{"a player of every form", "function a() X(Player(1e2), Player(-0), Player(0x7fffffff), Player(1e30), Player (3)) end"},
}

func TestOracleOnTheSourcesOfTheTests(t *testing.T) {
	sources := oracleSources()
	t.Logf("%d sources", len(sources))
	for _, c := range sources {
		compareScanners(t, c.name, c.source)
	}
}

// pieces are what a mutation puts into a source: the bits of Lua that change what the text around them is.
var pieces = []string{
	"\"", "'", "\\", "[", "]", "[[", "]]", "[=[", "]=]", "--", "--[[", "\n", "\r", "\r\n", "\n\r", " ", "\t", "\v", "\f",
	"\\z", "\\\n", "\\\r\n", "\\\r", "0", "1", "9", ".", "..", "...", "=", "==", "-", "+", "e", "E", "p", "x", "0x",
	"(", ")", "{", "}", ",", ";", ":", "::", "~", "#", "//", "<<", "@", "a", "_", "\xFF", "é", "\x00", "\x7F",
	"end", " end ", "function", " function ", "local ", " do ", " if ", " then ", " return ", " until ", "repeat ",
	"not ", "nil", "for ", " in ", "while ", "goto ", "else ", "elseif ", " = ", "require", "Player(", "__jarray(",
	"\nudg_A = ", "\nfunction F()", "gg_trg_",
}

// mutate changes a source in up to four places: a piece put in, a few bytes taken out, or both at once.
func mutate(random *rand.Rand, source string) string {
	for range 1 + random.IntN(4) {
		at := random.IntN(len(source) + 1)
		end := at
		if random.IntN(3) > 0 {
			end = min(len(source), at+random.IntN(6))
		}
		piece := ""
		if random.IntN(3) > 0 {
			piece = pieces[random.IntN(len(pieces))]
		}
		source = source[:at] + piece + source[end:]
	}
	return source
}

// hasWiderSpace reports whether the text has a character that the other tree's tokenizer skips as white space and
// Lua does not. The two trees read such a source differently, and that is meant.
func hasWiderSpace(text string) bool {
	return strings.ContainsFunc(text, func(r rune) bool {
		return (r >= 0x2000 && r <= 0x200A) ||
			slices.Contains([]rune{0xA0, 0x1680, 0x2028, 0x2029, 0x202F, 0x205F, 0x3000, 0xFEFF}, r)
	})
}

// The sources of the tests, each broken in twenty ways that are the same on every run: most are no longer Lua, which
// is where the two trees must still agree on every token, fault and refusal.
func TestOracleOnMutatedSources(t *testing.T) {
	random := rand.New(rand.NewPCG(8, 2026))
	compared := 0
	for _, c := range oracleSources() {
		if len(c.source) > 3000 {
			continue
		}
		for range 20 {
			mutated := mutate(random, c.source)
			if hasWiderSpace(mutated) {
				continue
			}
			compareScanners(t, c.name+", mutated into "+strconv.Quote(mutated), mutated)
			compared++
			if t.Failed() {
				t.FailNow()
			}
		}
	}
	t.Logf("%d mutated sources", compared)
}

func TestOracleOnQuote(t *testing.T) {
	values := []string{
		"", "plain", `a"b\c`, "it's", "\n123", "\r\n", "\t", "\x00\x01\x1F\x20\x7E\x7F\x80", "é", "\xC2\xA0",
		"\xE2\x80\xA8", "\U0001F319", "\xFF\xFE", "\xC3", "a\x1Bb", `\\"`, "war3mapImported\\minimap.blp",
	}
	for c := range 256 {
		values = append(values, string([]byte{'a', byte(c), '1'}))
	}
	for _, value := range values {
		oracle.Bytes(t, "Quote of "+strconv.Quote(value), []byte(settings.LuaString(value)), []byte(Quote(value)))
	}
}
