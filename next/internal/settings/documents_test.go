package settings

import (
	"slices"
)

// The settings documents that the recorded test plans, and patches every script for. Each is a settings block as
// pkl prints it.

// Documents for the map info: its texts, a player, a force and the environment, mostly one at a time.
var mapInfoDocuments = []string{
	`{"loadingScreen":{"model":"Loading.mdx","text":"Text","title":"Title","subtitle":"Subtitle"}}`,
	`{"environment":{"soundEnvironment":"Dungeon","waterColor":[255,0,0,255],` +
		`"fog":{"enabled":true,"color":[255,0,0,255]}}}`,
	`{"info":{"name":"M` + "\xc3\xb8\xc3\xb8" + `nwell","author":"","description":"TRIGSTR_001"},` +
		`"loadingScreen":{"title":"Changed","background":7}}`,
	`{"info":{"name":"TRIGSTR_001","author":"Author","description":"Description"},` +
		`"loadingScreen":{"title":"Title","background":0}}`,
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

// Documents for the two text files.
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

// Documents for the plan: the settings of each file alone and together, what sets nothing, and a preview of each
// kind.
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

// Documents for the script: each setting with a counterpart in war3map.lua, alone and beside others.
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
		"environment":{"soundEnvironment":"","waterColor":[0,0,0,0],
			"fog":{"enabled":true,"start":0,"color":[255,0,0,0]}}}`,
	`{"environment":{"fog":{"enabled":false,"density":0}}}`,
}

// Documents at the edges of what the schema takes: a null, an empty text, a zero, the least and the most of each
// number, each controller, each race, and each flag of a force turned off.
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
	flags := []string{"allied", "alliedVictory", "sharedVision", "sharedControl", "sharedAdvancedControl"}
	for _, flag := range flags {
		documents = append(documents, `{"forces":{"0":{"`+flag+`":false}}}`)
	}
	return documents
}

// Documents that each set every setting that some map info can take, and documents with more than one thing to
// refuse.
var everyDocuments = []string{
	// Every setting there is, for the player and the force that every map with players and forces has.
	`{"info":{"name":"Every \"setting\"","author":"An author","description":"One|nTwo","recommendedPlayers":"2-4",
			"preview":"preview.tga"},
		"loadingScreen":{"background":3,"model":"Loading\\Screen.mdx","text":"Text","title":"Title",
			"subtitle":"Subtitle"},
		"players":{"0":{"name":"Hero","controller":"computer","race":"nightelf","fixedStart":false,"x":-512.25,
			"y":1024.5}},
		"forces":{"0":{"name":"The Alliance","allied":true,"alliedVictory":false,"sharedVision":true,
			"sharedControl":false,"sharedAdvancedControl":true}},
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

// documents is every document above.
func documents() []string {
	return slices.Concat(mapInfoDocuments, textDocuments, planDocuments, luaDocuments, optionDocuments(),
		everyDocuments)
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
				documents = append(documents, `{"gameplay":{`+typed+`},"gameplayConstants":{`+
					`"First":{"FoodCeiling":"1"},"`+section+`":{`+keys+`},"Last":{}}}`)
			}
		}
	}
	return documents
}

// The documents that are refused for names that differ only in letter case.
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
	`{"gameplay":{"foodLimit":1},"gameplayConstants":{"Misc":{"FoodCeiling":"2"}},` +
		`"gameInterface":{"A":{},"a":{}}}`,
	`{"gameInterface":{"A":{},"a":{}},"gameplayConstants":{"Misc":{"Key":"1","KEY":"2"}}}`,
}

// The documents with a zero below 0: set as one, or what the map info keeps of a number too small for it.
var zeroDocuments = []string{
	`{"players":{"0":{"x":-0.0,"y":0}}}`,
	`{"players":{"0":{"x":1e-46,"y":-1e-46}}}`,
	`{"environment":{"fog":{"enabled":true,"start":-0.0,"end":-0.0,"density":-0.0,"color":[1,2,3,4]}}}`,
}

// The documents with a number below 0.000001 in size, which a script takes in plain decimal, beside the one among
// everyDocuments; and after them documents with such a number that no script takes: of a fog that is not shown,
// and of a player whose position is not set.
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
	`{"info":{"name":"N","preview":"preview.blp"},"gameplay":{"foodLimit":200},` +
		`"gameplayConstants":{"MISC":{"foodCeiling":"1"}}}`,
	duplicateDocuments[0],
}
