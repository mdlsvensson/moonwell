package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

func readHandWritten(checkout, path string, target any) error {
	data, err := os.ReadFile(pathIn(checkout, path))
	if err != nil {
		return errInCheckout(checkout, path, err)
	}
	return decodeHandWritten(path, data, target)
}

func decodeHandWritten(path string, data []byte, target any) error {
	text := fsx.TrimBOM(data)
	decoder := json.NewDecoder(bytes.NewReader(text))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return errNoJSON(path, err)
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return errTrailingData(path)
	}
	if key, found := findDuplicateKey(json.NewDecoder(bytes.NewReader(text))); found {
		return errDuplicateKey(path, key)
	}
	return nil
}

func findDuplicateKey(decoder *json.Decoder) (string, bool) {
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
		if key, found := findDuplicateKey(decoder); found {
			return key, true
		}
	}
	_, _ = decoder.Token()
	return "", false
}

func errTrailingData(path string) error {
	return errors.New(path + ": something follows the JSON value: the file is one object")
}

func errDuplicateKey(path, key string) error {
	return errors.New(path + ": the key " + fsx.QuoteJSON(key) + " stands twice in one object: write it once")
}
