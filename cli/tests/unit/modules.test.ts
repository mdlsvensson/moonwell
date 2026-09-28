import { assertEquals, assertRejects } from "@std/assert";
import { dirname, join } from "@std/path";
import { collectModules, moduleLoader } from "../../src/bundle/modules.ts";
import { luaTopLevelGlobals } from "../../src/lint/lua-globals.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

async function project(files: Record<string, string>): Promise<string> {
  const root = await Deno.makeTempDir({ prefix: "moonwell-modules-" });
  for (const [path, text] of Object.entries(files)) {
    await Deno.mkdir(dirname(join(root, path)), { recursive: true });
    await Deno.writeTextFile(join(root, path), text);
  }
  return root;
}

Deno.test("collectModules lists YueScript in src/ and Lua in lua/, named by path", async () => {
  const root = await project({
    "src/main.yue": "x = 1\n",
    "src/game/units.yue": "x = 1\n",
    "src/main.lua": "-- the editor's output, ignored\n",
    "lua/tools/init.lua": "return {}\n",
    "lua/counter.lua": "Count = 0\n",
    "lua/README.md": "ignored\n",
  });
  try {
    assertEquals(await collectModules(root), [
      { name: "game.units", path: "src/game/units.yue", kind: "yue" },
      { name: "main", path: "src/main.yue", kind: "yue" },
      { name: "counter", path: "lua/counter.lua", kind: "lua", source: "Count = 0\n" },
      { name: "tools.init", path: "lua/tools/init.lua", kind: "lua", source: "return {}\n" },
    ]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("collectModules reads a Lua file saved with a BOM without it, so the scan sees its first line", async () => {
  const root = await project({ "src/main.yue": "x = 1\n", "lua/x.lua": "\uFEFFCounter = 0\n" });
  try {
    const [, lua] = await collectModules(root);
    assertEquals(lua.source, "Counter = 0\n");
    assertEquals(luaTopLevelGlobals(lua.source ?? ""), ["Counter"]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("collectModules refuses a module that is or claims a built-in module's name", async () => {
  for (const path of ["lua/moonwell.lua", "lua/moonwell/init.lua", "src/moonwell.yue"]) {
    const root = await project({ "src/main.yue": "x = 1\n", [path]: "" });
    try {
      const error = await assertRejects(
        () => collectModules(root),
        MoonwellError,
        `Module moonwell is built into Moonwell; rename ${path}.`,
      );
      assertEquals(error.file, path);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  }
  const root = await project({ "src/main.yue": "x = 1\n", "lua/moonwell/extra.lua": "" });
  try {
    assertEquals((await collectModules(root)).map((module) => module.name), ["main", "moonwell.extra"]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("collectModules works without lua/ and requires src/", async () => {
  const root = await project({ "src/main.yue": "x = 1\n" });
  try {
    assertEquals((await collectModules(root)).map((module) => module.path), ["src/main.yue"]);
    await Deno.remove(join(root, "src"), { recursive: true });
    await assertRejects(() => collectModules(root), MoonwellError, "The src/ folder is missing.");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("collectModules refuses dotted names in either folder", async () => {
  for (const path of ["src/a.b.yue", "lua/x.y/z.lua"]) {
    const root = await project({ "src/main.yue": "x = 1\n", [path]: "" });
    try {
      const error = await assertRejects(() => collectModules(root), MoonwellError, "cannot contain dots");
      assertEquals(error.file, path);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  }
});

Deno.test("collectModules refuses a name two files define, naming both", async () => {
  const root = await project({ "src/main.yue": "x = 1\n", "src/tools.yue": "x = 1\n", "lua/tools.lua": "" });
  try {
    const error = await assertRejects(
      () => collectModules(root),
      MoonwellError,
      "Module tools is defined by src/tools.yue and lua/tools.lua.",
    );
    assertEquals(error.file, "lua/tools.lua");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("collectModules refuses an init module next to a module of its parent's name", async () => {
  const cases = [
    { files: ["src/tools.yue", "lua/tools/init.lua"], first: "src/tools.yue", second: "lua/tools/init.lua" },
    { files: ["lua/tools.lua", "lua/tools/init.lua"], first: "lua/tools.lua", second: "lua/tools/init.lua" },
  ];
  for (const { files, first, second } of cases) {
    const root = await project({ "src/main.yue": "x = 1\n", ...Object.fromEntries(files.map((path) => [path, ""])) });
    try {
      const error = await assertRejects(
        () => collectModules(root),
        MoonwellError,
        `Module tools is defined by ${first} and ${second}.`,
      );
      assertEquals(error.file, second);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  }
});

Deno.test("collectModules lets a top-level init module claim only its own name", async () => {
  const root = await project({ "src/main.yue": "x = 1\n", "lua/init.lua": "" });
  try {
    assertEquals((await collectModules(root)).map((module) => module.name), ["main", "init"]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("moduleLoader resolves a name, then <name>.init, under the name that was required", () => {
  const modules = [
    { name: "main", path: "src/main.yue", kind: "yue" as const },
    { name: "tools.init", path: "lua/tools/init.lua", kind: "lua" as const, source: "return {}" },
  ];
  const compiled = { name: "main", sourcePath: "src/main.yue", source: "local x = 1" };
  const load = moduleLoader(modules, (name) => name === "main" ? compiled : undefined);
  assertEquals(load("main"), compiled);
  assertEquals(load("tools"), { name: "tools", sourcePath: "lua/tools/init.lua", source: "return {}", kind: "lua" });
  assertEquals(load("tools.init")?.name, "tools.init");
  assertEquals(load("missing"), undefined);

  const gameInit = { name: "game.init", sourcePath: "src/game/init.yue", source: "local y = 2" };
  const loadGame = moduleLoader(
    [{ name: "game.init", path: "src/game/init.yue", kind: "yue" as const }],
    (name) => name === "game.init" ? gameInit : undefined,
  );
  assertEquals(loadGame("game"), { ...gameInit, name: "game" });
});
