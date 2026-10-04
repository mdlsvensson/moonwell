package assets

import (
	"cmp"
	"context"
	"errors"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	oldassets "github.com/mdlsvensson/moonwell/internal/assets"
	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/layout"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// What this file compares, and what it leaves out. Both trees are given one project on disk, built by project:
// its files, the assets block of its manifest as pkl prints it, and the keys of its libraries.
//
//   - CollectProject against Collect: what is refused (kind, message, file and hint), else the assets in their
//     order (source, library, in-map path, bytes and hash) and the replaced lines. The other tree finds a
//     library's files under .moonwell/library-assets/<key>/ by itself; this tree is given those folders. The
//     projects are every scenario of the other tree's tests of collecting, an assets block of every kind over
//     one set of files, and seeded projects for the order of the assets, the names a map keeps for itself, and
//     libraries beside, over and inside each other.
//   - TargetPath against TargetPath, on every path the other tree's tests of the assets name, and a few more.
//   - The place of the state file, Locations against StateFile.
//   - The reader of the state file against ReadState. The other tree's reader is reached through its
//     PlanAssets, on a project whose map folder holds each file the state lists: the plan removes every file
//     the state owns, in the order read, so the removals are what was read. What is refused is compared whole.
//   - The writer of the state file, reached through the other tree's ApplyPlan, against State.Bytes: byte for
//     byte, and read back by ReadState.
//
// Compared in part, and counted. The first five are what a folder cannot hold or give, which this tree leaves to
// mapdir and its words: assets/ and each library's folder are read through mapdir.Open, as a map folder is.
//
//   - A link in assets/, in place of assets/, or in a library's folder
//     (TestCollectRefusesALinkBelowTheProjectFolder). The other tree names the link by its path on disk and
//     this tree by its path from the project folder, as the file of the error. Both must refuse and name the
//     same link, with the same hint; a link in a library's folder is the library's failure in both. A link in
//     place of a library's folder is compared whole: it is no failure of the library in either tree, and both
//     give the plain link error with the path on disk and the hint to replace the link.
//   - A file where assets/ or a library's folder should be (TestAFileWhereAFolderOfAssetsShouldBeIsRefused). The
//     other tree says "Expected a folder" with the path on disk and, for assets/, neither file nor hint; this
//     tree names the folder from the project folder, as the file of the error too, with a hint. Both must
//     refuse and name the same place; a library's folder is the library's failure in both, with one hint.
//   - Two paths that differ only in letter case (TestCollectRefusesTwoSpellingsOfOnePath), on a file system
//     that holds both. Both must refuse and name the second of the two.
//   - A name Windows cannot hold (TestCollectRefusesANameWindowsCannotHold), on a system that holds one. The
//     other tree calls it an invalid asset path; both must refuse and name the file. A name with a backslash is
//     a path to the other tree, which then fails to read a file that is not there, as an error that is not a
//     diag error: there only the refusal itself is compared.
//   - A file that cannot be read, of the map's own or of a library (TestAnAssetThatCannotBeReadIsRefusedByItsName,
//     TestALibrarysFileThatCannotBeReadIsRefusedByItsNameAndIsNotTheLibrarysFailure). The other tree passes
//     the system's error on, which names the file's path on disk, as an error that is not a diag error; a
//     command shows that as an internal error. It is decided on that. This tree must refuse with mapdir's
//     failure at the same file, and in neither tree is it a library's. The test is skipped where a file cannot
//     be made unreadable.
//   - A map folder's name that is no path, for the place of the state file. The other tree places the map
//     folder first and names that path; this tree names the state file's
//     (TestStateFileIsUnderAssetStateByTheMapFoldersName).
//
// Not among the inputs:
//
//   - An assets block with more than one mapping whose source looks like a number ("7"), and a state file that
//     lists such a path among others. This tree keeps the order written and the other read such keys first, so
//     the first of two problems, and the order of the files of a state, differ there
//     (TestTheFirstBadMappingIsTheFirstWrittenAlsoWhereASourceLooksLikeANumber,
//     TestTheFirstBadEntryOfAStateIsTheFirstWrittenAlsoWhereAPathLooksLikeANumber, and the case of
//     TestStateBytesAreTheFilesInTheOrderGivenWithTwoSpacesAndAFinalLineBreak). One mapping of such a source,
//     which has no order, is among the projects.
//   - A state without files, for State.Bytes: the other tree writes no file for it.
//   - A state file that lists a path with a backslash and is read: the map folder holds the file under "/", so
//     the removals of the other tree's plan do not show the spelling of the state
//     (TestReadStateKeepsThePathsAsWrittenInTheOrderWritten). Such a path is among the refused states.
//   - A link above a library's folder (.moonwell as a junction): this tree is given the folder and does not
//     know the way to it.
//   - Names and library keys with a character beyond the basic plane beside one from U+E000 to U+FFFF: this tree
//     orders by bytes and the other by UTF-16 units (TestBeyondTheBasicPlaneAssetsAndLibrariesAreOrderedByBytes).

// oldManifest is the manifest the other tree names in every error about the assets block.
const oldManifest = "moonwell.pkl"

// mapFolder is the source map of every project here that has one.
const mapFolder = "map.w3x"

// shippedUnder is where the other tree looks for the files a library ships, from the project folder.
const shippedUnder = layout.LibraryAssetsDir + "/"

// project is a temporary project for both trees.
type project struct {
	name      string
	files     map[string]string // by path from the project folder, with "/": what each file holds
	folders   []string          // folders to make besides those the files are in
	block     string            // the manifest's assets block as pkl prints it; "" for a block that sets nothing
	libraries []string          // the keys of the libraries, whose files are under shippedUnder
	refused   bool              // whether both trees refuse the project
}

// holding is files that each hold their own name, so that no two hold the same bytes.
func holding(names ...string) map[string]string {
	files := map[string]string{}
	for _, name := range names {
		files[name] = bytesOf(name)
	}
	return files
}

// bytesOf is what holding puts into the file of a name.
func bytesOf(name string) string { return "the bytes of " + name }

// onDisk writes the project into a folder of its own.
func (p project) onDisk(t testing.TB) string {
	t.Helper()
	root := t.TempDir()
	for _, folder := range p.folders {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(folder)), 0o777); err != nil {
			t.Fatal(err)
		}
	}
	for name, content := range p.files {
		testkit.WriteFile(t, root, name, []byte(content))
	}
	return root
}

