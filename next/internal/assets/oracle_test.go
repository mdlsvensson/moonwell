package assets

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	moonwell "github.com/mdlsvensson/moonwell"
	oldassets "github.com/mdlsvensson/moonwell/internal/assets"
	oldcli "github.com/mdlsvensson/moonwell/internal/cli"
	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/layout"
	"github.com/mdlsvensson/moonwell/internal/logging"
	oldmodels "github.com/mdlsvensson/moonwell/internal/models"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/pipeline"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/imp"
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
//   - PlanAssets and ApplyPlan against Collect, ReadState, Plan and Sync, each tree on a copy of its own of one
//     project, run after run (the stories): what is refused; where both planned, the assets, the changes by
//     name, bytes and removal, and the ownership; how often each tree asked its context; and then the two
//     project folders, file by file and folder by folder, the map and the state file among them. A build is
//     compared the same way: the other tree plans in a staged copy and applies there, this tree lays the changes
//     over the source map and stages. A build that is refused has no stage in this tree, and the other tree's,
//     which it copies before it plans, is taken away before the folders are compared. The stories are every
//     scenario of the other tree's tests of the plan, and seeded ones.
//   - The same for a context that is cancelled at every ask of a plan and of a sync, one after the other.
//   - GamePathKey against GamePathKey, on every line of the list of the game's paths that the program carries
//     and on seeded paths: stems with every ending the keys tell apart and more, each as written, in capitals
//     and with backslashes.
//   - ParseGamePaths against ParseGamePaths and LoadGamePaths against LoadGamePaths: the list the program
//     carries, and seeded lists with comments, blank lines, either line end and white space around the paths.
//   - The report of one model outside a project. The other tree's steps of the report are inside its command,
//     AssetsPaths, which outside a project needs only a folder and the model's file: its references with their
//     statuses against ReportModels, the lines it logs against RenderReports byte for byte, and what it refuses
//     a model with against the reason this tree reports. Inside a project the command loads the manifest and
//     syncs the libraries first, which is not this package's; the statuses of a project and their lines are held
//     by the tests of report.go, with the lines the other tree's tests expect.
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
// Of the refusals of a plan and a sync, those that are about one file are compared in part too, and counted by
// kind. Each tree is on its own copy of the project, so a path on disk is never the same in both.
//
//   - By name: the other tree names the file by its path on disk, as the file of the refusal or in its message,
//     or not at all; this tree names it by the folder's label, as mapdir does, and the state file by its path.
//     The message must be the other tree's with the one name for the other, the file this tree's name, and the
//     hint the other tree's, or one where it has none. The unit tests of plan.go and sync.go pin each file and
//     hint.
//   - Reworded: a folder that appears where a new file goes, between the plan and its write. The other tree
//     fails to read it and says the write failed; this tree's mapdir sees that the map changed and says so
//     (TestSyncRefusesAFileThatChangedAfterThePlanAndWritesOverNothing). Both undo.
//   - By system: war3map.imp as a folder, and a map file that cannot be read. The other tree passes the
//     system's error on, which a command shows as an internal error; this tree refuses with mapdir's failure
//     at the file (TestAFolderNamedAsTheIndexOfImportsIsRefusedBeforeAnythingIsPlanned,
//     TestAMapFileThePlanNeedsAndCannotReadIsRefusedByItsName).
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
//   - A map folder that is not there: the other tree's plan refuses it and names the manifest; in this tree
//     mapdir.Open fails before a plan, and the command that opens the folder words it.
//   - A plan the other tree's ApplyPlan takes and this tree cannot make: one that turns a file of the map into
//     a folder, which the other tree's test of an undo builds by hand. The stories reach an undo that cannot put
//     a file back through a context that changes the map between two writes.
//   - A path with a capital I with a dot, for the keys of the game's paths, alone and in a list: this tree folds
//     each letter to one letter, a plain i, and the other tree to an i and a combining dot
//     (TestGamePathKeyFoldsEachLetterToOneLetter). Counted, and each must differ.
//   - A list of the game's paths with a byte order mark or a next-line character (U+0085) at the edge of a line:
//     this tree takes Unicode's white space off a line, which the second is and the first is not, and to the
//     other tree the first is white space and the second is not
//     (TestParseGamePathsTakesTheWhiteSpaceOffEachLine). Counted, and each must differ.
//   - A model's path with a character beyond the basic plane, for the lines of the report: this tree pads a
//     column by characters and the other by UTF-16 units (TestTheColumnsOfAReportArePaddedByCharacters).

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

// ---- the plan and the sync ----

// sourceLabel is how this tree names the source map in errors: its path from the project folder.
const sourceLabel = "maps/" + mapFolder

// stageFolder is where a build of the stories stages the map, from the project folder.
const stageFolder = "dist/stage/" + mapFolder

// How the two refusals of a run are compared; the header of this file says what each is.
const (
	asWhole  = "whole"
	byName   = "by name"
	reworded = "reworded"
	bySystem = "by system"
)

// run is one run of assets:sync, or of a build, in a project as the runs before it left it.
type run struct {
	edit      func(t testing.TB, root string) // what changes in the project before the run
	block     string
	libraries []string
	build     bool                            // the changes go into a staged copy, and no state is written
	meddle    func(t testing.TB, root string) // what another program does between the plan and its writes
	// The context of the plan and of the writes, for a run that makes its own: else one that is never cancelled.
	planCtx, syncCtx func(t testing.TB, root string) *countdown
	refused          string // how the two refusals are compared; "" for a run both trees take
	about            string // the file a refusal names, from the project folder with "/"
}

// story is a project and the runs in it, one after the other.
type story struct {
	name    string
	project project
	runs    []run
}

// planned is a plan of either tree in one shape.
type planned struct {
	Assets  []Asset
	Changes []plannedChange
	Owned   []Owned
}

// plannedChange is one change of a plan, its file by the path from the folder the plan is for, with "/".
type plannedChange struct {
	Name   string
	Bytes  []byte
	Remove bool
}

