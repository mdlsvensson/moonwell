import { assertEquals, assertStringIncludes } from "@std/assert";
import { fromFileUrl } from "@std/path";
import { projectDenoJson, projectLocalPkl, projectPklProject } from "../../src/project-files.ts";

const repo = (path: string) => fromFileUrl(new URL(`../../../${path}`, import.meta.url));

Deno.test("projectDenoJson runs every task through the given CLI", () => {
  const json = JSON.parse(projectDenoJson("jsr:@moonwell/cli@0.1.0"));
  assertEquals(json.tasks.build, "deno run -A jsr:@moonwell/cli@0.1.0 build");
  assertEquals(Object.keys(json.tasks), ["build", "test", "dev", "check", "setup", "assets:check", "assets:sync"]);
  assertEquals("nodeModulesDir" in json, false);
});

Deno.test("projectPklProject declares a remote or local moonwell dependency", () => {
  assertStringIncludes(
    projectPklProject({ version: "0.1.0" }),
    '["moonwell"] { uri = "package://pkg.pkl-lang.org/github.com/mdlsvensson/moonwell/moonwell@0.1.0" }',
  );
  assertStringIncludes(projectPklProject({ local: "../schema" }), '["moonwell"] = import("../schema/PklProject")');
});

Deno.test("the committed template uses the local-link forms", async () => {
  assertEquals(await Deno.readTextFile(repo("template/deno.json")), projectDenoJson("../cli/src/main.ts"));
  assertEquals(await Deno.readTextFile(repo("template/PklProject")), projectPklProject({ local: "../schema" }));
});

Deno.test("projectLocalPkl amends moonwell.pkl and sets the escaped default game path", () => {
  const local = projectLocalPkl();
  assertStringIncludes(local, 'amends "moonwell.pkl"');
  assertStringIncludes(
    local,
    String.raw`gameExecutable = "C:\\Program Files (x86)\\Warcraft III\\_retail_\\x86_64\\Warcraft III.exe"`,
  );
});
