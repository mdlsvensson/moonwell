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
	"sync"
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
	// Two spellings in the interface beside a typed constant that disagrees with a raw one, and two spellings in
	// both blocks: which refusal is the one told.
	`{"gameplay":{"foodLimit":1},"gameplayConstants":{"Misc":{"FoodCeiling":"2"}},"gameInterface":{"A":{},"a":{}}}`,
	`{"gameInterface":{"A":{},"a":{}},"gameplayConstants":{"Misc":{"Key":"1","KEY":"2"}}}`,
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

// The documents with a zero below 0: set as one, or what the map info keeps of a number too small for it.
var zeroDocuments = []string{
	`{"players":{"0":{"x":-0.0,"y":0}}}`,
	`{"players":{"0":{"x":1e-46,"y":-1e-46}}}`,
	`{"environment":{"fog":{"enabled":true,"start":-0.0,"end":-0.0,"density":-0.0,"color":[1,2,3,4]}}}`,
}

// The documents with a number that the two trees write apart, beside the one among everyDocuments, and after
// them documents with such a number that no script takes: of a fog that is not shown, and of a player whose
// position is not set.
var apartDocuments = []string{
	`{"players":{"0":{"x":0.000001}}}`,
	`{"players":{"1":{"x":7},"11":{"y":-0.0000001}}}`,
	`{"environment":{"fog":{"enabled":true,"start":-0.0000001,"density":1e-7}}}`,
	`{"environment":{"fog":{"enabled":false,"density":1e-7}}}`,
	`{"environment":{"fog":{"density":1e-7}}}`,
}

// scriptDocuments is the documents that every script is patched for.
func scriptDocuments() []string {
	return slices.Concat(documents(), zeroDocuments, apartDocuments)
}

// script is a war3map.lua, or a text given as one.
type script struct{ name, text string }

// scripts is the fixture's script, every script the other tree's tests of the Lua patch or see refused, and the
// fixture's in other layouts.
func scripts(t *testing.T) []script {
	t.Helper()
	fixture := fixtureLua(t)
	notHeld := swapped(t, fixture, "ForcePlayerStartLocation(Player(1), 1)\r\n", "")
	all := []script{
		{"the fixture", fixture},
		{"without SetMapName", swapped(t, fixture, "SetMapName(", "Other(")},
		{"with a second config()", fixture + "\nfunction config() SetMapName(\"x\") end"},
		{"with SetMapName of an object", swapped(t, fixture, "SetMapName(", "object.SetMapName(")},
		{"a script that does not read", "function (((unreadable"},
		{"with player 1 not held to its start", notHeld},
		{"with a SetPlayerName", swapped(t, fixture, "SetPlayerColor(Player(1), ConvertPlayerColor(1))",
			"SetPlayerColor(Player(1), ConvertPlayerColor(1))\r\nSetPlayerName(Player(1), \"TRIGSTR_006\")")},
		{"a main() alone, indented", indentedMain},
	}
	for _, shape := range unsafeShapes {
		all = append(all, script{fmt.Sprintf("with %q for %q", shape.new, shape.old), swapped(t, fixture, shape.old, shape.new)})
	}
	for _, c := range joinable {
		all = append(all, script{fmt.Sprintf("the script %q", c.source), c.source})
	}
	for _, source := range slices.Concat(slices.Sorted(maps.Keys(minimapSources)), slices.Sorted(maps.Keys(withoutOneMain))) {
		all = append(all, script{fmt.Sprintf("the script %q", source), source})
	}
	// A call that is added takes its line ending, its indentation and its semicolon from the layout, and the one
	// call a setting adds to a player is the one that holds the player to its start: each layout comes once more
	// without that call for player 1.
	all = append(all, layouts(t, "", fixture)...)
	return append(all, layouts(t, "with player 1 not held to its start, ", notHeld)...)
}

