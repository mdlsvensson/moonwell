package lua

import (
	"strings"
	"unicode/utf8"
)

func simpleTokens(source string) []Token {
	tokens, _ := Tokenize(source)
	simple := make([]Token, 0, len(tokens))
	for _, token := range tokens {
		switch {
		case token.Kind == NumberToken:
		case token.Kind == SymbolToken && strings.Trim(token.Raw, ".") != "":
			simple = append(simple, splitSymbol(token)...)
		default:
			simple = append(simple, token)
		}
	}
	return simple
}

func splitSymbol(symbol Token) []Token {
	var split []Token
	for offset := 0; offset < len(symbol.Raw); {
		_, size := utf8.DecodeRuneInString(symbol.Raw[offset:])
		piece := symbol
		piece.Raw = symbol.Raw[offset : offset+size]
		piece.Text = piece.Raw
		piece.Start = symbol.Start + offset
		piece.End = piece.Start + size
		split = append(split, piece)
		offset += size
	}
	return split
}

func tokenAt(tokens []Token, i int) Token {
	if i < 0 || i >= len(tokens) {
		return Token{Kind: SymbolToken}
	}
	return tokens[i]
}
