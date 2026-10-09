package mapdir

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

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

func withoutDirs(entries map[string]string) map[string]string {
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
	folder, dir := openFolder(t, sourceMap)
	view := folder.WithChanges([]Change{
		newWrite("war3map.w3i", "patched"),
		newWrite("war3mapMinimap.blp", "minimap"),
		newRemoval("war3mapmap.blp"),
		newWrite("war3mapSkin.txt", "merged"),
		newWrite("Sound/Music/theme.mp3", "theme"),
		newWrite("sound/effects/hit.wav", "hit"),
		newWrite("textures/New.blp", "new"),
	})
	source := snapshot(t, dir)
	stage := filepath.Join(t.TempDir(), "dist", "stage", "map.w3x")
	want := map[string]string{
		"war3map.w3i":           "patched",
		"war3mapMinimap.blp":    "minimap",
		"war3mapskin.txt":       "merged",
		"Textures":              "<folder>",
		"Textures/Old.blp":      "old",
		"Textures/New.blp":      "new",
		"Sound":                 "<folder>",
		"Sound/Music":           "<folder>",
		"Sound/Music/theme.mp3": "theme",
		"Sound/effects":         "<folder>",
		"Sound/effects/hit.wav": "hit",
	}
	for _, round := range []string{"into a new folder", "over an older stage"} {
		if err := view.StageTo(stage); err != nil {
			t.Fatalf("%s: %v", round, err)
		}
		if got := snapshot(t, stage); !reflect.DeepEqual(got, want) {
			t.Errorf("%s: the stage holds %v, want %v", round, got, want)
		}
		if _, err := Open(stage, mapDisplayPath); err != nil {
			t.Errorf("%s: the staged map cannot be opened: %v", round, err)
		}
		if got := snapshot(t, dir); !reflect.DeepEqual(got, source) {
			t.Errorf("%s: staging changed the source map: %v", round, got)
		}
		testkit.WriteFile(t, stage, "stale/left.txt", []byte("from an earlier build"))
		testkit.WriteFile(t, stage, "war3map.w3i", []byte("from an earlier build"))
	}
}

func TestStageToWithoutChangesIsACopy(t *testing.T) {
	folder, dir := openFolder(t, sourceMap)
	stage := filepath.Join(t.TempDir(), "map.w3x")
	if err := folder.StageTo(stage); err != nil {
		t.Fatal(err)
	}
	if got, want := snapshot(t, stage), snapshot(t, dir); !reflect.DeepEqual(got, want) {
		t.Errorf("the stage holds %v, want %v", got, want)
	}
}

func TestStageToRemovesAFileThatIsAlreadyGone(t *testing.T) {
	folder, dir := openFolder(t, sourceMap)
	view := folder.WithChanges([]Change{newRemoval("war3mapMap.blp")})
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
	folder, dir := openFolder(t, sourceMap)
	view := folder.WithChanges([]Change{newWrite("new.txt", "data")})
	if err := os.Mkdir(filepath.Join(dir, "new.txt"), 0o777); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(t.TempDir(), "stage", "map.w3x")
	e := asDiagError(t, view.StageTo(stage))
	if !contains(e.Msg, "Staging the map failed") || e.Cause == nil || e.Hint != hint ||
		e.File != filepath.Join(stage, "new.txt") {
		t.Errorf("error = %+v", e)
	}
}

func withBaseChanges(changes ...Change) []Change {
	return append([]Change{newWrite("war3mapskin.txt", "merged"), newWrite("new.txt", "new")}, changes...)
}

