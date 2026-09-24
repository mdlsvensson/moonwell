import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { MoonwellError } from "../../src/shared/errors.ts";
import { type Runner, runProcess } from "../../src/shared/process.ts";
import { compileSources } from "../../src/yue/compile.ts";
import { testYue } from "../support/yue.ts";

async function project(files: Record<string, string>): Promise<string> {
  const root = await Deno.makeTempDir();
  for (const [path, text] of Object.entries(files)) {
    const full = join(root, path);
    await Deno.mkdir(dirname(full), { recursive: true });
    await Deno.writeTextFile(full, text);
  }
  return root;
}

function countingRunner(): { run: Runner; compiled: string[] } {
  const compiled: string[] = [];
  const run: Runner = (command, args, options) => {
    compiled.push(args[args.length - 1]);
    return runProcess(command, args, options);
  };
  return { run, compiled };
}

Deno.test("compileSources compiles modules and loads them by dotted name", async () => {
  const yue = await testYue();
  const root = await project({
    "src/main.yue": 'import "util.math" as M\nexport answer = M.double 21\n',
    "src/util/math.yue": "export double = (x) -> x * 2\n",
  });
  const output = await compileSources({ yue, root, minify: false });
  const main = output.load("main")!;
  assertEquals(main.sourcePath, "src/main.yue");
  assertStringIncludes(main.source, 'require("util.math")');
  assertEquals(output.load("util.math")?.sourcePath, "src/util/math.yue");
  assertEquals(output.load("missing"), undefined);
  assertEquals(output.load("util/math"), undefined);
  assertEquals(output.load("Util.Math"), undefined);
});

Deno.test("compileSources only recompiles changed files and removes deleted outputs", async () => {
  const yue = await testYue();
  const root = await project({
    "src/a.yue": "export x = 1\n",
    "src/b.yue": "export y = 2\n",
    "src/c.yue": "export z = 4\n",
  });
  await compileSources({ yue, root, minify: false });

  await Deno.writeTextFile(join(root, "src/a.yue"), "export x = 3\n");
  await Deno.remove(join(root, "src/b.yue"));
  const counted = countingRunner();
  const output = await compileSources({ yue, root, minify: false, run: counted.run });
  assertEquals(counted.compiled.length, 1);
  assert(counted.compiled[0].endsWith("a.yue"));
  assertStringIncludes(output.load("a")!.source, "3");
  assertEquals(await exists(join(root, "dist/stage/lua/b.lua")), false);

  const unchanged = countingRunner();
  await compileSources({ yue, root, minify: false, run: unchanged.run });
  assertEquals(unchanged.compiled.length, 0);

  const minified = countingRunner();
  await compileSources({ yue, root, minify: true, run: minified.run });
  assertEquals(minified.compiled.length, 2);
});

Deno.test("compileSources reports syntax errors with file and line", async () => {
  const yue = await testYue();
  const root = await project({ "src/ok.yue": "export x = 1\n", "src/bad.yue": "x = 1\ny = \n  if then\n" });
  const error = await assertRejects(() => compileSources({ yue, root, minify: false }), MoonwellError);
  assertEquals(error.file, "src/bad.yue");
  assertEquals(error.line, 2);
});

Deno.test("compileSources rejects dots in file names", async () => {
  const yue = await testYue();
  const root = await project({ "src/a.b.yue": "export x = 1\n" });
  await assertRejects(() => compileSources({ yue, root, minify: false }), MoonwellError, "dots");
});
