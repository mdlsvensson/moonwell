package fsx_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func refusedInside(t *testing.T, root, relative string) *diag.Error {
	t.Helper()
	place, err := fsx.Inside(root, relative)
	var failure *diag.Error
	if !errors.As(err, &failure) {
		t.Fatalf("Inside(%q) = %q, %v, want a *diag.Error", relative, place, err)
	}
	if place != "" || failure.File != relative {
		t.Errorf("Inside(%q) = %q, with the file %q", relative, place, failure.File)
	}
	return failure
}

func TestInsideIsThePlaceOfAPathBelowTheFolder(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "maps/demo.w3x/war3map.lua", nil)
	for relative, want := range map[string]string{
		"maps":                       filepath.Join(root, "maps"),
		"maps/demo.w3x/war3map.lua":  filepath.Join(root, "maps", "demo.w3x", "war3map.lua"),
		"dist/stage/demo.w3x":        filepath.Join(root, "dist", "stage", "demo.w3x"),
		`maps\demo.w3x`:              filepath.Join(root, "maps", "demo.w3x"),
		"maps/demo.w3x/..hidden.txt": filepath.Join(root, "maps", "demo.w3x", "..hidden.txt"),
	} {
		if got, err := fsx.Inside(root, relative); err != nil || got != want {
			t.Errorf("Inside(%q) = %q, %v, want %q", relative, got, err, want)
		}
	}
}

func TestInsideRefusesAPathThatLeavesTheFolder(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	testkit.WriteFile(t, root, "maps/a.txt", nil)
	testkit.WriteFile(t, filepath.Dir(root), "outside/a.txt", nil)
	for _, relative := range []string{
		"../outside", "../outside/a.txt", "maps/../../outside/a.txt", `maps\..\..\outside`, "..", "maps/..", "",
		"/outside", "C:/outside",
	} {
		failure := refusedInside(t, root, relative)
		if !strings.Contains(failure.Msg, "Invalid path: "+relative) ||
			!strings.Contains(failure.Hint, "relative path") || failure.Cause != nil {
			t.Errorf("Inside(%q): %+v", relative, failure)
		}
	}
}

func TestInsideRefusesALinkOnTheWayAndALinkAtTheEnd(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "real/sub/a.txt", nil)
	testkit.LinkDir(t, filepath.Join(root, "real"), filepath.Join(root, "link"))
	testkit.LinkDir(t, filepath.Join(root, "real", "sub"), filepath.Join(root, "real", "inner"))
	cases := []struct {
		relative string
		link     string
	}{
		{"link", "link"},
		{"link/sub/a.txt", "link"},
		{`link\sub`, "link"},
		{"link/nothing/there", "link"},
		{"real/inner", "real/inner"},
		{"real/inner/a.txt", "real/inner"},
	}
	for _, c := range cases {
		failure := refusedInside(t, root, c.relative)
		link := filepath.Join(root, filepath.FromSlash(c.link))
		if failure.Msg != "Symlinks are not supported: "+link || !strings.Contains(failure.Hint, "real files") ||
			failure.Cause != nil {
			t.Errorf("Inside(%q): %+v", c.relative, failure)
		}
	}
	beside := filepath.Join(root, "real", "sub", "a.txt")
	if got, err := fsx.Inside(root, "real/sub/a.txt"); err != nil || got != beside {
		t.Errorf("Inside beside the links = %q, %v", got, err)
	}
}

func TestInsideRefusesALinkToAFileAndALinkToNothing(t *testing.T) {
	root := t.TempDir()
	target := testkit.WriteFile(t, root, "real.txt", nil)
	testkit.LinkFile(t, target, filepath.Join(root, "link.txt"))
	testkit.LinkFile(t, filepath.Join(root, "nothing"), filepath.Join(root, "dangling"))
	for _, relative := range []string{"link.txt", "dangling", "dangling/below"} {
		failure := refusedInside(t, root, relative)
		link := filepath.Join(root, strings.TrimSuffix(relative, "/below"))
		if failure.Msg != "Symlinks are not supported: "+link || !strings.Contains(failure.Hint, "real files") {
			t.Errorf("Inside(%q): %+v", relative, failure)
		}
	}
}

func TestInsideTrustsTheFolderItIsGiven(t *testing.T) {
	base := t.TempDir()
	testkit.WriteFile(t, base, "project/maps/a.txt", nil)
	root := filepath.Join(base, "opened")
	testkit.LinkDir(t, filepath.Join(base, "project"), root)
	if got, err := fsx.Inside(root, "maps/a.txt"); err != nil || got != filepath.Join(root, "maps", "a.txt") {
		t.Errorf("Inside below a folder that is a link = %q, %v", got, err)
	}
}

func TestInsideTakesAFileOnTheWayForNothingThere(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "maps", []byte("a file, not a folder"))
	testkit.WriteFile(t, root, "real/maps", []byte("a file, not a folder"))
	for _, relative := range []string{
		"maps/demo.w3x", "maps/demo.w3x/war3map.lua", "maps/demo.w3x/sub/war3map.lua", "real/maps/demo.w3x",
		"real/maps/demo.w3x/war3map.lua",
	} {
		want := filepath.Join(root, filepath.FromSlash(relative))
		place, err := fsx.Inside(root, relative)
		if err != nil || place != want {
			t.Errorf("Inside(%q) = %q, %v, want %q", relative, place, err, want)
			continue
		}
		if fsx.Exists(place) {
			t.Errorf("Inside(%q): something is at %s", relative, place)
		}
		if err := os.MkdirAll(place, 0o777); err == nil {
			t.Errorf("Inside(%q): a folder was made at %s, below a file", relative, place)
		}
		if err := os.WriteFile(place, []byte("written"), 0o666); err == nil {
			t.Errorf("Inside(%q): a file was written at %s, below a file", relative, place)
		}
	}
	testkit.LinkDir(t, filepath.Join(root, "real"), filepath.Join(root, "link"))
	for _, relative := range []string{"link/maps/demo.w3x", "link/maps/demo.w3x/sub/war3map.lua"} {
		failure := refusedInside(t, root, relative)
		if failure.Msg != "Symlinks are not supported: "+filepath.Join(root, "link") || failure.Cause != nil {
			t.Errorf("Inside(%q): %+v", relative, failure)
		}
	}
	for _, file := range []string{"maps", "real/maps"} {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file)))
		if string(data) != "a file, not a folder" {
			t.Errorf("the file %s holds %q, %v", file, data, err)
		}
	}
}

func TestInsideNamesAWayTheSystemCannotLookAt(t *testing.T) {
	root := t.TempDir()
	testkit.WriteFile(t, root, "maps/a.txt", nil)
	for _, relative := range []string{strings.Repeat("n", 300), "maps/" + strings.Repeat("n", 300) + "/a.txt"} {
		failure := refusedInside(t, root, relative)
		if failure.Cause == nil || failure.Msg != relative+" cannot be reached: "+fsx.Reason(failure.Cause) ||
			!strings.Contains(failure.Hint, "on the way to it") {
			t.Errorf("Inside(%.20q...): %+v", relative, failure)
		}
		if strings.Contains(failure.Msg, root) {
			t.Errorf("Inside(%.20q...) names the folder on disk: %s", relative, failure.Msg)
		}
	}
}
