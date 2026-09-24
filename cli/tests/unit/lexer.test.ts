import { assertEquals } from "@std/assert";
import { findRequires, tokenize } from "../../src/bundle/lexer.ts";

Deno.test("finds literal requires in all call forms with line numbers", () => {
  const source = 'local a = require("a.b")\n\nlocal c = require "c"\nlocal d = require [[d.e]]\n';
  assertEquals(findRequires(source), [
    { line: 1, name: "a.b" },
    { line: 3, name: "c" },
    { line: 4, name: "d.e" },
  ]);
});

Deno.test("ignores require inside comments and strings", () => {
  const source = [
    '-- require("nope")',
    "--[[ require('nope')",
    "]]",
    '--[==[ require("nope") ]==]',
    "local s = \"require('nope')\"",
    "local t = [[",
    'require("nope")',
    "]]",
    'local real = require("yes")',
  ].join("\n");
  assertEquals(findRequires(source), [{ line: 9, name: "yes" }]);
});

Deno.test("marks non-literal requires as dynamic", () => {
  assertEquals(findRequires('local m = require(prefix .. "x")\nlocal n = require("a" .. b)'), [
    { line: 1 },
    { line: 2 },
  ]);
});

Deno.test("treats escaped string literals as dynamic", () => {
  assertEquals(findRequires('require("a\\65")'), [{ line: 1 }]);
});

Deno.test("skips method calls, field access and redefinitions", () => {
  const source = 'obj:require("x")\nobj.require("y")\nlocal function require(n) end\nlocal require = f\nf(require)';
  assertEquals(findRequires(source), []);
});

Deno.test("does not mistake concatenation for field access", () => {
  assertEquals(findRequires('local s = "a" .. require("b")'), [{ line: 1, name: "b" }]);
});

Deno.test("tokenize tracks lines across long strings and escaped newlines", () => {
  const tokens = tokenize('x = [[\n\n]]\ny = "a\\\nb"\nz');
  assertEquals(tokens.find((token) => token.value === "y")?.line, 4);
  assertEquals(tokens.find((token) => token.value === "z")?.line, 6);
});

Deno.test("numbers with exponents do not produce tokens", () => {
  assertEquals(tokenize("x = 1e-5 + 0x1F").map((token) => token.value), ["x", "=", "+"]);
});
