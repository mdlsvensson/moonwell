package assets

import (
	"regexp"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/fsx"
)

// What the start of a path says about it, in any letter case.
var (
	// mapInternal starts the name of a file the map holds for itself, or that the archive does.
	mapInternal = regexp.MustCompile(`(?i)^(?:war3map|war3campaign|\(listfile\)|\(attributes\)|\(signature\))`)
	// importedFolder is the folder World Editor imports into. Its name starts as a map internal's does.
	importedFolder = regexp.MustCompile(`(?i)^war3mapImported/`)
	// mapScript is the script where a map keeps it in a folder.
	mapScript = regexp.MustCompile(`(?i)^scripts/war3map\.`)
	// mapListPicture is the two names a picture for the game's map list is usually tried under: Reforged ignores
	// the one, and a build writes the other.
	mapListPicture = regexp.MustCompile(`(?i)^war3map(?:Preview|Map)\.`)
)

// TargetPath returns value as an in-map path an asset may be imported as. Map internals are never replaced.
func TargetPath(value string) (string, error) {
	path, ok := fsx.RelPath(value)
	switch {
	case !ok:
		return "", errInvalidPath(value)
	case mapListPicture.MatchString(path):
		return "", errMapListPicture(value)
	case reserved(path):
		return "", errReserved(value)
	}
	return path, nil
}

// reserved reports whether path, with "/", is one of the map's own files.
func reserved(path string) bool {
	internal := mapInternal.MatchString(path) && !importedFolder.MatchString(path)
	return internal || mapScript.MatchString(path)
}

// ---- errors ----

// The errors of a path have no file: a path is written in a manifest, in a state file, or is a file's own name,
// and the caller knows which.

func errInvalidPath(value string) error {
	return &diag.Error{
		Msg:  "Invalid asset path: " + value,
		Hint: "Use a relative path such as icons/BTNSword.blp, without .., drive letters or characters Windows forbids.",
	}
}

func errReserved(value string) error {
	return &diag.Error{
		Msg:  "Reserved map path: " + value,
		Hint: "Assets cannot replace map internals such as war3map.lua or war3map.imp.",
	}
}

func errMapListPicture(value string) error {
	return &diag.Error{
		Msg:  "Reserved map path: " + value,
		Hint: "For a picture of your own in the game's map list, set settings.info.preview in moonwell.pkl.",
	}
}
