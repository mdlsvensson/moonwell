package lua

import (
	"encoding/json"
	"errors"
	"io/fs"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/luasrc"
	"github.com/mdlsvensson/moonwell/internal/settings"
	oldtext "github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
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
		compareRefusals(t, what+": the refusal", source, wantErr, gotErr)
		return
	}
	oracle.Values(t, what+": functions", wantFunctions, gotFunctions)
	wantArguments, gotArguments := argumentsOfTheOtherTree(wantFunctions), argumentsOf(gotFunctions)
	if len(wantArguments) != len(gotArguments) {
		return // the functions differ, which is reported above, and their arguments do not pair
	}
	for i, argument := range wantArguments {
		compareLiterals(t, what+": argument "+strconv.Itoa(i), argument, gotArguments[i])
	}
}

// argumentsOf returns every argument of every call statement, in order.
func argumentsOf(functions []Function) [][]Token {
	var arguments [][]Token
	for _, function := range functions {
		for _, call := range function.Calls {
			arguments = append(arguments, call.Args...)
		}
	}
	return arguments
}

func argumentsOfTheOtherTree(functions []luasrc.Function) [][]luasrc.Token {
	var arguments [][]luasrc.Token
	for _, function := range functions {
		for _, call := range function.Calls {
			arguments = append(arguments, call.Args...)
		}
	}
	return arguments
}

// refusalOf is what an error of Functions says, in the shape both trees are compared in.
type refusalOf struct {
	Msg, Hint    string
	Line, Column int
}

var atCharacter = regexp.MustCompile(` at character (\d+)$`)

// compareRefusals compares the errors of both trees for a source that both refuse. This tree names the place of an
// error as a line and a column, counted in characters; that difference is meant, so the other tree's place is
// turned into a line and a column here before the two are compared. The other tree ends its message with
// ` at character N`, N being a UTF-16 offset into the source; an error that comes from a tokenizer fault has no such
// ending, and its place is the byte offset of the fault its tokenizer reports.
func compareRefusals(t *testing.T, what, source string, want, got error) {
	t.Helper()
	var wanted *olddiag.Error
	var given *diag.Error
	if !errors.As(want, &wanted) || !errors.As(got, &given) {
		t.Errorf("%s: an error is not a diag error: want %v, got %v", what, want, got)
		return
	}
	msg, offset := wanted.Msg, -1
	if place := atCharacter.FindStringSubmatch(wanted.Msg); place != nil {
		units, _ := strconv.Atoi(place[1])
		msg, offset = strings.TrimSuffix(wanted.Msg, place[0]), offsetOfUnit(source, units)
	} else if _, fault := luasrc.Tokenize(source); fault != nil {
		offset = fault.Offset
	} else {
		t.Errorf("%s: the other tree's error has no place and its tokenizer no fault: %v", what, want)
		return
	}
	line, column := lineAndColumn(source, offset)
	oracle.Values(t, what, refusalOf{msg, wanted.Hint, line, column},
		refusalOf{given.Msg, given.Hint, given.Line, given.Column})
}

// offsetOfUnit is the byte offset in text of the character that starts at the UTF-16 unit; the length of the text
// when it has no more units. A character above U+FFFF is two units, any other one, a byte that is not text one.
func offsetOfUnit(text string, unit int) int {
	units := 0
	for offset, character := range text {
		if units >= unit {
			return offset
		}
		units++
		if character > 0xFFFF {
			units++
		}
	}
	return len(text)
}

