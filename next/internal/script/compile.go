// Package script turns a project's source files into the Lua that goes into a map: it finds the modules, compiles
// the YueScript ones, follows the requires from the entry, checks for globals nobody defines, and writes the
// bundle.
//
// It takes a compiler, the libraries' folders and what the map's own script defines, and returns a Program; Inject
// places a Program in a map folder as a change.
//
// It knows nothing of libraries' sources, of manifests beyond the lint block, or of how a map is built.
//
// Of Moonwell it imports manifest, mapdir, war3/lua, env, diag, fsx and the root package, for the files the
// program carries.
package script

import (
	"context"
	"errors"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/war3/lua"
)

// Input is what a compile is made from.
type Input struct {
	Yue       string // the compiler to run
	Entry     string // the entry file, from the project folder, such as "src/main.yue"
	Minify    bool
	Libraries []Library // in key order
	Lint      manifest.Lint
	Map       *lua.MapGlobals // what the source map's war3map.lua defines; nil for a map without a script
	Natives   *Natives        // the game's API: LoadNatives()
}

// Module is a module as Lua, ready for the bundle.
type Module struct {
	Name    string // the name it is required by, which is the name the bundle defines it with
	Path    string // the file the user wrote, from the project folder
	Kind    Kind   // Lua for a Lua module, which a minified bundle keeps line for line
	Library string // the library's key; "" for the project's own
	// Lua is a Lua module's text, or what the compiler wrote for a YueScript module. It is bytes: nothing is
	// decoded, so it may hold bytes that are not UTF-8.
	Lua string
}

// Program is a project's gameplay as Lua.
type Program struct {
	Entry   string         // the entry module's name
	Modules []Module       // what the entry reaches, each after the modules it requires
	Minify  bool           // whether the YueScript modules are compiled minified
	Sources []Source       // every module found, reached or not
	Unknown []diag.Problem // unknown globals that lint.unknownGlobals = "warning" let pass

	lua map[string]string // the Lua of each YueScript module that Compile read and that has some, by its path
}

// Lua returns a module as Lua; ok is false for a module that has none.
//
// A Lua module is its own text, whichever module it is. A YueScript module is what the compiler wrote for it,
// for the modules whose Lua Compile read: those the entry reaches, and every module of a library, reached or
// not. For any other YueScript module ok is false: for a module of the project that the entry does not reach,
// for a module that is not among Sources, and for a source without code, which the compiler writes no Lua for.
func (p *Program) Lua(source Source) (text string, ok bool) {
	if source.Kind == Lua {
		return source.Text, true
	}
	text, ok = p.lua[source.Path]
	return text, ok
}

// Compile writes the macro module, finds the modules, compiles them, follows the requires from the entry and
// checks the modules it reaches for unknown globals. With lint.unknownGlobals = "error" an unknown global fails
// with every one listed; with "warning" they are logged and returned in the Program.
//
// The steps are taken in that order, and the first fault ends the compile: a file the compiler refuses is
// reported before an entry that is no file of src/, that before a module that is not found, and an unknown
// global last. The Lua of every module of a library is read once the compile is over, and that of the modules
// the entry reaches as the requires are followed, so that Program.Lua has it without a read that could fail.
func Compile(ctx context.Context, e *env.Env, in Input) (*Program, error) {
	if in.Natives == nil {
		// A plain error: the caller passes LoadNatives(), which is never nil, so a compile without the game's API
		// is a mistake in Moonwell and nothing the user can put right.
		return nil, errors.New("script.Compile: Input.Natives is nil; pass script.LoadNatives()")
	}
	search, err := macroModule(e.Root)
	if err != nil {
		return nil, err
	}
	sources, err := Collect(e.Root, in.Libraries)
	if err != nil {
		return nil, err
	}
	output, err := compileAll(ctx, e, in.Yue, in.Minify, search, sources)
	if err != nil {
		return nil, err
	}
	read := &reader{output: output, lua: map[string]string{}, none: map[string]bool{}}
	if err := read.libraries(sources); err != nil {
		return nil, err
	}
	entry, err := EntryName(in.Entry)
	if err != nil {
		return nil, err
	}
	modules, err := reached(entry, loaderOf(sources, read.luaOf))
	if err != nil {
		return nil, err
	}
	unknown, err := unknownGlobals(ctx, e, in, search, output, modules)
	if err != nil {
		return nil, err
	}
	return &Program{Entry: entry, Modules: modules, Minify: in.Minify, Sources: sources, Unknown: unknown, lua: read.lua}, nil
}

// macroModule writes the macro module of the project at root, and returns how the compiler finds it. A project
// folder whose path the search cannot hold is refused before anything is written.
func macroModule(root string) (macros, error) {
	search, err := macrosOf(root)
	if err != nil {
		return macros{}, err
	}
	if _, err := RefreshMacros(root); err != nil {
		return macros{}, err
	}
	return search, nil
}

// reader reads the Lua of compiled modules, each once, and keeps what it read: a module that is asked for again,
// under another name or by the Program, is the Lua that was read first.
type reader struct {
	output *compiled
	lua    map[string]string // the Lua of each module read that has some, by the source's path
	none   map[string]bool   // the modules read that have none, by the same path
}

// luaOf is the Lua of a YueScript module; ok is false for a module without an output.
func (r *reader) luaOf(source Source) (text string, ok bool, err error) {
	if text, ok = r.lua[source.Path]; ok || r.none[source.Path] {
		return text, ok, nil
	}
	if text, ok, err = r.output.luaOf(source); err != nil {
		return "", false, err
	}
	if ok {
		r.lua[source.Path] = text
	} else {
		r.none[source.Path] = true
	}
	return text, ok, nil
}

// libraries reads the Lua of every YueScript module of a library, whether the entry reaches it or not: a view
// of a library for an editor holds all of its modules.
func (r *reader) libraries(sources []Source) error {
	for _, source := range sources {
		if source.Library == "" || source.Kind != Yue {
			continue
		}
		if _, _, err := r.luaOf(source); err != nil {
			return err
		}
	}
	return nil
}
