package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

func readHandWritten(checkout, path string, into any) error {
	data, err := os.ReadFile(pathIn(checkout, path))
	if err != nil {
		return errInCheckout(checkout, path, err)
	}
	return decodeHandWritten(path, data, into)
}

func decodeHandWritten(path string, data []byte, into any) error {
	text := fsx.TrimBOM(data)
	decoder := json.NewDecoder(bytes.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return errNoJSON(path, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errGoesOn(path)
	}
	if key, twice := keyTwice(json.NewDecoder(bytes.NewReader(text))); twice {
		return errKeyTwice(path, key)
	}
	return nil
}

func keyTwice(decoder *json.Decoder) (string, bool) {
	opening, _ := decoder.Token()
	if opening != json.Delim('{') && opening != json.Delim('[') {
		return "", false
	}
	seen := map[string]bool{}
	for decoder.More() {
		if opening == json.Delim('{') {
			token, _ := decoder.Token()
			key, _ := token.(string)
			if seen[key] {
				return key, true
			}
			seen[key] = true
		}
		if key, twice := keyTwice(decoder); twice {
			return key, true
		}
	}
	_, _ = decoder.Token()
	return "", false
}

func errGoesOn(path string) error {
	return errors.New(path + ": something follows the JSON value: the file is one object")
}

func errKeyTwice(path, key string) error {
	return errors.New(path + ": the key " + fsx.QuoteJSON(key) + " stands twice in one object: write it once")
}
