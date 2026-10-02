package mapdir

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
)

func TestKeyIgnoresSeparatorsAndLetterCase(t *testing.T) {
	if got := Key(`War3mapImported\Icon.BLP`); got != "war3mapimported/icon.blp" {
		t.Errorf("Key = %q", got)
	}
	if Key("war3mapSkin.txt") != Key("WAR3MAPSKIN.TXT") {
		t.Error("letter case changed the key")
	}
}

func TestNamesFindsTheSpellingEachWantedFileHas(t *testing.T) {
	entries := []string{"war3map.w3i", "war3mapskin.txt", "WAR3MAP.LUA", "other.txt"}
	wanted := []string{"war3map.w3i", "war3map.lua", "war3mapMisc.txt", "war3mapSkin.txt"}
	names, err := Names(entries, wanted, func(name string) string { return "maps/map.w3x/" + name })
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"war3map.w3i":     "war3map.w3i",
		"war3map.lua":     "WAR3MAP.LUA",
		"war3mapskin.txt": "war3mapskin.txt",
	}
	if len(names) != len(want) {
		t.Errorf("Names = %v", names)
	}
	for key, name := range want {
		if names[key] != name {
			t.Errorf("Names[%q] = %q, want %q", key, names[key], name)
		}
	}
}

func TestNamesRefusesTwoSpellingsOfOneFile(t *testing.T) {
	entries := []string{"war3mapskin.txt", "other.txt", "war3mapSkin.txt"}
	_, err := Names(entries, []string{"war3mapSkin.txt"}, func(name string) string { return "maps/map.w3x/" + name })
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v", err)
	}
	// Sorted, the capital spelling comes first, so the lower-case one is the second that is found and named.
	if e.Msg != "Map files war3mapSkin.txt and war3mapskin.txt differ only in letter case." ||
		e.File != "maps/map.w3x/war3mapskin.txt" || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
	// A file nobody asked for may have two spellings.
	if _, err := Names([]string{"a.txt", "A.txt"}, []string{"war3map.lua"}, nil); err != nil {
		t.Error(err)
	}
}

func TestApplyWritesAndRemovesInOrder(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "war3mapMap.blp"), []byte("minimap"), 0o666); err != nil {
		t.Fatal(err)
	}
	changes := []Change{
		{Name: "war3mapMinimap.blp", Bytes: []byte("minimap")},
		{Name: "war3mapMap.blp", Remove: true},
		{Name: "war3mapMap.tga", Bytes: []byte("picture")},
	}
	if err := Apply(dir, changes, "Writing staged map settings failed."); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "war3mapMap.blp")); !os.IsNotExist(err) {
		t.Error("the removed file is still there")
	}
	for name, want := range map[string]string{"war3mapMinimap.blp": "minimap", "war3mapMap.tga": "picture"} {
		if got, _ := os.ReadFile(filepath.Join(dir, name)); string(got) != want {
			t.Errorf("%s holds %q", name, got)
		}
	}
}

func TestApplyReportsTheFileItCouldNotWrite(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "war3map.w3u"), 0o777); err != nil {
		t.Fatal(err)
	}
	err := Apply(dir, []Change{{Name: "war3map.w3u", Bytes: []byte{1}}}, "Writing staged object data failed.")
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v", err)
	}
	if e.Msg != "Writing staged object data failed." || e.File != filepath.Join(dir, "war3map.w3u") ||
		e.Cause == nil || e.Hint != "Close Warcraft III or World Editor if they have the staged map open, then rebuild." {
		t.Errorf("error = %+v", e)
	}
}
