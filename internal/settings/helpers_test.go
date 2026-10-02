package settings_test

import (
	"errors"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/settings"
)

// tree parses a JSON document into the tree the manifest is read into.
func tree(t *testing.T, document string) any {
	t.Helper()
	value, err := ordered.Decode([]byte(document))
	if err != nil {
		t.Fatalf("test JSON %s: %v", document, err)
	}
	return value
}

// validated returns the settings of a JSON document, which must be valid.
func validated(t *testing.T, document string) *settings.Settings {
	t.Helper()
	s, err := settings.Validate(tree(t, document), "")
	if err != nil {
		t.Fatalf("Validate(%s): %v", document, err)
	}
	return s
}

// asError returns err as a *diag.Error, failing the test when it is another kind of error or nil.
func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return e
}

// sections builds Sections from alternating section names and key, value pairs.
func sections(t *testing.T, document string) settings.Sections {
	t.Helper()
	return validated(t, `{"gameInterface":`+document+`}`).GameInterface
}
