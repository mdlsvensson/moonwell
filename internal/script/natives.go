package script

import (
	"encoding/json"
	"sync"

	moonwell "github.com/mdlsvensson/moonwell"
)

type NativeParam struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

type NativeFunction struct {
	Name     string        `json:"name"`
	Source   string        `json:"source"`
	Constant bool          `json:"constant"`
	Params   []NativeParam `json:"params"`
	Returns  string        `json:"returns"`
}

type NativeGlobal struct {
	Name     string `json:"name"`
	Source   string `json:"source"`
	Type     string `json:"type"`
	Constant bool   `json:"constant"`
	Array    bool   `json:"array"`
}

type NativeType struct {
	Name    string `json:"name"`
	Extends string `json:"extends"`
}

type Natives struct {
	GameVersion string           `json:"gameVersion"`
	Types       []NativeType     `json:"types"`
	Functions   []NativeFunction `json:"functions"`
	Globals     []NativeGlobal   `json:"globals"`
	Lua         struct {
		Globals []string `json:"globals"`
		Removed []string `json:"removed"`
	} `json:"lua"`
}

func LoadNatives() *Natives { return embeddedNatives() }

var embeddedNatives = sync.OnceValue(func() *Natives { return parseNatives(moonwell.Natives) })

func parseNatives(document []byte) *Natives {
	var parsed Natives
	if err := json.Unmarshal(document, &parsed); err != nil {
		panic("the embedded natives.json does not parse: " + err.Error())
	}
	return &parsed
}
