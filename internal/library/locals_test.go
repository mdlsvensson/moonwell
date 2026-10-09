package library

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func localDirs(locals []Local) []string {
	var dirs []string
	for _, local := range locals {
		for _, folder := range local.Dirs {
			dirs = append(dirs, folder.Dir)
		}
	}
	return dirs
}

func localDisplayPaths(locals []Local) []string {
	var labels []string
	for _, local := range locals {
		for _, folder := range local.Dirs {
			labels = append(labels, folder.DisplayPath)
		}
	}
	return labels
}

func TestLocalsListsTheFoldersOfLocalLibrariesOnly(t *testing.T) {
	root := t.TempDir()
	mine := filepath.Join(filepath.Dir(root), "mine")
	locals := Locals(root, librariesOf("mine", localLibrary("../mine", "src"), "remote", githubLibrary("v1", "")))
	want := []Local{{Key: "mine", Dir: mine, Dirs: []LocalDir{{filepath.Join(mine, "src"), "../mine/src/"}}}}
	if !slices.EqualFunc(locals, want, sameLocal) {
		t.Errorf("Locals = %+v, want %+v", locals, want)
	}
	if got := Locals(root, librariesOf("remote", githubLibrary("v1", ""))); got != nil {
		t.Errorf("Locals of a project without a local library = %+v", got)
	}
	if got := Locals(root, nil); got != nil {
		t.Errorf("Locals of a project without libraries = %+v", got)
	}
}

func sameLocal(a, b Local) bool {
	return a.Key == b.Key && a.Dir == b.Dir && slices.Equal(a.Dirs, b.Dirs)
}

func TestLocalsTakesTheModuleAndAssetsFoldersFromALibrarysOwnFile(t *testing.T) {
	root := t.TempDir()
	writeTestFiles(t, root,
		"described/moonwell-library.json", `{"dir":"src","assets":"art"}`,
		"broken/moonwell-library.json", "{",
		"rooted/moonwell-library.json", "{}",
	)
	locals := Locals(root, librariesOf(
		"a", localLibrary("described", ""), "b", localLibrary("described", "lua"), "c", localLibrary("broken", ""),
		"d", localLibrary("rooted", ""),
	))
	wantDirs := []string{
		filepath.Join(root, "described", "src"),
		filepath.Join(root, "described", "art"),
		filepath.Join(root, "described", "lua"),
		filepath.Join(root, "described", "art"),
		filepath.Join(root, "broken"),
		filepath.Join(root, "rooted"),
	}
	if got := localDirs(locals); !slices.Equal(got, wantDirs) {
		t.Errorf("the folders are %q, want %q", got, wantDirs)
	}
	wantLabels := []string{"described/src/", "described/art/", "described/lua/", "described/art/", "broken/", "rooted/"}
	if got := localDisplayPaths(locals); !slices.Equal(got, wantLabels) {
		t.Errorf("the labels are %q, want %q", got, wantLabels)
	}
	var keys, dirs []string
	for _, local := range locals {
		keys, dirs = append(keys, local.Key), append(dirs, local.Dir)
	}
	wantOwn := []string{
		filepath.Join(root, "described"), filepath.Join(root, "described"), filepath.Join(root, "broken"),
		filepath.Join(root, "rooted"),
	}
	if !slices.Equal(keys, []string{"a", "b", "c", "d"}) || !slices.Equal(dirs, wantOwn) {
		t.Errorf("the libraries are %q at %q", keys, dirs)
	}
}

func TestLocalsAreInTheOrderOfTheirKeysBytes(t *testing.T) {
	root := t.TempDir()
	locals := Locals(root, librariesOf(
		"b", localLibrary("one", ""), "a", localLibrary("two", ""), "_x", localLibrary("three", ""),
		"B", localLibrary("four", ""), "a-1", localLibrary("five", ""), "0", localLibrary("six", ""),
	))
	var keys []string
	for _, local := range locals {
		keys = append(keys, local.Key)
	}
	if want := []string{"0", "B", "_x", "a", "a-1", "b"}; !slices.Equal(keys, want) {
		t.Errorf("the keys are in the order %q, want %q", keys, want)
	}
}

