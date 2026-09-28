import { assertEquals, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import { main } from "../../src/main.ts";
import { VERSION } from "../../src/version.ts";

async function run(args: string[], root?: string) {
  const lines: string[] = [];
  const printed: string[] = [];
  const code = await main(
    args,
    root ?? await Deno.makeTempDir(),
    (line) => lines.push(line),
    (text) => printed.push(text),
  );
  return { code, output: lines.join("\n"), stdout: printed.join("\n") };
}

Deno.test("--help and no command print usage", async () => {
  for (const args of [["--help"], []]) {
    const { code, output } = await run(args);
    assertEquals(code, 0);
    assertStringIncludes(output, "Usage: moonwell <command>");
    assertStringIncludes(output, "settings:check");
    assertStringIncludes(output, "objects:eval");
    assertStringIncludes(output, "objects:check");
    assertStringIncludes(output, "test [--entry f] [--minify]");
  }
});

Deno.test("--version prints the version", async () => {
  assertEquals(await run(["--version"]), { code: 0, output: VERSION, stdout: "" });
});

Deno.test("unknown commands fail with usage", async () => {
  const { code, output } = await run(["frobnicate"]);
  assertEquals(code, 1);
  assertStringIncludes(output, "Unknown command 'frobnicate'");
});

Deno.test("command failures are formatted and return 1", async () => {
  const { code, output } = await run(["check"]);
  assertEquals(code, 1);
  assertStringIncludes(output, "error:");
});

Deno.test("commands outside a project leave no dist/ behind", async () => {
  const root = await Deno.makeTempDir();
  for (const command of ["check", "build", "test"]) {
    const { code } = await run([command], root);
    assertEquals(code, 1);
  }
  assertEquals(await exists(join(root, "dist")), false);
});

Deno.test("the assets, settings and objects commands are known commands", async () => {
  for (
    const command of ["assets:check", "assets:sync", "assets:paths", "settings:check", "objects:eval", "objects:check"]
  ) {
    const { code, output } = await run([command]);
    assertEquals(code, 1);
    assertEquals(output.includes("Unknown command"), false, output);
  }
});

Deno.test("a failing objects:eval prints its error to the log writer and nothing to stdout", async () => {
  const { code, output, stdout } = await run(["objects:eval"]);
  assertEquals(code, 1);
  assertStringIncludes(output, "error:");
  assertEquals(stdout, "");
});
