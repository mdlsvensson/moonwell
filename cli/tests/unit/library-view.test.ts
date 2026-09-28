import { assertEquals } from "@std/assert";
import { exists } from "@std/fs";
import { join } from "@std/path";
import type { SourceModule } from "../../src/bundle/modules.ts";
import { refreshLibraryView } from "../../src/editor/library-view.ts";

Deno.test("refreshLibraryView writes library modules as Lua by module path, then only changes", async () => {
  const root = await Deno.makeTempDir({ prefix: "moonwell-view-" });
  try {
    const modules: SourceModule[] = [
      { name: "main", path: "src/main.yue", kind: "yue" },
      { name: "tools", path: "lua/tools.lua", kind: "lua", source: "return {}" },
      {
        name: "example.greet",
        path: ".moonwell/libraries/ex/example/greet.lua",
        kind: "lua",
        source: "return 1",
        library: "ex",
      },
      { name: "example.loud", path: ".moonwell/libraries/ex/example/loud.yue", kind: "yue", library: "ex" },
      { name: "kit.init", path: ".moonwell/libraries/ex/kit/init.lua", kind: "lua", source: "return 2", library: "ex" },
    ];
    const compiled = (module: SourceModule) =>
      module.name === "example.loud" ? { name: module.name, sourcePath: module.path, source: "return 3" } : undefined;
    assertEquals((await refreshLibraryView(root, modules, compiled)).sort(), [
      ".moonwell/lua/example/greet.lua",
      ".moonwell/lua/example/loud.lua",
      ".moonwell/lua/kit/init.lua",
    ]);
    assertEquals(await Deno.readTextFile(join(root, ".moonwell", "lua", "example", "loud.lua")), "return 3");
    assertEquals(await exists(join(root, ".moonwell", "lua", "tools.lua")), false, "project modules are not copied");
    assertEquals(await refreshLibraryView(root, modules, compiled), []);
    assertEquals(await refreshLibraryView(root, modules.slice(0, 3)), []);
    assertEquals(await exists(join(root, ".moonwell", "lua", "kit", "init.lua")), false, "gone modules are removed");
    assertEquals(await exists(join(root, ".moonwell", "lua", "example", "loud.lua")), false, "no compiled Lua: none");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
