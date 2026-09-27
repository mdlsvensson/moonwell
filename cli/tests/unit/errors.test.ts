import { assertEquals, assertStringIncludes } from "@std/assert";
import { formatError, formatProblem, MoonwellError, ObjectDataError, ProblemsError } from "../../src/shared/errors.ts";

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

Deno.test("ObjectDataError is a MoonwellError whose file, message and hint are the first problem's", () => {
  const error = new ObjectDataError([
    { file: "objects/a.pkl", message: "first", hint: "one" },
    { file: "objects/b.pkl", message: "second" },
  ]);
  assertEquals([error instanceof MoonwellError, error.file, error.message, error.hint], [
    true,
    "objects/a.pkl",
    "first",
    "one",
  ]);
});

Deno.test("formatError prints each object problem with its own hint, at most 20, then how many more", () => {
  assertEquals(
    formatError(
      new ObjectDataError([
        { file: "objects/a.pkl", message: 'units["a"].id: bad', hint: "fix it" },
        { file: "objects/b.pkl", message: 'units["b"].base: worse' },
      ]),
    ),
    'error: objects/a.pkl › units["a"].id: bad\nhint: fix it\nerror: objects/b.pkl › units["b"].base: worse',
  );
  const many = new ObjectDataError(Array.from({ length: 25 }, (_, i) => ({ file: "f.pkl", message: `p${i}` })));
  const lines = formatError(many).split("\n");
  assertEquals(lines.length, 21);
  assertEquals([lines[0], lines[19], lines[20]], ["error: f.pkl › p0", "error: f.pkl › p19", "and 5 more"]);
  assertEquals(
    formatError(new ObjectDataError(Array.from({ length: 20 }, () => ({ file: "f", message: "p" })))).split("\n")
      .length,
    20,
  );
});

Deno.test("formatError prints each problem with its line and column when it has them", () => {
  const error = new ProblemsError([
    {
      file: "src/main.yue",
      line: 7,
      column: 11,
      message: "Unknown global CreatUnit.",
      hint: "Did you mean CreateUnit?",
    },
    { file: "src/main.yue", line: 9, message: "second" },
    { file: "objects/a.pkl", message: "third" },
  ]);
  assertEquals(
    formatError(error),
    [
      "error: src/main.yue:7:11 › Unknown global CreatUnit.",
      "hint: Did you mean CreateUnit?",
      "error: src/main.yue:9 › second",
      "error: objects/a.pkl › third",
    ].join("\n"),
  );
  assertEquals([error instanceof MoonwellError, error.file, error.line, error.message], [
    true,
    "src/main.yue",
    7,
    "Unknown global CreatUnit.",
  ]);
});

Deno.test("formatProblem has no prefix, so warnings can use it", () => {
  assertEquals(
    formatProblem({ file: "src/a.yue", line: 1, column: 2, message: "m", hint: "h" }),
    "src/a.yue:1:2 › m\nhint: h",
  );
  assertEquals(formatProblem({ message: "m" }), "m");
});

Deno.test("ObjectDataError is a ProblemsError", () => {
  const error = new ObjectDataError([{ file: "objects/a.pkl", message: "first" }]);
  assertEquals([error instanceof ProblemsError, error.name], [true, "ObjectDataError"]);
});
