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

const File = "moonwell-library.json"

type LibraryFile struct {
	Dir    *string
	Assets *string
}

var knownKeys = []string{"dir", "assets"}

func parseLibraryFile(key string, data []byte, exists bool, displayPath string) (LibraryFile, error) {
	if !exists {
		return LibraryFile{}, nil
	}
	members, err := parseObject(key, data, displayPath)
	if err != nil {
		return LibraryFile{}, err
	}
	if unknown, found := firstUnknownKey(members); found {
		return LibraryFile{}, errUnknownKey(key, displayPath, unknown)
	}
	var libraryFile LibraryFile
	if libraryFile.Dir, err = parseDirMember(key, displayPath, members, "dir"); err != nil {
		return LibraryFile{}, err
	}
	if libraryFile.Assets, err = parseDirMember(key, displayPath, members, "assets"); err != nil {
		return LibraryFile{}, err
	}
	return libraryFile, nil
}

func parseObject(key string, data []byte, displayPath string) (map[string]json.RawMessage, error) {
	text := fsx.TrimBOM(data)
	if !utf8.Valid(text) || !json.Valid(text) {
		return nil, errNotJSON(key, displayPath)
	}
	members, isObject := asObject(text)
	if !isObject {
		return nil, errNotAnObject(key, displayPath)
	}
	return members, nil
}

func firstUnknownKey(members map[string]json.RawMessage) (name string, found bool) {
	for _, name := range slices.Sorted(maps.Keys(members)) {
		if !slices.Contains(knownKeys, name) {
			return name, true
		}
	}
	return "", false
}

func parseDirMember(key, displayPath string, members map[string]json.RawMessage, name string) (*string, error) {
	raw, ok := members[name]
	if !ok {
		return nil, nil
	}
	dir, isString := asString(raw)
	if !isString || !isInsideLibrary(dir) {
		return nil, errNotAFolder(key, displayPath, name, compactJSON(raw))
	}
	return &dir, nil
}

func asObject(value []byte) (members map[string]json.RawMessage, isObject bool) {
	if !bytes.HasPrefix(bytes.TrimLeft(value, " \t\r\n"), []byte("{")) {
		return nil, false
	}
	if err := json.Unmarshal(value, &members); err != nil {
		return nil, false
	}
	return members, true
}

func asString(value json.RawMessage) (text string, isString bool) {
	if !bytes.HasPrefix(value, []byte(`"`)) {
		return "", false
	}
	if err := json.Unmarshal(value, &text); err != nil {
		return "", false
	}
	return text, true
}

func compactJSON(value json.RawMessage) string {
	var out bytes.Buffer
	if err := json.Compact(&out, value); err != nil {
		return string(value)
	}
	return out.String()
}

const reportHint = "Report it to the library's author, or use another tag of the library."

func errInFile(key, displayPath, problem, hint string) error {
	return &diag.Error{Msg: "Library " + key + ": " + File + " " + problem, File: displayPath, Hint: hint}
}

func errNotJSON(key, displayPath string) error {
	return errInFile(key, displayPath, "is not valid JSON.", reportHint)
}

func errNotAnObject(key, displayPath string) error {
	return errInFile(key, displayPath, "is not a JSON object.", reportHint)
}

func errUnknownKey(key, displayPath, unknown string) error {
	return errInFile(key, displayPath, `has an unknown key "`+unknown+`".`,
		"This Moonwell knows "+strings.Join(knownKeys, " and ")+"; the library may need a newer Moonwell.")
}

func errNotAFolder(key, displayPath, name, value string) error {
	return errInFile(key, displayPath, "has "+name+" = "+value+", which is not a folder inside the library.", reportHint)
}
