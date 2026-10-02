package editor

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/bundle"
	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

// LibraryViewDir is the editor's view of the libraries' modules: Lua by module path, which the workspace.library of
// .luarc.json lists.
const LibraryViewDir = ".moonwell/lua"

// RefreshLibraryView brings .moonwell/lua/ up to date with the libraries' modules: each module of a library as
// `<name with "." as "/">.lua`, a Lua module's source or a YueScript module's compiled output from loadCompiled
// (skipped when it has none). With a nil loadCompiled (setup, which compiles nothing), a YueScript module's existing
// file is kept as it is. Files are written only when their content differs, and every other file under the folder
// is removed. It returns the POSIX paths it wrote.
func RefreshLibraryView(root string, modules []bundle.SourceModule, loadCompiled func(bundle.SourceModule) (*bundle.CompiledModule, error)) ([]string, error) {
	written, err := refreshLibraryView(filepath.Join(root, filepath.FromSlash(LibraryViewDir)), modules, loadCompiled)
	if err == nil {
		return written, nil
	}
	var expected *diag.Error
	if errors.As(err, &expected) {
		return nil, err
	}
	return nil, &diag.Error{
		Msg:   "Writing " + LibraryViewDir + " failed: " + fsx.Reason(err),
		File:  LibraryViewDir,
		Cause: err,
		Hint:  "The editor reads .moonwell/; make sure it is a folder you can write, then retry.",
	}
}

func refreshLibraryView(dir string, modules []bundle.SourceModule, loadCompiled func(bundle.SourceModule) (*bundle.CompiledModule, error)) ([]string, error) {
	type view struct{ file, text string }
	var views []view
	// current are the files that stay: the ones written below and, without a loader, the YueScript modules' views from
	// the last compile.
	current := map[string]bool{}
	for _, module := range modules {
		if module.Library == "" {
			continue
		}
		file := strings.ReplaceAll(module.Name, ".", "/") + ".lua"
		switch {
		case module.Kind == bundle.Lua:
			views = append(views, view{file, module.Source})
		case loadCompiled == nil:
		default:
			// Reading a compiled output can fail too, for example a file another program holds.
			compiled, err := loadCompiled(module)
			if err != nil {
				return nil, err
			}
			if compiled == nil {
				continue
			}
			views = append(views, view{file, compiled.Source})
		}
		current[file] = true
	}
	written := []string{}
	for _, view := range views {
		wrote, err := fsx.WriteIfChanged(filepath.Join(dir, filepath.FromSlash(view.file)), view.text)
		if err != nil {
			return nil, err
		}
		if wrote {
			written = append(written, LibraryViewDir+"/"+view.file)
		}
	}
	existing, err := fsx.ListFiles(dir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	for _, file := range existing {
		if !current[file] {
			if err := fsx.RemoveFile(filepath.Join(dir, filepath.FromSlash(file))); err != nil {
				return nil, err
			}
		}
	}
	return written, nil
}
