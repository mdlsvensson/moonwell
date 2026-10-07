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

// mapLua is the script of the tests' map, as the project names it.
const mapLua = "maps/map.w3x/war3map.lua"

// typesHint ends a failure to write a file of declarations.
const typesHint = "Moonwell's compiler and the editor read .moonwell/; make sure it is a folder you can write, then retry."

// The four files of declarations, from the project folder, in the order they are written.
var declarationFiles = []string{
	".moonwell/types/natives.d.lua",
	".moonwell/types/moonwell.d.lua",
	".moonwell/types/objects.d.lua",
	".moonwell/types/map.d.lua",
}

// defined is what a script defines.
func defined(script string) *lua.MapGlobals {
	globals := lua.ReadMapGlobals(script)
	return &globals
}

// types is what the declarations of the tests are made from: one object, a small API, and what the script
// defines, nil for a map without one.
func types(globals *lua.MapGlobals) Types {
	miniature := &script.Natives{
		GameVersion: "9.9.9",
		Types:       []script.NativeType{{Name: "agent", Extends: "handle"}},
		Functions:   []script.NativeFunction{{Name: "DoNothing", Source: "common.j", Returns: "nothing"}},
	}
	return Types{Objects: resolved()[:1], Map: globals, MapLua: mapLua, Natives: miniature}
}

func TestRefreshTypesWritesTheFourDeclarationsThenOnlyWhatChanged(t *testing.T) {
	root := t.TempDir()
	in := types(defined("udg_Score = 0\nfunction main()\nend\n"))
	written, err := RefreshTypes(root, in)
	if err != nil || !slices.Equal(written, declarationFiles) {
		t.Fatalf("RefreshTypes = %q, %v", written, err)
	}
	want := map[string]string{
		"types/natives.d.lua":  RenderNatives(in.Natives),
		"types/moonwell.d.lua": RuntimeDeclarations,
		"types/objects.d.lua":  RenderObjects(in.Objects),
		"types/map.d.lua":      RenderMap(in.Map, mapLua),
	}
	// The macro module is the compiler's to write: the declarations are all there is.
	if got := filesIn(t, filepath.Join(root, ".moonwell")); !maps.Equal(got, want) {
		t.Errorf(".moonwell holds %q", slices.Sorted(maps.Keys(got)))
	}
	contains(t, read(t, root, ".moonwell/types/map.d.lua"), "udg_Score = nil", "from "+mapLua+";")
	if written, err := RefreshTypes(root, in); err != nil || written == nil || len(written) != 0 {
		t.Errorf("unchanged, but RefreshTypes = %#v, %v", written, err)
	}
	in.Map = defined("udg_Other = 0\n")
	if written, err := RefreshTypes(root, in); err != nil || !slices.Equal(written, declarationFiles[3:]) {
		t.Errorf("after a change to the map, RefreshTypes = %q, %v", written, err)
	}
	in.Objects = resolved()
	if written, err := RefreshTypes(root, in); err != nil || !slices.Equal(written, declarationFiles[2:3]) {
		t.Errorf("after a change to the objects, RefreshTypes = %q, %v", written, err)
	}
}

func TestRefreshTypesWorksWithoutAMapsScriptAndWithTheEmbeddedNatives(t *testing.T) {
	root := t.TempDir()
	if _, err := RefreshTypes(root, Types{MapLua: mapLua, Natives: script.LoadNatives()}); err != nil {
		t.Fatal(err)
	}
	contains(t, read(t, root, ".moonwell/types/map.d.lua"), "no maps/map.w3x/war3map.lua")
	contains(t, read(t, root, ".moonwell/types/natives.d.lua"), "function CreateUnit(", "---@class unit: widget\n")
	if got := read(t, root, ".moonwell/types/objects.d.lua"); got != RenderObjects(nil) {
		t.Errorf("without objects, objects.d.lua =\n%s", got)
	}
}

