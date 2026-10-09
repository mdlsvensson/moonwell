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

	input  Input
	macros macroFile
	output *compileOutput
	lua    map[string]string
	noLua  map[string]bool
}

func (c *Compiled) Lua(source Source) (text string, ok bool) {
	if source.Kind == Lua {
		return source.Text, true
	}
	text, ok = c.lua[source.Path]
	return text, ok
}

func CompileSources(ctx context.Context, e *env.Env, input Input) (*Compiled, error) {
	if input.Natives == nil {
		return nil, errNoNatives()
	}
	macros, err := loadMacroModule(e.Root)
	if err != nil {
		return nil, err
	}
	sources, err := CollectSources(e.Root, input.Libraries)
	if err != nil {
		return nil, err
	}
	output, err := compileAll(ctx, e, input.Compiler, input.Minify, macros, sources)
	if err != nil {
		return nil, err
	}
	reader := &luaReader{output: output, lua: map[string]string{}, noLua: map[string]bool{}}
	if err := reader.readLibraries(sources); err != nil {
		return nil, err
	}
	return &Compiled{Sources: sources, input: input, macros: macros, output: output, lua: reader.lua, noLua: reader.noLua}, nil
}

func Link(ctx context.Context, e *env.Env, compiled *Compiled) (*Program, error) {
	if compiled == nil || compiled.output == nil {
		return nil, errors.New("script.Link: compiled is not what script.CompileSources returned")
	}
	input, reader := compiled.input, compiled.newLuaReader()
	entry, err := EntryName(input.Entry)
	if err != nil {
		return nil, err
	}
	modules, err := reachableModules(entry, newLoader(compiled.Sources, reader.readLua))
	if err != nil {
		return nil, err
	}
	unknown, err := findUnknownGlobals(ctx, e, input, compiled.macros, compiled.output, modules)
	if err != nil {
		return nil, err
	}
	return &Program{
		Entry: entry, Modules: modules, Minify: input.Minify, Sources: compiled.Sources, Unknown: unknown, lua: reader.lua,
	}, nil
}

func loadMacroModule(root string) (macroFile, error) {
	macros, err := readMacros(root)
	if err != nil {
		return macroFile{}, err
	}
	if _, err := RefreshMacros(root); err != nil {
		return macroFile{}, err
	}
	return macros, nil
}

type luaReader struct {
	output *compileOutput
	lua    map[string]string
	noLua  map[string]bool
}

func (c *Compiled) newLuaReader() *luaReader {
	return &luaReader{output: c.output, lua: maps.Clone(c.lua), noLua: maps.Clone(c.noLua)}
}

func (r *luaReader) readLua(source Source) (text string, ok bool, err error) {
	if text, ok = r.lua[source.Path]; ok || r.noLua[source.Path] {
		return text, ok, nil
	}
	if text, ok, err = r.output.readLua(source); err != nil {
		return "", false, err
	}
	if ok {
		r.lua[source.Path] = text
	} else {
		r.noLua[source.Path] = true
	}
	return text, ok, nil
}

func (r *luaReader) readLibraries(sources []Source) error {
	for _, source := range sources {
		if source.Library == "" || source.Kind != Yue {
			continue
		}
		if _, _, err := r.readLua(source); err != nil {
			return err
		}
	}
	return nil
}

func errNoNatives() error {
	return errors.New("script.CompileSources: Input.Natives is nil; pass script.LoadNatives()")
}