// blocks is the project's assets block as each tree holds it.
func (p project) blocks(t testing.TB) (oldassets.Config, manifest.Assets) {
	t.Helper()
	block := blockOf(t, cmp.Or(p.block, noBlock))
	paths := &ordered.Map[string]{}
	for source, target := range block.Paths.All() {
		paths.Set(source, target)
	}
	return oldassets.Config{Paths: paths, Exclude: block.Exclude}, block
}

// shippedIn is the project's libraries as this tree is given them: each with the folder the other tree reads.
func (p project) shippedIn(root string) []Library {
	var libraries []Library
	for _, key := range p.libraries {
		libraries = append(libraries, Library{Key: key, Dir: filepath.Join(root, filepath.FromSlash(shippedUnder), key)})
	}
	return libraries
}

// converted is the assets the other tree collected, in the type of this tree.
func converted(old []*oldassets.Asset) []Asset {
	assets := []Asset{}
	for _, asset := range old {
		assets = append(assets, Asset{
			Source: asset.Source, Library: asset.Library, Target: asset.Target, Bytes: asset.Bytes, Hash: asset.Hash,
		})
	}
	return assets
}

// compareCollected gives a project on disk to both trees, compares all they return, and reports whether both
// refused it.
func compareCollected(t *testing.T, p project, root string) (refused bool) {
	t.Helper()
	oldBlock, block := p.blocks(t)
	want, wantReplaced, wantErr := oldassets.CollectProject(root, oldBlock, p.libraries)
	got, gotReplaced, gotErr := Collect(root, block, oldManifest, p.shippedIn(root))
	if oracle.Refusals(t, p.name, wantErr, gotErr) {
		return true
	}
	if wantErr != nil || gotErr != nil {
		return false // one tree refused alone, which Refusals has reported
	}
	oracle.Values(t, p.name+": the assets", converted(want), got)
	oracle.Values(t, p.name+": the replaced lines", wantReplaced, gotReplaced)
	return false
}

// ---- the projects ----

// scenarioProjects is the projects of the other tree's tests of collecting, and of its tests of the plan that
// have libraries.
func scenarioProjects() []project {
	const sword = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	const disabled = "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp"
	const s = shippedUnder
	byKey := holding("assets/Models/Own.mdx", s+"zeta/Sounds/Horn.wav", s+"alpha/war3mapImported/alpha/frames.toc",
		s+"alpha/.hidden/x.txt", s+"other/never.txt")
	return []project{
		{name: "mapped, excluded, and a dot file",
			files: holding("assets/"+sword, "assets/icons/disabled.blp", "assets/credits/readme.txt", "assets/.gitkeep"),
			block: `{"paths":{"icons/disabled.blp":"` + disabled + `"},"exclude":["credits/"]}`},
		{name: "no assets folder"},
		{name: "libraries by key, with a dot folder, one the project does not use and one without files",
			files: byKey, libraries: []string{"zeta", "alpha", "none"}},
		{name: "the same project without libraries", files: byKey},
		{name: "the map's own assets replace a library's in any letter case",
			files: holding("assets/custom/golem.blp", "assets/Models/own.mdx", s+"lib/Textures/Golem.blp",
				s+"lib/models/Own.mdx", s+"lib/Textures/Other.blp"),
			block: `{"paths":{"custom/golem.blp":"textures/golem.blp"}}`, libraries: []string{"lib"}},
		{name: "two libraries at one in-map path",
			files: holding(s+"a/UI/Frame.fdf", s+"b/ui/frame.fdf"), libraries: []string{"b", "a"}, refused: true},
		{name: "two libraries at one in-map path that the map's own file has",
			files:     holding("assets/UI/Frame.fdf", s+"a/UI/Frame.fdf", s+"b/ui/frame.fdf"),
			libraries: []string{"b", "a"}},
		{name: "a library file with a reserved path",
			files: holding(s + "bad/war3map.lua"), folders: []string{"assets"}, libraries: []string{"bad"}, refused: true},
		{name: "a library file inside an asset file",
			files: holding("assets/data", s+"lib/data/inner.txt"), libraries: []string{"lib"}, refused: true},
		{name: "a library's files beside the map's own, one of them replaced",
			files: holding("assets/Models/Own.mdx", "assets/icons/shared.blp", s+"ui/war3mapImported/ui/frames.toc",
				s+"ui/icons/shared.blp", s+"unlisted/never.txt"),
			libraries: []string{"ui"}},
		{name: "a library alone", files: holding(s + "ui/Models/Golem.mdx"), folders: []string{"assets"}, libraries: []string{"ui"}},
	}
}

