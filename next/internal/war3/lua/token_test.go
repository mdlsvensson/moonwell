package lua

import (
	"slices"
	"testing"
)

// raws returns the source text of each token.
func raws(tokens []Token) []string {
	var out []string
	for _, token := range tokens {
		out = append(out, token.Raw)
	}
	return out
}

func TestTokenizeTracksLinesAcrossLongStringsAndEscapedNewlines(t *testing.T) {
	tokens, fault := Tokenize("x = [[\n\n]]\ny = \"a\\\nb\"\nz")
	if fault != nil {
		t.Fatalf("fault = %+v", fault)
	}
	lines := map[string]int{}
	for _, token := range tokens {
		lines[token.Raw] = token.Line
	}
	if lines["y"] != 4 || lines["z"] != 6 {
		t.Errorf("y is on line %d and z on line %d, want 4 and 6", lines["y"], lines["z"])
	}
}

func TestTheNameScannersSeeNoNumbersAndSymbolsOneCharacterAtATime(t *testing.T) {
	for _, c := range []struct {
		name, source string
		want         []string
	}{
		{"numbers are left out", "x = 1e-5 + 0x1F", []string{"x", "=", "+"}},
		{
			"only a run of dots stays whole",
			"a == b ~= c .. d ... :: <<",
			[]string{"a", "=", "=", "b", "~", "=", "c", "..", "d", "...", ":", ":", "<", "<"},
		},
	} {
		tokens := simpleTokens(c.source)
		if got := raws(tokens); !slices.Equal(got, c.want) {
			t.Errorf("%s: simple tokens = %q, want %q", c.name, got, c.want)
		}
		for _, token := range tokens {
			if c.source[token.Start:token.End] != token.Raw || token.Text != token.Raw {
				t.Errorf("%s: token %q has the text %q and is at %q", c.name, token.Raw, token.Text, c.source[token.Start:token.End])
			}
		}
	}
}