// plannedByOtherTree is a plan of the other tree for the folder at dir.
func plannedByOtherTree(t testing.TB, plan *oldassets.Plan, dir string) *planned {
	t.Helper()
	p := &planned{Assets: converted(plan.Assets), Changes: []plannedChange{}, Owned: []Owned{}}
	for _, change := range plan.Changes {
		name, err := filepath.Rel(dir, change.File)
		if err != nil {
			t.Fatal(err)
		}
		p.Changes = append(p.Changes, plannedChange{filepath.ToSlash(name), change.After, change.Remove})
	}
	for path, hash := range plan.State.Files.All() {
		p.Owned = append(p.Owned, Owned{path, hash})
	}
	return p
}

// plannedByThisTree is a plan of this tree.
func plannedByThisTree(result *Result) *planned {
	p := &planned{Assets: result.Assets, Changes: []plannedChange{}, Owned: append([]Owned{}, result.State.Files...)}
	for _, change := range result.Changes {
		p.Changes = append(p.Changes, plannedChange{change.Name, change.Bytes, change.Remove})
	}
	return p
}

// ran is what a run did in one tree.
type ran struct {
	plan *planned // nil for a plan that was refused
	err  error
	asks [2]int // how often the plan and the writes asked their contexts
}

// contextOf is the context of one step of a run in the project at root.
func contextOf(t testing.TB, root string, own func(testing.TB, string) *countdown) *countdown {
	if own != nil {
		return own(t, root)
	}
	return &countdown{Context: context.Background(), limit: never}
}

// inOtherTree is the run in the other tree, on its copy of the project.
func (r run) inOtherTree(t *testing.T, root string) (did ran) {
	t.Helper()
	mapDir, stateFile, err := oldassets.Locations(root, mapFolder)
	if err != nil {
		t.Fatal(err)
	}
	dir, written := mapDir, stateFile
	if r.build {
		dir, written = filepath.Join(root, filepath.FromSlash(stageFolder)), ""
		if err := fsx.ReplaceDir(mapDir, dir); err != nil {
			t.Fatal(err)
		}
		defer func() {
			if did.err != nil {
				os.RemoveAll(filepath.Join(root, "dist"))
			}
		}()
	}
	block, _ := project{block: r.block}.blocks(t)
	planCtx, syncCtx := contextOf(t, root, r.planCtx), contextOf(t, root, r.syncCtx)
	plan, err := oldassets.PlanAssets(planCtx, root, dir, stateFile, block, r.libraries)
	did.asks[0] = planCtx.asks
	if err != nil {
		did.err = err
		return did
	}
	did.plan = plannedByOtherTree(t, plan, dir)
	if r.meddle != nil {
		r.meddle(t, root)
	}
	did.err = oldassets.ApplyPlan(syncCtx, plan, written)
	did.asks[1] = syncCtx.asks
	return did
}

// inThisTree is the run in this tree, on its copy of the project: as a command does it.
func (r run) inThisTree(t *testing.T, root string) (did ran) {
	t.Helper()
	p := project{block: r.block, libraries: r.libraries}
	_, block := p.blocks(t)
	stateFile, err := StateFile(root, mapFolder)
	if err != nil {
		t.Fatal(err)
	}
	assets, _, err := Collect(root, block, oldManifest, p.shippedIn(root))
	if err != nil {
		return ran{err: err}
	}
	owned, err := ReadState(stateFile)
	if err != nil {
		return ran{err: err}
	}
	folder, err := mapdir.Open(filepath.Join(root, filepath.FromSlash(sourceLabel)), sourceLabel)
	if err != nil {
		return ran{err: err}
	}
	planCtx, syncCtx := contextOf(t, root, r.planCtx), contextOf(t, root, r.syncCtx)
	result, err := Plan(planCtx, folder, assets, owned)
	did.asks[0] = planCtx.asks
	if err != nil {
		did.err = err
		return did
	}
	did.plan = plannedByThisTree(result)
	if r.meddle != nil {
		r.meddle(t, root)
	}
	if r.build {
		did.err = folder.With(result.Changes).StageTo(filepath.Join(root, filepath.FromSlash(stageFolder)))
		return did
	}
	did.err = Sync(syncCtx, folder, result, stateFile)
	did.asks[1] = syncCtx.asks
	return did
}

// names is how each tree names the file a refusal of the run is about: the other tree by its path on disk, in
// the staged copy for a build; this tree a file of the map by the folder's label, and any other by its path.
func (r run) names(oldRoot, newRoot string) (inOld, inNew string) {
	onDisk := func(root, name string) string { return filepath.Join(root, filepath.FromSlash(name)) }
	inMap, ofMap := strings.CutPrefix(r.about, sourceLabel+"/")
	switch {
	case !ofMap:
		return onDisk(oldRoot, r.about), onDisk(newRoot, r.about)
	case r.build:
		return onDisk(oldRoot, stageFolder+"/"+inMap), r.about
	}
	return onDisk(oldRoot, r.about), r.about
}

// sameButForTheName reports whether two refusals say the same about one file, which the other tree names inOld,
// in its message or as its file or not at all, and this tree inNew.
func sameButForTheName(want *olddiag.Error, got *diag.Error, inOld, inNew string) bool {
	msg := want.Msg
	if at := strings.Index(msg, inOld); at >= 0 {
		msg = msg[:at] + inNew + msg[at+len(inOld):]
	}
	// A refusal that lists files in its message has no file of its own.
	names := got.File == inNew || got.File == "" && strings.Contains(got.Msg, inNew)
	return got.Msg == msg && (want.File == "" || want.File == inOld) && names &&
		(got.Hint == want.Hint || want.Hint == "" && got.Hint != "")
}

