package assets_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/assets"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/mapdir"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

func asError(t *testing.T, err error, what string) *diag.Error {
	t.Helper()
	var e *diag.Error
	if !errors.As(err, &e) {
		t.Fatalf("%s: got %v, want a *diag.Error", what, err)
	}
	return e
}

func put(t *testing.T, root, file string, value ...string) {
	t.Helper()
	content := "asset"
	if len(value) > 0 {
		content = value[0]
	}
	testkit.WriteFile(t, root, file, []byte(content))
}

func projectRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "assets"), 0o777); err != nil {
		t.Fatal(err)
	}
	return root
}

// config builds the assets block: paths as alternating source and target, in order.
func config(exclude []string, paths ...string) assets.Config {
	mapped := &ordered.Map[string]{}
	for i := 0; i < len(paths); i += 2 {
		mapped.Set(paths[i], paths[i+1])
	}
	return assets.Config{Paths: mapped, Exclude: exclude}
}

var defaults = config(nil)

type row struct{ library, source, target string }

func rows(list []*assets.Asset) []row {
	out := []row{}
	for _, asset := range list {
		out = append(out, row{asset.Library, asset.Source, asset.Target})
	}
	return out
}

func TestImportsRoundTripDefaultAndCustomPathEntries(t *testing.T) {
	entries := []assets.Import{{Flag: 5, Path: "default.blp"}, {Flag: 13, Path: `Textures\custom.blp`}}
	read, err := assets.ReadImports(assets.WriteImports(entries), "war3map.imp")
	if err != nil || !slices.Equal(read, entries) {
		t.Errorf("round trip = %+v, %v", read, err)
	}
	if assets.ImportPath(entries[0]) != `war3mapImported\default.blp` || assets.ImportPath(entries[1]) != `Textures\custom.blp` {
		t.Error("ImportPath is wrong")
	}
	if none, err := assets.ReadImports(assets.WriteImports(nil), "war3map.imp"); err != nil || len(none) != 0 {
		t.Errorf("no entries = %+v, %v", none, err)
	}
}

func TestWorldEditor300sFlag29IsACustomPath(t *testing.T) {
	// assets:sync wrote this entry with flag 13; World Editor 3.00 saved it back with flag 29.
	saved := testkit.Fixture(t, "imports-we3/war3map-flag29.imp")
	entries, err := assets.ReadImports(saved, "war3map.imp")
	if err != nil || !slices.Equal(entries, []assets.Import{{Flag: 29, Path: "wa3mapPreview.tga"}}) {
		t.Fatalf("entries = %+v, %v", entries, err)
	}
	if assets.ImportPath(entries[0]) != "wa3mapPreview.tga" || !bytes.Equal(assets.WriteImports(entries), saved) {
		t.Error("the entry does not write back as World Editor saved it")
	}
}

func TestReadImportsRejectsCorruptDataNamingTheFile(t *testing.T) {
	valid := assets.WriteImports([]assets.Import{{Flag: 13, Path: "a.blp"}})
	wrongVersion, badFlag := bytes.Clone(valid), bytes.Clone(valid)
	wrongVersion[0], badFlag[8] = 2, 7
	for _, data := range [][]byte{{1, 0}, valid[:10], wrongVersion, badFlag} {
		_, err := assets.ReadImports(data, "maps/map.w3x/war3map.imp")
		if e := asError(t, err, "corrupt data"); e.File != "maps/map.w3x/war3map.imp" || e.Hint == "" {
			t.Errorf("error = %+v", e)
		}
	}
	_, err := assets.ReadImports(append(bytes.Clone(valid), 0), "war3map.imp")
	if e := asError(t, err, "trailing data"); !strings.Contains(e.Msg, "trailing") {
		t.Errorf("error = %+v", e)
	}
	empty := assets.WriteImports([]assets.Import{{Flag: 13, Path: "x"}})
	empty[9] = 0 // the path's only character becomes the terminator
	_, err = assets.ReadImports(empty[:10], "war3map.imp")
	if e := asError(t, err, "an empty path"); !strings.Contains(e.Msg, "empty path") {
		t.Errorf("error = %+v", e)
	}
}

