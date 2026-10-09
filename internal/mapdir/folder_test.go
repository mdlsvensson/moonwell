package mapdir

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

const mapDisplayPath = "maps/map.w3x"

func writeFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "map.w3x")
	if err := os.MkdirAll(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	for name, content := range files {
		testkit.WriteFile(t, dir, name, []byte(content))
	}
	return dir
}

func openFolder(t *testing.T, files map[string]string) (folder *Folder, dir string) {
	t.Helper()
	dir = writeFiles(t, files)
	folder, err := Open(dir, mapDisplayPath)
	if err != nil {
		t.Fatal(err)
	}
	return folder, dir
}

func readFile(t *testing.T, folder *Folder, name string) string {
	t.Helper()
	data, found, err := folder.Read(name)
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		return "<missing>"
	}
	return string(data)
}

func contains(text, words string) bool { return strings.Contains(text, words) }

func symlinkTexturesOutside(t *testing.T, dir string) (outside string) {
	t.Helper()
	outside = filepath.Join(filepath.Dir(dir), "outside")
	testkit.WriteFile(t, outside, "Old.blp", []byte("outside the map"))
	if err := os.RemoveAll(filepath.Join(dir, "Textures")); err != nil {
		t.Fatal(err)
	}
	testkit.LinkDir(t, outside, filepath.Join(dir, "Textures"))
	return outside
}

func asDiagError(t *testing.T, err error) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("got %v, want a *diag.Error", err)
	}
	return e
}

func TestKeyIgnoresSeparatorsAndLetterCase(t *testing.T) {
	cases := []struct{ path, want string }{
		{`War3mapImported\Icon.BLP`, "war3mapimported/icon.blp"},
		{"war3mapSkin.txt", "war3mapskin.txt"},
		{"WAR3MAPSKIN.TXT", "war3mapskin.txt"},
		{"textures/old.blp", "textures/old.blp"},
		{"", ""},
	}
	for _, c := range cases {
		if got := Key(c.path); got != c.want {
			t.Errorf("Key(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

func TestAFileIsFoundUnderTheSpellingItHas(t *testing.T) {
	folder, dir := openFolder(t, map[string]string{
		"war3map.w3i":       "info",
		"war3mapskin.txt":   "skin",
		"WAR3MAP.LUA":       "script",
		"Textures/Icon.blp": "icon",
	})
	cases := []struct {
		asked, name, content string
		has                  bool
	}{
		{"war3map.w3i", "war3map.w3i", "info", true},
		{"war3mapSkin.txt", "war3mapskin.txt", "skin", true},
		{"war3map.lua", "WAR3MAP.LUA", "script", true},
		{`textures\icon.BLP`, "Textures/Icon.blp", "icon", true},
		{"war3mapMisc.txt", "war3mapMisc.txt", "<missing>", false},
		{`textures\Other.blp`, `textures\Other.blp`, "<missing>", false},
	}
	for _, c := range cases {
		if got := folder.HasFile(c.asked); got != c.has {
			t.Errorf("Has(%q) = %v", c.asked, got)
		}
		if got := folder.CanonicalPath(c.asked); got != c.name {
			t.Errorf("Name(%q) = %q, want %q", c.asked, got, c.name)
		}
		if got := folder.DisplayPath(c.asked); got != mapDisplayPath+"/"+c.name {
			t.Errorf("Label(%q) = %q, want %q", c.asked, got, mapDisplayPath+"/"+c.name)
		}
		if got := readFile(t, folder, c.asked); got != c.content {
			t.Errorf("Read(%q) = %q, want %q", c.asked, got, c.content)
		}
	}
	if got := folder.DisplayPath(""); got != mapDisplayPath {
		t.Errorf(`Label("") = %q, want the label alone`, got)
	}
	if folder.Dir() != dir {
		t.Errorf("Dir = %q, want %q", folder.Dir(), dir)
	}
}

func TestAFolderOfTheMapIsNotAFile(t *testing.T) {
	folder, _ := openFolder(t, map[string]string{"Textures/Icon.blp": "icon"})
	if folder.HasFile("Textures") {
		t.Error("Has finds the folder Textures as a file")
	}
	if got := readFile(t, folder, "textures"); got != "<missing>" {
		t.Errorf("Read of a folder = %q", got)
	}
}

func TestAFolderIsNamedAsItIsSpelled(t *testing.T) {
	folder, _ := openFolder(t, map[string]string{"Textures/Old.blp": "old", "Units/Hero/a.txt": "", "WAR3MAP.LUA": "script"})
	planned := folder.WithChanges([]Change{newWrite("Sound/Music/theme.mp3", "theme"), newRemoval("units/hero/A.TXT")})
	takenBack := planned.WithChanges([]Change{newRemoval("sound/music/theme.mp3")})
	cases := []struct {
		what        string
		view        *Folder
		asked, name string
	}{
		{"a folder of the map", folder, "textures", "Textures"},
		{"a folder below a folder, with a backslash", folder, `UNITS\hero`, "Units/Hero"},
		{"a folder a planned file makes", planned, "sound", "Sound"},
		{"and the folder below it", planned, "SOUND/music", "Sound/Music"},
		{"a folder of the map whose one file the view removes", planned, "units/HERO", "Units/Hero"},
		{"a file of the map", folder, "war3map.lua", "WAR3MAP.LUA"},
		{"a file the view removes", planned, "UNITS/HERO/a.txt", "Units/Hero/a.txt"},
		{"a name the map does not have", folder, "sound", "sound"},
		{"a new file below a folder of the map", folder, "textures/New.blp", "textures/New.blp"},
		{"a planned folder whose one write is taken back", takenBack, "sound/MUSIC", "sound/MUSIC"},
	}
	for _, c := range cases {
		if got := c.view.CanonicalPath(c.asked); got != c.name {
			t.Errorf("%s: Name(%q) = %q, want %q", c.what, c.asked, got, c.name)
		}
		if got := c.view.DisplayPath(c.asked); got != mapDisplayPath+"/"+c.name {
			t.Errorf("%s: Label(%q) = %q, want %q", c.what, c.asked, got, mapDisplayPath+"/"+c.name)
		}
	}
}

func TestReadNamesTheFileItCannotRead(t *testing.T) {
	folder, dir := openFolder(t, map[string]string{"war3map.w3i": "info"})
	file := filepath.Join(dir, "war3map.w3i")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0o777); err != nil {
		t.Fatal(err)
	}
	data, found, err := folder.Read("WAR3MAP.W3I")
	e := asDiagError(t, err)
	if data != nil || found {
		t.Errorf("Read = %q, %v beside its error", data, found)
	}
	if e.File != mapDisplayPath+"/war3map.w3i" || !contains(e.Msg, "Reading a map file failed") || e.Cause == nil ||
		!contains(e.Hint, "locked") {
		t.Errorf("error = %+v", e)
	}
}

func TestReadDoesNotFollowALinkMadeAfterTheScan(t *testing.T) {
	folder, dir := openFolder(t, map[string]string{"war3map.w3i": "info", "Textures/Old.blp": "old"})
	symlinkTexturesOutside(t, dir)
	data, found, err := folder.Read("textures/old.blp")
	e := asDiagError(t, err)
	if data != nil || found {
		t.Errorf("Read = %q, %v: it read through the link", data, found)
	}
	if e.File != mapDisplayPath+"/Textures/Old.blp" || !contains(e.Msg, "Symlinks") {
		t.Errorf("error = %+v", e)
	}
}
