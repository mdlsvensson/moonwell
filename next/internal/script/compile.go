// Package script turns a project's source files into the Lua that goes into a map: it writes the macro module
// the sources import, finds the modules, compiles the YueScript ones, follows the requires from the entry, checks
// for globals nobody defines, renders the bundle, and plans the bundle's place at the end of the map's script.
//
// A compile is made from the outside world, an env.Env whose Root is the project folder, and from an Input: a
// compiler, the entry, the libraries' folders, the lint block, the game's API (LoadNatives) and what the map's
// own script defines. It is two steps. CompileSources takes the Input and returns a Compiled: the modules, and
// the Lua of the libraries' ones. Link takes the Compiled and returns a Program: what the entry reaches, checked
// for unknown globals. Compile takes the Input and returns the Program, as the two steps in a row. Bundle takes
// a Program, the runtime and the line the bundle starts on, and returns the block of Lua. Inject takes a map
// folder and a Program, and returns one change: the map's script with the bundle after it. Collect, EntryName
// and RefreshMacros are steps of a compile that other packages take alone, and CollectLibraries is Collect for
// the libraries' modules alone.
//
// It knows nothing of where a library comes from, of manifests beyond the lint block, or of how a map is built.
// Of a map it knows one file, war3map.lua: what it defines, which a compile is handed, and its bytes, which
// Inject reads through the folder. It writes the macro module and what a compile leaves below dist/stage/lua, and
// no file of a map.
//
// Of Moonwell it imports manifest, mapdir, war3/lua, env, diag, fsx and the root package, for the files the
// program carries: the macro module, the game's API and the runtime.
package script

import (
	"context"
	"errors"
	"maps"

	"github.com/mdlsvensson/moonwell/next/internal/diag"
	"github.com/mdlsvensson/moonwell/next/internal/env"
	"github.com/mdlsvensson/moonwell/next/internal/manifest"
	"github.com/mdlsvensson/moonwell/next/internal/war3/lua"
)

// Input is what a compile is made from.
type Input struct {
	Compiler  string // the compiler to run, as its path
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

	lua map[string]string // the Lua of each YueScript module that the compile read and that has some, by its path
}

// Lua returns a module as Lua; ok is false for a module that has none.
//
// A Lua module is its own text, whichever module it is. A YueScript module is what the compiler wrote for it,
// for the modules whose Lua the compile read: those the entry reaches, and every module of a library, reached or
// not. For any other YueScript module ok is false: for a module of the project that the entry does not reach,
// for a module that is not among Sources, and for a source without code, which the compiler writes no Lua for.
func (p *Program) Lua(source Source) (text string, ok bool) {
	if source.Kind == Lua {
		return source.Text, true
	}
	text, ok = p.lua[source.Path]
	return text, ok
}

// Compiled is a project's modules with their YueScript compiled, before anything follows a require: what
// CompileSources returns and Link is handed. It is there when Link fails, so what needs the modules and not the
// program, such as an editor's view of the libraries, is made from it between the two.
type Compiled struct {
	Sources []Source // every module found, as Collect lists them

	in     Input             // what it was compiled from: Link goes on from the same Input
	search macros            // how the compiler finds the macro module
	output *staged           // what the compile left in dist/stage/lua
	lua    map[string]string // the Lua of each YueScript module of a library that has some, by its path
	none   map[string]bool   // the YueScript modules of a library that have none, by the same path
}

// Lua returns a module as Lua; ok is false for a module that has none.
//
// A Lua module is its own text, whichever module it is. A YueScript module of a library is what the compiler
// wrote for it, whether the entry reaches it or not. For any other YueScript module ok is false: for a module of
// the project's own, whose Lua is read when Link follows the requires to it, for a module that is not among
// Sources, and for a source without code, which the compiler writes no Lua for. Nothing is read, and a Link
// changes no answer: for a module of a library, the Program of that Link answers the same.
func (c *Compiled) Lua(source Source) (text string, ok bool) {
	if source.Kind == Lua {
		return source.Text, true
	}
	text, ok = c.lua[source.Path]
	return text, ok
}

