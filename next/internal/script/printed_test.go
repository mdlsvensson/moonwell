package script

import (
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/tooltest"
)

// These tests read recorded text and run no compiler, but for the one that runs the compiler on the sources
// behind the recordings and compares what it prints with them.

// What the pinned compiler, 0.34.3, printed on Windows for the sources named beside each, recorded from runs of
// it with the arguments a compile gives it. The project folder is written as C:\project and the compiler's as
// C:\yue; nothing else is changed. On Windows the compiler ends its lines with "\r\n".
const (
	// src/bad.yue: "x = 1\ny = \n  if then\n", with -r and with -m.
	printedSyntax = "Failed to compile: C:\\project\\src\\bad.yue\r\n2: expected valid expression\r\n1 | x = 1\r\n2 | y = \r\n       ^\r\n3 |   if then\r\n\r\n"
	// src/mac.yue: the macro import, "x = 1\ny = $FourCC \"hfo\"\n".
	printedMacro = "Failed to compile: C:\\project\\src\\mac.yue\r\n" +
		"3: failed to expand macro: (macro FourCC):21: $FourCC needs a string literal of exactly 4 characters, such as \"hfoo\".\r\n" +
		"1 | import \"moonwell.macros\" as {:$FourCC}\r\n2 | x = 1\r\n3 | y = $FourCC \"hfo\"\r\n        ^\r\n\r\n"
	// src/nomod.yue: "import \"nothing.here\" as {:$X}\nx = 1\n".
	printedNoModule = "Failed to compile: C:\\project\\src\\nomod.yue\r\n1: module 'nothing.here' not found:\r\n" +
		"\tno file \"C:\\project\\.moonwell\\yue\\nothing\\here.yue\"\r\n" +
		"\tno file \"C:\\project\\src\\nothing\\here.yue\"\r\n" +
		"\tno file \"C:\\yue\\lua\\nothing\\here.yue\"\r\n" +
		"\tno file \"C:\\yue\\lua\\nothing\\here\\init.yue\"\r\n" +
		"\tno file \"C:\\yue\\nothing\\here.yue\"\r\n" +
		"\tno file \"C:\\yue\\nothing\\here\\init.yue\"\r\n" +
		"\tno file \"C:\\yue\\..\\share\\lua\\5.4\\nothing\\here.yue\"\r\n" +
		"\tno file \"C:\\yue\\..\\share\\lua\\5.4\\nothing\\here\\init.yue\"\r\n" +
		"\tno file \".\\nothing\\here.yue\"\r\n" +
		"\tno file \".\\nothing\\here\\init.yue\"\r\n" +
		"1 | import \"nothing.here\" as {:$X}\r\n                             ^\r\n2 | x = 1\r\n\r\n"
	// src/a.yue: "export x = '\xff'\n", a byte that is not UTF-8. The compiler names no line.
	printedNoLine = "Failed to compile: C:\\project\\src\\a.yue\r\n\r\n"
	// src/a.yue: "export x = 1\rexport y = 2\n", a carriage return alone, which the excerpt keeps.
	printedLoneReturn = "Failed to compile: C:\\project\\src\\a.yue\r\n1: syntax error\r\n1 | export x = 1\rexport y = 2\r\n                 ^\r\n\r\n"

	// src/bit.yue: "x = 1\n\n\nflags = x & 3\nprint flags\n", with -r: what was printed, and the Lua left at the
	// output.
	printedRewrite = "Failed to rewrite: C:\\project\\dist\\stage\\lua\\bit.lua\r\n>> :3:17: Unexpected Symbol `&` in source.\r\n"
	leftByRewrite  = "-- [yue]: C:\\project\\src\\bit.yue\r\nlocal x = 1 -- 1\r\nlocal flags = x & 3 -- 4\r\nreturn print(flags) -- 5\r\n"
	// The same source with -m.
	printedMinify = "Failed to minify: C:\\project\\dist\\stage\\lua\\bit.lua\r\n>> :2:17: Unexpected Symbol `&` in source.\r\n"
	leftByMinify  = "local x = 1\r\nlocal flags = x & 3\r\nreturn print(flags)\r\n"
	// src/shl.yue: "a = 1\nb = a << 2\nc = ~a\n", with -r.
	printedRewriteTilde = "Failed to rewrite: C:\\project\\dist\\stage\\lua\\shl.lua\r\n>> :4:12: Unexpected symbol `~` in source.\r\n"
	leftByRewriteTilde  = "-- [yue]: C:\\project\\src\\shl.yue\r\nlocal a = 1 -- 1\r\nlocal b = a << 2 -- 2\r\nlocal c = ~a -- 3\r\n"
)

