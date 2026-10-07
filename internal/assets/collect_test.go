package assets

import (
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/testkit"
)

// sources is the file each asset was read from, in the order of the assets.
func sources(assets []Asset) []string {
	list := []string{}
	for _, asset := range assets {
		list = append(list, asset.Source)
	}
	return list
}

func TestCollectMapsExcludesSkipsDotFilesAndWritesNothing(t *testing.T) {
	root := t.TempDir()
	const sword = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	const disabled = "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp"
	put(t, root, "assets/"+sword)
	put(t, root, "assets/icons/disabled.blp", "disabled")
	put(t, root, "assets/credits/readme.txt")
	put(t, root, "assets/.gitkeep")
	before := testkit.Snapshot(t, root)

	collected, replaced := collect(t, root, `{"paths":{"icons/disabled.blp":"`+disabled+`"},"exclude":["credits/"]}`)
	if got := rows(collected); !slices.Equal(got, []row{{"", sword, sword}, {"", "icons/disabled.blp", disabled}}) {
		t.Errorf("assets = %+v", got)
	}
	if mapped := collected[1]; string(mapped.Bytes) != "disabled" || mapped.Hash != fsx.SHA256Hex([]byte("disabled")) {
		t.Errorf("the mapped asset holds %q with the hash %s", mapped.Bytes, mapped.Hash)
	}
	if len(replaced) != 0 {
		t.Errorf("replaced = %q in a project without libraries", replaced)
	}
	if after := testkit.Snapshot(t, root); !maps.EqualFunc(before, after, slices.Equal) {
		t.Error("collecting changed the project")
	}
}

func TestCollectReturnsNothingWhenAssetsIsMissing(t *testing.T) {
	if collected, replaced := collect(t, t.TempDir(), noBlock); len(collected) != 0 || len(replaced) != 0 {
		t.Errorf("Collect = %+v, %q", rows(collected), replaced)
	}
}

func TestAssetsAreSortedByInMapPathWithoutRegardToLetterCase(t *testing.T) {
	root := t.TempDir()
	for _, file := range []string{"b.blp", "A.blp", "a/c.blp", "Z.blp", "mapped.blp"} {
		put(t, root, "assets/"+file)
	}
	collected, _ := collect(t, root, `{"paths":{"mapped.blp":"B/first.blp"},"exclude":[]}`)
	want := []row{{"", "A.blp", "A.blp"}, {"", "a/c.blp", "a/c.blp"}, {"", "b.blp", "b.blp"},
		{"", "mapped.blp", "B/first.blp"}, {"", "Z.blp", "Z.blp"}}
	if got := rows(collected); !slices.Equal(got, want) {
		t.Errorf("assets = %+v, want %+v", got, want)
	}
}

