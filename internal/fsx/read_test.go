package fsx

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestListFilesReturnsPosixRelativePathsSortedByBytes(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "z.txt"), "")
	writeFile(t, filepath.Join(dir, "a", "b", "c.txt"), "")
	writeFile(t, filepath.Join(dir, "Y.txt"), "")
	writeFile(t, filepath.Join(dir, "a.txt"), "")
	got, err := ListFiles(dir)
	if err != nil || !slices.Equal(got, []string{"Y.txt", "a.txt", "a/b/c.txt", "z.txt"}) {
		t.Errorf("ListFiles = %q, %v", got, err)
	}
}

func TestReadIfThereFindsNoFileWhereNoneCanBeAndFailsForAnythingElse(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "held.txt"), "held")
	if err := os.Mkdir(filepath.Join(dir, "folder"), 0o777); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct {
		name, path, data string
		found            bool
	}{
		{"a file", "held.txt", "held", true},
		{"nothing at the path", "gone.txt", "", false},
		{"nothing on the way to the path", "gone/below.txt", "", false},
		{"a file on the way to the path", "held.txt/below.txt", "", false},
	} {
		data, found, err := ReadFileIfExists(filepath.Join(dir, filepath.FromSlash(c.path)))
		if string(data) != c.data || found != c.found || err != nil {
			t.Errorf("%s: ReadIfThere = %q, %v, %v", c.name, data, found, err)
		}
	}
	if data, found, err := ReadFileIfExists(filepath.Join(dir, "folder")); err == nil || found || data != nil {
		t.Errorf("a folder: ReadIfThere = %q, %v, %v", data, found, err)
	}
}

func TestReadSourceDropsALeadingByteOrderMarkAndBlanksAFirstLineStartingWithHash(t *testing.T) {
	file := filepath.Join(t.TempDir(), "x.lua")
	for _, c := range []struct{ content, want string }{
		{bom + "Count = 0\n", "Count = 0\n"},
		{bom + "#!/usr/bin/lua\r\nCount = 0\n# not the first line\n", "\r\nCount = 0\n# not the first line\n"},
		{"#only line", ""},
		{"Count = 0 -- " + bom + " kept\n", "Count = 0 -- " + bom + " kept\n"},
		{"bad \xFF byte\n", "bad \xFF byte\n"},
		{"s = '\xE9\xE9\xFF' -- \xE2\x80\n", "s = '\xE9\xE9\xFF' -- \xE2\x80\n"},
		{bom + "#!lua \xFF\n-- \xC0\xC1", "\n-- \xC0\xC1"},
	} {
		writeFile(t, file, c.content)
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
	e := asDiagError(t, err)
	if !strings.Contains(e.Msg, "Reading lua/x.lua failed") || e.File != "lua/x.lua" || e.Cause == nil ||
		!strings.Contains(e.Hint, "readable") {
		t.Errorf("error = %+v", e)
	}
	_, err = ReadSource(filepath.Join(dir, "gone.lua"), "lua/gone.lua")
	if e := asDiagError(t, err); e.File != "lua/gone.lua" {
		t.Errorf("error = %+v", e)
	}
}