var callerMistakes = []struct {
	name    string
	changes []Change
	words   []string
}{
	{"a path that leaves the folder", withBaseChanges(newWrite("../outside.txt", "data")),
		[]string{`Cannot write "../outside.txt"`, "relative path"}},
	{"a leading slash before a file the map has", withBaseChanges(newWrite("/WAR3MAP.W3I", "data")),
		[]string{`Cannot write "/WAR3MAP.W3I"`, "relative path"}},
	{"an empty folder name", withBaseChanges(newWrite("a//b", "data")), []string{`Cannot write "a//b"`, "relative path"}},
	{"a way out of a folder", withBaseChanges(newWrite("textures/../war3map.w3i", "data")),
		[]string{`Cannot write "Textures/../war3map.w3i"`, "relative path"}},
	{"a character Windows forbids", withBaseChanges(newWrite("what?.blp", "data")),
		[]string{`Cannot write "what?.blp"`, "relative path"}},
	{"a device name below a new folder", withBaseChanges(newWrite("Sound/nul.mp3", "data")),
		[]string{`Cannot write "Sound/nul.mp3"`, "relative path"}},
	{"a change named as a folder of the map", withBaseChanges(newWrite("textures", "data")),
		[]string{"Cannot write textures", "the folder Textures of the map"}},
	{"a change through a file of the map", withBaseChanges(newWrite("WAR3MAP.W3I/x.txt", "data")),
		[]string{"Cannot write WAR3MAP.W3I/x.txt", "goes through war3map.w3i, a file of the map"}},
	{"a change through a file below a folder", withBaseChanges(newWrite("textures/old.blp/deep/x.txt", "data")),
		[]string{"Cannot write Textures/old.blp/deep/x.txt", "goes through Textures/Old.blp, a file of the map"}},
	{"a change through a file that the plan removes first",
		withBaseChanges(newRemoval("WAR3MAP.W3I"), newWrite("war3map.w3i/x.txt", "data")),
		[]string{"Cannot write war3map.w3i/x.txt", "goes through war3map.w3i, a file of the map"}},
	{"a change through a file that the plan removes after it",
		withBaseChanges(newWrite("war3map.w3i/x.txt", "data"), newRemoval("WAR3MAP.W3I")),
		[]string{"Cannot write war3map.w3i/x.txt", "goes through war3map.w3i, a file of the map"}},
	{"a new file named as the folder a later change makes",
		withBaseChanges(newWrite("Sound", "data"), newWrite("sound/theme.mp3", "theme")),
		[]string{"Cannot write Sound", "the folder sound of the map"}},
	{"a new file below the name a later change writes as a file",
		withBaseChanges(newWrite("Sound/Music/theme.mp3", "theme"), newWrite("sound", "data")),
		[]string{"Cannot write Sound/Music/theme.mp3", "goes through sound, a file of the map"}},
}

func mustBePlainError(t *testing.T, err error) string {
	t.Helper()
	if err == nil {
		t.Fatal("got no error")
	}
	var expected *diag.Error
	if errors.As(err, &expected) {
		t.Fatalf("got the *diag.Error %+v, want a plain error", expected)
	}
	return err.Error()
}

func TestStageToRefusesAPlannersBugBeforeItWrites(t *testing.T) {
	for _, c := range callerMistakes {
		t.Run(c.name, func(t *testing.T) {
			folder, dir := openFolder(t, sourceMap)
			source := snapshot(t, dir)
			around := t.TempDir()
			stage := filepath.Join(around, "stage", "map.w3x")
			testkit.WriteFile(t, stage, "left.txt", []byte("from an earlier build"))
			earlier := snapshot(t, around)
			text := mustBePlainError(t, folder.WithChanges(c.changes).StageTo(stage))
			for _, words := range c.words {
				if !contains(text, words) {
					t.Errorf("error = %q, want it to say %q", text, words)
				}
			}
			if got := snapshot(t, around); !reflect.DeepEqual(got, earlier) {
				t.Errorf("the stage and the folders around it hold %v, want them untouched: %v", got, earlier)
			}
			if got := snapshot(t, dir); !reflect.DeepEqual(got, source) {
				t.Errorf("staging changed the source map: %v", got)
			}
		})
	}
}

func TestApplyInPlaceRefusesAPlannersBugBeforeItWrites(t *testing.T) {
	for _, c := range callerMistakes {
		t.Run(c.name, func(t *testing.T) {
			folder, dir := openFolder(t, sourceMap)
			before := snapshot(t, filepath.Dir(dir))
			var journal fsx.Journal
			text := mustBePlainError(t, folder.WithChanges(c.changes).ApplyInPlace(context.Background(), &journal))
			for _, words := range c.words {
				if !contains(text, words) {
					t.Errorf("error = %q, want it to say %q", text, words)
				}
			}
			if got := snapshot(t, filepath.Dir(dir)); journal.Len() != 0 || !reflect.DeepEqual(got, before) {
				t.Errorf("the journal touched %d files; the map holds %v, want %v", journal.Len(), got, before)
			}
		})
	}
}

func TestStageToNamesTheStageItCouldNotReplace(t *testing.T) {
	folder, _ := openFolder(t, sourceMap)
	blocked := filepath.Join(t.TempDir(), "dist")
	if err := os.WriteFile(blocked, nil, 0o666); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(blocked, "map.w3x")
	e := asDiagError(t, folder.StageTo(stage))
	if !contains(e.Msg, "Staging the map failed") || e.File != stage || e.Cause == nil || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
}

