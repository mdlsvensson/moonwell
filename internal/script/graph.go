package script

import (
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

type loadResult struct {
	module  *Module
	without string
}

func newLoader(sources []Source, read func(Source) (text string, ok bool, err error)) func(name string) (loadResult, error) {
	byName := make(map[string]Source, len(sources))
	for _, source := range sources {
		byName[source.Name] = source
	}
	return func(name string) (loadResult, error) {
		source, has := byName[name]
		if !has {
			if source, has = byName[name+".init"]; !has {
				return loadResult{}, nil
			}
		}
		module := &Module{Name: name, Path: source.Path, Kind: source.Kind, Library: source.Library, Lua: source.Text}
		if source.Kind == Lua {
			return loadResult{module: module}, nil
		}
		text, ok, err := read(source)
		switch {
		case err != nil:
			return loadResult{}, err
		case !ok:
			return loadResult{without: source.Path}, nil
		}
		module.Lua = text
		return loadResult{module: module}, nil
	}
}

func reachableModules(entry string, load func(name string) (loadResult, error)) ([]Module, error) {
	w := graphWalk{load: load, state: map[string]int{}}
	if err := w.visit(entry, requireSite{}); err != nil {
		return nil, err
	}
	return w.ordered, nil
}

const (
	visiting = iota + 1
	visited
)

type graphWalk struct {
	load    func(name string) (loadResult, error)
	state   map[string]int
	stack   []string
	ordered []Module
}

type requireSite struct {
	file string
	line int
}

func (w *graphWalk) visit(name string, site requireSite) error {
	if slices.Contains(builtins, name) || w.state[name] == visited {
		return nil
	}
	if w.state[name] == visiting {
		return errCircular(append(slices.Clone(w.stack[slices.Index(w.stack, name):]), name), site)
	}
	led, err := w.load(name)
	if err != nil {
		return err
	}
	switch {
	case led.without != "":
		return errNoCode(name, led.without, site)
	case led.module == nil:
		return errNoModule(name, site)
	}
	w.state[name] = visiting
	w.stack = append(w.stack, name)
	if err := w.visitRequires(led.module); err != nil {
		return err
	}
	w.stack = w.stack[:len(w.stack)-1]
	w.state[name] = visited
	w.ordered = append(w.ordered, *led.module)
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

func errCircular(chain []string, at requireSite) error {
	return &diag.Error{
		Msg:  "Circular require: " + strings.Join(chain, " \xe2\x86\x92 "),
		File: at.file,
		Line: at.line,
		Hint: "Move the shared code into a module that both can require.",
	}
}

func errNoModule(name string, at requireSite) error {
	path := strings.ReplaceAll(name, ".", "/")
	return &diag.Error{
		Msg:  "Module '" + name + "' not found.",
		File: at.file,
		Line: at.line,
		Hint: "Expected src/" + path + ".yue, lua/" + path + ".lua, lua/" + path + "/init.lua or a module of a library in " +
			"moonwell.pkl. Built-in modules: " + strings.Join(builtins, ", ") + ".",
	}
}

func errNoCode(name, file string, at requireSite) error {
	if at.file == "" {
		at = requireSite{file: file}
	}
	return &diag.Error{
		Msg:  "Module '" + name + "' has no code.",
		File: at.file,
		Line: at.line,
		Hint: "YueScript writes no Lua for a file with nothing but comments and macros, and " + file + " is such a file.",
	}
}

func errComputedRequire(at requireSite) error {
	return &diag.Error{
		Msg:  "require must be called with a single string literal.",
		File: at.file,
		Line: at.line,
		Hint: "Moonwell bundles modules at build time and cannot follow computed module names.",
	}
}
