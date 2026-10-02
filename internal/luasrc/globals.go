package luasrc

import "slices"

// TopLevelGlobals returns the globals a Lua module defines at its top level: `function Name(` and `Name = ...` or
// `Name, Other = ...` without `local`, outside every function, block and bracket. A name the top level declares
// `local` anywhere (`local Timer` before `Timer = {}`) is the file's own, not a global. In order, without
// duplicates.
func TopLevelGlobals(source string) []string {
	tokens := simpleTokens(source)
	get := func(i int) simpleToken {
		if i < 0 || i >= len(tokens) {
			return simpleToken{kind: simplePunct} // matches nothing
		}
		return tokens[i]
	}
	isName := func(i int) bool {
		token := get(i)
		return token.kind == simpleName && !keywords[token.value]
	}
	// startsStatement reports whether tokens[i] begins a statement: it starts a line or follows `;`.
	startsStatement := func(i int) bool {
		if i == 0 {
			return true
		}
		previous := tokens[i-1]
		return previous.line < tokens[i].line || previous.isPunct(";")
	}
	// assignedNames returns the names of `Name {, Name} =` (not `==`) starting at i; none for anything else.
	assignedNames := func(i int) []string {
		var names []string
		for j := i; isName(j); j += 2 {
			names = append(names, tokens[j].value)
			if get(j + 1).isPunct(",") {
				continue
			}
			if get(j+1).isPunct("=") && !get(j+2).isPunct("=") {
				return names
			}
			return nil
		}
		return nil
	}
	// localNames returns the names `local Name {, Name}` or `local function Name` at i (the `local`) declares.
	localNames := func(i int) []string {
		if next := get(i + 1); next.kind != simplePunct && next.value == "function" {
			if isName(i + 2) {
				return []string{tokens[i+2].value}
			}
			return nil
		}
		var names []string
		for j := i + 1; isName(j); j += 2 {
			names = append(names, tokens[j].value)
			if !get(j + 1).isPunct(",") {
				break
			}
		}
		return names
	}

	var names []string
	locals := map[string]bool{}
	blocks, brackets := 0, 0
	for i, token := range tokens {
		topLevel := blocks == 0 && brackets == 0
		switch token.kind {
		case simpleName:
			previous := get(i - 1)
			switch {
			case topLevel && token.value == "local":
				for _, name := range localNames(i) {
					locals[name] = true
				}
			case topLevel && token.value == "function" && !(i > 0 && previous.value == "local"):
				if isName(i+1) && get(i+2).isPunct("(") {
					names = append(names, tokens[i+1].value)
				}
			case topLevel && isName(i) && startsStatement(i):
				names = append(names, assignedNames(i)...)
			}
			switch token.value {
			case "function", "do", "if", "repeat":
				blocks++
			case "end", "until":
				blocks = max(0, blocks-1)
			}
		case simplePunct:
			switch token.value {
			case "(", "{", "[":
				brackets++
			case ")", "}", "]":
				brackets = max(0, brackets-1)
			}
		}
	}

	var globals []string
	for _, name := range names {
		if !locals[name] && !slices.Contains(globals, name) {
			globals = append(globals, name)
		}
	}
	return globals
}
