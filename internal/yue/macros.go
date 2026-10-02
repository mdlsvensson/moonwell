package yue

import (
	"path/filepath"
	"strings"

	moonwell "github.com/mdlsvensson/moonwell"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// MacrosFile is where check, build, test, dev and setup write the macro module, from the project root.
const MacrosFile = ".moonwell/yue/moonwell/macros.yue"

// MacroSearch is how a yue run finds `import "moonwell.macros"`.
type MacroSearch struct {
	// Path is the compiler's --path: yue tries each .lua pattern with .yue, so this finds moonwell/macros.yue.
	Path string
	// Hash is the SHA-256 of the macro module. It joins the compile and `yue -g` cache keys, so a changed macro
	// redoes every file.
	Hash string
}

// Macros is the macro search of the project at root.
func Macros(root string) (*MacroSearch, error) {
	// Lua's search path splits on ";" and puts the module name in place of "?", so neither may be in the root.
	if strings.ContainsAny(root, ";?") {
		return nil, &diag.Error{
			Msg:  `The project folder's path contains ";" or "?", which YueScript's module search cannot handle.`,
			File: root,
			Hint: "Move the project to a folder whose path has neither character.",
		}
	}
	return &MacroSearch{
		Path: filepath.Join(root, ".moonwell", "yue", "?.lua"),
		Hash: fsx.SHA256Hex([]byte(moonwell.MacrosYue)),
	}, nil
}

// PathArgs are the --path arguments of a yue run; none without macros. They go before the source file.
func (m *MacroSearch) PathArgs() []string {
	if m == nil {
		return nil
	}
	return []string{"--path", m.Path}
}
