import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { dirname, join } from "@std/path";
import { collectModules, libraryModuleRoots, PROJECT_MODULE_ROOTS } from "../../src/bundle/modules.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { type Runner, runProcess } from "../../src/shared/process.ts";
import { compileSources } from "../../src/yue/compile.ts";
import { MACROS_YUE } from "../../src/embedded/macros.ts";
import { MACROS_FILE, macroSearch } from "../../src/yue/macros.ts";
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
  assertEquals(Object.keys(output.hashes).sort(), ["main.yue", "util/math.yue"]);
  assertEquals(output.hashes["main.yue"].length, 64);
  assertEquals(output.sources["util/math.yue"], "export double = (x) -> x * 2\n");
  const main = output.load("main")!;
  assertEquals(main.sourcePath, "src/main.yue");
  assertStringIncludes(main.source, 'require("util.math")');
  assertEquals(output.load("util.math")?.sourcePath, "src/util/math.yue");
  assertEquals(output.load("missing"), undefined);
  assertEquals(output.load("util/math"), undefined);
  assertEquals(output.load("Util.Math"), undefined);
});

Deno.test("compileSources compiles library YueScript into dist/stage/lua/.libraries/<key>/", async () => {
  const yue = await testYue();
  const root = await project({
    "src/main.yue": 'import "example.loud"\n',
    ".moonwell/libraries/ex/example/loud.yue": "export shout = (name) -> name\\upper!\n",
  });
  const modules = await collectModules(root, [...PROJECT_MODULE_ROOTS, ...libraryModuleRoots(["ex"])]);
  const output = await compileSources({ yue, root, minify: false, modules });
  const library = modules.find((module) => module.name === "example.loud")!;
  assertEquals(library.library, "ex");
  const compiled = output.loadModule(library)!;
  assertEquals(compiled.sourcePath, ".moonwell/libraries/ex/example/loud.yue");
  assertStringIncludes(compiled.source, "upper");
  assert(await exists(join(root, "dist", "stage", "lua", ".libraries", "ex", "example", "loud.lua")));
  assertEquals(Object.keys(output.hashes), ["main.yue"], "the unknown-global check still sees src/ only");
  assertEquals(Object.keys(output.texts).sort(), [".moonwell/libraries/ex/example/loud.yue", "src/main.yue"]);
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

Deno.test("compileSources expands $FourCC through the macro module", async () => {
  const yue = await testYue();
  const root = await project({
    [MACROS_FILE]: MACROS_YUE,
    "src/main.yue": 'import "moonwell.macros" as {:$FourCC}\nexport footman = $FourCC "hfoo"\n',
  });
  const output = await compileSources({ yue, root, minify: false, macros: await macroSearch(root) });
  assertStringIncludes(output.load("main")!.source, "1751543663");
});

Deno.test("a changed macro module recompiles every file", async () => {
  const yue = await testYue();
  const root = await project({
    [MACROS_FILE]: MACROS_YUE,
    "src/a.yue": "export x = 1\n",
    "src/b.yue": "export y = 2\n",
  });
  const macros = await macroSearch(root);
  await compileSources({ yue, root, minify: false, macros });
  const unchanged = countingRunner();
  await compileSources({ yue, root, minify: false, macros, run: unchanged.run });
  assertEquals(unchanged.compiled.length, 0);
  const changed = countingRunner();
  await compileSources({ yue, root, minify: false, macros: { ...macros, hash: "another" }, run: changed.run });
  assertEquals(changed.compiled.length, 2);
});

Deno.test("a failed macro names the file and line, with the macro's own message", async () => {
  const yue = await testYue();
  const root = await project({
    [MACROS_FILE]: MACROS_YUE,
    "src/main.yue": 'import "moonwell.macros" as {:$FourCC}\nx = 1\ny = $FourCC "hfo"\n',
  });
  const error = await assertRejects(
    () => compileSources({ yue, root, minify: false, macros: undefined }).then(() => undefined),
    MoonwellError,
  );
  assertStringIncludes(error.message, "moonwell.macros", "without --path the module is not found");
  const failed = await assertRejects(
    async () => compileSources({ yue, root, minify: false, macros: await macroSearch(root) }),
    MoonwellError,
  );
  assertEquals([failed.file, failed.line], ["src/main.yue", 3]);
  assert(
    failed.message.startsWith('$FourCC needs a string literal of exactly 4 characters, such as "hfoo".\n'),
    failed.message,
  );
});
