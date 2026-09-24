import { assertEquals } from "@std/assert";
import { join } from "@std/path";
import { renderEmbedded, REPO } from "../../../tools/gen.ts";

Deno.test("embedded modules are up to date (run `deno task gen`)", async () => {
  for (const [path, expected] of await renderEmbedded()) {
    assertEquals(await Deno.readTextFile(join(REPO, ...path.split("/"))), expected, path);
  }
});
