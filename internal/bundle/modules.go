// Package bundle finds a project's gameplay modules, follows their requires and writes them as the one block of Lua
// that is appended to a map's war3map.lua.
package bundle

import (
	"path/filepath"
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/layout"
	"github.com/mdlsvensson/moonwell/internal/text"
)

// Kind is the language of a module file.
type Kind string

const (
	Yue Kind = "yue"
	Lua Kind = "lua"
)

// Builtins are the modules the runtime provides; no file may take their names.
var Builtins = []string{"moonwell"}

// narrowDir ends a hint for a library's file, which is not the project's to rename.
const narrowDir = "narrow the library's `dir` in moonwell.pkl so it leaves this file out."

// SourceModule is a gameplay module on disk.
type SourceModule struct {
	// Name is the dotted name, such as "utils.timer".
	Name string
	// Path is the POSIX path from the project root, such as "lua/utils/timer.lua".
	Path string
	Kind Kind
	// Source is a Lua module's text; a YueScript module is read when it is compiled.
	Source string
	// Library is the library's key for a library's module, and "" for the project's own.
	Library string
}

// CompiledModule is a module as Lua, ready for the bundle.
type CompiledModule struct {
	// Name is the name the module was required by, which is the name the bundle defines it with.
	Name string
	// SourcePath is the POSIX path of the file the user wrote, from the project root.
	SourcePath string
	// Source is the module's Lua.
	Source string
	// Kind is Lua for a Lua module, which a minified bundle keeps line for line.
	Kind Kind
}

// ModuleRoot is a folder of modules and the kind of file it holds.
type ModuleRoot struct {
	// Dir is the POSIX path from the project root, such as "src".
	Dir  string
	Kind Kind
	// Required makes a missing folder an error.
	Required bool
	// Library is the library's key when the folder is a library's.
	Library string
}

// ProjectRoots are the project's own modules: YueScript in src/ and Lua in lua/. A .lua file under src/ is the
// editor's output.
var ProjectRoots = []ModuleRoot{
	{Dir: "src", Kind: Yue, Required: true},
	{Dir: "lua", Kind: Lua},
}

// LibraryRoots gives each library a YueScript and a Lua root over .moonwell/libraries/<key>, in key order.
func LibraryRoots(keys []string) []ModuleRoot {
	sorted := slices.Clone(keys)
	text.Sort(sorted)
	var roots []ModuleRoot
	for _, key := range sorted {
		for _, kind := range []Kind{Yue, Lua} {
			roots = append(roots, ModuleRoot{Dir: layout.LibrariesDir + "/" + key, Kind: kind, Library: key})
		}
	}
	return roots
}

// claimedNames are the names a module answers to: its own and, for `<parent>.init`, `<parent>` too.
func claimedNames(name string) []string {
	if parent, isInit := strings.CutSuffix(name, ".init"); isInit {
		return []string{name, parent}
	}
	return []string{name}
}

// CollectModules lists every module under roots, in root order and then by path. It fails on a dotted file or folder
// name, on two files with one name and on a file that takes a built-in module's name. In a library root, a .lua file
// beside a .yue file of the same stem is that module's compiled output and is skipped. Lua sources are read as Lua's
// loadfile reads them.
func CollectModules(root string, roots []ModuleRoot) ([]SourceModule, error) {
	var modules []SourceModule
	byName := map[string]SourceModule{}
	for _, moduleRoot := range roots {
		dir := filepath.Join(root, filepath.FromSlash(moduleRoot.Dir))
		if !fsx.IsDir(dir) {
			if moduleRoot.Required {
				return nil, &diag.Error{Msg: "The " + moduleRoot.Dir + "/ folder is missing.", File: root}
			}
			continue
		}
		extension := "." + string(moduleRoot.Kind)
		files, err := fsx.ListFiles(dir)
		if err != nil {
			return nil, err
		}
		inLibrary := moduleRoot.Library != ""
		// In a library, YueScript and Lua share one folder: a .lua beside a .yue of the same stem is its compiled output.
		compiled := map[string]bool{}
		if inLibrary && moduleRoot.Kind == Lua {
			for _, file := range files {
				if stem, isYue := strings.CutSuffix(file, ".yue"); isYue {
					compiled[stem+".lua"] = true
				}
			}
		}
		for _, file := range files {
			stem, matches := strings.CutSuffix(file, extension)
			if !matches || compiled[file] {
				continue
			}
			path := moduleRoot.Dir + "/" + file
			segments := strings.Split(stem, "/")
			if slices.ContainsFunc(segments, func(segment string) bool { return strings.Contains(segment, ".") }) {
				advice := "rename the file or folder."
				if inLibrary {
					advice = narrowDir
				}
				return nil, &diag.Error{
					Msg:  "Module file and folder names cannot contain dots.",
					File: path,
					Hint: "Dots separate module names in `import`; " + advice,
				}
			}
			module := SourceModule{Name: strings.Join(segments, "."), Path: path, Kind: moduleRoot.Kind, Library: moduleRoot.Library}
			for _, name := range claimedNames(module.Name) {
				if slices.Contains(Builtins, name) {
					if inLibrary {
						return nil, &diag.Error{
							Msg:  "Module " + name + " is built into Moonwell; " + path + " takes its name.",
							File: path,
							Hint: "`require` of a built-in name always loads the built-in module; " + narrowDir,
						}
					}
					return nil, &diag.Error{
						Msg:  "Module " + name + " is built into Moonwell; rename " + path + ".",
						File: path,
						Hint: "`require` of a built-in name always loads the built-in module, never a project file.",
					}
				}
				if clash, taken := byName[name]; taken {
					hint := "Rename one of them: module names are shared by src/ and lua/."
					if inLibrary || clash.Library != "" {
						hint = "Rename one of them, or narrow the library's `dir`: module names are shared by src/, lua/ and libraries."
					}
					return nil, &diag.Error{
						Msg: "Module " + name + " is defined by " + clash.Path + " and " + path + ".", File: path, Hint: hint,
					}
				}
				byName[name] = module
			}
			if module.Kind == Lua {
				if module.Source, err = fsx.ReadSource(filepath.Join(dir, filepath.FromSlash(file)), path); err != nil {
					return nil, err
				}
			}
			modules = append(modules, module)
		}
	}
	return modules, nil
}

// Loader resolves require names for the bundler: the name, then `<name>.init`, as Lua's `?/init.lua`. A Lua module
// is its own source; a YueScript module is its compiled output, which loadCompiled gives for the module (nil when it
// has none). Either is returned under the name it was required by; an unknown name gives nil.
func Loader(modules []SourceModule, loadCompiled func(SourceModule) (*CompiledModule, error)) func(name string) (*CompiledModule, error) {
	byName := make(map[string]SourceModule, len(modules))
	for _, module := range modules {
		byName[module.Name] = module
	}
	return func(name string) (*CompiledModule, error) {
		module, found := byName[name]
		if !found {
			if module, found = byName[name+".init"]; !found {
				return nil, nil
			}
		}
		if module.Kind == Lua {
			return &CompiledModule{Name: name, SourcePath: module.Path, Source: module.Source, Kind: Lua}, nil
		}
		compiled, err := loadCompiled(module)
		if err != nil || compiled == nil {
			return nil, err
		}
		renamed := *compiled
		renamed.Name = name
		return &renamed, nil
	}
}
