package settings

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/luasrc"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	oldsettings "github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/mapdir"
	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
	"github.com/mdlsvensson/moonwell/next/internal/war3/txt"
	"github.com/mdlsvensson/moonwell/next/internal/war3/w3i"
)

// What this file compares, and what it leaves out. Both trees are given one settings document: the other tree
// reads it with its Validate, this tree with manifest.Decode. Then, with the same map info, the same project
// folder and the same names for errors:
//
//   - PatchMapInfo against patchInfo, for every document and every map info: what is refused (kind, message,
//     file and hint), else the patched bytes.
//   - GameplaySections, and the game interface sections the other tree's Validate returns, against
//     textSections: what is refused, else the sections with their keys and values in order.
//   - What the other tree's Validate refuses for names that differ only in letter case, against the refusal of
//     textSections: this tree makes that check where the sections are used. Among the documents are one with
//     two spellings in the interface beside a typed constant that disagrees with a raw one, and one with two
//     spellings in both blocks: both trees must tell the same refusal of the two.
//   - LoadPreview against loadPreview, on a project folder with a picture of each kind: what is refused, else
//     the extension and the bytes of the picture.
//   - PatchLua against patchLua, for every document and every script: what is refused, else the text as bytes.
//     The map info given to both is what patchInfo returns for the fixture's; the other tree takes it as bytes,
//     this tree as w3i.Read reads those bytes (afterInfo, the way a plan does it). The same on the fixture's
//     script for every map info of mapInfos with the settings in it, and for every one as it is, those that do
//     not read among them.
//   - PatchMinimapLua against patchMinimap, for every script and for scripts with settings in them.
//   - Plan against Plan, on map folders written to disk: what is refused, else the changes in their order, each
//     by its name, whether it removes its file, and its bytes. The other tree is given the folder's path, the
//     label errors name it by, the manifest's name and the project folder; this tree is given the folder as
//     mapdir opens it under that label, and the project. A document that the other tree's Validate refuses is
//     refused there, before its Plan: that refusal is compared with what this tree's Plan refuses.
//
// The documents are every one that the other tree's tests give its Validate and see accepted, one that sets
// every setting, and for the constants every pairing of a typed constant with a raw one. The script is given
// more: numbers that are a zero below 0, and for each block with a counterpart in the script a document for
// every subset of its settings. The scripts are the fixture's, every one the other tree's tests of the Lua give
// it, and the fixture's in other layouts, each with and without a call that a setting adds. Every script is
// paired with every document but those of the subsets, which are paired with the fixture's script and with one
// layout of it: reading a script is what takes the time here, and the settings of the subsets are set, together,
// by documents that every script is paired with.
//
// Left out, for the one difference in what is written into the script that is meant: a script into which a
// number goes that is not 0 and is below 0.000001 in size, or 1e21 or larger. The other tree writes such a number
// with an exponent, this tree in plain decimal
// (TestANumberIsWrittenInPlainDecimalHoweverSmallOrLargeAndAZeroAsZero). The numbers a script takes are the map
// info's, which holds each as a float32: a setting of 0.000001 is stored as a number below it and is in the
// class, and a setting of 1e-46 is stored as 0 and is not. So the class is decided on the map info given to both
// trees, for the values the settings make the script take, and not on the document. A script is left out only
// when the other tree did not refuse it; it is counted, this tree must not refuse it either, and the two texts
// must differ, or the class is wider than the difference.
//
// Left out, for the one difference in the minimap call that is meant: a script whose main() ends in a return
// that gives a value. The other tree writes the call after it and returns a script that Lua cannot load; this
// tree reads its result once more and refuses it
// (TestTheMinimapCallIsRefusedWhereItWouldStandAfterAReturnedValue). It is decided on the other tree's result,
// which its own reader of Lua does not read, and counted: one script.
//
// Compared in part, and counted: a script that does not read. This tree says where by line and column and the
// other tree by a count of characters at the end of its message, a difference that the oracle of war3/lua holds,
// place by place. Here the two refusals must have the same file and hint, and the same message before the place
// (TestAScriptThatDoesNotReadIsRefusedWithItsLineAndColumn). It is decided on the other tree's refusal.
//
// Left out, for the one difference that is meant: a preview path that runs through a file
// ("preview.tga/inner.tga"), on a system that calls that "not a directory". There the other tree passes the
// system's error on as an error that is not a diag error, and this tree says on every system that the picture
// does not exist (TestAPreviewPathThroughAFileNamesAFileThatDoesNotExist). The path is among the inputs. It is
// left out when the other tree's result is such an error, which is counted: once on those systems, and never on
// Windows, where both trees say the same and are compared. That this tree refuses it too is still looked at.
//
// The plans are not every document for every map folder. What a document does to a map info, a script, a text
// file or a picture is compared above, for every document; a plan adds which files it reads, what it refuses
// first, what it leaves out as unchanged, and the names and the order of its changes, and those depend on the
// kind of the document and on the files of the folder. So every document (those of the map info, of the
// constants, and of the names that differ only in letter case) is planned for two folders: the fixture as it is,
// which has neither optional text file and no minimap, and the fixture with every file a plan reads, each text
// file behind a byte order mark, where the previews of each kind go in. And one document for each way through a
// plan (routeDocuments) is planned for sixteen more folders (sourceMaps): the fixture with each optional file
// alone, with all of them, with text files that hold nothing and under other spellings, and a folder for each
// thing a plan refuses a map for. The plans run on several goroutines, as the scripts do.
//
// Left out of the plans, for the two differences in the script named above. A plan that puts a number into the
// script that the trees write apart is decided as it is for the script, on the map info as the other tree's plan
// leaves it; its other changes are compared, and the bytes of its script must differ. A plan with a preview for
// a script whose main() ends in a return that gives a value is decided on the script the other tree's plan
// writes; this tree must refuse it for the script. Both are counted for each folder.
//
// Compared in part: a map file that cannot be read. This tree reads through mapdir, which says "Reading a map
// file failed", and the other tree says "Reading a map file for map settings failed"
// (TestAMapFileThatCannotBeReadIsRefusedByItsNameAndNotTakenForAnEmptyFile). The two refusals must have the same
// file, hint and reason after those words (TestOracleOnAMapFileThatCannotBeRead), for each of the four files a
// plan reads.
//
// Compared in part: a map with a folder under a name a preview adds. The other tree takes the folder for a file
// and says "The map already has ..., a name the preview picture needs"; this tree asks mapdir for the file's
// place, which says "... would replace a folder in the map"
// (TestAFolderUnderANameThePreviewAddsIsRefusedBeforeAMapFileIsRead). Both must refuse before the map info is
// read, for a preview of each kind, and name the same file (TestOracleOnAFolderUnderANameAPreviewAdds): twelve
// refusals.
//
// Compared in part: a map with a folder under the name of a file the settings need (war3map.w3i, war3map.lua, and
// war3mapMap.blp for a preview). The other tree fails to read the folder as a file and says "Reading a map file
// for map settings failed", with the system's reason; this tree says that the name is a folder, not a file
// (TestAFolderWhereAFileTheSettingsNeedBelongsIsToldAsAFolder). Both must refuse and name the same file, which
// is the folder as the map spells it (TestOracleOnAFolderUnderAFileTheSettingsNeed): six refusals, one of them
// for a folder in another letter case. The other tree meets the minimap's folder last, when it reads the minimap
// to keep it, and this tree first, before it reads a map file.
//
// TestOracleOnAPreviewThatCannotBeRead and TestOracleOnAMapFileThatCannotBeRead compare nothing, and are
// skipped, where the test cannot make a file that is there and cannot be read: as root on a system other than
// Windows, since root reads a file without permissions, and on a Windows that lets the file be read although the
// test holds it with no sharing and an exclusive lock.
//
// Not among the inputs:
//
//   - A map folder with a folder under the name of one of the two text files. The other tree fails to read it
//     as a file. In this tree a folder is not a file: the map has no such text file, and mapdir refuses to write
//     one there (TestAFolderWhereATextFileGoesIsRefusedAndNotTakenForAMapWithoutTheFile).
//   - A map folder that is not there, and one with two paths that differ only in letter case: mapdir.Open
//     refuses both before there is a folder to plan for.
//   - A plan without the manifest's name, for which the other tree says moonwell.pkl: a project knows its
//     manifest.
//   - A map folder whose script does not read: the refusal is compared in part above, for the script.
//   - A document whose shape the other tree refuses (a wrong name, range or type): that is Pkl's to refuse.
//   - A section or a key whose name looks like a number. Neither tree is given one: the schema and the other
//     tree's Validate both take only names that start with a letter or an underscore.
//   - A script whose SetPlayerTeam names a team below 0.000001 in size or from 1e21, where a refusal shows the
//     team: this tree shows it in plain decimal (TestATeamInARefusalIsWrittenInPlainDecimal).

