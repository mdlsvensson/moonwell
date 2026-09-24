import { assertEquals, assertStringIncludes } from "@std/assert";
import { main } from "../../src/main.ts";
import { VERSION } from "../../src/version.ts";

async function run(args: string[], root?: string) {
  const lines: string[] = [];
  const code = await main(args, root ?? await Deno.makeTempDir(), (line) => lines.push(line));
  return { code, output: lines.join("\n") };
}

Deno.test("--help and no command print usage", async () => {
  for (const args of [["--help"], []]) {
    const { code, output } = await run(args);
    assertEquals(code, 0);
    assertStringIncludes(output, "Usage: moonwell <command>");
  }
});

Deno.test("--version prints the version", async () => {
  assertEquals(await run(["--version"]), { code: 0, output: VERSION });
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
