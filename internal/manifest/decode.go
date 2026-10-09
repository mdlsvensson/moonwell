package manifest

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"reflect"
	"strings"

	"github.com/go-viper/mapstructure/v2"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/viper"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

func readSettingsFile(fullPath, displayPath string, defaults map[string]any) (settings *viper.Viper, exists bool, err error) {
	settings = viper.New()
	settings.SetConfigFile(fullPath)
	settings.SetConfigType("toml")
	for key, value := range defaults {
		settings.SetDefault(key, value)
	}
	err = settings.ReadInConfig()
	var parseErr viper.ConfigParseError
	switch {
	case err == nil:
		return settings, true, nil
	case errors.Is(err, fs.ErrNotExist):
		return settings, false, nil
	case errors.As(err, &parseErr):
		return nil, false, errNotTOML(displayPath, parseErr)
	}
	return nil, false, errSettingsUnreadable(displayPath, err)
}

func decodeSettings(settings *viper.Viper, displayPath string, target any) error {
	if err := settings.UnmarshalExact(target, decodeStrictly); err != nil {
		return errNotDecoded(displayPath, err)
	}
	return nil
}

func decodeStrictly(config *mapstructure.DecoderConfig) {
	config.WeaklyTypedInput = false
	config.TagName = "json"
	config.DecodeHook = refuseLossyNumbers
}

func refuseLossyNumbers(from, to reflect.Type, data any) (any, error) {
	if from.Kind() == reflect.Slice && to.Kind() == reflect.Array {
		if length := reflect.ValueOf(data).Len(); length != to.Len() {
			return nil, fmt.Errorf("expected %d values, got %d", to.Len(), length)
		}
		return data, nil
	}
	number, isNumber := asFloat(data)
	if !isNumber || !isWholeNumberKind(to.Kind()) {
		return data, nil
	}
	if number != math.Trunc(number) {
		return nil, fmt.Errorf("expected a whole number, got %v", data)
	}
	if !fitsIn(number, to) {
		return nil, fmt.Errorf("%v is outside the numbers this setting takes", data)
	}
	return data, nil
}

func asFloat(data any) (number float64, isNumber bool) {
	switch value := data.(type) {
	case int64:
		return float64(value), true
	case float64:
		return value, true
	}
	return 0, false
}

func isWholeNumberKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int32, reflect.Int64, reflect.Uint8:
		return true
	}
	return false
}

func fitsIn(number float64, to reflect.Type) bool {
	const largestExactWhole = 1 << 53
	zero := reflect.Zero(to)
	if math.Abs(number) >= largestExactWhole {
		return false
	}
	if zero.CanUint() {
		return number >= 0 && !zero.OverflowUint(uint64(number))
	}
	return !zero.OverflowInt(int64(number))
}

const settingsListHint = "Every setting and its rule is listed under \"Settings\" in Moonwell's README."

func errNotTOML(displayPath string, parseErr viper.ConfigParseError) error {
	diagErr := &diag.Error{
		Msg:   "This file is not valid TOML: " + withoutPrefixes(parseErr.Error(), "While parsing config: ", "toml: "),
		File:  displayPath,
		Hint:  "Fix the line, or compare the file with the examples under \"Settings\" in Moonwell's README.",
		Cause: parseErr,
	}
	var positioned *toml.DecodeError
	if errors.As(parseErr, &positioned) {
		diagErr.Line, diagErr.Column = positioned.Position()
	}
	return diagErr
}

func withoutPrefixes(text string, prefixes ...string) string {
	for _, prefix := range prefixes {
		text = strings.TrimPrefix(text, prefix)
	}
	return text
}

func errSettingsUnreadable(displayPath string, cause error) error {
	return &diag.Error{
		Msg:   "Reading this file failed: " + fsx.Reason(cause),
		File:  displayPath,
		Hint:  "Check that it is a file this user may read.",
		Cause: cause,
	}
}

func errNotDecoded(displayPath string, cause error) error {
	_, lines, hasHeading := strings.Cut(cause.Error(), "\n\n")
	if !hasHeading {
		lines = cause.Error()
	}
	return &diag.Error{Msg: fsx.TrimASCIISpace(lines), File: displayPath, Hint: settingsListHint, Cause: cause}
}
