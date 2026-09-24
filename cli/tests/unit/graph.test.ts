import { assertEquals, assertThrows } from "@std/assert";
import { resolveGraph } from "../../src/bundle/graph.ts";
import type { CompiledModule } from "../../src/yue/compile.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

function modules(sources: Record<string, string>) {
  return (name: string): CompiledModule | undefined =>
    name in sources ? { name, sourcePath: `src/${name.split(".").join("/")}.yue`, source: sources[name] } : undefined;
}
const BUILTINS = new Set(["moonwell"]);

Deno.test("resolveGraph returns reachable modules dependencies-first", () => {
  const load = modules({
    main: 'local mw = require("moonwell")\nlocal a = require("a")\nlocal b = require("b")',
    a: 'local c = require("c")',
    b: 'local c = require("c")',
    c: "return {}",
    unused: "return {}",
  });
  assertEquals(resolveGraph("main", load, BUILTINS).map((module) => module.name), ["c", "a", "b", "main"]);
});

Deno.test("resolveGraph reports a missing module where it is required", () => {
  const error = assertThrows(
    () => resolveGraph("main", modules({ main: '\n\nrequire("nope")' }), BUILTINS),
    MoonwellError,
    "Module 'nope' not found",
  );
  assertEquals(error.file, "src/main.yue");
  assertEquals(error.line, 3);
});

Deno.test("resolveGraph reports a missing entry", () => {
  assertThrows(() => resolveGraph("main", modules({}), BUILTINS), MoonwellError, "Module 'main' not found");
});

Deno.test("resolveGraph rejects dynamic requires", () => {
  const error = assertThrows(
    () => resolveGraph("main", modules({ main: "require(name)" }), BUILTINS),
    MoonwellError,
    "string literal",
  );
  assertEquals(error.line, 1);
});

Deno.test("resolveGraph reports cycles with the chain", () => {
  const load = modules({ main: 'require("a")', a: 'require("b")', b: 'require("a")' });
  assertThrows(() => resolveGraph("main", load, BUILTINS), MoonwellError, "a → b → a");
});
