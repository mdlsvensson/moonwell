// Package natives holds the names and signatures of the game's script API, from common.j and blizzard.j.
package natives

import (
	"encoding/json"
	"sync"

	moonwell "github.com/mdlsvensson/moonwell"
)

// Param is one parameter of a function.
type Param struct {
	Name string `json:"name"`
	// Type is a JASS type (integer, real, unit, ...), or a Lua type for Lua-only functions (any, table).
	Type string `json:"type"`
}

// Function is a native or a function of blizzard.j.
type Function struct {
	Name     string  `json:"name"`
	Source   string  `json:"source"` // common.j, blizzard.j or lua
	Constant bool    `json:"constant"`
	Params   []Param `json:"params"`
	// Returns is "nothing" when the function returns no value.
	Returns string `json:"returns"`
}

// Global is a global variable or constant.
type Global struct {
	Name     string `json:"name"`
	Source   string `json:"source"` // common.j or blizzard.j
	Type     string `json:"type"`
	Constant bool   `json:"constant"`
	Array    bool   `json:"array"`
}

// Type is a handle type and the type it extends.
type Type struct {
	Name    string `json:"name"`
	Extends string `json:"extends"`
}

// Natives is natives.json.
type Natives struct {
	GameVersion string     `json:"gameVersion"`
	Types       []Type     `json:"types"`
	Functions   []Function `json:"functions"`
	Globals     []Global   `json:"globals"`
	// Lua lists the Lua standard-library globals the game provides, and those it removes.
	Lua struct {
		Globals []string `json:"globals"`
		Removed []string `json:"removed"`
	} `json:"lua"`
}

// Load returns the embedded natives, parsed once per run.
var Load = sync.OnceValue(func() *Natives {
	var natives Natives
	if err := json.Unmarshal(moonwell.Natives, &natives); err != nil {
		panic("the embedded natives.json does not parse: " + err.Error())
	}
	return &natives
})
