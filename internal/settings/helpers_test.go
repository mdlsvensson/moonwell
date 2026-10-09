package settings

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/manifest"
)

const manifestName = manifest.ProjectFile

func mustDecodeProject(t testing.TB, root, document string) *manifest.Project {
	t.Helper()
	project := &manifest.Project{Root: root, ManifestName: manifestName}
	if err := json.Unmarshal([]byte(document), &project.Settings); err != nil {
		t.Fatalf("settings %s: %v", document, err)
	}
	return project
}

func mustDecodeSettings(t testing.TB, document string) manifest.Settings {
	t.Helper()
	return mustDecodeProject(t, "", document).Settings
}

func asDiagError(t testing.TB, err error, what string) *diag.Error {
	t.Helper()
	var diagErr *diag.Error
	if !errors.As(err, &diagErr) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return diagErr
}
