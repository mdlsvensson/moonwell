package settings

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"syscall"
	"testing"

	olddiag "github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	oldsettings "github.com/mdlsvensson/moonwell/internal/settings"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
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
//   - GameplaySections against gameplaySections, and the game interface sections the other tree's Validate
//     returns against sections: what is refused, else the sections with their keys and values in order.
//   - What the other tree's Validate refuses for names that differ only in letter case, against the refusal of
//     sections: this tree makes that check where the sections are used.
//   - LoadPreview against loadPreview, on a project folder with a picture of each kind: what is refused, else
//     the extension and the bytes of the picture.
//
// The documents are every one that the other tree's tests give its Validate and see accepted, one that sets
// every setting, and for the constants every pairing of a typed constant with a raw one.
//
// Left out, for the one difference that is meant: a preview path that runs through a file
// ("preview.tga/inner.tga"), on a system that calls that "not a directory". There the other tree passes the
// system's error on as an error that is not a diag error, and this tree says on every system that the picture
// does not exist (TestAPreviewPathThroughAFileNamesAFileThatDoesNotExist). The path is among the inputs. It is
// left out when the other tree's result is such an error, which is counted: once on those systems, and never on
// Windows, where both trees say the same and are compared. That this tree refuses it too is still looked at.
//
// TestOracleOnAPreviewThatCannotBeRead compares nothing, and is skipped, where the test cannot make a file that
// is there and cannot be read: as root on a system other than Windows, since root reads a file without
// permissions, and on a Windows that lets the file be read although the test holds it with no sharing and an
// exclusive lock.
//
// Not among the inputs:
//
//   - A document whose shape the other tree refuses (a wrong name, range or type): that is Pkl's to refuse.
//   - A section or a key whose name looks like a number. Neither tree is given one: the schema and the other
//     tree's Validate both take only names that start with a letter or an underscore.

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

// ---- the documents ----

// The documents of the other tree's tests of the map info.
var mapInfoDocuments = []string{
	`{"loadingScreen":{"model":"Loading.mdx","text":"Text","title":"Title","subtitle":"Subtitle"}}`,
	`{"environment":{"soundEnvironment":"Dungeon","waterColor":[255,0,0,255],"fog":{"enabled":true,"color":[255,0,0,255]}}}`,
	`{"info":{"name":"M` + "\xc3\xb8\xc3\xb8" + `nwell","author":"","description":"TRIGSTR_001"},"loadingScreen":{"title":"Changed","background":7}}`,
	`{"info":{"name":"TRIGSTR_001","author":"Author","description":"Description"},"loadingScreen":{"title":"Title","background":0}}`,
	`{"players":{"0":{"x":256,"controller":"computer"}},"forces":{"0":{"name":"Blue","sharedVision":true}},
		"environment":{"soundEnvironment":"Dungeon","waterColor":[2,3,4,255],"fog":{"start":2000}}}`,
	`{"players":{"0":{"x":128,"controller":"user"}},"forces":{"0":{"name":"Force 1","sharedVision":false}},
		"environment":{"soundEnvironment":"Default","waterColor":[255,255,255,255],"fog":{"start":1000}}}`,
	`{"players":{"0":{"name":"P"}}}`,
	`{"loadingScreen":{"model":""}}`,
	`{"players":{"2":{"name":"P"}}}`,
	`{"forces":{"1":{"name":"F"}}}`,
	`{"environment":{"fog":{"start":6000}}}`,
	`{"info":{"name":"Safe"}}`,
	`{"environment":{"soundEnvironment":"Safe"}}`,
	`{"environment":{"fog":{"enabled":true}}}`,
	`{}`,
	`{"forces":{"0":{"name":"X"}}}`,
	`{"players":{"7":{"name":"P"}}}`,
	`{"forces":{"0":{"name":"F"}}}`,
}

