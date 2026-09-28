import { assertEquals, assertRejects } from "@std/assert";
import { join, resolve } from "@std/path";
import { dev, isLibraryChange, isRelevantChange, localLibraryFolders } from "../../src/commands/dev.ts";
import { type CommandContext, createContext } from "../../src/context.ts";
import type { Project } from "../../src/project/project.ts";
import { MoonwellError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";

Deno.test("isRelevantChange watches Yue sources, Lua modules, assets, object files and project manifests only", () => {
  const root = join("C:", "proj");
  const check = (...parts: string[]) => isRelevantChange(root, join(root, ...parts));
  assertEquals(check("src", "main.yue"), true);
  assertEquals(check("src", "game", "units.yue"), true);
  assertEquals(check("src", "notes.txt"), false);
  assertEquals(check("src", "generated", "objects.yue"), false);
  assertEquals(check("moonwell.pkl"), true);
  assertEquals(check("moonwell.local.pkl"), true);
  assertEquals(check("PklProject"), true);
  assertEquals(check("assets", "icons", "a.blp"), true);
  assertEquals(check("objects", "units.pkl"), true);
  assertEquals(check("objects", "human", "barracks", "units.pkl"), true);
  assertEquals(check("objects", "notes.txt"), false);
  assertEquals(check("lua", "tools", "init.lua"), true);
  assertEquals(check("lua", "notes.txt"), false);
  assertEquals(check("dist", "stage", "lua", "main.lua"), false);
  assertEquals(check("README.md"), false);
});

Deno.test("localLibraryFolders lists the folders of local libraries only", () => {
  const root = Deno.cwd();
  const folders = localLibraryFolders(root, {
    libraries: {
      mine: { github: null, tag: null, path: "../mine", dir: "src" },
      remote: { github: "o/r", tag: "v1", path: null, dir: "" },
    },
  } as unknown as Project);
  assertEquals(folders, [resolve(root, "..", "mine", "src")]);
});

Deno.test("isLibraryChange ignores changes under a local library's dot-folders", () => {
  const folder = join("C:", "lib", "src");
  assertEquals(isLibraryChange(folder, join(folder, "example", "greet.lua")), true);
  assertEquals(isLibraryChange(folder, join(folder, "example")), true);
  assertEquals(isLibraryChange(folder, join(folder, ".git", "index")), false);
  assertEquals(isLibraryChange(folder, join(folder, "example", ".cache", "x.lua")), false);
});

Deno.test("dev fails with MoonwellError before any work when src/ is missing", async () => {
  const root = await Deno.makeTempDir();
  const ctx: CommandContext = {
    ...createContext(root, silentLogger()),
    run: (command) => Promise.reject(new Error(`unexpected run: ${command}`)),
  };
  await assertRejects(() => dev(ctx, { signal: AbortSignal.abort() }), MoonwellError, "src/");
});

Deno.test("dev watches assets/, objects/ and lua/ when they exist", async () => {
  for (
    const [folders, message] of [
      [[], "Watching src/ and the project manifests. Press Ctrl+C to stop."],
      [["objects"], "Watching src/, objects/ and the project manifests. Press Ctrl+C to stop."],
      [["assets", "objects"], "Watching src/, assets/, objects/ and the project manifests. Press Ctrl+C to stop."],
      [["lua"], "Watching src/, lua/ and the project manifests. Press Ctrl+C to stop."],
    ] as const
  ) {
    const root = await Deno.makeTempDir();
    try {
      for (const folder of ["src", ...folders]) await Deno.mkdir(join(root, folder));
      const logger = silentLogger();
      const ctx: CommandContext = {
        ...createContext(root, logger),
        run: () => Promise.reject(new MoonwellError("no Pkl in this test")),
      };
      // The first check fails and is reported; an aborted signal then stops watching at once.
      await dev(ctx, { signal: AbortSignal.abort() });
      assertEquals(logger.lines, ["error: no Pkl in this test", message]);
    } finally {
      await Deno.remove(root, { recursive: true });
    }
  }
});
