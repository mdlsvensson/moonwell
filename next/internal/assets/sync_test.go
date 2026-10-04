package assets

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/imp"
)

func TestSyncImportsMappedAssetsAndKeepsTheEditorsOwnImports(t *testing.T) {
	s := newSite(t)
	const icon = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	const disabled = "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp"
	const block = `{"paths":{"icons/disabled.blp":"` + disabled + `"},"exclude":[]}`
	put(t, s.root, "assets/"+icon)
	put(t, s.root, "assets/icons/disabled.blp", "disabled")
	put(t, s.mapDir, "war3mapImported/existing.wav", "editor")
	s.setImports(imp.Entry{Flag: 5, Path: "existing.wav"})

	s.synced(block)
	want := []imp.Entry{
		{Flag: 5, Path: "existing.wav"},
		{Flag: imp.CustomPath, Path: strings.ReplaceAll(icon, "/", `\`)},
		{Flag: imp.CustomPath, Path: strings.ReplaceAll(disabled, "/", `\`)},
	}
	if got := s.imports(); !slices.Equal(got, want) {
		t.Errorf("the index lists %+v, want %+v", got, want)
	}
	if s.inMap(icon) != "asset" || s.inMap(disabled) != "disabled" || s.inMap("war3mapImported/existing.wav") != "editor" {
		t.Error("the map does not hold the two assets beside the editor's own file")
	}
	wantState := "{\n  \"version\": 1,\n  \"files\": {\n    \"" + icon + "\": \"" + hashed("asset") + "\",\n    \"" +
		disabled + "\": \"" + hashed("disabled") + "\"\n  }\n}\n"
	if got := s.stateText(); got != wantState {
		t.Errorf("the state file is\n%s\nwant\n%s", got, wantState)
	}
	if _, again := s.planned(block); len(again.Changes) != 0 {
		t.Errorf("a second sync changes %q", names(again.Changes))
	}
}

func TestSyncUpdatesRenamesAndRemovesOnlyOwnedFiles(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/Models/unit.mdx", "first")
	put(t, s.mapDir, "unmanaged.txt", "keep")
	s.synced(noBlock)

	put(t, s.root, "assets/Models/unit.mdx", "second")
	s.synced(noBlock)
	if got := s.inMap("Models/unit.mdx"); got != "second" {
		t.Errorf("the changed asset is %q in the map", got)
	}
	s.synced(`{"paths":{"Models/unit.mdx":"Models/renamed.mdx"},"exclude":[]}`)
	if s.inMap("Models/unit.mdx") != missing || s.inMap("Models/renamed.mdx") != "second" {
		t.Error("the asset was not renamed")
	}
	if got, want := s.imports(), []imp.Entry{{Flag: imp.CustomPath, Path: `Models\renamed.mdx`}}; !slices.Equal(got, want) {
		t.Errorf("the index lists %+v, want %+v", got, want)
	}

	if err := os.Remove(filepath.Join(s.root, "assets", "Models", "unit.mdx")); err != nil {
		t.Fatal(err)
	}
	s.synced(noBlock)
	if len(s.imports()) != 0 || s.inMap("Models/renamed.mdx") != missing || s.inMap("unmanaged.txt") != "keep" {
		t.Error("removing the asset left its file or its import, or touched another file")
	}
}

func TestSyncKeepsTheFlagWorldEditorSavedOnAnOwnedImport(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/Textures/a.blp")
	put(t, s.root, "assets/Textures/b.blp")
	s.synced(noBlock)
	// World Editor 3.00 saves the flag 13 as 29.
	saved := []imp.Entry{{Flag: 29, Path: `Textures\a.blp`}, {Flag: 29, Path: `Textures\b.blp`}}
	s.setImports(saved...)
	if _, result := s.planned(noBlock); len(result.Changes) != 0 {
		t.Errorf("a map World Editor saved is changed: %q", names(result.Changes))
	}
	put(t, s.root, "assets/Textures/c.blp")
	s.synced(noBlock)
	if got, want := s.imports(), append(saved, imp.Entry{Flag: imp.CustomPath, Path: `Textures\c.blp`}); !slices.Equal(got, want) {
		t.Errorf("the index lists %+v, want %+v", got, want)
	}
}

func TestSyncWritesNoStateFileWhenItOwnsNothingAndRemovesOneItDoesNotNeed(t *testing.T) {
	s := newSite(t)
	s.synced(noBlock)
	if got := s.stateText(); got != missing {
		t.Errorf("owning nothing wrote the state file %q", got)
	}
	put(t, s.root, "assets/a.blp")
	s.synced(noBlock)
	if s.stateText() == missing {
		t.Error("owning a file wrote no state file")
	}
	if err := os.Remove(filepath.Join(s.root, "assets", "a.blp")); err != nil {
		t.Fatal(err)
	}
	s.synced(noBlock)
	if s.inMap("a.blp") != missing || s.stateText() != missing {
		t.Error("the owned file or the state file is there after the asset was removed")
	}
}

func TestTheFilesLibrariesShipAreSyncedLikeTheMapsOwnAndOwned(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/Models/Own.mdx", "own")
	put(t, s.root, "assets/icons/shared.blp", "the map's")
	put(t, s.root, "libraries/ui/war3mapImported/ui/frames.toc", "toc")
	put(t, s.root, "libraries/ui/icons/shared.blp", "the library's")

	result := s.synced(noBlock, "ui")
	want := []row{
		{"", "icons/shared.blp", "icons/shared.blp"},
		{"", "Models/Own.mdx", "Models/Own.mdx"},
		{"ui", "war3mapImported/ui/frames.toc", "war3mapImported/ui/frames.toc"},
	}
	if got := rows(result.Assets); !slices.Equal(got, want) {
		t.Fatalf("the assets are %+v, want %+v", got, want)
	}
	if s.inMap("war3mapImported/ui/frames.toc") != "toc" || s.inMap("icons/shared.blp") != "the map's" {
		t.Error("the map does not hold the library's file and the map's own")
	}
	paths := func() []string {
		var list []string
		for _, entry := range s.imports() {
			list = append(list, entry.Path)
		}
		return list
	}
	if got := paths(); !slices.Equal(got, []string{`icons\shared.blp`, `Models\Own.mdx`, `war3mapImported\ui\frames.toc`}) {
		t.Errorf("the index lists %q", got)
	}
	if _, again := s.planned(noBlock, "ui"); len(again.Changes) != 0 {
		t.Errorf("a second sync changes %q", names(again.Changes))
	}

	// Without the library, the next sync removes the file it owned.
	s.synced(noBlock)
	if s.inMap("war3mapImported/ui/frames.toc") != missing {
		t.Error("the library's file is in the map after the library was dropped")
	}
	if got := paths(); !slices.Equal(got, []string{`icons\shared.blp`, `Models\Own.mdx`}) {
		t.Errorf("the index lists %q", got)
	}
}

// ---- a sync that does not finish ----

// inTheWayOf puts a file where the folder of a new asset goes, which no check before the writes foresees: the
// write of that asset fails.
func inTheWayOf(t *testing.T, s *site, folder string) {
	t.Helper()
	put(t, s.mapDir, folder, "in the way")
}

func TestAFailedSyncUndoesTheWritesItMade(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/a.blp")
	put(t, s.root, "assets/Sound/b.blp")
	folder, result := s.planned(noBlock)
	inTheWayOf(t, s, "Sound")
	before := testkit.Snapshot(t, s.root)

	e := asError(t, Sync(background, folder, result, s.state), "a failed sync")
	if !strings.HasPrefix(e.Msg, "Writing assets failed: ") || !strings.HasSuffix(e.Msg, ". Every change was undone.") ||
		e.File != mapLabel+"/Sound/b.blp" || e.Hint == "" || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	s.unchanged(before, "a failed sync")
}

func TestASyncThatCannotWriteOverAnOwnedFileUndoesItsWritesAndLeavesTheFileAsItWas(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/0.blp", "first")
	put(t, s.root, "assets/a.blp", "first")
	s.synced(noBlock)
	put(t, s.root, "assets/0.blp", "second")
	put(t, s.root, "assets/a.blp", "second")
	folder, result := s.planned(noBlock)
	before := testkit.Snapshot(t, s.root)
	makeUnwritable(t, filepath.Join(s.mapDir, "a.blp"))

	e := asError(t, Sync(background, folder, result, s.state), "a sync that cannot write over a file")
	if !strings.HasPrefix(e.Msg, "Writing assets failed: ") || !strings.HasSuffix(e.Msg, ". Every change was undone.") ||
		e.File != mapLabel+"/a.blp" || e.Hint == "" || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	s.unchanged(before, "a failed sync")
}

// changedMap is a site whose map a sync has written once and whose next sync changes it in every way: a file
// replaced, a file added, a file removed and the index. The state file is the fifth and last thing that sync
// writes.
func changedMap(t *testing.T) (*site, *mapdir.Folder, *Result) {
	t.Helper()
	s := newSite(t)
	put(t, s.root, "assets/a.blp", "first")
	put(t, s.root, "assets/dropped.blp")
	put(t, s.mapDir, "war3mapImported/existing.wav", "editor")
	s.setImports(imp.Entry{Flag: 5, Path: "existing.wav"})
	s.synced(noBlock)

	put(t, s.root, "assets/a.blp", "second")
	put(t, s.root, "assets/new.blp")
	if err := os.Remove(filepath.Join(s.root, "assets", "dropped.blp")); err != nil {
		t.Fatal(err)
	}
	folder, result := s.planned(noBlock)
	if got, want := names(result.Changes), []string{"a.blp", "new.blp", "-dropped.blp", "war3map.imp"}; !slices.Equal(got, want) {
		t.Fatalf("the changes are %q, want %q", got, want)
	}
	return s, folder, result
}

// mapIsChanged fails the test unless the map of a changedMap holds every change of its second sync.
func (s *site) mapIsChanged() {
	s.t.Helper()
	if s.inMap("a.blp") != "second" || s.inMap("new.blp") != "asset" || s.inMap("dropped.blp") != missing {
		s.t.Error("the map's changes are not made before the state file is written")
	}
}

// The state file is written last. When that write fails, every change of the map is made already. Each is put
// back, and the state file is as it was.
func TestASyncThatCannotWriteItsStateRestoresTheMapByteForByte(t *testing.T) {
	s, folder, result := changedMap(t)
	before := testkit.Snapshot(t, s.root)
	makeUnwritable(t, s.state)
	ctx := &countdown{Context: background, limit: never, before: map[int]func(){5: s.mapIsChanged}}

	e := asError(t, Sync(ctx, folder, result, s.state), "a sync that cannot write its state")
	if !strings.HasPrefix(e.Msg, "Writing assets failed: ") || !strings.HasSuffix(e.Msg, ". Every change was undone.") ||
		e.File != s.state || !strings.Contains(e.Hint, "can be written") || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	if ctx.asks != 5 {
		t.Errorf("the sync asked %d times, want the state file to be its ask 5 and its last", ctx.asks)
	}
	s.unchanged(before, "a failed sync")
}

// The state file is read once more just before it is written. When it cannot be read at that moment, here
// because a folder has taken its place, every change of the map is made already. Each is put back.
func TestASyncThatCannotReadItsStateJustBeforeWritingItRestoresTheMapByteForByte(t *testing.T) {
	s, folder, result := changedMap(t)
	before := testkit.Snapshot(t, s.mapDir)
	// Before the fifth ask, the one for the state file, a folder takes the state file's place.
	ctx := &countdown{Context: background, limit: never, before: map[int]func(){5: func() {
		s.mapIsChanged()
		if err := errors.Join(os.Remove(s.state), os.Mkdir(s.state, 0o777)); err != nil {
			t.Fatal(err)
		}
	}}}

	e := asError(t, Sync(ctx, folder, result, s.state), "a sync that cannot read its state before it writes it")
	if !strings.HasPrefix(e.Msg, "Writing assets failed: ") || !strings.HasSuffix(e.Msg, ". Every change was undone.") ||
		e.File != s.state || !strings.Contains(e.Hint, "a readable file, not a folder") || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	if after := testkit.Snapshot(t, s.mapDir); !maps.EqualFunc(before, after, slices.Equal) {
		t.Errorf("the map holds %q, want what it held before the sync", slices.Sorted(maps.Keys(after)))
	}
}

func TestAnInterruptedSyncWritesNothingAndOneInterruptedMidwayUndoesItsWrites(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/a.blp")
	put(t, s.root, "assets/b.blp")
	before := testkit.Snapshot(t, s.root)
	// One ask before each change, and the state file is the last: a.blp, b.blp, war3map.imp, the state.
	const asks = 4
	for limit := range asks {
		folder, result := s.planned(noBlock)
		ctx := &countdown{Context: background, limit: limit}
		e := asError(t, Sync(ctx, folder, result, s.state), "an interrupted sync")
		want := "Interrupted; every change was undone."
		if limit == 0 {
			want = "Interrupted; nothing was written."
		}
		if e.Msg != want || ctx.asks != limit+1 {
			t.Errorf("cancelled at ask %d: error = %+v after %d asks, want %q", limit+1, e, ctx.asks, want)
		}
		s.unchanged(before, "an interrupted sync")
	}
	folder, result := s.planned(noBlock)
	ctx := &countdown{Context: background, limit: asks}
	if err := Sync(ctx, folder, result, s.state); err != nil || ctx.asks != asks {
		t.Errorf("Sync = %v after %d asks, want it to finish after %d", err, ctx.asks, asks)
	}
}

func TestASyncWithNothingToWriteIsNotInterrupted(t *testing.T) {
	s := newSite(t)
	folder, result := s.planned(noBlock)
	ctx := &countdown{Context: background}
	if err := Sync(ctx, folder, result, s.state); err != nil || ctx.asks != 0 {
		t.Errorf("Sync = %v after %d asks, want no ask where nothing is written", err, ctx.asks)
	}
}

// Another program gets between two writes: it replaces a.blp, which the sync has written, by a folder with a file
// in it, and puts a file where b.blp goes. The sync stops at b.blp and cannot take a.blp out again.
func TestAnUndoThatCannotRestoreAFileNamesTheFailureAndEveryFileItCouldNotRestore(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/a.blp")
	put(t, s.root, "assets/b.blp")
	folder, result := s.planned(noBlock)
	ctx := &countdown{Context: background, limit: never, before: map[int]func(){2: func() {
		if err := os.Remove(filepath.Join(s.mapDir, "a.blp")); err != nil {
			t.Fatal(err)
		}
		put(t, s.mapDir, "a.blp/inner.txt")
		put(t, s.mapDir, "b.blp", "another program's")
	}}}

	e := asError(t, Sync(ctx, folder, result, s.state), "a sync that cannot undo")
	failure := mapLabel + "/b.blp changed after the assets were checked."
	if !strings.HasPrefix(e.Msg, "Writing assets failed ("+failure+"), and these files could not be restored: "+mapLabel+"/a.blp (") ||
		!strings.Contains(e.Hint, "version control") {
		t.Errorf("error = %+v", e)
	}
	if cause, ok := diag.First(e.Cause); !ok || cause.Msg != failure {
		t.Errorf("the cause is %v, want the failure that stopped the sync", e.Cause)
	}
	if s.inMap("b.blp") != "another program's" || s.inMap("war3map.imp") != missing || s.stateText() != missing {
		t.Error("the sync wrote past the file it stopped at")
	}
}

func TestAnInterruptedSyncThatCannotUndoSaysSo(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/a.blp")
	put(t, s.root, "assets/b.blp")
	folder, result := s.planned(noBlock)
	ctx := &countdown{Context: background, limit: 1, before: map[int]func(){2: func() {
		if err := os.Remove(filepath.Join(s.mapDir, "a.blp")); err != nil {
			t.Fatal(err)
		}
		put(t, s.mapDir, "a.blp/inner.txt")
	}}}
	e := asError(t, Sync(ctx, folder, result, s.state), "a sync that cannot undo")
	if !strings.HasPrefix(e.Msg, "Writing assets failed (interrupted), and these files could not be restored: "+mapLabel+"/a.blp (") {
		t.Errorf("error = %+v", e)
	}
}

func TestSyncRefusesAFileThatChangedAfterThePlanAndWritesOverNothing(t *testing.T) {
	// What another program does to the map between the plan and the sync.
	writes := func(name string, content ...string) func(*site) {
		return func(s *site) { put(s.t, s.mapDir, name, content...) }
	}
	saves := func(s *site) { s.setImports(imp.Entry{Flag: 5, Path: "sound.wav"}) }
	removes := func(s *site) {
		if err := os.Remove(filepath.Join(s.mapDir, "replaced.blp")); err != nil {
			s.t.Fatal(err)
		}
	}
	tests := []struct {
		name    string
		meddle  func(s *site)
		changed string // the file the sync stops at
	}{
		{"an owned file the plan replaces, edited by hand", writes("replaced.blp", "edited by hand"), "replaced.blp"},
		{"an owned file the plan removes, edited by hand", writes("dropped.blp", "edited by hand"), "dropped.blp"},
		{"a file where a new asset goes", writes("new.blp", "the editor's"), "new.blp"},
		{"a folder where a new asset goes", writes("new.blp/inner.txt"), "new.blp"},
		{"the index, saved by World Editor", saves, "war3map.imp"},
		{"an owned file the plan replaces, removed", removes, "replaced.blp"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSite(t)
			put(t, s.root, "assets/replaced.blp", "first")
			put(t, s.root, "assets/dropped.blp")
			put(t, s.root, "assets/0.blp")
			s.synced(noBlock)
			put(t, s.root, "assets/replaced.blp", "second")
			put(t, s.root, "assets/new.blp")
			put(t, s.root, "assets/0.blp", "written before the others")
			if err := os.Remove(filepath.Join(s.root, "assets", "dropped.blp")); err != nil {
				t.Fatal(err)
			}
			folder, result := s.planned(noBlock)
			tt.meddle(s)
			before := testkit.Snapshot(t, s.root)

			e := asError(t, Sync(background, folder, result, s.state), "a sync of a map that changed")
			file := mapLabel + "/" + tt.changed
			if e.Msg != file+" changed after the assets were checked." || e.File != file || !strings.Contains(e.Hint, "Close World Editor") {
				t.Errorf("error = %+v", e)
			}
			s.unchanged(before, "a refused sync")
		})
	}
}

// Another program gets at the state file between the start of the sync and the ask before the state file, which
// is the last. The sync has changed the map by then.
func TestSyncRefusesAStateFileThatChangedAfterItBeganAndUndoesTheMap(t *testing.T) {
	writes := func(s *site) { put(s.t, s.root, ".asset-state/map.w3x.json", "another program's") }
	removes := func(s *site) {
		if err := os.Remove(s.state); err != nil {
			s.t.Fatal(err)
		}
	}
	tests := []struct {
		name    string
		owned   bool   // whether a sync before this one wrote a state file
		asset   string // what assets/a.blp holds at the sync; "" for a project without the file
		meddle  func(s *site)
		lastAsk int
		state   string // what the state file holds afterwards
	}{
		{"changed before it is written", true, "second", writes, 2, "another program's"},
		{"removed before it is written", true, "second", removes, 2, missing},
		{"made before it is written", false, "first", writes, 3, "another program's"},
		{"changed before it is removed", true, "", writes, 3, "another program's"},
		{"removed before it is removed", true, "", removes, 3, missing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newSite(t)
			if tt.owned {
				put(t, s.root, "assets/a.blp", "first")
				s.synced(noBlock)
			}
			if err := os.RemoveAll(filepath.Join(s.root, "assets", "a.blp")); err != nil {
				t.Fatal(err)
			}
			if tt.asset != "" {
				put(t, s.root, "assets/a.blp", tt.asset)
			}
			folder, result := s.planned(noBlock)
			before := testkit.Snapshot(t, s.mapDir)
			ctx := &countdown{Context: background, limit: never, before: map[int]func(){tt.lastAsk: func() { tt.meddle(s) }}}

			e := asError(t, Sync(ctx, folder, result, s.state), "a sync whose state file changed")
			if e.Msg != s.state+" changed after the assets were checked." || e.File != s.state ||
				!strings.Contains(e.Hint, "Close World Editor") {
				t.Errorf("error = %+v", e)
			}
			if ctx.asks != tt.lastAsk {
				t.Errorf("the sync asked %d times, want the state file to be its ask %d and its last", ctx.asks, tt.lastAsk)
			}
			if after := testkit.Snapshot(t, s.mapDir); !maps.EqualFunc(before, after, slices.Equal) {
				t.Errorf("the map holds %q, want what it held before the sync", slices.Sorted(maps.Keys(after)))
			}
			if got := s.stateText(); got != tt.state {
				t.Errorf("the state file holds %q, want %q: what the other program left", got, tt.state)
			}
		})
	}
}

// A state file that holds the state already is not written, and so it is not looked at again.
func TestAStateFileThatNeedsNoWriteIsNotLookedAtAgain(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/a.blp")
	s.synced(noBlock)
	s.setImports()
	folder, result := s.planned(noBlock)
	if got := names(result.Changes); !slices.Equal(got, []string{"war3map.imp"}) {
		t.Fatalf("the changes are %q", got)
	}
	ctx := &countdown{Context: background, limit: never, before: map[int]func(){1: func() {
		put(t, s.root, ".asset-state/map.w3x.json", "another program's")
	}}}
	if err := Sync(ctx, folder, result, s.state); err != nil || ctx.asks != 1 {
		t.Errorf("Sync = %v after %d asks, want it to finish after the one ask of the index", err, ctx.asks)
	}
	if len(s.imports()) != 1 || s.stateText() != "another program's" {
		t.Error("the index is not written, or the state file is")
	}
}

func TestSyncRefusesAStateFileItCannotReadBeforeItWritesAnything(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/a.blp")
	folder, result := s.planned(noBlock)
	if err := os.MkdirAll(s.state, 0o777); err != nil {
		t.Fatal(err)
	}
	before := testkit.Snapshot(t, s.root)
	e := asError(t, Sync(background, folder, result, s.state), "a sync with a folder in place of its state file")
	if !strings.Contains(e.Msg, "Reading the asset ownership state failed") || e.File != s.state {
		t.Errorf("error = %+v", e)
	}
	s.unchanged(before, "a refused sync")
}

// Collect refuses an asset inside another. Given by another caller, the two reach a plan, which the folder refuses
// to write, before its first write: that caller's bug.
func TestSyncWritesNothingOfAPlanWithAnAssetInsideAnother(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/a.blp")
	before := testkit.Snapshot(t, s.root)
	folder := s.open()
	assets := []Asset{{Target: "a.blp", Bytes: []byte("a")}, {Target: "data", Bytes: []byte("outer")},
		{Target: "data/inner.txt", Bytes: []byte("inner")}}
	result, err := Plan(background, folder, assets, State{})
	if err != nil {
		t.Fatalf("Plan: %v", diag.Format(err))
	}
	err = Sync(background, folder, result, s.state)
	var expected *diag.Error
	if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "Cannot write data") {
		t.Errorf("Sync = %v, want a plain error about data, a file and a folder at once", err)
	}
	s.unchanged(before, "a refused sync")
}

// The folder a plan was made from holds what the plan read, and that is what each file is checked against before
// it is written over. Another folder, even one opened on the same map, does not: the edited file would be lost.
func TestSyncRefusesAnotherFolderThanTheOneThePlanWasMadeFrom(t *testing.T) {
	s := newSite(t)
	put(t, s.root, "assets/a.blp", "first")
	s.synced(noBlock)
	put(t, s.root, "assets/a.blp", "second")
	_, result := s.planned(noBlock)
	put(t, s.mapDir, "a.blp", "edited by hand")
	before := testkit.Snapshot(t, s.root)

	err := Sync(background, s.open(), result, s.state)
	var expected *diag.Error
	if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "the folder the plan was made from") {
		t.Errorf("Sync = %v, want a plain error about the folder", err)
	}
	s.unchanged(before, "a refused sync")
}
