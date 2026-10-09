package mapdir

import (
	"cmp"
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func TestOpenListsEachFoldersEntriesInOrder(t *testing.T) {
	folder, _ := openFolder(t, map[string]string{
		"b.txt": "", "a/z.txt": "", "a/B.txt": "", "c/d/e.txt": "", "a.txt": "", "Z.txt": "",
	})
	want := []string{"Z.txt", "a/B.txt", "a/z.txt", "a.txt", "b.txt", "c/d/e.txt"}
	if got := folder.Files(); !slices.Equal(got, want) {
		t.Errorf("Files = %q, want %q", got, want)
	}
}

func TestOpenOfAnEmptyFolderHasNoFiles(t *testing.T) {
	folder, _ := openFolder(t, nil)
	if got := folder.Files(); len(got) != 0 {
		t.Errorf("Files = %q", got)
	}
}

func TestOpenRefusesTwoSpellingsOfOnePath(t *testing.T) {
	if !testkit.IsCaseSensitive(t, t.TempDir()) {
		t.Skip("this file system cannot hold two names that differ only in letter case")
	}
	cases := []struct {
		name          string
		files         []string
		first, second string
	}{
		{"two files", []string{"war3mapskin.txt", "other.txt", "war3mapSkin.txt"}, "war3mapSkin.txt", "war3mapskin.txt"},
		{"a file nothing asks for", []string{"a.txt", "A.txt"}, "A.txt", "a.txt"},
		{"two folders", []string{"textures/a.blp", "Textures/b.blp"}, "Textures", "textures"},
		{"a folder and a file", []string{"icons", "Icons/a.blp"}, "Icons", "icons"},
		{"below a folder", []string{"Units/hero.mdx", "Units/Hero.mdx"}, "Units/Hero.mdx", "Units/hero.mdx"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			files := map[string]string{}
			for _, name := range c.files {
				files[name] = ""
			}
			_, err := Open(writeFiles(t, files), mapDisplayPath)
			e := asDiagError(t, err)
			if !contains(e.Msg, "differ only in letter case") || !contains(e.Msg, c.first+" and "+c.second) {
				t.Errorf("message = %q, want it to name %s and %s", e.Msg, c.first, c.second)
			}
			if e.File != mapDisplayPath+"/"+c.second || !contains(e.Hint, "source map") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestOpenRefusesALinkInsideTheMap(t *testing.T) {
	dir := writeFiles(t, map[string]string{"war3map.w3i": "info", "Textures/Icon.blp": "icon"})
	outside := filepath.Join(filepath.Dir(dir), "outside")
	testkit.WriteFile(t, outside, "stray.blp", nil)
	testkit.LinkDir(t, outside, filepath.Join(dir, "Textures", "linked"))
	_, err := Open(dir, mapDisplayPath)
	e := asDiagError(t, err)
	if !contains(e.Msg, "Symlinks") || !contains(e.Msg, mapDisplayPath+"/Textures/linked") || e.Hint == "" ||
		e.File != mapDisplayPath+"/Textures/linked" {
		t.Errorf("error = %+v", e)
	}
}

func TestOpenRefusesAFolderThatIsALink(t *testing.T) {
	dir := writeFiles(t, map[string]string{"war3map.w3i": "info"})
	symlink := filepath.Join(filepath.Dir(dir), "linked.w3x")
	testkit.LinkDir(t, dir, symlink)
	_, err := Open(symlink, mapDisplayPath)
	if e := asDiagError(t, err); !contains(e.Msg, "Symlinks") || !contains(e.Msg, mapDisplayPath) || e.File != mapDisplayPath {
		t.Errorf("error = %+v", e)
	}
}

var invalidNames = []struct{ why, name string }{
	{"a backslash", `Textures\Icon.blp`},
	{"a control character", "a\tb.txt"},
	{"a question mark", "what?.blp"},
	{"an asterisk", "any*.blp"},
	{"a colon", "c:icon.blp"},
	{"a quote", `"icon".blp`},
	{"an angle bracket", "<icon>.blp"},
	{"a bar", "a|b.blp"},
	{"a dot at the end", "icon."},
	{"a space at the end", "icon.blp "},
	{"a device", "con"},
	{"a device with an extension", "NUL.txt"},
	{"a numbered device", "Com1.blp"},
}

type namedEntry struct {
	fs.DirEntry
	name string
}

func (n namedEntry) Name() string { return n.name }

func TestTheScanRefusesAnEntryWithANameWindowsCannotHold(t *testing.T) {
	for _, c := range invalidNames {
		for _, below := range []string{"", "Units/Hero"} {
			t.Run(c.why+" in "+cmp.Or(below, "the top folder"), func(t *testing.T) {
				w := walker{displayPath: mapDisplayPath, index: &diskIndex{}}
				path := joinPath(below, c.name)
				e := asDiagError(t, w.addEntry(path, namedEntry{name: c.name}))
				if !contains(e.Msg, "cannot be used in a map") || !contains(e.Msg, "Windows") || !contains(e.Msg, path) ||
					e.File != mapDisplayPath+"/"+path || !contains(e.Hint, "source map") {
					t.Errorf("error = %+v, want it at %s", e, mapDisplayPath+"/"+path)
				}
			})
		}
	}
}

func TestANameWindowsCanHoldIsUsable(t *testing.T) {
	for _, name := range []string{"war3map.w3i", "Icon 1.blp", ".hidden", "..b", "console.txt", "h\xC3\xA9ro.mdx", "a.b.c"} {
		if !isValidName(name) {
			t.Errorf("usable(%q) is false", name)
		}
	}
}

func TestOpenRefusesANameWindowsCannotHold(t *testing.T) {
	if filepath.Separator == '\\' {
		t.Skip("this system cannot hold such names")
	}
	for _, c := range invalidNames {
		for _, at := range []struct{ how, file, entry string }{
			{"a file", c.name, c.name},
			{"a file below a folder", "Units/" + c.name, "Units/" + c.name},
			{"a folder", c.name + "/theme.mp3", c.name},
		} {
			t.Run(c.why+" in the name of "+at.how, func(t *testing.T) {
				dir := writeFiles(t, map[string]string{"war3map.w3i": "info"})
				testkit.WriteFile(t, dir, at.file, nil)
				_, err := Open(dir, mapDisplayPath)
				e := asDiagError(t, err)
				if !contains(e.Msg, "cannot be used in a map") || !contains(e.Msg, at.entry) ||
					e.File != mapDisplayPath+"/"+at.entry || e.Hint == "" {
					t.Errorf("error = %+v, want it at %s", e, mapDisplayPath+"/"+at.entry)
				}
			})
		}
	}
}

func TestOpenRefusesWhatIsNeitherAFileNorAFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a socket's file made on Windows could not be removed again, so the test would leave it behind")
	}
	dir := writeFiles(t, map[string]string{"war3map.w3i": "info"})
	t.Chdir(dir)
	listener, err := net.Listen("unix", "socket")
	if err != nil {
		t.Skipf("cannot make a socket here: %v", err)
	}
	defer listener.Close()
	_, err = Open(dir, mapDisplayPath)
	if e := asDiagError(t, err); !contains(e.Msg, "not a regular file") || e.File != mapDisplayPath+"/socket" || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
}

func TestOpenOfAMissingFolderIsNotExist(t *testing.T) {
	folder, err := Open(filepath.Join(t.TempDir(), "map.w3x"), mapDisplayPath)
	if folder != nil || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open = %v, %v, want an error that is fs.ErrNotExist", folder, err)
	}
}

func TestOpenRefusesAFileWhereTheFolderShouldBe(t *testing.T) {
	file := filepath.Join(t.TempDir(), "map.w3x")
	if err := os.WriteFile(file, []byte("an archive"), 0o666); err != nil {
		t.Fatal(err)
	}
	_, err := Open(file, mapDisplayPath)
	e := asDiagError(t, err)
	if !contains(e.Msg, "is not a folder") || !contains(e.Msg, mapDisplayPath) || e.File != mapDisplayPath ||
		!contains(e.Hint, "folder format") {
		t.Errorf("error = %+v", e)
	}
}
