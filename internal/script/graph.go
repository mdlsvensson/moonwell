package script

import (
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

type leadsTo struct {
	module  *Module
	without string
}

func loaderOf(sources []Source, read func(Source) (text string, ok bool, err error)) func(name string) (leadsTo, error) {
	byName := make(map[string]Source, len(sources))
	for _, source := range sources {
		byName[source.Name] = source
	}
	return func(name string) (leadsTo, error) {
		source, has := byName[name]
		if !has {
			if source, has = byName[name+".init"]; !has {
				return leadsTo{}, nil
			}
		}
		module := &Module{Name: name, Path: source.Path, Kind: source.Kind, Library: source.Library, Lua: source.Text}
		if source.Kind == Lua {
			return leadsTo{module: module}, nil
		}
		text, ok, err := read(source)
		switch {
		case err != nil:
			return leadsTo{}, err
		case !ok:
			return leadsTo{without: source.Path}, nil
		}
		module.Lua = text
		return leadsTo{module: module}, nil
	}
}

func reached(entry string, load func(name string) (leadsTo, error)) ([]Module, error) {
	w := walk{load: load, state: map[string]int{}}
	if err := w.visit(entry, required{}); err != nil {
		return nil, err
	}
	return w.ordered, nil
}

const (
	visiting = iota + 1
	visited
)

type walk struct {
	load    func(name string) (leadsTo, error)
	state   map[string]int
	stack   []string
	ordered []Module
}

type required struct {
	file string
	line int
}

func (w *walk) visit(name string, at required) error {
	if slices.Contains(builtins, name) || w.state[name] == visited {
		return nil
	}
	if w.state[name] == visiting {
		return errCircular(append(slices.Clone(w.stack[slices.Index(w.stack, name):]), name), at)
	}
	led, err := w.load(name)
	if err != nil {
		return err
	}
	switch {
	case led.without != "":
		return errNoCode(name, led.without, at)
	case led.module == nil:
		return errNoModule(name, at)
	}
	w.state[name] = visiting
	w.stack = append(w.stack, name)
	if err := w.follow(led.module); err != nil {
		return err
	}
	w.stack = w.stack[:len(w.stack)-1]
	w.state[name] = visited
	w.ordered = append(w.ordered, *led.module)
	return nil
}

func (w *walk) follow(module *Module) error {
	for _, call := range lua.FindRequires(module.Lua) {
		if !call.Literal {
			return errComputedRequire(required{module.Path, call.Line})
		}
		if err := w.visit(call.Name, required{module.Path, call.Line}); err != nil {
			return err
		}
	}
	return nil
}

func errCircular(chain []string, at required) error {
	return &diag.Error{
		Msg:  "Circular require: " + strings.Join(chain, " \xe2\x86\x92 "),
		File: at.file,
		Line: at.line,
		Hint: "Move the shared code into a module that both can require.",
	}
}

func errNoModule(name string, at required) error {
	path := strings.ReplaceAll(name, ".", "/")
	return &diag.Error{
		Msg:  "Module '" + name + "' not found.",
		File: at.file,
		Line: at.line,
		Hint: "Expected src/" + path + ".yue, lua/" + path + ".lua, lua/" + path + "/init.lua or a module of a library in " +
			"moonwell.pkl. Built-in modules: " + strings.Join(builtins, ", ") + ".",
	}
}

func errNoCode(name, file string, at required) error {
	if at.file == "" {
		at = required{file: file}
	}
	return &diag.Error{
		Msg:  "Module '" + name + "' has no code.",
		File: at.file,
		Line: at.line,
		Hint: "YueScript writes no Lua for a file with nothing but comments and macros, and " + file + " is such a file.",
	}
}

func errComputedRequire(at required) error {
	return &diag.Error{
		Msg:  "require must be called with a single string literal.",
		File: at.file,
		Line: at.line,
		Hint: "Moonwell bundles modules at build time and cannot follow computed module names.",
	}
}
