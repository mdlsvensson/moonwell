package objects_test

import (
	"errors"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

var mini = testkit.MiniMetadata()

// manifest parses the `objects` JSON of a manifest; objects without a source get objects/a.pkl.
func manifest(t *testing.T, document string) objects.Manifest {
	t.Helper()
	tree, err := ordered.Decode([]byte(document))
	if err != nil {
		t.Fatalf("test JSON %s: %v", document, err)
	}
	parsed, err := objects.ParseManifest(tree, true, "objects/a.pkl")
	if err != nil {
		t.Fatalf("ParseManifest(%s): %v", document, err)
	}
	return parsed
}

func ids(existing []string) map[string]bool {
	set := map[string]bool{}
	for _, id := range existing {
		set[id] = true
	}
	return set
}

func resolve(t *testing.T, document string, existing ...string) []objects.Resolved {
	t.Helper()
	resolved, err := objects.Resolve(mini, manifest(t, document), ids(existing))
	if err != nil {
		t.Fatalf("Resolve(%s): %v", document, diag.Format(err))
	}
	return resolved
}

// problems returns the problems resolving the manifest reports.
func problems(t *testing.T, document string, existing ...string) diag.Problems {
	t.Helper()
	_, err := objects.Resolve(mini, manifest(t, document), ids(existing))
	var found diag.Problems
	if !errors.As(err, &found) {
		t.Fatalf("Resolve(%s) = %v, want problems", document, err)
	}
	return found
}

// problem checks that resolving the manifest reports exactly one problem, with this message and hint.
func problem(t *testing.T, document, message, hint string, existing ...string) {
	t.Helper()
	found := problems(t, document, existing...)
	if len(found) != 1 {
		t.Fatalf("Resolve(%s) reports %d problems:\n%s", document, len(found), diag.Format(found))
	}
	if found[0].Msg != message {
		t.Errorf("message:\n got %s\nwant %s", found[0].Msg, message)
	}
	if hint != "*" && found[0].Hint != hint {
		t.Errorf("hint:\n got %s\nwant %s", found[0].Hint, hint)
	}
}

func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return e
}

func fixture(t *testing.T, name string) []byte {
	return testkit.Fixture(t, "objects-v3-names/"+name)
}
