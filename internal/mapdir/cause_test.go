package mapdir

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const (
	userMistake   = false
	systemFailure = true
)

var errorCauses = []struct {
	name    string
	words   string
	system  bool
	provoke func(t *testing.T) error
}{
	{"a link inside the map", "Symlinks are not supported", userMistake, func(t *testing.T) error {
		dir := writeFiles(t, map[string]string{"Textures/Icon.blp": "icon"})
		outside := filepath.Join(filepath.Dir(dir), "outside")
		testkit.WriteFile(t, outside, "stray.blp", nil)
		testkit.LinkDir(t, outside, filepath.Join(dir, "Textures", "linked"))
		_, err := Open(dir, mapDisplayPath)
		return err
	}},
	{"a map folder that is a link", "Symlinks are not supported", userMistake, func(t *testing.T) error {
		dir := writeFiles(t, sourceMap)
		link := filepath.Join(filepath.Dir(dir), "linked.w3x")
		testkit.LinkDir(t, dir, link)
		_, err := Open(link, mapDisplayPath)
		return err
	}},
	{"a name that cannot be used", "cannot be used in a map", userMistake, func(t *testing.T) error {
		w := walker{displayPath: mapDisplayPath, index: &diskIndex{}}
		return w.addEntry("what?.blp", namedEntry{name: "what?.blp"})
	}},
	{"two spellings of one path", "differ only in letter case", userMistake, func(t *testing.T) error {
		w := walker{displayPath: mapDisplayPath, index: &diskIndex{filePaths: map[string]string{"a.txt": "A.txt"}}}
		return w.addEntry("a.txt", namedEntry{name: "a.txt"})
	}},
	{"an entry that is no regular file", "is not a regular file", userMistake, func(t *testing.T) error {
		if runtime.GOOS == "windows" {
			t.Skip("a socket's file made on Windows could not be removed again, so the test would leave it behind")
		}
		dir := writeFiles(t, sourceMap)
		t.Chdir(dir)
		listener, err := net.Listen("unix", "socket")
		if err != nil {
			t.Skipf("cannot make a socket here: %v", err)
		}
		defer listener.Close()
		_, err = Open(dir, mapDisplayPath)
		return err
	}},
	{"a file that is no folder", "is not a folder", userMistake, func(t *testing.T) error {
		_, err := Open(testkit.WriteFile(t, t.TempDir(), "map.w3x", []byte("an archive")), mapDisplayPath)
		return err
	}},

	{"a new file below a file of the map", "is a file, not a folder", userMistake, func(t *testing.T) error {
		folder, _ := openFolder(t, sourceMap)
		_, err := folder.ResolveNewPath("war3map.w3i/x.txt")
		return err
	}},
	{"a new file named as a folder of the map", "would replace a folder", userMistake, func(t *testing.T) error {
		folder, _ := openFolder(t, sourceMap)
		_, err := folder.ResolveNewPath("textures")
		return err
	}},
	{"a stage that is the source map", "would replace the source map", userMistake, func(t *testing.T) error {
		folder, dir := openFolder(t, sourceMap)
		return folder.StageTo(dir)
	}},
	{"a file that changed after it was read", "changed after", userMistake, func(t *testing.T) error {
		folder, dir := openFolder(t, sourceMap)
		readFile(t, folder, "war3map.w3i")
		testkit.WriteFile(t, dir, "war3map.w3i", []byte("edited elsewhere"))
		return folder.WithChanges([]Change{newWrite("war3map.w3i", "patched")}).ApplyInPlace(context.Background(), &fsx.Journal{})
	}},

	{"a folder that cannot be listed", "Reading the map folder failed", systemFailure, func(t *testing.T) error {
		w := walker{dir: writeFiles(t, sourceMap), displayPath: mapDisplayPath, index: &diskIndex{}}
		return w.walk("Gone")
	}},
	{"a file that cannot be read", "Reading a map file failed", systemFailure, func(t *testing.T) error {
		folder, dir := openFolder(t, sourceMap)
		file := filepath.Join(dir, "war3map.w3i")
		if err := os.Remove(file); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(file, 0o777); err != nil {
			t.Fatal(err)
		}
		_, _, err := folder.Read("war3map.w3i")
		return err
	}},
	{"a read through a link made after the scan", "Reading a map file failed: Symlinks", systemFailure, func(t *testing.T) error {
		folder, dir := openFolder(t, sourceMap)
		symlinkTexturesOutside(t, dir)
		_, _, err := folder.Read("Textures/Old.blp")
		return err
	}},
	{"a stage that cannot be made", "Staging the map failed", systemFailure, func(t *testing.T) error {
		folder, _ := openFolder(t, sourceMap)
		blocked := testkit.WriteFile(t, t.TempDir(), "dist", nil)
		return folder.StageTo(filepath.Join(blocked, "map.w3x"))
	}},
	{"a file that cannot be written", "Writing a map file failed", systemFailure, func(t *testing.T) error {
		folder, dir := openFolder(t, sourceMap)
		view := folder.WithChanges([]Change{newWrite("Sound/theme.mp3", "theme")})
		testkit.WriteFile(t, dir, "Sound", []byte("in the way"))
		return view.ApplyInPlace(context.Background(), &fsx.Journal{})
	}},
	{"a write through a link made after the scan", "Writing a map file failed: Symlinks", systemFailure, func(t *testing.T) error {
		folder, dir := openFolder(t, sourceMap)
		view := folder.WithChanges([]Change{newWrite("Textures/New.blp", "new")})
		symlinkTexturesOutside(t, dir)
		return view.ApplyInPlace(context.Background(), &fsx.Journal{})
	}},
}

func TestAnErrorHasACauseExactlyWhenTheSystemFailed(t *testing.T) {
	for _, c := range errorCauses {
		t.Run(c.name, func(t *testing.T) {
			e := asDiagError(t, c.provoke(t))
			if !contains(e.Msg, c.words) || e.File == "" || e.Hint == "" {
				t.Fatalf("error = %+v, want the failure that says %q, with a file and a hint", e, c.words)
			}
			if hasCause := e.Cause != nil; hasCause != c.system {
				t.Errorf("the cause is %v, want one exactly when the system failed (%v): %+v", e.Cause, c.system, e)
			}
		})
	}
}
