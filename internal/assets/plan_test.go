package assets_test

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

var background = context.Background()

// project makes a project with an empty source map and an assets folder.
func project(t *testing.T) (root, mapDir, state string) {
	t.Helper()
	root = t.TempDir()
	mapDir, state, err := assets.Locations(root, "map.w3x")
	if err != nil {
		t.Fatal(err)
	}
	os.MkdirAll(mapDir, 0o777)
	os.Mkdir(filepath.Join(root, "assets"), 0o777)
	return root, mapDir, state
}

func planFor(t *testing.T, root, mapDir, state string, c assets.Config, libraries ...string) *assets.Plan {
	t.Helper()
	plan, err := assets.PlanAssets(background, root, mapDir, state, c, libraries)
	if err != nil {
		t.Fatalf("PlanAssets: %v", diag.Format(err))
	}
	return plan
}

// sync plans and applies, as assets:sync does.
func sync(t *testing.T, root, mapDir, state string, c assets.Config, libraries ...string) *assets.Plan {
	t.Helper()
	plan := planFor(t, root, mapDir, state, c, libraries...)
	if err := assets.ApplyPlan(background, plan, state); err != nil {
		t.Fatalf("ApplyPlan: %v", diag.Format(err))
	}
	return plan
}

func refusedPlan(t *testing.T, root, mapDir, state string, c assets.Config, libraries ...string) *diag.Error {
	t.Helper()
	_, err := assets.PlanAssets(background, root, mapDir, state, c, libraries)
	return asError(t, err, "a refused plan")
}

