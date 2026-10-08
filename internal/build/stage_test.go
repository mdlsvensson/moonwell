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

func (s *standIn) mapAt(folder string, blocks ...string) {
	s.t.Helper()
	if folder != "map.w3x" {
		s.folder(filepath.ToSlash(filepath.Dir(filepath.FromSlash("maps/" + folder))))
		if err := os.Rename(s.at("maps/map.w3x"), s.at("maps/"+folder)); err != nil {
			s.t.Fatal(err)
		}
	}
	s.evaluatesTo(append(blocks, `"map":{"folder":"`+folder+`","entry":"src/main.yue"}`)...)
}

func stagedOf(t testing.TB, s *standIn, plan *Result) place {
	t.Helper()
	at, err := stage(s.env, s.project, plan)
	if err != nil {
		t.Fatalf("stage: %v", diag.Format(err))
	}
	return at
}

func TestStageWritesThePlannedMapInPlaceOfAnEarlierStageAndSaysWhatItHolds(t *testing.T) {
	s := newStandIn(t, objectsWith(captain("hfoo")), settingsNamed("Staged"))
	s.templateMap()
	s.put("assets/icons/sword.blp", "own sword")
	s.put("dist/stage/map.w3x/left.txt", "from an earlier build")
	source := testkit.Snapshot(t, s.at("maps"))
	plan := planOf(t, s, Options{})
	at := stagedOf(t, s, plan)
	if at.label != "dist/stage/map.w3x" || at.file != s.at("dist/stage/map.w3x") {
		t.Errorf("staged at %+v", at)
	}
	staged := filesBelow(t, at.file)
	for _, name := range plan.Map.Files() {
		if staged[name] != heldBy(t, plan.Map, name) {
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
	if !reflect.DeepEqual(testkit.Snapshot(t, s.at("maps")), source) {
		t.Error("staging changed the source map")
	}
}

func TestStageOfAMapWithoutObjectsSettingsOrAssetsSaysNothing(t *testing.T) {
	s := newStandIn(t)
	stagedOf(t, s, planOf(t, s, Options{}))
	if lines := s.log.Lines(); len(lines) != 0 {
		t.Errorf("logged %q", lines)
	}
}

func TestStageRefusesAFileOnTheWayToTheStageByItsName(t *testing.T) {
	tests := []struct {
		name   string
		folder string
		file   string
	}{
		{"a file at dist/stage", "map.w3x", "dist/stage"},
		{"a file where a folder of the map's folder goes", "campaign/one.w3x", "dist/stage/campaign"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStandIn(t)
			s.mapAt(tt.folder)
			plan := planOf(t, s, Options{})
			s.remove(tt.file)
			s.put(tt.file, "a file")
			_, err := stage(s.env, s.project, plan)
			e := asError(t, err, tt.name)
			if e.Msg != tt.file+" is a file, not a folder." || e.File != tt.file || e.Hint == "" || e.Cause != nil {
				t.Errorf("error = %+v", e)
			}
			if held, _ := os.ReadFile(s.at(tt.file)); string(held) != "a file" {
				t.Errorf("the file on the way holds %q", held)
			}
		})
	}
}

func TestStageNamesTheStageItCouldNotWriteFromTheProjectFolder(t *testing.T) {
	s := newStandIn(t)
	plan := planOf(t, s, Options{})
	s.remove("maps/map.w3x/war3map.lua")
	s.folder("maps/map.w3x/war3map.lua")
	_, err := stage(s.env, s.project, plan)
	e := asError(t, err, "a stage that cannot be written")
	if !strings.HasPrefix(e.Msg, "Staging the map into dist/stage/map.w3x failed: ") ||
		strings.Contains(e.Msg, s.root) || e.File != "dist/stage/map.w3x/war3map.lua" || e.Cause == nil ||
		e.Hint != "Close Warcraft III or World Editor if they have dist/stage open, then retry." {
		t.Errorf("error = %+v", e)
	}
}

func TestStageRefusesALinkOnTheWayToTheStageByItsStep(t *testing.T) {
	tests := []struct {
		name   string
		link   string
		target string
	}{
		{"dist to the source map", "dist", "maps/map.w3x"},
		{"dist to a folder of the source map", "dist", "maps/map.w3x/war3mapImported"},
		{"dist to the folder the source map is in", "dist", "maps"},
		{"dist/stage to a folder beside the source map", "dist/stage", "maps/other"},
		{"the stage itself to the source map", "dist/stage/map.w3x", "maps/map.w3x"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newStandIn(t)
			s.put("maps/map.w3x/war3mapImported/a.txt", "asset")
			s.put("maps/other/kept.txt", "kept")
			plan := planOf(t, s, Options{})
			s.remove(tt.link)
			testkit.LinkDir(t, s.at(tt.target), s.at(tt.link))
			maps := testkit.Snapshot(t, s.at("maps"))
			_, err := stage(s.env, s.project, plan)
			e := asError(t, err, "a link on the way to the stage")
			if !strings.HasPrefix(e.Msg, tt.link+" is a link: ") || e.File != "dist/stage/map.w3x" || e.Hint == "" {
				t.Errorf("error = %+v", e)
			}
			if !reflect.DeepEqual(testkit.Snapshot(t, s.at("maps")), maps) {
				t.Error("the refused stage changed the maps")
			}
			if lines := s.log.Lines(); len(lines) != 0 {
				t.Errorf("logged %q", lines)
			}
		})
	}
}
