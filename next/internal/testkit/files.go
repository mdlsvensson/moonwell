// Package testkit holds what Moonwell's tests share: files and folders written and read back and the fixtures World
// Editor saved, a test world for the commands and the logger that keeps its lines (Recorder), byte helpers, builders
// for map info, object files, MDX models and pictures, a reader of the archives Moonwell writes, and the way to a
// program a test needs (NeedPkl).
//
// It is imported by tests only. Of Moonwell's packages it knows env, binio and the formats it builds or reads
// (war3/objmod and war3/mpq), so the tests of those two packages are external test packages. Most helpers take a
// testing.TB and fail the test themselves; the archive reader returns errors.
package testkit

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// RepoRoot returns the root of the Moonwell checkout: the folder with go.mod, found by walking up from the working
// folder. A test starts in the folder of its package, so call RepoRoot before changing the working folder.
func RepoRoot(t testing.TB) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("no working folder to look for go.mod from: %v", err)
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no go.mod in the working folder or any folder above it")
			return ""
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