func importsOf(t *testing.T, mapDir string) []assets.Import {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(mapDir, "war3map.imp"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := assets.ReadImports(data, "war3map.imp")
	if err != nil {
		t.Fatal(err)
	}
	return entries
}

func importPaths(t *testing.T, mapDir string) []string {
	var paths []string
	for _, entry := range importsOf(t, mapDir) {
		paths = append(paths, entry.Path)
	}
	slices.Sort(paths)
	return paths
}

func textOf(t *testing.T, dir, file string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(file)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// countdown is a context that reads as cancelled from its nth check on.
type countdown struct {
	context.Context
	checks, limit int
}

func (c *countdown) Err() error {
	c.checks++
	if c.checks > c.limit {
		return context.Canceled
	}
	return nil
}

func (c *countdown) Deadline() (time.Time, bool) { return time.Time{}, false }

func cancelled() context.Context {
	ctx, cancel := context.WithCancel(background)
	cancel()
	return ctx
}

func TestLocationsPlacesTheStateFileUnderAssetState(t *testing.T) {
	root := t.TempDir()
	mapDir, state, err := assets.Locations(root, "map.w3x")
	if err != nil || mapDir != filepath.Join(root, "maps", "map.w3x") || state != filepath.Join(root, ".asset-state", "map.w3x.json") {
		t.Errorf("Locations = %q, %q, %v", mapDir, state, err)
	}
}

func TestSyncImportsMappedAssetsAndKeepsTheEditorsOwnImports(t *testing.T) {
	root, mapDir, state := project(t)
	const icon = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	const disabled = "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp"
	put(t, root, "assets/"+icon)
	put(t, root, "assets/icons/disabled.blp", "disabled")
	put(t, mapDir, "war3mapImported/existing.wav", "editor")
	os.WriteFile(filepath.Join(mapDir, "war3map.imp"), assets.WriteImports([]assets.Import{{Flag: 5, Path: "existing.wav"}}), 0o666)
	c := config(nil, "icons/disabled.blp", disabled)

	plan := planFor(t, root, mapDir, state, c)
	if len(plan.Assets) != 2 || fsx.Exists(state) {
		t.Fatalf("%d assets; planning wrote the state: %v", len(plan.Assets), fsx.Exists(state))
	}
	if err := assets.ApplyPlan(background, plan, state); err != nil {
		t.Fatal(err)
	}
	want := []assets.Import{{Flag: 5, Path: "existing.wav"}}
	for _, asset := range plan.Assets {
		want = append(want, assets.Import{Flag: 13, Path: strings.ReplaceAll(asset.Target, "/", `\`)})
	}
	if got := importsOf(t, mapDir); !slices.Equal(got, want) {
		t.Errorf("imports = %+v", got)
	}
	if textOf(t, mapDir, disabled) != "disabled" {
		t.Error("the mapped asset is not in the map")
	}
	if again := planFor(t, root, mapDir, state, c); len(again.Changes) != 0 {
		t.Errorf("a second sync changes %d files", len(again.Changes))
	}
	wantState := "{\n  \"version\": 1,\n  \"files\": {\n    \"" + icon + "\": \"" + plan.Assets[0].Hash + "\",\n    \"" +
		disabled + "\": \"" + plan.Assets[1].Hash + "\"\n  }\n}\n"
	if got := textOf(t, filepath.Dir(state), filepath.Base(state)); got != wantState {
		t.Errorf("the state file is\n%s\nwant\n%s", got, wantState)
	}
}

func TestSyncUpdatesRenamesAndDeletesOnlyOwnedFilesAndAStagedCopyLeavesSourceStateAlone(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, "assets/Models/unit.mdx", "first")
	put(t, mapDir, "unmanaged.txt", "keep")
	sync(t, root, mapDir, state, defaults)
	previousState := textOf(t, filepath.Dir(state), filepath.Base(state))

	put(t, root, "assets/Models/unit.mdx", "second")
	staged := filepath.Join(root, "stage")
	if err := fsx.CopyTree(mapDir, staged); err != nil {
		t.Fatal(err)
	}
	if err := assets.ApplyPlan(background, planFor(t, root, staged, state, defaults), ""); err != nil {
		t.Fatal(err)
	}
	if textOf(t, staged, "Models/unit.mdx") != "second" || textOf(t, mapDir, "Models/unit.mdx") != "first" ||
		textOf(t, filepath.Dir(state), filepath.Base(state)) != previousState {
		t.Error("a build changed the source map or its state")
	}

	sync(t, root, mapDir, state, defaults)
	sync(t, root, mapDir, state, config(nil, "Models/unit.mdx", "Models/renamed.mdx"))
	if fsx.Exists(filepath.Join(mapDir, "Models", "unit.mdx")) || textOf(t, mapDir, "Models/renamed.mdx") != "second" {
		t.Error("the asset was not renamed")
	}

	os.Remove(filepath.Join(root, "assets", "Models", "unit.mdx"))
	sync(t, root, mapDir, state, defaults)
	if len(importsOf(t, mapDir)) != 0 || textOf(t, mapDir, "unmanaged.txt") != "keep" {
		t.Error("removing the asset left an import, or touched another file")
	}
}

func TestSyncKeepsTheFlagWorldEditorSavedOnAnOwnedImport(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, "assets/Textures/a.blp")
	put(t, root, "assets/Textures/b.blp")
	sync(t, root, mapDir, state, defaults)
	// World Editor 3.00 rewrites flag 13 as 29 when it saves the map.
	saved := []assets.Import{{Flag: 29, Path: `Textures\a.blp`}, {Flag: 29, Path: `Textures\b.blp`}}
	os.WriteFile(filepath.Join(mapDir, "war3map.imp"), assets.WriteImports(saved), 0o666)
	if plan := planFor(t, root, mapDir, state, defaults); len(plan.Changes) != 0 {
		t.Errorf("a save by World Editor is a change: %+v", plan.Changes)
	}
	put(t, root, "assets/Textures/c.blp")
	sync(t, root, mapDir, state, defaults)
	if got := importsOf(t, mapDir); !slices.Equal(got, append(saved, assets.Import{Flag: 13, Path: `Textures\c.blp`})) {
		t.Errorf("imports = %+v", got)
	}
}

func TestConflictsAndEditedOwnedFilesFailBeforeAnythingChanges(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, "assets/a.blp")
	put(t, mapDir, "a.blp", "editor owned")
	e := refusedPlan(t, root, mapDir, state, defaults)
	if e.Msg != "Asset a.blp conflicts with a file or import already in the map." ||
		e.Hint != "Import it under another path with assets.paths, or remove the map's own copy." ||
		textOf(t, mapDir, "a.blp") != "editor owned" {
		t.Errorf("error = %+v", e)
	}

	os.Remove(filepath.Join(mapDir, "a.blp"))
	sync(t, root, mapDir, state, defaults)
	put(t, mapDir, "a.blp", "manual edit")
	modified := refusedPlan(t, root, mapDir, state, defaults)
	if modified.Msg != "a.blp was modified in the map after assets:sync wrote it." ||
		modified.File != filepath.Join(mapDir, "a.blp") || !strings.Contains(modified.Hint, "source map") {
		t.Errorf("error = %+v", modified)
	}
	os.Remove(filepath.Join(root, "assets", "a.blp"))
	if e := refusedPlan(t, root, mapDir, state, defaults); !strings.Contains(e.Msg, "modified") {
		t.Errorf("error = %+v", e)
	}
	if textOf(t, mapDir, "a.blp") != "manual edit" {
		t.Error("a refused plan changed the file")
	}
}

func TestAnAssetWhoseFolderIsAFileInTheMapIsRejected(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, "assets/a.blp")
	put(t, mapDir, "Textures", "file")
	e := refusedPlan(t, root, mapDir, state, config(nil, "a.blp", "textures/a.blp"))
	if e.Msg != "Textures in the map is a file, not a directory, so textures/a.blp cannot go there." {
		t.Errorf("error = %+v", e)
	}
}

func TestExistingFolderSpellingIsReusedAndForgedStateCannotTargetMapInternals(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, mapDir, "Textures/existing.blp")
	put(t, root, "assets/textures/new.blp")
	if plan := planFor(t, root, mapDir, state, defaults); plan.Assets[0].Target != "Textures/new.blp" {
		t.Errorf("the target is %q", plan.Assets[0].Target)
	}
	os.MkdirAll(filepath.Dir(state), 0o777)
	os.WriteFile(state, []byte(`{"version":1,"files":{"war3map.lua":"`+strings.Repeat("0", 64)+`"}}`), 0o666)
	reserved := refusedPlan(t, root, mapDir, state, defaults)
	if reserved.Msg != "The asset ownership state is invalid: Reserved map path: war3map.lua" || reserved.File != state ||
		!strings.HasPrefix(reserved.Hint, "Restore it from version control.") {
		t.Errorf("error = %+v", reserved)
	}
	for content, problem := range map[string]string{
		`not json`:                              "it is not JSON",
		`{"version":2,"files":{}}`:              "version must be 1",
		`null`:                                  "version must be 1",
		`{"version":1,"files":[]}`:              "files must be an object",
		`{"version":1,"files":{"a.blp":"abc"}}`: "a.blp has no valid hash",
		`{"version":1,"files":{"a.blp":"` + strings.Repeat("0", 64) + `","A.BLP":"` + strings.Repeat("0", 64) + `"}}`: "A.BLP is listed twice",
	} {
		os.WriteFile(state, []byte(content), 0o666)
		if e := refusedPlan(t, root, mapDir, state, defaults); e.Msg != "The asset ownership state is invalid: "+problem+"." {
			t.Errorf("state %s: %q", content, e.Msg)
		}
	}
}

func TestSyncWritesNoStateFileWhenItOwnsNothingAndRemovesOneItNoLongerNeeds(t *testing.T) {
	root, mapDir, state := project(t)
	sync(t, root, mapDir, state, defaults)
	if fsx.Exists(state) {
		t.Error("owning nothing wrote a state file")
	}
	put(t, root, "assets/a.blp")
	sync(t, root, mapDir, state, defaults)
	if !fsx.Exists(state) {
		t.Error("owning a file wrote no state file")
	}
	os.Remove(filepath.Join(root, "assets", "a.blp"))
	sync(t, root, mapDir, state, defaults)
	if fsx.Exists(filepath.Join(mapDir, "a.blp")) || fsx.Exists(state) {
		t.Error("the owned file or the state file is still there")
	}
}

func TestNewFoldersPlannedInOneRunShareOneSpelling(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, "assets/a.blp")
	put(t, root, "assets/b.blp")
	plan := planFor(t, root, mapDir, state, config(nil, "a.blp", "Textures/a.blp", "b.blp", "textures/b.blp"))
	if got := rows(plan.Assets); got[0].target != "Textures/a.blp" || got[1].target != "Textures/b.blp" {
		t.Errorf("targets = %+v", got)
	}
}

func TestAFailedSyncUndoesTheWritesItAlreadyMade(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, "assets/a.blp")
	put(t, root, "assets/b.blp")
	plan := planFor(t, root, mapDir, state, defaults)
	// b.blp turning into a folder after planning makes the second write fail.
	os.Mkdir(filepath.Join(mapDir, "b.blp"), 0o777)
	asError(t, assets.ApplyPlan(background, plan, state), "a failed sync")
	if fsx.Exists(filepath.Join(mapDir, "a.blp")) || fsx.Exists(filepath.Join(mapDir, "war3map.imp")) || fsx.Exists(state) ||
		!fsx.IsDir(filepath.Join(mapDir, "b.blp")) {
		t.Error("the failed sync left its writes behind")
	}
}

func TestAnInterruptedSyncChangesNothingAndOneInterruptedMidwayUndoesItsWrites(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, "assets/a.blp")
	put(t, root, "assets/b.blp")
	plan := planFor(t, root, mapDir, state, defaults)

	before := asError(t, assets.ApplyPlan(cancelled(), plan, state), "an interrupted sync")
	if before.Msg != "Interrupted; nothing was written." || fsx.Exists(filepath.Join(mapDir, "a.blp")) {
		t.Errorf("error = %+v", before)
	}
	// The context reads as cancelled from the second change on, after a.blp was written.
	midway := &countdown{Context: background, limit: 1}
	e := asError(t, assets.ApplyPlan(midway, plan, state), "a sync interrupted midway")
	if e.Msg != "Interrupted; every change was undone." || midway.checks < 2 {
		t.Errorf("error = %+v after %d checks", e, midway.checks)
	}
	if fsx.Exists(filepath.Join(mapDir, "a.blp")) || fsx.Exists(filepath.Join(mapDir, "war3map.imp")) || fsx.Exists(state) {
		t.Error("the interrupted sync left its writes behind")
	}
}

func TestPlanningStopsAtAnInterruptBeforeAnythingIsWritten(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, "assets/a.blp")
	put(t, root, "assets/b.blp")
	_, err := assets.PlanAssets(cancelled(), root, mapDir, state, defaults, nil)
	if e := asError(t, err, "an interrupted plan"); e.Msg != "Interrupted; nothing was written." {
		t.Errorf("error = %+v", e)
	}
	// The context reads as cancelled from the second asset on, so planning stops midway.
	midway := &countdown{Context: background, limit: 1}
	_, err = assets.PlanAssets(midway, root, mapDir, state, defaults, nil)
	asError(t, err, "a plan interrupted midway")
	if midway.checks < 2 || fsx.Exists(filepath.Join(mapDir, "a.blp")) || fsx.Exists(state) {
		t.Errorf("%d checks; planning wrote something", midway.checks)
	}
}

func TestAnIncompleteRollbackNamesTheFailureAndEveryFileItCouldNotRestore(t *testing.T) {
	_, mapDir, _ := project(t)
	old := filepath.Join(mapDir, "old.blp")
	put(t, mapDir, "old.blp", "old")
	// Deleting old.blp and then writing old.blp/inner.blp makes old.blp a folder, so it cannot be restored as a
	// file.
	plan := &assets.Plan{
		Changes: []assets.FileChange{
			{File: old, Before: []byte("old"), Existed: true, Remove: true},
			{File: filepath.Join(old, "inner.blp"), After: []byte("new")},
			{File: filepath.Join(mapDir, "gone.blp"), Before: []byte("missing"), Existed: true, After: []byte("new")},
		},
		State: assets.State{Version: 1},
	}
	e := asError(t, assets.ApplyPlan(background, plan, ""), "an incomplete rollback")
	if !strings.Contains(e.Msg, "changed after the assets were checked") ||
		!strings.Contains(e.Msg, "could not be restored: "+old+" (") || !strings.Contains(e.Hint, "version control") {
		t.Errorf("error = %+v", e)
	}
	if _, ok := diag.First(e.Cause); !ok {
		t.Errorf("the cause is %v", e.Cause)
	}
}

func TestWithNoAssetsAndNothingOwnedWar3mapImpIsLeftUntouched(t *testing.T) {
	root, mapDir, state := project(t)
	if plan := planFor(t, root, mapDir, state, defaults); len(plan.Changes) != 0 || fsx.Exists(filepath.Join(mapDir, "war3map.imp")) {
		t.Errorf("changes = %+v", plan.Changes)
	}
	// A build never even reads war3map.imp then.
	os.WriteFile(filepath.Join(mapDir, "war3map.imp"), []byte{9, 9}, 0o666)
	plan := planFor(t, root, mapDir, state, defaults)
	if len(plan.Assets) != 0 || len(plan.Replaced) != 0 || len(plan.Changes) != 0 || plan.State.Version != 1 || plan.State.Files.Len() != 0 {
		t.Errorf("plan = %+v", plan)
	}
}

func TestAFailedSyncRestoresTheFilesItOverwroteByteForByte(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, "assets/a.blp", "first")
	put(t, mapDir, "war3mapImported/existing.wav", "editor")
	os.WriteFile(filepath.Join(mapDir, "war3map.imp"), assets.WriteImports([]assets.Import{{Flag: 5, Path: "existing.wav"}}), 0o666)
	sync(t, root, mapDir, state, defaults)
	read := func(path string) []byte {
		data, _ := os.ReadFile(path)
		return data
	}
	assetBefore, impBefore, stateBefore := read(filepath.Join(mapDir, "a.blp")), read(filepath.Join(mapDir, "war3map.imp")), read(state)

	put(t, root, "assets/a.blp", "second")
	put(t, root, "assets/b.blp")
	plan := planFor(t, root, mapDir, state, defaults)
	var existed []bool
	for _, change := range plan.Changes {
		existed = append(existed, change.Existed)
	}
	if !slices.Equal(existed, []bool{true, false, true}) {
		t.Fatalf("changes existed before: %v", existed)
	}
	// b.blp turning into a folder after planning makes the second write fail, after a.blp was overwritten.
	os.Mkdir(filepath.Join(mapDir, "b.blp"), 0o777)
	asError(t, assets.ApplyPlan(background, plan, state), "a failed sync")
	if !bytes.Equal(read(filepath.Join(mapDir, "a.blp")), assetBefore) ||
		!bytes.Equal(read(filepath.Join(mapDir, "war3map.imp")), impBefore) || !bytes.Equal(read(state), stateBefore) {
		t.Error("the failed sync did not restore what it overwrote")
	}
}

func TestTheFilesLibrariesShipArePlannedLikeTheMapsOwnAndOwnedByASync(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, "assets/Models/Own.mdx", "own")
	put(t, root, "assets/icons/shared.blp", "the map's")
	put(t, root, ".moonwell/library-assets/ui/war3mapImported/ui/frames.toc", "toc")
	put(t, root, ".moonwell/library-assets/ui/icons/shared.blp", "the library's")
	put(t, root, ".moonwell/library-assets/unlisted/never.txt", "not in the manifest")

	plan := planFor(t, root, mapDir, state, defaults, "ui")
	want := []row{
		{"", "icons/shared.blp", "icons/shared.blp"},
		{"", "Models/Own.mdx", "Models/Own.mdx"},
		{"ui", "war3mapImported/ui/frames.toc", "war3mapImported/ui/frames.toc"},
	}
	if got := rows(plan.Assets); !slices.Equal(got, want) ||
		!slices.Equal(plan.Replaced, []string{"assets/icons/shared.blp replaces library ui's icons/shared.blp"}) {
		t.Fatalf("assets = %+v, replaced %q", got, plan.Replaced)
	}
	if err := assets.ApplyPlan(background, plan, state); err != nil {
		t.Fatal(err)
	}
	if textOf(t, mapDir, "war3mapImported/ui/frames.toc") != "toc" || textOf(t, mapDir, "icons/shared.blp") != "the map's" {
		t.Error("the map does not hold the library's file and the map's own")
	}
	if got := importPaths(t, mapDir); !slices.Equal(got, []string{`Models\Own.mdx`, `icons\shared.blp`, `war3mapImported\ui\frames.toc`}) {
		t.Errorf("imports = %q", got)
	}
	if again := planFor(t, root, mapDir, state, defaults, "ui"); len(again.Changes) != 0 {
		t.Errorf("a second sync changes %d files", len(again.Changes))
	}

	// Without the library, the next sync deletes the file it owned.
	without := sync(t, root, mapDir, state, defaults)
	if len(without.Replaced) != 0 || fsx.Exists(filepath.Join(mapDir, "war3mapImported", "ui", "frames.toc")) {
		t.Error("the library's file is still in the map")
	}
	if got := importPaths(t, mapDir); !slices.Equal(got, []string{`Models\Own.mdx`, `icons\shared.blp`}) {
		t.Errorf("imports = %q", got)
	}
}

func TestALibraryFileThatClashesWithTheMapsOwnCopyNamesTheLibrary(t *testing.T) {
	root, mapDir, state := project(t)
	put(t, root, ".moonwell/library-assets/ui/Models/Golem.mdx", "library")
	put(t, mapDir, "Models/Golem.mdx", "editor")
	e := refusedPlan(t, root, mapDir, state, defaults, "ui")
	if e.Msg != "Asset Models/Golem.mdx of library ui conflicts with a file or import already in the map." ||
		e.Hint != "Remove the map's own copy in World Editor's Import Manager." {
		t.Errorf("error = %+v", e)
	}
}

func TestAMissingMapFolderNamesTheManifest(t *testing.T) {
	root := t.TempDir()
	put(t, root, "assets/a.blp")
	mapDir, state, _ := assets.Locations(root, "map.w3x")
	e := refusedPlan(t, root, mapDir, state, defaults)
	if e.Msg != "The map folder "+mapDir+" does not exist." || e.File != "moonwell.pkl" || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
}
