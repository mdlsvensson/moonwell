package fsx

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
)

func TestReplaceDirReplacesDestinationContents(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "src", "new.txt"), "new")
	writeFile(t, filepath.Join(dir, "src", "deep", "er.txt"), "deeper")
	writeFile(t, filepath.Join(dir, "dest", "old.txt"), "old")
	if err := ReplaceDir(filepath.Join(dir, "src"), filepath.Join(dir, "dest")); err != nil {
		t.Fatal(err)
	}
	got, _ := ListFiles(filepath.Join(dir, "dest"))
	if !slices.Equal(got, []string{"deep/er.txt", "new.txt"}) {
		t.Errorf("dest holds %q", got)
	}
	if err := ReplaceDir(filepath.Join(dir, "src"), filepath.Join(dir, "a", "b", "dest")); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dir, "a", "b", "dest", "new.txt")); got != "new" {
		t.Errorf("copied file holds %q", got)
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
	if got := readFile(t, file); got != "b" {
		t.Errorf("file holds %q", got)
	}
}

func TestRemoveAllAndRemoveFileIgnoreMissingPaths(t *testing.T) {
	dir := t.TempDir()
	if err := RemoveAll(filepath.Join(dir, "missing")); err != nil {
		t.Error(err)
	}
	if err := RemoveFile(filepath.Join(dir, "missing")); err != nil {
		t.Error(err)
	}
	writeFile(t, filepath.Join(dir, "f"), "")
	if err := RemoveAll(filepath.Join(dir, "f")); err != nil {
		t.Error(err)
	}
	if got, _ := ListFiles(dir); len(got) != 0 {
		t.Errorf("left %q", got)
	}
}

func TestRemovingAFileAnotherProgramHoldsOpenNamesTheFileAndSaysToCloseTheGame(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only Windows refuses to delete an open file")
	}
	file := filepath.Join(t.TempDir(), "map.w3x")
	writeFile(t, file, "archive")
	unlock := lockFile(t, file)
	for _, remove := range []func(string) error{RemoveFile, RemoveAll} {
		e := asDiagError(t, remove(file))
		if !strings.Contains(e.Msg, "in use by another program") || !strings.Contains(e.Msg, file) ||
			!strings.Contains(e.Hint, "Warcraft III") || e.Cause == nil {
			t.Errorf("error = %+v", e)
		}
	}
	unlock()
	if err := RemoveFile(file); err != nil || Exists(file) {
		t.Errorf("after unlocking: %v, exists %v", err, Exists(file))
	}
}

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