func TestRefreshTypesReadsNoMap(t *testing.T) {
	// What the script defines is handed in: whatever lies where the script would be is not looked at.
	projects := map[string][]string{
		"a script that defines other names":           {mapLua, "udg_OnDisk = 0\n"},
		"a folder in the place of the script":         {mapLua + "/inside.txt", ""},
		"a packed map in the place of the map folder": {"maps/map.w3x", "a packed map, not a folder"},
		"no map": nil,
	}
	for name, files := range projects {
		root := lay(t, files...)
		before := testkit.Snapshot(t, root)
		for _, globals := range []*lua.MapGlobals{nil, defined("udg_Handed = 0\n")} {
			if _, err := RefreshTypes(root, types(globals)); err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			if got := read(t, root, ".moonwell/types/map.d.lua"); got != RenderMap(globals, mapLua) {
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
	root := lay(t, ".moonwell", "a file, not a folder")
	written, err := RefreshTypes(root, types(nil))
	failure := asError(t, err, "a file for .moonwell")
	if !strings.HasPrefix(failure.Msg, "Writing .moonwell/types/natives.d.lua failed: ") ||
		failure.File != ".moonwell/types/natives.d.lua" || failure.Hint != typesHint || failure.Cause == nil || written != nil {
		t.Errorf("RefreshTypes = %q, %+v", written, failure)
	}
}

func TestADeclarationFileThatCannotBeWrittenIsNamed(t *testing.T) {
	root := t.TempDir()
	in := types(nil)
	if _, err := RefreshTypes(root, in); err != nil {
		t.Fatal(err)
	}
	testkit.MakeUnwritable(t, filepath.Join(root, ".moonwell", "types", "objects.d.lua"))
	// A file that cannot be written and holds what it is to hold is no failure: nothing is written.
	if written, err := RefreshTypes(root, in); err != nil || len(written) != 0 {
		t.Errorf("unchanged, but RefreshTypes = %q, %v", written, err)
	}
	in.Objects, in.Map = resolved(), defined("udg_Score = 0\n")
	written, err := RefreshTypes(root, in)
	failure := asError(t, err, "a file that cannot be written")
	if !strings.HasPrefix(failure.Msg, "Writing .moonwell/types/objects.d.lua failed: ") ||
		failure.File != ".moonwell/types/objects.d.lua" || failure.Hint != typesHint || written != nil {
		t.Errorf("RefreshTypes = %q, %+v", written, failure)
	}
	// The files are written in their order, and the first failure ends the refresh.
	if got := read(t, root, ".moonwell/types/map.d.lua"); got != RenderMap(nil, mapLua) {
		t.Errorf("map.d.lua was written after the failure:\n%s", got)
	}
}

func TestALinkOnTheWayToTheDeclarationsIsRefused(t *testing.T) {
	for _, link := range []string{".moonwell", ".moonwell/types"} {
		root := t.TempDir()
		at, target := linkAt(t, root, link)
		written, err := RefreshTypes(root, types(nil))
		failure := asError(t, err, "a link at "+link)
		if failure.Msg != "Symlinks are not supported: "+at || !strings.Contains(failure.Hint, "real files") || written != nil {
			t.Errorf("a link at %s: RefreshTypes = %q, %+v", link, written, failure)
		}
		if behind := testkit.Snapshot(t, target); len(behind) != 0 {
			t.Errorf("a link at %s: written behind the link: %q", link, slices.Sorted(maps.Keys(behind)))
		}
	}
}

func TestALinkAtAFileOfDeclarationsIsRefused(t *testing.T) {
	root := lay(t, "elsewhere/mine.lua", "mine", ".moonwell/types/natives.d.lua", "")
	at := filepath.Join(root, ".moonwell", "types", "map.d.lua")
	linkToFile(t, filepath.Join(root, "elsewhere", "mine.lua"), at)
	written, err := RefreshTypes(root, types(nil))
	failure := asError(t, err, "a link at map.d.lua")
	if failure.Msg != "Symlinks are not supported: "+at || !strings.Contains(failure.Hint, "real files") || written != nil {
		t.Errorf("RefreshTypes = %q, %+v", written, failure)
	}
	if got := read(t, root, "elsewhere/mine.lua"); got != "mine" {
		t.Errorf("the file behind the link holds %q", got)
	}
}

func TestRefreshTypesWithoutTheGamesAPIIsAMistakeOfTheCaller(t *testing.T) {
	root := t.TempDir()
	in := types(nil)
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