// bothTrees is one settings document as each tree reads it.
type bothTrees struct {
	old     *oldsettings.Settings // nil when the other tree refuses the document
	refusal error                 // why it does
	project *manifest.Project
}

// inBothTrees reads a settings document in both trees, for a project in the folder root. This tree must take
// the document; whether the other tree does is the caller's to look at.
func inBothTrees(t testing.TB, root, document string) bothTrees {
	t.Helper()
	tree, err := ordered.Decode([]byte(document))
	if err != nil {
		t.Fatalf("settings %s: %v", document, err)
	}
	old, refusal := oldsettings.Validate(tree, manifestName)
	return bothTrees{old, refusal, projectOf(t, root, document)}
}

// accepted is a settings document as each tree reads it, which both must take.
func accepted(t testing.TB, root, document string) (*oldsettings.Settings, *manifest.Project) {
	t.Helper()
	read := inBothTrees(t, root, document)
	if read.refusal != nil {
		t.Fatalf("settings %s: the other tree refuses the document: %v", document, read.refusal)
	}
	return read.old, read.project
}

// The documents are in documents_test.go: those of the other tree's tests of the map info, of the two text
// files, of the plan, of the Lua and of its Validate, and those made for this comparison.

// ---- the map info ----

type mapInfo struct {
	name string
	data []byte
}

// mapInfos is a map info of every version, the two World Editor saved, those the other tree's tests change by
// hand, and three that are no map info.
func mapInfos(t *testing.T) []mapInfo {
	t.Helper()
	var infos []mapInfo
	for _, version := range everyVersion {
		infos = append(infos, mapInfo{fmt.Sprintf("version %d", version), testkit.SyntheticMapInfo(version)})
	}
	endless := testkit.SyntheticMapInfo(39)
	details := readInfo(t, endless, w3i.Extended).Details
	binary.LittleEndian.PutUint32(endless[details.Fog.End.Start:], math.Float32bits(float32(math.Inf(1))))
	unknown := testkit.SyntheticMapInfo(39)
	binary.LittleEndian.PutUint32(unknown[details.Fog.Density.Start:], math.Float32bits(float32(math.NaN())))
	return append(infos,
		mapInfo{"the fixture", testkit.Fixture(t, "map-settings-v39/war3map.w3i")},
		mapInfo{"the fixture with colours", testkit.Fixture(t, "map-settings-v39/war3map-colors.w3i")},
		mapInfo{"version 39 without custom forces", withoutFlags(t)},
		mapInfo{"version 39 with a fog without an end", endless},
		mapInfo{"version 39 with a fog whose density is no number", unknown},
		mapInfo{"version 39 cut short in its player", testkit.SyntheticMapInfo(39)[:details.Players[0].Name.Start+3]},
		mapInfo{"bytes that are no map info", []byte("not a map info")},
		mapInfo{"an empty file", nil},
	)
}

// patches counts what the documents came to for one map info.
type patches struct{ refused, changed, unchanged int }

// The patches of each map info of mapInfos, in its order. A document is refused by a map info that does not
// read or that lacks what the document sets; what is left changes the file or sets nothing stored in it.
var wantPatches = []patches{
	{77, 22, 30}, // version 18: no players, forces or environment, and no loading-screen model
	{74, 24, 31}, // version 25
	{21, 68, 40}, // version 28
	{21, 68, 40}, // version 31
	{21, 68, 40}, // version 32
	{21, 68, 40}, // version 33
	{21, 68, 40}, // version 39
	{10, 74, 45}, // the fixture, which has more players and forces
	{10, 74, 45}, // the fixture with colours
	{36, 57, 36}, // without custom forces
	{32, 59, 38}, // a fog without an end
	{31, 60, 38}, // a fog whose density is no number
	{74, 24, 31}, // cut short in its player: what is before the player reads
	{100, 0, 29}, // no map info: only a document that sets nothing stored in it is taken
	{100, 0, 29}, // an empty file
}

func TestOracleOnTheMapInfo(t *testing.T) {
	infos, all := mapInfos(t), documents()
	counted := make([]patches, len(infos))
	changesTheFixture := map[string]bool{}
	for _, document := range all {
		old, project := accepted(t, "", document)
		for i, info := range infos {
			what := info.name + ", settings " + document
			source := slices.Clone(info.data)
			want, wantErr := oldsettings.PatchMapInfo(source, old, infoFile)
			got, gotErr := patchInfo(source, project.Settings, infoFile)
			if !bytes.Equal(source, info.data) {
				t.Fatalf("%s: a patch changed the bytes it was given", what)
			}
			switch {
			case oracle.Refusals(t, what, wantErr, gotErr):
				counted[i].refused++
			case bytes.Equal(want, source):
				oracle.Bytes(t, what, want, got)
				counted[i].unchanged++
			default:
				oracle.Bytes(t, what, want, got)
				counted[i].changed++
				changesTheFixture[document] = changesTheFixture[document] || info.name == "the fixture"
			}
		}
	}
	if len(all) != 129 {
		t.Errorf("%d documents, want 129", len(all))
	}
	// The documents that set every setting must go in somewhere, or their settings are compared nowhere.
	for _, document := range everyDocuments[:5] {
		if !changesTheFixture[document] {
			t.Errorf("the fixture does not take %s", document)
		}
	}
	if !slices.Equal(counted, wantPatches) {
		for i, info := range infos {
			t.Logf("%s: %+v", info.name, counted[i])
		}
		t.Errorf("the patches of the map infos are not the ones pinned: %+v", wantPatches)
	}
}