func TestExcludeLeavesOutAFileOrAFolderAndANameThatStartsWithADotIsSkipped(t *testing.T) {
	root := t.TempDir()
	for _, file := range []string{"credits/readme.txt", "credits/deep/more.txt", "credits.txt", "Icons/a.blp",
		".gitkeep", ".git/config", "Icons/.DS_Store", "Icons/..b"} {
		put(t, root, "assets/"+file)
	}
	all := []string{"credits.txt", "credits/deep/more.txt", "credits/readme.txt", "Icons/a.blp"}
	tests := []struct {
		name    string
		exclude string // the list, as JSON
		want    []string
	}{
		{"nothing", `[]`, all},
		{"a folder", `["credits/"]`, []string{"credits.txt", "Icons/a.blp"}},
		{"a folder with a backslash", `["credits\\"]`, []string{"credits.txt", "Icons/a.blp"}},
		{"a folder in another letter case", `["CREDITS/"]`, []string{"credits.txt", "Icons/a.blp"}},
		{"a folder below a folder", `["credits\\Deep/"]`, []string{"credits.txt", "credits/readme.txt", "Icons/a.blp"}},
		{"a file", `["Credits/README.txt", "icons/a.blp"]`, []string{"credits.txt", "credits/deep/more.txt"}},
		{"a folder named without its slash is a file, and there is none", `["credits"]`, all},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			collected, _ := collect(t, root, `{"paths":{},"exclude":`+tt.exclude+`}`)
			if got := sources(collected); !slices.Equal(got, tt.want) {
				t.Errorf("assets = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCollectRefusesBadMappingsCollisionsAndReservedTargets(t *testing.T) {
	root := t.TempDir()
	put(t, root, "assets/a.blp")
	put(t, root, "assets/b.blp")
	put(t, root, "assets/.hidden")
	const inBlock = "Fix the assets block"
	tests := []struct {
		name, block string
		words       string
		file, hint  string
	}{
		{"a target that leaves the map", `{"paths":{"a.blp":"../escape"}}`, "Invalid asset path: ../escape", "", "relative path"},
		{"an absolute target", `{"paths":{"a.blp":"/absolute"}}`, "Invalid asset path: /absolute", "", "relative path"},
		{"a target on a drive", `{"paths":{"a.blp":"C:\\escape"}}`, `Invalid asset path: C:\escape`, "", "relative path"},
		{"the script as a target", `{"paths":{"a.blp":"war3map.lua"}}`, "Reserved map path: war3map.lua", "", "map internals"},
		{"the import index as a target", `{"paths":{"a.blp":"war3map.imp"}}`, "Reserved map path: war3map.imp", "", "map internals"},
		{"the JASS script as a target", `{"paths":{"a.blp":"scripts/war3map.j"}}`, "Reserved map path: scripts/war3map.j", "", "map internals"},
		{"a source that leaves assets", `{"paths":{"../a.blp":"x.blp"}}`, "Invalid asset path: ../a.blp", "", "relative path"},
		{"an exclusion that leaves assets", `{"exclude":["../x/"]}`, "Invalid asset path: ../x", "", "relative path"},
		{"a source that does not exist", `{"paths":{"missing":"x.blp"}}`,
			"assets.paths names a file that does not exist: assets/missing", manifestName, inBlock},
		{"an excluded source", `{"paths":{"a.blp":"x"},"exclude":["a.blp"]}`,
			"assets.paths names an excluded file: a.blp", manifestName, inBlock},
		{"a source that is skipped for its dot", `{"paths":{".hidden":"x"}}`,
			"assets.paths names an excluded file: .hidden", manifestName, inBlock},
		{"a source named twice by letter case", `{"paths":{"a.blp":"x.blp","A.BLP":"y.blp"}}`,
			"assets.paths names A.BLP twice.", manifestName, inBlock},
		{"two assets at one path", `{"paths":{"a.blp":"X.blp","b.blp":"x.blp"}}`,
			"Two assets would be imported as x.blp (target collision).", manifestName, inBlock},
		{"an asset inside an asset", `{"paths":{"a.blp":"x","b.blp":"x/y"}}`,
			"Asset x/y would sit inside the asset file x (file/folder collision).", manifestName, inBlock},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := refused(t, root, tt.block)
			if !strings.Contains(e.Msg, tt.words) || e.File != tt.file || !strings.Contains(e.Hint, tt.hint) {
				t.Errorf("error = %+v, want %q at %q with a hint about %q", e, tt.words, tt.file, tt.hint)
			}
		})
	}
}

func TestTheFirstBadMappingIsTheFirstWrittenAlsoWhereASourceLooksLikeANumber(t *testing.T) {
	root := t.TempDir()
	put(t, root, "assets/a.blp")
	e := refused(t, root, `{"paths":{"missing.blp":"x.blp","7":"y.blp","a.blp":"z.blp"}}`)
	if !strings.Contains(e.Msg, "assets/missing.blp") {
		t.Errorf("error = %+v, want it about missing.blp, which is written before 7", e)
	}
}

func TestAFileUnderAssetsNamedAsOneOfTheMapsOwnIsRefusedUnlessItIsMappedOrExcluded(t *testing.T) {
	root := t.TempDir()
	put(t, root, "assets/war3mapMap.blp")
	if e := refused(t, root, noBlock); e.Msg != "Reserved map path: war3mapMap.blp" || !strings.Contains(e.Hint, "settings.info.preview") {
		t.Errorf("error = %+v", e)
	}
	if collected, _ := collect(t, root, `{"exclude":["war3mapMap.blp"]}`); len(collected) != 0 {
		t.Errorf("excluded, it is still among the assets: %+v", rows(collected))
	}
	collected, _ := collect(t, root, `{"paths":{"war3mapMap.blp":"UI/Minimap.blp"}}`)
	if got := rows(collected); !slices.Equal(got, []row{{"", "war3mapMap.blp", "UI/Minimap.blp"}}) {
		t.Errorf("mapped, the assets are %+v", got)
	}
}

func TestCollectReadsThroughAProjectFolderThatIsALink(t *testing.T) {
	parent := t.TempDir()
	put(t, parent, "real/assets/a.blp")
	linked := filepath.Join(parent, "linked-root")
	testkit.LinkDir(t, filepath.Join(parent, "real"), linked)
	if collected, _ := collect(t, linked, noBlock); !slices.Equal(rows(collected), []row{{"", "a.blp", "a.blp"}}) {
		t.Errorf("assets through a linked project folder = %+v", rows(collected))
	}
}

func TestCollectRefusesALinkBelowTheProjectFolder(t *testing.T) {
	outside := t.TempDir()
	put(t, outside, "stray.blp")
	t.Run("in assets", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "assets/a.blp")
		testkit.LinkDir(t, outside, filepath.Join(root, "assets", "linked"))
		if e := refused(t, root, noBlock); !strings.Contains(e.Msg, "Symlinks") || e.File != "assets/linked" {
			t.Errorf("error = %+v", e)
		}
	})
	t.Run("in place of assets", func(t *testing.T) {
		root := t.TempDir()
		testkit.LinkDir(t, outside, filepath.Join(root, "assets"))
		if e := refused(t, root, noBlock); !strings.Contains(e.Msg, "Symlinks") || e.File != "assets" {
			t.Errorf("error = %+v", e)
		}
	})
	t.Run("in a library's folder", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "libraries/ui/a.blp")
		testkit.LinkDir(t, outside, filepath.Join(root, "libraries", "ui", "linked"))
		e := refused(t, root, noBlock, "ui")
		if !strings.HasPrefix(e.Msg, "Library ui: Symlinks") || e.File != "libraries/ui" || !strings.Contains(e.Hint, "library's author") {
			t.Errorf("error = %+v", e)
		}
	})
	// The link is on this machine and not among what the library ships, so the failure is not the library's. Its
	// file is the folder, by its path from the project folder, as for a link in place of assets.
	t.Run("in place of a library's folder", func(t *testing.T) {
		root := t.TempDir()
		put(t, root, "libraries/other/a.blp")
		link := filepath.Join(root, "libraries", "ui")
		testkit.LinkDir(t, outside, link)
		e := refused(t, root, noBlock, "ui")
		if e.Msg != "Symlinks are not supported: "+link || e.File != "libraries/ui" ||
			!strings.Contains(e.Hint, "Replace the link") {
			t.Errorf("error = %+v", e)
		}
	})
	// A library's folder outside the project is named by its path.
	t.Run("in place of a library's folder outside the project", func(t *testing.T) {
		root := t.TempDir()
		link := filepath.Join(t.TempDir(), "ui")
		testkit.LinkDir(t, outside, link)
		_, _, err := Collect(root, blockOf(t, noBlock), manifestName, []Library{{Key: "ui", Dir: link}})
		e := asError(t, err, "a library behind a link")
		if e.Msg != "Symlinks are not supported: "+link || e.File != filepath.ToSlash(link) {
			t.Errorf("error = %+v", e)
		}
	})
}