// refusals compares what the two trees refused the run with, in the way the run says, and returns that way: ""
// for a run neither tree refuses.
func (r run) refusals(t *testing.T, what string, wantErr, gotErr error, oldRoot, newRoot string) string {
	t.Helper()
	if r.refused == "" || r.refused == asWhole || wantErr == nil || gotErr == nil {
		refused := oracle.Refusals(t, what, wantErr, gotErr)
		if refused != (r.refused == asWhole) {
			t.Errorf("%s: both trees refuse it: %v (%v, %v), want it compared %q", what, refused, wantErr, gotErr, r.refused)
		}
		return map[bool]string{true: asWhole}[refused]
	}
	inOld, inNew := r.names(oldRoot, newRoot)
	if r.refused == bySystem {
		got, expected := diag.First(gotErr)
		if _, old := olddiag.First(wantErr); old || !expected || got.File != inNew || !strings.Contains(wantErr.Error(), inOld) ||
			!oracle.Errors(t, what, wantErr, gotErr) {
			t.Errorf("%s: want the system's error about %s and a failure at %s:\nwant: %v\ngot:  %+v", what, inOld, inNew, wantErr, got)
		}
		return bySystem
	}
	want, got, ok := bothDiag(t, what, wantErr, gotErr)
	switch {
	case !ok:
	case r.refused == byName:
		whole := want.Msg == got.Msg && want.File == got.File && want.Hint == got.Hint
		if whole || !sameButForTheName(want, got, inOld, inNew) {
			t.Errorf("%s: the refusals must differ in how they name %s and in nothing else:\nwant: %+v\ngot:  %+v", what, r.about, want, got)
		}
	case r.refused == reworded:
		if !strings.HasPrefix(want.Msg, "Writing assets failed: ") || got.Msg != inNew+" changed after the assets were checked." ||
			got.File != inNew {
			t.Errorf("%s: want a failed write and a map that changed at %s:\nwant: %+v\ngot:  %+v", what, inNew, want, got)
		}
	}
	return r.refused
}

// sameProjects compares the two project folders, every file and every folder below them, and returns how many
// there are.
func sameProjects(t *testing.T, what, oldRoot, newRoot string) int {
	t.Helper()
	want, got := testkit.Snapshot(t, oldRoot), testkit.Snapshot(t, newRoot)
	for name, held := range want {
		data, has := got[name]
		switch {
		case !has:
			t.Errorf("%s: this tree's project has no %s", what, name)
		case (held == nil) != (data == nil):
			t.Errorf("%s: %s is a folder in one project and a file in the other", what, name)
		default:
			oracle.Bytes(t, what+": "+name, held, data)
		}
	}
	for name := range got {
		if _, has := want[name]; !has {
			t.Errorf("%s: this tree's project has %s, which the other tree's has not", what, name)
		}
	}
	return len(want)
}

// compare makes the run in both trees and compares all it did. It returns how the run was refused, "" for a run
// both trees take, and how many files and folders of the two projects it compared.
func (r run) compare(t *testing.T, what, oldRoot, newRoot string) (refused string, entries int) {
	t.Helper()
	if r.edit != nil {
		r.edit(t, oldRoot)
		r.edit(t, newRoot)
	}
	want, got := r.inOtherTree(t, oldRoot), r.inThisTree(t, newRoot)
	refused = r.refusals(t, what, want.err, got.err, oldRoot, newRoot)
	switch {
	case want.plan != nil && got.plan != nil:
		oracle.Values(t, what+": the plan", want.plan, got.plan)
	case want.plan != nil || got.plan != nil:
		t.Errorf("%s: one tree planned and the other refused the plan: %v, %v", what, want.err, got.err)
	}
	if want.asks[0] != got.asks[0] || !r.build && want.asks[1] != got.asks[1] {
		t.Errorf("%s: the plan and the writes asked their contexts %v times, want %v", what, got.asks, want.asks)
	}
	return refused, sameProjects(t, what, oldRoot, newRoot)
}

// ---- the stories ----

// with adds files that hold a text of their own to files: a name, then the text, and so on.
func with(files map[string]string, pairs ...string) map[string]string {
	for i := 0; i < len(pairs); i += 2 {
		files[pairs[i]] = pairs[i+1]
	}
	return files
}

// putting is an edit that writes files of the project: a name from the project folder with "/", then the text
// the file holds, and so on.
func putting(pairs ...string) func(testing.TB, string) {
	return func(t testing.TB, root string) {
		t.Helper()
		for i := 0; i < len(pairs); i += 2 {
			testkit.WriteFile(t, root, pairs[i], []byte(pairs[i+1]))
		}
	}
}

// removing is an edit that removes files of the project, and folders with all that is in them.
func removing(names ...string) func(testing.TB, string) {
	return func(t testing.TB, root string) {
		t.Helper()
		for _, name := range names {
			if err := os.RemoveAll(filepath.Join(root, filepath.FromSlash(name))); err != nil {
				t.Fatal(err)
			}
		}
	}
}

// renaming is an edit that renames a file or a folder of the project, by way of a third name: a system that
// ignores letter case takes two spellings of a name for one.
func renaming(from, to string) func(testing.TB, string) {
	return func(t testing.TB, root string) {
		t.Helper()
		at := func(name string) string { return filepath.Join(root, filepath.FromSlash(name)) }
		if err := errors.Join(os.Rename(at(from), at(from+".aside")), os.Rename(at(from+".aside"), at(to))); err != nil {
			t.Fatal(err)
		}
	}
}

// several is edits made one after the other.
func several(edits ...func(testing.TB, string)) func(testing.TB, string) {
	return func(t testing.TB, root string) {
		t.Helper()
		for _, edit := range edits {
			edit(t, root)
		}
	}
}

// indexOf is what a war3map.imp with the entries holds.
func indexOf(entries ...imp.Entry) string { return string(imp.Write(entries)) }

// stopped is a context that is cancelled from the ask after limit on, and before that ask has another program
// do something in the project.
func stopped(limit int, meddle func(testing.TB, string)) func(testing.TB, string) *countdown {
	return func(t testing.TB, root string) *countdown {
		return &countdown{Context: context.Background(), limit: limit, before: map[int]func(){limit + 1: func() { meddle(t, root) }}}
	}
}

