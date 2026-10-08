package lua

import "slices"

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

type nesting struct{ blocks, brackets int }

func (n nesting) nothing() bool { return n.blocks == 0 && n.brackets == 0 }

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

func isWord(token Token, word string) bool { return token.Kind == NameToken && token.Raw == word }

func startsStatement(tokens []Token, i int) bool {
	return i == 0 || tokens[i-1].Line < tokens[i].Line || tokens[i-1].is(";")
}

func functionName(tokens []Token, i int) []string {
	if isName(tokenAt(tokens, i+1)) && tokenAt(tokens, i+2).is("(") {
		return []string{tokens[i+1].Raw}
	}
	return nil
}

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