func TestCollectRefusesTwoSpellingsOfOnePath(t *testing.T) {
	root := t.TempDir()
	if !testkit.CaseSensitive(t, root) {
		t.Skip("this file system cannot hold two names that differ only in letter case")
	}
	put(t, root, "assets/a.blp")
	put(t, root, "assets/A.blp")
	if e := refused(t, root, noBlock); !strings.Contains(e.Msg, "letter case") || e.File != "assets/a.blp" {
		t.Errorf("error = %+v", e)
	}
	put(t, root, "libraries/ui/Icons/a.blp")
	put(t, root, "libraries/ui/icons/b.blp")
	os.Remove(filepath.Join(root, "assets", "A.blp"))
	e := refused(t, root, noBlock, "ui")
	if !strings.HasPrefix(e.Msg, "Library ui: ") || !strings.Contains(e.Msg, "letter case") || e.File != "libraries/ui" {
		t.Errorf("error = %+v", e)
	}
}

func TestCollectRefusesANameWindowsCannotHold(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this system cannot hold such names")
	}
	// A device's name with an extension is one: the game's tools on Windows could not open the file.
	for _, name := range []string{"what?.blp", "aux.blp"} {
		root := t.TempDir()
		put(t, root, "assets/Icons/"+name)
		if e := refused(t, root, noBlock); !strings.Contains(e.Msg, "Windows") || e.File != "assets/Icons/"+name {
			t.Errorf("%s: error = %+v", name, e)
		}
	}
}