// lineAndColumn walks the text up to the byte offset and counts: a line feed starts a line, and every other
// character, or byte that is not text, is one column.
func lineAndColumn(text string, offset int) (line, column int) {
	line, column = 1, 1
	for _, character := range text[:offset] {
		if character == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return line, column
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

// otherScan is what the other tree's scanners make of a source, in the types of this tree: the two trees write
// the same JSON of what they read, which TestOracleOnTheSourcesOfTheTests holds, so each value is carried over
// through its JSON. A refusal of Functions is carried as this tree places it, by a line and a column, which is
// the one difference of the two that is meant (compareRefusals).
func otherScan(t *testing.T, source string) scanned {
	t.Helper()
	carry := func(from, to any) {
		data, err := json.Marshal(from)
		if err == nil {
			err = json.Unmarshal(data, to)
		}
		if err != nil {
			t.Fatalf("what the other tree read cannot be carried over: %v", err)
		}
	}
	// JSON has no way to write a byte that is not UTF-8, and a token may hold one: what holds the bytes of a
	// source is carried over field by field.
	carried := func(tokens []luasrc.Token) []Token {
		var out []Token
		for _, token := range tokens {
			out = append(out, Token{Kind(token.Kind), token.Raw, token.Text, token.Start, token.End, token.Line, token.Escaped})
		}
		return out
	}
	// What the other tree's two literal helpers make of the other tree's tokens.
	literalOf := func(tokens []luasrc.Token) literal {
		var made literal
		made.Number, made.IsNumber = luasrc.LiteralNumber(tokens)
		if made.Player, made.IsPlayer = luasrc.PlayerID(tokens); made.IsPlayer {
			made.Inside, _ = luasrc.LiteralNumber(tokens[2 : len(tokens)-1])
		}
		return made
	}
	var made scanned
	tokens, fault := luasrc.Tokenize(source)
	made.Tokens = carried(tokens)
	made.Whole = literalOf(tokens)
	carry(fault, &made.Fault)
	for _, require := range luasrc.Requires(source) {
		made.Requires = append(made.Requires, Require{require.Line, require.Name, require.Literal})
	}
	made.Globals = luasrc.TopLevelGlobals(source)
	carry(luasrc.ReadMapGlobals(source), &made.Map)
	functions, err := luasrc.Functions(source, "war3map.lua")
	for _, function := range functions {
		own := Function{Name: function.Name, Start: function.Start, End: function.End, EndStart: function.EndStart}
		for _, call := range function.Calls {
			var arguments [][]Token
			for _, argument := range call.Args {
				arguments = append(arguments, carried(argument))
				made.Arguments = append(made.Arguments, literalOf(argument))
			}
			own.Calls = append(own.Calls, Call{call.Name, arguments, call.Start, call.End})
		}
		made.Functions = append(made.Functions, own)
	}
	if err != nil {
		made.Refusal = otherRefusal(t, source, err)
	}
	return made
}

// otherRefusal is what an error of the other tree's Functions says, with its place as this tree gives one: the
// other tree ends its message with ` at character N`, or has the place of its tokenizer's fault.
func otherRefusal(t *testing.T, source string, err error) refusedAt {
	t.Helper()
	said := refusedAt{Message: err.Error()}
	var failure *olddiag.Error
	if errors.As(err, &failure) {
		said = refusedAt{File: failure.File, Message: failure.Msg, Hint: failure.Hint}
	}
	offset := -1
	if place := atCharacter.FindStringSubmatch(said.Message); place != nil {
		units, _ := strconv.Atoi(place[1])
		said.Message, offset = strings.TrimSuffix(said.Message, place[0]), offsetOfUnit(source, units)
	} else if _, fault := luasrc.Tokenize(source); fault != nil {
		offset = fault.Offset
	} else {
		t.Fatalf("the other tree's error has no place and its tokenizer no fault: %v", err)
	}
	said.Line, said.Column = lineAndColumn(source, offset)
	return said
}

// TestOracleOnTheRecordedScans holds testdata/recorded/corners.txt to what the other tree's scanners make of
// every source the recording names. It is the test that writes it: MOONWELL_RECORD=1 with -run of this test
// alone. The place of an error is this tree's, a line and a column (otherRefusal).
func TestOracleOnTheRecordedScans(t *testing.T) {
	other := func(source string) scanned { return otherScan(t, source) }
	testkit.Recorded(t, "corners.txt", scans(cornerSources, other))
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
	made, compared := 0, 0
	for _, c := range oracleSources() {
		if len(c.source) > 3000 {
			continue
		}
		for range 20 {
			mutated := mutate(random, c.source)
			made++
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
	t.Logf("%d mutated sources compared of %d made", compared, made)
	if compared < 2000 || compared*10 < made*9 {
		t.Errorf("only %d of %d mutated sources were compared; want 2000 or more, and nine in ten", compared, made)
	}
}

// numbers are values a script is given: zeros of both signs, whole values, values with many digits, colour
// channels as parts of 1, and values a map info holds, which are float32 values widened. Each is 0, or from
// 0.000001 up to 1e21 in size.
//
// Not among them, for the one difference that is meant: a value below 0.000001 in size that is not 0, or from
// 1e21. The other tree writes such a value with an exponent, Number in plain decimal
// (TestNumberWritesPlainDecimalWithTheFewestDigits).
func numbers() []float64 {
	values := []float64{
		0, math.Copysign(0, -1), 1, -1, 2, 255, 5000, 10000000, -10000000, 4294967296, 1e15, 1e20, -1e20,
		999999999999999900000, 0.000001, -0.000001, 0.0000011, 0.1, -0.5, 0.2, 0.30000000000000004, 1234567.891,
		math.Pi, -math.E, 123456789.12345679, 0.00392156862745098, 1.7976931348623157e20, 9007199254740993,
	}
	for channel := range 256 {
		values = append(values, float64(channel)/255)
	}
	for _, stored := range []float32{0.0000011, 0.1, 0.3, -0.3, 0.75, 44.7, -860.1, 3.208, 269.898, 128, -896, 500.5, 16777216, 1e7, 3.4e20} {
		values = append(values, float64(stored))
	}
	// A float32 of every size in the range, of each sign: 1.2345678 times each power of two from 2^-19 to 2^69.
	for power := -19; power < 70; power++ {
		stored := float32(math.Ldexp(1.2345678, power))
		values = append(values, float64(stored), -float64(stored))
	}
	return values
}

func TestOracleOnNumber(t *testing.T) {
	values := numbers()
	for _, value := range values {
		if size := math.Abs(value); size != 0 && (size < 0.000001 || size >= 1e21) {
			t.Fatalf("%v is a value the two trees write apart, and no value for this oracle", value)
		}
		what := "Number of " + strconv.FormatFloat(value, 'g', -1, 64)
		oracle.Bytes(t, what, []byte(oldtext.Number(value)), []byte(Number(value)))
	}
	if len(values) != 28+256+15+2*89 {
		t.Errorf("%d values, want %d", len(values), 28+256+15+2*89)
	}
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
