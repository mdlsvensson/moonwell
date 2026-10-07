package lua

// Require is one call of the global `require`.
type Require struct {
	Line int
	// Name is the module name when Literal is true.
	Name string
	// Literal is false when the argument is not one plain string literal: nothing can follow it without running the
	// source.
	Literal bool
}

// Requires finds the calls of the global `require`: `require "name"`, `require [[name]]` and `require(...)`. A
// `require` that is passed on or assigned without being called is none.
func Requires(source string) []Require {
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

// isAnotherRequire reports whether the token before a `require` makes it something else than the global: a field
// or a method of a value, or a function or a local that the source declares under that name.
func isAnotherRequire(previous Token) bool {
	switch previous.Kind {
	case SymbolToken:
		return previous.Raw == "." || previous.Raw == ":"
	case NameToken:
		return previous.Raw == "function" || previous.Raw == "local"
	}
	return false
}

// stringArgument looks at the tokens after a `require`. called is false when they do not call it. The literal is
// the string it is called with, or nil when its arguments are anything but one string.
func stringArgument(after []Token) (literal *Token, called bool) {
	switch {
	case len(after) > 0 && after[0].Kind == StringToken:
		return &after[0], true
	case len(after) > 0 && after[0].is("("):
		if len(after) > 2 && after[1].Kind == StringToken && after[2].is(")") {
			return &after[1], true
		}
		return nil, true
	}
	return nil, false
}
