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

/** A local library like mdlsvensson/moonwell-example-lib, in a temp folder. */
async function exampleLibrary(): Promise<string> {
  const dir = await Deno.makeTempDir({ prefix: "moonwell-lib-" });
  await Deno.mkdir(join(dir, "src", "example"), { recursive: true });
  await Deno.writeTextFile(
    join(dir, "src", "example", "greet.lua"),
    'local M = {}\nfunction M.hello(name)\n  return "Hello, " .. name\nend\nreturn M\n',
  );
  await Deno.writeTextFile(
    join(dir, "src", "example", "loud.yue"),
    'import "example.greet"\n\nexport shout = (name) -> greet.hello(name)\\upper!\n',
  );
  // What the editor's YueScript extension writes on save: loud.yue's compiled output, which must not be a module.
  await Deno.writeTextFile(
    join(dir, "src", "example", "loud.lua"),
    'return { shout = function() return "stale" end }\n',
  );
  await Deno.writeTextFile(
    join(dir, "src", "example", "globals.lua"),
    "function ExampleAdd(a, b)\n  return a + b\nend\n",
  );
  return dir;
}

async function useLibrary(project: string, library: string): Promise<void> {
  const local = join(project, "moonwell.local.pkl");
  await Deno.writeTextFile(
    local,
    `${await Deno.readTextFile(local)}\nlibraries { ["ex"] { path = "${
      library.replaceAll("\\", "/")
    }"; dir = "src" } }\n`,
  );
  const main = join(project, "src", "main.yue");
  await Deno.writeTextFile(
    main,
    `${await Deno.readTextFile(
      main,
    )}\nimport "example.loud"\nrequire "example.globals"\nprint loud.shout "Moonwell"\nprint ExampleAdd 1, 2\n`,
  );
}

Deno.test("a local library's modules build, and its editor view and folder are written", async () => {
  const project = await newProject();
  const library = await exampleLibrary();
  await useLibrary(project, library);
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  const archive = openMpq(await Deno.readFile(join(project, "dist", "bin", "map.w3x")));
  const lua = new TextDecoder().decode(await archive.read("war3map.lua"));
  assertStringIncludes(lua, '__mw.define("example.loud", function(...)');
  assertStringIncludes(lua, '"example.greet", ".moonwell/libraries/ex/example/greet.lua"}');
  assertStringIncludes(lua, '".moonwell/libraries/ex/example/loud.yue"');
  assert(!lua.includes('"stale"'), "the compiled loud.lua beside loud.yue is not bundled");
  assert(await exists(join(project, ".moonwell", "libraries", "ex", "example", "loud.yue")));
  assertEquals(await exists(join(project, "moonwell.lock")), false, "a local library is not locked");
  assertStringIncludes(
    await Deno.readTextFile(join(project, ".moonwell", "lua", "example", "loud.lua")),
    "shout",
  );
  assert(await exists(join(project, ".moonwell", "lua", "example", "greet.lua")));
});

Deno.test("a project module that clashes with a library module fails check, naming both", async () => {
  const project = await newProject();
  await useLibrary(project, await exampleLibrary());
  await Deno.mkdir(join(project, "lua", "example"), { recursive: true });
  await Deno.writeTextFile(join(project, "lua", "example", "greet.lua"), "return {}\n");
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1, checked.text);
  assertStringIncludes(
    checked.text,
    "Module example.greet is defined by lua/example/greet.lua and .moonwell/libraries/ex/example/greet.lua.",
  );
});

Deno.test("setup writes a local library's Lua modules to .moonwell/lua, even with a module clash in lua/", async () => {
  const project = await newProject();
  await useLibrary(project, await exampleLibrary());
  await Deno.mkdir(join(project, "lua", "example"), { recursive: true });
  await Deno.writeTextFile(join(project, "lua", "example", "greet.lua"), "return {}\n");
  const result = await deno(["task", "setup"], project);
  assertEquals(result.code, 0, result.text);
  assertStringIncludes(
    await Deno.readTextFile(join(project, ".moonwell", "lua", "example", "greet.lua")),
    "Hello, ",
  );
  assert(await exists(join(project, ".moonwell", "types", "natives.d.lua")));
});
