package jass_test

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/tools/gen/jass"
)

// scriptsOfTheTests are the scripts that the tests of this package keep in their constants and lists, the ones
// that are read and the ones that are refused, each with a name for a failure. A script that a test writes in
// its own body is not among them.
func scriptsOfTheTests() map[string]string {
	named := map[string]string{
		"common": common, "blizzard": blizzard, "corners": corners, "indented": indented,
		"no declaration": noDeclaration, "comments": comments,
	}
	for i, c := range refused {
		named[fmt.Sprintf("refused script %02d, of %s", i, c.source)] = c.text
	}
	for _, c := range widerSpace {
		named[c.name] = c.text
	}
	return named
}

// outcomes counts the damaged scripts that were read and the ones that were refused.
type outcomes struct{ read, refused int }

// longerScripts are scripts of this file's own, longer than the ones of the tests, with what a script of the
// game has side by side: carriage returns, comments, a globals block, natives and functions with bodies.
var longerScripts = map[string]string{
	"types, globals and natives": "// first\r\ntype agent extends handle\r\n" +
		"type unit   extends agent // a unit\r\n\r\nglobals\r\n    constant integer MAX = 24\r\n" +
		"    string URL = \"http://example\" // no comment in the string\r\n    real array sizes\r\nendglobals\r\n" +
		"\r\nnative Make takes player p, real x returns unit\r\nconstant native Get takes nothing returns unit\r\n",
	"functions with bodies": "globals\n\treal angle = 0.0\nendglobals\n" +
		"function Helper takes unit u, boolean b returns nothing\n    local integer i = 0\n    call Make(null)\n" +
		"    endfunctions\nendfunction // done\nconstant function One takes nothing returns integer\n" +
		"    return 1\nendfunction\n",
	"a line that is no declaration": "type unit extends widget\nlibrary Foo\n" +
		"native F takes nothing returns nothing\n",
	"a body that never ends": "native A takes nothing returns nothing\n\n" +
		"function F takes integer a returns nothing\n",
}

// damageSeed is the seed of the changes that TestADamagedScriptIsReadOrRefusedByFileAndLineAndNeverPanics makes.
// A failure names the script and the index of the change: testkit.Changed makes the same text of the three
// again.
const damageSeed = 2003

// damagedFile is the name the test gives Parse for its errors.
const damagedFile = "scripts/damaged.j"

// byteOrderMark is U+FEFF in UTF-8, which is text to Parse.
const byteOrderMark = "\xEF\xBB\xBF"

// readOrRefused gives Parse the text. It stops the test when Parse panics, when it returns declarations with an
// error, and when the error does not start with the file and a line of the text; the error for a globals block
// that the script never ends names the file alone.
func (c *outcomes) readOrRefused(t *testing.T, what, text string) {
	t.Helper()
	var file jass.File
	var err error
	if value := testkit.Panic(func() { file, err = jass.Parse(text, damagedFile) }); value != nil {
		t.Fatalf("%s: Parse panics: %v", what, value)
	}
	if err == nil {
		c.read++
		return
	}
	c.refused++
	if file.Types != nil || file.Functions != nil || file.Globals != nil {
		t.Fatalf("%s: a refused script is %+v, want none", what, file)
	}
	rest, named := strings.CutPrefix(err.Error(), damagedFile+":")
	number, _, _ := strings.Cut(rest, ": ")
	line, notANumber := strconv.Atoi(number)
	onALine := notANumber == nil && line >= 1 && line <= strings.Count(text, "\n")+1
	if !named || !onALine && rest != " the globals block never reaches endglobals" {
		t.Fatalf("%s: got %v, want an error that starts with %s and a line of the text", what, err, damagedFile)
	}
}

// TestADamagedScriptIsReadOrRefusedByFileAndLineAndNeverPanics gives Parse the scripts of the tests and the
// longer scripts of this file, each cut at every length, after each of 60 seeded changes of its lines, quotes
// and white space, a byte order mark before every third, and with one character of ASCII white space put in at
// every place.
func TestADamagedScriptIsReadOrRefusedByFileAndLineAndNeverPanics(t *testing.T) {
	named := scriptsOfTheTests()
	maps.Copy(named, longerScripts)
	var damaged outcomes
	for _, name := range slices.Sorted(maps.Keys(named)) {
		text := named[name]
		for length := range len(text) {
			damaged.readOrRefused(t, fmt.Sprintf("%s cut at %d bytes", name, length), text[:length])
		}
		for index := range uint64(60) {
			made := testkit.Changed(text, damageSeed, index)
			if index%3 == 0 {
				made = byteOrderMark + made
			}
			damaged.readOrRefused(t, fmt.Sprintf("%s, change %d of seed %d: %q", name, index, damageSeed, made), made)
		}
		for i, made := range testkit.Swept(text) {
			damaged.readOrRefused(t, fmt.Sprintf("%s with white space put in, text %d: %q", name, i, made), made)
		}
	}
	// The floor is against a test that passes because it gave Parse nothing.
	if damaged.read == 0 || damaged.refused == 0 {
		t.Errorf("%d damaged scripts were read and %d refused; want some of each", damaged.read, damaged.refused)
	}
}
