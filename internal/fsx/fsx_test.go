package fsx

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

const bom = "\xEF\xBB\xBF"

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o666); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return "<missing>"
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func asDiagError(t *testing.T, err error) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v, want a *diag.Error", err)
	}
	return e
}

func TestIsWithin(t *testing.T) {
	dir := filepath.Join("projects", "map")
	for _, c := range []struct {
		path string
		want bool
		why  string
	}{
		{dir, true, "a path is within itself"},
		{filepath.Join(dir, "src", "main.yue"), true, "and its descendants"},
		{filepath.Join(dir, "..map", "x"), true, "a name starting with two dots is inside"},
		{filepath.Join("projects", "map-2"), false, "not its siblings"},
		{filepath.Join("projects", "other", "x"), false, "not its siblings' files"},
		{"projects", false, "not its parent"},
		{filepath.Join("Projects", "MAP", "src"), runtime.GOOS == "windows", "case on Windows only"},
	} {
		if got := IsWithin(c.path, dir); got != c.want {
			t.Errorf("IsWithin(%q) = %v: %s", c.path, got, c.why)
		}
	}
}

func TestSHA256HexHashesBytes(t *testing.T) {
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := SHA256Hex([]byte("abc")); got != want {
		t.Errorf("SHA256Hex = %s", got)
	}
}

var unsafePaths = []string{
	"", "/a", "a//b", "a/", "../a", "a/./b", "C:/a", "a/b?.blp", "a\tb", "a/b.", "a/b ", "con", "a/NUL.txt",
	"com1.blp", `a\..\b`,
}

func TestCleanRelPathRefusesWhatCouldLeaveItsFolderOrFailOnWindows(t *testing.T) {
	for _, c := range []struct{ value, want string }{
		{"icons/BTNSword.blp", "icons/BTNSword.blp"},
		{`icons\BTNSword.blp`, "icons/BTNSword.blp"},
		{"a/..b/c", "a/..b/c"},
		{"Textures/héro 1.blp", "Textures/héro 1.blp"},
		{"console.txt", "console.txt"},
	} {
		if got, ok := CleanRelPath(c.value); !ok || got != c.want {
			t.Errorf("RelPath(%q) = %q, %v, want %q", c.value, got, ok, c.want)
		}
	}
	for _, value := range unsafePaths {
		if got, ok := CleanRelPath(value); ok || got != "" {
			t.Errorf("RelPath(%q) = %q, %v, want it refused", value, got, ok)
		}
	}
}

func TestSafeJoinNamesAPathRelPathRefuses(t *testing.T) {
	root := t.TempDir()
	for _, value := range unsafePaths {
		got, err := SafeJoin(root, value)
		e := asDiagError(t, err)
		if got != "" || !strings.Contains(e.Msg, "Invalid path: "+value) || !strings.Contains(e.Hint, "relative path") {
			t.Errorf("SafeJoin(%q) = %q, %+v", value, got, e)
		}
	}
}

func TestSafeJoinRefusesASymlinkBelowTheRoot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "real", "a.txt"), "a")
	got, err := SafeJoin(root, "real/a.txt")
	if err != nil || got != filepath.Join(root, "real", "a.txt") {
		t.Errorf("SafeJoin = %q, %v", got, err)
	}
	if got, err := SafeJoin(root, "missing/b.txt"); err != nil || got != filepath.Join(root, "missing", "b.txt") {
		t.Errorf("SafeJoin of a missing path = %q, %v", got, err)
	}
	err = os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link"))
	switch {
	case err == nil:
	case runtime.GOOS == "windows" && errors.Is(err, syscall.Errno(1314)):
		t.Skipf("this account has not the right to make a symlink on Windows; the case is covered where it has: %v", err)
	default:
		t.Fatalf("no symlink was made: %v", err)
	}
	checkSymlinkRefused(t, root)
}

func TestSafeJoinRefusesAJunctionBelowTheRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows has junctions")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "real", "a.txt"), "a")
	mklink := exec.Command("cmd", "/c", "mklink", "/J", filepath.Join(root, "link"), filepath.Join(root, "real"))
	if out, err := mklink.CombinedOutput(); err != nil {
		t.Fatalf("mklink /J: %v\n%s", err, out)
	}
	checkSymlinkRefused(t, root)
}

func checkSymlinkRefused(t *testing.T, root string) {
	t.Helper()
	symlink := filepath.Join(root, "link")
	_, err := SafeJoin(root, "link/a.txt")
	e := asDiagError(t, err)
	if !strings.Contains(e.Msg, "Symlinks are not supported") || !strings.Contains(e.Msg, symlink) ||
		!strings.Contains(e.Hint, "real files") {
		t.Errorf("error = %+v", e)
	}
}
