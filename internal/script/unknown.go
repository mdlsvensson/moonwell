package script

import (
	"cmp"
	"context"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/env"
	"github.com/mdlsvensson/moonwell/internal/fsx"
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

func findUnknownGlobals(
	ctx context.Context, e *env.Env, input Input, macros macroFile, output *compileOutput, modules []Module,
) ([]diag.Problem, error) {
	uses, err := listGlobalUses(ctx, e, input.Compiler, macros, output.macroSources, lintSources(modules, output))
	if err != nil {
		return nil, err
	}
	known := knownGlobals(input.Natives, input.Map, collectDeclaredGlobals(modules, output), input.Lint.Globals)
	problems := unknownGlobalProblems(uses, known, input.Natives.Lua.Removed)
	if len(problems) == 0 {
		return nil, nil
	}
	if input.Lint.UnknownGlobals == "error" {
		return nil, diag.Problems(problems)
	}
	logWarnings(e.Log, problems)
	return problems, nil
}

func logWarnings(log *env.Logger, problems []diag.Problem) {
	for _, problem := range problems[:min(len(problems), diag.MaxProblems)] {
		log.Warn(diag.FormatProblem(problem))
	}
	if more := len(problems) - diag.MaxProblems; more > 0 {
		log.Warn(formatMoreUnknown(more))
	}
}

func collectDeclaredGlobals(modules []Module, output *compileOutput) []string {
	var declared []string
	for _, module := range modules {
		if module.Kind == Lua {
			declared = append(declared, lua.FindTopLevelGlobals(module.Lua)...)
		} else {
			declared = append(declared, parseDeclaredGlobals(output.sourceTexts[module.Path])...)
		}
	}
	return declared
}

var (
	globalLine   = regexp.MustCompile(`^` + space + `*global` + space + `+([^\n\r]*)$`)
	constOrClass = regexp.MustCompile(`^(const|class)` + space + `+(.*)$`)
	leadingName  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
	wholeName    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func parseDeclaredGlobals(source string) []string {
	var declared []string
	for _, line := range lineEnd.Split(source, -1) {
		global := globalLine.FindStringSubmatch(line)
		if global == nil {
			continue
		}
		rest, _, _ := strings.Cut(global[1], "--")
		rest = fsx.TrimASCIISpace(rest)
		if keyword := constOrClass.FindStringSubmatch(rest); keyword != nil {
			if keyword[1] == "class" {
				if name := leadingName.FindString(keyword[2]); name != "" {
					declared = append(declared, name)
				}
				continue
			}
			rest = keyword[2]
		}
		targets, _, _ := strings.Cut(rest, "=")
		for part := range strings.SplitSeq(targets, ",") {
			if name := fsx.TrimASCIISpace(part); wholeName.MatchString(name) {
				declared = append(declared, name)
			}
		}
	}
	return declared
}

func knownGlobals(natives *Natives, mapGlobals *lua.MapGlobals, declared, extra []string) map[string]bool {
	known := map[string]bool{}
	for _, function := range natives.Functions {
		known[function.Name] = true
	}
	for _, global := range natives.Globals {
		known[global.Name] = true
	}
	if mapGlobals != nil {
		for _, global := range mapGlobals.Globals {
			known[global.Name] = true
		}
		for _, function := range mapGlobals.Functions {
			known[function] = true
		}
	}
	for _, names := range [][]string{natives.Lua.Globals, declared, extra} {
		for _, name := range names {
			known[name] = true
		}
	}
	return known
}

func unknownGlobalProblems(uses map[string][]globalUse, known map[string]bool, removed []string) []diag.Problem {
	hints := hintBuilder{known: known, removed: removed, cache: map[string]string{}}
	var problems []diag.Problem
	for path, fileUses := range uses {
		for _, use := range fileUses {
			if !known[use.Name] {
				problems = append(problems, newUnknownGlobalProblem(path, use, hints.hintFor(use.Name)))
			}
		}
	}
	slices.SortFunc(problems, func(a, b diag.Problem) int {
		return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column), strings.Compare(a.Msg, b.Msg))
	})
	return problems
}

type hintBuilder struct {
	known   map[string]bool
	removed []string
	names   []string
	cache   map[string]string
}

func (h *hintBuilder) hintFor(name string) string {
	hint, ok := h.cache[name]
	if !ok {
		hint = h.buildHint(name)
		h.cache[name] = hint
	}
	return hint
}

func (h *hintBuilder) buildHint(name string) string {
	if slices.Contains(h.removed, name) {
		return hintRemoved(name)
	}
	if h.names == nil {
		h.names = slices.Sorted(maps.Keys(h.known))
	}
	if closest := diag.ClosestNames(h.names, name, 3); len(closest) > 0 {
		return hintClose(closest)
	}
	return unknownGlobalHint
}

const unknownGlobalHint = "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl."

func newUnknownGlobalProblem(path string, use globalUse, hint string) diag.Problem {
	return diag.Problem{File: path, Line: use.Line, Column: use.Column, Msg: "Unknown global " + use.Name + ".", Hint: hint}
}

func hintRemoved(name string) string {
	return "Warcraft III's Lua does not provide " + name + "."
}

func hintClose(closest []string) string {
	return "Did you mean " + diag.JoinWords(closest, "or", -1) + "? " + unknownGlobalHint
}

func formatMoreUnknown(more int) string {
	return "and " + strconv.Itoa(more) + " more unknown global(s)"
}
