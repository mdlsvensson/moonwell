import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { dirname, join } from "@std/path";
import { MACROS_YUE } from "../../src/embedded/macros.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { sha256Hex } from "../../src/shared/fs.ts";
import { runProcess } from "../../src/shared/process.ts";
import { macroPathArgs, MACROS_FILE, macroSearch } from "../../src/yue/macros.ts";
import { testYue } from "../support/yue.ts";

const MESSAGE = '$FourCC needs a string literal of exactly 4 characters, such as "hfoo".';

/** Compiles `print <call>` after the macro import in a temp project; returns the Lua, or the compiler's output. */
async function compile(call: string): Promise<{ ok: boolean; text: string }> {
  const yue = await testYue();
  const root = await Deno.makeTempDir({ prefix: "moonwell-macros-" });
  try {
    await Deno.mkdir(dirname(join(root, MACROS_FILE)), { recursive: true });
    await Deno.writeTextFile(join(root, MACROS_FILE), MACROS_YUE);
    await Deno.mkdir(join(root, "src"));
    const source = join(root, "src", "main.yue");
    await Deno.writeTextFile(source, `import "moonwell.macros" as {:$FourCC}\nprint ${call}\n`);
    const output = join(root, "main.lua");
    const args = ["--target=5.3", "-r", "-o", output, ...macroPathArgs(await macroSearch(root)), source];
    const result = await runProcess(yue, args);
    if (result.code !== 0) return { ok: false, text: `${result.stdout}\n${result.stderr}` };
    return { ok: true, text: await Deno.readTextFile(output) };
  } finally {
    await Deno.remove(root, { recursive: true });
  }
}

Deno.test("$FourCC turns a 4-character string literal into the rawcode's integer", async () => {
  for (const call of ['$FourCC "hfoo"', "$FourCC 'hfoo'", '$FourCC("hfoo")']) {
    const result = await compile(call);
    assert(result.ok, result.text);
    assertStringIncludes(result.text, "1751543663");
    assertEquals(result.text.includes("moonwell.macros"), false, "the macro import leaves nothing in the Lua");
  }
  const hero = await compile('$FourCC "Hpal"');
  assert(hero.ok, hero.text);
  assertStringIncludes(hero.text, "1215324524");
  // A single-quoted string does not interpolate, so '#{a}' is its own four characters.
  const literal = await compile("$FourCC '#{a}'");
  assert(literal.ok, literal.text);
  assertStringIncludes(literal.text, "595288445");
});

Deno.test("$FourCC refuses anything but a 4-character string literal", async () => {
  for (
    const call of [
      "$FourCC!",
      "$FourCC x",
      '$FourCC "hfo"',
      '$FourCC "hfooo"',
      "$FourCC 1234",
      '$FourCC "h\\oo"',
      '$FourCC "héé"',
      '$FourCC "hé!"',
      "$FourCC [[hfoo]]",
      '$FourCC "hfoo", "x"',
      '$FourCC "#{x}"',
    ]
  ) {
    const result = await compile(call);
    assertEquals(result.ok, false, call);
    assertStringIncludes(result.text, MESSAGE, call);
  }
});

Deno.test("macroSearch points yue at .moonwell/yue and hashes the module", async () => {
  const search = await macroSearch("/project");
  assertEquals(search.path, join("/project", ".moonwell", "yue", "?.lua"));
  assertEquals(search.hash, await sha256Hex(new TextEncoder().encode(MACROS_YUE)));
  assertEquals(macroPathArgs(search), ["--path", search.path]);
  assertEquals(macroPathArgs(undefined), []);
});

Deno.test("macroSearch refuses a project folder whose path has ';' or '?'", async () => {
  for (const root of ["/pro;ject", "/pro?ject"]) {
    const error = await assertRejects(() => macroSearch(root), MoonwellError);
    assertStringIncludes(error.message, "YueScript's module search cannot handle");
    assertEquals(error.file, root);
    assertStringIncludes(error.hint ?? "", "Move the project to a folder whose path has neither character.");
  }
});