// asPrinted is what a compile reads of a run: what the compiler printed on its two streams, with a line break
// between them.
func asPrinted(stdout string) string { return stdout + "\n" }

// withLineFeeds is a recorded text with the line ends of a system that ends its lines with "\n". It is made of
// the recording; TestTheCompilerPrintsWhatWasRecorded holds the compiler of such a system to it.
func withLineFeeds(recorded string) string { return strings.ReplaceAll(recorded, "\r\n", "\n") }

// withoutSearchedFiles is a text without the lines that name a file the compiler looked for a module in: which
// files those are is the system's and the compiler's folder's own.
func withoutSearchedFiles(printed string) string {
	var kept []string
	for line := range strings.SplitSeq(printed, "\n") {
		if !strings.HasPrefix(line, "\tno file ") {
			kept = append(kept, line)
		}
	}
	return strings.Join(kept, "\n")
}

// recordedVersion is the version of the compiler that the recordings are of.
const recordedVersion = "0.34.3"

// versionLine finds the version in what a compiler prints when it is asked for it: the version ends at the first
// white space.
var versionLine = regexp.MustCompile(`Yuescript version: ([^ \t\n\v\f\r]+)`)

// reportedVersion is the version a compiler reports when it is asked with -v, on either stream; "" when what it
// prints names none.
func reportedVersion(t *testing.T, yue string) string {
	t.Helper()
	result, err := env.Run(background, yue, []string{"-v"}, env.RunOptions{})
	if err != nil {
		t.Fatalf("asking %s for its version: %v", yue, err)
	}
	if reported := versionLine.FindStringSubmatch(result.Stdout + result.Stderr); reported != nil {
		return reported[1]
	}
	return ""
}

