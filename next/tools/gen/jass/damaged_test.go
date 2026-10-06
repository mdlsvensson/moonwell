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

// counted are scripts of this file's own, each with what Parse makes of it cut at every length, after the
// seeded changes and with white space put in: how many it reads and how many it refuses. They are written here
// and not taken from the lists of the tests, so that a script added to a list changes no number, and a number
// that differs names its script: it is a reading of that script that changed.
var counted = []struct {
	name, text          string
	cut, changed, swept outcomes
}{
	{name: "types, globals and natives",
		text: "// first\r\ntype agent extends handle\r\ntype unit   extends agent // a unit\r\n\r\nglobals\r\n" +
			"    constant integer MAX = 24\r\n    string URL = \"http://example\" // no comment in the string\r\n" +
			"    real array sizes\r\nendglobals\r\n\r\nnative Make takes player p, real x returns unit\r\n" +
			"constant native Get takes nothing returns unit\r\n",
		cut: outcomes{52, 260}, changed: outcomes{36, 24}, swept: outcomes{883, 127}},
	{name: "functions with bodies",
		text: "globals\n\treal angle = 0.0\nendglobals\nfunction Helper takes unit u, boolean b returns nothing\n" +
			"    local integer i = 0\n    call Make(null)\n    endfunctions\nendfunction // done\n" +
			"constant function One takes nothing returns integer\n    return 1\nendfunction\n",
		cut: outcomes{15, 236}, changed: outcomes{20, 40}, swept: outcomes{631, 66}},
	{name: "a line that is no declaration", text: "type unit extends widget\nlibrary Foo\nnative F takes nothing returns nothing\n",
		cut: outcomes{8, 68}, changed: outcomes{7, 53}, swept: outcomes{0, 183}},
	{name: "a body that never ends", text: "native A takes nothing returns nothing\n\nfunction F takes integer a returns nothing\n",
		cut: outcomes{10, 73}, changed: outcomes{4, 56}, swept: outcomes{0, 209}},
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

// damaged gives Parse the script cut at every length, after each of 60 seeded changes of its lines, quotes and
// white space, a byte order mark before every third, and with one character of ASCII white space put in at
// every place, and counts what it makes of each.
func damaged(t *testing.T, name, text string) (cut, changed, swept outcomes) {
	t.Helper()
	for length := range len(text) {
		cut.readOrRefused(t, fmt.Sprintf("%s cut at %d bytes", name, length), text[:length])
	}
	for index := range uint64(60) {
		made := testkit.Changed(text, damageSeed, index)
		if index%3 == 0 {
			made = byteOrderMark + made
		}
		changed.readOrRefused(t, fmt.Sprintf("%s, change %d of seed %d: %q", name, index, damageSeed, made), made)
	}
	for i, made := range testkit.Swept(text) {
		swept.readOrRefused(t, fmt.Sprintf("%s with white space put in, text %d: %q", name, i, made), made)
	}
	return cut, changed, swept
}

// TestADamagedScriptIsReadOrRefusedByFileAndLineAndNeverPanics damages the scripts of the tests and the scripts
// of this file, and compares what Parse makes of the damaged forms of the latter with the numbers written
// beside them.
func TestADamagedScriptIsReadOrRefusedByFileAndLineAndNeverPanics(t *testing.T) {
	named := scriptsOfTheTests()
	inputs := 0
	for _, name := range slices.Sorted(maps.Keys(named)) {
		cut, changed, swept := damaged(t, name, named[name])
		inputs += cut.read + cut.refused + changed.read + changed.refused + swept.read + swept.refused
	}
	if len(named) < 26 || inputs < 9000 {
		t.Errorf("%d scripts of the tests and %d damaged forms of them, want 26 and 9000 or more", len(named), inputs)
	}
	for _, c := range counted {
		cut, changed, swept := damaged(t, c.name, c.text)
		if cut != c.cut || changed != c.changed || swept != c.swept {
			t.Errorf("%s:\n got cut: %+v, changed: %+v, with white space: %+v\nwant cut: %+v, changed: %+v, with white space: %+v",
				c.name, cut, changed, swept, c.cut, c.changed, c.swept)
		}
	}
}
