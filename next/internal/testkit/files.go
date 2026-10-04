// Package testkit holds what Moonwell's tests share: the folder of the checkout, files and folders written and read
// back, the files World Editor saved, builders for binary data, a recording logger and a test world for the commands.
// Its helpers take a testing.TB and fail the test themselves. It is imported by tests only, and it knows no package
// of Moonwell but env and binio.
package testkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// RepoRoot returns the root of the Moonwell checkout: the folder with go.mod, found by walking up from the folder the
// test runs in.
func RepoRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod in the working folder or any folder above it")
		}
		dir = parent
	}
}

// WriteFile writes a file under dir, creating its folders. name uses "/".
func WriteFile(t testing.TB, dir, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o666); err != nil {
		t.Fatal(err)
	}
	return path
}

// Snapshot returns every entry below dir with its bytes (folders as nil), to prove that something wrote nothing.
func Snapshot(t testing.TB, dir string) map[string][]byte {
	t.Helper()
	result := map[string][]byte{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		key := filepath.ToSlash(rel)
		if entry.IsDir() {
			result[key] = nil
			return nil
		}
		data, err := os.ReadFile(path)
		if data == nil {
			data = []byte{}
		}
		result[key] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

// LinkDir makes link a link to the folder target: a symlink, or a junction on Windows, where symlinks need a
// privilege. The test is skipped when the machine allows neither.
func LinkDir(t testing.TB, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Skipf("cannot create a junction here: %v: %s", err, out)
		}
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
}

// CaseSensitive reports whether dir's file system keeps two names that differ only in letter case apart (Linux
// usually; Windows never).
func CaseSensitive(t testing.TB, dir string) bool {
	t.Helper()
	WriteFile(t, dir, "probe", nil)
	_, err := os.Stat(filepath.Join(dir, "PROBE"))
	os.Remove(filepath.Join(dir, "probe"))
	return err != nil
}
