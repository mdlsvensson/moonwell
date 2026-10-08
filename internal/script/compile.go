package script

import (
	"context"
	"errors"
	"maps"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/manifest"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

type Input struct {
	Compiler  string
	Entry     string
	Minify    bool
	Libraries []Library
	Lint      manifest.Lint
	Map       *lua.MapGlobals
	Natives   *Natives
}

type Module struct {
	Name    string
	Path    string
	Kind    Kind
	Library string
	Lua     string
}

type Program struct {
	Entry   string
	Modules []Module
	Minify  bool
	Sources []Source
	Unknown []diag.Problem

	lua map[string]string
}

func (p *Program) Lua(source Source) (text string, ok bool) {
	if source.Kind == Lua {
		return source.Text, true
	}
	text, ok = p.lua[source.Path]
	return text, ok
}

type Compiled struct {
	Sources []Source

	in     Input
	search macros
	output *staged
	lua    map[string]string
	none   map[string]bool
}

func (c *Compiled) Lua(source Source) (text string, ok bool) {
	if source.Kind == Lua {
		return source.Text, true
	}
	text, ok = c.lua[source.Path]
	return text, ok
}

func CompileSources(ctx context.Context, e *env.Env, in Input) (*Compiled, error) {
	if in.Natives == nil {
		return nil, errNoNatives()
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

func Link(ctx context.Context, e *env.Env, compiled *Compiled) (*Program, error) {
	if compiled == nil || compiled.output == nil {
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

type reader struct {
	output *staged
	lua    map[string]string
	none   map[string]bool
}

func (c *Compiled) reader() *reader {
	return &reader{output: c.output, lua: maps.Clone(c.lua), none: maps.Clone(c.none)}
}

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

func errNoNatives() error {
	return errors.New("script.CompileSources: Input.Natives is nil; pass script.LoadNatives()")
}