// layouts is a script of the fixture's shape written in other ways that say the same. Each is named by what
// stands before its name.
func layouts(t *testing.T, before, fixture string) []script {
	t.Helper()
	oneLine := strings.ReplaceAll(swapped(t, fixture, "--\r\n", ""), "\r\n", " ")
	together := swapped(t, fixture, "SetPlayerStartLocation(Player(0), 0)\r\nForcePlayerStartLocation(Player(0), 0)\r\nSetPlayerColor",
		"SetPlayerStartLocation(Player(0), 0)ForcePlayerStartLocation(Player(0), 0)SetPlayerColor")
	together = swapped(t, together, "NewSoundEnvironment(\"Default\")\r\n", "")
	together = swapped(t, together, "SetMapMusic(\"Music\", true, 0)\r\nCreateAllUnits()\r\n",
		"SetMapMusic(\"Music\", true, 0)\r\nNewSoundEnvironment(\"Default\")ResetTerrainFog()CreateAllUnits()")
	return []script{
		{before + "with the line endings of Unix", strings.ReplaceAll(fixture, "\r\n", "\n")},
		{before + "with a semicolon after every call", strings.ReplaceAll(fixture, ")\r\n", ");\r\n")},
		{before + "with every line indented", strings.ReplaceAll(fixture, "\r\n", "\r\n\t  ")},
		{before + "on one line", oneLine},
		{before + "on one line with semicolons", strings.ReplaceAll(oneLine, ") ", "); ")},
		{before + "with calls that touch", together},
	}
}

