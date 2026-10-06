package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mdlsvensson/moonwell/next/internal/oracle"
	"github.com/mdlsvensson/moonwell/next/internal/testkit"
)

// ---- the runs of natives ----

// The scripts of an export, each by its path from the folder of the export, as a run names the one that the
// system cannot give.
const (
	commonOfAnExport   = "war3.w3mod/scripts/common.j"
	blizzardOfAnExport = "war3.w3mod/scripts/blizzard.j"
)

// nativesOf is the line of the mode natives for an export with these two scripts, and for a version. The export
// is a folder beside the checkouts.
func nativesOf(common, blizzard, version string) lineOfARun {
	return func(t testing.TB, outside string) []string {
		testkit.WriteFile(t, outside, "export/"+commonOfAnExport, []byte(common))
		testkit.WriteFile(t, outside, "export/"+blizzardOfAnExport, []byte(blizzard))
		return []string{"natives", filepath.Join(outside, "export"), version}
	}
}

// ofTheMiniatures is the line of the mode natives for the two miniature scripts.
var ofTheMiniatures = nativesOf(miniCommon, miniBlizzard, "9.9.9")

// withoutBlizzard is the line of the mode natives for an export that has the miniature common.j, and no
// blizzard.j.
func withoutBlizzard(t testing.TB, outside string) []string {
	testkit.WriteFile(t, outside, "export/"+commonOfAnExport, []byte(miniCommon))
	return []string{"natives", filepath.Join(outside, "export"), "9.9.9"}
}

// luaExtras lays Lua extras with this text, and a data folder that holds nothing.
func luaExtras(text string) func(checkout) {
	return func(c checkout) {
		c.write(extrasPath, text)
		c.folder("data")
	}
}

// functionsOfTheExtras is the text of Lua extras whose functions are these, each written as a JSON object.
func functionsOfTheExtras(functions ...string) string {
	return `{"functions": [` + strings.Join(functions, ", ") + `], "globals": ["print"], "removed": ["io"]}`
}

// keyNotRead is what this tree says as it refuses Lua extras for a key that it does not read: what has the key,
// the key, and the keys that it may have.
func keyNotRead(holder, key, keys string) string {
	return "error: tools/natives/lua-extras.json: " + holder + " has the key \"" + key +
		"\", which the generator does not read. Its keys are " + keys + ": take the key out, or rename it.\n"
}

// keyLeftOut is what this tree says as it refuses Lua extras for a function without one of its keys: how the
// function is named, and the key.
func keyLeftOut(function, key string) string {
	return "error: tools/natives/lua-extras.json: " + function + " has no \"" + key +
		"\". Its keys are name, params and returns: write all of them.\n"
}

// keyOfAParameterLeftOut is what this tree says as it refuses Lua extras for a parameter without one of its
// keys: how the function is named, and the key.
func keyOfAParameterLeftOut(function, key string) string {
	return "error: tools/natives/lua-extras.json: a parameter of " + function + " has no \"" + key +
		"\". Its keys are name and type: write all of them.\n"
}

// endsTooSoon is what a run of EndOfJSON carries: the one place of standard error that the two trees write
// apart, each in the words of the reader it gives the Lua extras to.
var endsTooSoon = map[string][]place{standardError: {{"unexpected end of JSON input", "unexpected EOF"}}}

// inTheNativesWritten is what a run of an accepted difference in the natives carries: the one place of
// data/natives.json that the two trees write apart, as the other tree writes it and as this tree must.
func inTheNativesWritten(other, this literal) map[string][]place {
	return map[string][]place{nativesPath: {{other, this}}}
}

