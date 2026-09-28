import { assertEquals, assertRejects } from "@std/assert";
import { dirname, join } from "@std/path";
import { collectModules, moduleLoader } from "../../src/bundle/modules.ts";
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
});
