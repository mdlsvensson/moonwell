import { assertEquals, assertStringIncludes } from "@std/assert";
import { formatError, MoonwellError } from "../../src/shared/errors.ts";

Deno.test("formatError prints file, line, message and hint", () => {
  const error = new MoonwellError("unexpected symbol", {
    file: "src/main.yue",
    line: 3,
    hint: "check the indentation",
  });
  assertEquals(formatError(error), "error: src/main.yue:3 › unexpected symbol\nhint: check the indentation");
});

Deno.test("formatError without location or hint", () => {
  assertEquals(formatError(new MoonwellError("no project")), "error: no project");
});

Deno.test("formatError with file but no line", () => {
  assertEquals(formatError(new MoonwellError("bad", { file: "moonwell.pkl" })), "error: moonwell.pkl › bad");
});

Deno.test("formatError treats other errors as internal", () => {
  const text = formatError(new TypeError("boom"));
  assertStringIncludes(text, "internal error:");
  assertStringIncludes(text, "boom");
  assertStringIncludes(text, "please report");
});
