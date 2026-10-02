package luasrc

import (
	"slices"
	"strings"
	"testing"
)

func lit(line int, name string) Require { return Require{Line: line, Name: name, Literal: true} }

func TestFindsLiteralRequiresInAllCallFormsWithLineNumbers(t *testing.T) {
	source := "local a = require(\"a.b\")\n\nlocal c = require \"c\"\nlocal d = require [[d.e]]\n"
	want := []Require{lit(1, "a.b"), lit(3, "c"), lit(4, "d.e")}
	if got := Requires(source); !slices.Equal(got, want) {
		t.Errorf("Requires = %+v, want %+v", got, want)
	}
}

func TestIgnoresRequireInsideCommentsAndStrings(t *testing.T) {
	source := strings.Join([]string{
		`-- require("nope")`,
		`--[[ require('nope')`,
		`]]`,
		`--[==[ require("nope") ]==]`,
		`local s = "require('nope')"`,
		`local t = [[`,
		`require("nope")`,
		`]]`,
		`local real = require("yes")`,
	}, "\n")
	if got := Requires(source); !slices.Equal(got, []Require{lit(9, "yes")}) {
		t.Errorf("Requires = %+v", got)
	}
}

func TestMarksNonLiteralRequiresAsDynamic(t *testing.T) {
	got := Requires("local m = require(prefix .. \"x\")\nlocal n = require(\"a\" .. b)")
	if !slices.Equal(got, []Require{{Line: 1}, {Line: 2}}) {
		t.Errorf("Requires = %+v", got)
	}
}

func TestTreatsEscapedStringLiteralsAsDynamic(t *testing.T) {
	if got := Requires(`require("a\65")`); !slices.Equal(got, []Require{{Line: 1}}) {
		t.Errorf("Requires = %+v", got)
	}
}

func TestSkipsMethodCallsFieldAccessAndRedefinitions(t *testing.T) {
	source := "obj:require(\"x\")\nobj.require(\"y\")\nlocal function require(n) end\nlocal require = f\nf(require)"
	if got := Requires(source); len(got) != 0 {
		t.Errorf("Requires = %+v", got)
	}
}

func TestDoesNotMistakeConcatenationForFieldAccess(t *testing.T) {
	if got := Requires(`local s = "a" .. require("b")`); !slices.Equal(got, []Require{lit(1, "b")}) {
		t.Errorf("Requires = %+v", got)
	}
}

func TestTokenizeTracksLinesAcrossLongStringsAndEscapedNewlines(t *testing.T) {
	tokens, fault := Tokenize("x = [[\n\n]]\ny = \"a\\\nb\"\nz")
	if fault != nil {
		t.Fatalf("fault = %+v", fault)
	}
	lines := map[string]int{}
	for _, token := range tokens {
		lines[token.Raw] = token.Line
	}
	if lines["y"] != 4 || lines["z"] != 6 {
		t.Errorf("y is on line %d and z on line %d, want 4 and 6", lines["y"], lines["z"])
	}
}

func TestNumbersAreNotSeenByTheNameScanners(t *testing.T) {
	var values []string
	for _, token := range simpleTokens("x = 1e-5 + 0x1F") {
		values = append(values, token.value)
	}
	if !slices.Equal(values, []string{"x", "=", "+"}) {
		t.Errorf("simple tokens = %q", values)
	}
	// Symbols of several characters are single characters to them, except runs of dots.
	values = nil
	for _, token := range simpleTokens("a == b ~= c .. d ... :: <<") {
		values = append(values, token.value)
	}
	want := []string{"a", "=", "=", "b", "~", "=", "c", "..", "d", "...", ":", ":", "<", "<"}
	if !slices.Equal(values, want) {
		t.Errorf("simple tokens = %q, want %q", values, want)
	}
}

func TestTokenizeRecoversFromMalformedTokensAndReportsTheFirst(t *testing.T) {
	for _, c := range []struct {
		source, fault string
		raws          []string
	}{
		{"a = \"open\nb = 1", "unescaped newline in quoted string", []string{"a", "=", "\"open", "b", "=", "1"}},
		{"a = 'open", "unterminated quoted string", []string{"a", "=", "'open"}},
		{"a = [==[ open ]] b", "unterminated long string or comment", []string{"a", "=", "[==[ open ]] b"}},
		{"a --[[ open", "unterminated long string or comment", []string{"a"}},
		{"a = 0x + 1.2.3 b", "invalid numeral", []string{"a", "=", "0x", "+", "1.2.3", "b"}},
		{"a @ b", "unsupported symbol", []string{"a", "@", "b"}},
	} {
		tokens, fault := Tokenize(c.source)
		if fault == nil || fault.Msg != c.fault {
			t.Errorf("Tokenize(%q) fault = %+v, want %q", c.source, fault, c.fault)
		}
		var raws []string
		for _, token := range tokens {
			raws = append(raws, token.Raw)
		}
		if !slices.Equal(raws, c.raws) {
			t.Errorf("Tokenize(%q) = %q, want %q", c.source, raws, c.raws)
		}
	}
	// A line break that ends an unterminated string still counts as a line.
	if got := Requires("x = \"open\nrequire \"m\""); !slices.Equal(got, []Require{lit(2, "m")}) {
		t.Errorf("Requires after an unterminated string = %+v", got)
	}
}

func TestQuotedStringsFollowLuasEscapes(t *testing.T) {
	// \z skips the white space after it, line breaks included; a backslash before CRLF continues the string.
	source := "a = \"one\\z\n   two\" b = \"x\\\r\ny\" require \"m\""
	tokens, fault := Tokenize(source)
	if fault != nil {
		t.Fatalf("fault = %+v", fault)
	}
	if tokens[2].Text != "one\\z\n   two" || !tokens[2].Escaped || tokens[5].Text != "x\\\r\ny" {
		t.Errorf("strings = %q, %q", tokens[2].Text, tokens[5].Text)
	}
	if got := Requires(source); !slices.Equal(got, []Require{lit(3, "m")}) {
		t.Errorf("Requires = %+v", got)
	}
}
