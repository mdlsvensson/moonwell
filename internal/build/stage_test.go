package build

import (
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func (s *fakeProject) setMapDir(dir string, blocks ...string) {
	s.t.Helper()
	if dir != "map.w3x" {
		s.makeDir(filepath.ToSlash(filepath.Dir(filepath.FromSlash("maps/" + dir))))
		if err := os.Rename(s.fullPath("maps/map.w3x"), s.fullPath("maps/"+dir)); err != nil {
			s.t.Fatal(err)
		}
	}
	s.setManifest(append(blocks, `"map":{"folder":"`+dir+`","entry":"src/main.yue"}`)...)
}

func mustStage(t testing.TB, s *fakeProject, plan *Result) outputFile {
	t.Helper()
	at, err := stage(s.env, s.project, plan)
	if err != nil {
		t.Fatalf("stage: %v", diag.Format(err))
	}
	return at
}

func TestStageWritesThePlannedMapInPlaceOfAnEarlierStageAndSaysWhatItHolds(t *testing.T) {
	s := newFakeProject(t, objectsWith(captain("hfoo")), settingsNamed("Staged"))
	s.copyTemplateMap()
	s.writeFile("assets/icons/sword.blp", "own sword")
	s.writeFile("dist/stage/map.w3x/left.txt", "from an earlier build")
	source := testkit.Snapshot(t, s.fullPath("maps"))
	plan := mustPlan(t, s, Options{})
	at := mustStage(t, s, plan)
	if at.displayPath != "dist/stage/map.w3x" || at.fullPath != s.fullPath("dist/stage/map.w3x") {
		t.Errorf("staged at %+v", at)
	}
	staged := filesBelow(t, at.fullPath)
	for _, name := range plan.Map.Files() {
		if staged[name] != readViewFile(t, plan.Map, name) {
			t.Errorf("the stage's %s is not the planned one", name)
		}
	}
	if _, left := staged["left.txt"]; left || len(staged) != len(plan.Map.Files()) {
		t.Errorf("the stage holds %q, want the planned %q", slices.Sorted(maps.Keys(staged)), plan.Map.Files())
	}
	want := []string{
		"Added 1 custom object(s) to 2 file(s).",
		"Applied map settings to 2 internal file(s).",
		"Imported 1 asset(s).",
	}
	if lines := s.log.Lines(); !slices.Equal(lines, want) {
		t.Errorf("logged %q, want %q", lines, want)
	}
	if !reflect.DeepEqual(testkit.Snapshot(t, s.fullPath("maps")), source) {
		t.Error("staging changed the source map")
	}
}

func TestStageOfAMapWithoutObjectsSettingsOrAssetsSaysNothing(t *testing.T) {
	s := newFakeProject(t)
	mustStage(t, s, mustPlan(t, s, Options{}))
	if lines := s.log.Lines(); len(lines) != 0 {
		t.Errorf("logged %q", lines)
	}
}

func TestStageRefusesAFileOnTheWayToTheStageByItsName(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		file string
	}{
		{"a file at dist/stage", "map.w3x", "dist/stage"},
		{"a file where a folder of the map's folder goes", "campaign/one.w3x", "dist/stage/campaign"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t)
			s.setMapDir(tt.dir)
			plan := mustPlan(t, s, Options{})
			s.removeFile(tt.file)
			s.writeFile(tt.file, "a file")
			_, err := stage(s.env, s.project, plan)
			e := asDiagError(t, err, tt.name)
			if e.Msg != tt.file+" is a file, not a folder." || e.File != tt.file || e.Hint == "" || e.Cause != nil {
				t.Errorf("error = %+v", e)
			}
			if held, _ := os.ReadFile(s.fullPath(tt.file)); string(held) != "a file" {
				t.Errorf("the file on the way holds %q", held)
			}
		})
	}
}

func TestStageNamesTheStageItCouldNotWriteFromTheProjectFolder(t *testing.T) {
	s := newFakeProject(t)
	plan := mustPlan(t, s, Options{})
	s.removeFile("maps/map.w3x/war3map.lua")
	s.makeDir("maps/map.w3x/war3map.lua")
	_, err := stage(s.env, s.project, plan)
	e := asDiagError(t, err, "a stage that cannot be written")
	if !strings.HasPrefix(e.Msg, "Staging the map into dist/stage/map.w3x failed: ") ||
		strings.Contains(e.Msg, s.root) || e.File != "dist/stage/map.w3x/war3map.lua" || e.Cause == nil ||
		e.Hint != "Close Warcraft III or World Editor if they have dist/stage open, then retry." {
		t.Errorf("error = %+v", e)
	}
}

func TestStageRefusesALinkOnTheWayToTheStageByItsStep(t *testing.T) {
	tests := []struct {
		name    string
		symlink string
		target  string
	}{
		{"dist to the source map", "dist", "maps/map.w3x"},
		{"dist to a folder of the source map", "dist", "maps/map.w3x/war3mapImported"},
		{"dist to the folder the source map is in", "dist", "maps"},
		{"dist/stage to a folder beside the source map", "dist/stage", "maps/other"},
		{"the stage itself to the source map", "dist/stage/map.w3x", "maps/map.w3x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newFakeProject(t)
			s.writeFile("maps/map.w3x/war3mapImported/a.txt", "asset")
			s.writeFile("maps/other/kept.txt", "kept")
			plan := mustPlan(t, s, Options{})
			s.removeFile(tt.symlink)
			testkit.LinkDir(t, s.fullPath(tt.target), s.fullPath(tt.symlink))
			maps := testkit.Snapshot(t, s.fullPath("maps"))
			_, err := stage(s.env, s.project, plan)
			e := asDiagError(t, err, "a link on the way to the stage")
			if !strings.HasPrefix(e.Msg, tt.symlink+" is a link: ") || e.File != "dist/stage/map.w3x" || e.Hint == "" {
				t.Errorf("error = %+v", e)
			}
			if !reflect.DeepEqual(testkit.Snapshot(t, s.fullPath("maps")), maps) {
				t.Error("the refused stage changed the maps")
			}
			if lines := s.log.Lines(); len(lines) != 0 {
				t.Errorf("logged %q", lines)
			}
		})
	}
}
