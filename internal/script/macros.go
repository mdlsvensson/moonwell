package script

import (
	"path/filepath"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

const MacrosFile = ".moonwell/yue/moonwell/macros.yue"

func RefreshMacros(root string) (wrote bool, err error) {
	file, err := fsx.SafeJoinNoSymlinks(root, MacrosFile)
	if err != nil {
		return false, err
	}
	if wrote, err = fsx.WriteIfChanged(file, moonwell.MacrosYue); err != nil {
		return false, errMacrosNotWritten(err)
	}
	return wrote, nil
}

type macros struct {
	path string
	hash string
}

func macrosOf(root string) (macros, error) {
	if strings.ContainsAny(root, ";?") {
		return macros{}, errUnsearchableFolder(root)
	}
	return macros{
		path: filepath.Join(root, ".moonwell", "yue", "?.lua"),
		hash: fsx.SHA256Hex([]byte(moonwell.MacrosYue)),
	}, nil
}

func errMacrosNotWritten(cause error) error {
	return &diag.Error{
		Msg:   "Writing " + MacrosFile + " failed: " + fsx.Reason(cause),
		File:  MacrosFile,
		Hint:  "Moonwell's compiler and the editor read .moonwell/; make sure it is a folder you can write, then retry.",
		Cause: cause,
	}
}

func errUnsearchableFolder(root string) error {
	return &diag.Error{
		Msg:  `The project folder's path contains ";" or "?", which YueScript's module search cannot handle.`,
		File: root,
		Hint: "Move the project to a folder whose path has neither character.",
	}
}
