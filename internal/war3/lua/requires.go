package lua

type Require struct {
	Line    int
	Name    string
	Literal bool
}

func FindRequires(source string) []Require {
	tokens := simpleTokens(source)
	var calls []Require
	for i, token := range tokens {
		if token.Kind != NameToken || token.Raw != "require" || isAnotherRequire(tokenAt(tokens, i-1)) {
			continue
		}
		literal, called := stringArgument(tokens[i+1:])
		switch {
		case !called:
		case literal == nil || literal.Escaped:
			calls = append(calls, Require{Line: token.Line})
		default:
			calls = append(calls, Require{Line: token.Line, Name: literal.Text, Literal: true})
		}
	}
	return calls
}

func isAnotherRequire(previous Token) bool {
	switch previous.Kind {
	case SymbolToken:
		return previous.Raw == "." || previous.Raw == ":"
	case NameToken:
		return previous.Raw == "function" || previous.Raw == "local"
	}
	return false
}

func stringArgument(after []Token) (literal *Token, called bool) {
	switch {
	case len(after) > 0 && after[0].Kind == StringToken:
		return &after[0], true
	case len(after) > 0 && after[0].isSymbol("("):
		if len(after) > 2 && after[1].Kind == StringToken && after[2].isSymbol(")") {
			return &after[1], true
		}
		return nil, true
	}
	return nil, false
}
