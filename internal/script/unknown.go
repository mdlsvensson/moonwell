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

func unknownGlobals(
	ctx context.Context, e *env.Env, in Input, m macros, output *staged, modules []Module,
) ([]diag.Problem, error) {
	uses, err := listUses(ctx, e, in.Compiler, m, output.macroSources, checkedAmong(modules, output))
	if err != nil {
		return nil, err
	}
	known := knownGlobals(in.Natives, in.Map, declaredBy(modules, output), in.Lint.Globals)
	problems := unknownGlobalProblems(uses, known, in.Natives.Lua.Removed)
	if len(problems) == 0 {
		return nil, nil
	}
	if in.Lint.UnknownGlobals == "error" {
		return nil, diag.Problems(problems)
	}
	warnOf(e.Log, problems)
	return problems, nil
}

func warnOf(log *env.Logger, problems []diag.Problem) {
	for _, problem := range problems[:min(len(problems), diag.MaxProblems)] {
		log.Warn(diag.FormatProblem(problem))
	}
	if more := len(problems) - diag.MaxProblems; more > 0 {
		log.Warn(moreUnknown(more))
	}
}

func declaredBy(modules []Module, output *staged) []string {
	var declared []string
	for _, module := range modules {
		if module.Kind == Lua {
			declared = append(declared, lua.TopLevelGlobals(module.Lua)...)
		} else {
			declared = append(declared, declaredGlobals(output.texts[module.Path])...)
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

func declaredGlobals(source string) []string {
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
	hint := hints{known: known, removed: removed, made: map[string]string{}}
	var problems []diag.Problem
	for file, used := range uses {
		for _, use := range used {
			if !known[use.Name] {
				problems = append(problems, unknownGlobal(file, use, hint.of(use.Name)))
			}
		}
	}
	slices.SortFunc(problems, func(a, b diag.Problem) int {
		return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column), strings.Compare(a.Msg, b.Msg))
	})
	return problems
}

type hints struct {
	known   map[string]bool
	removed []string
	names   []string
	made    map[string]string
}

func (h *hints) of(name string) string {
	hint, made := h.made[name]
	if !made {
		hint = h.worded(name)
		h.made[name] = hint
	}
	return hint
}

func (h *hints) worded(name string) string {
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

type checked struct {
	path string
	hash string
}

func checkedAmong(modules []Module, output *staged) []checked {
	byPath := map[string]checked{}
	for _, module := range modules {
		if hash, hashed := output.hashes[module.Path]; hashed && module.Kind == Yue && module.Library == "" {
			byPath[module.Path] = checked{path: module.Path, hash: hash}
		}
	}
	sources := make([]checked, 0, len(byPath))
	for _, path := range slices.Sorted(maps.Keys(byPath)) {
		sources = append(sources, byPath[path])
	}
	return sources
}

type listed struct {
	source  checked
	uses    []globalUse
	failure *diag.Error
}

func listUses(ctx context.Context, e *env.Env, yue string, m macros, macroSources string, sources []checked) (map[string][]globalUse, error) {
	now := listedWith{Compiler: yue, Macros: m.hash, MacroSources: macroSources}
	kept, err := readUses(e.Root, now)
	if err != nil {
		return nil, err
	}
	lists, stale := keptAndStale(kept, sources)
	with := compiler{ctx: ctx, run: e.Run, program: yue, search: m.path}
	results, err := eachOf(stale, func(source checked) (listed, error) { return with.list(e.Root, source) })
	if err != nil {
		return nil, err
	}
	var failures []*diag.Error
	for _, result := range results {
		if result.failure != nil {
			failures = append(failures, result.failure)
		} else {
			lists[result.source.path] = sourceUses{Hash: result.source.hash, Uses: result.uses}
		}
	}
	if err := writeUses(e.Root, now, lists); err != nil {
		return nil, err
	}
	if len(failures) > 0 {
		return nil, firstByPath(failures)
	}
	return usesOf(lists), nil
}

func keptAndStale(kept map[string]sourceUses, sources []checked) (lists map[string]sourceUses, stale []checked) {
	lists = map[string]sourceUses{}
	for _, source := range sources {
		if last, found := kept[source.path]; found && last.Hash == source.hash {
			lists[source.path] = last
		} else {
			stale = append(stale, source)
		}
	}
	return lists, stale
}

func usesOf(lists map[string]sourceUses) map[string][]globalUse {
	uses := make(map[string][]globalUse, len(lists))
	for path, list := range lists {
		uses[path] = list.Uses
	}
	return uses
}

func (c compiler) list(root string, source checked) (listed, error) {
	file := filepath.Join(root, filepath.FromSlash(source.path))
	result, err := c.run(c.ctx, c.program, []string{"-g", "--path", c.search, file}, env.RunOptions{})
	if err != nil {
		return listed{}, err
	}
	if result.ExitCode != 0 {
		return listed{source: source, failure: compileError(source.path, result.Stdout+"\n"+result.Stderr)}, nil
	}
	uses, failure := usesPrinted(result.Stdout, source.path)
	return listed{source: source, uses: uses, failure: failure}, nil
}

const unknownGlobalHint = "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl."

func unknownGlobal(file string, use globalUse, hint string) diag.Problem {
	return diag.Problem{File: file, Line: use.Line, Column: use.Column, Msg: "Unknown global " + use.Name + ".", Hint: hint}
}

func hintRemoved(name string) string {
	return "Warcraft III's Lua does not provide " + name + "."
}

func hintClose(closest []string) string {
	return "Did you mean " + diag.JoinWords(closest, "or", -1) + "? " + unknownGlobalHint
}

func moreUnknown(more int) string {
	return "and " + strconv.Itoa(more) + " more unknown global(s)"
}
