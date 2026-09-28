import { assert, assertEquals, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { fromFileUrl, join } from "@std/path";
import { openMpq } from "../support/mpq-reader.ts";

const REPO = fromFileUrl(new URL("../../../", import.meta.url));
const MAIN = join(REPO, "cli", "src", "main.ts");

async function deno(args: string[], cwd: string) {
  const output = await new Deno.Command(Deno.execPath(), { args, cwd, stdout: "piped", stderr: "piped" }).output();
  const decoder = new TextDecoder();
  return { code: output.code, text: decoder.decode(output.stdout) + decoder.decode(output.stderr) };
}

async function newProject(): Promise<string> {
  const project = join(await Deno.makeTempDir({ prefix: "moonwell-e2e-" }), "my-map");
  const result = await deno(["run", "-A", MAIN, "init", "--link", project], REPO);
  assertEquals(result.code, 0, result.text);
  return project;
}

async function builtScript(project: string): Promise<string> {
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  const archive = openMpq(await Deno.readFile(join(project, "dist", "bin", "map.w3x")));
  return new TextDecoder().decode(await archive.read("war3map.lua"));
}

Deno.test("a new project has lua/, and build bundles the Lua modules main.yue requires", async () => {
  const project = await newProject();
  assert(await exists(join(project, "lua", ".gitkeep")));
  await Deno.mkdir(join(project, "lua", "tools"), { recursive: true });
  await Deno.writeTextFile(
    join(project, "lua", "tools", "init.lua"),
    'local M = {}\nfunction M.greet(name)\n  return "Hello, " .. name\nend\nreturn M\n',
  );
  await Deno.writeTextFile(
    join(project, "lua", "counter.lua"),
    "Count = 0\nfunction CountUp()\n  Count = Count + 1\nend\n",
  );
  await Deno.writeTextFile(join(project, "lua", "unused.lua"), "Unused = true\n");
  const main = join(project, "src", "main.yue");
  await Deno.writeTextFile(
    main,
    `${await Deno.readTextFile(main)}\nimport "tools"\nrequire "counter"\nCountUp!\nprint tools.greet "Moonwell"\n`,
  );
  const lua = await builtScript(project);
  assertStringIncludes(lua, '__mw.define("tools", function(...)');
  assertStringIncludes(lua, '__mw.define("counter", function(...)');
  assertStringIncludes(lua, '"tools", "lua/tools/init.lua"}');
  assertEquals(lua.includes("Unused = true"), false, "a Lua module nothing requires is not bundled");
});

Deno.test("check refuses a module name that src/ and lua/ both define", async () => {
  const project = await newProject();
  await Deno.writeTextFile(join(project, "lua", "main.lua"), "return {}\n");
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(checked.text, "error: lua/main.lua › Module main is defined by src/main.yue and lua/main.lua.");
});

Deno.test("check reports an unknown global the Lua file does not define", async () => {
  const project = await newProject();
  await Deno.writeTextFile(join(project, "lua", "counter.lua"), "local function hidden() end\n");
  const main = join(project, "src", "main.yue");
  await Deno.writeTextFile(main, `${await Deno.readTextFile(main)}\nrequire "counter"\nhidden!\n`);
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(checked.text, "Unknown global hidden.");
});
