package lua

import (
	"slices"
	"strings"
	"testing"
)

func literalRequire(line int, name string) Require {
	return Require{Line: line, Name: name, Literal: true}
}

func TestRequires(t *testing.T) {
	for _, c := range []struct {
		name, source string
		want         []Require
	}{
		{
			"literal requires in every call form, with their lines",
			"local a = require(\"a.b\")\n\nlocal c = require \"c\"\nlocal d = require [[d.e]]\n",
			[]Require{literalRequire(1, "a.b"), literalRequire(3, "c"), literalRequire(4, "d.e")},
		},
		{
			"a require inside a comment or a string is none",
			strings.Join([]string{
				`-- require("nope")`,
				`--[[ require('nope')`,
				`]]`,
				`--[==[ require("nope") ]==]`,
				`local s = "require('nope')"`,
				`local t = [[`,
				`require("nope")`,
				`]]`,
				`local real = require("yes")`,
			}, "\n"),
			[]Require{literalRequire(9, "yes")},
		},
		{
			"an argument that is not one literal is not followed",
			"local m = require(prefix .. \"x\")\nlocal n = require(\"a\" .. b)",
			[]Require{{Line: 1}, {Line: 2}},
		},
		{
			"a literal with an escape is not followed",
			`require("a\65")`,
			[]Require{{Line: 1}},
		},
		{
			"a method, a field and a redefinition are not the global",
			"obj:require(\"x\")\nobj.require(\"y\")\nlocal function require(n) end\nlocal require = f\nf(require)",
			nil,
		},
		{
			"a require after a word that declares nothing is the global",
			"return require(\"m\")\nx = y and require \"n\"",
			[]Require{literalRequire(1, "m"), literalRequire(2, "n")},
		},
		{"a function named require that starts the source", "function require(name) end", nil},
		{"a local named require, whatever follows it", "local require(\"m\")", nil},
		{"a require that ends the source is not called", "x = require", nil},
		{"a require whose bracket ends the source is called without a string", "require(", []Require{{Line: 1}}},
		{"a require whose string ends the source is called without a string", `require("m"`, []Require{{Line: 1}}},
		{
			"a concatenation before it is not a field access",
			`local s = "a" .. require("b")`,
			[]Require{literalRequire(1, "b")},
		},
	} {
		if got := FindRequires(c.source); !slices.Equal(got, c.want) {
			t.Errorf("%s: Requires = %+v, want %+v", c.name, got, c.want)
		}
	}
}
