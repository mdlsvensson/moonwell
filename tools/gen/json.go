package main

import (
	"strings"

	"github.com/mdlsvensson/moonwell/internal/fsx"
)

type jsonMember struct{ key, value string }

func jsonObject(members ...jsonMember) string {
	entries := make([]string, len(members))
	for i, m := range members {
		entries[i] = fsx.QuoteJSON(m.key) + ": " + m.value
	}
	return jsonBlock("{", entries, "}")
}

func jsonList[T any](items []T, render func(T) string) string {
	entries := make([]string, len(items))
	for i, item := range items {
		entries[i] = render(item)
	}
	return jsonBlock("[", entries, "]")
}

func jsonBlock(opening string, entries []string, closing string) string {
	if len(entries) == 0 {
		return opening + closing
	}
	return opening + "\n  " + strings.ReplaceAll(strings.Join(entries, ",\n"), "\n", "\n  ") + "\n" + closing
}
