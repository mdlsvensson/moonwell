package library

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// foldersOfLocals is the folders of the local libraries, on disk, in the order they are listed.
func foldersOfLocals(locals []Local) []string {
	var dirs []string
	for _, local := range locals {
		for _, folder := range local.Folders {
			dirs = append(dirs, folder.Dir)
		}
	}
	return dirs
}

// labelsOfLocals is the folders of the local libraries as they are written, in the order they are listed.
func labelsOfLocals(locals []Local) []string {
	var labels []string
	for _, local := range locals {
		for _, folder := range local.Folders {
			labels = append(labels, folder.Label)
		}
	}
	return labels
}

func TestLocalsListsTheFoldersOfLocalLibrariesOnly(t *testing.T) {
	root := t.TempDir()
	mine := filepath.Join(filepath.Dir(root), "mine")
	locals := Locals(root, block("mine", fromFolder("../mine", "src"), "remote", fromGitHub("v1", "")))
	want := []Local{{Key: "mine", Dir: mine, Folders: []LocalFolder{{filepath.Join(mine, "src"), "../mine/src/"}}}}
	if !slices.EqualFunc(locals, want, sameLocal) {
		t.Errorf("Locals = %+v, want %+v", locals, want)
	}
	if got := Locals(root, block("remote", fromGitHub("v1", ""))); got != nil {
		t.Errorf("Locals of a project without a local library = %+v", got)
	}
	if got := Locals(root, nil); got != nil {
		t.Errorf("Locals of a project without libraries = %+v", got)
	}
}

func sameLocal(a, b Local) bool {
	return a.Key == b.Key && a.Dir == b.Dir && slices.Equal(a.Folders, b.Folders)
}

