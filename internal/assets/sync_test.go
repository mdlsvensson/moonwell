package assets

import (
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/imp"
)

func TestSyncImportsMappedAssetsAndKeepsTheEditorsOwnImports(t *testing.T) {
	s := newAssetProject(t)
	const icon = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	const disabled = "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp"
	const block = `{"paths":{"icons/disabled.blp":"` + disabled + `"},"exclude":[]}`
	writeFile(t, s.root, "assets/"+icon)
	writeFile(t, s.root, "assets/icons/disabled.blp", "disabled")
	writeFile(t, s.mapDir, "war3mapImported/existing.wav", "editor")
	s.writeImports(imp.Entry{Flag: 5, Path: "existing.wav"})

	s.mustSync(block)
	want := []imp.Entry{
		{Flag: 5, Path: "existing.wav"},
		{Flag: imp.CustomPath, Path: strings.ReplaceAll(icon, "/", `\`)},
		{Flag: imp.CustomPath, Path: strings.ReplaceAll(disabled, "/", `\`)},
	}
	if got := s.readImports(); !slices.Equal(got, want) {
		t.Errorf("the index lists %+v, want %+v", got, want)
	}
	if s.readMapFile(icon) != "asset" || s.readMapFile(disabled) != "disabled" || s.readMapFile("war3mapImported/existing.wav") != "editor" {
		t.Error("the map does not hold the two assets beside the editor's own file")
	}
	wantState := "{\n  \"version\": 1,\n  \"files\": {\n    \"" + icon + "\": \"" + sha256Of("asset") + "\",\n    \"" +
		disabled + "\": \"" + sha256Of("disabled") + "\"\n  }\n}\n"
	if got := s.readState(); got != wantState {
		t.Errorf("the state file is\n%s\nwant\n%s", got, wantState)
	}
	if _, again := s.mustPlan(block); len(again.Changes) != 0 {
		t.Errorf("a second sync changes %q", changePaths(again.Changes))
	}
}

func TestSyncUpdatesRenamesAndRemovesOnlyOwnedFiles(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/Models/unit.mdx", "first")
	writeFile(t, s.mapDir, "unmanaged.txt", "keep")
	s.mustSync(noBlock)

	writeFile(t, s.root, "assets/Models/unit.mdx", "second")
	s.mustSync(noBlock)
	if got := s.readMapFile("Models/unit.mdx"); got != "second" {
		t.Errorf("the changed asset is %q in the map", got)
	}
	s.mustSync(`{"paths":{"Models/unit.mdx":"Models/renamed.mdx"},"exclude":[]}`)
	if s.readMapFile("Models/unit.mdx") != missing || s.readMapFile("Models/renamed.mdx") != "second" {
		t.Error("the asset was not renamed")
	}
	if got, want := s.readImports(), []imp.Entry{{Flag: imp.CustomPath, Path: `Models\renamed.mdx`}}; !slices.Equal(got, want) {
		t.Errorf("the index lists %+v, want %+v", got, want)
	}

	if err := os.Remove(filepath.Join(s.root, "assets", "Models", "unit.mdx")); err != nil {
		t.Fatal(err)
	}
	s.mustSync(noBlock)
	if len(s.readImports()) != 0 || s.readMapFile("Models/renamed.mdx") != missing || s.readMapFile("unmanaged.txt") != "keep" {
		t.Error("removing the asset left its file or its import, or touched another file")
	}
}

func TestSyncKeepsTheFlagWorldEditorSavedOnAnOwnedImport(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/Textures/a.blp")
	writeFile(t, s.root, "assets/Textures/b.blp")
	s.mustSync(noBlock)
	saved := []imp.Entry{{Flag: 29, Path: `Textures\a.blp`}, {Flag: 29, Path: `Textures\b.blp`}}
	s.writeImports(saved...)
	if _, result := s.mustPlan(noBlock); len(result.Changes) != 0 {
		t.Errorf("a map World Editor saved is changed: %q", changePaths(result.Changes))
	}
	writeFile(t, s.root, "assets/Textures/c.blp")
	s.mustSync(noBlock)
	if got, want := s.readImports(), append(saved, imp.Entry{Flag: imp.CustomPath, Path: `Textures\c.blp`}); !slices.Equal(got, want) {
		t.Errorf("the index lists %+v, want %+v", got, want)
	}
}

func TestSyncWritesNoStateFileWhenItOwnsNothingAndRemovesOneItDoesNotNeed(t *testing.T) {
	s := newAssetProject(t)
	s.mustSync(noBlock)
	if got := s.readState(); got != missing {
		t.Errorf("owning nothing wrote the state file %q", got)
	}
	writeFile(t, s.root, "assets/a.blp")
	s.mustSync(noBlock)
	if s.readState() == missing {
		t.Error("owning a file wrote no state file")
	}
	if err := os.Remove(filepath.Join(s.root, "assets", "a.blp")); err != nil {
		t.Fatal(err)
	}
	s.mustSync(noBlock)
	if s.readMapFile("a.blp") != missing || s.readState() != missing {
		t.Error("the owned file or the state file is there after the asset was removed")
	}
}

func TestTheFilesLibrariesShipAreSyncedLikeTheMapsOwnAndOwned(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/Models/Own.mdx", "own")
	writeFile(t, s.root, "assets/icons/shared.blp", "the map's")
	writeFile(t, s.root, "libraries/ui/war3mapImported/ui/frames.toc", "toc")
	writeFile(t, s.root, "libraries/ui/icons/shared.blp", "the library's")

	result := s.mustSync(noBlock, "ui")
	want := []row{
		{"", "icons/shared.blp", "icons/shared.blp"},
		{"", "Models/Own.mdx", "Models/Own.mdx"},
		{"ui", "war3mapImported/ui/frames.toc", "war3mapImported/ui/frames.toc"},
	}
	if got := rows(result.Assets); !slices.Equal(got, want) {
		t.Fatalf("the assets are %+v, want %+v", got, want)
	}
	if s.readMapFile("war3mapImported/ui/frames.toc") != "toc" || s.readMapFile("icons/shared.blp") != "the map's" {
		t.Error("the map does not hold the library's file and the map's own")
	}
	paths := func() []string {
		var list []string
		for _, entry := range s.readImports() {
			list = append(list, entry.Path)
		}
		return list
	}
	if got := paths(); !slices.Equal(got, []string{`icons\shared.blp`, `Models\Own.mdx`, `war3mapImported\ui\frames.toc`}) {
		t.Errorf("the index lists %q", got)
	}
	if _, again := s.mustPlan(noBlock, "ui"); len(again.Changes) != 0 {
		t.Errorf("a second sync changes %q", changePaths(again.Changes))
	}

	s.mustSync(noBlock)
	if s.readMapFile("war3mapImported/ui/frames.toc") != missing {
		t.Error("the library's file is in the map after the library was dropped")
	}
	if got := paths(); !slices.Equal(got, []string{`icons\shared.blp`, `Models\Own.mdx`}) {
		t.Errorf("the index lists %q", got)
	}
}

func blockWithFile(t *testing.T, s *assetProject, dir string) {
	t.Helper()
	writeFile(t, s.mapDir, dir, "in the way")
}

func TestAFailedSyncUndoesTheWritesItMade(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	writeFile(t, s.root, "assets/Sound/b.blp")
	folder, result := s.mustPlan(noBlock)
	blockWithFile(t, s, "Sound")
	before := testkit.Snapshot(t, s.root)

	e := asDiagError(t, Sync(background, folder, result, s.root, stateName), "a failed sync")
	if !strings.HasPrefix(e.Msg, "Writing assets failed: ") || !strings.HasSuffix(e.Msg, ". Every change was undone.") ||
		e.File != mapLabel+"/Sound/b.blp" || e.Hint == "" || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	s.checkUnchanged(before, "a failed sync")
}

func TestASyncThatCannotWriteOverAnOwnedFileUndoesItsWritesAndLeavesTheFileAsItWas(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/0.blp", "first")
	writeFile(t, s.root, "assets/a.blp", "first")
	s.mustSync(noBlock)
	writeFile(t, s.root, "assets/0.blp", "second")
	writeFile(t, s.root, "assets/a.blp", "second")
	folder, result := s.mustPlan(noBlock)
	before := testkit.Snapshot(t, s.root)
	testkit.MakeUnwritable(t, filepath.Join(s.mapDir, "a.blp"))

	e := asDiagError(t, Sync(background, folder, result, s.root, stateName), "a sync that cannot write over a file")
	if !strings.HasPrefix(e.Msg, "Writing assets failed: ") || !strings.HasSuffix(e.Msg, ". Every change was undone.") ||
		e.File != mapLabel+"/a.blp" || e.Hint == "" || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	s.checkUnchanged(before, "a failed sync")
}

func newSyncedProject(t *testing.T) (*assetProject, *mapdir.Folder, *Result) {
	t.Helper()
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp", "first")
	writeFile(t, s.root, "assets/dropped.blp")
	writeFile(t, s.mapDir, "war3mapImported/existing.wav", "editor")
	s.writeImports(imp.Entry{Flag: 5, Path: "existing.wav"})
	s.mustSync(noBlock)

	writeFile(t, s.root, "assets/a.blp", "second")
	writeFile(t, s.root, "assets/new.blp")
	if err := os.Remove(filepath.Join(s.root, "assets", "dropped.blp")); err != nil {
		t.Fatal(err)
	}
	folder, result := s.mustPlan(noBlock)
	if got, want := changePaths(result.Changes), []string{"a.blp", "new.blp", "-dropped.blp", "war3map.imp"}; !slices.Equal(got, want) {
		t.Fatalf("the changes are %q, want %q", got, want)
	}
	return s, folder, result
}

func (s *assetProject) checkMapChanged() {
	s.t.Helper()
	if s.readMapFile("a.blp") != "second" || s.readMapFile("new.blp") != "asset" || s.readMapFile("dropped.blp") != missing {
		s.t.Error("the map's changes are not made before the state file is written")
	}
}

func TestASyncThatCannotWriteItsStateRestoresTheMapByteForByte(t *testing.T) {
	s, folder, result := newSyncedProject(t)
	before := testkit.Snapshot(t, s.root)
	testkit.MakeUnwritable(t, s.stateFullPath)
	ctx := &cancelAfterCtx{Context: background, limit: never, before: map[int]func(){5: s.checkMapChanged}}

	e := asDiagError(t, Sync(ctx, folder, result, s.root, stateName), "a sync that cannot write its state")
	if !strings.HasPrefix(e.Msg, "Writing assets failed: ") || !strings.HasSuffix(e.Msg, ". Every change was undone.") ||
		e.File != stateName || !strings.Contains(e.Hint, "can be written") || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	if ctx.calls != 5 {
		t.Errorf("the sync asked %d times, want the state file to be its ask 5 and its last", ctx.calls)
	}
	s.checkUnchanged(before, "a failed sync")
}

func TestASyncThatCannotReadItsStateJustBeforeWritingItRestoresTheMapByteForByte(t *testing.T) {
	s, folder, result := newSyncedProject(t)
	before := testkit.Snapshot(t, s.mapDir)
	ctx := &cancelAfterCtx{Context: background, limit: never, before: map[int]func(){5: func() {
		s.checkMapChanged()
		if err := errors.Join(os.Remove(s.stateFullPath), os.Mkdir(s.stateFullPath, 0o777)); err != nil {
			t.Fatal(err)
		}
	}}}

	e := asDiagError(t, Sync(ctx, folder, result, s.root, stateName),
		"a sync that cannot read its state before it writes it")
	if !strings.HasPrefix(e.Msg, "Writing assets failed: ") || !strings.HasSuffix(e.Msg, ". Every change was undone.") ||
		e.File != stateName || !strings.Contains(e.Hint, "a readable file, not a folder") || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	if after := testkit.Snapshot(t, s.mapDir); !maps.EqualFunc(before, after, slices.Equal) {
		t.Errorf("the map holds %q, want what it held before the sync", slices.Sorted(maps.Keys(after)))
	}
}

func TestAnInterruptedSyncWritesNothingAndOneInterruptedMidwayUndoesItsWrites(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	writeFile(t, s.root, "assets/b.blp")
	before := testkit.Snapshot(t, s.root)
	const asks = 4
	for limit := range asks {
		folder, result := s.mustPlan(noBlock)
		ctx := &cancelAfterCtx{Context: background, limit: limit}
		e := asDiagError(t, Sync(ctx, folder, result, s.root, stateName), "an interrupted sync")
		want := "Interrupted; every change was undone."
		if limit == 0 {
			want = "Interrupted; nothing was written."
		}
		if e.Msg != want || ctx.calls != limit+1 {
			t.Errorf("cancelled at ask %d: error = %+v after %d asks, want %q", limit+1, e, ctx.calls, want)
		}
		s.checkUnchanged(before, "an interrupted sync")
	}
	folder, result := s.mustPlan(noBlock)
	ctx := &cancelAfterCtx{Context: background, limit: asks}
	if err := Sync(ctx, folder, result, s.root, stateName); err != nil || ctx.calls != asks {
		t.Errorf("Sync = %v after %d asks, want it to finish after %d", err, ctx.calls, asks)
	}
}

func TestASyncWithNothingToWriteIsNotInterrupted(t *testing.T) {
	s := newAssetProject(t)
	folder, result := s.mustPlan(noBlock)
	ctx := &cancelAfterCtx{Context: background}
	if err := Sync(ctx, folder, result, s.root, stateName); err != nil || ctx.calls != 0 {
		t.Errorf("Sync = %v after %d asks, want no ask where nothing is written", err, ctx.calls)
	}
}

func TestAnUndoThatCannotRestoreAFileNamesTheFailureAndEveryFileItCouldNotRestore(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	writeFile(t, s.root, "assets/b.blp")
	folder, result := s.mustPlan(noBlock)
	ctx := &cancelAfterCtx{Context: background, limit: never, before: map[int]func(){2: func() {
		if err := os.Remove(filepath.Join(s.mapDir, "a.blp")); err != nil {
			t.Fatal(err)
		}
		writeFile(t, s.mapDir, "a.blp/inner.txt")
		writeFile(t, s.mapDir, "b.blp", "another program's")
	}}}

	e := asDiagError(t, Sync(ctx, folder, result, s.root, stateName), "a sync that cannot undo")
	failure := mapLabel + "/b.blp changed after the assets were checked."
	if !strings.HasPrefix(e.Msg, "Writing assets failed ("+failure+"), and these files could not be restored: "+mapLabel+"/a.blp (") ||
		!strings.Contains(e.Hint, "version control") {
		t.Errorf("error = %+v", e)
	}
	if cause, ok := diag.FirstProblem(e.Cause); !ok || cause.Msg != failure {
		t.Errorf("the cause is %v, want the failure that stopped the sync", e.Cause)
	}
	if s.readMapFile("b.blp") != "another program's" || s.readMapFile("war3map.imp") != missing || s.readState() != missing {
		t.Error("the sync wrote past the file it stopped at")
	}
}

func TestAnInterruptedSyncThatCannotUndoSaysSo(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	writeFile(t, s.root, "assets/b.blp")
	folder, result := s.mustPlan(noBlock)
	ctx := &cancelAfterCtx{Context: background, limit: 1, before: map[int]func(){2: func() {
		if err := os.Remove(filepath.Join(s.mapDir, "a.blp")); err != nil {
			t.Fatal(err)
		}
		writeFile(t, s.mapDir, "a.blp/inner.txt")
	}}}
	e := asDiagError(t, Sync(ctx, folder, result, s.root, stateName), "a sync that cannot undo")
	if !strings.HasPrefix(e.Msg, "Writing assets failed (interrupted), and these files could not be restored: "+mapLabel+"/a.blp (") {
		t.Errorf("error = %+v", e)
	}
}

func TestSyncRefusesAFileThatChangedAfterThePlanAndWritesOverNothing(t *testing.T) {
	writes := func(name string, content ...string) func(*assetProject) {
		return func(s *assetProject) { writeFile(s.t, s.mapDir, name, content...) }
	}
	saves := func(s *assetProject) { s.writeImports(imp.Entry{Flag: 5, Path: "sound.wav"}) }
	removes := func(s *assetProject) {
		if err := os.Remove(filepath.Join(s.mapDir, "replaced.blp")); err != nil {
			s.t.Fatal(err)
		}
	}
	tests := []struct {
		name    string
		meddle  func(s *assetProject)
		changed string
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
			s := newAssetProject(t)
			writeFile(t, s.root, "assets/replaced.blp", "first")
			writeFile(t, s.root, "assets/dropped.blp")
			writeFile(t, s.root, "assets/0.blp")
			s.mustSync(noBlock)
			writeFile(t, s.root, "assets/replaced.blp", "second")
			writeFile(t, s.root, "assets/new.blp")
			writeFile(t, s.root, "assets/0.blp", "written before the others")
			if err := os.Remove(filepath.Join(s.root, "assets", "dropped.blp")); err != nil {
				t.Fatal(err)
			}
			folder, result := s.mustPlan(noBlock)
			tt.meddle(s)
			before := testkit.Snapshot(t, s.root)

			e := asDiagError(t, Sync(background, folder, result, s.root, stateName), "a sync of a map that changed")
			file := mapLabel + "/" + tt.changed
			if e.Msg != file+" changed after the assets were checked." || e.File != file || !strings.Contains(e.Hint, "Close World Editor") {
				t.Errorf("error = %+v", e)
			}
			s.checkUnchanged(before, "a refused sync")
		})
	}
}

func TestSyncRefusesAStateFileThatChangedAfterItBeganAndUndoesTheMap(t *testing.T) {
	writes := func(s *assetProject) { writeFile(s.t, s.root, ".asset-state/map.w3x.json", "another program's") }
	removes := func(s *assetProject) {
		if err := os.Remove(s.stateFullPath); err != nil {
			s.t.Fatal(err)
		}
	}
	tests := []struct {
		name    string
		owned   bool
		asset   string
		meddle  func(s *assetProject)
		lastAsk int
		state   string
	}{
		{"changed before it is written", true, "second", writes, 2, "another program's"},
		{"removed before it is written", true, "second", removes, 2, missing},
		{"made before it is written", false, "first", writes, 3, "another program's"},
		{"changed before it is removed", true, "", writes, 3, "another program's"},
		{"removed before it is removed", true, "", removes, 3, missing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newAssetProject(t)
			if tt.owned {
				writeFile(t, s.root, "assets/a.blp", "first")
				s.mustSync(noBlock)
			}
			if err := os.RemoveAll(filepath.Join(s.root, "assets", "a.blp")); err != nil {
				t.Fatal(err)
			}
			if tt.asset != "" {
				writeFile(t, s.root, "assets/a.blp", tt.asset)
			}
			folder, result := s.mustPlan(noBlock)
			before := testkit.Snapshot(t, s.mapDir)
			ctx := &cancelAfterCtx{Context: background, limit: never, before: map[int]func(){tt.lastAsk: func() { tt.meddle(s) }}}

			e := asDiagError(t, Sync(ctx, folder, result, s.root, stateName), "a sync whose state file changed")
			if e.Msg != stateName+" changed after the assets were checked." || e.File != stateName ||
				!strings.Contains(e.Hint, "Close World Editor") {
				t.Errorf("error = %+v", e)
			}
			if ctx.calls != tt.lastAsk {
				t.Errorf("the sync asked %d times, want the state file to be its ask %d and its last", ctx.calls, tt.lastAsk)
			}
			if after := testkit.Snapshot(t, s.mapDir); !maps.EqualFunc(before, after, slices.Equal) {
				t.Errorf("the map holds %q, want what it held before the sync", slices.Sorted(maps.Keys(after)))
			}
			if got := s.readState(); got != tt.state {
				t.Errorf("the state file holds %q, want %q: what the other program left", got, tt.state)
			}
		})
	}
}

func TestAStateFileThatNeedsNoWriteIsNotLookedAtAgain(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	s.mustSync(noBlock)
	s.writeImports()
	folder, result := s.mustPlan(noBlock)
	if got := changePaths(result.Changes); !slices.Equal(got, []string{"war3map.imp"}) {
		t.Fatalf("the changes are %q", got)
	}
	ctx := &cancelAfterCtx{Context: background, limit: never, before: map[int]func(){1: func() {
		writeFile(t, s.root, ".asset-state/map.w3x.json", "another program's")
	}}}
	if err := Sync(ctx, folder, result, s.root, stateName); err != nil || ctx.calls != 1 {
		t.Errorf("Sync = %v after %d asks, want it to finish after the one ask of the index", err, ctx.calls)
	}
	if len(s.readImports()) != 1 || s.readState() != "another program's" {
		t.Error("the index is not written, or the state file is")
	}
}

func TestAStateFileMadeDuringASyncThatOwnsNothingAndFoundNoneIsLeftAsItIs(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	s.mustSync(noBlock)
	if err := os.Remove(filepath.Join(s.root, "assets", "a.blp")); err != nil {
		t.Fatal(err)
	}
	folder, result := s.mustPlan(noBlock)
	if got := changePaths(result.Changes); !slices.Equal(got, []string{"-a.blp", "war3map.imp"}) || len(result.State.Files) != 0 {
		t.Fatalf("the changes are %q and the state owns %d files, want the owned file removed and nothing owned",
			got, len(result.State.Files))
	}
	if err := os.Remove(s.stateFullPath); err != nil {
		t.Fatal(err)
	}
	ctx := &cancelAfterCtx{Context: background, limit: never, before: map[int]func(){2: func() {
		writeFile(t, s.root, ".asset-state/map.w3x.json", "another program's")
	}}}
	if err := Sync(ctx, folder, result, s.root, stateName); err != nil || ctx.calls != 2 {
		t.Errorf("Sync = %v after %d asks, want it to finish after the two asks of the map's changes", err, ctx.calls)
	}
	if s.readMapFile("a.blp") != missing || len(s.readImports()) != 0 {
		t.Error("the map's changes are not made")
	}
	if got := s.readState(); got != "another program's" {
		t.Errorf("the state file holds %q, want what the other program wrote", got)
	}
}

func TestAFileNamedAssetStateOwnsNothingAndStopsASyncByTheStateFilesName(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	testkit.WriteFile(t, s.root, ".asset-state", []byte("a file, not a folder"))
	folder, result := s.mustPlan(noBlock)
	before := testkit.Snapshot(t, s.root)
	e := asDiagError(t, Sync(background, folder, result, s.root, stateName), "a sync with a file named .asset-state")
	if !strings.HasPrefix(e.Msg, "Writing assets failed: ") || e.File != stateName || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
	s.checkUnchanged(before, "a refused sync")
}

func TestSyncRefusesAStateFileItCannotReadBeforeItWritesAnything(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	folder, result := s.mustPlan(noBlock)
	if err := os.MkdirAll(s.stateFullPath, 0o777); err != nil {
		t.Fatal(err)
	}
	before := testkit.Snapshot(t, s.root)
	e := asDiagError(t, Sync(background, folder, result, s.root, stateName),
		"a sync with a folder in place of its state file")
	if !strings.Contains(e.Msg, "Reading the asset ownership state failed") || e.File != stateName {
		t.Errorf("error = %+v", e)
	}
	s.checkUnchanged(before, "a refused sync")
}

func TestSyncWritesNothingOfAPlanWithAnAssetInsideAnother(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	before := testkit.Snapshot(t, s.root)
	folder := s.openMap()
	assets := []Asset{{Target: "a.blp", Data: []byte("a")}, {Target: "data", Data: []byte("outer")},
		{Target: "data/inner.txt", Data: []byte("inner")}}
	result, err := Plan(background, folder, assets, State{})
	if err != nil {
		t.Fatalf("Plan: %v", diag.Format(err))
	}
	err = Sync(background, folder, result, s.root, stateName)
	var expected *diag.Error
	if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "Cannot write data") {
		t.Errorf("Sync = %v, want a plain error about data, a file and a folder at once", err)
	}
	s.checkUnchanged(before, "a refused sync")
}

func TestSyncRefusesAnotherFolderThanTheOneThePlanWasMadeFrom(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp", "first")
	s.mustSync(noBlock)
	writeFile(t, s.root, "assets/a.blp", "second")
	_, result := s.mustPlan(noBlock)
	writeFile(t, s.mapDir, "a.blp", "edited by hand")
	before := testkit.Snapshot(t, s.root)

	err := Sync(background, s.openMap(), result, s.root, stateName)
	var expected *diag.Error
	if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "the folder the plan was made from") {
		t.Errorf("Sync = %v, want a plain error about the folder", err)
	}
	s.checkUnchanged(before, "a refused sync")
}

func TestSyncRefusesAFolderThatCarriesPlannedChanges(t *testing.T) {
	s := newAssetProject(t)
	writeFile(t, s.root, "assets/a.blp")
	writeFile(t, s.mapDir, "war3map.w3i", "the map's own")
	view := s.openMap().WithChanges([]mapdir.Change{{Path: "war3map.w3i", Data: []byte("patched")}})
	assets, _ := mustCollect(t, s.root, noBlock)
	result, err := Plan(background, view, assets, State{})
	if err != nil {
		t.Fatalf("Plan: %v", diag.Format(err))
	}
	if got, want := changePaths(result.Changes), []string{"a.blp", "war3map.imp"}; !slices.Equal(got, want) {
		t.Fatalf("the changes are %q, want %q", got, want)
	}
	before := testkit.Snapshot(t, s.root)

	err = Sync(background, view, result, s.root, stateName)
	var expected *diag.Error
	if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "carries planned changes") {
		t.Errorf("Sync = %v, want a plain error about the folder's planned changes", err)
	}
	s.checkUnchanged(before, "a refused sync")
}
