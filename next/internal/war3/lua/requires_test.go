package lua

import (
	"slices"
	"strings"
	"testing"
)

func lit(line int, name string) Require { return Require{Line: line, Name: name, Literal: true} }

func TestRequires(t *testing.T) {
	for _, c := range []struct {
		name, source string
		want         []Require
	}{
		{
			"literal requires in every call form, with their lines",
			"local a = require(\"a.b\")\n\nlocal c = require \"c\"\nlocal d = require [[d.e]]\n",
			[]Require{lit(1, "a.b"), lit(3, "c"), lit(4, "d.e")},
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
			[]Require{lit(9, "yes")},
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
			"a concatenation before it is not a field access",
			`local s = "a" .. require("b")`,
			[]Require{lit(1, "b")},
		},
	} {
		if got := Requires(c.source); !slices.Equal(got, c.want) {
			t.Errorf("%s: Requires = %+v, want %+v", c.name, got, c.want)
		}
	}
}
