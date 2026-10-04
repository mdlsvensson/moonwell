package mapdir

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
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
		put("sound/effects/hit.wav", "hit"),
		put("textures/New.blp", "new"),
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
		// Each folder has one spelling, so the staged map is one the scan accepts.
		if _, err := Open(stage, label); err != nil {
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
	folder, dir := open(t, sourceMap)
	view := folder.With([]Change{put("new.txt", "data")})
	// A folder made after the scan, where the new file goes: the copy has it, and no system writes a file over it.
	if err := os.Mkdir(filepath.Join(dir, "new.txt"), 0o777); err != nil {
		t.Fatal(err)
	}
	stage := filepath.Join(t.TempDir(), "stage", "map.w3x")
	e := asError(t, view.StageTo(stage))
	if !contains(e.Msg, "Staging the map failed") || e.Cause == nil || e.Hint != hint ||
		e.File != filepath.Join(stage, "new.txt") {
		t.Errorf("error = %+v", e)
	}
}

// plan is two changes the map has a place for, and then the given ones: a refusal of the plan must come before
// the first two are written.
func plan(changes ...Change) []Change {
	return append([]Change{put("war3mapskin.txt", "merged"), put("new.txt", "new")}, changes...)
}

// plannersBugs are plans no planner may make: one of their changes has a name no file can have, or no place in
// the map.
var plannersBugs = []struct {
	name    string
	changes []Change
	words   []string // of the error
}{
	// With keeps a name that cannot be written as given, so the refusal names it so.
	{"a path that leaves the folder", plan(put("../outside.txt", "data")),
		[]string{`Cannot write "../outside.txt"`, "relative path"}},
	{"a leading slash before a file the map has", plan(put("/WAR3MAP.W3I", "data")),
		[]string{`Cannot write "/WAR3MAP.W3I"`, "relative path"}},
	{"an empty folder name", plan(put("a//b", "data")), []string{`Cannot write "a//b"`, "relative path"}},
	{"a way out of a folder", plan(put("textures/../war3map.w3i", "data")),
		[]string{`Cannot write "Textures/../war3map.w3i"`, "relative path"}},
	{"a character Windows forbids", plan(put("what?.blp", "data")),
		[]string{`Cannot write "what?.blp"`, "relative path"}},
	{"a device name below a new folder", plan(put("Sound/nul.mp3", "data")),
		[]string{`Cannot write "Sound/nul.mp3"`, "relative path"}},
	{"a change named as a folder of the map", plan(put("textures", "data")),
		[]string{"Cannot write textures", "the folder Textures of the map"}},
	{"a change through a file of the map", plan(put("WAR3MAP.W3I/x.txt", "data")),
		[]string{"Cannot write WAR3MAP.W3I/x.txt", "goes through war3map.w3i, a file of the map"}},
	{"a change through a file below a folder", plan(put("textures/old.blp/deep/x.txt", "data")),
		[]string{"Cannot write Textures/old.blp/deep/x.txt", "goes through Textures/Old.blp, a file of the map"}},
	// A file of the map never becomes a folder, whatever the plan does with the file.
	{"a change through a file that the plan removes first",
		plan(drop("WAR3MAP.W3I"), put("war3map.w3i/x.txt", "data")),
		[]string{"Cannot write war3map.w3i/x.txt", "goes through war3map.w3i, a file of the map"}},
	{"a change through a file that the plan removes after it",
		plan(put("war3map.w3i/x.txt", "data"), drop("WAR3MAP.W3I")),
		[]string{"Cannot write war3map.w3i/x.txt", "goes through war3map.w3i, a file of the map"}},
	// Of two changes that have no place beside each other, the one planned first is named.
	{"a new file named as the folder a later change makes",
		plan(put("Sound", "data"), put("sound/theme.mp3", "theme")),
		[]string{"Cannot write Sound", "the folder sound of the map"}},
	{"a new file below the name a later change writes as a file",
		plan(put("Sound/Music/theme.mp3", "theme"), put("sound", "data")),
		[]string{"Cannot write Sound/Music/theme.mp3", "goes through sound, a file of the map"}},
}