func TestTargetPathRejectsMapInternalsButAllowsWar3mapImported(t *testing.T) {
	if mapdir.Key(`Textures\A.BLP`) != "textures/a.blp" {
		t.Error("Key is wrong")
	}
	// Reforged ignores war3mapPreview.tga (the map list shows war3mapMap.blp), so it stays reserved like the rest.
	for _, reserved := range []string{
		"war3map.lua", "war3map.imp", "WAR3MAP.W3I", "scripts/war3map.j", "(listfile)", "war3mapMap.blp", "war3mapPreview.tga",
	} {
		_, err := assets.TargetPath(reserved)
		if e := asError(t, err, reserved); e.Msg != "Reserved map path: "+reserved {
			t.Errorf("TargetPath(%s): %+v", reserved, e)
		}
	}
	hint := func(path string) string {
		_, err := assets.TargetPath(path)
		return asError(t, err, path).Hint
	}
	// The names tried for a map list picture point at the setting that does it; other internals do not.
	for _, picture := range []string{"war3mapPreview.tga", "WAR3MAPPREVIEW.BLP", "war3mapMap.blp", "war3mapMap.tga"} {
		if !strings.Contains(hint(picture), "settings.info.preview") {
			t.Errorf("the hint for %s is %q", picture, hint(picture))
		}
	}
	for _, other := range []string{"war3map.lua", "war3mapMisc.txt", "scripts/war3map.j", "war3mapPreviews/a.tga"} {
		if hint(other) != "Assets cannot replace map internals such as war3map.lua or war3map.imp." {
			t.Errorf("the hint for %s is %q", other, hint(other))
		}
	}
	if got, err := assets.TargetPath("war3mapImported/sound.wav"); err != nil || got != "war3mapImported/sound.wav" {
		t.Errorf("war3mapImported = %q, %v", got, err)
	}
	if _, err := assets.TargetPath("../escape"); err == nil {
		t.Error("a path that leaves the map was accepted")
	}
}

func TestCollectMapsExcludesSkipsDotfilesAndSortsByTarget(t *testing.T) {
	root := projectRoot(t)
	put(t, root, "assets/ReplaceableTextures/CommandButtons/BTNSword.blp")
	put(t, root, "assets/icons/disabled.blp", "disabled")
	put(t, root, "assets/credits/readme.txt")
	put(t, root, "assets/.gitkeep")
	const disabled = "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp"
	collected, err := assets.Collect(root, config([]string{"credits/"}, "icons/disabled.blp", disabled))
	if err != nil {
		t.Fatal(err)
	}
	const sword = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	if got := rows(collected); !slices.Equal(got, []row{{"", sword, sword}, {"", "icons/disabled.blp", disabled}}) {
		t.Errorf("assets = %+v", got)
	}
	if string(collected[1].Bytes) != "disabled" || len(collected[1].Hash) != 64 {
		t.Errorf("the mapped asset = %+v", collected[1])
	}
}

func TestCollectReturnsNothingWhenAssetsIsMissing(t *testing.T) {
	if collected, err := assets.Collect(t.TempDir(), defaults); err != nil || len(collected) != 0 {
		t.Errorf("Collect = %+v, %v", collected, err)
	}
}

func TestCollectRejectsBadMappingsCollisionsAndReservedTargets(t *testing.T) {
	root := projectRoot(t)
	put(t, root, "assets/a.blp")
	put(t, root, "assets/b.blp")
	refused := func(c assets.Config, message string) {
		t.Helper()
		_, err := assets.Collect(root, c)
		if e := asError(t, err, message); !strings.Contains(e.Msg, message) {
			t.Errorf("error = %q, want %q", e.Msg, message)
		}
	}
	for _, target := range []string{"../escape", "/absolute", `C:\escape`, "war3map.lua", "war3map.imp", "scripts/war3map.j"} {
		refused(config(nil, "a.blp", target), "path")
	}
	refused(config(nil, "missing", "x.blp"), "assets.paths names a file that does not exist: assets/missing")
	refused(config(nil, "a.blp", "X.blp", "b.blp", "x.blp"), "Two assets would be imported as x.blp (target collision).")
	refused(config(nil, "a.blp", "x", "b.blp", "x/y"), "Asset x/y would sit inside the asset file x (file/folder collision).")
	refused(config([]string{"a.blp"}, "a.blp", "x"), "assets.paths names an excluded file: a.blp")
	_, err := assets.Collect(root, config(nil, "missing", "x.blp"))
	if e := asError(t, err, "a config error"); e.File != "moonwell.pkl" || e.Hint != "Fix the assets block in moonwell.pkl." {
		t.Errorf("error = %+v", e)
	}
}