// ---- the constants ----

// sectionsOfOld is sections of the other tree in the shape of this tree's.
func sectionsOfOld(old *oldsettings.Sections) []txt.Section {
	var result []txt.Section
	for name, entries := range old.All() {
		section := txt.Section{Name: name}
		for key, value := range entries.All() {
			section.Fields = append(section.Fields, txt.Field{Key: key, Value: value})
		}
		result = append(result, section)
	}
	return result
}

func TestOracleOnTheGameplayConstantsAndTheInterfaceSections(t *testing.T) {
	all := slices.Concat(documents(), constantDocuments())
	merged, conflicts, typed, interfaces := 0, 0, 0, 0
	for _, document := range all {
		old, project := accepted(t, "", document)
		want, wantErr := oldsettings.GameplaySections(old, manifestName)
		got, skin, gotErr := textSections(project.Settings, manifestName)
		if oracle.Refusals(t, "the constants of "+document, wantErr, gotErr) {
			conflicts++
			// The interface of a document whose constants are refused is compared all the same.
			var err error
			if skin, err = sections(project.Settings.GameInterface, "settings.gameInterface", manifestName); err != nil {
				t.Errorf("the interface of %s: %v", document, err)
			}
		} else {
			oracle.Values(t, "the constants of "+document, sectionsOfOld(&want), got)
			merged++
			if project.Settings.Gameplay != (manifest.Gameplay{}) {
				typed++
			}
		}
		oracle.Values(t, "the interface of "+document, sectionsOfOld(&old.GameInterface), skin)
		if len(skin) > 0 {
			interfaces++
		}
	}
	// Three documents of the other tree's tests conflict, and twelve pairings: a raw value that differs, in each
	// of the three spellings of Misc, for each typed constant alone and for both.
	if merged != 202 || conflicts != 15 || typed != 70 || interfaces != 18 || merged+conflicts != len(all) {
		t.Errorf("%d merges, %d of them of a typed constant, %d conflicts and %d documents with interface sections, "+
			"of %d documents; want 202, 70, 15 and 18", merged, typed, conflicts, interfaces, len(all))
	}
}

func TestOracleOnNamesThatDifferOnlyInLetterCase(t *testing.T) {
	refused := 0
	for _, document := range duplicateDocuments {
		read := inBothTrees(t, "", document)
		_, _, err := textSections(read.project.Settings, manifestName)
		if oracle.Refusals(t, document, read.refusal, err) {
			refused++
		}
	}
	if refused != len(duplicateDocuments) {
		t.Errorf("both trees refused %d of %d documents", refused, len(duplicateDocuments))
	}
}

// ---- the preview ----

// previewProject is a project folder with a picture of each kind, and files that are no picture the game shows.
func previewProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	picture := testkit.NewPixels(256)
	for name, data := range map[string][]byte{
		"preview.blp":         testkit.BLP(256, 1),
		"jpeg.blp":            testkit.BLP(512, 0),
		"preview.tga":         plainTGA(),
		"art/Preview.TGA":     testkit.TGA(picture, testkit.TGAOptions{RLE: true, Depth: 24, FromTop: true}),
		"art/Preview.PNG":     testkit.PNG(picture, "rgba"),
		"art/deep/grey.png":   testkit.PNG(testkit.GreyPixels(256), "grey"),
		"big.png":             testkit.PNG(testkit.NewPixels(512), "interlaced"),
		"assets2/preview.blp": testkit.BLP(256, 1),
		"assets/preview.tga":  plainTGA(),
		"preview.jpg":         plainTGA(),
		"preview.png":         plainTGA(),
		"preview":             plainTGA(),
		".tga":                plainTGA(),
		"small.tga":           plainTGA()[:100],
		"tiny.tga":            testkit.TGA(testkit.NewPixels(128), testkit.TGAOptions{}),
		"world.blp":           append([]byte("BLP2"), make([]byte, 200)...),
		"empty.png":           {},
	} {
		testkit.WriteFile(t, root, name, data)
	}
	if err := os.Mkdir(filepath.Join(root, "folder.tga"), 0o777); err != nil {
		t.Fatal(err)
	}
	return root
}

// fileOnTheWay is a path through a file of previewProject.
const fileOnTheWay = "preview.tga/inner.tga"

// passedOnAsItIs reports whether an error of the other tree is the operating system's word for a file where a
// path needs a folder, passed on as an error that is not a diag error.
func passedOnAsItIs(err error) bool {
	var expected *olddiag.Error
	return errors.Is(err, syscall.ENOTDIR) && !errors.As(err, &expected)
}

// previewPaths is the path of each file of previewProject, some in more than one spelling, and paths that name
// no file of it or no file a preview may be.
var previewPaths = []string{
	"preview.blp", "jpeg.blp", "preview.tga", "art/Preview.TGA", "art/Preview.PNG", "art/deep/grey.png", "big.png",
	"assets2/preview.blp", `art\Preview.TGA`, `art\deep/grey.png`,
	"assets/preview.tga", `Assets\preview.tga`, "ASSETS/missing.tga", "assets/", "assets",
	"preview.jpg", "preview.png", "preview", ".tga", "small.tga", "tiny.tga", "world.blp", "empty.png",
	"folder.tga", "art", "missing.tga", "art/missing.tga", "nowhere/preview.tga", fileOnTheWay,
	"../preview.tga", "/preview.tga", `C:\preview.tga`, "art//preview.tga", "", ".", "./preview.tga", "art/../preview.tga",
	"preview.tga ", "preview.tga.", "con.tga", "art/nul", "pre\tview.tga", "what?.tga",
}

// comparePreviews gives one preview setting to both trees and reports whether both refused it.
func comparePreviews(t *testing.T, root, preview string) (refused bool) {
	t.Helper()
	what := "the preview " + preview
	want, wantErr := oldsettings.LoadPreview(root, preview, manifestName)
	got, gotErr := loadPreview(root, preview, manifestName)
	if oracle.Refusals(t, what, wantErr, gotErr) {
		return true
	}
	if want == nil || got == nil {
		t.Fatalf("%s: a tree returned no picture and no error", what)
	}
	oracle.Values(t, what+": the extension", want.Extension, got.Extension)
	oracle.Bytes(t, what, want.Bytes, got.Bytes)
	return false
}

