package mapdir

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// snapshot is every entry below dir as text, a folder as "<folder>".
func snapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	entries := map[string]string{}
	for name, data := range testkit.Snapshot(t, dir) {
		entries[name] = "<folder>"
		if data != nil {
			entries[name] = string(data)
		}
	}
	return entries
}

// filesOf is a snapshot without its folders.
func filesOf(entries map[string]string) map[string]string {
	files := map[string]string{}
	for name, content := range entries {
		if content != "<folder>" {
			files[name] = content
		}
	}
	return files
}

var sourceMap = map[string]string{
	"war3map.w3i":      "info",
	"war3mapMap.blp":   "minimap",
	"war3mapskin.txt":  "skin",
	"Textures/Old.blp": "old",
}

func TestStageToWritesTheChangesIntoACopy(t *testing.T) {
	folder, dir := open(t, sourceMap)
	view := folder.With([]Change{
		put("war3map.w3i", "patched"),
		put("war3mapMinimap.blp", "minimap"),
		drop("war3mapmap.blp"),
		put("war3mapSkin.txt", "merged"),
		put("Sound/Music/theme.mp3", "theme"),
	})
	source := snapshot(t, dir)
	// The stage is below folders that do not exist the first time, and holds an older copy the second time.
	stage := filepath.Join(t.TempDir(), "dist", "stage", "map.w3x")
	want := map[string]string{
		"war3map.w3i":           "patched",
		"war3mapMinimap.blp":    "minimap",
		"war3mapskin.txt":       "merged",
		"Textures":              "<folder>",
		"Textures/Old.blp":      "old",
		"Sound":                 "<folder>",
		"Sound/Music":           "<folder>",
		"Sound/Music/theme.mp3": "theme",
	}
	for _, round := range []string{"into a new folder", "over an older stage"} {
		if err := view.StageTo(stage); err != nil {
			t.Fatalf("%s: %v", round, err)
		}
		if got := snapshot(t, stage); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the stage holds %v, want %v", round, got, want)
		}
		if got := snapshot(t, dir); !reflect.DeepEqual(got, source) {
			t.Errorf("%s: staging changed the source map: %v", round, got)
		}
		testkit.WriteFile(t, stage, "stale/left.txt", []byte("from an earlier build"))
		testkit.WriteFile(t, stage, "war3map.w3i", []byte("from an earlier build"))
	}
}

func TestStageToWithoutChangesIsACopy(t *testing.T) {
	folder, dir := open(t, sourceMap)
	stage := filepath.Join(t.TempDir(), "map.w3x")
	if err := folder.StageTo(stage); err != nil {
		t.Fatal(err)
	}
	if got, want := snapshot(t, stage), snapshot(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("the stage holds %v, want %v", got, want)
	}
}

func TestStageToRemovesAFileThatIsAlreadyGone(t *testing.T) {
	folder, dir := open(t, sourceMap)
	view := folder.With([]Change{drop("war3mapMap.blp")})
	if err := os.Remove(filepath.Join(dir, "war3mapMap.blp")); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(t.TempDir(), "map.w3x")
	if err := view.StageTo(stage); err != nil {
		t.Errorf("removing a file the copy does not have failed: %v", err)
	}
}

func TestStageToNamesTheFileItCouldNotWrite(t *testing.T) {
	const hint = "Close Warcraft III or World Editor if they have dist/stage open, then retry."
	cases := []struct {
		name, file string
	}{
		{"a folder where the file goes", "Textures"},
		{"a path that leaves the stage", "../outside.txt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			folder, dir := open(t, sourceMap)
			source := snapshot(t, dir)
			parent := t.TempDir()
			stage := filepath.Join(parent, "stage", "map.w3x")
			err := folder.With([]Change{put(c.file, "data")}).StageTo(stage)
			e := asError(t, err)
			if !contains(e.Msg, "Staging the map failed") || e.Cause == nil || e.Hint != hint ||
				e.File != filepath.Join(stage, filepath.FromSlash(c.file)) {
				t.Errorf("error = %+v", e)
			}
			if fsx.Exists(filepath.Join(parent, "stage", "outside.txt")) {
				t.Error("a file was written outside the stage")
			}
			if got := snapshot(t, dir); !reflect.DeepEqual(got, source) {
				t.Errorf("staging changed the source map: %v", got)
			}
		})
	}
}

func TestStageToNamesTheStageItCouldNotReplace(t *testing.T) {
	folder, _ := open(t, sourceMap)
	// A file where the folder the stage goes into should be.
	blocked := filepath.Join(t.TempDir(), "dist")
	if err := os.WriteFile(blocked, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(blocked, "map.w3x")
	e := asError(t, folder.StageTo(stage))
	if !contains(e.Msg, "Staging the map failed") || e.File != stage || e.Cause == nil || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
}

func TestApplyInPlaceWritesThroughTheJournalWhichCanUndoIt(t *testing.T) {
	folder, dir := open(t, sourceMap)
	before := snapshot(t, dir)
	// A planner reads what it changes.
	read(t, folder, "war3map.w3i")
	read(t, folder, "war3mapMap.blp")
	view := folder.With([]Change{
		put("WAR3MAP.W3I", "patched"),
		drop("war3mapmap.blp"),
		put("Sound/Music/theme.mp3", "theme"),
		put("Textures/New.blp", "new"),
	})
	var journal fsx.Journal
	if err := view.ApplyInPlace(context.Background(), &journal); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"war3map.w3i":           "patched",
		"war3mapskin.txt":       "skin",
		"Textures":              "<folder>",
		"Textures/Old.blp":      "old",
		"Textures/New.blp":      "new",
		"Sound":                 "<folder>",
		"Sound/Music":           "<folder>",
		"Sound/Music/theme.mp3": "theme",
	}
	if got := snapshot(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("the map holds %v, want %v", got, want)
	}
	if journal.Len() != 4 {
		t.Errorf("the journal has touched %d files, want 4: nothing may be written past it", journal.Len())
	}
	if unrestored := journal.Undo(); len(unrestored) != 0 {
		t.Errorf("unrestored = %q", unrestored)
	}
	// The journal puts files back; the folders made for them stay.
	if got := filesOf(snapshot(t, dir)); !reflect.DeepEqual(got, filesOf(before)) {
		t.Errorf("after the undo the map holds %v, want %v", got, filesOf(before))
	}
}