// writtenApart reports whether the settings make a script take, from the map info given as bytes, a number that
// the two trees write apart: a position of a player whose position is set, or a start, an end or a density of a
// fog that is set and shown, that is not 0 and is below 0.000001 in size or from 1e21.
func writtenApart(s manifest.Settings, patchedInfo []byte) bool {
	info, err := w3i.Read(patchedInfo, infoFile, w3i.Extended)
	if err != nil {
		return false
	}
	apart := func(values ...float32) bool {
		return slices.ContainsFunc(values, func(value float32) bool {
			size := math.Abs(float64(value))
			return size != 0 && (size < 0.000001 || size >= 1e21)
		})
	}
	for _, player := range info.Details.Players {
		override := s.Players[int(player.ID.Value)]
		if (override.X != nil || override.Y != nil) && apart(player.X.Value, player.Y.Value) {
			return true
		}
	}
	fog := info.Details.Fog
	return s.Environment.Fog != (manifest.Fog{}) && info.Flags.Value&fogOn != 0 &&
		apart(fog.Start.Value, fog.End.Value, fog.Density.Value)
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

// onEveryCore runs each piece of work on one of as many goroutines as the machine runs at once, and returns when
// all are done.
func onEveryCore(pieces int, work func(piece int)) {
	queue := make(chan int)
	var workers sync.WaitGroup
	for range runtime.GOMAXPROCS(0) {
		workers.Go(func() {
			for piece := range queue {
				work(piece)
			}
		})
	}
	for piece := range pieces {
		queue <- piece
	}
	close(queue)
	workers.Wait()
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

// planProject is a project folder that holds, at each path a document names as its preview, a picture of the
// kind the path says. The picture that documents name in two spellings is written under both: a file system
// that keeps the spellings apart then has two files and another has one, and both give the same bytes for either.
func planProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	picture := testkit.NewPixels(256)
	packed := testkit.TGA(picture, testkit.TGAOptions{RLE: true, Depth: 24, FromTop: true})
	for _, file := range []struct {
		name string
		data []byte
	}{
		{"preview.blp", testkit.BLP(256, 1)},
		{"p.blp", testkit.BLP(512, 0)},
		{"preview.tga", plainTGA()},
		{"art/Preview.TGA", packed},
		{"art/preview.tga", packed},
		{"art/Preview.PNG", testkit.PNG(picture, "rgba")},
		{"art/p.png", testkit.PNG(testkit.GreyPixels(256), "grey")},
	} {
		testkit.WriteFile(t, root, file.name, file.data)
	}
	return root
}

// routeDocuments is one document for each way through a plan: the settings of each file alone and together, a
// preview of each kind alone and beside other settings, and what is refused before the map is read, by the map
// info, and by neither.
var routeDocuments = []string{
	`{}`,
	`{"info":{"author":"Someone"},"loadingScreen":{"title":"T"}}`,
	`{"info":{"name":"Planned"},"gameplay":{"foodLimit":200}}`,
	`{"environment":{"soundEnvironment":"Mountains"}}`,
	// A force's name is stored in the map info alone, and the script is read all the same.
	`{"forces":{"0":{"name":"Blue"}}}`,
	`{"players":{"5":{"name":"Absent"}}}`,
	// The text files of sourceMaps hold this constant, and lack the interface's key.
	`{"gameplay":{"foodLimit":100}}`,
	`{"gameInterface":{"A":{"B":"c"}}}`,
	`{"gameplayConstants":{"Empty":{}},"gameInterface":{"Empty":{}}}`,
	everyDocuments[0],
	`{"info":{"preview":"preview.blp"}}`,
	`{"info":{"preview":"preview.tga"}}`,
	`{"info":{"preview":"art/Preview.PNG"}}`,
	`{"info":{"name":"Both","preview":"preview.blp"},"gameplay":{"foodLimit":200},
		"gameInterface":{"CustomSkin":{"Test":"value"}}}`,
	`{"info":{"name":"N","preview":"missing.tga"}}`,
	`{"info":{"name":"N","preview":"preview.blp"},"gameplay":{"foodLimit":200},"gameplayConstants":{"MISC":{"foodCeiling":"1"}}}`,
	duplicateDocuments[0],
}

// planCounts is what came of the plans for one map folder.
type planCounts struct{ refused, changed, unchanged, apart, broken int }

// sourceMap is a map folder that both trees plan for.
type sourceMap struct {
	name  string
	files map[string][]byte
	// every says that every document is planned for the folder. For the others it is those of routeDocuments.
	every bool
	want  planCounts
}

// sourceMaps is the map folders the plans are compared for. The first two get every document: the fixture as
// it is, which has neither of the optional text files and no minimap, and the fixture with every file a plan
// reads, each text file behind a byte order mark. The others get the documents of routeDocuments: the fixture
// with each optional file alone and with all, with files that hold nothing, under other spellings, and each
// folder that a plan refuses for a file it lacks, cannot read as text or cannot patch, or for a name it has.
func sourceMaps(t *testing.T) []sourceMap {
	t.Helper()
	info, script := fixtureInfo(t), fixtureLua(t)
	misc := "[Misc]\r\nFoodCeiling=100\r\nKeep=1\r\n\r\n[Other]\r\nA=0\r\n"
	skin := "[CustomSkin]\nTest=old\n\n[A]\nOld=1"
	type files = map[string][]byte
	fixture := func(more files) files {
		all := files{"war3map.w3i": info, "war3map.lua": []byte(script)}
		maps.Copy(all, more)
		return all
	}
	returnsValue := swapped(t, script, "RunInitializationTriggers()\r\nend", "RunInitializationTriggers()\r\nreturn 1\r\nend")
	// Of the 17 documents of routeDocuments, four are refused for every folder: the player the fixture lacks, the
	// preview that is not there, the constant set twice, and the two spellings of a section. Two change nothing
	// in any folder: the one that sets nothing, and the sections without keys.
	return []sourceMap{
		// One document puts a number into the script that the trees write apart (the fourth of everyDocuments).
		// Without a minimap, the nine documents with a preview are refused; with every file, they are planned.
		{name: "the fixture", every: true, files: fixture(nil), want: planCounts{refused: 43, changed: 170, unchanged: 12, apart: 1}},
		{name: "every file, each text file behind a byte order mark", every: true,
			want: planCounts{refused: 34, changed: 178, unchanged: 13, apart: 1}, files: fixture(files{
				"war3map.lua": []byte(byteOrderMark + script), "war3mapMisc.txt": []byte(byteOrderMark + misc),
				"war3mapSkin.txt": []byte(byteOrderMark + skin), "war3mapMap.blp": minimapBytes})},
		// The constant the file holds already is no change.
		{name: "every file", want: planCounts{refused: 4, changed: 10, unchanged: 3}, files: fixture(files{
			"war3mapMisc.txt": []byte(misc), "war3mapSkin.txt": []byte(skin), "war3mapMap.blp": minimapBytes})},
		// Without a minimap, the five documents with a preview are refused as well.
		{name: "with war3mapMisc.txt alone", want: planCounts{refused: 9, changed: 5, unchanged: 3},
			files: fixture(files{"war3mapMisc.txt": []byte(misc)})},
		{name: "with war3mapSkin.txt alone", want: planCounts{refused: 9, changed: 6, unchanged: 2},
			files: fixture(files{"war3mapSkin.txt": []byte(skin)})},
		{name: "with the minimap alone", want: planCounts{refused: 4, changed: 11, unchanged: 2},
			files: fixture(files{"war3mapMap.blp": minimapBytes})},
		{name: "with text files that hold nothing", want: planCounts{refused: 9, changed: 6, unchanged: 2},
			files: fixture(files{"war3mapMisc.txt": {}, "war3mapSkin.txt": {}})},
		{name: "every file under another spelling", want: planCounts{refused: 4, changed: 10, unchanged: 3}, files: files{
			"WAR3MAP.W3I": info, "War3Map.Lua": []byte(script), "WAR3MAPMISC.TXT": []byte(misc),
			"war3mapskin.txt": []byte(skin), "WAR3MAPMAP.BLP": minimapBytes}},
		// Only the two text files can be planned for a folder without a file.
		{name: "an empty folder", want: planCounts{refused: 13, changed: 2, unchanged: 2}},
		// The author and the loading screen need the map info alone; the force's name needs the script too.
		{name: "without the script", want: planCounts{refused: 12, changed: 3, unchanged: 2},
			files: files{"war3map.w3i": info, "war3mapMap.blp": minimapBytes}},
		// A preview alone needs the script and the minimap, and no map info.
		{name: "without the map info", want: planCounts{refused: 10, changed: 5, unchanged: 2},
			files: files{"war3map.lua": []byte(script), "war3mapMap.blp": minimapBytes}},
		{name: "a script that is not UTF-8", want: planCounts{refused: 12, changed: 3, unchanged: 2}, files: fixture(files{
			"war3map.lua": {0x66, 0xff, 0x66}, "war3mapMap.blp": minimapBytes})},
		{name: "text files that are not UTF-8", want: planCounts{refused: 12, changed: 3, unchanged: 2}, files: fixture(files{
			"war3mapMisc.txt": {0xc3}, "war3mapSkin.txt": []byte(byteOrderMark + "[A]\n\xff")})},
		// The three documents that set the map's name are refused by the script, after the map info was patched.
		{name: "a script without SetMapName", want: planCounts{refused: 7, changed: 8, unchanged: 2}, files: fixture(files{
			"war3map.lua": []byte(swapped(t, script, "SetMapName(", "Other(")), "war3mapMap.blp": minimapBytes})},
		// The five documents with a preview are left out: the other tree writes the minimap call after the return.
		{name: "a script whose main() returns a value", want: planCounts{refused: 4, changed: 6, unchanged: 2, broken: 5},
			files: fixture(files{"war3map.lua": []byte(returnsValue), "war3mapMap.blp": minimapBytes})},
		{name: "bytes that are no map info", want: planCounts{refused: 10, changed: 5, unchanged: 2}, files: fixture(files{
			"war3map.w3i": []byte("not a map info"), "war3mapMap.blp": minimapBytes})},
		// The five documents with a preview are refused for the name the map has already, of either kind of picture.
		{name: "with the name the minimap is kept under", want: planCounts{refused: 9, changed: 6, unchanged: 2},
			files: fixture(files{"war3mapMap.blp": minimapBytes, "war3mapminimap.blp": {1}})},
		{name: "with the name a TGA preview takes", want: planCounts{refused: 9, changed: 6, unchanged: 2},
			files: fixture(files{"war3mapMap.blp": minimapBytes, "War3mapMap.TGA": {1}})},
	}
}

// onDisk writes the map folder into a temporary folder and returns its path.
func (m sourceMap) onDisk(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for name, data := range m.files {
		testkit.WriteFile(t, dir, name, data)
	}
	return dir
}

// held is the bytes of the file the map folder has under name, in any letter case.
func (m sourceMap) held(name string) []byte {
	for spelled, data := range m.files {
		if mapdir.Key(spelled) == mapdir.Key(name) {
			return data
		}
	}
	return nil
}

// changeTo is the change a plan makes to the file under name, in any letter case.
func changeTo(changes []mapdir.Change, name string) (mapdir.Change, bool) {
	for _, change := range changes {
		if mapdir.Key(change.Name) == mapdir.Key(name) {
			return change, true
		}
	}
	return mapdir.Change{}, false
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
		if counted != source.want {
			t.Errorf("%s: the plans came to %+v, want %+v", source.name, counted, source.want)
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