func TestLocalsTakesALibrarysFileThatTheSyncRefusesForNone(t *testing.T) {
	cases := []struct {
		name string
		file string
		text string
		held bool
	}{
		{"not JSON", File, "{", false},
		{"no object", File, `["src"]`, false},
		{"an unknown key", File, `{"dir":"src","assets":"art","more":1}`, false},
		{"a dir outside the library", File, `{"dir":"../src","assets":"art"}`, false},
		{"assets that are no folder inside the library", File, `{"dir":"src","assets":""}`, false},
		{"a file that cannot be read", File, `{"dir":"src","assets":"art"}`, true},
		{"a folder in the file's place", File + "/inside.txt", "a file of the folder", false},
	}
	for _, c := range cases {
		root := t.TempDir()
		writeTestFiles(t, root, "lib/"+c.file, c.text, "lib/src/a.lua", "return 1", "lib/art/x.blp", "x")
		if c.held {
			testkit.MakeUnreadable(t, filepath.Join(root, "lib", File))
		}
		libraries := librariesOf("mine", localLibrary("lib", ""))
		lib := filepath.Join(root, "lib")
		want := []Local{{Key: "mine", Dir: lib, Dirs: []LocalDir{{lib, "lib/"}}}}
		if got := Locals(root, libraries); !slices.EqualFunc(got, want, sameLocal) {
			t.Errorf("%s: Locals = %+v, want %+v", c.name, got, want)
		}
		want[0].Dirs = []LocalDir{{filepath.Join(lib, "src"), "lib/src/"}}
		if got := Locals(root, librariesOf("mine", localLibrary("lib", "src"))); !slices.EqualFunc(got, want, sameLocal) {
			t.Errorf("%s, with a dir in the manifest: Locals = %+v, want %+v", c.name, got, want)
		}
		mustFailSync(t, root, libraries, nil, c.name)
	}
}

func TestALocalsFolderIsWrittenAsTheManifestAndTheLibrarysFileWriteIt(t *testing.T) {
	root := t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), "kit")
	writeTestFiles(t, elsewhere, "moonwell-library.json", `{"dir":"src/modules","assets":"art"}`)
	cases := []struct {
		name      string
		path, dir string
		own       string
		want      []LocalDir
	}{
		{"the project folder", ".", "", root, []LocalDir{{root, "./"}}},
		{"a folder that is not there", "nowhere/lib", "", filepath.Join(root, "nowhere", "lib"),
			[]LocalDir{{filepath.Join(root, "nowhere", "lib"), "nowhere/lib/"}}},
		{"a path that ends in a separator", "lib/", "src/", filepath.Join(root, "lib"),
			[]LocalDir{{filepath.Join(root, "lib", "src"), "lib/src/"}}},
		{"a path that is not the shortest", "./lib/../lib", "a/../b", filepath.Join(root, "lib"),
			[]LocalDir{{filepath.Join(root, "lib", "b"), "lib/b/"}}},
		{"a folder outside the project, by its whole path", elsewhere, "", elsewhere, []LocalDir{
			{filepath.Join(elsewhere, "src", "modules"), fsx.ToSlash(elsewhere) + "/src/modules/"},
			{filepath.Join(elsewhere, "art"), fsx.ToSlash(elsewhere) + "/art/"},
		}},
	}
	for _, c := range cases {
		want := []Local{{Key: "mine", Dir: c.own, Dirs: c.want}}
		if got := Locals(root, librariesOf("mine", localLibrary(c.path, c.dir))); !slices.EqualFunc(got, want, sameLocal) {
			t.Errorf("%s: Locals = %+v, want %+v", c.name, got, want)
		}
	}
}

func TestLocalsListsTheFoldersTheSyncCopiesFrom(t *testing.T) {
	root := t.TempDir()
	writeTestFiles(t, root,
		"plain/a.lua", "return 'plain'",
		"named/moonwell-library.json", `{"dir":"src","assets":"art"}`,
		"named/src/b.lua", "return 'named'", "named/other/c.lua", "return 'other'", "named/art/x.blp", "x",
		"../beside/lua/d.lua", "return 'beside'",
	)
	libraries := librariesOf(
		"plain", localLibrary("plain", ""), "named", localLibrary("named", ""), "over", localLibrary("named", "other"),
		"beside", localLibrary("../beside", "lua"),
	)
	mustSync(t, root, libraries, nil)
	locals := Locals(root, libraries)
	if len(locals) != len(libraries) {
		t.Fatalf("Locals = %+v", locals)
	}
	for _, local := range locals {
		modules := local.Dirs[0].Dir
		if got, want := readFile(t, root, modulesDirName(local.Key)+"/"+stampFile), localStampText(modules); got != want {
			t.Errorf("%s: the sync stamped %q, and Locals lists %q", local.Key, got, modules)
		}
		isStamp := func(name string) bool { return name == stampFile }
		copied := slices.DeleteFunc(listFiles(t, root, modulesDirName(local.Key)), isStamp)
		from, err := listFilesBelow(modules, "", isModule, "")
		if err != nil || !slices.Equal(copied, from) {
			t.Errorf("%s: the sync copied %q, and the folder Locals lists holds %q, %v", local.Key, copied, from, err)
		}
		if shipped := exists(root, assetsDirName(local.Key)); shipped != (len(local.Dirs) == 2) {
			t.Errorf("%s: Locals lists %d folders, and the sync's folder of files for the map is there: %v",
				local.Key, len(local.Dirs), shipped)
		}
	}
	if got, want := listFiles(t, root, assetsDirName("named")), listFiles(t, root, "named/art"); !slices.Equal(got, want) {
		t.Errorf("the sync copied the files %q for the map, and the folder Locals lists holds %q", got, want)
	}
}