// scenarioStories is the scenarios of the other tree's tests of the plan.
func scenarioStories() []story {
	const m, s = sourceLabel + "/", shippedUnder
	const sword = "ReplaceableTextures/CommandButtons/BTNSword.blp"
	const mapped = `{"paths":{"icons/disabled.blp":"ReplaceableTextures/CommandButtonsDisabled/DISBTNSword.blp"}}`
	const spellings = `{"paths":{"a.blp":"Sound/Music/a.blp","b.blp":"sound/music/b.blp","c.blp":"SOUND/Effects/c.blp"}}`
	return []story{
		{"mapped assets beside the editor's own import, and nothing to do the second time",
			project{files: with(holding("assets/"+sword, "assets/icons/disabled.blp", m+"war3mapImported/existing.wav"),
				m+"war3map.imp", indexOf(imp.Entry{Flag: 5, Path: "existing.wav"}))},
			[]run{{block: mapped}, {block: mapped}}},
		{"an asset is changed, staged by a build, synced, renamed by a mapping and removed",
			project{files: holding("assets/Models/unit.mdx", m+"unmanaged.txt")},
			[]run{{}, {edit: putting("assets/Models/unit.mdx", "second"), build: true}, {},
				{block: `{"paths":{"Models/unit.mdx":"Models/renamed.mdx"}}`}, {edit: removing("assets/Models/unit.mdx")}}},
		{"World Editor saves the owned imports with its own flag, and a third asset is added",
			project{files: holding("assets/Textures/a.blp", "assets/Textures/b.blp")},
			[]run{{}, {edit: putting(m+"war3map.imp",
				indexOf(imp.Entry{Flag: 29, Path: `Textures\a.blp`}, imp.Entry{Flag: 29, Path: `Textures\b.blp`}))},
				{edit: putting("assets/Textures/c.blp", "a third")}}},
		{"a file of the map where an asset goes, then an owned file edited by hand, with and without its asset",
			project{files: with(holding("assets/a.blp"), m+"a.blp", "editor owned")},
			[]run{{refused: byName, about: m + "a.blp"}, {edit: removing(m + "a.blp")},
				{edit: putting(m+"a.blp", "manual edit"), refused: byName, about: m + "a.blp"},
				{edit: removing("assets/a.blp"), refused: byName, about: m + "a.blp"},
				{build: true, refused: byName, about: m + "a.blp"}}},
		{"an asset below a file of the map",
			project{files: holding("assets/a.blp", m+"Textures")},
			[]run{{block: `{"paths":{"a.blp":"textures/a.blp"}}`, refused: byName, about: m + "Textures"}}},
		{"a folder the map spells in its own way, and new folders in three spellings",
			project{files: holding(m+"Textures/existing.blp", "assets/textures/new.blp", "assets/a.blp", "assets/b.blp", "assets/c.blp")},
			[]run{{block: spellings}, {block: spellings}}},
		{"nothing to own, then one file, then nothing again",
			project{}, []run{{}, {edit: putting("assets/a.blp", "one")}, {edit: removing("assets/a.blp")}}},
		{"a folder appears where a new file goes, after the plan",
			project{files: holding("assets/a.blp", "assets/b.blp")},
			[]run{{meddle: putting(m+"b.blp/inner.txt", "another program's"), refused: reworded, about: m + "b.blp"}}},
		{"a write fails after a file was replaced: a file is where the folder of a new file goes",
			project{files: with(holding("assets/a.blp"), m+"war3mapImported/existing.wav", "editor",
				m+"war3map.imp", indexOf(imp.Entry{Flag: 5, Path: "existing.wav"}))},
			[]run{{}, {edit: putting("assets/a.blp", "second", "assets/Sound/b.blp", "new"),
				meddle: putting(m+"Sound", "another program's"), refused: byName, about: m + "Sound/b.blp"}}},
		{"a file that cannot be taken out again when the sync is interrupted",
			project{files: holding("assets/a.blp", "assets/b.blp")},
			[]run{{syncCtx: stopped(1, several(removing(m+"a.blp"), putting(m+"a.blp/inner.txt", "another program's"))),
				refused: byName, about: m + "a.blp"}}},
		{"nothing to import and nothing owned, in a map whose index does not read",
			project{files: map[string]string{m + "war3map.imp": "\x09\x09"}}, []run{{}, {build: true}}},
		{"a library's files beside the map's own, and the library dropped",
			project{files: holding("assets/Models/Own.mdx", "assets/icons/shared.blp", s+"ui/war3mapImported/ui/frames.toc",
				s+"ui/icons/shared.blp", s+"unlisted/never.txt")},
			[]run{{libraries: []string{"ui"}}, {libraries: []string{"ui"}}, {}}},
		{"a library's file where the map has a file of its own",
			project{files: holding(s+"ui/Models/Golem.mdx", m+"Models/Golem.mdx")},
			[]run{{libraries: []string{"ui"}, refused: byName, about: m + "Models/Golem.mdx"}}},
	}
}