func TestAFileWhereAFolderOfAssetsShouldBeIsRefused(t *testing.T) {
	root := t.TempDir()
	file := put(t, root, "assets", "a file")
	e := refused(t, root, noBlock)
	if e.Msg != "Expected a folder: assets" || e.File != "assets" || !strings.Contains(e.Hint, "remove the file") {
		t.Errorf("error = %+v", e)
	}
	os.Remove(file)
	// A library that ships a file where its folder of files should be: that is the library's.
	put(t, root, "libraries/ui", "a file")
	e = refused(t, root, noBlock, "ui")
	if e.Msg != "Library ui: Expected a folder: libraries/ui" || e.File != "libraries/ui" || !strings.Contains(e.Hint, "library's author") {
		t.Errorf("error = %+v", e)
	}
}

func TestAnAssetThatCannotBeReadIsRefusedByItsName(t *testing.T) {
	root := t.TempDir()
	put(t, root, "assets/a.blp")
	testkit.MakeUnreadable(t, put(t, root, "assets/Icons/held.blp"))
	e := refused(t, root, noBlock)
	if !strings.Contains(e.Msg, "Reading") || e.File != "assets/Icons/held.blp" || e.Hint == "" || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
}

func TestALibrarysFileThatCannotBeReadIsRefusedByItsNameAndIsNotTheLibrarysFailure(t *testing.T) {
	root := t.TempDir()
	testkit.MakeUnreadable(t, put(t, root, "libraries/ui/Icons/held.blp"))
	e := refused(t, root, noBlock, "ui")
	// What the system refuses is not for the library's author to mend: the error is the read's own, at the file.
	if !strings.HasPrefix(e.Msg, "Reading") || e.File != "libraries/ui/Icons/held.blp" || e.Cause == nil ||
		e.Hint == "" || strings.Contains(e.Hint, "library's author") {
		t.Errorf("error = %+v", e)
	}
}

func TestBeyondTheBasicPlaneAssetsAndLibrariesAreOrderedByBytes(t *testing.T) {
	// U+FF41 is three bytes that start with EF, and U+1F600 four that start with F0: by bytes the first comes
	// first, where an order by UTF-16 units puts the second, which is two surrogates, before it.
	const high, beyond = "\xef\xbd\x81", "\xf0\x9f\x98\x80"
	root := t.TempDir()
	for _, name := range []string{beyond + ".blp", high + ".blp", "shared.blp"} {
		put(t, root, "assets/"+name)
	}
	put(t, root, "libraries/"+beyond+"/shared.blp")
	put(t, root, "libraries/"+high+"/shared.blp")
	collected, replaced := collect(t, root, noBlock, beyond, high)
	if got := sources(collected); !slices.Equal(got, []string{"shared.blp", high + ".blp", beyond + ".blp"}) {
		t.Errorf("assets = %q", got)
	}
	// The libraries are taken in the order of their keys, which the lines about their files show.
	want := []string{
		"assets/shared.blp replaces library " + high + "'s shared.blp",
		"assets/shared.blp replaces library " + beyond + "'s shared.blp",
	}
	if !slices.Equal(replaced, want) {
		t.Errorf("replaced = %q", replaced)
	}
}