// Compile is CompileSources and then Link: it writes the macro module, finds the modules, compiles them, follows
// the requires from the entry and checks the modules it reaches for unknown globals. With
// lint.unknownGlobals = "error" an unknown global fails with every one listed; with "warning" they are logged
// and returned in the Program.
//
// The steps are taken in that order, and the first fault ends the compile: a file the compiler refuses is
// reported before an entry that is no file of src/, that before a module that is not found, and an unknown
// global last. A caller that makes something of the modules whatever the entry reaches takes the two steps
// itself.
func Compile(ctx context.Context, e *env.Env, in Input) (*Program, error) {
	if in.Natives == nil {
		return nil, errNoNatives("Compile")
	}
	compiled, err := CompileSources(ctx, e, in)
	if err != nil {
		return nil, err
	}
	return Link(ctx, e, compiled)
}

// CompileSources is the first step of a compile: it writes the macro module, finds the modules, compiles every
// YueScript source that changed since the last compile, and reads the Lua of every YueScript module of a
// library.
//
// The first fault ends it, and a file the compiler refuses is one. What the second step refuses is not looked at
// here: neither the entry nor a require, nor a global. The libraries' Lua is read once the compile is over, so
// that Compiled.Lua has it without a read that could fail.
func CompileSources(ctx context.Context, e *env.Env, in Input) (*Compiled, error) {
	if in.Natives == nil {
		return nil, errNoNatives("CompileSources")
	}
	search, err := macroModule(e.Root)
	if err != nil {
		return nil, err
	}
	sources, err := Collect(e.Root, in.Libraries)
	if err != nil {
		return nil, err
	}
	output, err := compileAll(ctx, e, in.Compiler, in.Minify, search, sources)
	if err != nil {
		return nil, err
	}
	read := &reader{output: output, lua: map[string]string{}, none: map[string]bool{}}
	if err := read.libraries(sources); err != nil {
		return nil, err
	}
	return &Compiled{Sources: sources, in: in, search: search, output: output, lua: read.lua, none: read.none}, nil
}

// Link is the second step of a compile: it follows the requires from the entry, checks the modules it reaches
// for unknown globals, and returns the program. compiled is what CompileSources returned, and e the outside world
// it was handed; the entry, the lint block, the game's API and what the map's script defines are those of the
// Input that CompileSources took.
//
// The first fault ends it: an entry that is no file of src/ is reported before a module that is not found, and
// an unknown global last. With lint.unknownGlobals = "error" an unknown global fails with every one listed; with
// "warning" they are logged and returned in the Program. The Lua of the libraries' modules is what
// CompileSources read, and that of the project's modules is read as the requires are followed, so that
// Program.Lua has both without a read that could fail. compiled stays as it is, whether Link fails or not.
//
// Every Link reports what it finds: a second Link of one Compiled gives the same Program, and logs the warnings
// again. A caller that wants them logged once links once.
func Link(ctx context.Context, e *env.Env, compiled *Compiled) (*Program, error) {
	if compiled == nil || compiled.output == nil {
		// A plain error: the caller passes what CompileSources returned without a failure, which is never nil and
		// never a Compiled of the caller's own making, so a link of anything else is a mistake in Moonwell and
		// nothing the user can put right.
		return nil, errors.New("script.Link: compiled is not what script.CompileSources returned")
	}
	in, read := compiled.in, compiled.reader()
	entry, err := EntryName(in.Entry)
	if err != nil {
		return nil, err
	}
	modules, err := reached(entry, loaderOf(compiled.Sources, read.luaOf))
	if err != nil {
		return nil, err
	}
	unknown, err := unknownGlobals(ctx, e, in, compiled.search, compiled.output, modules)
	if err != nil {
		return nil, err
	}
	return &Program{
		Entry: entry, Modules: modules, Minify: in.Minify, Sources: compiled.Sources, Unknown: unknown, lua: read.lua,
	}, nil
}

// errNoNatives is the failure of a door, by its name, that is handed an Input without the game's API. It is a
// plain error: the caller passes LoadNatives(), which is never nil, so a compile without the game's API is a
// mistake in Moonwell and nothing the user can put right.
func errNoNatives(door string) error {
	return errors.New("script." + door + ": Input.Natives is nil; pass script.LoadNatives()")
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
	output *staged
	lua    map[string]string // the Lua of each module read that has some, by the source's path
	none   map[string]bool   // the modules read that have none, by the same path
}

// reader is a reader that has read what CompileSources read, and keeps what it reads from there on for itself:
// a Link reads the Lua of the project's modules, which is its Program's, and c stays as it is.
func (c *Compiled) reader() *reader {
	return &reader{output: c.output, lua: maps.Clone(c.lua), none: maps.Clone(c.none)}
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
