package mapdir

import (
	"errors"
	"io/fs"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

func TestOpenListsEachFoldersEntriesInOrder(t *testing.T) {
	folder, _ := open(t, map[string]string{
		"b.txt": "", "a/z.txt": "", "a/B.txt": "", "c/d/e.txt": "", "a.txt": "", "Z.txt": "",
	})
	// Names are ordered byte by byte, so a capital comes first; a folder is entered where it stands among them.
	want := []string{"Z.txt", "a/B.txt", "a/z.txt", "a.txt", "b.txt", "c/d/e.txt"}
	if got := folder.Files(); !slices.Equal(got, want) {
		t.Errorf("Files = %q, want %q", got, want)
	}
}

func TestOpenOfAnEmptyFolderHasNoFiles(t *testing.T) {
	folder, _ := open(t, nil)
	if got := folder.Files(); len(got) != 0 {
		t.Errorf("Files = %q", got)
	}
}

func TestOpenRefusesTwoSpellingsOfOnePath(t *testing.T) {
	if !testkit.CaseSensitive(t, t.TempDir()) {
		t.Skip("this file system cannot hold two names that differ only in letter case")
	}
	// Ordered byte by byte, the spelling with the capital is met first, so the other is the one the error is at.
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
			_, err := Open(write(t, files), label)
			e := asError(t, err)
			if !contains(e.Msg, "differ only in letter case") || !contains(e.Msg, c.first+" and "+c.second) {
				t.Errorf("message = %q, want it to name %s and %s", e.Msg, c.first, c.second)
			}
			if e.File != label+"/"+c.second || !contains(e.Hint, "source map") {
				t.Errorf("error = %+v", e)
			}
		})
	}
}

func TestOpenRefusesALinkInsideTheMap(t *testing.T) {
	dir := write(t, map[string]string{"war3map.w3i": "info", "Textures/Icon.blp": "icon"})
	outside := filepath.Join(filepath.Dir(dir), "outside")
	testkit.WriteFile(t, outside, "stray.blp", nil)
	testkit.LinkDir(t, outside, filepath.Join(dir, "Textures", "linked"))
	_, err := Open(dir, label)
	if e := asError(t, err); !contains(e.Msg, "Symlinks") || !contains(e.Msg, label+"/Textures/linked") || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
}

func TestOpenRefusesAFolderThatIsALink(t *testing.T) {
	dir := write(t, map[string]string{"war3map.w3i": "info"})
	link := filepath.Join(filepath.Dir(dir), "linked.w3x")
	testkit.LinkDir(t, dir, link)
	_, err := Open(link, label)
	if e := asError(t, err); !contains(e.Msg, "Symlinks") || !contains(e.Msg, label) {
		t.Errorf("error = %+v", e)
	}
}

func TestOpenRefusesWhatIsNeitherAFileNorAFolder(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("a socket's file made on Windows could not be removed again, so the test would leave it behind")
	}
	dir := write(t, map[string]string{"war3map.w3i": "info"})
	// A socket is such an entry that needs no privilege. Its path must be short, so it is made from inside the folder.
	t.Chdir(dir)
	listener, err := net.Listen("unix", "socket")
	if err != nil {
		t.Skipf("cannot make a socket here: %v", err)
	}
	defer listener.Close()
	_, err = Open(dir, label)
	if e := asError(t, err); !contains(e.Msg, "not a regular file") || e.File != label+"/socket" || e.Hint == "" {
		t.Errorf("error = %+v", e)
	}
}

func TestOpenOfAMissingFolderIsNotExist(t *testing.T) {
	folder, err := Open(filepath.Join(t.TempDir(), "map.w3x"), label)
	if folder != nil || !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Open = %v, %v, want an error that is fs.ErrNotExist", folder, err)
	}
}

func TestOpenRefusesAFileWhereTheFolderShouldBe(t *testing.T) {
	file := filepath.Join(t.TempDir(), "map.w3x")
	if err := os.WriteFile(file, []byte("an archive"), 0o666); err != nil {
		t.Fatal(err)
	}
	_, err := Open(file, label)
	e := asError(t, err)
	if !contains(e.Msg, "is not a folder") || !contains(e.Msg, label) || e.File != label ||
		!contains(e.Hint, "folder format") {
		t.Errorf("error = %+v", e)
	}
}
