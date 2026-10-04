package mapdir

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// label is how the folders of these tests are named in errors.
const label = "maps/map.w3x"

// write fills a new folder with files, each named with "/" and holding its text.
func write(t *testing.T, files map[string]string) string {
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

// open writes a folder with files and opens it.
func open(t *testing.T, files map[string]string) (folder *Folder, dir string) {
	t.Helper()
	dir = write(t, files)
	folder, err := Open(dir, label)
	if err != nil {
		t.Fatal(err)
	}
	return folder, dir
}

// read is what the folder holds under name, or "<missing>" when it has no such file.
func read(t *testing.T, folder *Folder, name string) string {
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

// linkAway replaces the folder Textures of the map at dir with a link to a folder outside the map, which holds an
// Old.blp of its own, and returns that folder. It stands for a link made after the map was scanned.
func linkAway(t *testing.T, dir string) (outside string) {
	t.Helper()
	outside = filepath.Join(filepath.Dir(dir), "outside")
	testkit.WriteFile(t, outside, "Old.blp", []byte("outside the map"))
	if err := os.RemoveAll(filepath.Join(dir, "Textures")); err != nil {
		t.Fatal(err)
	}
	testkit.LinkDir(t, outside, filepath.Join(dir, "Textures"))
	return outside
}

func asError(t *testing.T, err error) *diag.Error {
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
	folder, dir := open(t, map[string]string{
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
		if got := folder.Has(c.asked); got != c.has {
			t.Errorf("Has(%q) = %v", c.asked, got)
		}
		if got := folder.Name(c.asked); got != c.name {
			t.Errorf("Name(%q) = %q, want %q", c.asked, got, c.name)
		}
		if got := folder.Label(c.asked); got != label+"/"+c.name {
			t.Errorf("Label(%q) = %q, want %q", c.asked, got, label+"/"+c.name)
		}
		// On a file system that tells letter cases apart, only the folder's own scan can find these.
		if got := read(t, folder, c.asked); got != c.content {
			t.Errorf("Read(%q) = %q, want %q", c.asked, got, c.content)
		}
	}
	if got := folder.Label(""); got != label {
		t.Errorf(`Label("") = %q, want the label alone`, got)
	}
	if folder.Dir() != dir {
		t.Errorf("Dir = %q, want %q", folder.Dir(), dir)
	}
}

func TestAFolderOfTheMapIsNotAFile(t *testing.T) {
	folder, _ := open(t, map[string]string{"Textures/Icon.blp": "icon"})
	if folder.Has("Textures") {
		t.Error("Has finds the folder Textures as a file")
	}
	if got := read(t, folder, "textures"); got != "<missing>" {
		t.Errorf("Read of a folder = %q", got)
	}
}

func TestReadNamesTheFileItCannotRead(t *testing.T) {
	folder, dir := open(t, map[string]string{"war3map.w3i": "info"})
	// A folder where the scan saw a file: reading it fails on every system.
	file := filepath.Join(dir, "war3map.w3i")
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(file, 0o777); err != nil {
		t.Fatal(err)
	}
	data, found, err := folder.Read("WAR3MAP.W3I")
	e := asError(t, err)
	if data != nil || found {
		t.Errorf("Read = %q, %v beside its error", data, found)
	}
	if e.File != label+"/war3map.w3i" || !contains(e.Msg, "Reading a map file failed") || e.Cause == nil ||
		!contains(e.Hint, "locked") {
		t.Errorf("error = %+v", e)
	}
}

func TestReadDoesNotFollowALinkMadeAfterTheScan(t *testing.T) {
	folder, dir := open(t, map[string]string{"war3map.w3i": "info", "Textures/Old.blp": "old"})
	linkAway(t, dir)
	data, found, err := folder.Read("textures/old.blp")
	e := asError(t, err)
	if data != nil || found {
		t.Errorf("Read = %q, %v: it read through the link", data, found)
	}
	if e.File != label+"/Textures/Old.blp" || !contains(e.Msg, "Symlinks") {
		t.Errorf("error = %+v", e)
	}
}
