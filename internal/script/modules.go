package script

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type Kind string

const (
	Yue Kind = "yue"
	Lua Kind = "lua"
)

var builtins = []string{"moonwell"}

type Library struct{ Key, Dir string }

type Source struct {
	Name    string
	Path    string
	Kind    Kind
	Library string
	Text    string
}

func CollectSources(root string, libraries []Library) ([]Source, error) {
	ofLibraries, err := libraryModuleDirs("Collect", libraries)
	if err != nil {
		return nil, err
	}
	return collectSources(root, append(projectModuleDirs(), ofLibraries...))
}

func CollectLibrarySources(root string, libraries []Library) ([]Source, error) {
	searched, err := libraryModuleDirs("CollectLibraries", libraries)
	if err != nil {
		return nil, err
	}
	return collectSources(root, searched)
}

func collectSources(root string, searched []moduleDir) ([]Source, error) {
	folders, err := findModuleDirs(root, searched)
	if err != nil {
		return nil, err
	}
	found := sourceSet{byName: map[string]Source{}}
	for _, f := range folders {
		if err := found.addDir(f); err != nil {
			return nil, err
		}
	}
	return found.sources, nil
}

func EntryName(entry string) (string, error) {
	file := strings.TrimPrefix(strings.ReplaceAll(entry, `\`, "/"), "./")
	stem, isYue := strings.CutSuffix(file, ".yue")
	under, inSrc := strings.CutPrefix(stem, "src/")
	if !isYue || !inSrc {
		return "", errNoEntryFile(entry)
	}
	return strings.ReplaceAll(under, "/", "."), nil
}

type moduleDir struct {
	dir      string
	path     string
	kind     Kind
	library  string
	required bool
}

func findModuleDirs(root string, searched []moduleDir) ([]moduleDir, error) {
	var present []moduleDir
	for _, f := range searched {
		path, found, err := findDir(root, f.dir)
		switch {
		case err != nil:
			return nil, err
		case found:
			f.path = path
			present = append(present, f)
		case f.required:
			return nil, errNoSrc(root)
		}
	}
	return present, nil
}

func projectModuleDirs() []moduleDir {
	return []moduleDir{{dir: "src", kind: Yue, required: true}, {dir: "lua", kind: Lua}}
}

func libraryModuleDirs(door string, libraries []Library) ([]moduleDir, error) {
	var searched []moduleDir
	for _, library := range libraries {
		dir, ok := fsx.CleanRelPath(library.Dir)
		if library.Key == "" || !ok {
			return nil, fmt.Errorf("script.%s: library %q has the folder %q, which no library can have",
				door, library.Key, library.Dir)
		}
		searched = append(searched,
			moduleDir{dir: dir, kind: Yue, library: library.Key}, moduleDir{dir: dir, kind: Lua, library: library.Key})
	}
	return searched, nil
}

func findDir(root, dir string) (path string, found bool, err error) {
	if !fsx.IsDir(filepath.Join(root, filepath.FromSlash(dir))) {
		return "", false, nil
	}
	path, err = fsx.SafeJoinNoSymlinks(root, dir)
	return path, err == nil, err
}

type sourceSet struct {
	sources []Source
	byName  map[string]Source
}

func (s *sourceSet) addDir(f moduleDir) error {
	files, err := f.listModuleFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		source, err := f.readSource(file)
		if err != nil {
			return err
		}
		if err := s.addSource(source); err != nil {
			return err
		}
		if source.Kind == Lua {
			if source.Text, err = fsx.ReadSource(filepath.Join(f.path, filepath.FromSlash(file)), source.Path); err != nil {
				return err
			}
		}
		s.sources = append(s.sources, source)
	}
	return nil
}

func (d moduleDir) listModuleFiles() ([]string, error) {
	files, err := fsx.ListFiles(d.path)
	if err != nil {
		return nil, errUnreadableFolder(d.failedAt(err), err)
	}
	outputs := d.compiledOutputs(files)
	extension := "." + string(d.kind)
	var modules []string
	for _, file := range files {
		if strings.HasSuffix(file, extension) && !outputs[file] {
			modules = append(modules, file)
		}
	}
	return modules, nil
}

func (d moduleDir) failedAt(cause error) string {
	var failure *fs.PathError
	if !errors.As(cause, &failure) {
		return d.dir
	}
	below, err := filepath.Rel(d.path, failure.Path)
	if err != nil || below == "." || !filepath.IsLocal(below) {
		return d.dir
	}
	return d.dir + "/" + fsx.ToSlash(below)
}

func (d moduleDir) compiledOutputs(files []string) map[string]bool {
	outputs := map[string]bool{}
	if d.library == "" || d.kind != Lua {
		return outputs
	}
	for _, file := range files {
		if stem, isYue := strings.CutSuffix(file, ".yue"); isYue {
			outputs[stem+".lua"] = true
		}
	}
	return outputs
}

func (d moduleDir) readSource(file string) (Source, error) {
	path := d.dir + "/" + file
	stem := strings.TrimSuffix(file, "."+string(d.kind))
	switch {
	case strings.Contains(stem, "."):
		return Source{}, errDottedName(path, d.library != "")
	case !utf8.ValidString(file):
		return Source{}, errNameNotUTF8(path, d.library != "")
	}
	return Source{Name: strings.ReplaceAll(stem, "/", "."), Path: path, Kind: d.kind, Library: d.library}, nil
}

func (s *sourceSet) addSource(source Source) error {
	for _, name := range claimedNames(source.Name) {
		if slices.Contains(builtins, name) {
			return errBuiltinName(name, source)
		}
		if other, taken := s.byName[name]; taken {
			return errTwoFiles(name, other, source)
		}
		s.byName[name] = source
	}
	return nil
}

func claimedNames(name string) []string {
	if parent, isInit := strings.CutSuffix(name, ".init"); isInit {
		return []string{name, parent}
	}
	return []string{name}
}

const narrowDir = "narrow the library's `dir` in moonwell.pkl so it leaves this file out."

func errNoSrc(root string) error {
	return &diag.Error{Msg: "The src/ folder is missing.", File: root}
}

func errUnreadableFolder(dir string, cause error) error {
	return &diag.Error{
		Msg:   "Reading " + dir + "/ failed: " + fsx.Reason(cause),
		File:  dir,
		Hint:  "Check that the folder and everything in it can be read, then try again.",
		Cause: cause,
	}
}

func errDottedName(path string, inLibrary bool) error {
	advice := "rename the file or folder."
	if inLibrary {
		advice = narrowDir
	}
	return &diag.Error{
		Msg:  "Module file and folder names cannot contain dots.",
		File: path,
		Hint: "Dots separate module names in `import`; " + advice,
	}
}

func errNameNotUTF8(path string, inLibrary bool) error {
	advice := "rename the file or folder."
	if inLibrary {
		advice = narrowDir
	}
	return &diag.Error{
		Msg:  "Module file and folder names must be valid UTF-8.",
		File: path,
		Hint: "A module is required by its name, which the map's script holds as text; " + advice,
	}
}

func errBuiltinName(name string, source Source) error {
	if source.Library != "" {
		return &diag.Error{
			Msg:  "Module " + name + " is built into Moonwell; " + source.Path + " takes its name.",
			File: source.Path,
			Hint: "`require` of a built-in name always loads the built-in module; " + narrowDir,
		}
	}
	return &diag.Error{
		Msg:  "Module " + name + " is built into Moonwell; rename " + source.Path + ".",
		File: source.Path,
		Hint: "`require` of a built-in name always loads the built-in module, never a project file.",
	}
}

func errTwoFiles(name string, first, second Source) error {
	hint := "Rename one of them: module names are shared by src/ and lua/."
	if first.Library != "" || second.Library != "" {
		hint = "Rename one of them, or narrow the library's `dir`: module names are shared by src/, lua/ and libraries."
	}
	return &diag.Error{
		Msg:  "Module " + name + " is defined by " + first.Path + " and " + second.Path + ".",
		File: second.Path,
		Hint: hint,
	}
}

func errNoEntryFile(entry string) error {
	return &diag.Error{Msg: "Entry '" + entry + "' must be a .yue file under src/.", Hint: "For example: src/main.yue"}
}