func TestOracleOnThePreview(t *testing.T) {
	root := previewProject(t)
	read, refused, leftOut := 0, 0, 0
	for _, preview := range previewPaths {
		_, wantErr := oldsettings.LoadPreview(root, preview, manifestName)
		switch {
		case passedOnAsItIs(wantErr):
			leftOut++
			_, gotErr := loadPreview(root, preview, manifestName)
			if !oracle.Errors(t, "the preview "+preview, wantErr, gotErr) {
				t.Errorf("the preview %s: this tree reads a picture through a file", preview)
			}
		case comparePreviews(t, root, preview):
			refused++
		default:
			read++
		}
	}
	// Only the path through a file is left out, and only where the system has a word of its own for it.
	wantLeftOut := 1
	if runtime.GOOS == "windows" {
		wantLeftOut = 0
	}
	// The first ten paths name a picture the game shows.
	if wantRefused := len(previewPaths) - 10 - wantLeftOut; read != 10 || refused != wantRefused || leftOut != wantLeftOut {
		t.Errorf("%d previews read, %d refused and %d left out, want 10, %d and %d", read, refused, leftOut, wantRefused, wantLeftOut)
	}
}

func TestOracleOnAPreviewThatCannotBeRead(t *testing.T) {
	root := t.TempDir()
	testkit.MakeUnreadable(t, testkit.WriteFile(t, root, "art/preview.tga", plainTGA()))
	if !comparePreviews(t, root, "art/preview.tga") {
		t.Error("both trees read a picture that cannot be read")
	}
}

func TestOracleOnAPreviewBehindALink(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	testkit.WriteFile(t, outside, "preview.tga", plainTGA())
	testkit.LinkDir(t, outside, filepath.Join(root, "art"))
	if !comparePreviews(t, root, "art/preview.tga") {
		t.Error("both trees read a picture behind a link")
	}
}

// ---- the script ----

// block is one block of the settings with a counterpart in the script: where its settings are written, and a
// value for each. The fog's settings are a block inside the environment's.
type block struct {
	open     string
	settings []string
	inner    string // what opens the block inside, or "" without one
	within   []string
	close    string
}

var blocks = []block{
	{open: `{"info":{`, close: `}}`, settings: []string{
		`"name":"Subset"`, `"author":"A"`, `"description":"One|nTwo"`, `"recommendedPlayers":"2"`, `"preview":"p.tga"`}},
	{open: `{"players":{"0":{`, close: `}}}`, settings: []string{
		`"name":"Hero"`, `"controller":"computer"`, `"race":"undead"`, `"fixedStart":false`, `"x":-512.25`, `"y":1024.5`}},
	// Player 11 is the map's fifth, so its start location is not its id.
	{open: `{"players":{"11":{`, close: `}}}`, settings: []string{
		`"name":""`, `"controller":"user"`, `"race":"selectable"`, `"fixedStart":false`, `"x":0.1`, `"y":-0.3`}},
	{open: `{"forces":{"0":{`, close: `}}}`, settings: []string{
		`"name":"Blue"`, `"allied":false`, `"alliedVictory":true`, `"sharedVision":false`, `"sharedControl":true`,
		`"sharedAdvancedControl":true`}},
	{open: `{"forces":{"1":{`, close: `}}}`, settings: []string{
		`"name":""`, `"allied":true`, `"alliedVictory":true`, `"sharedVision":true`, `"sharedControl":false`,
		`"sharedAdvancedControl":false`}},
	{open: `{"environment":{`, close: `}}`, settings: []string{`"soundEnvironment":"Mountains"`, `"waterColor":[10,20,30,40]`},
		inner: `"fog":{`, within: []string{
			`"enabled":true`, `"style":2`, `"start":500.5`, `"end":4000`, `"density":0.75`, `"color":[50,60,70,80]`}},
}

// document is the block with the settings of the subset: bit i of the subset is setting i, the settings of the
// block inside after the block's own.
func (b block) document(subset int) string {
	chosen := func(settings []string, first int) []string {
		var result []string
		for i, setting := range settings {
			if subset&(1<<(first+i)) != 0 {
				result = append(result, setting)
			}
		}
		return result
	}
	parts := chosen(b.settings, 0)
	if b.inner != "" {
		parts = append(parts, b.inner+strings.Join(chosen(b.within, len(b.settings)), ",")+"}")
	}
	return b.open + strings.Join(parts, ",") + b.close
}

// subsetDocuments is, for each block, a document for every subset of its settings.
func subsetDocuments() []string {
	var documents []string
	for _, b := range blocks {
		for subset := range 1 << (len(b.settings) + len(b.within)) {
			documents = append(documents, b.document(subset))
		}
	}
	return documents
}

// The documents with a zero below 0 and those with a number that the two trees write apart (zeroDocuments,
// apartDocuments), the scripts and their layouts, and the map folders of the plans are in documents_test.go and
// recorded_test.go.

// writtenApart reports whether the settings make a script take, from the map info given as bytes, a number that
// the two trees write apart: the other tree with an exponent, this tree in plain decimal.
func writtenApart(s manifest.Settings, patchedInfo []byte) bool {
	return inPlainDecimal(s, patchedInfo)
}

// unreadIn is the refusal of the other tree when it is that of its Lua reader for a script that does not read,
// and nil for any other error.
func unreadIn(err error) *olddiag.Error {
	if failure, ok := err.(*olddiag.Error); ok && strings.HasPrefix(failure.Msg, "Cannot safely read map Lua: ") {
		return failure
	}
	return nil
}

// compareUnread compares the refusals of a script that the other tree's Lua reader refuses, in everything but
// the place: the same file, the same hint, and the same message before the other tree's " at character N".
func compareUnread(t *testing.T, what string, want *olddiag.Error, got error) {
	t.Helper()
	failure, ok := got.(*diag.Error)
	switch {
	case !ok:
		t.Errorf("%s: got %v, want the refusal of a script that does not read: %v", what, got, want)
	case failure.File != want.File || failure.Hint != want.Hint || failure.Line == 0 || failure.Column == 0 ||
		want.Msg != failure.Msg && !strings.HasPrefix(want.Msg, failure.Msg+" at character "):
		t.Errorf("%s: the refusals of a script that does not read differ:\nwant: %+v\ngot:  %+v", what, want, failure)
	}
}

// scriptCounts is what came of the scripts both trees were given.
type scriptCounts struct{ refused, unread, changed, unchanged, leftOut int }

// compareScripts gives both trees one script, one document and one map info as bytes, and counts what came of it.
func (c *scriptCounts) compareScripts(t *testing.T, what, source string, old *oldsettings.Settings, s manifest.Settings, info []byte) {
	t.Helper()
	want, wantErr := oldsettings.PatchLua(source, old, info, luaFile, infoFile)
	got, gotErr := afterInfo(source, s, info)
	unread := unreadIn(wantErr)
	switch {
	case unread != nil:
		compareUnread(t, what, unread, gotErr)
		c.unread++
	case oracle.Refusals(t, what, wantErr, gotErr):
		c.refused++
	case wantErr == nil && writtenApart(s, info):
		c.leftOut++
		if gotErr == nil && got == want {
			t.Errorf("%s: left out, and both trees write the same", what)
		}
	case want == source:
		oracle.Bytes(t, what, []byte(want), []byte(got))
		c.unchanged++
	default:
		oracle.Bytes(t, what, []byte(want), []byte(got))
		c.changed++
	}
}