func TestScanFilesRejectsCaseCollisionsAndLinkedFolders(t *testing.T) {
	root := projectRoot(t)
	put(t, root, "assets/a.blp")
	if testkit.CaseSensitive(t, root) {
		// A case-insensitive file system cannot hold two such names.
		put(t, root, "assets/A.blp")
		_, err := assets.ScanFiles(filepath.Join(root, "assets"))
		if e := asError(t, err, "a case collision"); !strings.Contains(e.Msg, "letter case") {
			t.Errorf("error = %+v", e)
		}
		os.Remove(filepath.Join(root, "assets", "A.blp"))
	}
	external := filepath.Join(root, "external")
	os.Mkdir(external, 0o777)
	testkit.LinkDir(t, external, filepath.Join(root, "assets", "linked"))
	_, err := assets.Collect(root, defaults)
	if e := asError(t, err, "a linked folder"); !strings.Contains(e.Msg, "Symlinks") {
		t.Errorf("error = %+v", e)
	}
}

func TestScanFilesListsEachFoldersEntriesInOrder(t *testing.T) {
	root := t.TempDir()
	for _, file := range []string{"b.txt", "a/z.txt", "a/B.txt", "c/d/e.txt", "a.txt"} {
		put(t, root, file)
	}
	files, err := assets.ScanFiles(root)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a/b.txt", "a/z.txt", "a.txt", "b.txt", "c/d/e.txt"}; !slices.Equal(files.Keys(), want) {
		t.Errorf("keys = %q", files.Keys())
	}
	if name, ok := files.Get("a/b.txt"); !ok || name != "a/B.txt" || files.Has("missing") {
		t.Errorf("a/b.txt = %q, %v", name, ok)
	}
	if none, err := assets.ScanFiles(filepath.Join(root, "missing")); err != nil || len(none.Keys()) != 0 {
		t.Errorf("a missing folder = %v, %v", none, err)
	}
	_, err = assets.ScanFiles(filepath.Join(root, "b.txt"))
	if e := asError(t, err, "a file"); !strings.HasPrefix(e.Msg, "Expected a folder: ") {
		t.Errorf("error = %+v", e)
	}
}

func TestSafeJoinAcceptsARootThatIsItselfALinkButRejectsALinkBelowIt(t *testing.T) {
	parent := projectRoot(t)
	real := filepath.Join(parent, "real")
	put(t, real, "assets/a.blp")
	linkedRoot := filepath.Join(parent, "linked-root")
	testkit.LinkDir(t, real, linkedRoot)
	if got, err := fsx.SafeJoin(linkedRoot, "assets/a.blp"); err != nil || got != filepath.Join(linkedRoot, "assets", "a.blp") {
		t.Errorf("SafeJoin = %q, %v", got, err)
	}
	collected, err := assets.Collect(linkedRoot, defaults)
	if err != nil || len(collected) != 1 || collected[0].Target != "a.blp" {
		t.Errorf("Collect through a linked root = %+v, %v", rows(collected), err)
	}
	testkit.LinkDir(t, filepath.Join(parent, "assets"), filepath.Join(real, "inner"))
	_, err = fsx.SafeJoin(linkedRoot, "inner/x.blp")
	if e := asError(t, err, "a link below the root"); !strings.Contains(e.Msg, "Symlinks") {
		t.Errorf("error = %+v", e)
	}
}

func TestLibraryAssetsFollowTheMapsOwnByKeyAndImportAtTheirPathInTheLibrary(t *testing.T) {
	root := projectRoot(t)
	put(t, root, "assets/Models/Own.mdx", "own")
	put(t, root, ".moonwell/library-assets/zeta/Sounds/Horn.wav", "horn")
	put(t, root, ".moonwell/library-assets/alpha/war3mapImported/alpha/frames.toc", "toc")
	put(t, root, ".moonwell/library-assets/alpha/.hidden/x.txt", "hidden")
	put(t, root, ".moonwell/library-assets/other/never.txt", "not in the manifest")
	collected, replaced, err := assets.CollectProject(root, defaults, []string{"zeta", "alpha", "none"})
	if err != nil {
		t.Fatal(err)
	}
	want := []row{
		{"", "Models/Own.mdx", "Models/Own.mdx"},
		{"zeta", "Sounds/Horn.wav", "Sounds/Horn.wav"},
		{"alpha", "war3mapImported/alpha/frames.toc", "war3mapImported/alpha/frames.toc"},
	}
	if got := rows(collected); !slices.Equal(got, want) || string(collected[1].Bytes) != "horn" || len(replaced) != 0 {
		t.Errorf("assets = %+v, replaced %q", got, replaced)
	}
	own, replaced, err := assets.CollectProject(root, defaults, nil)
	if err != nil || !slices.Equal(rows(own), want[:1]) || len(replaced) != 0 {
		t.Errorf("without libraries = %+v, %v", rows(own), err)
	}
}

