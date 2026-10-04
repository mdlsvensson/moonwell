package manifest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

func TestEnsureLocalManifestCreatesMoonwellLocalPklOnceAndNeverOverwritesIt(t *testing.T) {
	root := t.TempDir()
	local := filepath.Join(root, "moonwell.local.pkl")
	if created, err := EnsureLocalManifest(root); err != nil || !created {
		t.Fatalf("the first call: %v, %v", created, err)
	}
	if content, _ := os.ReadFile(local); string(content) != LocalPkl() {
		t.Errorf("the file holds %q", content)
	}
	testkit.WriteFile(t, root, "moonwell.local.pkl", []byte("mine"))
	if created, err := EnsureLocalManifest(root); err != nil || created {
		t.Errorf("the second call: %v, %v", created, err)
	}
	if content, _ := os.ReadFile(local); string(content) != "mine" {
		t.Errorf("the file was overwritten: %q", content)
	}
	if created, err := EnsureLocalManifest(filepath.Join(root, "no-such-folder")); err == nil || created {
		t.Errorf("in a folder that does not exist: %v, %v", created, err)
	}
}

func TestPklProjectDependsOnThePublishedPackageOrOnALocalOne(t *testing.T) {
	remote := PklProject("0.1.0", "")
	if !strings.Contains(remote, `["moonwell"] { uri = "`+PackageBaseURI+`@0.1.0" }`) ||
		PackageBaseURI != "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell" {
		t.Errorf("remote = %q", remote)
	}
	local := PklProject("", "../schema")
	if !strings.Contains(local, `["moonwell"] = import("../schema/PklProject")`) {
		t.Errorf("local = %q", local)
	}
	// The template in the checkout is a project linked to the schema beside it.
	template, err := os.ReadFile(filepath.Join(testkit.RepoRoot(t), "template", "PklProject"))
	if err != nil || string(template) != local {
		t.Errorf("template/PklProject = %q, %v", template, err)
	}
}

func TestLocalPklAmendsMoonwellPklAndSetsTheDefaultGamePathEscaped(t *testing.T) {
	local := LocalPkl()
	for _, want := range []string{
		`amends "moonwell.pkl"`,
		`gameExecutable = "C:\\Program Files (x86)\\Warcraft III\\_retail_\\x86_64\\Warcraft III.exe"`,
		"`moonwell setup` recreates it.",
	} {
		if !strings.Contains(local, want) {
			t.Errorf("LocalPkl lacks %q:\n%s", want, local)
		}
	}
	if DefaultGameExecutable != `C:\Program Files (x86)\Warcraft III\_retail_\x86_64\Warcraft III.exe` {
		t.Errorf("DefaultGameExecutable = %q", DefaultGameExecutable)
	}
}
