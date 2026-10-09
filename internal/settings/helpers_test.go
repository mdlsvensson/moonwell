package settings

import (
	"errors"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

const manifestName = "moonwell.local.pkl"

const plainBlocks = `"map":{"folder":"map.w3x","entry":"src/main.yue"},"build":{"folder":"dist/bin","minify":false},` +
	`"launch":{"args":[]},"yue":{"version":"0.34.3"},"assets":{"paths":{},"exclude":[]},` +
	`"lint":{"unknownGlobals":"error","globals":[]},"libraries":{},"objects":{}`

func projectOf(t testing.TB, root, document string) *manifest.Project {
	t.Helper()
	project, err := manifest.DecodeProject(root, manifestName, []byte("{"+plainBlocks+`,"settings":`+document+"}"))
	if err != nil {
		t.Fatalf("settings %s: %v", document, diag.Format(err))
	}
	return project
}

func settingsOf(t testing.TB, document string) manifest.Settings {
	t.Helper()
	return projectOf(t, "", document).Settings
}

func asError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return diagErr
}
