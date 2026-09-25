import { assertEquals, assertStringIncludes, assertThrows } from "@std/assert";
import { emitBundle, injectBundle } from "../../src/bundle/emit.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

const modules = [
  { name: "util", sourcePath: "src/util.yue", source: "local M = {}\nreturn M\n" },
  { name: "main", sourcePath: "src/main.yue", source: 'local u = require("util")\nprint(u)\nreturn nil' },
];

Deno.test("emitBundle wraps modules and records absolute line ranges", () => {
  const bundle = emitBundle({ runtime: "local __mw = {}\n-- runtime", modules, entry: "main", firstLine: 10 });
  const lines = bundle.split("\n");
  assertEquals(lines[0], "do");
  // Line 10 = "do", 11-12 = runtime, 13 = define(util), 14-15 = util source.
  assertEquals(lines[13 - 10], '__mw.define("util", function(...)');
  assertEquals(lines[14 - 10], "local M = {}");
  assertStringIncludes(bundle, '{14, 15, "util", "src/util.yue"},');
  // 16 = end), 17 = define(main), 18-20 = main source.
  assertStringIncludes(bundle, '{18, 20, "main", "src/main.yue"},');
  assertEquals(lines[18 - 10], 'local u = require("util")');
  assertStringIncludes(bundle, '__mw.install()\n__mw.boot("main")\nend\n');
});

Deno.test("emitBundle marks minified bundles so errors name modules without lines", () => {
  const plain = emitBundle({ runtime: "", modules, entry: "main", firstLine: 1 });
  const minified = emitBundle({ runtime: "", modules, entry: "main", firstLine: 1, minify: true });
  assertEquals(plain.includes("__mw.minified = true"), false);
  assertStringIncludes(minified, '"src/main.yue"},\n}\n__mw.minified = true\n__mw.install()');
});

Deno.test("injectBundle appends after the map script and passes the first line", () => {
  const script = "function config()\nend\nfunction main()\nend";
  let seen = 0;
  const result = injectBundle(script, (first) => {
    seen = first;
    return "do\nend\n";
  });
  assertEquals(seen, 5);
  assertEquals(result.split("\n")[4], "do");
});

Deno.test("injectBundle handles CRLF scripts", () => {
  const script = "function config()\r\nend\r\nfunction main()\r\nend\r\n";
  let seen = 0;
  injectBundle(script, (first) => {
    seen = first;
    return "";
  });
  assertEquals(seen, 5);
});

Deno.test("injectBundle requires main and config", () => {
  const error = assertThrows(
    () => injectBundle("function main()\nend\n", () => "", "maps/map.w3x/war3map.lua"),
    MoonwellError,
    "config",
  );
  assertEquals(error.file, "maps/map.w3x/war3map.lua");
});
