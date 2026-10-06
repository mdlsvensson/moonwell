package oracle

import (
	"errors"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// RefusalOf is what an error of the other tree says of the input it refuses, as testkit.RefusalOf gives it for
// an error of this tree. With it an oracle makes a recording of refusals from what the other tree says:
// testkit.Recorded(t, "refusals.txt", testkit.Refusals(...)) of these, run alone with MOONWELL_RECORD=1.
func RefusalOf(input string, err error) testkit.Refusal {
	var failure *olddiag.Error
	switch {
	case err == nil:
		return testkit.RefusalOf(input, nil)
	case errors.As(err, &failure):
		return testkit.Refusal{
			Input: input, File: failure.File, Message: failure.Msg, Hint: failure.Hint,
			Line: failure.Line, Column: failure.Column,
		}
	}
	return testkit.Refusal{Input: input, Message: err.Error()}
}