// add counts what another count has.
func (c *scriptCounts) add(other scriptCounts) {
	c.refused += other.refused
	c.unread += other.unread
	c.changed += other.changed
	c.unchanged += other.unchanged
	c.leftOut += other.leftOut
}

// together runs each piece of work on one of as many goroutines as the machine runs at once, and adds up what
// they counted. Each pair of a script and a document has both trees read the script twice, and there are
// thousands of pairs: one after the other they take several seconds.
func together(work []func(*scriptCounts)) scriptCounts {
	counts := make([]scriptCounts, len(work))
	onEveryCore(len(work), func(piece int) { work[piece](&counts[piece]) })
	var sum scriptCounts
	for _, count := range counts {
		sum.add(count)
	}
	return sum
}

// onTheFixturesInfo puts the settings of each document into the fixture's map info and gives both trees each
// script with the result. It returns what came of the scripts, and how many documents the map info refused.
func onTheFixturesInfo(t *testing.T, documents []string, sources []script) (counted scriptCounts, notInTheMap int) {
	t.Helper()
	fixture := fixtureInfo(t)
	var work []func(*scriptCounts)
	for _, document := range documents {
		old, project := accepted(t, "", document)
		info, err := patchInfo(fixture, project.Settings, infoFile)
		if err != nil {
			notInTheMap++
			continue
		}
		work = append(work, func(c *scriptCounts) {
			for _, source := range sources {
				c.compareScripts(t, source.name+", settings "+document, source.text, old, project.Settings, info)
			}
		})
	}
	return together(work), notInTheMap
}

func TestOracleOnTheScript(t *testing.T) {
	all, sources := scriptDocuments(), scripts(t)
	counted, notInTheMap := onTheFixturesInfo(t, all, sources)
	if len(all) != 137 || len(sources) != 53 || notInTheMap != 10 {
		t.Errorf("%d documents, %d of them not for the fixture's map info, and %d scripts; want 137, 10 and 53",
			len(all), notInTheMap, len(sources))
	}
	// 127 documents for 53 scripts. 39 of the documents set nothing the script has, and leave all 53 as they
	// are; each of the other 88 is refused by the two scripts that do not read. Four documents put a number
	// into the script that the trees write apart (the fourth of everyDocuments, and the first three of
	// apartDocuments): the pairs of them that the other tree does not refuse are left out, which are 18, 38, 36
	// and 45 of the 53 of each.
	want := scriptCounts{refused: 1177, unread: 2 * 88, changed: 2830, unchanged: 39*53 + 344, leftOut: 18 + 38 + 36 + 45}
	if counted != want {
		t.Errorf("the scripts came to %+v, want %+v", counted, want)
	}
}

func TestOracleOnTheScriptForEverySubsetOfABlock(t *testing.T) {
	all, sources := subsetDocuments(), scripts(t)
	// The script as World Editor writes it, and the layout least like it. The other scripts are compared for
	// the documents of TestOracleOnTheScript, which set every setting among them.
	layout := slices.IndexFunc(sources, func(source script) bool { return source.name == "on one line with semicolons" })
	counted, notInTheMap := onTheFixturesInfo(t, all, []script{sources[0], sources[layout]})
	// 32 subsets of the description, 64 of each of two players and two forces, 256 of the environment.
	if len(all) != 32+4*64+256 || notInTheMap != 0 {
		t.Errorf("%d documents, %d of them not for the fixture's map info; want 544 and 0", len(all), notInTheMap)
	}
	// What leaves a script as it is: the 8 subsets of the description without a name or a description, the
	// subset with nothing and the one with only a name of each player and each force, and the environment with
	// nothing. Every other subset changes both scripts, and none is refused.
	unchanged := 2 * (8 + 4*2 + 1)
	if want := (scriptCounts{changed: 2*len(all) - unchanged, unchanged: unchanged}); counted != want {
		t.Errorf("the scripts came to %+v, want %+v", counted, want)
	}
}

// unreadInfos are bytes that are no map info, beside those of mapInfos.
var unreadInfos = []mapInfo{{"two bytes", []byte{1, 2}}}

func TestOracleOnTheScriptOfEveryMapInfo(t *testing.T) {
	source, infos := fixtureLua(t), slices.Concat(mapInfos(t), unreadInfos)
	var withSettings, without []func(*scriptCounts)
	for _, document := range scriptDocuments() {
		old, project := accepted(t, "", document)
		for _, info := range infos {
			what := info.name + ", settings " + document
			// A map info as it is stands for one of another map: the settings may name what it lacks.
			without = append(without, func(c *scriptCounts) {
				c.compareScripts(t, what+", not in the map info", source, old, project.Settings, info.data)
			})
			if patched, err := patchInfo(info.data, project.Settings, infoFile); err == nil {
				withSettings = append(withSettings, func(c *scriptCounts) {
					c.compareScripts(t, what, source, old, project.Settings, patched)
				})
			}
		}
	}
	// The fixture's script is that of the fixture's map info: for another map info it is mostly refused, for a
	// player or a force that is not where the script has it.
	want := scriptCounts{refused: 79, changed: 646, unchanged: 635, leftOut: 24}
	if counted := together(withSettings); counted != want || len(withSettings) != 1384 {
		t.Errorf("%d scripts for a map info with the settings came to %+v, want 1384 and %+v", len(withSettings), counted, want)
	}
	want = scriptCounts{refused: 772, changed: 422, unchanged: 998}
	if counted := together(without); counted != want || len(without) != 137*16 {
		t.Errorf("%d scripts for a map info as it is came to %+v, want %d and %+v", len(without), counted, 137*16, want)
	}
}

// broken reports whether the other tree returned, without an error, a script that its own reader of Lua does
// not read.
func broken(result string, err error) bool {
	if err != nil {
		return false
	}
	_, unread := luasrc.Functions(result, luaFile)
	return unread != nil
}