// The documents of the other tree's tests of the two text files.
var textDocuments = []string{
	`{"gameInterface":{"Misc":{"MaxHeroLevel":"25","Added":"0"},"CustomSkin":{"Text":""}}}`,
	`{"gameInterface":{"Misc":{"FoodCeiling":"0"}}}`,
	`{"gameplay":{"foodLimit":200},"gameplayConstants":{"misc":{"foodceiling":"200"}}}`,
	`{"gameplay":{"foodLimit":200},"gameplayConstants":{"misc":{"foodceiling":"0200"}}}`,
	`{"gameplay":{"foodLimit":200},"gameplayConstants":{"misc":{"foodceiling":"100"}}}`,
	`{"gameplay":{"heroMaxLevel":25,"foodLimit":150},"gameplayConstants":{"Other":{"A":"1"}}}`,
	`{"gameInterface":{"Misc":{"FoodCeiling":"200"}}}`,
	`{"gameInterface":{"Misc":{"FoodCeiling":"0"},"Skin":{"Text":""},"New":{"Value":"1"}}}`,
	`{"gameInterface":{"Misc":{"B":"2"}}}`,
	`{"gameInterface":{"A":{"K":"v"}}}`,
	`{"gameInterface":{"New":{"K":"v"}}}`,
}

// The documents of the other tree's tests of the plan.
var planDocuments = []string{
	`{"info":{"name":"Planned"},"gameplay":{"foodLimit":200}}`,
	`{"info":{"name":"Not written"},"players":{"5":{"name":"Absent"}}}`,
	`{"info":{"name":"Planned"},"gameplay":{"heroMaxLevel":25}}`,
	`{"gameInterface":{"CustomSkin":{"Test":"value"}},
		"gameplayConstants":{"Misc":{"GoldCost":"1"}},"info":{"description":"Described"}}`,
	`{"gameInterface":{"CustomSkin":{"Test":""}}}`,
	`{"info":{"name":null},"players":{"5":{"name":null}},"environment":{"fog":{}},
		"gameplayConstants":{"Misc":{}},"gameInterface":{"CustomSkin":{}}}`,
	`{"gameplay":{"foodLimit":100}}`,
	`{"info":{"author":"Someone"},"loadingScreen":{"title":"T"}}`,
	`{"loadingScreen":{"title":"T"}}`,
	`{"info":{"name":"Needs Lua"}}`,
	`{"environment":{"soundEnvironment":"Mountains"}}`,
	`{"gameplay":{"heroMaxLevel":5}}`,
	`{"gameInterface":{"A":{"B":"c"}}}`,
	`{"info":{"name":"X"}}`,
	`{"info":{"name":"Refused"},"gameplay":{"foodLimit":1},"gameInterface":{"A":{"B":"c"}}}`,
	`{"gameplay":{"foodLimit":200},"gameplayConstants":{"MISC":{"foodCeiling":"1"}}}`,
	`{"info":{"name":"BOM"},"gameInterface":{"A":{"B":"c"}}}`,
	`{
		"info":{"name":"Name","description":""},
		"players":{"0":{"name":"Hero","controller":"computer","fixedStart":false,"x":256}},
		"forces":{"0":{"allied":false,"alliedVictory":true}},
		"environment":{"soundEnvironment":"","waterColor":[1,2,3,4],"fog":{"enabled":true,"start":1,"end":2}},
		"gameplay":{"heroMaxLevel":20,"foodLimit":150},
		"gameplayConstants":{"misc":{"Other":"1"}},
		"gameInterface":{"CustomSkin":{"A":"b"}}}`,
	`{"players":{"5":{"name":"Absent"}}}`,
	`{"info":{"name":"Refused"}}`,
	`{"gameplay":{"foodLimit":1}}`,
	`{"info":{"name":"Labelled"}}`,
	`{"info":{"name":"Cased"},"gameplay":{"foodLimit":7},"gameInterface":{"A":{"B":"c"}}}`,
	`{"info":{"preview":"preview.blp"}}`,
	`{"info":{"preview":"art/Preview.TGA"}}`,
	`{"info":{"preview":"art/Preview.PNG"}}`,
	`{"info":{"preview":"preview.tga"}}`,
	`{"info":{"name":"Both","preview":"preview.blp"},"gameplay":{"foodLimit":200},
		"gameInterface":{"CustomSkin":{"Test":"value"}}}`,
}

