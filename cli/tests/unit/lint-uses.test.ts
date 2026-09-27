import { assertEquals, assertRejects, assertThrows } from "@std/assert";
import { dirname, join, relative } from "@std/path";
import { listGlobalUses, parseGlobalUses, USES_CACHE } from "../../src/lint/uses.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import type { Runner, RunResult } from "../../src/shared/process.ts";

/** A stand-in compiler: `yue -g <file>` prints `outputs[<path under src/>]` and records the call. */
function stubYue(root: string, outputs: Record<string, RunResult>) {
  const calls: string[] = [];
  const run: Runner = (_command, args) => {
    assertEquals(args.length, 2);
    assertEquals(args[0], "-g");
    const file = relative(join(root, "src"), args[1]).replaceAll("\\", "/");
    calls.push(file);
    return Promise.resolve(outputs[file] ?? { code: 0, stdout: "", stderr: "" });
  };
  return { run, calls };
}

const ok = (stdout: string): RunResult => ({ code: 0, stdout, stderr: "" });

Deno.test("parseGlobalUses reads CRLF output and skips blank lines", () => {
  assertEquals(parseGlobalUses("Score 1 8\r\nCreatUnit 2 7\r\n\r\n", "src/main.yue"), [
    { name: "Score", line: 1, column: 8 },
    { name: "CreatUnit", line: 2, column: 7 },
  ]);
  assertEquals(parseGlobalUses("\n", "src/main.yue"), []);
});

Deno.test("parseGlobalUses refuses output it cannot read", () => {
  const error = assertThrows(() => parseGlobalUses("Score one 8\n", "src/main.yue"), MoonwellError, "Score one 8");
  assertEquals(error.file, "src/main.yue");
});

Deno.test("listGlobalUses runs yue -g once per changed file and caches by hash", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    const outputs = { "main.yue": ok("print 1 1\n"), "heroes/captain.yue": ok("CreatUnit 3 5\n") };
    const hashes = { "main.yue": "h1", "heroes/captain.yue": "h2" };
    const expected = {
      "heroes/captain.yue": [{ name: "CreatUnit", line: 3, column: 5 }],
      "main.yue": [{ name: "print", line: 1, column: 1 }],
    };

    const first = stubYue(root, outputs);
    assertEquals(await listGlobalUses({ yue: "yue", root, hashes, run: first.run }), expected);
    assertEquals(first.calls.sort(), ["heroes/captain.yue", "main.yue"]);

    const unchanged = stubYue(root, outputs);
    assertEquals(await listGlobalUses({ yue: "yue", root, hashes, run: unchanged.run }), expected);
    assertEquals(unchanged.calls, []);

    const edited = stubYue(root, { ...outputs, "main.yue": ok("") });
    const afterEdit = await listGlobalUses({
      yue: "yue",
      root,
      hashes: { ...hashes, "main.yue": "h3" },
      run: edited.run,
    });
    assertEquals(edited.calls, ["main.yue"]);
    assertEquals(afterEdit["main.yue"], []);

    const otherCompiler = stubYue(root, outputs);
    await listGlobalUses({ yue: "other-yue", root, hashes, run: otherCompiler.run });
    assertEquals(otherCompiler.calls.sort(), ["heroes/captain.yue", "main.yue"]);

    // A deleted file leaves the cache.
    await listGlobalUses({ yue: "other-yue", root, hashes: { "main.yue": "h1" }, run: stubYue(root, outputs).run });
    const cache = JSON.parse(await Deno.readTextFile(join(root, USES_CACHE)));
    assertEquals(Object.keys(cache.files), ["main.yue"]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("listGlobalUses runs every file again when the cache is unreadable", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    await Deno.mkdir(dirname(join(root, USES_CACHE)), { recursive: true });
    await Deno.writeTextFile(join(root, USES_CACHE), '{"settings": "yue", "files": {"main.yue": {"hash": "h1"}}}');
    const stub = stubYue(root, { "main.yue": ok("print 1 1\n") });
    assertEquals(await listGlobalUses({ yue: "yue", root, hashes: { "main.yue": "h1" }, run: stub.run }), {
      "main.yue": [{ name: "print", line: 1, column: 1 }],
    });
    assertEquals(stub.calls, ["main.yue"]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("listGlobalUses reports a failed yue -g like a compile error", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    const failed = { code: 1, stdout: "Failed to compile: main.yue\n2: unexpected expression\n", stderr: "" };
    const stub = stubYue(root, { "main.yue": failed });
    const error = await assertRejects(
      () => listGlobalUses({ yue: "yue", root, hashes: { "main.yue": "h1" }, run: stub.run }),
      MoonwellError,
      "unexpected expression",
    );
    assertEquals([error.file, error.line], ["src/main.yue", 2]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
