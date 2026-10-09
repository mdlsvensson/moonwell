package editor

import (
	"errors"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/script"
	"github.com/mdlsvensson/moonwell/internal/testkit"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

const mapLua = "maps/map.w3x/war3map.lua"

const typesHint = "Moonwell's compiler and the editor read .moonwell/; make sure it is a folder you can write, then retry."

var declarationFiles = []string{
	".moonwell/types/natives.d.lua",
	".moonwell/types/moonwell.d.lua",
	".moonwell/types/objects.d.lua",
	".moonwell/types/map.d.lua",
}

func mapGlobalsOf(script string) *lua.MapGlobals {
	globals := lua.ReadMapGlobals(script)
	return &globals
}

func typesWith(globals *lua.MapGlobals) Types {
	miniature := &script.Natives{
		GameVersion: "9.9.9",
		Types:       []script.NativeType{{Name: "agent", Extends: "handle"}},
		Functions:   []script.NativeFunction{{Name: "DoNothing", Source: "common.j", Returns: "nothing"}},
	}
	return Types{Objects: testObjects()[:1], Map: globals, MapLua: mapLua, Natives: miniature}
}

func TestRefreshTypesWritesTheFourDeclarationsThenOnlyWhatChanged(t *testing.T) {
	root := t.TempDir()
	in := typesWith(mapGlobalsOf("udg_Score = 0\nfunction main()\nend\n"))
	written, err := RefreshTypes(root, in)
	if err != nil || !slices.Equal(written, declarationFiles) {
		t.Fatalf("RefreshTypes = %q, %v", written, err)
	}
	want := map[string]string{
		"types/natives.d.lua":  renderNatives(in.Natives),
		"types/moonwell.d.lua": runtimeDeclarations,
		"types/objects.d.lua":  renderObjects(in.Objects),
		"types/map.d.lua":      renderMap(in.Map, mapLua),
	}
	if got := readFiles(t, filepath.Join(root, ".moonwell")); !maps.Equal(got, want) {
		t.Errorf(".moonwell holds %q", slices.Sorted(maps.Keys(got)))
	}
	checkContains(t, readFile(t, root, ".moonwell/types/map.d.lua"), "udg_Score = nil", "from "+mapLua+";")
	if written, err := RefreshTypes(root, in); err != nil || written == nil || len(written) != 0 {
		t.Errorf("unchanged, but RefreshTypes = %#v, %v", written, err)
	}
	in.Map = mapGlobalsOf("udg_Other = 0\n")
	if written, err := RefreshTypes(root, in); err != nil || !slices.Equal(written, declarationFiles[3:]) {
		t.Errorf("after a change to the map, RefreshTypes = %q, %v", written, err)
	}
	in.Objects = testObjects()
	if written, err := RefreshTypes(root, in); err != nil || !slices.Equal(written, declarationFiles[2:3]) {
		t.Errorf("after a change to the objects, RefreshTypes = %q, %v", written, err)
	}
}

func TestRefreshTypesWorksWithoutAMapsScriptAndWithTheEmbeddedNatives(t *testing.T) {
	root := t.TempDir()
	if _, err := RefreshTypes(root, Types{MapLua: mapLua, Natives: script.LoadNatives()}); err != nil {
		t.Fatal(err)
	}
	checkContains(t, readFile(t, root, ".moonwell/types/map.d.lua"), "no maps/map.w3x/war3map.lua")
	checkContains(t, readFile(t, root, ".moonwell/types/natives.d.lua"), "function CreateUnit(", "---@class unit: widget\n")
	if got := readFile(t, root, ".moonwell/types/objects.d.lua"); got != renderObjects(nil) {
		t.Errorf("without objects, objects.d.lua =\n%s", got)
	}
}

func TestRefreshTypesReadsNoMap(t *testing.T) {
	projects := map[string][]string{
		"a script that defines other names":           {mapLua, "udg_OnDisk = 0\n"},
		"a folder in the place of the script":         {mapLua + "/inside.txt", ""},
		"a packed map in the place of the map folder": {"maps/map.w3x", "a packed map, not a folder"},
		"no map": nil,
	}
	for name, files := range projects {
		root := newProjectDir(t, files...)
		before := testkit.Snapshot(t, root)
		for _, globals := range []*lua.MapGlobals{nil, mapGlobalsOf("udg_Handed = 0\n")} {
			if _, err := RefreshTypes(root, typesWith(globals)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if got := readFile(t, root, ".moonwell/types/map.d.lua"); got != renderMap(globals, mapLua) {
				t.Errorf("%s: map.d.lua =\n%s", name, got)
			}
		}
		after := testkit.Snapshot(t, root)
		maps.DeleteFunc(after, func(path string, _ []byte) bool { return strings.HasPrefix(path, ".moonwell") })
		if !reflect.DeepEqual(before, after) {
			t.Errorf("%s: the project outside .moonwell changed", name)
		}
	}
}

func TestAMoonwellFolderThatCannotBeWrittenFails(t *testing.T) {
	root := newProjectDir(t, ".moonwell", "a file, not a folder")
	written, err := RefreshTypes(root, typesWith(nil))
	diagErr := asDiagError(t, err, "a file for .moonwell")
	if !strings.HasPrefix(diagErr.Msg, "Writing .moonwell/types/natives.d.lua failed: ") ||
		diagErr.File != ".moonwell/types/natives.d.lua" || diagErr.Hint != typesHint || diagErr.Cause == nil || written != nil {
		t.Errorf("RefreshTypes = %q, %+v", written, diagErr)
	}
}

func TestADeclarationFileThatCannotBeWrittenIsNamed(t *testing.T) {
	root := t.TempDir()
	in := typesWith(nil)
	if _, err := RefreshTypes(root, in); err != nil {
		t.Fatal(err)
	}
	testkit.MakeUnwritable(t, filepath.Join(root, ".moonwell", "types", "objects.d.lua"))
	if written, err := RefreshTypes(root, in); err != nil || len(written) != 0 {
		t.Errorf("unchanged, but RefreshTypes = %q, %v", written, err)
	}
	in.Objects, in.Map = testObjects(), mapGlobalsOf("udg_Score = 0\n")
	written, err := RefreshTypes(root, in)
	diagErr := asDiagError(t, err, "a file that cannot be written")
	if !strings.HasPrefix(diagErr.Msg, "Writing .moonwell/types/objects.d.lua failed: ") ||
		diagErr.File != ".moonwell/types/objects.d.lua" || diagErr.Hint != typesHint || written != nil {
		t.Errorf("RefreshTypes = %q, %+v", written, diagErr)
	}
	if got := readFile(t, root, ".moonwell/types/map.d.lua"); got != renderMap(nil, mapLua) {
		t.Errorf("map.d.lua was written after the failure:\n%s", got)
	}
}

func TestALinkOnTheWayToTheDeclarationsIsRefused(t *testing.T) {
	for _, symlink := range []string{".moonwell", ".moonwell/types"} {
		root := t.TempDir()
		at, target := symlinkDirAt(t, root, symlink)
		written, err := RefreshTypes(root, typesWith(nil))
		diagErr := asDiagError(t, err, "a link at "+symlink)
		if diagErr.Msg != "Symlinks are not supported: "+at || diagErr.File != ".moonwell/types/natives.d.lua" ||
			!strings.Contains(diagErr.Hint, "real files") || written != nil {
			t.Errorf("a link at %s: RefreshTypes = %q, %+v", symlink, written, diagErr)
		}
		if behind := testkit.Snapshot(t, target); len(behind) != 0 {
			t.Errorf("a link at %s: written behind the link: %q", symlink, slices.Sorted(maps.Keys(behind)))
		}
	}
}

func TestALinkAtAFileOfDeclarationsIsRefused(t *testing.T) {
	root := newProjectDir(t, "elsewhere/mine.lua", "mine", ".moonwell/types/natives.d.lua", "")
	at := filepath.Join(root, ".moonwell", "types", "map.d.lua")
	symlinkFile(t, filepath.Join(root, "elsewhere", "mine.lua"), at)
	written, err := RefreshTypes(root, typesWith(nil))
	diagErr := asDiagError(t, err, "a link at map.d.lua")
	if diagErr.Msg != "Symlinks are not supported: "+at || diagErr.File != ".moonwell/types/map.d.lua" ||
		!strings.Contains(diagErr.Hint, "real files") || written != nil {
		t.Errorf("RefreshTypes = %q, %+v", written, diagErr)
	}
	if got := readFile(t, root, "elsewhere/mine.lua"); got != "mine" {
		t.Errorf("the file behind the link holds %q", got)
	}
}

func TestRefreshTypesWithoutTheGamesAPIIsAMistakeOfTheCaller(t *testing.T) {
	root := t.TempDir()
	in := typesWith(nil)
	in.Natives = nil
	written, err := RefreshTypes(root, in)
	var expected *diag.Error
	if err == nil || errors.As(err, &expected) || !strings.Contains(err.Error(), "script.LoadNatives()") || written != nil {
		t.Errorf("RefreshTypes = %q, %v, want an error that is no expected failure", written, err)
	}
	if left := testkit.Snapshot(t, root); len(left) != 0 {
		t.Errorf("written without the game's API: %q", slices.Sorted(maps.Keys(left)))
	}
}