func TestStageToRefusesAStageThatOverlapsTheSourceMap(t *testing.T) {
	cases := []struct {
		name  string
		stage func(dir string) string
	}{
		{"the source map itself", func(dir string) string { return dir }},
		{"a folder the source map is in", func(dir string) string { return filepath.Dir(filepath.Dir(dir)) }},
		{"a folder inside the source map", func(dir string) string { return filepath.Join(dir, "dist", "map.w3x") }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			project := t.TempDir()
			dir := filepath.Join(project, "maps", "map.w3x")
			testkit.WriteFile(t, project, "moonwell.pkl", []byte("manifest"))
			for name, content := range sourceMap {
				testkit.WriteFile(t, dir, name, []byte(content))
			}
			folder, err := Open(dir, mapDisplayPath)
			if err != nil {
				t.Fatal(err)
			}
			before := testkit.Snapshot(t, project)
			err = folder.WithChanges([]Change{newWrite("war3map.w3i", "patched"), newWrite("new.txt", "new")}).StageTo(c.stage(dir))
			e := asDiagError(t, err)
			if !contains(e.Msg, "source map") || e.File != mapDisplayPath || e.Hint == "" {
				t.Errorf("error = %+v", e)
			}
			if after := testkit.Snapshot(t, project); !reflect.DeepEqual(after, before) {
				t.Errorf("the project holds %v, want it untouched: %v", after, before)
			}
		})
	}
}

func TestStageToDoesNotWriteThroughALinkMadeAfterTheScan(t *testing.T) {
	folder, dir := openFolder(t, sourceMap)
	view := folder.WithChanges([]Change{newWrite("textures/old.blp", "patched"), newWrite("Textures/New.blp", "new")})
	outside := symlinkTexturesOutside(t, dir)
	before := snapshot(t, outside)
	stage := filepath.Join(t.TempDir(), "map.w3x")
	if e := asDiagError(t, view.StageTo(stage)); !contains(e.Msg, "Staging the map failed") {
		t.Errorf("error = %+v", e)
	}
	if after := snapshot(t, outside); !reflect.DeepEqual(after, before) {
		t.Errorf("the folder the link points at holds %v, want %v", after, before)
	}
}

func TestApplyInPlaceDoesNotWriteThroughALinkMadeAfterTheScan(t *testing.T) {
	cases := []struct {
		name   string
		change Change
		at     string
	}{
		{"a file the map had", newWrite("textures/old.blp", "patched"), "Textures/Old.blp"},
		{"a file to remove", newRemoval("Textures/Old.blp"), "Textures/Old.blp"},
		{"a new file", newWrite("textures/New.blp", "new"), "Textures/New.blp"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			folder, dir := openFolder(t, sourceMap)
			view := folder.WithChanges([]Change{c.change})
			outside := symlinkTexturesOutside(t, dir)
			before := snapshot(t, outside)
			var journal fsx.Journal
			e := asDiagError(t, view.ApplyInPlace(context.Background(), &journal))
			if !contains(e.Msg, "Writing a map file failed") || !contains(e.Msg, "Symlinks") || e.File != mapDisplayPath+"/"+c.at {
				t.Errorf("error = %+v", e)
			}
			if after := snapshot(t, outside); journal.Len() != 0 || !reflect.DeepEqual(after, before) {
				t.Errorf("the journal touched %d files; the folder the link points at holds %v", journal.Len(), after)
			}
		})
	}
}

func TestStageDoesNotWriteAChangeThroughALinkInTheCopy(t *testing.T) {
	_, dir := openFolder(t, sourceMap)
	outside := symlinkTexturesOutside(t, dir)
	before := snapshot(t, outside)
	for _, change := range []Change{newWrite("Textures/Old.blp", "patched"), newWrite("Textures/New.blp", "new"), newRemoval("Textures/Old.blp")} {
		err := writeChange(dir, change)
		if e := asDiagError(t, err); !contains(e.Msg, "Symlinks are not supported") || !contains(e.Msg, filepath.Join(dir, "Textures")) {
			t.Errorf("staging %s: error = %+v, want the link refused by its path", formatChanges([]Change{change}), e)
		}
	}
	if after := snapshot(t, outside); !reflect.DeepEqual(after, before) {
		t.Errorf("the folder the link points at holds %v, want %v", after, before)
	}
}

func TestApplyInPlaceStopsAtAFileItCannotWrite(t *testing.T) {
	folder, dir := openFolder(t, sourceMap)
	view := folder.WithChanges([]Change{newWrite("war3mapskin.txt", "merged"), newWrite("Sound/theme.mp3", "theme"), newWrite("new.txt", "new")})
	testkit.WriteFile(t, dir, "Sound", []byte("in the way"))
	var journal fsx.Journal
	e := asDiagError(t, view.ApplyInPlace(context.Background(), &journal))
	if !contains(e.Msg, "Writing a map file failed") || e.File != mapDisplayPath+"/Sound/theme.mp3" || e.Cause == nil ||
		!contains(e.Hint, "Close World Editor") {
		t.Errorf("error = %+v", e)
	}
	after := withoutDirs(snapshot(t, dir))
	if after["war3mapskin.txt"] != "merged" || after["Sound"] != "in the way" {
		t.Errorf("the map holds %v, want the first change written and the file in the way as it was", after)
	}
	if _, written := after["new.txt"]; written {
		t.Error("new.txt was written after the change that failed")
	}
	if journal.Len() < 1 || journal.Len() > 2 {
		t.Errorf("the journal has touched %d files, want the first change, and the second at most", journal.Len())
	}
	if unrestored := journal.Undo(); len(unrestored) != 0 {
		t.Errorf("unrestored = %v", unrestored)
	}
	undone := withoutDirs(snapshot(t, dir))
	if undone["war3mapskin.txt"] != "skin" || undone["Sound"] != "in the way" {
		t.Errorf("after the undo the map holds %v, want the first change taken back", undone)
	}
}