func TestLocalsTakesTheModuleAndAssetsFoldersFromALibrarysOwnFile(t *testing.T) {
	root := t.TempDir()
	put(t, root,
		"described/moonwell-library.json", `{"dir":"src","assets":"art"}`,
		"broken/moonwell-library.json", "{",
		"rooted/moonwell-library.json", "{}",
	)
	locals := Locals(root, block(
		"a", fromFolder("described", ""), "b", fromFolder("described", "lua"), "c", fromFolder("broken", ""),
		"d", fromFolder("rooted", ""),
	))
	wantDirs := []string{
		filepath.Join(root, "described", "src"),
		filepath.Join(root, "described", "art"),
		filepath.Join(root, "described", "lua"),
		filepath.Join(root, "described", "art"),
		filepath.Join(root, "broken"),
		filepath.Join(root, "rooted"),
	}
	if got := foldersOfLocals(locals); !slices.Equal(got, wantDirs) {
		t.Errorf("the folders are %q, want %q", got, wantDirs)
	}
	wantLabels := []string{"described/src/", "described/art/", "described/lua/", "described/art/", "broken/", "rooted/"}
	if got := labelsOfLocals(locals); !slices.Equal(got, wantLabels) {
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
	locals := Locals(root, block(
		"b", fromFolder("one", ""), "a", fromFolder("two", ""), "_x", fromFolder("three", ""),
		"B", fromFolder("four", ""), "a-1", fromFolder("five", ""), "0", fromFolder("six", ""),
	))
	var keys []string
	for _, local := range locals {
		keys = append(keys, local.Key)
	}
	if want := []string{"0", "B", "_x", "a", "a-1", "b"}; !slices.Equal(keys, want) {
		t.Errorf("the keys are in the order %q, want %q", keys, want)
	}
}

// What the sync refuses of a library's own file is no file to Locals: the library is listed by the manifest
// alone, so that it is watched, and the sync says what is wrong with the file.
func TestLocalsTakesALibrarysFileThatTheSyncRefusesForNone(t *testing.T) {
	cases := []struct {
		name string
		file string // the path of what is laid in the file's place, from the library
		text string
		held bool // the file cannot be read
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
		put(t, root, "lib/"+c.file, c.text, "lib/src/a.lua", "return 1", "lib/art/x.blp", "x")
		if c.held {
			testkit.MakeUnreadable(t, filepath.Join(root, "lib", File))
		}
		libraries := block("mine", fromFolder("lib", ""))
		lib := filepath.Join(root, "lib")
		want := []Local{{Key: "mine", Dir: lib, Folders: []LocalFolder{{lib, "lib/"}}}}
		if got := Locals(root, libraries); !slices.EqualFunc(got, want, sameLocal) {
			t.Errorf("%s: Locals = %+v, want %+v", c.name, got, want)
		}
		// The manifest's dir is taken all the same.
		want[0].Folders = []LocalFolder{{filepath.Join(lib, "src"), "lib/src/"}}
		if got := Locals(root, block("mine", fromFolder("lib", "src"))); !slices.EqualFunc(got, want, sameLocal) {
			t.Errorf("%s, with a dir in the manifest: Locals = %+v, want %+v", c.name, got, want)
		}
		refusal(t, root, libraries, nil, c.name)
	}
}

func TestALocalsFolderIsWrittenAsTheManifestAndTheLibrarysFileWriteIt(t *testing.T) {
	root := t.TempDir()
	elsewhere := filepath.Join(t.TempDir(), "kit")
	put(t, elsewhere, "moonwell-library.json", `{"dir":"src/modules","assets":"art"}`)
	cases := []struct {
		name      string
		path, dir string
		own       string // the library's folder
		want      []LocalFolder
	}{
		{"the project folder", ".", "", root, []LocalFolder{{root, "./"}}},
		{"a folder that is not there", "nowhere/lib", "", filepath.Join(root, "nowhere", "lib"),
			[]LocalFolder{{filepath.Join(root, "nowhere", "lib"), "nowhere/lib/"}}},
		{"a path that ends in a separator", "lib/", "src/", filepath.Join(root, "lib"),
			[]LocalFolder{{filepath.Join(root, "lib", "src"), "lib/src/"}}},
		{"a path that is not the shortest", "./lib/../lib", "a/../b", filepath.Join(root, "lib"),
			[]LocalFolder{{filepath.Join(root, "lib", "b"), "lib/b/"}}},
		{"a folder outside the project, by its whole path", elsewhere, "", elsewhere, []LocalFolder{
			{filepath.Join(elsewhere, "src", "modules"), fsx.ToPosix(elsewhere) + "/src/modules/"},
			{filepath.Join(elsewhere, "art"), fsx.ToPosix(elsewhere) + "/art/"},
		}},
	}
	for _, c := range cases {
		want := []Local{{Key: "mine", Dir: c.own, Folders: c.want}}
		if got := Locals(root, block("mine", fromFolder(c.path, c.dir))); !slices.EqualFunc(got, want, sameLocal) {
			t.Errorf("%s: Locals = %+v, want %+v", c.name, got, want)
		}
	}
}

// Locals and the sync have one rule for where a local library is read from: the module folder that Locals lists
// is the one whose path the sync stamps the library's copy with, and the copies hold the files of the folders
// that Locals lists.
func TestLocalsListsTheFoldersTheSyncCopiesFrom(t *testing.T) {
	root := t.TempDir()
	put(t, root,
		"plain/a.lua", "return 'plain'",
		"named/moonwell-library.json", `{"dir":"src","assets":"art"}`,
		"named/src/b.lua", "return 'named'", "named/other/c.lua", "return 'other'", "named/art/x.blp", "x",
		"../beside/lua/d.lua", "return 'beside'",
	)
	libraries := block(
		"plain", fromFolder("plain", ""), "named", fromFolder("named", ""), "over", fromFolder("named", "other"),
		"beside", fromFolder("../beside", "lua"),
	)
	sync(t, root, libraries, nil)
	locals := Locals(root, libraries)
	if len(locals) != len(libraries) {
		t.Fatalf("Locals = %+v", locals)
	}
	for _, local := range locals {
		modules := local.Folders[0].Dir
		if got, want := textOf(t, root, modulesOf(local.Key)+"/"+stampFile), stampOfFolder(modules); got != want {
			t.Errorf("%s: the sync stamped %q, and Locals lists %q", local.Key, got, modules)
		}
		isStamp := func(name string) bool { return name == stampFile }
		copied := slices.DeleteFunc(filesIn(t, root, modulesOf(local.Key)), isStamp)
		from, err := listBelow(modules, "", isModule, "")
		if err != nil || !slices.Equal(copied, from) {
			t.Errorf("%s: the sync copied %q, and the folder Locals lists holds %q, %v", local.Key, copied, from, err)
		}
		if shipped := there(root, assetsOf(local.Key)); shipped != (len(local.Folders) == 2) {
			t.Errorf("%s: Locals lists %d folders, and the sync's folder of files for the map is there: %v",
				local.Key, len(local.Folders), shipped)
		}
	}
	if got, want := filesIn(t, root, assetsOf("named")), filesIn(t, root, "named/art"); !slices.Equal(got, want) {
		t.Errorf("the sync copied the files %q for the map, and the folder Locals lists holds %q", got, want)
	}
}
