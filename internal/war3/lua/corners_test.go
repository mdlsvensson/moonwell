package lua

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

type namedSource struct{ name, source string }

func nested(brackets int) string {
	return "function config() Capture(" + strings.Repeat("(", brackets) + "1" + strings.Repeat(")", brackets) + ") end"
}

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
	{"globals inside brackets and blocks",
		"t = { A = 1 }\nf(\nB = 2)\nwhile x do\nC = 3\nend\nrepeat\nD = 4\nuntil E\nF = 5"},
	{"locals of every form", "local A, B\nlocal function C() end\nlocal function\nlocal\nA = 1\nB = 2\nC = 3\nD = 4"},
	{"a string that holds a keyword",
		"print 'local' function F() end\nlocal 'function' G\nG = 1\nf [[local]] function H() end"},
	{"ends without a block", "end end\nA = 1\n) ]\nB = 2"},
	{"declaration lines",
		"udg_A   =   5  \r\nudg_B=-3\nudg_C = -.5\nudg_D = 1.\nudg_E = \"x\" -- y\nudg_F = true\nudg_G==1"},
	{"array declarations",
		"udg_A = __jarray( \"\" )\nudg_B = __jarray(__jarray(0))\nudg_C = __jarray({}) \nudg_D = __jarray()"},
	{"handle declarations",
		"gg_snd_A = nil\ngg_dest_B = nil\ngg_item_C = 5\ngg_xyz_D = 5\ngg_Trg_E = 5\ngg_cam_ = 1\ngg__F = 2"},
	{"lines that are not declarations", " udg_A = 1\nlocal udg_B = 1\nudg_C.x = 1\n\tudg_D = 1\nudg_E\n= 1\n1udg = 2"},
	{"function lines",
		"function  Spaced  ()\nend\n function Indented()\nfunction a.b()\nfunction\tTabbed\t(\nudg_Late = 1"},
	{"two functions on a line", "function A()function B()\nend"},
	{"declarations with tabs and returns", "udg_A\t=\t5\t\r\nudg_B =\v1\f\nudg_C = 1\r"},
	{"a declaration with a lone return", "udg_A = 1\rudg_B = 2\nudg_C = 3"},
	{"a function line with a lone return", "udg_A = 1\rfunction main()\nudg_B = 2"},
	{"a call and its semicolon", "function config() A() ; B() --[[ c ]] ; C()\n; D() -- c\n; E();; end"},
	{"functions of every form",
		"function a() end local function b() end function c.d() end function e:f() end g = function() end"},
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
	{"a hexadecimal number too long",
		"function a() X(0x" + strings.Repeat("F", 300) + ", -0x" + strings.Repeat("f", 300) + ") end"},
	{"a hexadecimal number that rounds", "function a() X(0x20000000000001, 0x20000000000003, 0x1FFFFFFFFFFFFF, 0x0) end"},
	{"a decimal number at the edges", "function a() X(1e308, 1e309, 1e-400, 4.9e-324, 00012, 1.e5, .0) end"},
	{"a player of every form",
		"function a() X(Player(1e2), Player(-0), Player(0x7fffffff), Player(1e30), Player (3)) end"},
}

