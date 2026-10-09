package manifest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestPklProjectDependsOnThePublishedPackageOrOnALocalOne(t *testing.T) {
	remote := PklProjectText("0.1.0", "")
	const wantRemote = "amends \"pkl:Project\"\n\ndependencies {\n" +
		"  [\"moonwell\"] { uri = \"package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0.1.0\" }\n}\n"
	if remote != wantRemote {
		t.Errorf("remote = %q, want %q", remote, wantRemote)
	}
	local := PklProjectText("", "../schema")
	const wantLocal = "amends \"pkl:Project\"\n\ndependencies {\n  [\"moonwell\"] = import(\"../schema/PklProject\")\n}\n"
	if local != wantLocal {
		t.Errorf("local = %q, want %q", local, wantLocal)
	}
	template, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "template", "PklProject"))
	if err != nil || string(template) != local {
		t.Errorf("template/PklProject = %q, %v", template, err)
	}
}
