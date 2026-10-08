package testkit

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func plantedExport(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	WriteFile(t, root, "War3.W3Mod/Units/UnitData.SLK", []byte("a table"))
	WriteFile(t, root, "War3.W3Mod/Units/abilitydata.slk", nil)
	WriteFile(t, root, "War3.W3Mod/Units/Notes.TXT", nil)
	WriteFile(t, root, "War3.W3Mod/_Locales/enUS.w3mod/UI/WorldEditStrings.txt", nil)
	return root
}

func TestNeedExportFindsEachNameWithoutRegardToLetterCase(t *testing.T) {
	root := plantedExport(t)
	t.Setenv("MOONWELL_GAME_DATA", root)
	t.Setenv("MOONWELL_REQUIRE_EXPORTS", "1")
	export := NeedExport(t, "MOONWELL_GAME_DATA")
	if got := export.Path(); got != root {
		t.Errorf("the export is %q, want the folder the variable names, %q", got, root)
	}
	want := filepath.Join(root, "War3.W3Mod", "Units", "UnitData.SLK")
	if got := export.Path("war3.w3mod", "UNITS", "unitdata.slk"); got != want {
		t.Errorf("the table is %q, want %q", got, want)
	}
	data, err := os.ReadFile(export.Path("WAR3.W3MOD", "units", "UnitData.slk"))
	if err != nil || string(data) != "a table" {
		t.Errorf("the table holds %q, %v", data, err)
	}
	want = filepath.Join(root, "War3.W3Mod", "_Locales", "enUS.w3mod", "UI", "WorldEditStrings.txt")
	if got := export.Path("war3.w3mod", "_locales", "enus.w3mod", "ui", "worldeditstrings.txt"); got != want {
		t.Errorf("the editor's strings are %q, want %q", got, want)
	}
	wantEntries := []string{"Notes.TXT", "UnitData.SLK", "abilitydata.slk"}
	if got := export.Entries("war3.w3mod", "units"); !slices.Equal(got, wantEntries) {
		t.Errorf("the entries are %q, want %q", got, wantEntries)
	}
	if got := export.Entries(); !slices.Equal(got, []string{"War3.W3Mod"}) {
		t.Errorf("the entries of the export are %q", got)
	}
}

func TestNeedExportTakesAVariableThatNamesAFile(t *testing.T) {
	list := WriteFile(t, t.TempDir(), "Listfile.txt", []byte("units\\unitdata.slk\n"))
	t.Setenv("MOONWELL_GAME_LISTFILE", list)
	if got := NeedExport(t, "MOONWELL_GAME_LISTFILE").Path(); got != list {
		t.Errorf("the export is %q, want %q", got, list)
	}
}

func TestNeedExportSkipsOrFailsTheTestWithoutTheGamesFiles(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone")
	tests := []struct {
		name         string
		value        string
		require      string
		skips, fails int
		naming       []string
	}{
		{name: "no variable", skips: 1, naming: []string{"MOONWELL_GAME_DATA is not set"}},
		{name: "no variable, another value than 1", require: "0", skips: 1},
		{name: "no variable, the game's files required", require: "1", fails: 1,
			naming: []string{"MOONWELL_GAME_DATA is not set", "MOONWELL_REQUIRE_EXPORTS=1"}},
		{name: "a variable that names nothing", value: gone, fails: 1, naming: []string{"MOONWELL_GAME_DATA", gone}},
		{name: "a variable that names nothing, the game's files required", value: gone, require: "1", fails: 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MOONWELL_GAME_DATA", tc.value)
			t.Setenv("MOONWELL_REQUIRE_EXPORTS", tc.require)
			stand := newStandIn(t)
			export := NeedExport(stand, "MOONWELL_GAME_DATA")
			if len(stand.skipped) != tc.skips || len(stand.failed) != tc.fails {
				t.Fatalf("skipped %q and failed %q, want %d and %d", stand.skipped, stand.failed, tc.skips, tc.fails)
			}
			for _, words := range tc.naming {
				if said := strings.Join(append(stand.skipped, stand.failed...), ""); !strings.Contains(said, words) {
					t.Errorf("it said %q, want it to name %q", said, words)
				}
			}
			if path, entries := export.Path("war3.w3mod"), export.Entries(); path != "" || entries != nil {
				t.Errorf("the path is %q and the entries %q, want none", path, entries)
			}
			if len(stand.skipped) != tc.skips || len(stand.failed) != tc.fails {
				t.Errorf("after asking: skipped %q and failed %q", stand.skipped, stand.failed)
			}
		})
	}
}

func TestAnExportFailsTheTestForANameItCannotTellApart(t *testing.T) {
	root := plantedExport(t)
	t.Setenv("MOONWELL_GAME_DATA", root)
	units := filepath.Join(root, "War3.W3Mod", "Units")
	type lookup struct {
		name   string
		below  []string
		naming []string
	}
	tests := []lookup{
		{"a name no folder has", []string{"war3.w3mod", "units", "missing.slk"}, []string{units, "missing.slk", "0"}},
		{"a folder no folder has", []string{"war3.w3mod", "doodads", "doodads.slk"}, []string{"doodads", "0"}},
		{"a file where a folder should be", []string{"war3.w3mod", "units", "notes.txt", "x"}, []string{"Notes.TXT"}},
	}
	if CaseSensitive(t, root) {
		WriteFile(t, root, "War3.W3Mod/Units/UNITDATA.slk", nil)
		twice := []string{"war3.w3mod", "units", "unitdata.slk"}
		tests = append(tests, lookup{"a name two entries have", twice, []string{units, "unitdata.slk", "2"}})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stand := newStandIn(t)
			path := NeedExport(stand, "MOONWELL_GAME_DATA").Path(tc.below...)
			if path != "" || len(stand.failed) != 1 || len(stand.skipped) != 0 {
				t.Fatalf("the path is %q, with the failures %q, want none and one", path, stand.failed)
			}
			for _, words := range tc.naming {
				if !strings.Contains(stand.failed[0], words) {
					t.Errorf("the failure is %q, want it to name %q", stand.failed[0], words)
				}
			}
		})
	}
	stand := newStandIn(t)
	entries := NeedExport(stand, "MOONWELL_GAME_DATA").Entries("war3.w3mod", "units", "abilitydata.slk")
	if entries != nil || len(stand.failed) != 1 || !strings.Contains(stand.failed[0], "abilitydata.slk") {
		t.Errorf("the entries of a file are %q, with the failures %q", entries, stand.failed)
	}
}