func TestLibraryAssetsFollowTheMapsOwnByKeyAndImportAtTheirPathInTheLibrary(t *testing.T) {
	root := t.TempDir()
	put(t, root, "assets/Models/Own.mdx", "own")
	put(t, root, "libraries/zeta/Sounds/Horn.wav", "horn")
	put(t, root, "libraries/alpha/war3mapImported/alpha/frames.toc", "toc")
	put(t, root, "libraries/alpha/.hidden/x.txt", "hidden")
	put(t, root, "libraries/other/never.txt", "of a library the project does not use")
	collected, replaced := collect(t, root, noBlock, "zeta", "alpha", "none")
	want := []row{
		{"", "Models/Own.mdx", "Models/Own.mdx"},
		{"zeta", "Sounds/Horn.wav", "Sounds/Horn.wav"},
		{"alpha", "war3mapImported/alpha/frames.toc", "war3mapImported/alpha/frames.toc"},
	}
	if got := rows(collected); !slices.Equal(got, want) || len(replaced) != 0 {
		t.Fatalf("assets = %+v, replaced %q", got, replaced)
	}
	if horn := collected[1]; string(horn.Bytes) != "horn" || horn.Hash != fsx.SHA256Hex([]byte("horn")) {
		t.Errorf("the library's asset holds %q with the hash %s", horn.Bytes, horn.Hash)
	}
	own, replaced := collect(t, root, noBlock)
	if !slices.Equal(rows(own), want[:1]) || len(replaced) != 0 {
		t.Errorf("without libraries: assets = %+v, replaced %q", rows(own), replaced)
	}
}

func TestTheMapsOwnAssetReplacesALibrarysAtTheSameInMapPathInAnyLetterCase(t *testing.T) {
	root := t.TempDir()
	put(t, root, "assets/custom/golem.blp", "the map's")
	put(t, root, "assets/Models/own.mdx", "the map's model")
	put(t, root, "libraries/lib/Textures/Golem.blp", "the library's")
	put(t, root, "libraries/lib/models/Own.mdx", "the library's model")
	put(t, root, "libraries/lib/Textures/Other.blp", "kept")
	collected, replaced := collect(t, root, `{"paths":{"custom/golem.blp":"textures/golem.blp"}}`, "lib")
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
	root := t.TempDir()
	put(t, root, "libraries/a/UI/Frame.fdf", "a")
	put(t, root, "libraries/b/ui/frame.fdf", "b")
	// The libraries are taken by key, in whatever order they are given.
	e := refused(t, root, noBlock, "b", "a")
	if e.Msg != `Libraries a and b both import ui\frame.fdf.` || e.File != manifestName ||
		!strings.Contains(e.Hint, "Drop one of the libraries") {
		t.Errorf("error = %+v", e)
	}
	put(t, root, "assets/UI/Frame.fdf", "the map's")
	collected, replaced := collect(t, root, noBlock, "b", "a")
	if !slices.Equal(rows(collected), []row{{"", "UI/Frame.fdf", "UI/Frame.fdf"}}) || len(replaced) != 2 {
		t.Errorf("assets = %+v, replaced %q", rows(collected), replaced)
	}
}

func TestALibraryFileWithAReservedPathOrInsideAnotherAssetFileIsRefused(t *testing.T) {
	reserved := t.TempDir()
	put(t, reserved, "libraries/bad/war3map.lua", "script")
	e := refused(t, reserved, noBlock, "bad")
	if e.Msg != "Library bad: Reserved map path: war3map.lua" || e.File != "libraries/bad" ||
		!strings.Contains(e.Hint, "Report it to the library's author") || e.Cause == nil {
		t.Errorf("error = %+v", e)
	}
	nested := t.TempDir()
	put(t, nested, "assets/data", "a file")
	put(t, nested, "libraries/lib/data/inner.txt", "inside it")
	if e := refused(t, nested, noBlock, "lib"); !strings.Contains(e.Msg, "file/folder collision") || e.File != manifestName {
		t.Errorf("error = %+v", e)
	}
}

func TestALibrarysFolderOutsideTheProjectIsNamedByItsPath(t *testing.T) {
	root, elsewhere := t.TempDir(), t.TempDir()
	put(t, elsewhere, "war3map.lua", "script")
	_, _, err := Collect(root, blockOf(t, noBlock), manifestName, []Library{{Key: "far", Dir: elsewhere}})
	if e := asError(t, err, "a reserved path"); e.File != filepath.ToSlash(elsewhere) {
		t.Errorf("error = %+v, want it at %s", e, filepath.ToSlash(elsewhere))
	}
}