// seededStories is stories for what the scenarios do not reach.
func seededStories(t testing.TB) []story {
	const m, s = sourceLabel + "/", shippedUnder
	const state = ".asset-state/" + mapFolder + ".json"
	e := func(flag uint8, path string) imp.Entry { return imp.Entry{Flag: flag, Path: path} }
	one := func(name string, files map[string]string, r run) story {
		return story{name, project{files: files}, []run{r}}
	}
	return []story{
		one("an import World Editor made where an asset goes, without a file",
			with(holding("assets/Textures/a.blp"), m+"war3map.imp", indexOf(e(29, `textures\A.blp`))),
			run{refused: byName, about: m + "Textures/a.blp"}),
		one("an import in the folder World Editor imports into, where an asset goes",
			with(holding("assets/war3mapImported/a.wav"), m+"war3map.imp", indexOf(e(8, "A.wav"))),
			run{refused: byName, about: m + "war3mapImported/a.wav"}),
		one("an index that lists a path twice",
			with(holding("assets/a.blp"), m+"war3map.imp", indexOf(e(13, `Textures\x.blp`), e(5, "y.wav"), e(29, "textures/X.BLP"))),
			run{refused: byName, about: m + "war3map.imp"}),
		one("an index that is cut short", with(holding("assets/a.blp"), m+"war3map.imp", indexOf(e(13, "x.blp"))[:10]),
			run{refused: byName, about: m + "war3map.imp"}),
		one("an index with a flag World Editor does not write", with(holding("assets/a.blp"), m+"war3map.imp", indexOf(e(7, "x.blp"))),
			run{refused: byName, about: m + "war3map.imp"}),
		one("an index with a path that leaves the map", with(holding("assets/a.blp"), m+"War3Map.imp", indexOf(e(13, `..\x.blp`))),
			run{refused: byName, about: m + "War3Map.imp"}),
		one("an index named in capitals, with the editor's own import",
			with(holding("assets/a.blp", m+"war3mapImported/own.wav"), m+"WAR3MAP.IMP", indexOf(e(8, "own.wav"))), run{}),
		one("the index World Editor 3.00 saved",
			with(holding("assets/a.blp", m+"wa3mapPreview.tga"), m+"war3map.imp", string(testkit.Fixture(t, "imports-we3/war3map-flag29.imp"))),
			run{}),
		{"an owned file gone from the map, an asset changed, one added and one left out",
			project{files: holding("assets/a.blp", "assets/b.blp", "assets/c.blp", "assets/d.blp")},
			[]run{{}, {edit: several(removing(m+"a.blp"), putting("assets/b.blp", "second", "assets/Sound/e.blp", "new")),
				block: `{"exclude":["c.blp"]}`}}},
		one("an asset named as a folder of the map", holding("assets/textures", m+"Textures/x.blp"),
			run{refused: byName, about: m + "Textures"}),
		{"an asset named as an empty folder of the map",
			project{files: holding("assets/empty"), folders: []string{m + "Empty"}}, []run{{refused: byName, about: m + "Empty"}}},
		{"an asset below an owned file that no asset wants",
			project{files: holding("assets/data")},
			[]run{{}, {edit: several(removing("assets/data"), putting("assets/data/inner.txt", "inner")), refused: byName, about: m + "data"}}},
		one("two assets without room, the first below a file", holding("assets/a/inner.blp", "assets/b.blp", m+"a", m+"b.blp"),
			run{refused: byName, about: m + "a"}),
		one("two assets without room, the first at a file", holding("assets/a/inner.blp", "assets/b.blp", m+"a", m+"0.blp"),
			run{block: `{"paths":{"b.blp":"0.blp"}}`, refused: byName, about: m + "0.blp"}),
		{"an owned import that World Editor saved without a custom path",
			project{files: holding("assets/war3mapImported/a.wav")},
			[]run{{}, {edit: putting(m+"war3map.imp", indexOf(e(5, "a.wav")))}, {}, {}}},
		{"the last owned file is removed from a map without an index",
			project{files: holding("assets/a.blp")}, []run{{}, {edit: removing("assets/a.blp", m+"war3map.imp")}}},
		{"an owned file whose import is gone from the index",
			project{files: holding("assets/a.blp")}, []run{{}, {edit: putting(m+"war3map.imp", indexOf())}}},
		{"an owned file under another spelling in the map",
			project{files: holding("assets/Models/Unit.mdx")},
			[]run{{}, {edit: several(renaming(m+"Models/Unit.mdx", m+"Models/UNIT.mdx"), renaming(m+"Models", m+"models"),
				putting("assets/Models/Unit.mdx", "second"))}}},
		one("names outside ASCII and with a space, and a file without bytes",
			with(holding("assets/caf\xc3\xa9/\xc3\x89cole.blp", "assets/My Icons/a b.blp"), "assets/empty.blp", ""), run{}),
		one("a path that looks like a number, alone", holding("assets/7"), run{}),
		{"a build with an asset changed, one removed and one in a new folder",
			project{files: holding("assets/Models/unit.mdx", "assets/old.blp", m+"war3map.w3i")},
			[]run{{}, {edit: several(putting("assets/Models/unit.mdx", "second", "assets/sound/theme.mp3", "theme"), removing("assets/old.blp")),
				build: true}}},
		one("a library's file below a file of the map", holding(s+"ui/ui/frame.fdf", m+"UI"),
			run{libraries: []string{"ui"}, refused: byName, about: m + "UI"}),
		one("a folder named as the index", holding("assets/a.blp", m+"war3map.imp/stray.txt"),
			run{refused: bySystem, about: m + "war3map.imp"}),
		{"an owned file is edited by hand after the plan",
			project{files: holding("assets/0.blp", "assets/a.blp")},
			[]run{{}, {edit: putting("assets/0.blp", "second", "assets/a.blp", "second"),
				meddle: putting(m+"a.blp", "edited by hand"), refused: byName, about: m + "a.blp"}}},
		one("a file appears where a new asset goes, after the plan", holding("assets/0.blp", "assets/a.blp"),
			run{meddle: putting(m+"a.blp", "the editor's"), refused: byName, about: m + "a.blp"}),
		one("the state file cannot be written", holding("assets/a.blp"),
			run{syncCtx: beforeAsk(3, putting(state+"/in the way.txt", "another program's")), refused: byName, about: state}),
		// Another program gets at the state file after the sync began. The ask before the state file is the last:
		// the second where a.blp alone is written, the third where the index is written too.
		{"the state file is changed before it is written",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: putting("assets/a.blp", "second"), syncCtx: beforeAsk(2, putting(state, "another program's")),
				refused: byName, about: state}}},
		{"the state file is removed before it is written",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: putting("assets/a.blp", "second"), syncCtx: beforeAsk(2, removing(state)), refused: byName, about: state}}},
		one("a state file is made before the first is written", holding("assets/a.blp"),
			run{syncCtx: beforeAsk(3, putting(state, "another program's")), refused: byName, about: state}),
		{"the state file is changed before it is removed",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: removing("assets/a.blp"), syncCtx: beforeAsk(3, putting(state, "another program's")),
				refused: byName, about: state}}},
		// Neither tree looks again at a state file that needs no write.
		{"the state file is changed while a sync that does not write it writes the index",
			project{files: holding("assets/a.blp")},
			[]run{{}, {edit: putting(m+"war3map.imp", indexOf()), syncCtx: beforeAsk(1, putting(state, "another program's"))}}},
	}
}