// nativesRuns is the runs of natives: the miniature scripts and Lua extras of natives_test.go and extras_test.go,
// into an empty data folder, over the natives of another version, and with the committed Lua extras; extras
// without a key, with empty lists and a list that is null, with the keys of the file in another order, and with
// a key twice, of which both trees take the last value, also where a value before it is of another type; scripts
// with carriage returns and a byte order mark at the start, which both trees pass over, the other tree as white
// space before the first word and this tree as it decodes the file; scripts that declare nothing; one byte that
// is no UTF-8, in a comment, in a text and in a line that is no declaration; a version that is empty, and one
// that a JSON text escapes; a run in a folder below the checkout; a name declared twice, in every pair of places
// that the tests of natives_test.go name, and two such names at once; a line of either script that is no
// declaration, a function and a globals block that never end, and a script that does not parse beside extras
// that are no JSON; a wrong count of arguments; extras that are cut short inside a text and after a colon, that
// hold nothing, that are no JSON, that start with a byte order mark, that are a list and null, and that have
// something after their object; a function of the extras that is no object; the runs of the classes, which the
// header of oracle_test.go names; seeded changes of the miniature common.j; and a folder that is no checkout.
func nativesRuns() []oracleRun {
	const whole = `{"name": "Whole", "params": [{"name": "id", "type": "string"}], "returns": "integer"}`
	withBoth := func(text string) string { return mark + strings.ReplaceAll(text, "\n", "\r\n") }
	with := func(blizzard string) lineOfARun { return nativesOf(miniCommon, blizzard, "9.9.9") }
	runs := []oracleRun{
		{name: "the miniature scripts and extras, into an empty data folder", lay: luaExtras(miniExtrasText),
			line: ofTheMiniatures},
		{name: "the miniature scripts, over the natives of another version", line: ofTheMiniatures,
			lay: func(c checkout) {
				luaExtras(miniExtrasText)(c)
				c.write(nativesPath, "{\n  \"gameVersion\": \"1.0.0\"\n}\n")
			}},
		{name: "the miniature scripts with the committed extras", line: ofTheMiniatures,
			lay: func(c checkout) {
				c.carry(extrasPath)
				c.folder("data")
			}},
		{name: "extras without a key", lay: luaExtras("{}"), line: ofTheMiniatures},
		{name: "extras with empty lists, a list that is null, and a Lua function without a parameter",
			lay: luaExtras(`{"functions": [{"name": "Bare", "params": [], "returns": "nothing"}], ` +
				`"globals": [], "removed": null}`), line: ofTheMiniatures},
		{name: "extras with a key twice", line: ofTheMiniatures,
			lay: luaExtras(`{"globals": ["first"], "removed": ["io"], "globals": ["second", "print"]}`)},
		{name: "a function of the extras with a key twice, the first value of another type", line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(whole,
				`{"name": 1, "params": "none", "returns": "nothing", "name": "Twice", "params": []}`))},
		{name: "extras with the keys of the file in another order", line: ofTheMiniatures,
			lay: luaExtras(`{"removed": ["io"], "globals": ["print", "math"], "functions": [` + whole + `]}`)},
		{name: "scripts with carriage returns, and a byte order mark at the start of each",
			lay: luaExtras(miniExtrasText), line: nativesOf(withBoth(miniCommon), withBoth(miniBlizzard), "9.9.9")},
		{name: "scripts that declare nothing", lay: luaExtras(miniExtrasText),
			line: nativesOf("", "// nothing\n\n", "9.9.9")},
		{name: "one byte that is no UTF-8, in a comment and in a text of a script", lay: luaExtras(miniExtrasText),
			line: nativesOf("// caf\xE9\n"+miniCommon+"globals\n    string odd = \"\xFF\" // \xFE\nendglobals\n",
				miniBlizzard, "9.9.9")},
		{name: "one byte that is no UTF-8, in a line that is no declaration", lay: luaExtras(miniExtrasText),
			line: with("native Odd\xFF takes nothing returns nothing\n")},
		{name: "a version that is empty", lay: luaExtras(miniExtrasText),
			line: nativesOf(miniCommon, miniBlizzard, "")},
		{name: "a version that a JSON text escapes", lay: luaExtras(miniExtrasText),
			line: nativesOf(miniCommon, miniBlizzard, "1.0 \"beta\"\\\t<&>\x01\n\xC3\xA4")},
		{name: "natives in a folder below the checkout", below: "tools/gen/slk", lay: luaExtras(miniExtrasText),
			line: ofTheMiniatures},
		{name: "a name declared in both scripts", lay: luaExtras(miniExtrasText), line: with(miniCommon)},
		{name: "a Lua global named like a function", lay: luaExtras(`{"globals": ["CreateThing"]}`),
			line: ofTheMiniatures},
		{name: "a name both provided and removed", line: ofTheMiniatures,
			lay: luaExtras(`{"functions": [], "globals": ["print", "io"], "removed": ["io"]}`)},
		{name: "a Lua function named like a function of blizzard.j", line: ofTheMiniatures,
			lay: luaExtras(`{"functions": [{"name": "HelperBJ", "params": [], "returns": "nothing"}]}`)},
		{name: "a Lua global named like a global", lay: luaExtras(`{"globals": ["MAX_THINGS"]}`),
			line: ofTheMiniatures},
		{name: "a removed global named like a type", lay: luaExtras(`{"removed": ["widget"]}`), line: ofTheMiniatures},
		{name: "a global in both scripts", lay: luaExtras(miniExtrasText),
			line: with("globals\n    integer counts\nendglobals\n")},
		{name: "a type named like a global", lay: luaExtras(miniExtrasText),
			line: with("type counts extends handle\n")},
		{name: "a type and a global, each declared twice", lay: luaExtras(miniExtrasText),
			line: with("type agent extends handle\nglobals\n    real counts\nendglobals\n")},
		{name: "a function and a Lua global, each declared twice", line: with(miniCommon),
			lay: luaExtras(`{"globals": ["print", "print"]}`)},
		{name: "a native twice in common.j", lay: luaExtras(miniExtrasText),
			line: nativesOf(miniCommon+"native DoNothing takes nothing returns nothing\n", miniBlizzard, "9.9.9")},
		{name: "a global twice in blizzard.j", lay: luaExtras(miniExtrasText),
			line: with("globals\n    integer twice\n    real twice\nendglobals\n")},
		{name: "a type in both scripts", lay: luaExtras(miniExtrasText), line: with("type unit extends handle\n")},
		{name: "a Lua function twice", lay: luaExtras(functionsOfTheExtras(whole, whole)), line: ofTheMiniatures},
		{name: "a Lua function named like a function of common.j", line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(`{"name": "GetThing", "params": [], "returns": "unit"}`))},
		{name: "a Lua function named like a global of blizzard.j", line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(`{"name": "bj_ANGLE", "params": [], "returns": "real"}`))},
		{name: "a Lua global named like a type", lay: luaExtras(`{"globals": ["widget"]}`), line: ofTheMiniatures},
		{name: "a Lua global twice", lay: luaExtras(`{"globals": ["print", "math", "print"]}`), line: ofTheMiniatures},
		{name: "a removed global named like a function of blizzard.j", lay: luaExtras(`{"removed": ["ConstantBJ"]}`),
			line: ofTheMiniatures},
		{name: "a line of common.j that is no declaration", lay: luaExtras(miniExtrasText),
			line: nativesOf("type agent extends handle\n\nnative Broken takes returns nothing\n", miniBlizzard,
				"9.9.9")},
		{name: "a line of blizzard.j that is no declaration", lay: luaExtras(miniExtrasText),
			line: with("globals\n    real = 0.0 // \"quoted\"\tand a tab\nendglobals\n")},
		{name: "a function of blizzard.j that never ends", lay: luaExtras(miniExtrasText),
			line: with("\n\nfunction Open takes nothing returns nothing\n    call DoNothing(null)\n")},
		{name: "a globals block that never ends", lay: luaExtras(miniExtrasText),
			line: nativesOf("globals\n    integer open\n", miniBlizzard, "9.9.9")},
		{name: "a script that does not parse, and extras that are no JSON", lay: luaExtras("not JSON\n"),
			line: with("not a declaration\n")},
		{name: "natives alone", lay: luaExtras(miniExtrasText), line: words("natives")},
		{name: "natives without a version", lay: luaExtras(miniExtrasText), line: words("natives", "export")},
		{name: "natives with an argument too many", lay: luaExtras(miniExtrasText),
			line: words("natives", "export", "9.9.9", "more")},
		{name: "extras that are cut short", lay: luaExtras(`{"functions": [{ "name": "Fo`), line: ofTheMiniatures},
		{name: "extras that are cut short after a colon", lay: luaExtras(`{"functions":`), line: ofTheMiniatures},
		{name: "extras that hold nothing", lay: luaExtras(""), line: ofTheMiniatures},
		{name: "extras that are no JSON", lay: luaExtras("not JSON\n"), line: ofTheMiniatures},
		{name: "extras that start with a byte order mark", lay: luaExtras(mark + miniExtrasText),
			line: ofTheMiniatures},
		{name: "extras that are a list", lay: luaExtras("[]"), line: ofTheMiniatures},
		{name: "extras that are null", lay: luaExtras("null"), line: ofTheMiniatures},
		{name: "extras with something after the object", lay: luaExtras(miniExtrasText + "\n{}\n"),
			line: ofTheMiniatures},
		{name: "a function of the extras that is no object", lay: luaExtras(functionsOfTheExtras(whole, `"FourCC"`)),
			line: ofTheMiniatures},

		{name: "no extras, and no folder of theirs", class: "FromCheckout", lay: noList, line: ofTheMiniatures},
		{name: "no extras in their folder", class: "FromCheckout", line: ofTheMiniatures,
			lay: func(c checkout) {
				noList(c)
				c.folder("tools/natives")
			}},
		{name: "a folder at the place of the extras", class: "FromCheckout", line: ofTheMiniatures,
			lay: func(c checkout) {
				noList(c)
				c.folder(extrasPath)
			}},
		{name: "no data folder for the natives", class: "FromCheckout", line: ofTheMiniatures,
			lay: func(c checkout) { c.write(extrasPath, miniExtrasText) }},
		{name: "a folder at the place of the natives", class: "FromCheckout", line: ofTheMiniatures,
			lay: func(c checkout) {
				luaExtras(miniExtrasText)(c)
				c.write(nativesPath+"/held.txt", "held\n")
			}},

		{name: "no blizzard.j in the export", class: "AsGiven", cannotGive: &named{1, blizzardOfAnExport},
			lay: luaExtras(miniExtrasText), line: withoutBlizzard},
		{name: "no common.j in the export", class: "AsGiven", cannotGive: &named{1, commonOfAnExport},
			lay: luaExtras(miniExtrasText),
			line: func(t testing.TB, outside string) []string {
				testkit.WriteFile(t, outside, "export/"+blizzardOfAnExport, []byte(miniBlizzard))
				return []string{"natives", filepath.Join(outside, "export"), "9.9.9"}
			}},
		{name: "an export that is not there", class: "AsGiven", cannotGive: &named{1, commonOfAnExport},
			lay: luaExtras(miniExtrasText),
			line: func(_ testing.TB, outside string) []string {
				return []string{"natives", filepath.Join(outside, "no-export"), "9.9.9"}
			}},
		{name: "an export that is not there, and no extras", class: "AsGiven",
			cannotGive: &named{1, commonOfAnExport},
			line: func(_ testing.TB, outside string) []string {
				return []string{"natives", filepath.Join(outside, "no-export"), "9.9.9"}
			}},
		{name: "a folder at the place of common.j", class: "AsGiven", cannotGive: &named{1, commonOfAnExport},
			lay: luaExtras(miniExtrasText),
			line: func(t testing.TB, outside string) []string {
				testkit.WriteFile(t, outside, "export/"+commonOfAnExport+"/held.txt", []byte("held\n"))
				testkit.WriteFile(t, outside, "export/"+blizzardOfAnExport, []byte(miniBlizzard))
				return []string{"natives", filepath.Join(outside, "export"), "9.9.9"}
			}},

		{name: "extras with a key that the file has not", class: "UnknownKey", line: ofTheMiniatures,
			lay:     luaExtras(`{"functions": [], "globals": ["print"], "removed": [], "comment": "by hand"}`),
			refusal: keyNotRead("the file", "comment", "functions, globals and removed")},
		{name: "extras with a key in other letters", class: "UnknownKey", line: ofTheMiniatures,
			lay:     luaExtras(`{"Globals": ["print"]}`),
			refusal: keyNotRead("the file", "Globals", "functions, globals and removed")},
		{name: "a function of the extras with a key that a function has not", class: "UnknownKey",
			line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(
				`{"name": "Noted", "params": [], "returns": "nothing", "note": "kept"}`, whole)),
			refusal: keyNotRead("the function Noted", "note", "name, params and returns")},
		{name: "a parameter of the extras with a key that a parameter has not", class: "UnknownKey",
			line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(whole, `{"name": "Noted", "returns": "nothing", `+
				`"params": [{"name": "id", "type": "string", "optional": true}]}`)),
			refusal: keyNotRead("a parameter of the function Noted", "optional", "name and type")},

		{name: "a function of the extras without params", class: "LacksAKey", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(whole, `{"name": "Bare", "returns": "nothing"}`)),
			refusal: keyLeftOut("the function Bare", "params")},
		{name: "a function of the extras without returns", class: "LacksAKey", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(`{"name": "Bare", "params": []}`, whole)),
			refusal: keyLeftOut("the function Bare", "returns")},
		{name: "a function of the extras with its name alone", class: "LacksAKey", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(`{"name": "Bare"}`)),
			refusal: keyLeftOut("the function Bare", "params")},
		{name: "a function of the extras with null for its params", class: "LacksAKey", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(whole, `{"name": "Bare", "params": null, "returns": "nothing"}`)),
			refusal: keyLeftOut("the function Bare", "params")},
		{name: "a function of the extras with null for its returns", class: "LacksAKey", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(`{"name": "Bare", "params": [], "returns": null}`, whole)),
			refusal: keyLeftOut("the function Bare", "returns")},
		{name: "a parameter of the extras without its name", class: "LacksAKey", line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(whole,
				`{"name": "Bare", "params": [{"name": "id", "type": "any"}, {"type": "any"}], "returns": "nothing"}`)),
			refusal: keyOfAParameterLeftOut("the function Bare", "name")},
		{name: "a parameter of the extras without its type", class: "LacksAKey", line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(
				`{"name": "Bare", "params": [{"name": "id"}], "returns": "nothing"}`)),
			refusal: keyOfAParameterLeftOut("the function Bare", "type")},
		{name: "a parameter of the extras with null for its type", class: "LacksAKey", line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(
				`{"name": "Bare", "params": [{"name": "id", "type": null}], "returns": "nothing"}`)),
			refusal: keyOfAParameterLeftOut("the function Bare", "type")},

		{name: "a function of the extras without a name", class: "NoName", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(whole, `{"params": [], "returns": "nothing"}`)),
			refusal: keyLeftOut("function 2 of the list", "name")},
		{name: "a function of the extras that has nothing", class: "NoName", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(`{}`)),
			refusal: keyLeftOut("function 1 of the list", "name")},
		{name: "a function of the extras with null for its name", class: "NoName", line: ofTheMiniatures,
			lay:     luaExtras(functionsOfTheExtras(whole, `{"name": null, "params": [], "returns": "nothing"}`)),
			refusal: keyLeftOut("function 2 of the list", "name")},

		{name: "extras that are cut short after a bracket", class: "EndOfJSON", line: ofTheMiniatures,
			lay: luaExtras(`{"functions": [`), apart: endsTooSoon},
		{name: "extras that are cut short after a comma", class: "EndOfJSON", line: ofTheMiniatures,
			lay: luaExtras(`{"functions": [], `), apart: endsTooSoon},
		{name: "extras that are cut short after a whole value", class: "EndOfJSON", line: ofTheMiniatures,
			lay: luaExtras(`{"functions": [], "globals": ["print"]`), apart: endsTooSoon},

		{name: "a function of the extras with its keys in another order", class: "KeyOrder", line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(whole,
				`{"returns": "integer", "params": [{"name": "id", "type": "string"}], "name": "Odd"}`)),
			apart: inTheNativesWritten(
				"      \"returns\": \"integer\",\n      \"params\": [\n        {\n          \"name\": \"id\",\n"+
					"          \"type\": \"string\"\n        }\n      ],\n      \"name\": \"Odd\",\n",
				"      \"name\": \"Odd\",\n      \"params\": [\n        {\n          \"name\": \"id\",\n"+
					"          \"type\": \"string\"\n        }\n      ],\n      \"returns\": \"integer\",\n")},
		{name: "a parameter of the extras with its keys in another order", class: "KeyOrder", line: ofTheMiniatures,
			lay: luaExtras(functionsOfTheExtras(whole, `{"name": "Odd", "params": [`+
				`{"type": "table", "name": "odd"}, {"name": "second", "type": "any"}], "returns": "integer"}`)),
			apart: inTheNativesWritten(
				"          \"type\": \"table\",\n          \"name\": \"odd\"\n",
				"          \"name\": \"odd\",\n          \"type\": \"table\"\n")},
	}
	runs = append(runs, changedScripts()...)
	runs = append(runs, noCheckoutRuns("natives", ofTheMiniatures)...)
	return append(runs, noCheckoutRuns("natives alone", words("natives"))...)
}

