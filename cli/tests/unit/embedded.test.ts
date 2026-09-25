import { assert, assertEquals } from "@std/assert";
import { join } from "@std/path";
import { TEMPLATE_FILES } from "../../src/embedded/template.ts";
import { renderEmbedded, REPO, templateEntries } from "../../../tools/gen.ts";

Deno.test("the embedded runtime is up to date (run `deno task gen`)", async () => {
  const path = "cli/src/embedded/runtime.ts";
  const expected = (await renderEmbedded()).get(path);
  // A plain boolean: on a mismatch, printing both copies of the runtime would bury the hint.
  assert(await Deno.readTextFile(join(REPO, path)) === expected, `${path} is stale: run \`deno task gen\`.`);
});

Deno.test("the embedded template matches template/ (run `deno task gen`)", async () => {
  const embedded = new Map(TEMPLATE_FILES.map((file) => [file.path, file.base64]));
  const actual = new Map((await templateEntries()).map((file) => [file.path, file.base64]));
  const differences = [
    ...[...actual.keys()].filter((path) => !embedded.has(path)).map((path) => `added:   template/${path}`),
    ...[...embedded.keys()].filter((path) => !actual.has(path)).map((path) => `removed: template/${path}`),
    ...[...actual].filter(([path, base64]) => embedded.has(path) && embedded.get(path) !== base64)
      .map(([path]) => `changed: template/${path}`),
  ];
  assertEquals(
    differences,
    [],
    "cli/src/embedded/template.ts does not match template/. Run `deno task gen` if these changes belong in the " +
      "template; otherwise remove them.",
  );
});
