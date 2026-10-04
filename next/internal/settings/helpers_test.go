package settings

import (
	"errors"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
)

// manifestName is the manifest the projects of these tests were evaluated from.
const manifestName = "moonwell.local.pkl"

// plainBlocks is what pkl prints for the blocks of a new project other than its settings. A project is decoded
// whole, so a test that needs only settings has these around them.
const plainBlocks = `"map":{"folder":"map.w3x","entry":"src/main.yue"},"build":{"folder":"dist/bin","minify":false},` +
	`"launch":{"args":[]},"yue":{"version":"0.34.3"},"assets":{"paths":{},"exclude":[]},` +
	`"lint":{"unknownGlobals":"error","globals":[]},"libraries":{},"objects":{}`

// projectOf is the project in the folder root whose manifest has the document as its settings block.
func projectOf(t testing.TB, root, document string) *manifest.Project {
	t.Helper()
	project, err := manifest.Decode(root, manifestName, []byte("{"+plainBlocks+`,"settings":`+document+"}"))
	if err != nil {
		t.Fatalf("settings %s: %v", document, diag.Format(err))
	}
	return project
}

// settingsOf is the settings a manifest has whose settings block is the document.
func settingsOf(t testing.TB, document string) manifest.Settings {
	t.Helper()
	return projectOf(t, "", document).Settings
}

// asError is err as the expected failure it must be.
func asError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return failure
}