// TestTheCompilerPrintsWhatWasRecorded runs the real compiler on the source behind each recording, with the
// arguments a compile gives it, and compares what it prints and leaves with the recording. On Windows the two
// are the same once the folders are written as the recordings write them. On another system the recording is
// taken with "\n" for each line end, and without the lines that name the files a module was looked for in.
//
// The recordings are of one version of the compiler, which is the one Moonwell downloads. A compiler of the
// user's own that reports another version, named by MOONWELL_TEST_YUE, words its failures its own way: the test
// is skipped for it, and says so. Without that variable the compiler is the one Moonwell downloads, and another
// version fails the test: the recordings are then to be made again.
func TestTheCompilerPrintsWhatWasRecorded(t *testing.T) {
	yue := tooltest.Yue(t)
	if reported := reportedVersion(t, yue); reported != recordedVersion {
		if os.Getenv("MOONWELL_TEST_YUE") == "" {
			t.Fatalf("the recordings are of YueScript %s, and the compiler Moonwell downloads reports %q: record them again", recordedVersion, reported)
		}
		t.Skipf("the recordings are of YueScript %s, and the compiler MOONWELL_TEST_YUE names reports %q", recordedVersion, reported)
	}
	const bit = "x = 1\n\n\nflags = x & 3\nprint flags\n"
	for _, c := range []struct {
		name, text, mode string // the source is src/<name>.yue
		code             int
		printed, left    string // the recording, and the Lua left at the output, "" for none
	}{
		{"bad", "x = 1\ny = \n  if then\n", "-r", 1, printedSyntax, ""},
		{"bad", "x = 1\ny = \n  if then\n", "-m", 1, printedSyntax, ""},
		{"mac", macroImport + "x = 1\ny = $FourCC \"hfo\"\n", "-r", 1, printedMacro, ""},
		{"nomod", "import \"nothing.here\" as {:$X}\nx = 1\n", "-r", 1, printedNoModule, ""},
		{"a", "export x = '\xff'\n", "-r", 1, printedNoLine, ""},
		{"a", "export x = 1\rexport y = 2\n", "-r", 1, printedLoneReturn, ""},
		{"bit", bit, "-r", 2, printedRewrite, leftByRewrite},
		{"bit", bit, "-m", 2, printedMinify, leftByMinify},
		{"shl", "a = 1\nb = a << 2\nc = ~a\n", "-r", 2, printedRewriteTilde, leftByRewriteTilde},
	} {
		b := benchOf(t, files("src/"+c.name+".yue", c.text))
		source, output := filepath.Join(b.root, "src", c.name+".yue"), b.staged(c.name+".lua")
		if err := os.MkdirAll(filepath.Dir(output), 0o777); err != nil {
			t.Fatal(err)
		}
		args := []string{"--target=5.3", c.mode, "-o", output, "--path", b.search.path, source}
		result, err := env.Run(background, yue, args, env.RunOptions{})
		if err != nil {
			t.Fatal(err)
		}
		// The folders as the recordings write them; the longest first, where one begins with another.
		asRecorded := strings.NewReplacer(
			source, `C:\project\src\`+c.name+`.yue`, output, `C:\project\dist\stage\lua\`+c.name+`.lua`,
			b.root, `C:\project`, filepath.Dir(yue), `C:\yue`,
		)
		printed, left := asRecorded.Replace(result.Stdout), asRecorded.Replace(leftAt(output))
		wantPrinted, wantLeft := c.printed, c.left
		if runtime.GOOS != "windows" {
			wantPrinted, wantLeft = withoutSearchedFiles(withLineFeeds(wantPrinted)), withLineFeeds(wantLeft)
			printed = withoutSearchedFiles(printed)
		}
		if result.Code != c.code || printed != wantPrinted || result.Stderr != "" || left != wantLeft {
			t.Errorf("src/%s.yue with %s: exit code %d, want %d\nprinted %q\nwant    %q\non the error stream %q\nleft %q\nwant %q",
				c.name, c.mode, result.Code, c.code, printed, wantPrinted, result.Stderr, left, wantLeft)
		}
	}
}

func TestCompileErrorReadsTheLineAndTheMessageTheCompilerPrinted(t *testing.T) {
	syntaxDetail := "2: expected valid expression\n1 | x = 1\n2 | y = \n       ^\n3 |   if then"
	macroDetail := "3: failed to expand macro: (macro FourCC):21: " + fourCCMessage + "\n" +
		"1 | import \"moonwell.macros\" as {:$FourCC}\n2 | x = 1\n3 | y = $FourCC \"hfo\"\n        ^"
	for _, c := range []struct {
		what, printed, wantMsg string
		wantLine               int
	}{
		{"a syntax error", asPrinted(printedSyntax), "expected valid expression\n" + syntaxDetail, 2},
		{"a syntax error, with line feeds", asPrinted(withLineFeeds(printedSyntax)), "expected valid expression\n" + syntaxDetail, 2},
		// The macro's position, a line of the macro module, is dropped from the first line; the excerpt keeps it.
		{"a failed macro", asPrinted(printedMacro), fourCCMessage + "\n" + macroDetail, 3},
		{"a failed macro, with line feeds", asPrinted(withLineFeeds(printedMacro)), fourCCMessage + "\n" + macroDetail, 3},
		{"a module that is not found", asPrinted(printedNoModule), "module 'nothing.here' not found:\n" +
			strings.TrimSpace(withLineFeeds(strings.TrimPrefix(printedNoModule, "Failed to compile: C:\\project\\src\\nomod.yue\r\n"))), 1},
		{"no numbered line", asPrinted(printedNoLine), "YueScript compilation failed.", 0},
		{"a carriage return alone in the excerpt", asPrinted(printedLoneReturn),
			"syntax error\n1: syntax error\n1 | export x = 1\rexport y = 2\n                 ^", 1},
		// Made up, to show what the reading does with what the compiler does not print today.
		{"nothing printed", "\n", "YueScript compilation failed.", 0},
		{"white space only", " \t\r\n\v\f \n", "YueScript compilation failed.", 0},
		{"no numbered line and some text", "Failed to compile: x\n  something else  \n", "something else", 0},
		{"the message on the error stream", "\nFailed to compile: x\n7: late\n", "late\n7: late", 7},
		{"a numbered line first", "12: first\n3: second", "first\n12: first\n3: second", 12},
		{"a numbered line after a carriage return alone", "x\r4: after", "after\nx\r4: after", 4},
		{"a number that is no line", "a 5: no\n", "a 5: no", 0},
		{"a number without a message", "5: \n6:\n", "5: \n6:", 0},
		{"a macro's position without a macro", "2: failed to expand macro: nothing", "failed to expand macro: nothing\n2: failed to expand macro: nothing", 2},
		// A line beyond what a number holds is the greatest number.
		{"a line of many digits", "99999999999999999999999: far", "far\n99999999999999999999999: far", math.MaxInt},
		{"a line that only starts with the words", "Failed to compile\nFailed to compiler: x\n3: y", "y\n3: y", 3},
	} {
		failure := compileError("src/x.yue", c.printed)
		if failure.Msg != c.wantMsg || failure.Line != c.wantLine || failure.File != "src/x.yue" || failure.Hint != "" || failure.Column != 0 {
			t.Errorf("%s: compileError = %+v, want line %d and the message %q", c.what, failure, c.wantLine, c.wantMsg)
		}
	}
}

// leaving is the Lua a compiler left at an output, as rewriteError asks for it.
func leaving(lua string) func() string { return func() string { return lua } }

func TestRewriteErrorReadsTheStepTheReasonAndTheLineMarkOfTheLuaLeft(t *testing.T) {
	const and, tilde = "Unexpected Symbol `&` in source.", "Unexpected symbol `~` in source."
	for _, c := range []struct {
		what, printed, left, wantMsg string
		wantLine                     int
	}{
		{"a rewrite", asPrinted(printedRewrite), leftByRewrite, "YueScript compiled this file but could not rewrite its Lua: " + and, 4},
		{"a rewrite, with line feeds", asPrinted(withLineFeeds(printedRewrite)), withLineFeeds(leftByRewrite),
			"YueScript compiled this file but could not rewrite its Lua: " + and, 4},
		// Minified Lua carries no line marks.
		{"a minify", asPrinted(printedMinify), leftByMinify, "YueScript compiled this file but could not minify its Lua: " + and, 0},
		{"a rewrite of another operator", asPrinted(printedRewriteTilde), leftByRewriteTilde,
			"YueScript compiled this file but could not rewrite its Lua: " + tilde, 3},
		// Made up.
		{"no Lua left", asPrinted(printedRewrite), "", "YueScript compiled this file but could not rewrite its Lua: " + and, 0},
		{"a line beyond the Lua left", "Failed to rewrite: x\n>> :9:1: far\n", "local x -- 1\n", "YueScript compiled this file but could not rewrite its Lua: far", 0},
		{"line 0", "Failed to rewrite: x\n>> :0:1: zero\n", "local x -- 1\n", "YueScript compiled this file but could not rewrite its Lua: zero", 0},
		{"a line without a mark", "Failed to rewrite: x\n>> :1:1: bare\n", "local x\nlocal y -- 2\n", "YueScript compiled this file but could not rewrite its Lua: bare", 0},
		{"a mark that does not end the line", "Failed to rewrite: x\n>> :1:1: inside\n", "local x -- 1 \n", "YueScript compiled this file but could not rewrite its Lua: inside", 0},
		{"a reason with white space around it", "Failed to rewrite: x\n>> :1:1:  \t spaced \t\n", "local x -- 7\n",
			"YueScript compiled this file but could not rewrite its Lua: spaced", 7},
		{"the step on the error stream", "\nFailed to rewrite: x\n>> :2:1: late\n", "a -- 1\nb -- 12\n", "YueScript compiled this file but could not rewrite its Lua: late", 12},
	} {
		failure := rewriteError("src/x.yue", c.printed, leaving(c.left))
		if failure == nil || failure.Msg != c.wantMsg || failure.Line != c.wantLine || failure.File != "src/x.yue" ||
			!strings.Contains(failure.Hint, "bitwise operators (&, |, ~, <<, >>)") || !strings.Contains(failure.Hint, "lua/") {
			t.Errorf("%s: rewriteError = %+v, want line %d and the message %q", c.what, failure, c.wantLine, c.wantMsg)
		}
	}
}

func TestTheLuaLeftIsReadOnlyForAStepThatFailedAtAPosition(t *testing.T) {
	asked := 0
	left := func() string {
		asked++
		return leftByRewrite
	}
	// Any other failure is no failure of that step, and the Lua at the output is not read for it.
	for _, printed := range []string{asPrinted(printedSyntax), asPrinted(printedMacro), "\n", "x Failed to rewrite: y\n", "Failed to rewrite\n", "Failed to compile: Failed to rewrite: x\n"} {
		if failure := rewriteError("src/x.yue", printed, left); failure != nil || asked != 0 {
			t.Errorf("rewriteError(%q) = %+v, and the Lua left was read %d times, want no failure and no read", printed, failure, asked)
		}
	}
	// A step that failed without a position has no line to find in it.
	failure := rewriteError("src/x.yue", "Failed to minify: x\n", left)
	if failure == nil || failure.Msg != "YueScript compiled this file but could not minify its Lua." || failure.Line != 0 || asked != 0 {
		t.Errorf("without a reason: rewriteError = %+v, and the Lua left was read %d times", failure, asked)
	}
	if failure := rewriteError("src/x.yue", asPrinted(printedRewrite), left); failure == nil || failure.Line != 4 || asked != 1 {
		t.Errorf("with a position: rewriteError = %+v, and the Lua left was read %d times, want once", failure, asked)
	}
}

func TestASourceHasCodeUnlessEveryLineIsBlankOrAComment(t *testing.T) {
	for source, want := range map[string]bool{
		"":                              false,
		"\n\n":                          false,
		"-- only comments\n\n":          false,
		"  \t-- indented\r\n\r\n--\r\n": false,
		" \t\v\f\r\n":                   false,
		"-- a comment\nexport x = 1\n":  true,
		"x = 1 -- a comment":            true,
		"- not a comment\n":             true,
		// A line ends at "\n": after a carriage return alone, the rest of the line is no comment.
		"-- a\rx = 1\n": true,
		// A block comment is told from code by its first line only.
		"--[[ a\nb\n]]\n": true,
		"--[[ a ]]\n":     false,
	} {
		if got := hasCode(source); got != want {
			t.Errorf("hasCode(%q) = %v, want %v", source, got, want)
		}
	}
}

// Characters outside ASCII that are white space or line ends elsewhere, as the bytes they are.
const (
	noBreakSpace  = "\xc2\xa0"     // U+00A0
	lineSeparator = "\xe2\x80\xa8" // U+2028
	paragraphEnd  = "\xe2\x80\xa9" // U+2029
	wideSpace     = "\xe3\x80\x80" // U+3000
)

func TestWhatTheCompilerPrintsIsReadWithWhiteSpaceAndLineEndsOfASCIIOnly(t *testing.T) {
	// White space is " \t\n\v\f\r" and a line ends at "\n" or "\r": a character outside ASCII is part of the
	// text it stands in.
	for _, c := range []struct {
		printed, wantMsg string
		wantLine         int
	}{
		{"3: boom" + lineSeparator + "rest\n", "boom" + lineSeparator + "rest\n3: boom" + lineSeparator + "rest", 3},
		{"3: boom" + paragraphEnd + "rest\n", "boom" + paragraphEnd + "rest\n3: boom" + paragraphEnd + "rest", 3},
		{"x" + lineSeparator + "5: late\n", "x" + lineSeparator + "5: late", 0},
		{"x" + paragraphEnd + "5: late\n", "x" + paragraphEnd + "5: late", 0},
		{noBreakSpace + "oops" + wideSpace + "\n", noBreakSpace + "oops" + wideSpace, 0},
		{mark + "\n", mark, 0},
	} {
		if failure := compileError("src/x.yue", c.printed); failure.Msg != c.wantMsg || failure.Line != c.wantLine {
			t.Errorf("compileError(%q) = %+v, want line %d and the message %q", c.printed, failure, c.wantLine, c.wantMsg)
		}
	}
	for reason, want := range map[string]string{
		"a" + lineSeparator + "b": "a" + lineSeparator + "b",
		"a" + paragraphEnd + "b":  "a" + paragraphEnd + "b",
		"a" + noBreakSpace + " ":  "a" + noBreakSpace,
		wideSpace + "a":           wideSpace + "a",
	} {
		failure := rewriteError("src/x.yue", "Failed to rewrite: x\n>> :1:1: "+reason+"\n", leaving(""))
		if wantMsg := "YueScript compiled this file but could not rewrite its Lua: " + want; failure == nil || failure.Msg != wantMsg {
			t.Errorf("rewriteError with the reason %q = %+v, want the message %q", reason, failure, wantMsg)
		}
	}
	for source, want := range map[string]bool{
		noBreakSpace + "-- a comment\n":    true,
		wideSpace + "\n":                   true,
		mark + "\n":                        true,
		"-- a" + lineSeparator + "x = 1\n": false,
		"-- a" + paragraphEnd + "x = 1\n":  false,
	} {
		if got := hasCode(source); got != want {
			t.Errorf("hasCode(%q) = %v, want %v", source, got, want)
		}
	}
}

// ---- what yue -g prints ----

func TestUsesPrintedReadsOutputWithCarriageReturnsAndSkipsBlankLines(t *testing.T) {
	uses, err := usesPrinted("Score 1 8\r\nCreatUnit 2 7\r\n\r\n", "src/main.yue")
	want := []globalUse{{Name: "Score", Line: 1, Column: 8}, {Name: "CreatUnit", Line: 2, Column: 7}}
	if err != nil || !slices.Equal(uses, want) {
		t.Errorf("usesPrinted = %+v, %v", uses, err)
	}
	// Nothing printed is a list without uses, and not no list: the file was listed.
	for _, printed := range []string{"", "\n", " \t\r\n\v\f\n"} {
		if uses, err := usesPrinted(printed, "src/main.yue"); err != nil || uses == nil || len(uses) != 0 {
			t.Errorf("usesPrinted(%q) = %#v, %v, want a list without uses", printed, uses, err)
		}
	}
	// White space around a line is dropped, and a name is whatever has none.
	uses, err = usesPrinted("\t a.b:c 10 200 \n", "src/main.yue")
	if want := []globalUse{{Name: "a.b:c", Line: 10, Column: 200}}; err != nil || !slices.Equal(uses, want) {
		t.Errorf("usesPrinted = %+v, %v", uses, err)
	}
}

func TestUsesPrintedRefusesOutputItCannotRead(t *testing.T) {
	// The first line that cannot be read is the one named, and no use is returned.
	uses, failure := usesPrinted("print 1 1\nScore one 8\nx\n", "src/main.yue")
	if uses != nil || failure == nil || failure.Msg != "yue -g printed a line Moonwell cannot read: Score one 8" ||
		failure.File != "src/main.yue" || failure.Line != 0 ||
		failure.Hint != "Use a YueScript version Moonwell supports: remove yue.version and yue.path from the manifests." {
		t.Fatalf("usesPrinted = %+v, %+v", uses, failure)
	}
	for _, printed := range []string{"Score", "Score 1", "Score 1 8 9", "Score  1 8", "Score 1\t8", "Score -1 8", "1 8", "Score 1.0 8"} {
		if uses, err := usesPrinted(printed+"\n", "src/main.yue"); err == nil {
			t.Errorf("usesPrinted(%q) = %+v, want a refusal", printed, uses)
		}
	}
}

func TestWhatYueGPrintsIsReadWithWhiteSpaceOfASCIIOnly(t *testing.T) {
	// A character outside ASCII is part of the name it stands in, and of the line it stands at the end of.
	uses, err := usesPrinted("a"+noBreakSpace+"b 1 2\n"+wideSpace+" 3 4\n"+lineSeparator+"Score 5 6\n", "src/main.yue")
	want := []globalUse{
		{Name: "a" + noBreakSpace + "b", Line: 1, Column: 2}, {Name: wideSpace, Line: 3, Column: 4}, {Name: lineSeparator + "Score", Line: 5, Column: 6},
	}
	if err != nil || !slices.Equal(uses, want) {
		t.Errorf("usesPrinted = %+v, %v", uses, err)
	}
	for _, printed := range []string{"Score 1 8" + noBreakSpace + "\n", "a 1 2" + paragraphEnd + "b 3 4\n", "Score" + wideSpace + "1 8\n"} {
		uses, failure := usesPrinted(printed, "src/main.yue")
		if failure == nil || !strings.HasPrefix(failure.Msg, "yue -g printed a line Moonwell cannot read: ") {
			t.Errorf("usesPrinted(%q) = %+v, %+v, want a refusal", printed, uses, failure)
		}
	}
}