func TestTheMapsOwnAssetReplacesALibrarysAtTheSameInMapPathInAnyLetterCase(t *testing.T) {
	root := projectRoot(t)
	put(t, root, "assets/custom/golem.blp", "the map's")
	put(t, root, "assets/Models/own.mdx", "the map's model")
	put(t, root, ".moonwell/library-assets/lib/Textures/Golem.blp", "the library's")
	put(t, root, ".moonwell/library-assets/lib/models/Own.mdx", "the library's model")
	put(t, root, ".moonwell/library-assets/lib/Textures/Other.blp", "kept")
	collected, replaced, err := assets.CollectProject(root, config(nil, "custom/golem.blp", "textures/golem.blp"), []string{"lib"})
	if err != nil {
		t.Fatal(err)
	}
	want := []row{
		{"", "Models/own.mdx", "Models/own.mdx"},
		{"", "custom/golem.blp", "textures/golem.blp"},
		{"lib", "Textures/Other.blp", "Textures/Other.blp"},
	}
	if got := rows(collected); !slices.Equal(got, want) {
		t.Errorf("assets = %+v", got)
	}
	wantReplaced := []string{
		"assets/custom/golem.blp replaces library lib's Textures/Golem.blp",
		"assets/Models/own.mdx replaces library lib's models/Own.mdx",
	}
	if !slices.Equal(replaced, wantReplaced) {
		t.Errorf("replaced = %q", replaced)
	}
}

func TestTwoLibrariesAtOneInMapPathFailUnlessTheMapsOwnFileReplacesBoth(t *testing.T) {
	root := projectRoot(t)
	put(t, root, ".moonwell/library-assets/a/UI/Frame.fdf", "a")
	put(t, root, ".moonwell/library-assets/b/ui/frame.fdf", "b")
	_, _, err := assets.CollectProject(root, defaults, []string{"b", "a"})
	e := asError(t, err, "two libraries at one path")
	want := &diag.Error{
		Msg:  `Libraries a and b both import ui\frame.fdf.`,
		File: "moonwell.pkl",
		Hint: "Drop one of the libraries, or put your own file at that path under assets/ to replace both.",
	}
	if !reflect.DeepEqual(e, want) {
		t.Errorf("error = %+v", e)
	}
	put(t, root, "assets/UI/Frame.fdf", "the map's")
	collected, replaced, err := assets.CollectProject(root, defaults, []string{"b", "a"})
	if err != nil || len(collected) != 1 || collected[0].Library != "" || len(replaced) != 2 {
		t.Errorf("assets = %+v, replaced %q, %v", rows(collected), replaced, err)
	}
}

func TestALibraryFileWithAReservedPathOrInsideAnotherAssetFileIsRefused(t *testing.T) {
	reserved := projectRoot(t)
	put(t, reserved, ".moonwell/library-assets/bad/war3map.lua", "script")
	_, _, err := assets.CollectProject(reserved, defaults, []string{"bad"})
	e := asError(t, err, "a reserved path")
	if e.Msg != "Library bad: Reserved map path: war3map.lua" || e.File != ".moonwell/library-assets/bad" ||
		e.Hint != "Report it to the library's author, or use another version of the library." {
		t.Errorf("error = %+v", e)
	}
	nested := projectRoot(t)
	put(t, nested, "assets/data", "a file")
	put(t, nested, ".moonwell/library-assets/lib/data/inner.txt", "inside it")
	_, _, err = assets.CollectProject(nested, defaults, []string{"lib"})
	if e := asError(t, err, "a nested path"); !strings.Contains(e.Msg, "file/folder collision") {
		t.Errorf("error = %+v", e)
	}
}