// The documents of the other tree's tests of the Lua, those that run it among them.
var luaDocuments = []string{
	`{
		"info":{"name":"A \"quoted\" map\n` + "\xe9\x9b\xaa" + `"},
		"players":{"0":{"name":"Hero","controller":"computer","race":"orc","fixedStart":false,"x":256}},
		"environment":{"waterColor":[10,20,30,255],"fog":{"enabled":true,"start":100,"end":1000}}}`,
	`{"info":{"name":"Name"}}`,
	`{"info":{"author":"Author","recommendedPlayers":""},"loadingScreen":{"title":"Title"},
		"forces":{"0":{"name":"Allies"}},"gameplay":{"heroMaxLevel":20}}`,
	`{"info":{"name":"Name","description":""}}`,
	`{"players":{"0":{"name":"Hero","race":"selectable","fixedStart":false}}}`,
	`{"players":{"1":{"fixedStart":true}}}`,
	`{"players":{"11":{"fixedStart":true,"controller":"computer"}}}`,
	`{"players":{"1":{"name":"Tab\there ` + "\xe2\x9c\x93" + `","controller":"rescuable"}}}`,
	`{"players":{"0":{"name":"Hero"}}}`,
	`{"players":{"11":{"x":0.1}}}`,
	`{"environment":{"fog":{"enabled":true,"start":0,"density":0.3,"color":[255,0,51,128]}}}`,
	`{"players":{"0":{"controller":"computer"}}}`,
	`{"players":{"1":{"controller":"computer"}}}`,
	`{"forces":{"0":{"allied":true}}}`,
	`{"environment":{"waterColor":[1,2,3,4]}}`,
	`{"info":{"name":"x"}}`,
	`{"players":{"0":{"x":1}}}`,
	`{"players":{"0":{"fixedStart":false}}}`,
	`{"players":{"0":{"fixedStart":true}}}`,
	`{"players":{"0":{"race":"orc"}}}`,
	`{"players":{"0":{"name":"x"}}}`,
	`{"forces":{"1":{"allied":true}}}`,
	`{"environment":{"soundEnvironment":"Cave"}}`,
	`{"environment":{"fog":{"enabled":false}}}`,
	`{"forces":{"0":{"allied":false,"alliedVictory":true,"sharedVision":false,"sharedControl":true},"1":{}}}`,
	`{"forces":{"1":{"sharedVision":true}}}`,
	`{"environment":{"soundEnvironment":"","fog":{"enabled":false}}}`,
	`{"environment":{"soundEnvironment":"Cave","fog":{"enabled":true}}}`,
	`{"info":{"name":"Both"},"environment":{"soundEnvironment":"Dungeon"}}`,
	`{"info":{"name":"\n123"},"environment":{"fog":{"enabled":true,"start":100,"end":1000}}}`,
	`{"info":{"name":"Moonwell","description":""},
		"players":{"0":{"name":"","controller":"computer","race":"selectable","fixedStart":false,"x":0,"y":0.1},
			"11":{"controller":"rescuable","race":"undead"}},
		"forces":{"0":{"allied":false,"alliedVictory":true,"sharedVision":false,"sharedControl":true}},
		"environment":{"soundEnvironment":"","waterColor":[0,0,0,0],"fog":{"enabled":true,"start":0,"color":[255,0,0,0]}}}`,
	`{"environment":{"fog":{"enabled":false,"density":0}}}`,
}

