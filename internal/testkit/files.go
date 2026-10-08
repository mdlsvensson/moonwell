package testkit

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
)

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

func LinkDir(t testing.TB, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, target).CombinedOutput(); err != nil {
			t.Fatalf("no junction was made at %s to %s: %v: %s", link, target, err, out)
		}
		return
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("no symlink was made at %s to %s: %v", link, target, err)
	}
}

const privilegeNotHeld = syscall.Errno(1314)

func LinkFile(t testing.TB, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	switch {
	case err == nil:
	case lacksTheRightToLink(err):
		t.Skipf("this account has not the right to make a symlink on Windows; the case is covered where it has: %v",
			err)
	default:
		t.Fatalf("no symlink was made at %s to %s: %v", link, target, err)
	}
}

func lacksTheRightToLink(err error) bool {
	return runtime.GOOS == "windows" && errors.Is(err, privilegeNotHeld)
}

func CaseSensitive(t testing.TB, dir string) bool {
	t.Helper()
	WriteFile(t, dir, "probe", nil)
	_, err := os.Stat(filepath.Join(dir, "PROBE"))
	os.Remove(filepath.Join(dir, "probe"))
	return err != nil
}