// The assets blocks that both trees refuse for the files of blockProjects. The first ten are those of the other
// tree's tests.
var refusedBlocks = []string{
	`{"paths":{"a.blp":"../escape"}}`,
	`{"paths":{"a.blp":"/absolute"}}`,
	`{"paths":{"a.blp":"C:\\escape"}}`,
	`{"paths":{"a.blp":"war3map.lua"}}`,
	`{"paths":{"a.blp":"war3map.imp"}}`,
	`{"paths":{"a.blp":"scripts/war3map.j"}}`,
	`{"paths":{"missing":"x.blp"}}`,
	`{"paths":{"a.blp":"X.blp","b.blp":"x.blp"}}`,
	`{"paths":{"a.blp":"x","b.blp":"x/y"}}`,
	`{"paths":{"a.blp":"x"},"exclude":["a.blp"]}`,
	// A source or an exclusion that is no path.
	`{"paths":{"../a.blp":"x.blp"}}`,
	`{"paths":{"a.blp//":"x.blp"}}`,
	`{"exclude":["../x/"]}`,
	`{"exclude":["a.blp//"]}`,
	`{"exclude":["sub\\/"]}`,
	`{"exclude":[""]}`,
	`{"exclude":["/"]}`,
	// A source that is not a file to import.
	`{"paths":{".hidden":"x"}}`,
	`{"paths":{"sub":"x"}}`,
	`{"paths":{"a.blp":"x"},"exclude":["A.BLP/"]}`,
	`{"paths":{"sub/c.blp":"x"},"exclude":["SUB\\"]}`,
	`{"paths":{"a.blp":"x.blp","A.BLP":"y.blp"}}`,
	`{"paths":{"sub/c.blp":"x.blp","sub\\c.blp":"y.blp"}}`,
	// A target that is taken, or that is a folder on the way to another, or the other way around.
	`{"paths":{"a.blp":"b.blp"}}`,
	`{"paths":{"a.blp":"B.BLP"}}`,
	`{"paths":{"a.blp":"sub"}}`,
	`{"paths":{"a.blp":"SUB"}}`,
	`{"paths":{"a.blp":"b.blp/inner.blp"}}`,
	`{"paths":{"a.blp":"x/y/z","b.blp":"X"}}`,
	// A target that is no in-map path an asset may have.
	`{"paths":{"a.blp":""}}`,
	`{"paths":{"a.blp":"x/"}}`,
	`{"paths":{"a.blp":"con.blp"}}`,
	`{"paths":{"a.blp":"war3mapMap.blp"}}`,
	`{"paths":{"a.blp":"WAR3MAPPREVIEW.TGA"}}`,
	`{"paths":{"a.blp":"war3mapImported"}}`,
	`{"paths":{"a.blp":"(listfile)"}}`,
	`{"paths":{"a.blp":"Scripts\\War3map.j"}}`,
	// Which of two problems is told: the exclusions are read before the mappings, and the mappings in order.
	`{"paths":{"missing":"x.blp"},"exclude":["../x"]}`,
	`{"paths":{"a.blp":"../escape","missing":"x"}}`,
	`{"paths":{"missing":"x","a.blp":"../escape"}}`,
	`{"paths":{"a.blp":"x","b.blp":"X","missing":"y"}}`,
	`{"paths":{"a.blp":"x","b.blp":"x/y","sub/c.blp":"x"}}`,
}

// The assets blocks that both trees take for the files of blockProjects.
var takenBlocks = []string{
	``,
	`{"paths":{},"exclude":[]}`,
	`{"paths":{"a.blp":"Textures\\A.blp"}}`,
	`{"paths":{"A.BLP":"x.blp"}}`,
	`{"paths":{"sub\\c.blp":"UI/c.blp"}}`,
	`{"paths":{"SUB/C.BLP":"c.blp","a.blp":"sub/c.blp"}}`,
	`{"paths":{"a.blp":"b.blp","b.blp":"a.blp"}}`,
	`{"paths":{"a.blp":"B/z.blp","b.blp":"a/y.blp"}}`,
	`{"paths":{"a.blp":"war3mapImported/a.blp"}}`,
	`{"paths":{"a.blp":"War3mapImported\\war3map.lua"}}`,
	`{"paths":{"a.blp":"scripts/common.j"}}`,
	`{"paths":{"a.blp":"x/y/z","b.blp":"x/y/w"}}`,
	`{"paths":{"a.blp":"x"},"exclude":["b.blp"]}`,
	`{"exclude":["sub/"]}`,
	`{"exclude":["SUB\\"]}`,
	`{"exclude":["sub"]}`,
	`{"exclude":["sub/c.blp"]}`,
	`{"exclude":["Sub\\C.blp"]}`,
	`{"exclude":["a.blp/"]}`,
	`{"exclude":["A.BLP","b.blp","sub/"]}`,
	`{"exclude":["missing","missing/",".hidden"]}`,
	`{"exclude":["a"]}`,
}

// blockProjects is one set of files under each assets block of refusedBlocks and takenBlocks.
func blockProjects() []project {
	var projects []project
	for _, block := range refusedBlocks {
		projects = append(projects, project{name: "the block " + block, block: block, refused: true})
	}
	for _, block := range takenBlocks {
		projects = append(projects, project{name: "the block " + block, block: block})
	}
	for i := range projects {
		projects[i].files = holding("assets/a.blp", "assets/b.blp", "assets/.hidden", "assets/sub/c.blp")
	}
	return projects
}

