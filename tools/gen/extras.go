package main

import (
	"errors"
	"slices"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/script"
)

// extrasPath is what the game's Lua adds to the two scripts, written by hand, by its path from the checkout.
const extrasPath = "tools/natives/lua-extras.json"

// extras is tools/natives/lua-extras.json: what the game's Lua adds to the two scripts, and the globals of
// Lua's standard library it provides and removes. A list that the file leaves out is empty.
type extras struct {
	Functions []extraFunction `json:"functions"`
	Globals   []string        `json:"globals"`
	Removed   []string        `json:"removed"`
}

// extraFunction is a function that only the game's Lua has. The type of a parameter, and what the function
// returns, may be a type of Lua.
type extraFunction struct {
	Name    string               `json:"name"`
	Params  []script.NativeParam `json:"params"`
	Returns string               `json:"returns"`
}

// readExtras reads the Lua extras of a checkout, and holds them to what the natives need of them.
func readExtras(checkout string) (extras, error) {
	var lua extras
	if err := readHandWritten(checkout, extrasPath, &lua); err != nil {
		return extras{}, err
	}
	return lua, checkExtras(lua)
}

// checkExtras holds the extras to what an entry of the natives needs: every function has its name, its list of
// parameters, which may be empty, and what it returns; every parameter has its name and its type; and no global
// is an empty name. A key that the file leaves out, null and an empty text are one here: the reading leaves all
// three empty, and so null for an entry of a list is refused as well. A function is named by its place in the
// list, counted from 1.
func checkExtras(lua extras) error {
	for i, function := range lua.Functions {
		called := "function " + strconv.Itoa(i+1)
		switch {
		case function.Name == "":
			return errExtraLacks(called, "name")
		case function.Params == nil:
			return errExtraLacks(called, "params")
		case function.Returns == "":
			return errExtraLacks(called, "returns")
		}
		for _, param := range function.Params {
			if param.Name == "" || param.Type == "" {
				return errParamLacks(called)
			}
		}
	}
	if slices.Contains(lua.Globals, "") {
		return errEmptyGlobal("globals")
	}
	if slices.Contains(lua.Removed, "") {
		return errEmptyGlobal("removed")
	}
	return nil
}

// ---- errors ----

// errExtraLacks refuses a function of the extras that lacks one of its three keys, or has nothing under it.
func errExtraLacks(called, key string) error {
	return errors.New(extrasPath + ": " + called + ` has no "` + key + `". Every function has a "name", its ` +
		`"params" (a list, [] for a function that takes nothing) and what it "returns".`)
}

// errParamLacks refuses a parameter of a function of the extras that lacks one of its two keys.
func errParamLacks(called string) error {
	return errors.New(extrasPath + ": a parameter of " + called + ` lacks its "name" or its "type".`)
}

// errEmptyGlobal refuses a list of globals that has an entry without a name.
func errEmptyGlobal(list string) error {
	return errors.New(extrasPath + `: "` + list + `" has an entry that is empty or null.`)
}
