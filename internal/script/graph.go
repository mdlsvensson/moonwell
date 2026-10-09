package script

import (
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

func reachableModules(entry string, load func(name string) (loadResult, error)) ([]Module, error) {
	w := graphWalk{load: load, state: map[string]int{}}
	if err := w.visit(entry, requireSite{}); err != nil {
		return nil, err
	}
	return w.ordered, nil
}

type graphWalk struct {
	load    func(name string) (loadResult, error)
	state   map[string]int
	stack   []string
	ordered []Module
}

const (
	visiting = iota + 1
	visited
)

type requireSite struct {
	path string
	line int
}

func (w *graphWalk) visit(name string, site requireSite) error {
	if slices.Contains(builtins, name) || w.state[name] == visited {
		return nil
	}
	if w.state[name] == visiting {
		return errCircular(append(slices.Clone(w.stack[slices.Index(w.stack, name):]), name), site)
	}
	loaded, err := w.load(name)
	if err != nil {
		return err
	}
	switch {
	case loaded.emptySourcePath != "":
		return errNoCode(name, loaded.emptySourcePath, site)
	case loaded.module == nil:
		return errNoModule(name, site)
	}
	w.state[name] = visiting
	w.stack = append(w.stack, name)
	if err := w.visitRequires(loaded.module); err != nil {
		return err
	}
	w.stack = w.stack[:len(w.stack)-1]
	w.state[name] = visited
	w.ordered = append(w.ordered, *loaded.module)
	return nil
}

func (w *graphWalk) visitRequires(module *Module) error {
	for _, call := range lua.FindRequires(module.Lua) {
		if !call.Literal {
			return errComputedRequire(requireSite{module.Path, call.Line})
		}
		if err := w.visit(call.Name, requireSite{module.Path, call.Line}); err != nil {
			return err
		}
	}
	return nil
}

type loadResult struct {
	module          *Module
	emptySourcePath string
}

type moduleLoader struct {
	sourcesByName map[string]Source
	readLua       func(Source) (text string, ok bool, err error)
}

func newLoader(sources []Source, readLua func(Source) (text string, ok bool, err error)) func(name string) (loadResult, error) {
	loader := moduleLoader{sourcesByName: make(map[string]Source, len(sources)), readLua: readLua}
	for _, source := range sources {
		loader.sourcesByName[source.Name] = source
	}
	return loader.load
}

func (l moduleLoader) load(name string) (loadResult, error) {
	source, ok := l.findSource(name)
	if !ok {
		return loadResult{}, nil
	}
	module := &Module{Name: name, Path: source.Path, Kind: source.Kind, Library: source.Library, Lua: source.Text}
	if source.Kind == Lua {
		return loadResult{module: module}, nil
	}
	text, ok, err := l.readLua(source)
	switch {
	case err != nil:
		return loadResult{}, err
	case !ok:
		return loadResult{emptySourcePath: source.Path}, nil
	}
	module.Lua = text
	return loadResult{module: module}, nil
}

func (l moduleLoader) findSource(name string) (Source, bool) {
	if source, ok := l.sourcesByName[name]; ok {
		return source, true
	}
	source, ok := l.sourcesByName[name+".init"]
	return source, ok
}

func errCircular(chain []string, site requireSite) error {
	return &diag.Error{
		Msg:  "Circular require: " + strings.Join(chain, " \xe2\x86\x92 "),
		File: site.path,
		Line: site.line,
		Hint: "Move the shared code into a module that both can require.",
	}
}

func errNoModule(name string, site requireSite) error {
	path := strings.ReplaceAll(name, ".", "/")
	return &diag.Error{
		Msg:  "Module '" + name + "' not found.",
		File: site.path,
		Line: site.line,
		Hint: "Expected src/" + path + ".yue, lua/" + path + ".lua, lua/" + path + "/init.lua or a module of a library in " +
			"moonwell.toml. Built-in modules: " + strings.Join(builtins, ", ") + ".",
	}
}

func errNoCode(name, emptySourcePath string, site requireSite) error {
	if site.path == "" {
		site = requireSite{path: emptySourcePath}
	}
	return &diag.Error{
		Msg:  "Module '" + name + "' has no code.",
		File: site.path,
		Line: site.line,
		Hint: "YueScript writes no Lua for a file with nothing but comments and macros, and " + emptySourcePath + " is such a file.",
	}
}

func errComputedRequire(site requireSite) error {
	return &diag.Error{
		Msg:  "require must be called with a single string literal.",
		File: site.path,
		Line: site.line,
		Hint: "Moonwell bundles modules at build time and cannot follow computed module names.",
	}
}
