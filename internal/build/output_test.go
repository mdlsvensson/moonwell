package build

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestOutputAtIsThePlaceBelowTheProjectFolderWhateverIsThere(t *testing.T) {
	root := t.TempDir()
	tests := []struct{ relative, want string }{
		{"dist", "dist"},
		{"dist/.lock", "dist/.lock"},
		{"dist/stage/campaign/one.w3x", "dist/stage/campaign/one.w3x"},
		{`dist\stage\map.w3x`, "dist/stage/map.w3x"},
		{"out/map.w3x", "out/map.w3x"},
	}
	for _, tt := range tests {
		place, err := outputPath(root, tt.relative)
		if want := filepath.Join(root, filepath.FromSlash(tt.want)); err != nil || place != want {
			t.Errorf("outputAt(%q) = %q, %v, want %q", tt.relative, place, err, want)
		}
	}
	if held := testkit.Snapshot(t, root); len(held) != 0 {
		t.Errorf("asking for a place made %q", held)
	}
}

func TestOutputAtRefusesALinkOnTheWayOrAtThePlaceByItsStep(t *testing.T) {
	tests := []struct {
		name    string
		symlink string
		place   string
	}{
		{"the first folder", "dist", "dist"},
		{"the first folder on the way", "dist", "dist/stage/map.w3x"},
		{"the first folder of an archive's place", "out", "out/map.w3x"},
		{"a folder on the way", "dist/stage", "dist/stage/map.w3x"},
		{"the place itself", "dist/stage/map.w3x", "dist/stage/map.w3x"},
		{"a place written with the other separator", "dist/stage", `dist\stage\map.w3x`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root, elsewhere := t.TempDir(), t.TempDir()
			at := filepath.Join(root, filepath.FromSlash(tt.symlink))
			if err := os.MkdirAll(filepath.Dir(at), 0o777); err != nil {
				t.Fatal(err)
			}
			testkit.LinkDir(t, elsewhere, at)
			place, err := outputPath(root, tt.place)
			e := asDiagError(t, err, tt.name)
			if place != "" || e.File != tt.place || !strings.HasPrefix(e.Msg, tt.symlink+" is a link: ") ||
				!strings.Contains(e.Hint, "junction) at "+tt.symlink+",") || e.Cause != nil ||
				strings.Contains(e.Msg, root) {
				t.Errorf("error = %+v", e)
			}
			if held := testkit.Snapshot(t, elsewhere); len(held) != 0 {
				t.Errorf("the folder the link leads to holds %q", held)
			}
		})
	}
}

func TestOutputAtPassesOnTheRefusalOfAPathThatLeavesTheProjectOrThatWindowsCannotHold(t *testing.T) {
	root := t.TempDir()
	for _, relative := range []string{
		"", ".", "..", "../dist", "dist/..", "dist/../maps", "dist/stage/../../maps", "dist//stage", "dist/", "/dist",
		`C:\dist`, "dist/con", "dist/stage/map?.w3x", "nul/x", `dist\..\maps`,
	} {
		place, err := outputPath(root, relative)
		e := asDiagError(t, err, "the place "+relative)
		if place != "" || e.File != relative || !strings.Contains(e.Msg, "Invalid path: "+relative) || e.Cause != nil {
			t.Errorf("outputAt(%q): error = %+v", relative, e)
		}
	}
}

func TestOutputAtNamesAPlaceTheSystemCannotLookAtByTheWholePath(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "dist"), 0o777); err != nil {
		t.Fatal(err)
	}
	relative := "dist/" + strings.Repeat("a", 300) + "/x"
	place, err := outputPath(root, relative)
	e := asDiagError(t, err, "a name the system cannot hold")
	if place != "" || e.File != relative || e.Cause == nil || strings.Contains(e.Msg, root) ||
		!strings.HasPrefix(e.Msg, relative+" cannot be reached") {
		t.Errorf("error = %+v", e)
	}
}
