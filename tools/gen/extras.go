package main

import (
	"errors"
	"slices"
	"strconv"

	"github.com/mdlsvensson/moonwell/internal/script"
)

const extrasPath = "tools/natives/lua-extras.json"

type extras struct {
	Functions []extraFunction `json:"functions"`
	Globals   []string        `json:"globals"`
	Removed   []string        `json:"removed"`
}

type extraFunction struct {
	Name    string               `json:"name"`
	Params  []script.NativeParam `json:"params"`
	Returns string               `json:"returns"`
}

func readExtras(checkout string) (extras, error) {
	var lua extras
	if err := readHandWritten(checkout, extrasPath, &lua); err != nil {
		return extras{}, err
	}
	return lua, checkExtras(lua)
}

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

func errExtraLacks(place, key string) error {
	return errors.New(extrasPath + ": " + place + ` has no "` + key + `". Every function has a "name", its ` +
		`"params" (a list, [] for a function that takes nothing) and what it "returns".`)
}

func errParamLacks(place string) error {
	return errors.New(extrasPath + ": " + place + ` lacks its "name" or its "type".`)
}

func errEmptyGlobal(place string) error {
	return errors.New(extrasPath + ": " + place + " is empty or null.")
}