func TestOracleOnTheMinimapCall(t *testing.T) {
	sources := append(scripts(t),
		script{"a main() that returns a value", mainReturnsValue}, script{"a main() that returns", mainReturns})
	for _, document := range luaDocuments {
		if result, err := withSettings(t, document, fixtureLua(t)); err == nil {
			sources = append(sources, script{"the fixture with settings " + document, result})
		}
	}
	added, refused, unread, leftOut := 0, 0, 0, 0
	for _, source := range sources {
		want, wantErr := oldsettings.PatchMinimapLua(source.text, luaFile)
		got, gotErr := patchMinimap(source.text, luaFile)
		switch failure := unreadIn(wantErr); {
		case failure != nil:
			compareUnread(t, source.name, failure, gotErr)
			unread++
		case broken(want, wantErr):
			// Left out, for the difference that is meant: the other tree writes the call after a returned
			// value and hands back a script the game cannot load. This tree reads its result once more and
			// refuses it (TestTheMinimapCallIsRefusedWhereItWouldStandAfterAReturnedValue).
			leftOut++
			if failure, ok := gotErr.(*diag.Error); !ok || !strings.Contains(failure.Msg, "could not be read back safely") {
				t.Errorf("%s: the other tree's script does not read, and this tree returns %q, %v", source.name, got, gotErr)
			}
		case oracle.Refusals(t, source.name, wantErr, gotErr):
			refused++
		default:
			oracle.Bytes(t, source.name, []byte(want), []byte(got))
			added++
		}
	}
	// Of the 53 scripts two have no main() or two, and two do not read; every document of luaDocuments goes
	// into the fixture's script. Of the two scripts whose main() returns, the one that returns nothing takes the
	// call, and the one that returns a value is the one left out.
	wantAdded := 49 + 1 + len(luaDocuments)
	if added != wantAdded || refused != 2 || unread != 2 || leftOut != 1 {
		t.Errorf("the call went into %d scripts, %d refused it, %d do not read and %d are left out; want %d, 2, 2 and 1",
			added, refused, unread, leftOut, wantAdded)
	}
}

// ---- the plan ----

// planCounts is what came of the plans for one map folder.
type planCounts struct{ refused, changed, unchanged, apart, broken int }

// wantPlans is what the plans for each map folder of sourceMaps (in recorded_test.go) must come to, by the
// folder's name. Of the 17 documents of routeDocuments, four are refused for every folder: the player the fixture
// lacks, the preview that is not there, the constant set twice, and the two spellings of a section. Two change
// nothing in any folder: the one that sets nothing, and the sections without keys.
var wantPlans = map[string]planCounts{
	// One document puts a number into the script that the trees write apart (the fourth of everyDocuments).
	// Without a minimap, the nine documents with a preview are refused; with every file, they are planned.
	"the fixture": {refused: 43, changed: 170, unchanged: 12, apart: 1},
	"every file, each text file behind a byte order mark": {refused: 34, changed: 178, unchanged: 13, apart: 1},
	// The constant the file holds already is no change.
	"every file": {refused: 4, changed: 10, unchanged: 3},
	// Without a minimap, the five documents with a preview are refused as well.
	"with war3mapMisc.txt alone":        {refused: 9, changed: 5, unchanged: 3},
	"with war3mapSkin.txt alone":        {refused: 9, changed: 6, unchanged: 2},
	"with the minimap alone":            {refused: 4, changed: 11, unchanged: 2},
	"with text files that hold nothing": {refused: 9, changed: 6, unchanged: 2},
	"every file under another spelling": {refused: 4, changed: 10, unchanged: 3},
	// Only the two text files can be planned for a folder without a file.
	"an empty folder": {refused: 13, changed: 2, unchanged: 2},
	// The author and the loading screen need the map info alone; the force's name needs the script too.
	"without the script": {refused: 12, changed: 3, unchanged: 2},
	// A preview alone needs the script and the minimap, and no map info.
	"without the map info":          {refused: 10, changed: 5, unchanged: 2},
	"a script that is not UTF-8":    {refused: 12, changed: 3, unchanged: 2},
	"text files that are not UTF-8": {refused: 12, changed: 3, unchanged: 2},
	// The three documents that set the map's name are refused by the script, after the map info was patched.
	"a script without SetMapName": {refused: 7, changed: 8, unchanged: 2},
	// The five documents with a preview are left out: the other tree writes the minimap call after the return.
	"a script whose main() returns a value": {refused: 4, changed: 6, unchanged: 2, broken: 5},
	"bytes that are no map info":            {refused: 10, changed: 5, unchanged: 2},
	// The five documents with a preview are refused for the name the map has already, of either kind of picture.
	"with the name the minimap is kept under": {refused: 9, changed: 6, unchanged: 2},
	"with the name a TGA preview takes":       {refused: 9, changed: 6, unchanged: 2},
}

// planOfOld is the other tree's plan for the map folder at dir: the refusal of its Validate, which is where it
// refuses names that differ only in letter case, else what its Plan returns, under the names this tree gives the
// manifest and the map folder.
func planOfOld(dir, root string, read bothTrees) ([]mapdir.Change, error) {
	if read.refusal != nil {
		return nil, read.refusal
	}
	changes, err := oldsettings.Plan(dir, read.old, oldsettings.PlanOptions{
		ManifestFile: manifestName, SourceLabel: mapLabel, Root: root,
	})
	converted := make([]mapdir.Change, 0, len(changes))
	for _, change := range changes {
		converted = append(converted, mapdir.Change(change))
	}
	return converted, err
}

// breaksTheScript reports whether a plan of the other tree writes a script that its own reader of Lua does not
// read.
func breaksTheScript(changes []mapdir.Change, err error) bool {
	change, written := changeTo(changes, luaName)
	return written && broken(strings.TrimPrefix(string(change.Bytes), byteOrderMark), err)
}

// reach is what the compared plans got to, over every map folder: how often the bytes of each file were
// compared, by the key of its name, and how many of the changes compared remove their file.
type reach struct {
	files    map[string]int
	removals int
}

// compare compares the changes of two plans: the names in order with which of them remove their file, then the
// bytes of each. The file named by apart is the one that both trees are known to write apart: its bytes must
// differ.
func (r *reach) compare(t *testing.T, what string, want, got []mapdir.Change, apart string) {
	t.Helper()
	type entry struct {
		Name   string
		Remove bool
	}
	entries := func(changes []mapdir.Change) []entry {
		listed := make([]entry, 0, len(changes))
		for _, change := range changes {
			listed = append(listed, entry{change.Name, change.Remove})
		}
		return listed
	}
	oracle.Values(t, what+": the changes", entries(want), entries(got))
	for i := range min(len(want), len(got)) {
		key := mapdir.Key(want[i].Name)
		if apart != "" && key == mapdir.Key(apart) {
			if bytes.Equal(want[i].Bytes, got[i].Bytes) {
				t.Errorf("%s: left out, and both trees write the same %s", what, want[i].Name)
			}
			continue
		}
		oracle.Bytes(t, what+": "+want[i].Name, want[i].Bytes, got[i].Bytes)
		r.files[key]++
		if want[i].Remove {
			r.removals++
		}
	}
}

// add counts what another reach has.
func (r *reach) add(other reach) {
	for key, count := range other.files {
		r.files[key] += count
	}
	r.removals += other.removals
}

// add counts what another count has.
func (c *planCounts) add(other planCounts) {
	c.refused += other.refused
	c.changed += other.changed
	c.unchanged += other.unchanged
	c.apart += other.apart
	c.broken += other.broken
}