// asPlannersBug is the text of an error that must not be one a user is told to act on.
func asPlannersBug(t *testing.T, err error) string {
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
	for _, c := range plannersBugs {
		t.Run(c.name, func(t *testing.T) {
			folder, dir := open(t, sourceMap)
			source := snapshot(t, dir)
			// The stage holds an earlier build, and the folder it is in shows a write beside it.
			around := t.TempDir()
			stage := filepath.Join(around, "stage", "map.w3x")
			testkit.WriteFile(t, stage, "left.txt", []byte("from an earlier build"))
			earlier := snapshot(t, around)
			text := asPlannersBug(t, folder.With(c.changes).StageTo(stage))
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
	for _, c := range plannersBugs {
		t.Run(c.name, func(t *testing.T) {
			folder, dir := open(t, sourceMap)
			// The folder the map is in shows a write beside the map.
			before := snapshot(t, filepath.Dir(dir))
			var journal fsx.Journal
			text := asPlannersBug(t, folder.With(c.changes).ApplyInPlace(context.Background(), &journal))
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
			folder, err := Open(dir, label)
			if err != nil {
				t.Fatal(err)
			}
			before := testkit.Snapshot(t, project)
			err = folder.With([]Change{put("war3map.w3i", "patched"), put("new.txt", "new")}).StageTo(c.stage(dir))
			e := asError(t, err)
			if !contains(e.Msg, "source map") || e.File != label || e.Hint == "" {
				t.Errorf("error = %+v", e)
			}
			if after := testkit.Snapshot(t, project); !reflect.DeepEqual(after, before) {
				t.Errorf("the project holds %v, want it untouched: %v", after, before)
			}
		})
	}
}

func TestStageToDoesNotWriteThroughALinkMadeAfterTheScan(t *testing.T) {
	folder, dir := open(t, sourceMap)
	view := folder.With([]Change{put("textures/old.blp", "patched"), put("Textures/New.blp", "new")})
	outside := linkAway(t, dir)
	before := snapshot(t, outside)
	stage := filepath.Join(t.TempDir(), "map.w3x")
	// How the copy fails differs by system (the link is copied as a link, or cannot be copied); that it fails, and
	// that nothing is written where the link points, does not.
	if e := asError(t, view.StageTo(stage)); !contains(e.Msg, "Staging the map failed") {
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
		{"a file the map had", put("textures/old.blp", "patched"), "Textures/Old.blp"},
		{"a file to remove", drop("Textures/Old.blp"), "Textures/Old.blp"},
		{"a new file", put("textures/New.blp", "new"), "Textures/New.blp"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			folder, dir := open(t, sourceMap)
			view := folder.With([]Change{c.change})
			outside := linkAway(t, dir)
			before := snapshot(t, outside)
			var journal fsx.Journal
			e := asError(t, view.ApplyInPlace(context.Background(), &journal))
			if !contains(e.Msg, "Writing a map file failed") || !contains(e.Msg, "Symlinks") || e.File != label+"/"+c.at {
				t.Errorf("error = %+v", e)
			}
			if after := snapshot(t, outside); journal.Len() != 0 || !reflect.DeepEqual(after, before) {
				t.Errorf("the journal touched %d files; the folder the link points at holds %v", journal.Len(), after)
			}
		})
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
		t.Errorf("unrestored = %v", unrestored)
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

// cancelledAfter is a context that is cancelled once Err has been asked a number of times. ApplyInPlace asks once
// before each change.
type cancelledAfter struct {
	context.Context
	asks int
}

func (c *cancelledAfter) Err() error {
	if c.asks == 0 {
		return context.Canceled
	}
	c.asks--
	return nil
}

func TestApplyInPlaceStopsBetweenTwoChanges(t *testing.T) {
	folder, dir := open(t, sourceMap)
	view := folder.With([]Change{put("war3map.w3i", "patched"), put("war3mapskin.txt", "merged"), put("new.txt", "new")})
	var journal fsx.Journal
	err := view.ApplyInPlace(&cancelledAfter{Context: context.Background(), asks: 1}, &journal)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("ApplyInPlace = %v, want the context's error", err)
	}
	after := filesOf(snapshot(t, dir))
	if after["war3map.w3i"] != "patched" || after["war3mapskin.txt"] != "skin" || journal.Len() != 1 {
		t.Errorf("the journal touched %d files and the map holds %v, want only war3map.w3i written", journal.Len(), after)
	}
	if _, written := after["new.txt"]; written {
		t.Error("new.txt was written after the context was cancelled")
	}
}