func checkoutLuaFiles(t *testing.T) []namedSource {
	t.Helper()
	var sources []namedSource
	for _, name := range []string{
		"internal/testkit/testdata/map-globals-we3/war3map.lua",
		"internal/testkit/testdata/map-settings-v39/war3map.lua",
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

type literal struct {
	Number   float64
	IsNumber bool
	Player   int
	IsPlayer bool
}

func literalOf(tokens []Token) literal {
	var made literal
	made.Number, made.IsNumber = ParseNumberLiteral(tokens)
	made.Player, made.IsPlayer = ParsePlayerID(tokens)
	return made
}

func (l literal) String() string {
	var parts []string
	if l.IsNumber {
		parts = append(parts, "the number "+strconv.FormatFloat(l.Number, 'g', -1, 64))
	}
	if l.IsPlayer {
		parts = append(parts, "the player "+strconv.Itoa(l.Player))
	}
	return strings.Join(parts, ", ")
}

type reportedError struct {
	File, Message, Hint string
	Line, Column        int
}

type scanResult struct {
	Tokens    []Token
	Fault     *Fault
	Requires  []Require
	Globals   []string
	Map       MapGlobals
	Functions []Function
	Error     reportedError
	Whole     literal
	Arguments []literal
}

func scan(source string) scanResult {
	made := scanResult{Requires: FindRequires(source), Globals: FindTopLevelGlobals(source), Map: ReadMapGlobals(source)}
	made.Tokens, made.Fault = Tokenize(source)
	made.Whole = literalOf(made.Tokens)
	functions, err := ParseFunctions(source, "war3map.lua")
	var diagErr *diag.Error
	switch {
	case errors.As(err, &diagErr):
		made.Error = reportedError{diagErr.File, diagErr.Msg, diagErr.Hint, diagErr.Line, diagErr.Column}
	case err != nil:
		made.Error = reportedError{Message: err.Error()}
	}
	made.Functions = functions
	for _, function := range functions {
		for _, call := range function.Calls {
			for _, argument := range call.Args {
				made.Arguments = append(made.Arguments, literalOf(argument))
			}
		}
	}
	return made
}

const longest = 80

func joinRaws(tokens []Token) string {
	var raws []string
	for _, token := range tokens {
		raws = append(raws, token.Raw)
	}
	return strings.Join(raws, " ")
}

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

func quoteEach(values []string) string {
	var shown []string
	for _, value := range values {
		shown = append(shown, testkit.QuoteIfNeeded(value))
	}
	return strings.Join(shown, " ")
}

func (s scanResult) text(name, source string) string {
	var out strings.Builder
	fmt.Fprintf(&out, "== %s\n", testkit.QuoteIfNeeded(name))
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
	if whole := s.Whole.String(); whole != "" {
		fmt.Fprintf(&out, "the tokens as a literal: %s\n", whole)
	}
	if s.Fault != nil {
		fmt.Fprintf(&out, "fault: %s at %d\n", testkit.QuoteIfNeeded(s.Fault.Msg), s.Fault.Offset)
	}
	for _, require := range s.Requires {
		fmt.Fprintf(&out, "require: line %d, %q, literal %v\n", require.Line, require.Name, require.Literal)
	}
	if len(s.Globals) > 0 {
		fmt.Fprintf(&out, "top-level globals: %s\n", quoteEach(s.Globals))
	}
	for _, global := range s.Map.Globals {
		fmt.Fprintf(&out, "map global: %s %s\n", testkit.QuoteIfNeeded(global.Name), testkit.QuoteIfNeeded(global.Type))
	}
	if len(s.Map.Functions) > 0 {
		fmt.Fprintf(&out, "map functions: %s\n", quoteEach(s.Map.Functions))
	}
	argument := 0
	for _, function := range s.Functions {
		fmt.Fprintf(&out, "function %s: %d-%d, its end at %d\n", testkit.QuoteIfNeeded(function.Name), function.Start,
			function.End, function.EndStart)
		for _, call := range function.Calls {
			fmt.Fprintf(&out, "  call %s: %d-%d, %d arguments\n", testkit.QuoteIfNeeded(call.Name), call.Start, call.End,
				len(call.Args))
			for i, tokens := range call.Args {
				text := joinRaws(tokens)
				if len(text) > longest {
					text = testkit.Digest([]byte(text))
				}
				fmt.Fprintf(&out, "    argument %d: %q", i+1, text)
				if literal := s.Arguments[argument].String(); literal != "" {
					fmt.Fprintf(&out, ", %s", literal)
				}
				out.WriteString("\n")
				argument++
			}
		}
	}
	if s.Error.Message != "" {
		fmt.Fprintf(&out, "functions refused at %d:%d: %s\n", s.Error.Line, s.Error.Column,
			testkit.QuoteIfNeeded(s.Error.Message))
	}
	return out.String() + "\n"
}

func formatScans(sources []namedSource) []byte {
	var text strings.Builder
	for _, c := range sources {
		text.WriteString(scan(c.source).text(c.name, c.source))
	}
	return []byte(text.String())
}

func TestTheScannersAreAsRecorded(t *testing.T) {
	testkit.CheckRecorded(t, "corners.txt", formatScans(cornerSources))
}
