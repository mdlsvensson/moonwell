package script

import (
	"cmp"
	"context"
	"maps"
	"path/filepath"
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
	ctx context.Context, e *env.Env, in Input, m macroFile, output *compileOutput, modules []Module,
) ([]diag.Problem, error) {
	uses, err := listGlobalUses(ctx, e, in.Compiler, m, output.macroSources, lintSources(modules, output))
	if err != nil {
		return nil, err
	}
	known := knownGlobals(in.Natives, in.Map, collectDeclaredGlobals(modules, output), in.Lint.Globals)
	problems := unknownGlobalProblems(uses, known, in.Natives.Lua.Removed)
	if len(problems) == 0 {
		return nil, nil
	}
	if in.Lint.UnknownGlobals == "error" {
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
			declared = append(declared, parseDeclaredGlobals(output.texts[module.Path])...)
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
	hint := hintBuilder{known: known, removed: removed, cache: map[string]string{}}
	var problems []diag.Problem
	for file, used := range uses {
		for _, use := range used {
			if !known[use.Name] {
				problems = append(problems, newUnknownGlobalProblem(file, use, hint.hintFor(use.Name)))
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
	hint, made := h.cache[name]
	if !made {
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

type lintSource struct {
	path string
	hash string
}

func lintSources(modules []Module, output *compileOutput) []lintSource {
	byPath := map[string]lintSource{}
	for _, module := range modules {
		if hash, hashed := output.hashes[module.Path]; hashed && module.Kind == Yue && module.Library == "" {
			byPath[module.Path] = lintSource{path: module.Path, hash: hash}
		}
	}
	sources := make([]lintSource, 0, len(byPath))
	for _, path := range slices.Sorted(maps.Keys(byPath)) {
		sources = append(sources, byPath[path])
	}
	return sources
}

type lintResult struct {
	source  lintSource
	uses    []globalUse
	diagErr *diag.Error
}

func listGlobalUses(ctx context.Context, e *env.Env, yue string, m macroFile, macroSources string, sources []lintSource) (map[string][]globalUse, error) {
	now := listedWith{Compiler: yue, Macros: m.hash, MacroSources: macroSources}
	kept, err := readUsesCache(e.Root, now)
	if err != nil {
		return nil, err
	}
	lists, stale := splitCachedUses(kept, sources)
	with := compiler{ctx: ctx, run: e.Run, program: yue, search: m.path}
	results, err := eachOf(stale, func(source lintSource) (lintResult, error) { return with.listUses(e.Root, source) })
	if err != nil {
		return nil, err
	}
	var failures []*diag.Error
	for _, result := range results {
		if result.diagErr != nil {
			failures = append(failures, result.diagErr)
		} else {
			lists[result.source.path] = cachedUses{Hash: result.source.hash, Uses: result.uses}
		}
	}
	if err := writeUsesCache(e.Root, now, lists); err != nil {
		return nil, err
	}
	if len(failures) > 0 {
		return nil, firstByPath(failures)
	}
	return flattenUses(lists), nil
}

func splitCachedUses(kept map[string]cachedUses, sources []lintSource) (lists map[string]cachedUses, stale []lintSource) {
	lists = map[string]cachedUses{}
	for _, source := range sources {
		if last, found := kept[source.path]; found && last.Hash == source.hash {
			lists[source.path] = last
		} else {
			stale = append(stale, source)
		}
	}
	return lists, stale
}

func flattenUses(lists map[string]cachedUses) map[string][]globalUse {
	uses := make(map[string][]globalUse, len(lists))
	for path, list := range lists {
		uses[path] = list.Uses
	}
	return uses
}

func (c compiler) listUses(root string, source lintSource) (lintResult, error) {
	file := filepath.Join(root, filepath.FromSlash(source.path))
	result, err := c.run(c.ctx, c.program, []string{"-g", "--path", c.search, file}, env.RunOptions{})
	if err != nil {
		return lintResult{}, err
	}
	if result.ExitCode != 0 {
		return lintResult{source: source, diagErr: newCompileError(source.path, result.Stdout+"\n"+result.Stderr)}, nil
	}
	uses, failure := parseGlobalUses(result.Stdout, source.path)
	return lintResult{source: source, uses: uses, diagErr: failure}, nil
}

const unknownGlobalHint = "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl."

func newUnknownGlobalProblem(file string, use globalUse, hint string) diag.Problem {
	return diag.Problem{File: file, Line: use.Line, Column: use.Column, Msg: "Unknown global " + use.Name + ".", Hint: hint}
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
