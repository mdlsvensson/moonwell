package lua

import "slices"

// TopLevelGlobals returns the globals a Lua module defines at its top level: `function Name(` and `Name = ...` or
// `Name, Other = ...` without `local`, outside every function, block and bracket. A name the top level declares
// `local` anywhere (`local Timer` before `Timer = {}`) is the file's own, not a global. In order, without
// duplicates.
func TopLevelGlobals(source string) []string {
	tokens := simpleTokens(source)
	var defined []string
	locals := map[string]bool{}
	var open nesting
	for i, token := range tokens {
		if open.nothing() && token.Kind == NameToken {
			switch {
			case token.Raw == "local":
				for _, name := range localNames(tokens, i) {
					locals[name] = true
				}
			case token.Raw == "function" && !isWord(tokenAt(tokens, i-1), "local"):
				defined = append(defined, functionName(tokens, i)...)
			case startsStatement(tokens, i):
				defined = append(defined, assignedNames(tokens, i)...)
			}
		}
		open.step(token)
	}
	var globals []string
	for _, name := range defined {
		if !locals[name] && !slices.Contains(globals, name) {
			globals = append(globals, name)
		}
	}
	return globals
}

// nesting counts the blocks and the brackets that are open at a token. A word or a bracket that closes what was
// never opened is passed over.
type nesting struct{ blocks, brackets int }

func (n nesting) nothing() bool { return n.blocks == 0 && n.brackets == 0 }

// step counts what the token opens or closes. Every block ends with `end` or `until`, and the words below are the
// ones that need one: a `while` and a `for` have their `do`, and `then`, `elseif` and `else` share the `end` of
// their `if`.
func (n *nesting) step(token Token) {
	switch {
	case token.Kind == NameToken && slices.Contains([]string{"function", "do", "if", "repeat"}, token.Raw):
		n.blocks++
	case token.Kind == NameToken && (token.Raw == "end" || token.Raw == "until"):
		n.blocks = max(0, n.blocks-1)
	case token.is("(") || token.is("{") || token.is("["):
		n.brackets++
	case token.is(")") || token.is("}") || token.is("]"):
		n.brackets = max(0, n.brackets-1)
	}
}

func isName(token Token) bool { return token.Kind == NameToken && !keywords[token.Raw] }

// isWord reports whether the token is the keyword. A string that says the word is a string.
func isWord(token Token, word string) bool { return token.Kind == NameToken && token.Raw == word }

// startsStatement reports whether tokens[i] begins a statement, as far as a scanner can tell: it starts a line or
// follows `;`.
func startsStatement(tokens []Token, i int) bool {
	return i == 0 || tokens[i-1].Line < tokens[i].Line || tokens[i-1].is(";")
}

// functionName returns the name of `function Name(` at tokens[i], the `function`; none for a field, a method or a
// function without a name.
func functionName(tokens []Token, i int) []string {
	if isName(tokenAt(tokens, i+1)) && tokenAt(tokens, i+2).is("(") {
		return []string{tokens[i+1].Raw}
	}
	return nil
}

// assignedNames returns the names of `Name {, Name} =` at tokens[i]; none for anything else, a comparison (`==`)
// included.
func assignedNames(tokens []Token, i int) []string {
	var names []string
	for ; isName(tokenAt(tokens, i)); i += 2 {
		names = append(names, tokens[i].Raw)
		switch next := tokenAt(tokens, i+1); {
		case next.is(","):
		case next.is("=") && !tokenAt(tokens, i+2).is("="):
			return names
		default:
			return nil
		}
	}
	return nil
}

// localNames returns the names that the `local` at tokens[i] declares: `local Name {, Name}` or
// `local function Name`.
func localNames(tokens []Token, i int) []string {
	if isWord(tokenAt(tokens, i+1), "function") {
		if name := tokenAt(tokens, i+2); isName(name) {
			return []string{name.Raw}
		}
		return nil
	}
	var names []string
	for i++; isName(tokenAt(tokens, i)); i += 2 {
		names = append(names, tokens[i].Raw)
		if !tokenAt(tokens, i+1).is(",") {
			break
		}
	}
	return names
}
