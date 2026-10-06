package lua

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

type namedSource struct{ name, source string }

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
	{"a letter that is not ASCII", "\xC3\xA9 = 1 \xC3\xA9t\xC3\xA9 = 2"},
	{"bytes that are not UTF-8", "a = \xFF\xFE b \xC3"},
	{"text that is not ASCII in a string", "a = '\xC3\xA9\U0001F319' b = [[\xC3\xA9]] -- \xC3\xA9\nc = \"\xFF\""},
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
	// A string that holds `local` or `function` passes for the keyword.
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

// luaFilesOfTheCheckout are the Lua files the checkout holds for good: the two scripts World Editor saved, the
// template's map script and the runtime. Each has its path below the root, with "/", for a name.
func luaFilesOfTheCheckout(t *testing.T) []namedSource {
	t.Helper()
	var sources []namedSource
	for _, name := range []string{
		"internal/testkit/testdata/map-globals-we3/war3map.lua",
		"internal/testkit/testdata/map-settings-v39/war3map.lua",
		"template/maps/map.w3x/war3map.lua",
		"runtime/moonwell.lua",
	} {
		data, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		sources = append(sources, namedSource{name, string(data)})
	}
	return sources
}

// scanned is what every scanner of the package makes of one source.
type scanned struct {
	Tokens    []Token
	Fault     *Fault
	Requires  []Require
	Globals   []string
	Map       MapGlobals
	Functions []Function
	// Refusal is what Functions says of a source it refuses; its message is empty for a source that is read.
	Refusal testkit.Refusal
}

// scan gives the source to every scanner.
func scan(source string) scanned {
	made := scanned{Requires: Requires(source), Globals: TopLevelGlobals(source), Map: ReadMapGlobals(source)}
	made.Tokens, made.Fault = Tokenize(source)
	functions, err := Functions(source, "war3map.lua")
	if err != nil {
		made.Refusal = testkit.RefusalOf("", err)
	}
	made.Functions = functions
	return made
}

// longest is the most tokens, and the most bytes of a source, that a recording writes out; of more it holds the
// digest.
const longest = 80

// rawsOf is the source text of each token, with a space between two.
func rawsOf(tokens []Token) string {
	var raws []string
	for _, token := range tokens {
		raws = append(raws, token.Raw)
	}
	return strings.Join(raws, " ")
}

// tokenLines is one line for each token: its line, its bytes, its kind and its source text as Go quotes a
// string, and its text where that is another, and whether it has an escape.
func tokenLines(tokens []Token) string {
	kinds := map[Kind]string{NameToken: "name", NumberToken: "number", StringToken: "string", SymbolToken: "symbol"}
	var lines strings.Builder
	for _, token := range tokens {
		fmt.Fprintf(&lines, "  %d:%d-%d %s %q", token.Line, token.Start, token.End, kinds[token.Kind], token.Raw)
		if token.Text != token.Raw {
			fmt.Fprintf(&lines, " text %q", token.Text)
		}
		if token.Escaped {
			lines.WriteString(" escaped")
		}
		lines.WriteString("\n")
	}
	return lines.String()
}

// text is the scan of a source as a recording holds it: a line or a few for what each scanner made. A source
// or a list of tokens that is long stands as its digest.
func (s scanned) text(name, source string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "== %s\n", name)
	if len(source) > longest {
		fmt.Fprintf(&out, "source: %s\n", testkit.Digest([]byte(source)))
	} else {
		fmt.Fprintf(&out, "source: %q\n", source)
	}
	if lines := tokenLines(s.Tokens); len(s.Tokens) > longest {
		fmt.Fprintf(&out, "tokens: %d, %s\n", len(s.Tokens), testkit.Digest([]byte(lines)))
	} else {
		fmt.Fprintf(&out, "tokens: %d\n%s", len(s.Tokens), lines)
	}
	if s.Fault != nil {
		fmt.Fprintf(&out, "fault: %s at %d\n", s.Fault.Msg, s.Fault.Offset)
	}
	for _, require := range s.Requires {
		fmt.Fprintf(&out, "require: line %d, %q, literal %v\n", require.Line, require.Name, require.Literal)
	}
	if len(s.Globals) > 0 {
		fmt.Fprintf(&out, "top-level globals: %s\n", strings.Join(s.Globals, " "))
	}
	for _, global := range s.Map.Globals {
		fmt.Fprintf(&out, "map global: %s %s\n", global.Name, global.Type)
	}
	if len(s.Map.Functions) > 0 {
		fmt.Fprintf(&out, "map functions: %s\n", strings.Join(s.Map.Functions, " "))
	}
	for _, function := range s.Functions {
		fmt.Fprintf(&out, "function %s: %d-%d, its end at %d\n", function.Name, function.Start, function.End, function.EndStart)
		for _, call := range function.Calls {
			var arguments []string
			for _, argument := range call.Args {
				arguments = append(arguments, rawsOf(argument))
			}
			text := strings.Join(arguments, " , ")
			if len(text) > longest {
				text = testkit.Digest([]byte(text))
			}
			fmt.Fprintf(&out, "  call %s: %d-%d, %d arguments: %s\n", call.Name, call.Start, call.End, len(call.Args), text)
		}
	}
	if s.Refusal.Message != "" {
		fmt.Fprintf(&out, "functions refused at %d:%d: %s\n", s.Refusal.Line, s.Refusal.Column, s.Refusal.Message)
	}
	return out.String() + "\n"
}

// scans is the recording of what scan makes of each source.
func scans(sources []namedSource, scan func(source string) scanned) []byte {
	var text strings.Builder
	for _, c := range sources {
		text.WriteString(scan(c.source).text(c.name, c.source))
	}
	return []byte(text.String())
}

// TestTheScannersAreAsRecorded holds what the tokenizer and the four scanners make of every corner source, and
// of the Lua files of the checkout, to two recordings. A change of what any of them reads is a line of a diff
// that names the source.
func TestTheScannersAreAsRecorded(t *testing.T) {
	testkit.Recorded(t, "corners.txt", scans(cornerSources, scan))
	testkit.Recorded(t, "files.txt", scans(luaFilesOfTheCheckout(t), scan))
}
