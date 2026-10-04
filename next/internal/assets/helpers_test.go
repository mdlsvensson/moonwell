package assets

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// manifestName is the manifest the projects of these tests were evaluated from.
const manifestName = "moonwell.local.pkl"

// noBlock is the assets block of a manifest that sets nothing in it.
const noBlock = `{"paths":{},"exclude":[]}`

// asError is err as the expected failure it must be.
func asError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}

// put writes a file of the project, with "asset" in it unless the test gives its content.
func put(t testing.TB, root, file string, content ...string) string {
	t.Helper()
	held := "asset"
	if len(content) > 0 {
		held = content[0]
	}
	return testkit.WriteFile(t, root, file, []byte(held))
}

// blockOf is a manifest's assets block as pkl prints it, with its paths in the order of the text.
func blockOf(t testing.TB, document string) manifest.Assets {
	t.Helper()
	var block manifest.Assets
	if err := json.Unmarshal([]byte(document), &block); err != nil {
		t.Fatalf("the assets block %s: %v", document, err)
	}
	return block
}

// shipped is the libraries with these keys, each with its files in the folder libraries/<key> of the project.
func shipped(root string, keys ...string) []Library {
	var libraries []Library
	for _, key := range keys {
		libraries = append(libraries, Library{Key: key, Dir: filepath.Join(root, "libraries", key)})
	}
	return libraries
}

// collect is what a project with this assets block and these libraries imports. It must not be refused.
func collect(t testing.TB, root, block string, libraries ...string) ([]Asset, []string) {
	t.Helper()
	assets, replaced, err := Collect(root, blockOf(t, block), manifestName, shipped(root, libraries...))
	if err != nil {
		t.Fatalf("Collect: %v", diag.Format(err))
	}
	return assets, replaced
}

// refused is the failure that a project with this assets block and these libraries is refused with.
func refused(t testing.TB, root, block string, libraries ...string) *diag.Error {
	t.Helper()
	assets, replaced, err := Collect(root, blockOf(t, block), manifestName, shipped(root, libraries...))
	if assets != nil || replaced != nil {
		t.Errorf("Collect returned %d assets and %q beside its error", len(assets), replaced)
	}
	return asError(t, err, "the assets block "+block)
}

// row is an asset without its bytes.
type row struct{ library, source, target string }

func rows(assets []Asset) []row {
	list := []row{}
	for _, asset := range assets {
		list = append(list, row{asset.Library, asset.Source, asset.Target})
	}
	return list
}