func TestApplyInPlaceRefusesAFileThatIsNotAsTheFolderSawIt(t *testing.T) {
	changes := []Change{put("a.txt", "1"), put("b.txt", "2"), put("new.txt", "3"), put("c.txt", "4")}
	edit := func(name string) func(t *testing.T, dir string) {
		return func(t *testing.T, dir string) { testkit.WriteFile(t, dir, name, []byte("edited elsewhere")) }
	}
	remove := func(name string) func(t *testing.T, dir string) {
		return func(t *testing.T, dir string) {
			if err := os.Remove(filepath.Join(dir, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	readIn := func(view func(*Folder) *Folder) func(t *testing.T, folder *Folder) {
		return func(t *testing.T, folder *Folder) { read(t, view(folder), "B.TXT") }
	}
	itself := func(folder *Folder) *Folder { return folder }
	aView := func(folder *Folder) *Folder { return folder.With([]Change{put("other.txt", "")}) }
	cases := []struct {
		name    string
		read    func(t *testing.T, folder *Folder)
		meddle  func(t *testing.T, dir string)
		refused string // the file the error is at; "" when everything is written
	}{
		{"a file that was read, then edited", readIn(itself), edit("b.txt"), "b.txt"},
		{"a file a view read, then edited", readIn(aView), edit("b.txt"), "b.txt"},
		{"a file that was read, then removed", readIn(itself), remove("b.txt"), "b.txt"},
		{"a file that was not read, then removed", nil, remove("b.txt"), "b.txt"},
		{"a file where none was scanned", nil, edit("new.txt"), "new.txt"},
		{"a file that was not read, then edited", nil, edit("b.txt"), ""},
		{"a file that was read and is the same", readIn(itself), nil, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			folder, dir := open(t, map[string]string{"a.txt": "a", "b.txt": "b", "c.txt": "c"})
			if c.read != nil {
				c.read(t, folder)
			}
			view := folder.With(changes)
			if c.meddle != nil {
				c.meddle(t, dir)
			}
			var journal fsx.Journal
			err := view.ApplyInPlace(context.Background(), &journal)
			if c.refused == "" {
				if err != nil || journal.Len() != len(changes) {
					t.Errorf("ApplyInPlace = %v after %d writes, want all %d", err, journal.Len(), len(changes))
				}
				return
			}
			e := asError(t, err)
			if !contains(e.Msg, label+"/"+c.refused+" changed after") || e.File != label+"/"+c.refused ||
				!contains(e.Hint, "Close World Editor") {
				t.Errorf("error = %+v", e)
			}
			// What was written before the refused file stays, for the caller's journal to undo; nothing follows it.
			after := filesOf(snapshot(t, dir))
			if after["a.txt"] != "1" || after["c.txt"] != "c" {
				t.Errorf("the map holds %v, want a.txt written and c.txt untouched", after)
			}
			earlier := slices.IndexFunc(changes, func(change Change) bool { return change.Name == c.refused })
			if journal.Len() != earlier {
				t.Errorf("the journal has touched %d files, want the %d before the refused one", journal.Len(), earlier)
			}
		})
	}
}

func TestApplyInPlaceStopsAtACancelledContext(t *testing.T) {
	folder, dir := open(t, sourceMap)
	before := snapshot(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var journal fsx.Journal
	err := folder.With([]Change{put("war3map.w3i", "patched")}).ApplyInPlace(ctx, &journal)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ApplyInPlace = %v, want the context's error", err)
	}
	if got := snapshot(t, dir); journal.Len() != 0 || !reflect.DeepEqual(got, before) {
		t.Errorf("a cancelled apply touched %d files; the map holds %v", journal.Len(), got)
	}
}

func TestApplyInPlaceNamesTheFileItCouldNotWrite(t *testing.T) {
	cases := []struct {
		name, file string
	}{
		{"a file where a folder of the new file goes", "war3map.w3i/x.txt"},
		{"a path that leaves the map", "../outside.txt"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			folder, dir := open(t, sourceMap)
			before := snapshot(t, filepath.Dir(dir))
			var journal fsx.Journal
			changes := []Change{put(c.file, "data"), put("war3mapskin.txt", "never written")}
			err := folder.With(changes).ApplyInPlace(context.Background(), &journal)
			e := asError(t, err)
			if !contains(e.Msg, "Writing a map file failed: ") || e.File != label+"/"+c.file || e.Cause == nil {
				t.Errorf("error = %+v", e)
			}
			if got := snapshot(t, filepath.Dir(dir)); !reflect.DeepEqual(got, before) {
				t.Errorf("a failed write left %v, want %v", got, before)
			}
		})
	}
}
