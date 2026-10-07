package script

import (
	"encoding/json"
	"sync"

	moonwell "github.com/mdlsvensson/moonwell"
)

// NativeParam is one parameter of a function.
type NativeParam struct {
	Name string `json:"name"`
	// Type is a JASS type (integer, real, unit, ...), or a Lua type for a function only Lua has (any, table).
	Type string `json:"type"`
}

// NativeFunction is a native or a function of blizzard.j.
type NativeFunction struct {
	Name     string        `json:"name"`
	Source   string        `json:"source"` // common.j, blizzard.j or lua
	Constant bool          `json:"constant"`
	Params   []NativeParam `json:"params"`
	// Returns is "nothing" when the function returns no value.
	Returns string `json:"returns"`
}

// NativeGlobal is a global variable or constant.
type NativeGlobal struct {
	Name     string `json:"name"`
	Source   string `json:"source"` // common.j or blizzard.j
	Type     string `json:"type"`
	Constant bool   `json:"constant"`
	Array    bool   `json:"array"`
}

// NativeType is a handle type and the type it extends.
type NativeType struct {
	Name    string `json:"name"`
	Extends string `json:"extends"`
}

// Natives is the game's script API, from common.j and blizzard.j: what data/natives.json holds.
type Natives struct {
	GameVersion string           `json:"gameVersion"`
	Types       []NativeType     `json:"types"`
	Functions   []NativeFunction `json:"functions"`
	Globals     []NativeGlobal   `json:"globals"`
	// Lua lists the globals of Lua's standard library that the game provides, and those it removes.
	Lua struct {
		Globals []string `json:"globals"`
		Removed []string `json:"removed"`
	} `json:"lua"`
}

// LoadNatives returns the game's script API from the embedded data/natives.json, parsed once per run. Every call
// returns the same value, which nobody may change.
//
// The file is part of the program, and a test proves that it parses and what it holds. So the function has no
// error to return: a file that does not parse is a bug in Moonwell, and a panic.
func LoadNatives() *Natives { return embeddedNatives() }

var embeddedNatives = sync.OnceValue(func() *Natives { return parseNatives(moonwell.Natives) })

// parseNatives reads a natives.json. A document that does not parse is a panic, which is printed as an internal
// error: the one document it is given is the program's own, so the fault is no failure to report to a user.
func parseNatives(document []byte) *Natives {
	var parsed Natives
	if err := json.Unmarshal(document, &parsed); err != nil {
		panic("the embedded natives.json does not parse: " + err.Error())
	}
	return &parsed
}