// seededProjects is projects for what the scenarios and the blocks do not reach.
func seededProjects() []project {
	const s = shippedUnder
	sorted := holding("assets/b.blp", "assets/A.blp", "assets/a/c.blp", "assets/Z.blp", "assets/_x.blp", "assets/a-b.blp",
		"assets/a b.blp", "assets/\xc3\x89cole.blp", "assets/\xc3\xa9t\xc3\xa9.blp", "assets/Abc/D.blp", "assets/abc.blp")
	sorted["assets/empty.blp"] = ""
	return []project{
		{name: "names in both letter cases, outside ASCII and with a space, and a file without bytes", files: sorted},
		{name: "a file named as the map's script", files: holding("assets/a.blp", "assets/war3map.lua"), refused: true},
		{name: "a file named as the JASS script in its folder", files: holding("assets/Scripts/war3map.j"), refused: true},
		{name: "a file named as a picture for the map list", files: holding("assets/war3mapPreview.tga"), refused: true},
		{name: "a file named as a picture for the map list, excluded",
			files: holding("assets/war3mapPreview.tga"), block: `{"exclude":["war3mappreview.tga"]}`},
		{name: "a file named as the map's script, mapped",
			files: holding("assets/war3map.lua"), block: `{"paths":{"war3map.lua":"Scripts/Mine.lua"}}`},
		{name: "a file in the folder World Editor imports into", files: holding("assets/war3mapImported/sound.wav")},
		{name: "a script that is not the map's", files: holding("assets/Scripts/common.j", "assets/Units/war3map.lua")},
		{name: "dot folders and dot files below folders",
			files: holding("assets/.git/config", "assets/icons/.DS_Store", "assets/icons/..b", "assets/icons/a.blp")},
		{name: "only a dot file", files: holding("assets/.keep")},
		{name: "an empty assets folder", folders: []string{"assets"}},
		// One mapping has no order, so the two trees agree on a source that looks like a number.
		{name: "a source that looks like a number",
			files: holding("assets/7", "assets/a.blp"), block: `{"paths":{"7":"seven.blp"}}`},
		{name: "an asset of one library inside a file of another",
			files: holding(s+"a/data", s+"b/data/inner.txt"), libraries: []string{"a", "b"}, refused: true},
		{name: "one of the map's own assets inside a library's file",
			files: holding("assets/data/inner.txt", s+"lib/data"), libraries: []string{"lib"}, refused: true},
		{name: "a library file named as the JASS script in its folder",
			files: holding(s + "bad/scripts/war3map.j"), libraries: []string{"bad"}, refused: true},
		{name: "three libraries, the first and the last at one path",
			files: holding(s+"c/x.blp", s+"b/y.blp", s+"a/X.BLP"), libraries: []string{"c", "b", "a"}, refused: true},
		{name: "two libraries at one path, a key in capitals before one in small letters",
			files: holding(s+"a/x.blp", s+"B/X.blp"), libraries: []string{"a", "B"}, refused: true},
		{name: "an exclusion does not reach a library's files",
			files: holding("assets/Sounds/own.wav", s+"zeta/Sounds/Horn.wav"), block: `{"exclude":["Sounds/"]}`,
			libraries: []string{"zeta"}},
		{name: "an excluded file of the map's own does not replace a library's",
			files: holding("assets/icons/shared.blp", s+"ui/icons/shared.blp"), block: `{"exclude":["icons/shared.blp"]}`,
			libraries: []string{"ui"}},
		{name: "the map's own assets inside each other, told before a library's reserved path",
			files: holding("assets/a.blp", "assets/b.blp", s+"bad/war3map.lua"),
			block: `{"paths":{"a.blp":"x","b.blp":"x/y"}}`, libraries: []string{"bad"}, refused: true},
		{name: "the files of two libraries among the map's own, in the order of their paths",
			files:     holding("assets/b.blp", "assets/D.blp", s+"m/A.blp", s+"m/c/e.blp", s+"k/C.blp", s+"k/\xc3\xa9.blp"),
			libraries: []string{"m", "k"}},
		{name: "a library that ships only a dot file", files: holding(s + "quiet/.gitkeep"), libraries: []string{"quiet"}},
		{name: "a library's file and the map's own that is mapped onto it, and one mapped away from it",
			files: holding("assets/a.blp", "assets/Icons/x.blp", s+"ui/Icons/X.blp", s+"ui/a.blp"),
			block: `{"paths":{"a.blp":"icons/x.blp","Icons/x.blp":"moved.blp"}}`, libraries: []string{"ui"}},
	}
}

func TestOracleOnWhatAProjectImports(t *testing.T) {
	refused, collected := 0, 0
	for _, p := range slices.Concat(scenarioProjects(), blockProjects(), seededProjects()) {
		bothRefused := compareCollected(t, p, p.onDisk(t))
		if bothRefused != p.refused {
			t.Errorf("%s: both trees refuse it: %v, want %v", p.name, bothRefused, p.refused)
		}
		if bothRefused {
			refused++
		} else {
			collected++
		}
	}
	// Refused: three scenarios, the forty-two refused blocks and nine seeded projects.
	if refused != 54 || collected != 44 {
		t.Errorf("%d projects refused and %d collected, want 54 and 44", refused, collected)
	}
}

// ---- what a folder cannot hold ----

// bothDiag is two refusals as the expected failure each must be in its tree.
func bothDiag(t *testing.T, what string, wantErr, gotErr error) (want *olddiag.Error, got *diag.Error, ok bool) {
	t.Helper()
	if !errors.As(wantErr, &want) || !errors.As(gotErr, &got) {
		t.Errorf("%s: the refusals are %v and %v, want a diag error of each tree", what, wantErr, gotErr)
		return nil, nil, false
	}
	return want, got, true
}

// refusals is what both trees refuse a project on disk with.
func refusals(t *testing.T, p project, root string) (wantErr, gotErr error) {
	t.Helper()
	oldBlock, block := p.blocks(t)
	_, _, wantErr = oldassets.CollectProject(root, oldBlock, p.libraries)
	_, _, gotErr = Collect(root, block, oldManifest, p.shippedIn(root))
	return wantErr, gotErr
}

func TestOracleOnALink(t *testing.T) {
	const library = layout.LibraryAssetsDir + "/ui"
	const words, inLibrary = "Symlinks are not supported: ", "Library ui: "
	tests := []struct {
		name, link        string // where the link is, from the project folder
		libraries         []string
		start             string // the words before the place of the link
		wantFile, gotFile string
		whole             bool // whether the two trees refuse in the same words
	}{
		{"inside assets", "assets/Icons/linked", nil, words, "", "assets/Icons/linked", false},
		{"in place of assets", "assets", nil, words, "", "assets", false},
		{"inside a library's folder", library + "/linked", []string{"ui"}, inLibrary + words, library, library, false},
		{"in place of a library's folder", library, []string{"ui"}, words, "", "", true},
	}
	inPart, whole := 0, 0
	for _, tt := range tests {
		p := project{name: "a link " + tt.name, folders: []string{path.Dir(tt.link)}, libraries: tt.libraries}
		root, outside := p.onDisk(t), t.TempDir()
		put(t, outside, "stray.blp")
		onDisk := filepath.Join(root, filepath.FromSlash(tt.link))
		testkit.LinkDir(t, outside, onDisk)
		wantErr, gotErr := refusals(t, p, root)
		want, got, ok := bothDiag(t, p.name, wantErr, gotErr)
		switch {
		case !ok:
		case tt.whole:
			if oracle.Refusals(t, p.name, wantErr, gotErr) && want.Msg == tt.start+onDisk {
				whole++
			}
		case want.Msg != tt.start+onDisk || want.File != tt.wantFile || got.Msg != tt.start+tt.link ||
			got.File != tt.gotFile || got.Hint != want.Hint:
			t.Errorf("%s: the refusals differ in more than how they name the link:\nwant: %+v\ngot:  %+v", p.name, want, got)
		default:
			inPart++
		}
	}
	if inPart != 3 || whole != 1 {
		t.Errorf("%d refusals compared in part and %d whole, want 3 and 1", inPart, whole)
	}
}