// changedScripts is the miniature common.j after seeded changes, eight times, beside the miniature blizzard.j:
// one to three changes of each, a line cut, a line doubled, or a character of ASCII white space put in. A change
// is named by the seed and its index, which make it again.
func changedScripts() []oracleRun {
	const seed, changes = 2026_10_06, 8
	var runs []oracleRun
	for index := uint64(1); index <= changes; index++ {
		runs = append(runs, oracleRun{
			name: fmt.Sprintf("the miniature common.j, changed: seed %d, change %d", seed, index),
			lay:  luaExtras(miniExtrasText),
			line: nativesOf(oracle.Changed(miniCommon, seed, index), miniBlizzard, "9.9.9"),
		})
	}
	return runs
}

// theGamesScripts is the runs on the game's two scripts, with the version that the committed natives state and
// the committed Lua extras: what each tree writes is the committed data/natives.json. The second run starts this
// tree's generator as a program too.
func theGamesScripts(t testing.TB, export testkit.Export) []oracleRun {
	var committed struct {
		GameVersion string `json:"gameVersion"`
	}
	if err := json.Unmarshal(realFile(t, nativesPath), &committed); err != nil || committed.GameVersion == "" {
		t.Fatalf("%s states no version of the game: %v", nativesPath, err)
	}
	run := oracleRun{
		name:        "the game's two scripts, with the version of the committed natives",
		line:        words("natives", export.Path(), committed.GameVersion),
		asCommitted: []string{nativesPath},
		lay: func(c checkout) {
			c.carry(extrasPath)
			c.folder("data")
		},
	}
	asPrograms := run
	asPrograms.name, asPrograms.asPrograms = "as programs: "+run.name, true
	return []oracleRun{run, asPrograms}
}
