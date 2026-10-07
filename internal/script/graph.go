package script

import (
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

// leadsTo is what a name that a require gives leads to: a module; or, for a module whose source compiled to no
// Lua, the file of that source; or neither, for a name that no module has.
type leadsTo struct {
	module  *Module
	without string // the path, from the project folder, of a YueScript source without an output
}

// loaderOf finds the module of a name that a require gives: the module of that name, else `<name>.init`, as
// Lua's `?/init.lua`. A Lua module is its own text; a YueScript module is its compiled Lua, which read gives for
// the source, with ok false for a source without an output. Either is returned under the name it was required
// by. A YueScript module without an output is told from a name that no module has: the compiler writes no Lua
// for a source without code, and the file is there all the same.
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

// reached walks the requires from the entry and returns the modules it reaches, each after the modules it
// requires. load says what a name leads to.
//
// A name is visited once, and a built-in module is never loaded. A require is followed only when it is called
// with one string literal: a module's name is what the bundle defines it by, so a name that is computed cannot
// be followed. A failure of load is passed on as it is.
func reached(entry string, load func(name string) (leadsTo, error)) ([]Module, error) {
	w := walk{load: load, state: map[string]int{}}
	if err := w.visit(entry, required{}); err != nil {
		return nil, err
	}
	return w.ordered, nil
}

// The states of a name in a walk that has come to it: its requires are being followed, or all of them have been.
const (
	visiting = iota + 1
	visited
)

// walk is one walk of the requires.
type walk struct {
	load    func(name string) (leadsTo, error)
	state   map[string]int // by name; 0 for a name the walk has not come to
	stack   []string       // the names whose requires are being followed, from the entry down
	ordered []Module       // the modules whose requires have all been followed
}

// required is where a module is required: the requiring module's file and the line of the call. The entry is
// required nowhere.
type required struct {
	file string
	line int
}

// visit loads the module of a name and follows its requires, unless the walk has been there.
func (w *walk) visit(name string, at required) error {
	if slices.Contains(Builtins, name) || w.state[name] == visited {
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

// follow visits what a module requires, in the order of the calls.
func (w *walk) follow(module *Module) error {
	for _, call := range lua.Requires(module.Lua) {
		if !call.Literal {
			return errComputedRequire(required{module.Path, call.Line})
		}
		if err := w.visit(call.Name, required{module.Path, call.Line}); err != nil {
			return err
		}
	}
	return nil
}

// ---- errors ----

// errCircular refuses a module that requires itself through the names of the chain, which starts and ends with
// it. at is the require that closes the circle.
func errCircular(chain []string, at required) error {
	return &diag.Error{
		Msg:  "Circular require: " + strings.Join(chain, " \xe2\x86\x92 "),
		File: at.file,
		Line: at.line,
		Hint: "Move the shared code into a module that both can require.",
	}
}

// errNoModule refuses a name that no module has. Its hint names every file the name is looked for in.
func errNoModule(name string, at required) error {
	path := strings.ReplaceAll(name, ".", "/")
	return &diag.Error{
		Msg:  "Module '" + name + "' not found.",
		File: at.file,
		Line: at.line,
		Hint: "Expected src/" + path + ".yue, lua/" + path + ".lua, lua/" + path + "/init.lua or a module of a library in " +
			"moonwell.pkl. Built-in modules: " + strings.Join(Builtins, ", ") + ".",
	}
}

// errNoCode refuses a module whose source, which is file, compiled to no Lua: there is nothing to put in the
// bundle under its name. at is the require; the entry, which nothing requires, is refused at its own file. The
// hint is true of every such source: a file without text, a file of comments, and a file that only defines
// macros, which are for the compiler alone.
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
