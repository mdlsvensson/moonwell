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

type Library struct{ Key, Dir string }

type Source struct {
	Name    string
	Path    string
	Kind    Kind
	Library string
	Text    string
}

func CollectSources(root string, libraries []Library) ([]Source, error) {
	libraryDirs, err := libraryModuleDirs("Collect", libraries)
	if err != nil {
		return nil, err
	}
	return collectSources(root, append(projectModuleDirs(), libraryDirs...))
}

func CollectLibrarySources(root string, libraries []Library) ([]Source, error) {
	dirs, err := libraryModuleDirs("CollectLibraries", libraries)
	if err != nil {
		return nil, err
	}
	return collectSources(root, dirs)
}

func collectSources(root string, candidates []moduleDir) ([]Source, error) {
	dirs, err := findModuleDirs(root, candidates)
	if err != nil {
		return nil, err
	}
	set := sourceSet{byName: map[string]Source{}}
	for _, dir := range dirs {
		if err := set.addDir(dir); err != nil {
			return nil, err
		}
	}
	return set.sources, nil
}

type moduleDir struct {
	dir      string
	fullPath string
	kind     Kind
	library  string
	required bool
}

func projectModuleDirs() []moduleDir {
	return []moduleDir{{dir: "src", kind: Yue, required: true}, {dir: "lua", kind: Lua}}
}

func libraryModuleDirs(caller string, libraries []Library) ([]moduleDir, error) {
	var dirs []moduleDir
	for _, library := range libraries {
		dir, ok := fsx.CleanRelPath(library.Dir)
		if library.Key == "" || !ok {
			return nil, fmt.Errorf("script.%s: library %q has the folder %q, which no library can have",
				caller, library.Key, library.Dir)
		}
		dirs = append(dirs,
			moduleDir{dir: dir, kind: Yue, library: library.Key}, moduleDir{dir: dir, kind: Lua, library: library.Key})
	}
	return dirs, nil
}

func findModuleDirs(root string, candidates []moduleDir) ([]moduleDir, error) {
	var existing []moduleDir
	for _, candidate := range candidates {
		fullPath, found, err := findDir(root, candidate.dir)
		switch {
		case err != nil:
			return nil, err
		case found:
			candidate.fullPath = fullPath
			existing = append(existing, candidate)
		case candidate.required:
			return nil, errNoSrc(root)
		}
	}
	return existing, nil
}

func findDir(root, dir string) (fullPath string, found bool, err error) {
	if !fsx.IsDir(filepath.Join(root, filepath.FromSlash(dir))) {
		return "", false, nil
	}
	fullPath, err = fsx.SafeJoinNoSymlinks(root, dir)
	return fullPath, err == nil, err
}

type sourceSet struct {
	sources []Source
	byName  map[string]Source
}

func (s *sourceSet) addDir(dir moduleDir) error {
	files, err := dir.listModuleFiles()
	if err != nil {
		return err
	}
	for _, file := range files {
		source, err := dir.readSource(file)
		if err != nil {
			return err
		}
		if err := s.addSource(source); err != nil {
			return err
		}
		if source.Kind == Lua {
			if source.Text, err = fsx.ReadSource(filepath.Join(dir.fullPath, filepath.FromSlash(file)), source.Path); err != nil {
				return err
			}
		}
		s.sources = append(s.sources, source)
	}
	return nil
}

func (d moduleDir) listModuleFiles() ([]string, error) {
	files, err := fsx.ListFiles(d.fullPath)
	if err != nil {
		return nil, errUnreadableFolder(d.failedPath(err), err)
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

func (d moduleDir) failedPath(cause error) string {
	var pathErr *fs.PathError
	if !errors.As(cause, &pathErr) {
		return d.dir
	}
	rel, err := filepath.Rel(d.fullPath, pathErr.Path)
	if err != nil || rel == "." || !filepath.IsLocal(rel) {
		return d.dir
	}
	return d.dir + "/" + fsx.ToSlash(rel)
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
		if existing, taken := s.byName[name]; taken {
			return errTwoFiles(name, existing, source)
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

var builtins = []string{"moonwell"}

func EntryName(entry string) (string, error) {
	path := strings.TrimPrefix(strings.ReplaceAll(entry, `\`, "/"), "./")
	withoutExtension, isYue := strings.CutSuffix(path, ".yue")
	modulePath, inSrc := strings.CutPrefix(withoutExtension, "src/")
	if !isYue || !inSrc {
		return "", errNoEntryFile(entry)
	}
	return strings.ReplaceAll(modulePath, "/", "."), nil
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