// beforeAsk is a context that is never cancelled, and before an ask, counted from 1, has another program do
// something in the project.
func beforeAsk(ask int, meddle func(testing.TB, string)) func(testing.TB, string) *countdown {
	return func(t testing.TB, root string) *countdown {
		return &countdown{Context: context.Background(), limit: never, before: map[int]func(){ask: func() { meddle(t, root) }}}
	}
}

func TestOracleOnThePlanAndTheSync(t *testing.T) {
	compared := map[string]int{}
	runs, entries := 0, 0
	for _, s := range slices.Concat(scenarioStories(), seededStories(t)) {
		s.project.folders = append(s.project.folders, sourceLabel)
		oldRoot, newRoot := s.project.onDisk(t), s.project.onDisk(t)
		for i, r := range s.runs {
			refused, files := r.compare(t, fmt.Sprintf("%s (run %d)", s.name, i+1), oldRoot, newRoot)
			compared[refused]++
			runs, entries = runs+1, entries+files
		}
	}
	// Thirty-one runs of the scenarios and forty-five seeded ones.
	want := map[string]int{"": 47, byName: 27, reworded: 1, bySystem: 1}
	if !maps.Equal(compared, want) || runs != 76 || entries != 775 {
		t.Errorf("%d runs compared, by how they were refused: %v, with %d files and folders; want 76 runs, %v and 775",
			runs, compared, entries, want)
	}
}

// interruptedProject is a project after a sync, with an asset changed, one added and one removed since: its plan
// asks its context before anything, before each of the two owned files and before each of the two assets, and its
// sync before each of four changes and before the state file.
func interruptedProject() project {
	const m = sourceLabel + "/"
	owned := State{Files: []Owned{{"a.blp", fsx.SHA256Hex([]byte("first"))}, {"dropped.blp", fsx.SHA256Hex([]byte("dropped"))}}}
	index := indexOf(imp.Entry{Flag: 29, Path: "a.blp"}, imp.Entry{Flag: 29, Path: "dropped.blp"})
	return project{files: map[string]string{
		"assets/a.blp": "second", "assets/b.blp": "new", m + "a.blp": "first", m + "dropped.blp": "dropped",
		m + "war3map.imp": index, ".asset-state/" + mapFolder + ".json": string(owned.Bytes()),
	}}
}

func TestOracleOnAnInterruptedPlanAndAnInterruptedSync(t *testing.T) {
	const asks = 5 // of the plan, and of the sync
	limited := func(limit int) func(testing.TB, string) *countdown {
		return func(testing.TB, string) *countdown { return &countdown{Context: context.Background(), limit: limit} }
	}
	interrupted, finished := 0, 0
	for _, step := range []string{"the plan", "the sync"} {
		for limit := range asks + 1 {
			r := run{planCtx: limited(limit), refused: asWhole}
			if step == "the sync" {
				r = run{syncCtx: limited(limit), refused: asWhole}
			}
			if limit == asks {
				r.refused = ""
			}
			p := interruptedProject()
			refused, _ := r.compare(t, fmt.Sprintf("%s, cancelled at ask %d", step, limit+1), p.onDisk(t), p.onDisk(t))
			if refused == asWhole {
				interrupted++
			} else {
				finished++
			}
		}
	}
	if interrupted != 2*asks || finished != 2 {
		t.Errorf("%d runs interrupted and %d finished, want %d and 2", interrupted, finished, 2*asks)
	}
}

func TestOracleOnAMapFileThatCannotBeRead(t *testing.T) {
	const m = sourceLabel + "/"
	compared := 0
	for _, held := range []string{"Textures/owned.blp", "war3map.imp"} {
		owned := State{Files: []Owned{{"Textures/owned.blp", fsx.SHA256Hex([]byte("owned"))}}}
		p := project{files: map[string]string{
			m + "Textures/owned.blp": "owned", m + "war3map.imp": indexOf(imp.Entry{Flag: 13, Path: `Textures\owned.blp`}),
			".asset-state/" + mapFolder + ".json": string(owned.Bytes()),
		}}
		oldRoot, newRoot := p.onDisk(t), p.onDisk(t)
		r := run{refused: bySystem, about: m + held}
		for _, root := range []string{oldRoot, newRoot} {
			makeUnreadable(t, filepath.Join(root, filepath.FromSlash(r.about)))
		}
		want, got := r.inOtherTree(t, oldRoot), r.inThisTree(t, newRoot)
		if r.refusals(t, "a plan that needs "+held, want.err, got.err, oldRoot, newRoot) == bySystem && want.plan == nil && got.plan == nil {
			compared++
		}
	}
	if compared != 2 {
		t.Errorf("%d refusals compared, want 2", compared)
	}
}

// ---- the game's paths ----

// The characters the two trees take differently. Each is a class that is left out, counted, and seen to differ.
const (
	dottedI       = "\xc4\xb0"     // U+0130, a capital I with a dot
	byteOrderMark = "\xef\xbb\xbf" // U+FEFF
	nextLine      = "\xc2\x85"     // U+0085
)

// carriedLines is every line of the list of the game's paths that the program carries, its header among them.
func carriedLines() []string {
	var lines []string
	for line := range strings.Lines(moonwell.GamePaths) {
		lines = append(lines, strings.TrimRight(line, "\r\n"))
	}
	return lines
}

