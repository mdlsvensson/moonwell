package lua

import (
	"fmt"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// refusedSources are the sources the lists of this package hold as refused by Functions.
func refusedSources() []refusal {
	return slices.Concat(ambiguousStructures, deepNesting, invalidShapes, placedErrors, parameterRefusals, returnRefusals)
}

// sourceName is a source as the name of an input: whole when it is short, and its start with its length when it
// is long.
func sourceName(source string) string {
	if len(source) <= longest {
		return source
	}
	return fmt.Sprintf("%s... (%d bytes)", source[:longest], len(source))
}

// overlappingEdits are two edits of one place.
var overlappingEdits = []Edit{{2, 5, "A"}, {4, 6, "B"}}

// TestRefusalsAreAsRecorded holds what Functions says of every source of refusedSources, and what ApplyEdits
// says of edits that overlap, to the recording: the file, the message, the hint and the place of each error,
// whole. The tests of an error beside it ask for its file, its place, its distinguishing words and a hint; the
// words themselves are held here, where a change of them is one line of a diff.
func TestRefusalsAreAsRecorded(t *testing.T) {
	var said []testkit.Refusal
	for _, c := range refusedSources() {
		_, err := Functions(c.source, "map.lua")
		if err == nil {
			t.Errorf("Functions reads:\n%s", c.source)
		}
		said = append(said, testkit.RefusalOf(sourceName(c.source), err))
	}
	_, err := ApplyEdits("0123456789", overlappingEdits)
	said = append(said, testkit.RefusalOf("edits that overlap", err))
	testkit.Recorded(t, "refusals.txt", testkit.Refusals(said))
}
