package fsx

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o777); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o666); err != nil {
		t.Fatal(err)
	}
}

func asError(t *testing.T, err error) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v, want a *diag.Error", err)
	}
	return e
}

func TestListFilesReturnsSortedPosixRelativePaths(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "z.txt"), "")
	write(t, filepath.Join(dir, "a", "b", "c.txt"), "")
	got, err := ListFiles(dir)
	if err != nil || !slices.Equal(got, []string{"a/b/c.txt", "z.txt"}) {
		t.Errorf("ListFiles = %q, %v", got, err)
	}
}

func TestReplaceDirReplacesDestinationContents(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "src", "new.txt"), "new")
	write(t, filepath.Join(dir, "src", "deep", "er.txt"), "deeper")
	write(t, filepath.Join(dir, "dest", "old.txt"), "old")
	if err := ReplaceDir(filepath.Join(dir, "src"), filepath.Join(dir, "dest")); err != nil {
		t.Fatal(err)
	}
	got, _ := ListFiles(filepath.Join(dir, "dest"))
	if !slices.Equal(got, []string{"deep/er.txt", "new.txt"}) {
		t.Errorf("dest holds %q", got)
	}
	// A destination whose parent is missing is created.
	if err := ReplaceDir(filepath.Join(dir, "src"), filepath.Join(dir, "a", "b", "dest")); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "a", "b", "dest", "new.txt")); string(data) != "new" {
		t.Errorf("copied file holds %q", data)
	}
}

func TestWriteIfChangedOnlyWritesDifferingContent(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x", "y.txt")
	for _, c := range []struct {
		content string
		want    bool
	}{{"a", true}, {"a", false}, {"b", true}} {
		got, err := WriteIfChanged(file, c.content)
		if err != nil || got != c.want {
			t.Errorf("WriteIfChanged(%q) = %v, %v, want %v", c.content, got, err, c.want)
		}
	}
	if data, _ := os.ReadFile(file); string(data) != "b" {
		t.Errorf("file holds %q", data)
	}
}

func TestReadSourceStripsALeadingBOMAndBlanksAFirstLineStartingWithHash(t *testing.T) {
	const bom = "\xEF\xBB\xBF"
	file := filepath.Join(t.TempDir(), "x.lua")
	for _, c := range []struct{ content, want string }{
		{bom + "Count = 0\n", "Count = 0\n"},
		{bom + "#!/usr/bin/lua\r\nCount = 0\n# not the first line\n", "\r\nCount = 0\n# not the first line\n"},
		{"#only line", ""},
		{"Count = 0 -- " + bom + " kept\n", "Count = 0 -- " + bom + " kept\n"},
		{"bad \xFF byte\n", "bad \uFFFD byte\n"},
	} {
		write(t, file, c.content)
		got, err := ReadSource(file, "lua/x.lua")
		if err != nil || got != c.want {
			t.Errorf("ReadSource(%q) = %q, %v, want %q", c.content, got, err, c.want)
		}
	}
}

func TestReadSourceReportsAFileItCannotReadNamingTheLabel(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "x.lua"), 0o777); err != nil {
		t.Fatal(err)
	}
	_, err := ReadSource(filepath.Join(dir, "x.lua"), "lua/x.lua")
	e := asError(t, err)
	if !strings.Contains(e.Msg, "Reading lua/x.lua failed") || e.File != "lua/x.lua" || e.Cause == nil ||
		!strings.Contains(e.Hint, "readable") {
		t.Errorf("error = %+v", e)
	}
	_, err = ReadSource(filepath.Join(dir, "gone.lua"), "lua/gone.lua")
	if e := asError(t, err); e.File != "lua/gone.lua" {
		t.Errorf("error = %+v", e)
	}
}

func TestIsWithin(t *testing.T) {
	folder := filepath.Join("projects", "map")
	for _, c := range []struct {
		path string
		want bool
		why  string
	}{
		{folder, true, "a path is within itself"},
		{filepath.Join(folder, "src", "main.yue"), true, "and its descendants"},
		{filepath.Join(folder, "..map", "x"), true, "a name starting with two dots is inside"},
		{filepath.Join("projects", "map-2"), false, "not its siblings"},
		{filepath.Join("projects", "other", "x"), false, "not its siblings' files"},
		{"projects", false, "not its parent"},
		{filepath.Join("Projects", "MAP", "src"), runtime.GOOS == "windows", "case on Windows only"},
	} {
		if got := IsWithin(c.path, folder); got != c.want {
			t.Errorf("IsWithin(%q) = %v: %s", c.path, got, c.why)
		}
	}
}

