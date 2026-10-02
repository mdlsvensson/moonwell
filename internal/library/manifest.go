// Package library brings a project's libraries of modules up to date: GitHub tag archives and local folders are
// copied into .moonwell/libraries/ and .moonwell/library-assets/, and moonwell.lock records what the tags resolved
// to.
package library

import (
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/ordered"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// File is the file a library describes its own layout with, at its root.
const File = "moonwell-library.json"

// Described is what a library says about itself. A nil field is not given. Both are folders inside the library,
// with "/".
type Described struct {
	// Dir is the folder module names start from.
	Dir *string
	// Assets is the folder whose files the map imports.
	Assets *string
}

var libraryKeys = []string{"dir", "assets"}

// isFolder reports whether value is a relative folder path of plain names: no empty, "." or ".." segment, no "\"
// and no ":".
func isFolder(value any) bool {
	folder, ok := value.(string)
	if !ok || strings.ContainsAny(folder, `\:`) {
		return false
	}
	for _, segment := range strings.Split(folder, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return false
		}
	}
	return true
}

// ParseFile reads a library's moonwell-library.json. data is the file's content; present is false for a library
// without one, which ships nothing but modules from its root. where names the file in errors: a path, or a URL for
// a download.
func ParseFile(key string, data []byte, present bool, where string) (Described, error) {
	if !present {
		return Described{}, nil
	}
	const report = "Report it to the library's author, or use another tag of the library."
	fail := func(problem, hint string) (Described, error) {
		return Described{}, &diag.Error{Msg: "Library " + key + ": " + File + " " + problem, File: where, Hint: hint}
	}
	if !utf8.Valid(data) {
		return fail("is not valid JSON.", report)
	}
	tree, err := ordered.Decode([]byte(strings.TrimPrefix(string(data), "\xEF\xBB\xBF")))
	if err != nil {
		return fail("is not valid JSON.", report)
	}
	fields, ok := tree.(*ordered.Object)
	if !ok {
		return fail("is not a JSON object.", report)
	}
	names := fields.Keys()
	text.Sort(names)
	for _, name := range names {
		if !slices.Contains(libraryKeys, name) {
			return fail(`has an unknown key "`+name+`".`,
				"This Moonwell knows "+strings.Join(libraryKeys, " and ")+"; the library may need a newer Moonwell.")
		}
	}
	var described Described
	for _, name := range libraryKeys {
		value, given := fields.Get(name)
		if !given {
			continue
		}
		if !isFolder(value) {
			return fail("has "+name+" = "+ordered.Stringify(value, 0)+", which is not a folder inside the library.", report)
		}
		folder := value.(string)
		if name == "dir" {
			described.Dir = &folder
		} else {
			described.Assets = &folder
		}
	}
	return described, nil
}
