package lua

import (
	"fmt"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func sourcesToMutate(t *testing.T) []namedSource {
	t.Helper()
	sources := append(append([]namedSource{}, cornerSources...), checkoutLuaFiles(t)...)
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
	for _, c := range mutationErrorCases() {
		sources = append(sources, namedSource{"refused: " + sourceName(c.source), c.source})
	}
	return sources
}

func mutationErrorCases() []errorCase {
	return slices.Concat(ambiguousStructures, deepNesting, invalidShapes, placedErrors, parameterErrorCases, returnErrorCases)
}

func sourceName(source string) string {
	if len(source) <= longest {
		return source
	}
	return fmt.Sprintf("%s... (%d bytes)", source[:longest], len(source))
}

const mutationSeed = 53

func scanOrFail(t *testing.T, what, source string) (read bool) {
	t.Helper()
	var made scanResult
	if value := testkit.PanicValue(func() { made = scan(source) }); value != nil {
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
	refusal := made.Error
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

func TestADamagedSourceIsScannedOrRefusedByNameAndNeverPanics(t *testing.T) {
	read, refused, sources := 0, 0, sourcesToMutate(t)
	count := func(what, source string) {
		if scanOrFail(t, what, source) {
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
			what := fmt.Sprintf("%s, change %d of seed %d", c.name, index, mutationSeed)
			count(what+" to its text", testkit.MutateText(c.source, mutationSeed, index))
			count(what+" to its bytes", string(testkit.MutateBytes([]byte(c.source), mutationSeed, index)))
		}
	}
	if read == 0 || refused == 0 {
		t.Errorf("%d damaged sources were read and %d refused; want some of each", read, refused)
	}
}
