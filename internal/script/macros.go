package script

import (
	"path/filepath"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// MacrosFile is where the macro module is written, from the project folder.
const MacrosFile = ".moonwell/yue/moonwell/macros.yue"

// RefreshMacros writes the macro module when its content differs, so that `import "moonwell.macros"` is found by
// the compiler and by an editor. A link on the way to the file is refused, and nothing is written through it.
func RefreshMacros(root string) (wrote bool, err error) {
	file, err := fsx.Inside(root, MacrosFile)
	if err != nil {
		return false, err
	}
	if wrote, err = fsx.WriteIfChanged(file, moonwell.MacrosYue); err != nil {
		return false, errMacrosNotWritten(err)
	}
	return wrote, nil
}

// macros is how a run of the compiler finds `import "moonwell.macros"`.
type macros struct {
	// path is the compiler's --path. The compiler tries each .lua pattern with .yue too, so this finds
	// moonwell/macros.yue: MacrosFile.
	path string
	// hash is the SHA-256 of the macro module. What a compile keeps depends on it, so a changed macro module
	// compiles every file again.
	hash string
}

// macrosOf is the macro search of the project at root. Lua's search path splits on ";" and puts the module's name
// in place of "?", so a project folder whose path has either is refused.
func macrosOf(root string) (macros, error) {
	if strings.ContainsAny(root, ";?") {
		return macros{}, errUnsearchableFolder(root)
	}
	return macros{
		path: filepath.Join(root, ".moonwell", "yue", "?.lua"),
		hash: fsx.SHA256Hex([]byte(moonwell.MacrosYue)),
	}, nil
}

// ---- errors ----

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