// comparePlans gives both trees one document, as each of them reads it, and the map folder that is on disk at
// dir, and counts what came of it. It runs beside others of its kind: it opens the folder for itself, and it
// reports and does not stop the test.
func (c *planCounts) comparePlans(t *testing.T, what string, source sourceMap, dir string, read bothTrees, seen *reach) {
	t.Helper()
	want, wantErr := planOfOld(dir, read.project.Root, read)
	folder, err := mapdir.Open(dir, mapLabel)
	if err != nil {
		t.Errorf("%s: %v", what, err)
		return
	}
	got, gotErr := Plan(folder, read.project)
	info := source.held(infoName)
	if change, patched := changeTo(want, infoName); patched {
		info = change.Bytes
	}
	switch {
	case breaksTheScript(want, wantErr):
		// Left out, for the difference that is meant: see TestOracleOnTheMinimapCall.
		c.broken++
		failure, ok := gotErr.(*diag.Error)
		if !ok || failure.File != mapLabel+"/"+luaName || !strings.Contains(failure.Msg, "could not be read back safely") {
			t.Errorf("%s: the other tree's script does not read, and this tree returns %v", what, gotErr)
		}
	case oracle.Refusals(t, what, wantErr, gotErr):
		c.refused++
	case wantErr != nil || gotErr != nil:
		// One tree refused alone, which Refusals has reported.
	case writtenApart(read.project.Settings, info):
		c.apart++
		seen.compare(t, what, want, got, luaName)
	case len(want) == 0:
		seen.compare(t, what, want, got, "")
		c.unchanged++
	default:
		seen.compare(t, what, want, got, "")
		c.changed++
	}
}

// plansFor gives both trees each document for the map folder, which it writes to disk, and returns what came of
// them. The plans run beside each other: each opens the folder for itself, and none writes to it.
func plansFor(t *testing.T, source sourceMap, documents []string, root string) (planCounts, reach) {
	t.Helper()
	dir := source.onDisk(t)
	before := testkit.Snapshot(t, dir)
	read := make([]bothTrees, len(documents))
	for i, document := range documents {
		read[i] = inBothTrees(t, root, document)
	}
	counts, seen := make([]planCounts, len(documents)), make([]reach, len(documents))
	onEveryCore(len(documents), func(i int) {
		seen[i].files = map[string]int{}
		counts[i].comparePlans(t, source.name+", settings "+documents[i], source, dir, read[i], &seen[i])
	})
	if !reflect.DeepEqual(testkit.Snapshot(t, dir), before) {
		t.Errorf("%s: planning wrote to the map", source.name)
	}
	counted, reached := planCounts{}, reach{files: map[string]int{}}
	for i := range documents {
		counted.add(counts[i])
		reached.add(seen[i])
	}
	return counted, reached
}

func TestOracleOnThePlan(t *testing.T) {
	root := planProject(t)
	every := slices.Concat(documents(), constantDocuments(), duplicateDocuments)
	seen := reach{files: map[string]int{}}
	for _, source := range sourceMaps(t) {
		documents := routeDocuments
		if source.every {
			documents = every
		}
		counted, reached := plansFor(t, source, documents, root)
		if want, known := wantPlans[source.name]; !known || counted != want {
			t.Errorf("%s: the plans came to %+v, want %+v", source.name, counted, want)
		}
		seen.add(reached)
	}
	if len(every) != 226 || len(routeDocuments) != 17 {
		t.Errorf("%d documents and %d of them for every map folder, want 226 and 17", len(every), len(routeDocuments))
	}
	// Every file a plan changes is compared, the picture as a BLP and as a TGA, and the minimap's file as one
	// that is removed.
	wantFiles := map[string]int{
		"war3map.w3i": 194, "war3map.lua": 185, "war3mapmisc.txt": 210, "war3mapskin.txt": 53,
		"war3mapminimap.blp": 33, "war3mapmap.blp": 33, "war3mapmap.tga": 21,
	}
	if !maps.Equal(seen.files, wantFiles) || seen.removals != 21 {
		t.Errorf("the plans compared the bytes of %v and %d removals, want %v and 21", seen.files, seen.removals, wantFiles)
	}
}

// ---- the recording ----

// fileOfOld is the file that a refusal of the other tree names.
func fileOfOld(err error) string {
	failure, expected := olddiag.First(err)
	if !expected {
		return "(an error without a file)"
	}
	return failure.File
}

// scriptsOfOld is what the other tree makes of every script for the settings of a document, in the shape
// scriptsWith gives for this tree. The map info is the fixture's as the other tree patches it. A script into
// which a number goes that the trees write apart is this tree's, which the note beside the document says.
func scriptsOfOld(t testing.TB, document string, sources []script) recordedScripts {
	t.Helper()
	old, project := accepted(t, "", document)
	info, err := oldsettings.PatchMapInfo(fixtureInfo(t), old, infoFile)
	if err != nil {
		return recordedScripts{refused: true, refusedAt: fileOfOld(err)}
	}
	made, apart := recordedScripts{}, writtenApart(project.Settings, info)
	if apart {
		made.note = notePlainDecimal
	}
	for _, source := range sources {
		text, err := oldsettings.PatchLua(source.text, old, info, luaFile, infoFile)
		switch {
		case err != nil:
			text = refusedAt(fileOfOld(err))
		case apart:
			if text, err = afterInfo(source.text, project.Settings, info); err != nil {
				t.Errorf("%s, settings %s: this tree refuses what the other tree takes: %v", source.name, document, err)
			}
		}
		made.scripts = append(made.scripts, madeOf(source, text))
	}
	return made
}

// planByOld is the other tree's plan of a document for a map folder, in the shape planFor gives for this tree.
// The two results that are this tree's each have their note: the refusal of a preview for a script whose main()
// returns a value, where the other tree writes a script that does not load; and the bytes of a script into which
// a number goes that the trees write apart.
func planByOld(t testing.TB, source sourceMap, dir, root, document string) recordedPlan {
	t.Helper()
	read := inBothTrees(t, root, document)
	want, wantErr := planOfOld(dir, root, read)
	switch {
	case breaksTheScript(want, wantErr):
		return recordedPlan{refused: true, refusedAt: mapLabel + "/" + luaName, note: noteAfterAReturn}
	case wantErr != nil:
		return recordedPlan{refused: true, refusedAt: fileOfOld(wantErr)}
	}
	planned := recordedPlan{changes: want}
	info := source.held(infoName)
	if change, patched := changeTo(want, infoName); patched {
		info = change.Bytes
	}
	if !writtenApart(read.project.Settings, info) {
		return planned
	}
	planned.note = notePlainDecimal
	folder, err := mapdir.Open(dir, mapLabel)
	if err != nil {
		t.Errorf("%s: %v", source.name, err)
		return planned
	}
	got, err := Plan(folder, read.project)
	ours, written := changeTo(got, luaName)
	at := slices.IndexFunc(planned.changes, func(change mapdir.Change) bool {
		return mapdir.Key(change.Name) == mapdir.Key(luaName)
	})
	if err != nil || !written || at < 0 {
		t.Errorf("%s, settings %s: no script of both trees to take this tree's of: %v", source.name, document, err)
		return planned
	}
	planned.changes[at].Bytes = ours.Bytes
	return planned
}

