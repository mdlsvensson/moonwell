package build

import (
	"fmt"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/editor"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/library"
	"github.com/mdlsvensson/moonwell/internal/objects"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestRefreshDeclarationsWritesTheDeclarationsAndTheMacroModuleAndNoIDsModule(t *testing.T) {
	s := newStandIn(t, objectsWith(captain("hfoo")))
	s.templateMap()
	source, err := Source(s.project)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	globals, err := MapGlobals(source)
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	objs, err := objects.Plan(source, s.project.Objects, objects.LoadMetadata())
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	if err := RefreshDeclarations(s.root, source, objs.Objects, globals); err != nil {
		t.Fatal(diag.Format(err))
	}
	for _, file := range []string{
		editor.TypesDir + "/natives.d.lua", editor.TypesDir + "/moonwell.d.lua", editor.TypesDir + "/objects.d.lua",
		editor.TypesDir + "/map.d.lua", script.MacrosFile,
	} {
		if !fsx.Exists(s.at(file)) {
			t.Errorf("%s is not written", file)
		}
	}
	ofObjects, _ := os.ReadFile(s.at(editor.TypesDir + "/objects.d.lua"))
	ofMap, _ := os.ReadFile(s.at(editor.TypesDir + "/map.d.lua"))
	if !strings.Contains(string(ofObjects), "captain") || !strings.Contains(string(ofMap), "gg_trg_Initialization") ||
		!strings.Contains(string(ofMap), "maps/map.w3x/war3map.lua") {
		t.Errorf("the declarations hold\n%s\n%s", ofObjects, ofMap)
	}
	if fsx.Exists(s.at(objects.IDsFile)) || fsx.Exists(s.at("src/generated")) {
		t.Errorf("the door wrote %s", objects.IDsFile)
	}
	if runs := s.ranSoFar(); len(runs) != 0 {
		t.Errorf("the door ran %+v", runs)
	}
}

func TestPlanAssetsPlansTheImportIntoTheFolderItIsGivenAgainstWhatTheStateOwns(t *testing.T) {
	s := newStandIn(t)
	s.put("assets/icons/sword.blp", "own sword")
	s.put(".moonwell/library-assets/kit/icons/Sword.blp", "kit sword")
	s.put(".moonwell/library-assets/kit/kit/axe.blp", "kit axe")
	s.put("maps/map.w3x/icons/old.blp", "an asset of an earlier sync")
	s.put("maps/map.w3x/icons/kept.blp", "a file World Editor imported")
	s.put(".asset-state/map.w3x.json", "{\n  \"version\": 1,\n  \"files\": {\n    \"icons/old.blp\": \""+
		fsx.SHA256Hex([]byte("an asset of an earlier sync"))+"\"\n  }\n}\n")
	before := testkit.Snapshot(t, s.root)
	source := openedSource(t, s)
	plan, replaced, err := PlanAssets(background, source, s.project, []library.Synced{syncedLibrary("kit", true)})
	if err != nil {
		t.Fatal(diag.Format(err))
	}
	imported := []string{
		"icons/sword.blp from icons/sword.blp of : own sword", "kit/axe.blp from kit/axe.blp of kit: kit axe",
	}
	if got := describedAssets(plan.Assets); !slices.Equal(got, imported) {
		t.Errorf("the plan imports %q, want %q", got, imported)
	}
	if want := []string{"assets/icons/sword.blp replaces library kit's icons/Sword.blp"}; !slices.Equal(replaced, want) {
		t.Errorf("replaced = %q, want %q", replaced, want)
	}
	var changes []string
	for _, change := range plan.Changes {
		changes = append(changes, fmt.Sprint(change.Name, " removed: ", change.Remove))
	}
	slices.Sort(changes)
	changed := []string{
		"icons/old.blp removed: true", "icons/sword.blp removed: false", "kit/axe.blp removed: false",
		"war3map.imp removed: false",
	}
	if !slices.Equal(changes, changed) {
		t.Errorf("the plan changes %q, want %q", changes, changed)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, s.root), before) {
		t.Error("planning the import changed the project")
	}
	if lines := s.log.Lines(); len(lines) != 0 {
		t.Errorf("the plan logged %q", lines)
	}
}