func TestOracleOnAFileWhereAFolderShouldBe(t *testing.T) {
	const library = layout.LibraryAssetsDir + "/ui"
	const words = "Expected a folder: "
	tests := []struct {
		name, file string // the file, from the project folder
		libraries  []string
		before     string // what a library's failure starts with
		wantFile   string
	}{
		{"assets", "assets", nil, "", ""},
		{"a library's folder", library, []string{"ui"}, "Library ui: ", library},
	}
	compared := 0
	for _, tt := range tests {
		p := project{name: "a file in place of " + tt.name, files: map[string]string{tt.file: "a file"}, libraries: tt.libraries}
		root := p.onDisk(t)
		wantErr, gotErr := refusals(t, p, root)
		want, got, ok := bothDiag(t, p.name, wantErr, gotErr)
		if !ok {
			continue
		}
		// The other tree has the message alone for assets, with the path on disk in it; a library's failure has the
		// same file and hint in both.
		sameHint := tt.wantFile == "" || got.Hint == want.Hint
		if want.Msg != tt.before+words+filepath.Join(root, filepath.FromSlash(tt.file)) || want.File != tt.wantFile ||
			got.Msg != tt.before+words+tt.file || got.File != tt.file || got.Hint == "" || !sameHint {
			t.Errorf("%s: the refusals differ in more than how they name the file:\nwant: %+v\ngot:  %+v", p.name, want, got)
			continue
		}
		compared++
	}
	if compared != 2 {
		t.Errorf("%d refusals compared, want 2", compared)
	}
}

func TestOracleOnTwoSpellingsOfOnePath(t *testing.T) {
	if !testkit.CaseSensitive(t, t.TempDir()) {
		t.Skip("this file system cannot hold two names that differ only in letter case")
	}
	const library = layout.LibraryAssetsDir + "/ui"
	const words, inLibrary = "Two paths differ only in letter case: ", "Library ui: "
	tests := []struct {
		name              string
		files             []string
		libraries         []string
		wantMsg           string
		wantFile, gotFile string
		gotWords          string
	}{
		{"two files under assets", []string{"assets/Icons/a.blp", "assets/Icons/A.blp"}, nil,
			words + "Icons/a.blp", "", "assets/Icons/a.blp", "Icons/A.blp and Icons/a.blp"},
		{"two folders of a library", []string{library + "/Icons/a.blp", library + "/icons/b.blp"}, []string{"ui"},
			inLibrary + words + "icons", library, library, "Library ui: Map paths Icons and icons"},
	}
	compared := 0
	for _, tt := range tests {
		p := project{name: tt.name, files: holding(tt.files...), libraries: tt.libraries}
		wantErr, gotErr := refusals(t, p, p.onDisk(t))
		want, got, ok := bothDiag(t, p.name, wantErr, gotErr)
		if !ok {
			continue
		}
		if want.Msg != tt.wantMsg || want.File != tt.wantFile || got.File != tt.gotFile || !strings.Contains(got.Msg, tt.gotWords) {
			t.Errorf("%s: the refusals differ in more than their words:\nwant: %+v\ngot:  %+v", p.name, want, got)
		}
		compared++
	}
	if compared != 2 {
		t.Errorf("%d refusals compared, want 2", compared)
	}
}

func TestOracleOnANameWindowsCannotHold(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("this system cannot hold such names")
	}
	const library = layout.LibraryAssetsDir + "/ui"
	const words, unusable = "Invalid asset path: ", " has a name that cannot be used in a map that Windows tools read."
	tests := []struct {
		name      string // of the file, from the folder it is in
		folder    string
		libraries []string
		before    string // what a library's failure starts with
		wantFile  string
	}{
		{"what?.blp", "assets", nil, "", ""},
		{"Icons/aux.blp", "assets", nil, "", ""},
		{"Icons/a:b.blp", "assets", nil, "", ""},
		{"icon.", library, []string{"ui"}, "Library ui: ", library},
	}
	compared := 0
	for _, tt := range tests {
		p := project{name: "a file named " + tt.name, files: holding(tt.folder + "/" + tt.name), libraries: tt.libraries}
		wantErr, gotErr := refusals(t, p, p.onDisk(t))
		want, got, ok := bothDiag(t, p.name, wantErr, gotErr)
		if !ok {
			continue
		}
		gotFile := cmp.Or(tt.wantFile, tt.folder+"/"+tt.name)
		if want.Msg != tt.before+words+tt.name || want.File != tt.wantFile ||
			got.Msg != tt.before+tt.folder+"/"+tt.name+unusable || got.File != gotFile {
			t.Errorf("%s: the refusals differ in more than their words:\nwant: %+v\ngot:  %+v", p.name, want, got)
		}
		compared++
	}
	// A backslash in a name is a separator to the other tree, which then finds no file at that path.
	p := project{name: `a file named a\b.blp`, files: holding(`assets/a\b.blp`)}
	wantErr, gotErr := refusals(t, p, p.onDisk(t))
	if _, expected := olddiag.First(wantErr); expected || !oracle.Errors(t, p.name, wantErr, gotErr) {
		t.Errorf("%s: the refusals are %v and %v, want an error that is not a diag error of the other tree", p.name, wantErr, gotErr)
	} else {
		compared++
	}
	if compared != 5 {
		t.Errorf("%d refusals compared, want 5", compared)
	}
}

