package assets

import (
	"regexp"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

var (
	mapInternal    = regexp.MustCompile(`(?i)^(?:war3map|war3campaign|\(listfile\)|\(attributes\)|\(signature\))`)
	importedFolder = regexp.MustCompile(`(?i)^war3mapImported/`)
	mapScript      = regexp.MustCompile(`(?i)^scripts/war3map\.`)
	mapListPicture = regexp.MustCompile(`(?i)^war3map(?:Preview|Map)\.`)
)

func targetPath(value string) (string, error) {
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

func reserved(path string) bool {
	internal := mapInternal.MatchString(path) && !importedFolder.MatchString(path)
	return internal || mapScript.MatchString(path)
}

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
		Hint: "For a picture of your own in the game's map list, set settings.info.preview.",
	}
}
