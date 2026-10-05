package build

import (
	"os"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/editor"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/objects"
	"github.com/mdlsvensson/moonwell/next/internal/script"
)

// The door is a command's that compiles nothing, such as setup: it runs no program, and the ids module, which
// the gameplay imports, is a build's to write.
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
