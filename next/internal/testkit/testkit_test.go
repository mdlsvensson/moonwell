package testkit

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/env"
)

// standIn is a testing.TB that records what Fatalf was told instead of stopping the test, so a test can watch a helper
// fail. Only the methods the helpers call are replaced.
type standIn struct {
	testing.TB
	real   *testing.T
	failed []string
}

func newStandIn(t *testing.T) *standIn { return &standIn{real: t} }

func (s *standIn) Helper()         {}
func (s *standIn) TempDir() string { return s.real.TempDir() }
func (s *standIn) Cleanup(f func()) {
	s.real.Cleanup(f)
}
func (s *standIn) Fatalf(format string, args ...any) {
	s.failed = append(s.failed, fmt.Sprintf(format, args...))
}

func TestFixtureReturnsTheBytesOfAFileUnderTestdata(t *testing.T) {
	data := Fixture(t, "map-settings-v39/war3map.w3i")
	if want := []byte{0x27, 0, 0, 0}; len(data) < 4 || !bytes.Equal(data[:4], want) {
		t.Errorf("the file starts with % x, want % x (version 39)", data[:min(4, len(data))], want)
	}
}

func TestFixtureFailsTheTestWhenTheFileIsMissing(t *testing.T) {
	stand := newStandIn(t)
	Fixture(stand, "no-such-folder/no-such-file")
	if len(stand.failed) != 1 || !strings.Contains(stand.failed[0], "no-such-folder/no-such-file") {
		t.Errorf("failures = %q, want one that names the fixture", stand.failed)
	}
}

func TestEnvFailsTheTestWhenRunFetchOrSpawnIsCalled(t *testing.T) {
	tests := []struct {
		name string
		call func(world *env.Env) error
		want string
	}{
		{"Run", func(w *env.Env) error {
			_, err := w.Run(context.Background(), "yue", []string{"-v"}, env.RunOptions{})
			return err
		}, "yue"},
		{"Fetch", func(w *env.Env) error {
			_, _, err := w.Fetch(context.Background(), "https://example.test/pkl")
			return err
		}, "https://example.test/pkl"},
		{"Spawn", func(w *env.Env) error {
			return w.Spawn("Warcraft III.exe", []string{"-window"})
		}, "Warcraft III.exe"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stand := newStandIn(t)
			world, _ := Env(stand, t.TempDir())
			err := tc.call(world)
			if len(stand.failed) != 1 || !strings.Contains(stand.failed[0], tc.want) {
				t.Errorf("failures = %q, want one that names %q", stand.failed, tc.want)
			}
			if err == nil {
				t.Error("the call returned no error after failing the test")
			}
		})
	}
}

func TestEnvIsATestWorld(t *testing.T) {
	root := t.TempDir()
	world, recorder := Env(t, root)
	if world.Root != root {
		t.Errorf("Root = %q, want %q", world.Root, root)
	}
	if world.Log != recorder.Logger {
		t.Error("Log is not the recorder's logger")
	}
	if world.CacheDir == "" || world.CacheDir == root {
		t.Errorf("CacheDir = %q, want a folder of its own", world.CacheDir)
	}
	if info, err := os.Stat(world.CacheDir); err != nil || !info.IsDir() {
		t.Errorf("CacheDir %q is not a folder: %v", world.CacheDir, err)
	}
	if world.Platform != env.CurrentPlatform() {
		t.Errorf("Platform = %q, want %q", world.Platform, env.CurrentPlatform())
	}
}

func TestRecorderKeepsEveryLevelInOrder(t *testing.T) {
	recorder := NewRecorder()
	recorder.Info("one")
	recorder.Warn("two")
	recorder.Error("three")
	want := []string{"one", "warning: two", "three"}
	if !reflect.DeepEqual(recorder.Lines, want) {
		t.Errorf("Lines = %q, want %q", recorder.Lines, want)
	}
}

func TestWriteFileCreatesTheFolders(t *testing.T) {
	dir := t.TempDir()
	path := WriteFile(t, dir, "a/b/c.txt", []byte("hi"))
	if want := filepath.Join(dir, "a", "b", "c.txt"); path != want {
		t.Errorf("path = %q, want %q", path, want)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != "hi" {
		t.Errorf("file = %q, %v", data, err)
	}
}

func TestSnapshotListsAFileAndItsFolders(t *testing.T) {
	dir := t.TempDir()
	WriteFile(t, dir, "a/b.txt", []byte("x"))
	WriteFile(t, dir, "empty.txt", nil)
	got := Snapshot(t, dir)
	want := map[string][]byte{"a": nil, "a/b.txt": []byte("x"), "empty.txt": {}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Snapshot = %v, want %v", got, want)
	}
}

func TestByteHelpersAreLittleEndian(t *testing.T) {
	if got := U32(0x01020304); !bytes.Equal(got, []byte{4, 3, 2, 1}) {
		t.Errorf("U32 = % x", got)
	}
	if got := F32(1); !bytes.Equal(got, []byte{0, 0, 0x80, 0x3f}) {
		t.Errorf("F32 = % x", got)
	}
	if got := Concat([]byte{1}, nil, []byte{2, 3}); !bytes.Equal(got, []byte{1, 2, 3}) {
		t.Errorf("Concat = % x", got)
	}
	if got := Fixed("ab", 4); !bytes.Equal(got, []byte{'a', 'b', 0, 0}) {
		t.Errorf("Fixed = % x", got)
	}
	original := []byte{0, 0, 0, 0, 9}
	if got := SetU32(original, 0, 1); !bytes.Equal(got, []byte{1, 0, 0, 0, 9}) || original[0] != 0 {
		t.Errorf("SetU32 = % x, original % x", got, original)
	}
}

func TestRepoRootIsTheFolderWithGoMod(t *testing.T) {
	if _, err := os.Stat(filepath.Join(RepoRoot(t), "go.mod")); err != nil {
		t.Error(err)
	}
}

func TestCaseSensitiveLeavesNoProbeBehind(t *testing.T) {
	dir := t.TempDir()
	CaseSensitive(t, dir)
	if got := Snapshot(t, dir); len(got) != 0 {
		t.Errorf("left %v behind", got)
	}
}

func TestLinkDirLinksToTheFolder(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	WriteFile(t, target, "file.txt", []byte("x"))
	link := filepath.Join(dir, "link")
	LinkDir(t, target, link)
	if data, err := os.ReadFile(filepath.Join(link, "file.txt")); err != nil || string(data) != "x" {
		t.Errorf("through the link: %q, %v", data, err)
	}
}
