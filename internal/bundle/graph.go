package bundle

import (
	"slices"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/luasrc"
)

// ResolveGraph walks the literal requires from entry and returns the modules it reaches, each after the modules it
// requires. load gives nil for a name no module has.
func ResolveGraph(entry string, load func(name string) (*CompiledModule, error), builtins []string) ([]CompiledModule, error) {
	var ordered []CompiledModule
	const visiting, done = 1, 2
	state := map[string]int{}
	var stack []string

	var visit func(name string, from *CompiledModule, line int) error
	visit = func(name string, from *CompiledModule, line int) error {
		if slices.Contains(builtins, name) || state[name] == done {
			return nil
		}
		fail := func(message, hint string) error {
			failure := &diag.Error{Msg: message, Hint: hint}
			if from != nil {
				failure.File, failure.Line = from.SourcePath, line
			}
			return failure
		}
		if state[name] == visiting {
			cycle := append(slices.Clone(stack[slices.Index(stack, name):]), name)
			return fail("Circular require: "+strings.Join(cycle, " → "), "Move the shared code into a module that both can require.")
		}
		module, err := load(name)
		if err != nil {
			return err
		}
		if module == nil {
			path := strings.ReplaceAll(name, ".", "/")
			return fail("Module '"+name+"' not found.",
				"Expected src/"+path+".yue, lua/"+path+".lua, lua/"+path+"/init.lua or a module of a library in "+
					"moonwell.pkl. Built-in modules: "+strings.Join(builtins, ", ")+".")
		}
		state[name] = visiting
		stack = append(stack, name)
		for _, call := range luasrc.Requires(module.Source) {
			if !call.Literal {
				return &diag.Error{
					Msg:  "require must be called with a single string literal.",
					File: module.SourcePath,
					Line: call.Line,
					Hint: "Moonwell bundles modules at build time and cannot follow computed module names.",
				}
			}
			if err := visit(call.Name, module, call.Line); err != nil {
				return err
			}
		}
		stack = stack[:len(stack)-1]
		state[name] = done
		ordered = append(ordered, *module)
		return nil
	}

	if err := visit(entry, nil, 0); err != nil {
		return nil, err
	}
	return ordered, nil
}
