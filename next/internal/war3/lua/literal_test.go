package lua

import (
	"math"
	"testing"
)

// argument returns the tokens of the one argument of a call statement.
func argument(t *testing.T, source string) []Token {
	t.Helper()
	return mustFunctions(t, "function test() Capture("+source+") end")[0].Calls[0].Args[0]
}

// literalNumbers are arguments and the number each is; the ones without a number are not one literal number.
var literalNumbers = []struct {
	source string
	value  float64
	ok     bool
}{
	{"0", 0, true},
	{"0xF", 15, true},
	{"-0xF", -15, true},
	{"-.5", -.5, true},
	{"1.", 1, true},
	{"2e-3", .002, true},
	// A hexadecimal integer of any length is read, rounded as a float.
	{"0xFFFFFFFFFFFFFFFFFF", 4722366482869645213696, true},
	{"0x1.fp2", 0, false},
	{"1e999", 0, false},
	{"1+2", 0, false},
	{"(1)", 0, false},
	{`"1"`, 0, false},
}

// playerIDs are arguments and the player each names; the ones without are not `Player(n)` with a whole number.
var playerIDs = []struct {
	source string
	value  int
	ok     bool
}{
	{"Player(0)", 0, true},
	{"Player(-1)", -1, true},
	{"Player(0x17)", 23, true},
	{"Player(1.5)", 0, false},
	{"Player(1+2)", 0, false},
	{"Player(0,1)", 0, false},
	{"Player()", 0, false},
	{"object.Player(0)", 0, false},
	{"Player(0).id", 0, false},
	{"(Player(0))", 0, false},
}

func TestLiteralHelpersAcceptOnlyFiniteLiteralNumericShapes(t *testing.T) {
	for _, c := range literalNumbers {
		if value, ok := LiteralNumber(argument(t, c.source)); ok != c.ok || value != c.value {
			t.Errorf("LiteralNumber(%s) = %v, %v, want %v, %v", c.source, value, ok, c.value, c.ok)
		}
	}
	// A numeral that is no number gives a plain zero beside its false, also behind a minus sign.
	for _, source := range []string{"-1e999", "-0x1.fp2"} {
		if value, ok := LiteralNumber(argument(t, source)); ok || value != 0 || math.Signbit(value) {
			t.Errorf("LiteralNumber(%s) = %v, %v, want 0 and false", source, value, ok)
		}
	}
	// Unary plus is not Lua syntax, so the helper gets the tokens as they would be.
	plusOne := []Token{
		{Kind: SymbolToken, Raw: "+", Start: 0, End: 1},
		{Kind: NumberToken, Raw: "1", Start: 1, End: 2},
	}
	if _, ok := LiteralNumber(plusOne); ok {
		t.Error("LiteralNumber accepted +1")
	}
	for _, c := range playerIDs {
		if value, ok := PlayerID(argument(t, c.source)); ok != c.ok || value != c.value {
			t.Errorf("PlayerID(%s) = %v, %v, want %v, %v", c.source, value, ok, c.value, c.ok)
		}
	}
}

func TestPlayerIDTakesOnlyTheWholeCallOfPlayer(t *testing.T) {
	for _, source := range []string{"", "Player", "Player(", "Player()", "Player(0", "Player[0)", "Player(0]",
		"Player 0 )", "'Player'(0)", "Other(0)", "player(0)", "(0)"} {
		tokens, _ := Tokenize(source)
		if id, ok := PlayerID(tokens); ok || id != 0 {
			t.Errorf("PlayerID of the tokens of %s = %d, %v", source, id, ok)
		}
	}
	tokens, _ := Tokenize("Player ( - 7 )")
	if id, ok := PlayerID(tokens); !ok || id != -7 {
		t.Errorf("PlayerID of Player ( - 7 ) = %d, %v", id, ok)
	}
}

func TestQuoteEscapesQuotesBackslashesAndControlCharacters(t *testing.T) {
	for _, c := range []struct{ value, want string }{
		{`a"b\c`, `"a\"b\\c"`},
		// A space is the first character that is written as it is, and the tilde the last of ASCII.
		{" ", `" "`},
		{"\x1F !~\x7F", `"\031 !~\127"`},
		{"\n1\t2\x7F3", `"\0101\0092\1273"`},
		{"\n", `"\010"`},
		{"\t", `"\009"`},
		{"\x7F", `"\127"`},
		{"\x00\x1F", `"\000\031"`},
		{"é", `"é"`},
		{"it's", `"it's"`},
		{"", `""`},
	} {
		if got := Quote(c.value); got != c.want {
			t.Errorf("Quote(%q) = %s, want %s", c.value, got, c.want)
		}
	}
}

func TestQuoteWritesWhatTheTokenizerReadsAsOneString(t *testing.T) {
	for _, value := range []string{`a"b\c`, "line\r\nbreak 123", "tab\there", "é \U0001F319"} {
		tokens, fault := Tokenize(Quote(value))
		if fault != nil || len(tokens) != 1 || tokens[0].Kind != StringToken {
			t.Errorf("Quote(%q) reads as %+v, fault %+v", value, tokens, fault)
		}
	}
}

func TestNumberWritesPlainDecimalWithTheFewestDigits(t *testing.T) {
	for _, c := range []struct {
		value float64
		want  string
	}{
		{1, "1"},
		{-0.5, "-0.5"},
		{0.1, "0.1"},
		{1e21, "1000000000000000000000"},
		{1e-7, "0.0000001"},
		{float64(float32(0.1)), "0.10000000149011612"},
		{0, "0"},
		{255.0 / 255, "1"},
		// A zero has one spelling; a number below 0 keeps its sign, however small.
		{math.Copysign(0, -1), "0"},
		{-1e-7, "-0.0000001"},
		{-float64(math.SmallestNonzeroFloat32), "-0.000000000000000000000000000000000000000000001401298464324817"},
	} {
		got := Number(c.value)
		if got != c.want {
			t.Errorf("Number(%v) = %s, want %s", c.value, got, c.want)
		}
		tokens, fault := Tokenize(got)
		if value, ok := LiteralNumber(tokens); fault != nil || !ok || value != c.value {
			t.Errorf("Number(%v) = %s reads back as %v, %v", c.value, got, value, ok)
		}
	}
}
