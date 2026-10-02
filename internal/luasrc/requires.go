package luasrc

// Require is one call of the global `require`.
type Require struct {
	Line int
	// Name is the module name when Literal is true.
	Name string
	// Literal is false when the argument is not a single plain string literal; the bundler cannot follow it.
	Literal bool
}

// Requires finds calls to the global `require`, the only form Moonwell's bundler follows.
func Requires(source string) []Require {
	tokens := simpleTokens(source)
	var calls []Require
	at := func(i int) (simpleToken, bool) {
		if i < 0 || i >= len(tokens) {
			return simpleToken{}, false
		}
		return tokens[i], true
	}
	literal := func(token simpleToken, line int) Require {
		if token.escaped {
			return Require{Line: line}
		}
		return Require{Line: line, Name: token.value, Literal: true}
	}
	for i, token := range tokens {
		if token.kind != simpleName || token.value != "require" {
			continue
		}
		if previous, ok := at(i - 1); ok {
			if previous.isPunct(".") || previous.isPunct(":") {
				continue
			}
			if previous.kind == simpleName && (previous.value == "function" || previous.value == "local") {
				continue
			}
		}
		next, ok := at(i + 1)
		if !ok {
			continue
		}
		if next.kind == simpleString {
			calls = append(calls, literal(next, token.line))
		} else if next.isPunct("(") {
			argument, hasArgument := at(i + 2)
			closing, hasClosing := at(i + 3)
			if hasArgument && argument.kind == simpleString && hasClosing && closing.isPunct(")") {
				calls = append(calls, literal(argument, token.line))
			} else {
				calls = append(calls, Require{Line: token.line})
			}
		}
	}
	return calls
}
