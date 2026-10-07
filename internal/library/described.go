package library

import (
	"bytes"
	"encoding/json"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// File is the file a library describes its own layout with, at its root.
const File = "moonwell-library.json"

// Described is what a library says about itself. A nil field is not given. Both are folders inside the library,
// with "/".
type Described struct {
	Dir    *string // the folder module names start from
	Assets *string // the folder whose files the map imports
}

// knownKeys is the keys a library's file may have.
var knownKeys = []string{"dir", "assets"}

// parseFile reads a library's moonwell-library.json. present is false for a library without one, which ships
// nothing but modules from its root. where names the file in errors: a path, or an address for a download.
func parseFile(key string, data []byte, present bool, where string) (Described, error) {
	if !present {
		return Described{}, nil
	}
	members, err := membersOf(key, data, where)
	if err != nil {
		return Described{}, err
	}
	if unknown, found := firstUnknown(members); found {
		return Described{}, errUnknownKey(key, where, unknown)
	}
	var described Described
	if described.Dir, err = folderOf(key, where, members, "dir"); err != nil {
		return Described{}, err
	}
	if described.Assets, err = folderOf(key, where, members, "assets"); err != nil {
		return Described{}, err
	}
	return described, nil
}

// membersOf is the members of the JSON object that a library's file holds, each value as the file writes it. A
// byte order mark at the start is read past.
func membersOf(key string, data []byte, where string) (map[string]json.RawMessage, error) {
	text := fsx.WithoutMark(data)
	if !utf8.Valid(text) || !json.Valid(text) {
		return nil, errNotJSON(key, where)
	}
	members, isObject := objectOf(text)
	if !isObject {
		return nil, errNotAnObject(key, where)
	}
	return members, nil
}

// firstUnknown is the first key by bytes that a library's file has and may not have.
func firstUnknown(members map[string]json.RawMessage) (name string, found bool) {
	for _, name := range slices.Sorted(maps.Keys(members)) {
		if !slices.Contains(knownKeys, name) {
			return name, true
		}
	}
	return "", false
}

// folderOf is the folder a library's file names under name; nil when the file has no such key. A value that is no
// folder inside the library is refused, and shown as the file writes it.
func folderOf(key, where string, members map[string]json.RawMessage, name string) (*string, error) {
	written, given := members[name]
	if !given {
		return nil, nil
	}
	folder, isString := stringOf(written)
	if !isString || !insideLibrary(folder) {
		return nil, errNotAFolder(key, where, name, compact(written))
	}
	return &folder, nil
}

// insideLibrary reports whether path, written with "/", is a way to a file or a folder inside a library by plain
// names: it has no "\" and no ":", and no segment that is empty, "." or "..". So it does not start with "/".
func insideLibrary(path string) bool {
	if strings.ContainsAny(path, `\:`) {
		return false
	}
	for segment := range strings.SplitSeq(path, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// objectOf reads a JSON value as an object: its members, each value as it is written. Of a member that is written
// twice, the last counts. It is false for a value of another kind. The value is valid JSON, or none at all.
func objectOf(value []byte) (members map[string]json.RawMessage, isObject bool) {
	if !bytes.HasPrefix(bytes.TrimLeft(value, " \t\r\n"), []byte("{")) {
		return nil, false
	}
	if err := json.Unmarshal(value, &members); err != nil {
		return nil, false
	}
	return members, true
}

// stringOf reads a JSON value as a string; false for a value of another kind, and for none at all.
func stringOf(value json.RawMessage) (text string, isString bool) {
	if !bytes.HasPrefix(value, []byte(`"`)) {
		return "", false
	}
	if err := json.Unmarshal(value, &text); err != nil {
		return "", false
	}
	return text, true
}

// compact is a JSON value in the characters it is written in, without the white space between its parts.
func compact(value json.RawMessage) string {
	var out bytes.Buffer
	if err := json.Compact(&out, value); err != nil {
		return string(value)
	}
	return out.String()
}

// ---- errors ----

const reportHint = "Report it to the library's author, or use another tag of the library."

// errInFile is a refusal of a library's file: problem follows the library and the file's name.
func errInFile(key, where, problem, hint string) error {
	return &diag.Error{Msg: "Library " + key + ": " + File + " " + problem, File: where, Hint: hint}
}

func errNotJSON(key, where string) error {
	return errInFile(key, where, "is not valid JSON.", reportHint)
}

func errNotAnObject(key, where string) error {
	return errInFile(key, where, "is not a JSON object.", reportHint)
}

func errUnknownKey(key, where, unknown string) error {
	return errInFile(key, where, `has an unknown key "`+unknown+`".`,
		"This Moonwell knows "+strings.Join(knownKeys, " and ")+"; the library may need a newer Moonwell.")
}

func errNotAFolder(key, where, name, written string) error {
	return errInFile(key, where, "has "+name+" = "+written+", which is not a folder inside the library.", reportHint)
}