// TestOracleOnTheRecordedSettings holds testdata/recorded/settings.txt to what the other tree makes of every
// document the recording names. It is the test that writes it: MOONWELL_RECORD=1 with -run of this test alone.
func TestOracleOnTheRecordedSettings(t *testing.T) {
	testkit.Recorded(t, "settings.txt", recordedSettings(t, recorder{
		scripts: func(document string, sources []script) recordedScripts {
			return scriptsOfOld(t, document, sources)
		},
		plan: func(source sourceMap, dir, root, document string) recordedPlan {
			return planByOld(t, source, dir, root, document)
		},
	}))
}

func TestOracleOnAFolderUnderANameAPreviewAdds(t *testing.T) {
	root := planProject(t)
	type entries struct {
		folder string
		file   string // a file under the other name the preview adds, or ""
	}
	// The last map has both names: both trees look at the name the minimap is kept under first.
	maps := []entries{{folder: "war3mapMinimap.blp"}, {folder: "war3mapMap.tga"}, {folder: "War3mapMap.TGA"},
		{folder: "war3mapMinimap.blp", file: "war3mapMap.tga"}}
	compared := 0
	for _, held := range maps {
		dir := fixtureMap(t)
		testkit.WriteFile(t, dir, "war3mapMap.blp", minimapBytes)
		if err := os.Mkdir(filepath.Join(dir, held.folder), 0o777); err != nil {
			t.Fatal(err)
		}
		if held.file != "" {
			testkit.WriteFile(t, dir, held.file, []byte{1})
		}
		for _, preview := range []string{"preview.blp", "preview.tga", "art/Preview.PNG"} {
			what := fmt.Sprintf("a folder %s, a file %q, the preview %s", held.folder, held.file, preview)
			// The player is one the fixture lacks: both trees must refuse for the name before they read the map info.
			read := inBothTrees(t, root, `{"info":{"preview":"`+preview+`"},"players":{"5":{"name":"Absent"}}}`)
			_, wantErr := planOfOld(dir, root, read)
			_, gotErr := Plan(openMap(t, dir), read.project)
			want, wantIs := wantErr.(*olddiag.Error)
			got, gotIs := gotErr.(*diag.Error)
			if !wantIs || !gotIs {
				t.Errorf("%s: the refusals are %v and %v, want a diag error of each tree", what, wantErr, gotErr)
				continue
			}
			if want.Msg != "The map already has "+held.folder+", a name the preview picture needs." ||
				!strings.HasSuffix(got.Msg, " would replace a folder in the map.") || got.File != want.File || got.Hint == "" {
				t.Errorf("%s: the refusals differ in more than their words:\nwant: %+v\ngot:  %+v", what, want, got)
			}
			compared++
		}
	}
	if compared != 12 {
		t.Errorf("%d refusals compared, want 12", compared)
	}
}

func TestOracleOnAFolderUnderAFileTheSettingsNeed(t *testing.T) {
	const wantStart, gotWords = "Reading a map file for map settings failed: ", " in the map is a folder, not a file."
	tests := []struct{ name, folder, document string }{
		{"war3map.w3i", "war3map.w3i", `{"loadingScreen":{"title":"T"}}`},
		{"war3map.lua", "war3map.lua", `{"info":{"name":"N"}}`},
		// A preview alone reads the script, for the call that gives the minimap back, and no map info.
		{"war3map.lua", "war3map.lua", previewAt("preview.tga")},
		// The minimap, for a picture that is written beside its file and for one that takes its file.
		{"war3mapMap.blp", "war3mapMap.blp", previewAt("preview.tga")},
		{"war3mapMap.blp", "war3mapMap.blp", previewAt("preview.blp")},
		// Both trees name the folder as the map spells it.
		{"war3map.w3i", "WAR3MAP.W3I", `{"loadingScreen":{"title":"T"}}`},
	}
	compared := 0
	for _, tt := range tests {
		what := "a folder " + tt.folder + ", settings " + tt.document
		dir, root := withPreview(t, "preview.tga", plainTGA())
		testkit.WriteFile(t, root, "preview.blp", testkit.BLP(256, 1))
		if err := os.Remove(filepath.Join(dir, tt.name)); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, tt.folder), 0o777); err != nil {
			t.Fatal(err)
		}
		read := inBothTrees(t, root, tt.document)
		_, wantErr := planOfOld(dir, root, read)
		_, gotErr := Plan(openMap(t, dir), read.project)
		want, wantIs := wantErr.(*olddiag.Error)
		got, gotIs := gotErr.(*diag.Error)
		if !wantIs || !gotIs {
			t.Errorf("%s: the refusals are %v and %v, want a diag error of each tree", what, wantErr, gotErr)
			continue
		}
		if !strings.HasPrefix(want.Msg, wantStart) || got.Msg != tt.folder+gotWords || got.File != want.File || got.Hint == "" {
			t.Errorf("%s: the refusals differ in more than their words:\nwant: %+v\ngot:  %+v", what, want, got)
			continue
		}
		compared++
	}
	if compared != 6 {
		t.Errorf("%d refusals compared, want 6", compared)
	}
}

func TestOracleOnAMapFileThatCannotBeRead(t *testing.T) {
	const wantStart, gotStart = "Reading a map file for map settings failed: ", "Reading a map file failed: "
	tests := []struct{ name, document string }{
		{"war3map.w3i", `{"loadingScreen":{"title":"T"}}`},
		{"war3map.lua", `{"info":{"name":"N"}}`},
		{"war3mapSkin.txt", `{"gameInterface":{"A":{"B":"c"}}}`},
		// The minimap, which a plan with a preview reads last, to keep it.
		{"war3mapMap.blp", previewAt("preview.tga")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, root := withPreview(t, "preview.tga", plainTGA())
			testkit.WriteFile(t, dir, "war3mapSkin.txt", []byte("[A]\n"))
			testkit.MakeUnreadable(t, filepath.Join(dir, tt.name))
			read := inBothTrees(t, root, tt.document)
			_, wantErr := planOfOld(dir, root, read)
			_, gotErr := Plan(openMap(t, dir), read.project)
			want, wantIs := wantErr.(*olddiag.Error)
			got, gotIs := gotErr.(*diag.Error)
			if !wantIs || !gotIs {
				t.Fatalf("the refusals are %v and %v, want a diag error of each tree", wantErr, gotErr)
			}
			reason, worded := strings.CutPrefix(want.Msg, wantStart)
			if !worded || reason == "" || got.Msg != gotStart+reason || got.File != want.File || got.Hint != want.Hint {
				t.Errorf("the refusals differ in more than their first words:\nwant: %+v\ngot:  %+v", want, got)
			}
		})
	}
}