func TestOracleOnAFileThatCannotBeRead(t *testing.T) {
	const library = layout.LibraryAssetsDir + "/ui"
	tests := []struct {
		name, held string // the file that cannot be read, from the project folder
		libraries  []string
		gotFile    string
	}{
		{"one of the map's own assets", "assets/Icons/held.blp", nil, "assets/Icons/held.blp"},
		{"a library's file", library + "/Icons/held.blp", []string{"ui"}, library + "/Icons/held.blp"},
	}
	compared := 0
	for _, tt := range tests {
		p := project{name: tt.name, files: holding("assets/a.blp", tt.held), libraries: tt.libraries}
		root := p.onDisk(t)
		onDisk := filepath.Join(root, filepath.FromSlash(tt.held))
		makeUnreadable(t, onDisk)
		wantErr, gotErr := refusals(t, p, root)
		if _, expected := olddiag.First(wantErr); expected || !oracle.Errors(t, p.name, wantErr, gotErr) {
			t.Errorf("%s: the refusals are %v and %v, want an error that is not a diag error of the other tree", p.name, wantErr, gotErr)
			continue
		}
		// Both name the file, the other tree by its path on disk in the system's error, and neither says that the
		// failure is a library's.
		got := asError(t, gotErr, p.name)
		if !strings.Contains(wantErr.Error(), onDisk) || got.File != tt.gotFile || !strings.HasPrefix(got.Msg, "Reading a map file failed: ") {
			t.Errorf("%s: the refusals do not name the same file:\nwant: %v\ngot:  %+v", p.name, wantErr, got)
			continue
		}
		compared++
	}
	if compared != 2 {
		t.Errorf("%d refusals compared, want 2", compared)
	}
}

// ---- in-map paths ----

// targetPaths is every path the other tree's tests of the assets name, as a source, as a target, as an import or
// as a file of a map, and more of each kind TargetPath tells apart.
var targetPaths = []string{
	// Its test of TargetPath.
	"war3map.lua", "war3map.imp", "WAR3MAP.W3I", "scripts/war3map.j", "(listfile)", "war3mapMap.blp", "war3mapPreview.tga",
	"WAR3MAPPREVIEW.BLP", "war3mapMap.tga", "war3mapMisc.txt", "war3mapPreviews/a.tga", "war3mapImported/sound.wav",
	"../escape", `Textures\A.BLP`,
	// Its tests of collecting.
	"ReplaceableTextures/CommandButtons/BTNSword.blp", "ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp",
	"icons/disabled.blp", "credits/readme.txt", "credits/", ".gitkeep", "a.blp", "b.blp", "/absolute", `C:\escape`,
	"missing", "x.blp", "X.blp", "x", "x/y", "A.blp", "linked", "b.txt", "a/z.txt", "a/B.txt", "c/d/e.txt", "a.txt",
	"inner/x.blp", "Models/Own.mdx", "Sounds/Horn.wav", "war3mapImported/alpha/frames.toc", ".hidden/x.txt", "never.txt",
	"custom/golem.blp", "Models/own.mdx", "textures/golem.blp", "Textures/Golem.blp", "models/Own.mdx",
	"Textures/Other.blp", "UI/Frame.fdf", "ui/frame.fdf", "data", "data/inner.txt", "default.blp", `Textures\custom.blp`,
	"wa3mapPreview.tga",
	// Its tests of the plan.
	"war3mapImported/existing.wav", "existing.wav", "Models/unit.mdx", "unmanaged.txt", "Models/renamed.mdx",
	"Textures/a.blp", "Textures/b.blp", "Textures/c.blp", `Textures\c.blp`, "Textures", "textures/a.blp",
	"Textures/existing.blp", "textures/new.blp", "Textures/new.blp", "A.BLP", "textures/b.blp", "old.blp",
	"old.blp/inner.blp", "gone.blp", "icons/shared.blp", "war3mapImported/ui/frames.toc", `icons\shared.blp`,
	"Models/Golem.mdx",
	// More of each kind.
	"", ".", "a//b", "a/./b", "a/", "con", "CON.blp", "com1.blp", "a.", "a ", "a\tb", "a|b", `"a"`, "a<b", "a:b",
	"console.blp", "..a", ".a", "a..b", "caf\xc3\xa9/\xc3\x89cole.blp",
	"(attributes)", "(signature)", "(LISTFILE)", "(listfile).txt", "listfile", "war3campaign.w3u", "War3Campaign.w3f",
	"war3map", "war3mapImported", "war3mapimported/", "WAR3MAPIMPORTED/x.blp", `war3mapImported\x.blp`,
	"war3mapImported2/x.blp", "war3mapImportedx", "x/war3map.lua", "war3.blp", "war3map/x.blp",
	"scripts/war3map", "scripts/war3map.", "Scripts/War3Map.J", `scripts\war3map.j`, "scripts/war3mapx.j",
	"scripts/common.j", "x/scripts/war3map.j", "war3mapMap", "war3mapMap.", "war3mapPreview", "war3mapMapx.blp",
	// The long s, which is an s to a pattern that ignores letter case.
	"\xc5\xbfcripts/war3map.j", "(li\xc5\xbftfile)",
}

func TestOracleOnTargetPath(t *testing.T) {
	taken, refused := 0, 0
	for _, value := range targetPaths {
		want, wantErr := oldassets.TargetPath(value)
		got, gotErr := TargetPath(value)
		switch {
		case oracle.Refusals(t, "the path "+value, wantErr, gotErr):
			refused++
		case wantErr == nil && gotErr == nil:
			oracle.Values(t, "the path "+value, want, got)
			taken++
		}
	}
	if taken != 77 || refused != 51 {
		t.Errorf("%d paths taken and %d refused, want 77 and 51", taken, refused)
	}
}

// ---- the state file ----

func TestOracleOnThePlaceOfTheStateFile(t *testing.T) {
	root := t.TempDir()
	placed, refused := 0, 0
	for _, folder := range []string{"map.w3x", "My Map.w3x", "campaign/map.w3x", `campaign\map.w3x`,
		"../map.w3x", "what?.w3x", "con/map.w3x", "campaign//map.w3x"} {
		what := "the map folder " + folder
		_, want, wantErr := oldassets.Locations(root, folder)
		got, gotErr := StateFile(root, folder)
		// A name that is no path is refused by both, each naming the path it tried to place.
		if oracle.Errors(t, what, wantErr, gotErr) {
			refused++
			continue
		}
		oracle.Values(t, what, want, got)
		placed++
	}
	if placed != 4 || refused != 4 {
		t.Errorf("%d state files placed and %d refused, want 4 and 4", placed, refused)
	}
}