// The documents of the other tree's tests of its Validate that it accepts.
func optionDocuments() []string {
	documents := []string{
		`{"info":{"name":null},"players":{"23":{"name":null}},"environment":{"fog":{}}}`,
		`{"info":{"name":""},"players":{"0":{"fixedStart":false,"x":0}},"gameplay":{"foodLimit":0}}`,
		`{"gameInterface":{"constructor":{"constructor":"ok"}}}`,
		`{"loadingScreen":{"background":-1}}`,
		`{"loadingScreen":{"background":2147483647}}`,
		`{"gameplay":{"heroMaxLevel":1,"foodLimit":0}}`,
		`{"gameplay":{"heroMaxLevel":10000,"foodLimit":300}}`,
		`{"players":{"23":{"x":-10000000,"y":10000000}}}`,
		`{"environment":{"fog":{"style":0,"density":0,"start":-10000000,"end":10000000}}}`,
		`{"environment":{"fog":{"style":2,"density":1},"waterColor":[0,255,0,255]}}`,
		`{"environment":{"waterColor":[1,2,3,4]},"gameplayConstants":{"Misc":{"FoodCeiling":"0"}}}`,
		`{"players":{"0":{"x":256,"y":-896}},"environment":{"fog":{"start":100,"end":1000,"density":1}}}`,
		`{"info":{"name":"N","preview":"art/preview.tga"}}`,
		`{"info":{"preview":"p.blp"}}`,
		`{"info":{"preview":null}}`,
		`{"players":{"10":{"name":"k"},"2":{"name":"c"},"0":{"name":"a"}}}`,
	}
	for _, controller := range controllers[1:] {
		documents = append(documents, `{"players":{"0":{"controller":"`+controller+`"}}}`)
	}
	for _, race := range races {
		documents = append(documents, `{"players":{"0":{"race":"`+race+`"}}}`)
	}
	for _, flag := range []string{"allied", "alliedVictory", "sharedVision", "sharedControl", "sharedAdvancedControl"} {
		documents = append(documents, `{"forces":{"0":{"`+flag+`":false}}}`)
	}
	return documents
}

// The documents of this file: each sets every setting that some map info can take.
var everyDocuments = []string{
	// Every setting there is, for the player and the force that every map with players and forces has.
	`{"info":{"name":"Every \"setting\"","author":"An author","description":"One|nTwo","recommendedPlayers":"2-4",
			"preview":"preview.tga"},
		"loadingScreen":{"background":3,"model":"Loading\\Screen.mdx","text":"Text","title":"Title","subtitle":"Subtitle"},
		"players":{"0":{"name":"Hero","controller":"computer","race":"nightelf","fixedStart":false,"x":-512.25,"y":1024.5}},
		"forces":{"0":{"name":"The Alliance","allied":true,"alliedVictory":false,"sharedVision":true,"sharedControl":false,
			"sharedAdvancedControl":true}},
		"environment":{"soundEnvironment":"Mountains","waterColor":[10,20,30,40],
			"fog":{"enabled":true,"style":1,"start":500.5,"end":4000,"density":0.75,"color":[50,60,70,80]}},
		"gameplay":{"heroMaxLevel":20,"foodLimit":150},
		"gameplayConstants":{"Misc":{"DefenseArmor":"0.05"},"Other":{"Key":""}},
		"gameInterface":{"FrameDef":{"UPKEEP_NONE":"No upkeep"},"CustomSkin":{"A":"b"}}}`,
	// Every setting that a map info of any version from 25 holds.
	`{"info":{"name":"","author":"A","description":"D","recommendedPlayers":"Any","preview":"art/p.png"},
		"loadingScreen":{"background":-1,"model":"","text":"","title":"T","subtitle":"S"}}`,
	// Every setting that a map info of any version holds.
	`{"info":{"name":"N","author":"A","description":"","recommendedPlayers":""},
		"loadingScreen":{"background":0,"text":"Text","title":"","subtitle":""}}`,
	// Every setting of the players and the forces of the fixture, written out of slot order, with the alliance
	// flags turned the other way.
	`{"players":{"11":{"name":"Last","controller":"rescuable","race":"undead","fixedStart":true,"x":1.5,"y":-2.5},
			"1":{"name":"Second","controller":"neutral","race":"human","fixedStart":false,"x":0,"y":0},
			"0":{"name":"","controller":"user","race":"selectable","fixedStart":true,"x":-0.0,"y":3e-7}},
		"forces":{"1":{"name":"","allied":false,"alliedVictory":true,"sharedVision":false,"sharedControl":true,
				"sharedAdvancedControl":false},
			"0":{"name":"First","allied":true,"alliedVictory":true,"sharedVision":true,"sharedControl":true,
				"sharedAdvancedControl":true}},
		"environment":{"soundEnvironment":"","waterColor":[0,0,0,0],
			"fog":{"enabled":false,"style":0,"start":-10,"end":-10,"density":0,"color":[0,0,0,0]}}}`,
	// Overrides with nothing set beside ones that set something, for slots the maps lack.
	`{"players":{"23":{},"0":{"name":"Hero"},"9":{"name":null}},"forces":{"7":{},"0":{"sharedControl":true}}}`,
	// The first of several refusals: a player, then a force, then the fog.
	`{"environment":{"fog":{"start":2,"end":1}},"forces":{"9":{"name":"F"},"3":{"name":"F"}},
		"players":{"10":{"name":"P"},"2":{"name":"P"}}}`,
	`{"environment":{"fog":{"start":2,"end":1}},"forces":{"9":{"name":"F"},"3":{"name":"F"}}}`,
	`{"environment":{"fog":{"start":2,"end":1},"waterColor":[1,1,1,1]}}`,
	`{"environment":{"fog":{"density":0.5}}}`,
	`{"environment":{"fog":{"end":999.5}}}`,
}

