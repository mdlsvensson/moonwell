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
// three empty, and so null for an entry of a list is refused as well.
//
// What lacks something is named by its place in the file, as the refusal of a value of the wrong kind names a
// place (errNoJSON): the keys on the way to it with dots between them, and an entry of a list by its number,
// counted from 0. So functions.1 is the second function, in both.
func checkExtras(lua extras) error {
	for i, function := range lua.Functions {
		place := "functions." + strconv.Itoa(i)
		switch {
		case function.Name == "":
			return errExtraLacks(place, "name")
		case function.Params == nil:
			return errExtraLacks(place, "params")
		case function.Returns == "":
			return errExtraLacks(place, "returns")
		}
		for j, param := range function.Params {
			if param.Name == "" || param.Type == "" {
				return errParamLacks(place + ".params." + strconv.Itoa(j))
			}
		}
	}
	if at := slices.Index(lua.Globals, ""); at >= 0 {
		return errEmptyGlobal("globals." + strconv.Itoa(at))
	}
	if at := slices.Index(lua.Removed, ""); at >= 0 {
		return errEmptyGlobal("removed." + strconv.Itoa(at))
	}
	return nil
}

// ---- errors ----

// errExtraLacks refuses a function of the extras, by its place, that lacks one of its three keys, or has nothing
// under it.
func errExtraLacks(place, key string) error {
	return errors.New(extrasPath + ": " + place + ` has no "` + key + `". Every function has a "name", its ` +
		`"params" (a list, [] for a function that takes nothing) and what it "returns".`)
}

// errParamLacks refuses a parameter of a function of the extras, by its place, that lacks one of its two keys.
func errParamLacks(place string) error {
	return errors.New(extrasPath + ": " + place + ` lacks its "name" or its "type".`)
}

// errEmptyGlobal refuses an entry of a list of globals, by its place, that has no name.
func errEmptyGlobal(place string) error {
	return errors.New(extrasPath + ": " + place + " is empty or null.")
}