// hashOf is the hash of the bytes holding gives the file of a name.
func hashOf(name string) string { return fsx.SHA256Hex([]byte(bytesOf(name))) }

// listing is a member of a state's files: the path with the hash of the file a map folder of holding has there.
func listing(path string) string { return `"` + path + `":"` + hashOf(path) + `"` }

// stateDocument is what a state file holds, and the paths it lists when both trees read it.
type stateDocument struct {
	name, document string
	owned          []string
}

// readDocuments is the states both trees read.
func readDocuments() []stateDocument {
	two := []string{"b.blp", "Textures/A.blp"}
	return []stateDocument{
		{"two files", `{"version":1,"files":{` + listing("b.blp") + `,` + listing("Textures/A.blp") + `}}`, two},
		{"as the other tree writes it",
			"{\n  \"version\": 1,\n  \"files\": {\n    " + listing("b.blp") + ",\n    " + listing("Textures/A.blp") + "\n  }\n}\n", two},
		{"the files before the version, which is written as a fraction, and members a state does not have",
			`{"files":{` + listing("a.blp") + `},"note":[1,{"a":null}],"version":1.0}`, []string{"a.blp"}},
		{"the version with an exponent", `{"version":10e-1,"files":{` + listing("a.blp") + `}}`, []string{"a.blp"}},
		{"paths outside ASCII, with a space, and in the folder World Editor imports into",
			`{"version":1,"files":{` + listing("war3mapImported/My File.wav") + `,` + listing("caf\xc3\xa9/\xc3\x89cole.blp") + `}}`,
			[]string{"war3mapImported/My File.wav", "caf\xc3\xa9/\xc3\x89cole.blp"}},
		{"a path written with an escape", `{"version":1,"files":{"\u0061.blp":"` + hashOf("a.blp") + `"}}`, []string{"a.blp"}},
		{"a path written twice has its last hash",
			`{"version":1,"files":{"a.blp":"bad",` + listing("b.blp") + `,` + listing("a.blp") + `}}`, []string{"a.blp", "b.blp"}},
		{"a version written twice has its last value", `{"version":2,"files":{` + listing("a.blp") + `},"version":1}`, []string{"a.blp"}},
		{"the files written twice are the last ones",
			`{"version":1,"files":{"a.blp":"bad"},"files":{` + listing("b.blp") + `}}`, []string{"b.blp"}},
		{"no files", `{"version":1,"files":{}}`, nil},
		{"no files, with white space around", " \n{ \"version\" : 1 , \"files\" : { } }\n ", nil},
	}
}

// refusedDocuments is the states both trees refuse. The first seven are those of the other tree's test.
func refusedDocuments() []string {
	valid := `"` + strings.Repeat("0", 64) + `"`
	entry := func(path, hash string) string { return `{"version":1,"files":{"` + path + `":` + hash + `}}` }
	return []string{
		entry("war3map.lua", valid),
		`not json`,
		`{"version":2,"files":{}}`,
		`null`,
		`{"version":1,"files":[]}`,
		entry("a.blp", `"abc"`),
		`{"version":1,"files":{"a.blp":` + valid + `,"A.BLP":` + valid + `}}`,
		// What is no JSON.
		``, ` `, `{`, `{"version":1,"files":{}`, `{"version":1,"files":{}} {}`, `{"version":1,"files":{},}`,
		`{"version":1 "files":{}}`, `{version:1,files:{}}`, `{'version':1,'files':{}}`, `{"version":01,"files":{}}`,
		"\xEF\xBB\xBF" + `{"version":1,"files":{}}`, `{"version":1,"files":{"a.blp":"\x"}}`, `{"version":1,"files":{}}//`,
		// What has no version 1.
		`[]`, `[{"version":1,"files":{}}]`, `1`, `"version"`, `true`, `{}`, `{"files":{}}`, `{"version":"1","files":{}}`,
		`{"version":true,"files":{}}`, `{"version":null,"files":{}}`, `{"version":[1],"files":{}}`, `{"version":0,"files":{}}`,
		`{"version":-1,"files":{}}`, `{"version":1.5,"files":{}}`, `{"version":1e999,"files":{}}`, `{"Version":1,"files":{}}`,
		`{"version":1,"files":{},"version":2}`,
		// What has no files.
		`{"version":1}`, `{"version":1,"files":null}`, `{"version":1,"files":"a.blp"}`, `{"version":1,"files":7}`,
		`{"version":1,"Files":{}}`, `{"version":1,"files":{},"files":[]}`,
		// A hash that is none.
		entry("a.blp", `""`), entry("a.blp", `"`+strings.Repeat("0", 63)+`"`), entry("a.blp", `"`+strings.Repeat("0", 65)+`"`),
		entry("a.blp", `"`+strings.Repeat("A", 64)+`"`), entry("a.blp", `"`+strings.Repeat("g", 64)+`"`),
		entry("a.blp", `"`+strings.Repeat("0", 64)+`\n"`), entry("a.blp", `7`), entry("a.blp", `1e999`), entry("a.blp", `null`),
		entry("a.blp", `true`), entry("a.blp", `[`+valid+`]`), entry("a.blp", `{"hash":`+valid+`}`),
		// A path no asset may have.
		entry("", valid), entry("../a.blp", valid), entry("/a.blp", valid), entry("a//b.blp", valid), entry(`C:\\a.blp`, valid),
		entry("con.blp", valid), entry("scripts/war3map.j", valid), entry("war3mapMap.blp", valid), entry("(listfile)", valid),
		entry("WAR3MAP.W3I", valid), entry(`a\tb.blp`, valid),
		// A path listed twice.
		`{"version":1,"files":{"t/a.blp":` + valid + `,"t\\a.blp":` + valid + `}}`,
		`{"version":1,"files":{"\u00c9.blp":` + valid + `,"\u00e9.blp":` + valid + `}}`,
		// Which of two problems is told: an entry's path, then its hash, then whether it is listed twice; and the
		// entries in order.
		entry("war3map.lua", `"abc"`),
		`{"version":1,"files":{"a.blp":` + valid + `,"A.blp":"abc"}}`,
		`{"version":1,"files":{"a.blp":"abc","war3map.lua":` + valid + `}}`,
		`{"version":1,"files":{"war3map.lua":` + valid + `,"a.blp":"abc"}}`,
		`{"version":2,"files":[]}`,
	}
}

