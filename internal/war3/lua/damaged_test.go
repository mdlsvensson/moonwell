package lua

import (
	"fmt"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// sourcesToDamage are the corner sources, the Lua files of the checkout, and the sources that the tests of this
// package keep in their constants and lists, the ones that are read and the ones that are refused.
func sourcesToDamage(t *testing.T) []namedSource {
	t.Helper()
	sources := append(append([]namedSource{}, cornerSources...), luaFilesOfTheCheckout(t)...)
	sources = append(sources,
		namedSource{"direct calls", directCalls}, namedSource{"every expression", everyExpression},
		namedSource{"token boundaries", tokenBoundaries}, namedSource{"adjacent statements", adjacentStatements},
		namedSource{"declaration forms", declarationForms}, namedSource{"every spelling", everySpelling},
		namedSource{"the lowest operator", lowestOperator}, namedSource{"escaped strings", escapedStrings},
		namedSource{"an open string before a require", afterAnOpenString},
	)
	for _, c := range malformedSources {
		sources = append(sources, namedSource{"malformed: " + c.source, c.source})
	}
	for _, c := range refusedSources() {
		sources = append(sources, namedSource{"refused: " + sourceName(c.source), c.source})
	}
	return sources
}

// refusedSources are the sources the lists of this package hold as refused by Functions.
func refusedSources() []refusal {
	return slices.Concat(ambiguousStructures, deepNesting, invalidShapes, placedErrors, parameterRefusals, returnRefusals)
}

// sourceName is a source as a name in a failure: whole when it is short, and its start with its length when it
// is long.
func sourceName(source string) string {
	if len(source) <= longest {
		return source
	}
	return fmt.Sprintf("%s... (%d bytes)", source[:longest], len(source))
}

// damageSeed is the seed of the changes that TestADamagedSourceIsScannedOrRefusedByNameAndNeverPanics makes. A
// failure names the source and the index of the change: testkit.Changed, and testkit.ChangedBytes for a change
// of the bytes, make the same source of the three again.
const damageSeed = 53

// scannedOrRefused gives the source to the tokenizer and to every scanner. It stops the test when one of them
// panics; when a token is not the bytes of the source it says it is, or stands before the one before it; when
// the fault is no place in the source; and when Functions returns functions with an error, an error that is no
// *diag.Error of the file with a hint, a line and a column, or a function that is no part of the source. It
// reports whether Functions read the source.
func scannedOrRefused(t *testing.T, what, source string) (read bool) {
	t.Helper()
	var made scanned
	if value := testkit.Panic(func() { made = scan(source) }); value != nil {
		t.Fatalf("%s: a scanner panics: %v", what, value)
	}
	end := 0
	for _, token := range made.Tokens {
		if token.Start < end || token.End <= token.Start || token.End > len(source) ||
			source[token.Start:token.End] != token.Raw || token.Line < 1 {
			t.Fatalf("%s: the token %+v is not the bytes of the source after the token before it", what, token)
		}
		end = token.End
	}
	if made.Fault != nil && (made.Fault.Offset < 0 || made.Fault.Offset > len(source) || made.Fault.Msg == "") {
		t.Fatalf("%s: the fault %+v is no place in the source", what, made.Fault)
	}
	refusal := made.Refusal
	switch {
	case refusal.Message == "":
		for _, function := range made.Functions {
			if function.Start < 0 || function.EndStart < function.Start || function.End > len(source) ||
				source[function.EndStart:function.End] != "end" {
				t.Fatalf("%s: the function %s at %d to %d is no part of the source", what, function.Name,
					function.Start, function.End)
			}
		}
		return true
	case made.Functions == nil && refusal.File == "war3map.lua" && refusal.Hint != "" && refusal.Line >= 1 &&
		refusal.Column >= 1:
		return false
	}
	t.Fatalf("%s: Functions = %d functions, %+v; want functions, or an error of war3map.lua with its place", what,
		len(made.Functions), refusal)
	return false
}

// TestADamagedSourceIsScannedOrRefusedByNameAndNeverPanics gives the tokenizer and the scanners every source of
// sourcesToDamage cut short and after seeded changes: of its lines, quotes and white space, and of its bytes. A
// source of up to 500 bytes is cut at every length and changed 20 times in each way; a longer one is cut at
// every length below 500 and at every 97th after it and changed 10 times in each way; and one of more than
// 16000 bytes, which is one of the two that nest too deep, is cut at every 4099th after the 500 and changed
// three times in each way: a scan of it takes a thousand times as long as one of a corner source.
func TestADamagedSourceIsScannedOrRefusedByNameAndNeverPanics(t *testing.T) {
	read, refused, sources := 0, 0, sourcesToDamage(t)
	count := func(what, source string) {
		if scannedOrRefused(t, what, source) {
			read++
		} else {
			refused++
		}
	}
	for _, c := range sources {
		step, changes := 97, uint64(20)
		switch {
		case len(c.source) > 16000:
			step, changes = 4099, 3
		case len(c.source) > 500:
			changes = 10
		}
		for length := 0; length < len(c.source); length++ {
			if length >= 500 && length%step != 0 {
				continue
			}
			count(fmt.Sprintf("%s, cut at %d bytes", c.name, length), c.source[:length])
		}
		for index := range changes {
			what := fmt.Sprintf("%s, change %d of seed %d", c.name, index, damageSeed)
			count(what+" to its text", testkit.Changed(c.source, damageSeed, index))
			count(what+" to its bytes", string(testkit.ChangedBytes([]byte(c.source), damageSeed, index)))
		}
	}
	if read == 0 || refused == 0 {
		t.Errorf("%d damaged sources were read and %d refused; want some of each", read, refused)
	}
}