// documents is every document that both trees accept.
func documents() []string {
	return slices.Concat(mapInfoDocuments, textDocuments, planDocuments, luaDocuments, optionDocuments(), everyDocuments)
}

// constantDocuments pairs each typed gameplay constant, and both, and none, with raw constants: none at all, or a
// Misc section in each of three spellings that holds no key of a typed constant, or such a key in one of three
// spellings with a value that is the typed one or another. The section before Misc holds a key of a typed
// constant too, and is left alone.
func constantDocuments() []string {
	var documents []string
	for _, typed := range []string{``, `"foodLimit":200`, `"heroMaxLevel":25`, `"heroMaxLevel":25,"foodLimit":200`} {
		documents = append(documents, `{"gameplay":{`+typed+`}}`)
		for _, section := range []string{"Misc", "misc", "MISC"} {
			for _, keys := range []string{
				``, `"Other":"1"`, `"FoodCeiling":"200"`, `"foodceiling":"0200"`, `"Other":"1","MAXHEROLEVEL":"25"`,
				`"maxherolevel":"7","FoodCeiling":"200"`, `"FOODCEILING":"200","Other":"","MaxHeroLevel":"25"`,
			} {
				documents = append(documents, `{"gameplay":{`+typed+`},"gameplayConstants":{"First":{"FoodCeiling":"1"},"`+
					section+`":{`+keys+`},"Last":{}}}`)
			}
		}
	}
	return documents
}

// The documents the other tree's Validate refuses for names that differ only in letter case: the two of its
// tests, and more of each kind.
var duplicateDocuments = []string{
	`{"gameplayConstants":{"Misc":{},"misc":{}}}`,
	`{"gameInterface":{"Frame":{"X":"a","x":"b"}}}`,
	`{"gameInterface":{"A":{"k":"v"},"B":{},"a":{}}}`,
	`{"gameplayConstants":{"Misc":{"Key":"1","Other":"2","KEY":"3"}}}`,
	`{"gameInterface":{"A":{"k":"1","K":"2"},"a":{}}}`,
	`{"gameplay":{"foodLimit":1},"gameplayConstants":{"Misc":{"FoodCeiling":"2"},"MISC":{}}}`,
	`{"gameplayConstants":{"A":{"B":"1"},"a":{"B":"1","b":"2"}}}`,
}

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
		got, gotErr := gameplaySections(project.Settings, manifestName)
		if oracle.Refusals(t, "the constants of "+document, wantErr, gotErr) {
			conflicts++
		} else {
			oracle.Values(t, "the constants of "+document, sectionsOfOld(&want), got)
			merged++
			if project.Settings.Gameplay != (manifest.Gameplay{}) {
				typed++
			}
		}
		skin, err := sections(project.Settings.GameInterface, "settings.gameInterface", manifestName)
		if err != nil {
			t.Errorf("the interface of %s: %v", document, err)
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
		_, err := gameplaySections(read.project.Settings, manifestName)
		if err == nil {
			_, err = sections(read.project.Settings.GameInterface, "settings.gameInterface", manifestName)
		}
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
	makeUnreadable(t, testkit.WriteFile(t, root, "art/preview.tga", plainTGA()))
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