// removedBy is the files the other tree's plan removes, in their order: each by its path in the map folder and
// the hash of what it held.
func removedBy(t *testing.T, plan *oldassets.Plan, mapDir string) []Owned {
	t.Helper()
	var removed []Owned
	for _, change := range plan.Changes {
		if !change.Remove {
			continue
		}
		name, err := filepath.Rel(mapDir, change.File)
		if err != nil {
			t.Fatal(err)
		}
		removed = append(removed, Owned{filepath.ToSlash(name), fsx.SHA256Hex(change.Before)})
	}
	return removed
}

// compareStates gives a project whose only content is a state file, and a map folder with the files it owns,
// to both trees, and reports whether both refused the state.
func compareStates(t *testing.T, c stateDocument) (refused bool) {
	t.Helper()
	p := project{name: "the state " + c.name, folders: []string{"maps/" + mapFolder},
		files: map[string]string{".asset-state/" + mapFolder + ".json": c.document}}
	for _, owned := range c.owned {
		p.files["maps/"+mapFolder+"/"+owned] = bytesOf(owned)
	}
	root := p.onDisk(t)
	mapDir, stateFile, err := oldassets.Locations(root, mapFolder)
	if err != nil {
		t.Fatal(err)
	}
	plan, wantErr := oldassets.PlanAssets(context.Background(), root, mapDir, stateFile, oldassets.Config{}, nil)
	got, gotErr := ReadState(stateFile)
	if oracle.Refusals(t, p.name, wantErr, gotErr) {
		return true
	}
	if wantErr == nil && gotErr == nil {
		oracle.Values(t, p.name, removedBy(t, plan, mapDir), got.Files)
	}
	return false
}

func TestOracleOnReadingTheStateFile(t *testing.T) {
	read, files, refused := 0, 0, 0
	for _, c := range readDocuments() {
		if compareStates(t, c) {
			t.Errorf("the state %s: both trees refuse it", c.name)
			continue
		}
		read++
		files += len(c.owned)
	}
	for _, document := range refusedDocuments() {
		if !compareStates(t, stateDocument{name: document, document: document}) {
			t.Errorf("the state %s: both trees must refuse it", document)
			continue
		}
		refused++
	}
	// A project without a state file owns nothing in both trees.
	p := project{name: "no state file", folders: []string{"maps/" + mapFolder}}
	root := p.onDisk(t)
	mapDir, stateFile, _ := oldassets.Locations(root, mapFolder)
	plan, wantErr := oldassets.PlanAssets(context.Background(), root, mapDir, stateFile, oldassets.Config{}, nil)
	got, gotErr := ReadState(stateFile)
	if oracle.Refusals(t, p.name, wantErr, gotErr) || plan == nil || len(plan.Changes) != 0 || len(got.Files) != 0 {
		t.Errorf("%s: the other tree plans %+v (%v), this tree reads %+v (%v)", p.name, plan, wantErr, got, gotErr)
	}
	if read != 11 || files != 13 || refused != 73 {
		t.Errorf("%d states read with %d files, and %d refused; want 11, 13 and 73", read, files, refused)
	}
}

// writtenByOld is the state file the other tree writes for the files, each a path and its hash.
func writtenByOld(t *testing.T, files []Owned) (file string, data []byte) {
	t.Helper()
	file = filepath.Join(t.TempDir(), ".asset-state", mapFolder+".json")
	plan := &oldassets.Plan{State: oldassets.State{Version: 1}}
	for _, owned := range files {
		plan.State.Files.Set(owned.Path, owned.Hash)
	}
	if err := oldassets.ApplyPlan(context.Background(), plan, file); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	return file, data
}

func TestOracleOnTheBytesOfAState(t *testing.T) {
	zeros, sevens := strings.Repeat("0", 64), strings.Repeat("7", 64)
	tests := []struct {
		name  string
		files []Owned
		read  bool // whether the state is one ReadState takes
	}{
		{"one file", []Owned{{"a.blp", zeros}}, true},
		{"files in an order that is not sorted", []Owned{{"b.blp", zeros}, {"Textures/A.blp", sevens}, {"a.blp", zeros}}, true},
		{"paths outside ASCII, with a space, and that only start like a number",
			[]Owned{{"caf\xc3\xa9/\xc3\x89cole.blp", sevens}, {"My Icons/a b.blp", zeros}, {"7.blp", sevens}, {"07", zeros}}, true},
		{"a path with a backslash", []Owned{{`Textures\a.blp`, zeros}}, true},
		// No state that is read holds these: the bytes are still the other tree's.
		{"what JSON escapes, and what only some encoders escape",
			[]Owned{{"a\"b\\c\n\r\t\b\f\x00\x01\x1f\x7f<&>\xe2\x80\xa8\xe2\x80\xa9.blp", "not a \"hash\""}}, false},
		{"bytes that are not UTF-8", []Owned{{"a\xff\xc3.blp", zeros}}, false},
		{"an empty path and an empty hash", []Owned{{"", ""}}, false},
	}
	compared, readBack := 0, 0
	for _, tt := range tests {
		file, want := writtenByOld(t, tt.files)
		oracle.Bytes(t, "the state of "+tt.name, want, State{Files: tt.files}.Bytes())
		compared++
		if !tt.read {
			continue
		}
		read, err := ReadState(file)
		if err != nil || !slices.Equal(read.Files, tt.files) {
			t.Errorf("the state of %s, read back: %+v, %v", tt.name, read.Files, err)
		}
		readBack++
	}
	if compared != 7 || readBack != 4 {
		t.Errorf("%d states compared and %d read back, want 7 and 4", compared, readBack)
	}
}
