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
	var parsed extras
	if err := readHandWritten(checkout, extrasPath, &parsed); err != nil {
		return extras{}, err
	}
	return parsed, checkExtras(parsed)
}

func checkExtras(parsed extras) error {
	for i, function := range parsed.Functions {
		location := "functions." + strconv.Itoa(i)
		switch {
		case function.Name == "":
			return errExtraLacks(location, "name")
		case function.Params == nil:
			return errExtraLacks(location, "params")
		case function.Returns == "":
			return errExtraLacks(location, "returns")
		}
		for j, param := range function.Params {
			if param.Name == "" || param.Type == "" {
				return errParamLacks(location + ".params." + strconv.Itoa(j))
			}
		}
	}
	if index := slices.Index(parsed.Globals, ""); index >= 0 {
		return errEmptyGlobal("globals." + strconv.Itoa(index))
	}
	if index := slices.Index(parsed.Removed, ""); index >= 0 {
		return errEmptyGlobal("removed." + strconv.Itoa(index))
	}
	return nil
}

func errExtraLacks(location, key string) error {
	return errors.New(extrasPath + ": " + location + ` has no "` + key + `". Every function has a "name", its ` +
		`"params" (a list, [] for a function that takes nothing) and what it "returns".`)
}

func errParamLacks(location string) error {
	return errors.New(extrasPath + ": " + location + ` lacks its "name" or its "type".`)
}

func errEmptyGlobal(location string) error {
	return errors.New(extrasPath + ": " + location + " is empty or null.")
}