func TestApplyInPlaceWritesThroughTheJournalWhichCanUndoIt(t *testing.T) {
	folder, dir := openFolder(t, sourceMap)
	before := snapshot(t, dir)
	readFile(t, folder, "war3map.w3i")
	readFile(t, folder, "war3mapMap.blp")
	view := folder.WithChanges([]Change{
		newWrite("WAR3MAP.W3I", "patched"),
		newRemoval("war3mapmap.blp"),
		newWrite("Sound/Music/theme.mp3", "theme"),
		newWrite("Textures/New.blp", "new"),
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
		t.Errorf("unrestored = %v", unrestored)
	}
	if got := withoutDirs(snapshot(t, dir)); !reflect.DeepEqual(got, withoutDirs(before)) {
		t.Errorf("after the undo the map holds %v, want %v", got, withoutDirs(before))
	}
}

func TestApplyInPlaceRefusesAFileThatIsNotAsTheFolderSawIt(t *testing.T) {
	changes := []Change{newWrite("a.txt", "1"), newWrite("b.txt", "2"), newWrite("new.txt", "3"), newWrite("c.txt", "4")}
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
		return func(t *testing.T, folder *Folder) { readFile(t, view(folder), "B.TXT") }
	}
	itself := func(folder *Folder) *Folder { return folder }
	aView := func(folder *Folder) *Folder { return folder.WithChanges([]Change{newWrite("other.txt", "")}) }
	cases := []struct {
		name    string
		read    func(t *testing.T, folder *Folder)
		meddle  func(t *testing.T, dir string)
		refused string
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
			folder, dir := openFolder(t, map[string]string{"a.txt": "a", "b.txt": "b", "c.txt": "c"})
			if c.read != nil {
				c.read(t, folder)
			}
			view := folder.WithChanges(changes)
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
			e := asDiagError(t, err)
			if !contains(e.Msg, mapDisplayPath+"/"+c.refused+" changed after") || e.File != mapDisplayPath+"/"+c.refused ||
				!contains(e.Hint, "Close World Editor") {
				t.Errorf("error = %+v", e)
			}
			after := withoutDirs(snapshot(t, dir))
			if after["a.txt"] != "1" || after["c.txt"] != "c" {
				t.Errorf("the map holds %v, want a.txt written and c.txt untouched", after)
			}
			earlier := slices.IndexFunc(changes, func(change Change) bool { return change.Path == c.refused })
			if journal.Len() != earlier {
				t.Errorf("the journal has touched %d files, want the %d before the refused one", journal.Len(), earlier)
			}
		})
	}
}

func TestApplyInPlaceStopsAtACancelledContext(t *testing.T) {
	folder, dir := openFolder(t, sourceMap)
	before := snapshot(t, dir)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var journal fsx.Journal
	err := folder.WithChanges([]Change{newWrite("war3map.w3i", "patched")}).ApplyInPlace(ctx, &journal)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ApplyInPlace = %v, want the context's error", err)
	}
	if got := snapshot(t, dir); journal.Len() != 0 || !reflect.DeepEqual(got, before) {
		t.Errorf("a cancelled apply touched %d files; the map holds %v", journal.Len(), got)
	}
}

type cancelAfterCtx struct {
	context.Context
	calls int
}

func (c *cancelAfterCtx) Err() error {
	if c.calls == 0 {
		return context.Canceled
	}
	c.calls--
	return nil
}

func TestApplyInPlaceStopsBetweenTwoChanges(t *testing.T) {
	folder, dir := openFolder(t, sourceMap)
	view := folder.WithChanges([]Change{newWrite("war3map.w3i", "patched"), newWrite("war3mapskin.txt", "merged"), newWrite("new.txt", "new")})
	var journal fsx.Journal
	err := view.ApplyInPlace(&cancelAfterCtx{Context: context.Background(), calls: 1}, &journal)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ApplyInPlace = %v, want the context's error", err)
	}
	after := withoutDirs(snapshot(t, dir))
	if after["war3map.w3i"] != "patched" || after["war3mapskin.txt"] != "skin" || journal.Len() != 1 {
		t.Errorf("the journal touched %d files and the map holds %v, want only war3map.w3i written", journal.Len(), after)
	}
	if _, written := after["new.txt"]; written {
		t.Error("new.txt was written after the context was cancelled")
	}
}
