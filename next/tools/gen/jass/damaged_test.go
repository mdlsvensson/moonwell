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

// everyScript is every script of the tests of this package, the ones that are read and the ones that are
// refused, each with a name for a failure.
func everyScript() map[string]string {
	named := map[string]string{
		"common": common, "blizzard": blizzard, "corners": corners, "indented": indented,
		"no declaration": noDeclaration,
	}
	for i, c := range refused {
		named[fmt.Sprintf("refused script %02d, of %s", i, c.source)] = c.text
	}
	for _, c := range widerSpace {
		named[c.name] = c.text
	}
	return named
}

// damageSeed is the seed of the changes that TestADamagedScriptIsReadOrRefusedByFileAndLineAndNeverPanics makes.
// A failure names the script and the index of the change: testkit.Changed makes the same text of the three
// again.
const damageSeed = 2003

// damagedFile is the name the test gives Parse for its errors.
const damagedFile = "scripts/damaged.j"

// byteOrderMark is U+FEFF in UTF-8, which is text to Parse.
const byteOrderMark = "\xEF\xBB\xBF"

// outcomes counts the damaged scripts that were read and the ones that were refused.
type outcomes struct{ read, refused int }

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

// TestADamagedScriptIsReadOrRefusedByFileAndLineAndNeverPanics gives Parse every script of the tests cut at every
// length, after each of 60 seeded changes of its lines, quotes and white space, a byte order mark before every
// third, and with one character of ASCII white space put in at every place.
func TestADamagedScriptIsReadOrRefusedByFileAndLineAndNeverPanics(t *testing.T) {
	var cut, changed, swept outcomes
	named := everyScript()
	for _, name := range slices.Sorted(maps.Keys(named)) {
		text := named[name]
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
	}
	// The numbers are what Parse makes of these inputs: one that differs is a reading that changed.
	if len(named) != 25 || cut != (outcomes{220, 1919}) || changed != (outcomes{319, 1181}) ||
		swept != (outcomes{3301, 2263}) {
		t.Errorf("%d scripts; cut: %+v, changed: %+v, with white space put in: %+v", len(named), cut, changed, swept)
	}
}
