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

func parseLibraryFile(key string, data []byte, present bool, where string) (LibraryFile, error) {
	if !present {
		return LibraryFile{}, nil
	}
	members, err := parseObject(key, data, where)
	if err != nil {
		return LibraryFile{}, err
	}
	if unknown, found := firstUnknownKey(members); found {
		return LibraryFile{}, errUnknownKey(key, where, unknown)
	}
	var described LibraryFile
	if described.Dir, err = parseDirMember(key, where, members, "dir"); err != nil {
		return LibraryFile{}, err
	}
	if described.Assets, err = parseDirMember(key, where, members, "assets"); err != nil {
		return LibraryFile{}, err
	}
	return described, nil
}

func parseObject(key string, data []byte, where string) (map[string]json.RawMessage, error) {
	text := fsx.TrimBOM(data)
	if !utf8.Valid(text) || !json.Valid(text) {
		return nil, errNotJSON(key, where)
	}
	members, isObject := asObject(text)
	if !isObject {
		return nil, errNotAnObject(key, where)
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

func parseDirMember(key, where string, members map[string]json.RawMessage, name string) (*string, error) {
	written, given := members[name]
	if !given {
		return nil, nil
	}
	folder, isString := asString(written)
	if !isString || !isInsideLibrary(folder) {
		return nil, errNotAFolder(key, where, name, compactJSON(written))
	}
	return &folder, nil
}

func isInsideLibrary(path string) bool {
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
