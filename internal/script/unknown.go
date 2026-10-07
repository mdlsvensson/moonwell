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
	"github.com/mdlsvensson/moonwell/internal/war3/lua"
)

// This file checks gameplay code for globals nobody defines: a misspelt native, or a variable that was never
// declared.

// unknownGlobals looks for unknown globals in the modules the entry reaches. Only the project's own YueScript
// is looked through, and only a module the entry reaches declares a global. With lint.unknownGlobals = "error"
// any unknown use fails, with a diag.Problems that holds all of them; else they are logged as warnings and
// returned.
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

// warnOf logs each problem as a warning, at most diag.MaxProblems of them, and then how many more there are.
func warnOf(log *env.Logger, problems []diag.Problem) {
	for _, problem := range problems[:min(len(problems), diag.MaxProblems)] {
		log.Warn(diag.FormatProblem(problem))
	}
	if more := len(problems) - diag.MaxProblems; more > 0 {
		log.Warn(moreUnknown(more))
	}
}

// ---- the names that are known ----

// declaredBy is the names the modules declare as globals: what the `global` lines of a YueScript module name,
// and what a Lua module defines at its top level. The YueScript texts are the ones the compile read and hashed:
// reading a file again could fail, or give other bytes.
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
	globalLine   = regexp.MustCompile(`^[ \t\n\v\f\r]*global[ \t\n\v\f\r]+([^\n\r]*)$`)
	constOrClass = regexp.MustCompile(`^(const|class)[ \t\n\v\f\r]+(.*)$`)
	leadingName  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
	wholeName    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// declaredGlobals lists the names a YueScript source's `global` lines declare: `global Score = 0`,
// `global a, b`, `global const K = 1`, `global class Boss`. `global *` and `global ^` name nothing, and a
// comment at the end of the line is dropped. A line ends at "\n" or "\r\n".
func declaredGlobals(source string) []string {
	var declared []string
	for _, line := range lineEnd.Split(source, -1) {
		global := globalLine.FindStringSubmatch(line)
		if global == nil {
			continue
		}
		rest, _, _ := strings.Cut(global[1], "--")
		rest = strings.Trim(rest, luaSpace)
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
			if name := strings.Trim(part, luaSpace); wholeName.MatchString(name) {
				declared = append(declared, name)
			}
		}
	}
	return declared
}

// knownGlobals is every name a gameplay file may use as a global: the game's functions and globals, the globals
// of Lua's standard library that the game keeps, what the source map's script defines (nil for a map without
// one), the declared names and the manifest's lint.globals.
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

// ---- the problems ----

// unknownGlobalProblems is one problem for each use of a name that is not known, sorted by file, line, column
// and then message, by bytes. uses is by the source's path from the project folder. removed lists the globals
// of Lua's standard library that the game takes away.
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
		// Two names at one place cannot be, but the order must not depend on that of a map.
		return cmp.Or(strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Column, b.Column), strings.Compare(a.Msg, b.Msg))
	})
	return problems
}

// hints makes the hint of each unknown name, once: finding the names that are close to one compares it with
// every known name.
type hints struct {
	known   map[string]bool
	removed []string
	names   []string          // the known names, each once and sorted; made when the first hint needs them
	made    map[string]string // by the unknown name
}

// of is the hint for an unknown name.
func (h *hints) of(name string) string {
	hint, made := h.made[name]
	if !made {
		hint = h.worded(name)
		h.made[name] = hint
	}
	return hint
}

// worded is the hint for an unknown name: the game's own for a global the game removes; else up to three known
// names that are close to it, before how a name is made known; else that alone.
//
// The close names are found among the known names in sorted order, each once: so the hint is the same whatever
// order a map gives its keys in, and no name is suggested twice.
func (h *hints) worded(name string) string {
	if slices.Contains(h.removed, name) {
		return hintRemoved(name)
	}
	if h.names == nil {
		h.names = slices.Sorted(maps.Keys(h.known))
	}
	if closest := diag.Closest(h.names, name, 3); len(closest) > 0 {
		return hintClose(closest)
	}
	return unknownGlobalHint
}

// ---- the globals each source uses ----

// checked is a source whose uses of globals are listed.
type checked struct {
	path string // from the project folder, with "/"
	hash string // the SHA-256 of the bytes the compile read
}

// checkedAmong is the sources to list among the modules the entry reaches: the project's own YueScript, each
// once, in the order of their paths. A library's modules are the library's to check, and a Lua module is not
// looked through.
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

// listed is how the listing of one source ended: with its uses, or with the failure of a source the compiler
// refused or printed something else for.
type listed struct {
	source  checked
	uses    []globalUse
	failure *diag.Error
}

// listUses gives the globals each source uses, by the source's path. The compiler lists the uses of a source
// (`yue -g`), which it cannot do in the run that compiles it; it is run only for the sources whose text changed
// since the last check, at most atOnce at a time, and for every source when the compiler, the macro module or
// the sources that may define macros changed. macroSources is what the compile hashed those sources to: a macro
// may give a source the globals it uses.
//
// A source the compiler refuses does not stop the others: the lists that were made are kept, and the failure
// returned is that of the first such source by its path. Any other failure (a compiler that cannot be started,
// a cancelled context) is returned as it is, and then nothing is kept.
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

// keptAndStale parts the sources into those whose list of uses is kept, with those lists, and those that are to
// be listed: a source without a list, or whose text is not the one its list was made of.
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

// usesOf is the uses of each source that has a list.
func usesOf(lists map[string]sourceUses) map[string][]globalUse {
	uses := make(map[string][]globalUse, len(lists))
	for path, list := range lists {
		uses[path] = list.Uses
	}
	return uses
}

// list runs the compiler on one source of the project at root to list the globals it uses, with the macro
// search before the file. A run that fails is a file that does not compile, and is read as one.
func (c compiler) list(root string, source checked) (listed, error) {
	file := filepath.Join(root, filepath.FromSlash(source.path))
	result, err := c.run(c.ctx, c.program, []string{"-g", "--path", c.search, file}, env.RunOptions{})
	if err != nil {
		return listed{}, err
	}
	if result.Code != 0 {
		return listed{source: source, failure: compileError(source.path, result.Stdout+"\n"+result.Stderr)}, nil
	}
	uses, failure := usesPrinted(result.Stdout, source.path)
	return listed{source: source, uses: uses, failure: failure}, nil
}

// ---- errors ----

// The failures of a source whose uses cannot be listed are worded where what the compiler prints is read:
// compileError and errUnreadableUse, in printed.go.

// unknownGlobalHint says how to make a name known.
const unknownGlobalHint = "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl."

// unknownGlobal is the problem of one use of a name that is not known.
func unknownGlobal(file string, use globalUse, hint string) diag.Problem {
	return diag.Problem{File: file, Line: use.Line, Column: use.Column, Msg: "Unknown global " + use.Name + ".", Hint: hint}
}

// hintRemoved is the hint for a global of Lua's standard library that the game takes away.
func hintRemoved(name string) string {
	return "Warcraft III's Lua does not provide " + name + "."
}

// hintClose is the hint that names the known names close to an unknown one, nearest first, before how a name is
// made known.
func hintClose(closest []string) string {
	return "Did you mean " + diag.JoinWords(closest, "or", -1) + "? " + unknownGlobalHint
}

// moreUnknown is the line of the log that counts the unknown globals beyond those it shows.
func moreUnknown(more int) string {
	return "and " + strconv.Itoa(more) + " more unknown global(s)"
}
