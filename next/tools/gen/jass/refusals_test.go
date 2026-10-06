package jass_test

import (
	"fmt"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/tools/gen/jass"
)

// refusedScript is a script that Parse refuses, with the name of its file and a name for the recording.
type refusedScript struct{ name, text, source string }

// refusedScripts are the scripts the tests of this package hold as refused: the ones of refused, and the ones
// of widerSpace that are refused for a character that is white space outside ASCII.
func refusedScripts() []refusedScript {
	var scripts []refusedScript
	for i, c := range refused {
		scripts = append(scripts, refusedScript{fmt.Sprintf("refused script %02d, of %s", i, c.source), c.text, c.source})
	}
	for _, c := range widerSpace {
		if c.words != "" {
			scripts = append(scripts, refusedScript{c.name, c.text, "wide.j"})
		}
	}
	return scripts
}

// TestRefusalsAreAsRecorded holds what Parse says of every script of refusedScripts to the recording: the text
// of each error, whole. The tests of an error beside it ask for its place and its distinguishing words; the
// words themselves are held here, where a change of them is one line of a diff.
func TestRefusalsAreAsRecorded(t *testing.T) {
	var said []testkit.Refusal
	for _, script := range refusedScripts() {
		_, err := jass.Parse(script.text, script.source)
		if err == nil {
			t.Errorf("%s is read", script.name)
		}
		said = append(said, testkit.RefusalOf(script.name, err))
	}
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
}
