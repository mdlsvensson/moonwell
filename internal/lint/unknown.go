// Package lint checks gameplay code for globals nobody defines: a misspelt native, or a variable that was never
// declared.
package lint

import (
	"context"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mdlsvensson/moonwell/internal/diag"
	"github.com/mdlsvensson/moonwell/internal/editor"
	"github.com/mdlsvensson/moonwell/internal/logging"
	"github.com/mdlsvensson/moonwell/internal/luasrc"
	"github.com/mdlsvensson/moonwell/internal/names"
	"github.com/mdlsvensson/moonwell/internal/natives"
	"github.com/mdlsvensson/moonwell/internal/proc"
	"github.com/mdlsvensson/moonwell/internal/project"
	"github.com/mdlsvensson/moonwell/internal/text"
	"github.com/mdlsvensson/moonwell/internal/yue"
)

// UnknownGlobalHint says how to make a name known.
const UnknownGlobalHint = "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl."

var (
	lineBreak    = regexp.MustCompile(`\r?\n`)
	globalLine   = regexp.MustCompile(`^` + text.SpaceClass + `*global` + text.SpaceClass + `+(` + text.NotLineBreak + `*)$`)
	comment      = regexp.MustCompile(`--.*$`)
	constOrClass = regexp.MustCompile(`^(const|class)` + text.SpaceClass + `+(.*)$`)
	leadingName  = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*`)
	wholeName    = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

// DeclaredGlobals lists the names a source's `global` lines declare: `global Score = 0`, `global a, b`,
// `global const K = 1`, `global class Boss`. `global *` and `global ^` name nothing.
func DeclaredGlobals(source string) []string {
	var declared []string
	for _, line := range lineBreak.Split(source, -1) {
		match := globalLine.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		rest := text.Trim(comment.ReplaceAllString(match[1], ""))
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
		for _, part := range strings.Split(targets, ",") {
			if name := text.Trim(part); wholeName.MatchString(name) {
				declared = append(declared, name)
			}
		}
	}
	return declared
}

// KnownGlobals is every name a gameplay file may use as a global: the game's functions and globals, the Lua
// standard library the game keeps, what the source map's war3map.lua defines (nil when there is none), the declared
// names and the manifest's lint.globals.
func KnownGlobals(api *natives.Natives, mapGlobals *luasrc.MapGlobals, declared, extra []string) map[string]bool {
	known := map[string]bool{}
	for _, function := range api.Functions {
		known[function.Name] = true
	}
	for _, global := range api.Globals {
		known[global.Name] = true
	}
	for _, name := range api.Lua.Globals {
		known[name] = true
	}
	if mapGlobals != nil {
		for _, global := range mapGlobals.Globals {
			known[global.Name] = true
		}
		for _, function := range mapGlobals.Functions {
			known[function] = true
		}
	}
	for _, name := range declared {
		known[name] = true
	}
	for _, name := range extra {
		known[name] = true
	}
	return known
}

// UnknownGlobalProblems gives one problem per use of a name that is not known, sorted by file, line and column. uses
// is keyed by path under src/. removed lists the standard-library globals the game takes away.
func UnknownGlobalProblems(uses map[string][]yue.GlobalUse, known map[string]bool, removed []string) []diag.Problem {
	var knownNames []string
	hints := map[string]string{}
	hintFor := func(name string) string {
		if hint, made := hints[name]; made {
			return hint
		}
		hint := UnknownGlobalHint
		if slices.Contains(removed, name) {
			hint = "Warcraft III's Lua does not provide " + name + "."
		} else {
			if knownNames == nil {
				knownNames = make([]string, 0, len(known))
				for known := range known {
					knownNames = append(knownNames, known)
				}
			}
			if closest := names.Closest(knownNames, name, 3); len(closest) > 0 {
				hint = "Did you mean " + names.JoinWords(closest, "or", -1) + "? " + UnknownGlobalHint
			}
		}
		hints[name] = hint
		return hint
	}
	problems := []diag.Problem{}
	for file, fileUses := range uses {
		for _, use := range fileUses {
			if known[use.Name] {
				continue
			}
			problems = append(problems, diag.Problem{
				File:   "src/" + file,
				Line:   use.Line,
				Column: use.Column,
				Msg:    "Unknown global " + use.Name + ".",
				Hint:   hintFor(use.Name),
			})
		}
	}
	slices.SortStableFunc(problems, func(a, b diag.Problem) int {
		if order := text.Compare(a.File, b.File); order != 0 {
			return order
		}
		if a.Line != b.Line {
			return a.Line - b.Line
		}
		if a.Column != b.Column {
			return a.Column - b.Column
		}
		// Two names at one place cannot be, but the order must not depend on the map's.
		return text.Compare(a.Msg, b.Msg)
	})
	return problems
}

// Compiled is what the check needs of a compile.
type Compiled struct {
	// Yue is the path of the compiler.
	Yue string
	// Hashes are the sources to check, by path under src/; the pipeline passes the ones reachable from the entry.
	Hashes map[string]string
	Macros *yue.MacroSearch
	// DeclaredYue are YueScript texts whose `global` lines name known globals, and DeclaredLua are Lua texts whose
	// top-level globals do. The pipeline passes the modules reachable from the entry, so a declaration counts only in
	// a module the map requires.
	DeclaredYue []string
	DeclaredLua []string
}

// Check looks for unknown globals in the compiled sources. With lint.unknownGlobals = "error" any unknown use fails
// with a diag.Problems that lists all of them; with "warning" they are logged and returned. A nil api is the
// embedded one.
func Check(ctx context.Context, root string, run proc.RunFunc, log *logging.Logger, p *project.Project, compiled Compiled, api *natives.Natives) ([]diag.Problem, error) {
	uses, err := yue.ListGlobalUses(ctx, yue.UsesOptions{
		Yue: compiled.Yue, Root: root, Hashes: compiled.Hashes, Macros: compiled.Macros, Run: run,
	})
	if err != nil {
		return nil, err
	}
	// The texts are the ones the compile hashed: reading the files again could fail or see other bytes, during dev
	// for example.
	var declared []string
	for _, source := range compiled.DeclaredYue {
		declared = append(declared, DeclaredGlobals(source)...)
	}
	for _, source := range compiled.DeclaredLua {
		declared = append(declared, luasrc.TopLevelGlobals(source)...)
	}
	label := "maps/" + p.Map.Folder + "/war3map.lua"
	script, exists, err := editor.ReadSourceScript(filepath.Join(root, filepath.FromSlash(label)), label)
	if err != nil {
		return nil, err
	}
	var mapGlobals *luasrc.MapGlobals
	if exists {
		globals := luasrc.ReadMapGlobals(script)
		mapGlobals = &globals
	}
	if api == nil {
		api = natives.Load()
	}
	problems := UnknownGlobalProblems(uses, KnownGlobals(api, mapGlobals, declared, p.Lint.Globals), api.Lua.Removed)
	if len(problems) == 0 {
		return problems, nil
	}
	if p.Lint.UnknownGlobals == "error" {
		return nil, diag.Problems(problems)
	}
	for _, problem := range problems[:min(len(problems), diag.MaxProblems)] {
		log.Warn(diag.FormatProblem(problem))
	}
	if more := len(problems) - diag.MaxProblems; more > 0 {
		log.Warn("and " + strconv.Itoa(more) + " more unknown global(s)")
	}
	return problems, nil
}