func TestRemoveAllIgnoresMissingPaths(t *testing.T) {
	dir := t.TempDir()
	if err := RemoveAll(filepath.Join(dir, "missing")); err != nil {
		t.Error(err)
	}
	if err := RemoveFile(filepath.Join(dir, "missing")); err != nil {
		t.Error(err)
	}
	write(t, filepath.Join(dir, "f"), "")
	if err := RemoveAll(filepath.Join(dir, "f")); err != nil {
		t.Error(err)
	}
	if got, _ := ListFiles(dir); len(got) != 0 {
		t.Errorf("left %q", got)
	}
}

func TestSHA256HexHashesBytes(t *testing.T) {
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := SHA256Hex([]byte("abc")); got != want {
		t.Errorf("SHA256Hex = %s", got)
	}
}

// lockFile holds path open without sharing, as a running Warcraft III holds its map, until the returned function is
// called.
func lockFile(t *testing.T, path string) func() {
	t.Helper()
	script := "$h = [System.IO.File]::Open('" + path + "', 'Open', 'Read', 'None'); 'locked'; [Console]::In.ReadLine()"
	cmd := exec.Command("powershell", "-NoProfile", "-Command", script)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	scanner := bufio.NewScanner(stdout)
	locked := false
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "locked") {
			locked = true
			break
		}
	}
	if !locked {
		t.Fatal("the locking process exited early")
	}
	return func() {
		stdin.Close()
		cmd.Wait()
	}
}

func TestRemovingAFileAnotherProgramHoldsOpenNamesTheFileAndSaysToCloseTheGame(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows refuses to delete an open file")
	}
	file := filepath.Join(t.TempDir(), "map.w3x")
	write(t, file, "archive")
	unlock := lockFile(t, file)
	for _, remove := range []func(string) error{RemoveFile, RemoveAll} {
		e := asError(t, remove(file))
		if !strings.Contains(e.Msg, "in use by another program") || !strings.Contains(e.Msg, file) ||
			!strings.Contains(e.Hint, "Warcraft III") {
			t.Errorf("error = %+v", e)
		}
	}
	unlock()
	if err := RemoveFile(file); err != nil || Exists(file) {
		t.Errorf("after unlocking: %v, exists %v", err, Exists(file))
	}
}

func TestRelPathRejectsWhatCouldEscapeOrFailOnWindows(t *testing.T) {
	for input, want := range map[string]string{
		"icons/BTNSword.blp":  "icons/BTNSword.blp",
		`icons\BTNSword.blp`:  "icons/BTNSword.blp",
		"a/..b/c":             "a/..b/c",
		"Textures/héro 1.blp": "Textures/héro 1.blp",
	} {
		if got, err := RelPath(input); err != nil || got != want {
			t.Errorf("RelPath(%q) = %q, %v", input, got, err)
		}
	}
	for _, input := range []string{
		"", "/a", "a//b", "a/", "../a", "a/./b", "C:/a", "a/b?.blp", "a\tb", "a/b.", "a/b ", "con", "a/NUL.txt",
		"com1.blp", `a\..\b`,
	} {
		_, err := RelPath(input)
		if e := asError(t, err); e.Msg != "Invalid asset path: "+input {
			t.Errorf("RelPath(%q) error = %q", input, e.Msg)
		}
	}
	if _, err := RelPath("console.txt"); err != nil {
		t.Errorf("a name that only starts like a device is fine: %v", err)
	}
}

func TestSafeJoinRefusesASymlinkBelowTheRoot(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "real", "a.txt"), "a")
	got, err := SafeJoin(root, "real/a.txt")
	if err != nil || got != filepath.Join(root, "real", "a.txt") {
		t.Errorf("SafeJoin = %q, %v", got, err)
	}
	if got, err := SafeJoin(root, "missing/b.txt"); err != nil || got != filepath.Join(root, "missing", "b.txt") {
		t.Errorf("SafeJoin of a missing path = %q, %v", got, err)
	}
	if err := os.Symlink(filepath.Join(root, "real"), filepath.Join(root, "link")); err != nil {
		t.Skipf("cannot create a symlink here: %v", err)
	}
	_, err = SafeJoin(root, "link/a.txt")
	if e := asError(t, err); !strings.HasPrefix(e.Msg, "Symlinks are not supported: ") {
		t.Errorf("error = %+v", e)
	}
}