// malformedSources are sources with one malformed token each: the fault it gives, and the tokens read around it.
var malformedSources = []struct {
	source, fault string
	offset        int
	raws          []string
}{
	{"a = \"open\nb = 1", "unescaped newline in quoted string", 4, []string{"a", "=", "\"open", "b", "=", "1"}},
	{"a = 'open\rb = 1", "unescaped newline in quoted string", 4, []string{"a", "=", "'open", "b", "=", "1"}},
	{"a = 'open", "unterminated quoted string", 4, []string{"a", "=", "'open"}},
	{"a = [==[ open ]] b", "unterminated long string or comment", 4, []string{"a", "=", "[==[ open ]] b"}},
	{"a --[[ open", "unterminated long string or comment", 2, []string{"a"}},
	{"a = 0x + 1.2.3 b", "invalid numeral", 4, []string{"a", "=", "0x", "+", "1.2.3", "b"}},
	{"a @ b", "unsupported symbol", 2, []string{"a", "@", "b"}},
	// The backslash is the last byte of the source: it escapes nothing, and the string is not closed.
	{`a = 'open\`, "unterminated quoted string", 4, []string{"a", "=", `'open\`}},
	// A dot alone after the prefix of a hexadecimal numeral is no digits.
	{"a = 0x. + 1", "invalid numeral", 4, []string{"a", "=", "0x.", "+", "1"}},
	// What opens a string or begins a numeral is the last thing in the source.
	{"a = [[", "unterminated long string or comment", 4, []string{"a", "=", "[["}},
	{"a = [==[", "unterminated long string or comment", 4, []string{"a", "=", "[==["}},
	{"a = \"x\\z  ", "unterminated quoted string", 4, []string{"a", "=", "\"x\\z  "}},
	{"a = 1e", "invalid numeral", 4, []string{"a", "=", "1e"}},
	{"a = 0x", "invalid numeral", 4, []string{"a", "=", "0x"}},
	// A malformed numeral takes a sign only after the letter of an exponent, and no bracket.
	{"a = 1e+ b", "invalid numeral", 4, []string{"a", "=", "1e+", "b"}},
	{"a = 0x1P- b", "invalid numeral", 4, []string{"a", "=", "0x1P-", "b"}},
	{"a = 1.2.3+4", "invalid numeral", 4, []string{"a", "=", "1.2.3", "+", "4"}},
	{"a = (0x1p)", "invalid numeral", 5, []string{"a", "=", "(", "0x1p", ")"}},
}

// afterAnOpenString has a string that its line ends, and a require on the next line.
const afterAnOpenString = "x = \"open\nrequire \"m\""

func TestTokenizeRecoversFromMalformedTokensAndReportsTheFirst(t *testing.T) {
	for _, c := range malformedSources {
		tokens, fault := Tokenize(c.source)
		if fault == nil || fault.Msg != c.fault || fault.Offset != c.offset {
			t.Errorf("Tokenize(%q) fault = %+v, want %q at %d", c.source, fault, c.fault, c.offset)
		}
		if got := raws(tokens); !slices.Equal(got, c.raws) {
			t.Errorf("Tokenize(%q) = %q, want %q", c.source, got, c.raws)
		}
	}
	// A line break that ends an unterminated string still counts as a line.
	if got := Requires(afterAnOpenString); !slices.Equal(got, []Require{{Line: 2, Name: "m", Literal: true}}) {
		t.Errorf("Requires after an unterminated string = %+v", got)
	}
}

// escapedStrings has a \z that skips the white space after it, line breaks included, and a backslash before CRLF
// that continues its string.
const escapedStrings = "a = \"one\\z\n   two\" b = \"x\\\r\ny\" require \"m\""

func TestQuotedStringsFollowLuasEscapes(t *testing.T) {
	tokens, fault := Tokenize(escapedStrings)
	if fault != nil {
		t.Fatalf("fault = %+v", fault)
	}
	if tokens[2].Text != "one\\z\n   two" || !tokens[2].Escaped || tokens[5].Text != "x\\\r\ny" {
		t.Errorf("strings = %q, %q", tokens[2].Text, tokens[5].Text)
	}
	if got := Requires(escapedStrings); !slices.Equal(got, []Require{{Line: 3, Name: "m", Literal: true}}) {
		t.Errorf("Requires = %+v", got)
	}
}

func TestATokenHasItsKindItsTextAndItsPlace(t *testing.T) {
	tokens, fault := Tokenize("name 0x1F 'it''s' [=[\nlong]=] ...")
	if fault != nil {
		t.Fatalf("fault = %+v", fault)
	}
	want := []Token{
		{Kind: NameToken, Raw: "name", Text: "name", Start: 0, End: 4, Line: 1},
		{Kind: NumberToken, Raw: "0x1F", Text: "0x1F", Start: 5, End: 9, Line: 1},
		{Kind: StringToken, Raw: "'it'", Text: "it", Start: 10, End: 14, Line: 1},
		{Kind: StringToken, Raw: "'s'", Text: "s", Start: 14, End: 17, Line: 1},
		{Kind: StringToken, Raw: "[=[\nlong]=]", Text: "long", Start: 18, End: 29, Line: 1},
		{Kind: SymbolToken, Raw: "...", Text: "...", Start: 30, End: 33, Line: 2},
	}
	if !slices.Equal(tokens, want) {
		t.Errorf("tokens = %+v\nwant     %+v", tokens, want)
	}
}

func TestTheEdgesOfNamesNumeralsAndDots(t *testing.T) {
	for _, c := range []struct {
		source string
		kind   Kind // of every token
		raws   []string
	}{
		{"A Z a z _ _9 Zz9_", NameToken, []string{"A", "Z", "a", "z", "_", "_9", "Zz9_"}},
		{"0xa 0xA 0xf 0xF 0x09afAF 0Xa.Fp1", NumberToken, []string{"0xa", "0xA", "0xf", "0xF", "0x09afAF", "0Xa.Fp1"}},
		{".5", NumberToken, []string{".5"}},
		{"1 .5", NumberToken, []string{"1", ".5"}},
		{".", SymbolToken, []string{"."}},
		{". .. ...", SymbolToken, []string{".", "..", "..."}},
		{"....", SymbolToken, []string{"...", "."}},
	} {
		tokens, fault := Tokenize(c.source)
		if got := raws(tokens); fault != nil || !slices.Equal(got, c.raws) {
			t.Errorf("Tokenize(%q) = %q, fault %+v, want %q", c.source, got, fault, c.raws)
		}
		for _, token := range tokens {
			if token.Kind != c.kind {
				t.Errorf("Tokenize(%q): %q is of kind %d, want %d", c.source, token.Raw, token.Kind, c.kind)
			}
		}
	}
	// A dot that ends the source after a name is a symbol, and a hexadecimal numeral takes no letter after f.
	if tokens, fault := Tokenize("a."); fault != nil || !slices.Equal(raws(tokens), []string{"a", "."}) {
		t.Errorf("Tokenize(a.) = %q, fault %+v", raws(tokens), fault)
	}
	for _, source := range []string{"0xg", "0xG", "0x1g"} {
		if _, fault := Tokenize(source); fault == nil || fault.Msg != "invalid numeral" {
			t.Errorf("Tokenize(%s): fault %+v, want an invalid numeral", source, fault)
		}
	}
}

func TestAShortCommentEndsBeforeItsLineBreakOrWithTheSource(t *testing.T) {
	for _, c := range []struct {
		source string
		raws   []string
		line   int // the line of the last token
	}{
		{"a -- the source ends in the comment", []string{"a"}, 1},
		{"--", nil, 0},
		{"a -- a line feed\nb", []string{"a", "b"}, 2},
		{"a -- a return and a line feed\r\nb", []string{"a", "b"}, 2},
		{"a -- [[ no long comment\nb ]]", []string{"a", "b", "]", "]"}, 2},
	} {
		tokens, fault := Tokenize(c.source)
		if got := raws(tokens); fault != nil || !slices.Equal(got, c.raws) {
			t.Errorf("Tokenize(%q) = %q, fault %+v, want %q", c.source, got, fault, c.raws)
		}
		if len(tokens) > 0 && tokens[len(tokens)-1].Line != c.line {
			t.Errorf("Tokenize(%q): the last token is on line %d, want %d", c.source, tokens[len(tokens)-1].Line, c.line)
		}
	}
}

func TestALongStringLeavesOutTheLineBreakThatFollowsItsOpeningBracket(t *testing.T) {
	for _, c := range []struct{ source, text string }{
		{"[[\ntext]]", "text"},
		{"[[\r\ntext]]", "text"},
		{"[==[\r\n\r\ntext]==]", "\r\ntext"},
		{"[[\n\ntext\n]]", "\ntext\n"},
		{"[[text\r\n]]", "text\r\n"},
		{"[[\r\n]]", ""},
	} {
		tokens, fault := Tokenize(c.source)
		if fault != nil || len(tokens) != 1 || tokens[0].Kind != StringToken || tokens[0].Text != c.text {
			t.Errorf("Tokenize(%q) = %+v, fault %+v, want one string with the text %q", c.source, tokens, fault, c.text)
		}
	}
}

func TestAfterZOnlyLuasOwnWhiteSpaceIsSkipped(t *testing.T) {
	// A space and a line break after \z are skipped, so the string goes on to its closing quote.
	tokens, fault := Tokenize("a = \"x\\z \n y\" b")
	if want := []string{"a", "=", "\"x\\z \n y\"", "b"}; fault != nil || !slices.Equal(raws(tokens), want) {
		t.Errorf("tokens = %q, fault = %+v", raws(tokens), fault)
	}
	// Lua's \z stops at a no-break space (the first source) and at a byte order mark (the second): neither is white
	// space to Lua. The line break after it is then not skipped, and ends the string unclosed.
	for _, c := range []struct {
		source string
		raws   []string
	}{
		{"a = \"x\\z\xC2\xA0\n y\" b", []string{"a", "=", "\"x\\z\xC2\xA0", "y", "\" b"}},
		{"a = \"x\\z\xEF\xBB\xBF\n y\" b", []string{"a", "=", "\"x\\z\xEF\xBB\xBF", "y", "\" b"}},
	} {
		tokens, fault := Tokenize(c.source)
		if fault == nil || fault.Msg != "unescaped newline in quoted string" || fault.Offset != 4 {
			t.Errorf("Tokenize(%q) fault = %+v, want an unescaped newline at 4", c.source, fault)
		}
		if got := raws(tokens); !slices.Equal(got, c.raws) {
			t.Errorf("Tokenize(%q) = %q, want %q", c.source, got, c.raws)
		}
	}
}

func TestOnlyLuasOwnWhiteSpaceSeparatesTokens(t *testing.T) {
	if tokens, fault := Tokenize("a\t\v\f\r\n b"); fault != nil || !slices.Equal(raws(tokens), []string{"a", "b"}) {
		t.Errorf("tokens = %q, fault = %+v", raws(tokens), fault)
	}
	// A no-break space and a byte order mark are not white space in Lua: each is a symbol Lua does not have.
	for _, c := range []struct {
		source string
		raws   []string
	}{
		{"a\xC2\xA0b", []string{"a", "\xC2\xA0", "b"}},
		{"\xEF\xBB\xBFa", []string{"\xEF\xBB\xBF", "a"}},
	} {
		tokens, fault := Tokenize(c.source)
		if fault == nil || fault.Msg != "unsupported symbol" {
			t.Errorf("Tokenize(%q) fault = %+v, want an unsupported symbol", c.source, fault)
		}
		if got := raws(tokens); !slices.Equal(got, c.raws) {
			t.Errorf("Tokenize(%q) = %q, want %q", c.source, got, c.raws)
		}
	}
}