// seededGamePaths is paths as a model may name them: each stem with each ending, as written, in capitals and
// with backslashes. The endings are every one a key tells apart, and ones it must not.
func seededGamePaths() []string {
	stems := []string{
		"textures/black32", "Units/Human/Footman/Footman", "a", "", "folder.blp/name", "folder.mdl/name", "two..dots",
		"caf\xc3\xa9/\xc3\x89cole", "stra\xc3\x9fe/gro\xc3\x9f", "\xce\x9f\xce\x94\xce\x9f\xce\xa3/\xce\xa3", "\xe2\x84\xaaelvin/\xc5\xbf",
		dottedI + "con/a", "icon/" + dottedI,
	}
	endings := []string{
		"", ".", ".blp", ".dds", ".tga", ".tif", ".tiff", ".png", ".jpg", ".jpeg", ".bmp", ".mdl", ".mdx", ".pkb",
		".pkfx", ".wav", ".blp.mdl", ".mdl.blp", ".mdl.mdl", ".tif.tiff", ".blp ", ".b lp", ".blp/", ".mdl/", "blp", "mdl",
	}
	var paths []string
	for _, stem := range stems {
		for _, ending := range endings {
			path := stem + ending
			paths = append(paths, path, strings.ToUpper(path), strings.ReplaceAll(path, "/", `\`))
		}
	}
	slices.Sort(paths)
	return slices.Compact(paths)
}

func TestOracleOnTheKeyOfAGamePath(t *testing.T) {
	compared, leftOut := 0, 0
	for _, path := range slices.Concat(carriedLines(), seededGamePaths()) {
		want, got := oldmodels.GamePathKey(path), GamePathKey(path)
		if strings.Contains(path, dottedI) {
			leftOut++
			if want == got {
				t.Errorf("the path %q is left out, and both trees give it the key %q", path, got)
			}
			continue
		}
		oracle.Values(t, "the key of the path "+path, want, got)
		compared++
	}
	// The list of Warcraft III 3.0.0.24268 has 42128 lines; 782 of the seeded paths are without the letter.
	if compared != 42128+782 || leftOut != 156 {
		t.Errorf("%d keys compared and %d left out, want 42910 and 156", compared, leftOut)
	}
}

// around is a short list with white inside and around each of its lines: a path, a comment, a line of white
// alone, and a path with white in it.
func around(white string) string {
	return white + "textures/a.blp" + white + "\n" + white + "# a comment" + white + "\n" + white + "\n" +
		"units/b" + white + "c.mdl" + white
}

// seededLists is lists of the game's paths as a file may hold them, without the characters the two trees take
// differently: the seeded paths under either line end and between comments and blank lines, and a short list
// with each kind of white space around its lines, and with characters that are none.
func seededLists() []string {
	paths := slices.DeleteFunc(seededGamePaths(), func(path string) bool { return strings.Contains(path, dottedI) })
	lists := []string{
		"", "\n", "\r\n", "# only a comment", "# only a comment\n", "#\n#\n", "textures/a.blp", "textures/a.blp\n\n\n",
		strings.Join(paths, "\n"), strings.Join(paths, "\n") + "\n", strings.Join(paths, "\r\n") + "\r\n",
		strings.Join(paths, "\r"), "# header\n\n" + strings.Join(paths, "\n\n# between\n"),
		" \t" + strings.Join(paths, " \t\n \t"), "#" + strings.Join(paths, "\n#"), " #" + strings.Join(paths, "\n\t#"),
	}
	whites := []string{
		" ", "\t", "\v", "\f", "\r", "\xc2\xa0", "\xe1\x9a\x80", "\xe2\x80\x83", "\xe2\x80\xa8", "\xe2\x80\xa9", "\xe2\x80\xaf",
		"\xe2\x81\x9f", "\xe3\x80\x80",
		// None of these is white space.
		"\x00", "\x1f", "\xe2\x80\x8b", "\xe1\xa0\x8e", "\xff", "_",
	}
	for _, white := range whites {
		lists = append(lists, around(white))
	}
	return lists
}

// differingLists is lists with a character the two trees take differently, each of which they must read
// differently.
func differingLists() []string {
	return []string{
		"textures/" + dottedI + ".blp\n", "units/a.mdx\n" + dottedI + "\n", "# " + dottedI + "\nI" + dottedI + "i.mdl",
		around(byteOrderMark), byteOrderMark + "# Warcraft III 3.0\ntextures/a.blp\n", "textures/a.blp" + byteOrderMark,
		around(nextLine), nextLine + "textures/a.blp\n", "textures/a.blp\n" + nextLine + "\n",
	}
}

// takenDifferently reports whether a list of the game's paths holds a character the two trees take differently.
func takenDifferently(list string) bool {
	return strings.Contains(list, dottedI) || strings.Contains(list, byteOrderMark) || strings.Contains(list, nextLine)
}

func TestOracleOnAListOfGamePaths(t *testing.T) {
	compared, leftOut := 0, 0
	for i, list := range slices.Concat([]string{moonwell.GamePaths}, seededLists(), differingLists()) {
		want, got := oldmodels.ParseGamePaths(list), ParseGamePaths(list)
		if takenDifferently(list) {
			leftOut++
			if maps.Equal(want, got) {
				t.Errorf("the list %q is left out, and both trees read it as %q", list, slices.Sorted(maps.Keys(got)))
			}
			continue
		}
		oracle.Values(t, fmt.Sprintf("the paths of list %d", i), want, got)
		compared++
	}
	if compared != 36 || leftOut != 9 {
		t.Errorf("%d lists compared and %d left out, want 36 and 9", compared, leftOut)
	}
	carried := LoadGamePaths()
	oracle.Values(t, "the list the program carries", oldmodels.LoadGamePaths(), carried)
	if len(carried) != 42114 {
		t.Errorf("the list the program carries has %d keys, want the 42114 of Warcraft III 3.0.0.24268", len(carried))
	}
}

// ---- the report of a model ----

// reportedModel is a model for both trees to report, by the name of its file.
type reportedModel struct {
	file       string
	data       []byte
	unreadable bool // whether both trees fail to read it
}

// reportedGamePaths is the list of the game's paths the reported models are classified by.
const reportedGamePaths = "# test\ntextures/knight.dds\nabilities/spells/human/heal.mdx\ndoodads/corn/plant1_normal.dds\n" +
	"textures/spark.blp\nunits/human/knight/knight_portrait.mdx\n"

// reportedText is a text model with a texture of each kind and two emitters.
const reportedText = `Version {
	FormatVersion 800,
}
Textures 4 {
	Bitmap {
		Image "Textures/Knight.blp",
	}
	Bitmap {
		Image "",
		ReplaceableId 2,
	}
	Bitmap {
		Image "",
		ReplaceableId 31,
	}
	Bitmap {
		Image "war3mapImported\Cape.tga",
	}
}
ParticleEmitter "Heal" {
	EmitterUsesMDL,
	Particle {
		Path "Abilities\Spells\Human\Heal.mdl",
	}
}
ParticleEmitter "Spark" {
	EmitterUsesTGA,
	Path "Textures\Spark.blp",
}
`

// reportedModels is models of both formats with references of every kind and status a report outside a project
// has, in columns of several widths, and models that cannot be read. No path has a character beyond the basic
// plane.
func reportedModels() []reportedModel {
	everyKind := testkit.MDX(
		testkit.Chunk("TEXS", testkit.Concat(testkit.Texture("Textures/Knight.blp", 0), testkit.Texture("", 1),
			testkit.Texture("", 2), testkit.Texture("", 7), testkit.Texture(`Textures\Mine.blp`, 0))),
		testkit.Chunk("PREM", testkit.Concat(testkit.Emitter(`Abilities\Spells\Human\Heal.mdl`, testkit.EmitterUsesMDL),
			testkit.Emitter(`Textures\Spark.tga`, testkit.EmitterUsesTGA), testkit.Emitter("Models/Both.mdx", testkit.EmitterUsesMDL|testkit.EmitterUsesTGA),
			testkit.Emitter(`Models\Neither.mdx`, 0))),
		testkit.Chunk("ATCH", testkit.Attachment(`Units\Human\Knight\Knight_Portrait.mdl`)),
		testkit.Chunk("CORN", testkit.Popcorn(`Particles\Fire.pkb`)),
		testkit.Chunk("FAFX", testkit.FaceEffect("Node", `Units\Human\Knight\Knight.facefx`)),
	)
	return []reportedModel{
		{file: "knight.mdx", data: everyKind},
		{file: "Models/Deep/Knight.mdx", data: everyKind},
		{file: "one.mdx", data: textured("Doodads/Corn/plant1_Normal.tif")},
		{file: "custom.mdx", data: textured("a.blp", `a\very\long\path\to\a\texture\that\is\wider\than\any\status.blp`)},
		{file: "slots.mdx", data: testkit.MDX(testkit.Chunk("TEXS", testkit.Concat(testkit.Texture("", 1), testkit.Texture("", 12))))},
		{file: "accents.mdx", data: textured("Textures\\caf\xc3\xa9.blp", "Textures/\xc3\x89cole du Nord.blp", "Textures\\tab\there.blp", "x ")},
		{file: "empty.mdx", data: testkit.MDX()},
		{file: "text.mdl", data: []byte(reportedText)},
		{file: "version.mdl", data: []byte("Version {\n\tFormatVersion 800,\n}\n")},
		{file: "broken.mdl", data: []byte("Model {\n}\nBroken {\n"), unreadable: true},
		{file: "cut.mdx", data: []byte("MDLXTEX"), unreadable: true},
		{file: "short.mdx", data: testkit.MDX(testkit.Chunk("TEXS", make([]byte, 100))), unreadable: true},
		{file: "picture.blp", data: []byte("BLP1\x00\x00\x00\x00"), unreadable: true},
		{file: "nothing.mdl", data: nil, unreadable: true},
	}
}

// byOtherTree is the other tree's report of a model outside a project: the references with their statuses and
// the lines it logs, or what it refuses the model with. Its command is given a folder that holds only the model.
func (m reportedModel) byOtherTree(t testing.TB) (refs []oldcli.ModelRef, lines []string, err error) {
	t.Helper()
	root := t.TempDir()
	testkit.WriteFile(t, root, m.file, m.data)
	env := &pipeline.Env{Root: root, Log: logging.New(func(line string) { lines = append(lines, line) }, "")}
	reports, err := oldcli.AssetsPaths(background, env, m.file, oldmodels.ParseGamePaths(reportedGamePaths))
	if err != nil {
		return nil, lines, err
	}
	if len(reports) != 1 || reports[0].Heading != m.file {
		t.Fatalf("the other tree reports %s as %+v", m.file, reports)
	}
	return reports[0].Refs, lines, nil
}

func TestOracleOnTheReportOfAModelOutsideAProject(t *testing.T) {
	read, unreadable := 0, 0
	for _, m := range reportedModels() {
		refs, lines, err := m.byOtherTree(t)
		reports := ReportModels([]Model{{m.file, m.data}}, ParseGamePaths(reportedGamePaths), nil)
		if m.unreadable {
			failure := &olddiag.Error{}
			if !errors.As(err, &failure) || len(lines) != 0 {
				t.Errorf("%s: the other tree gives %v after the lines %q, want a refusal and no line", m.file, err, lines)
				continue
			}
			oracle.Values(t, m.file+": why it is unreadable", failure.Msg, reports[0].Unreadable)
			unreadable++
			continue
		}
		if err != nil {
			t.Errorf("%s: the other tree refuses it: %v", m.file, err)
			continue
		}
		oracle.Values(t, m.file+": the references", refs, reports[0].Refs)
		oracle.Values(t, m.file+": the lines", lines, RenderReports(reports, false))
		read++
	}
	if read != 9 || unreadable != 5 {
		t.Errorf("%d models read and %d unreadable in both trees, want 9 and 5", read, unreadable)
	}
}
