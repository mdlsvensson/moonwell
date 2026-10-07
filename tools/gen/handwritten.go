package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// readHandWritten reads one of the two files of a checkout that a contributor writes by hand, the Lua extras
// and the overrides of the object metadata, into the struct that has the file's keys. path is the file by its
// path from the checkout.
func readHandWritten(checkout, path string, into any) error {
	data, err := os.ReadFile(fileIn(checkout, path))
	if err != nil {
		return errInCheckout(checkout, path, err)
	}
	return decodeHandWritten(path, data, into)
}

// decodeHandWritten reads the text of a hand-written file into the struct that has its keys. The text is one
// JSON value and nothing after it, every value of the kind that its place in the struct takes. A key that the
// struct does not have is refused, in the file and in every object below it whose keys the struct names (a
// function of the extras, and a parameter of one): what stands under it would reach no file that the generator
// writes. Where the struct has a map, every key is taken as it is written: the lists and the ids of the
// overrides, which their reader holds to the fields of the game (namingNothing). A key that an object has twice
// is refused in every object: one of its two values would be read, and nothing would say which.
//
// A byte order mark at the start of the text is dropped: an editor on Windows writes one. A key that is left
// out, and one that is null, leave empty what the struct has for it; whether the file must have the key is its
// reader's to say. A key that the struct names is known in letters of either case, as Go's decoder knows one.
func decodeHandWritten(path string, data []byte, into any) error {
	text := fsx.WithoutMark(data)
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

// keyTwice reads one JSON value from the decoder, and returns the first key that an object in it has twice.
// The value is one that a decoding has taken already: no token of it fails, so no failure is looked at.
func keyTwice(decoder *json.Decoder) (string, bool) {
	opening, _ := decoder.Token()
	if opening != json.Delim('{') && opening != json.Delim('[') {
		return "", false // a text, a number, true, false or null: it holds no key
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
	_, _ = decoder.Token() // the closing bracket
	return "", false
}

// ---- errors ----

// errGoesOn refuses a hand-written file that holds more than one JSON value.
func errGoesOn(path string) error {
	return errors.New(path + ": something follows the JSON value: the file is one object")
}

// errKeyTwice refuses a hand-written file in which an object has a key twice.
func errKeyTwice(path, key string) error {
	return errors.New(path + ": the key " + fsx.Quoted(key) + " stands twice in one object: write it once")
}
