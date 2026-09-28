import { assertEquals, assertRejects } from "@std/assert";
import { join, relative } from "@std/path";
import { type CommandContext, createContext } from "../../src/context.ts";
import {
  checkUnknownGlobals,
  declaredGlobals,
  knownGlobals,
  UNKNOWN_GLOBAL_HINT,
  unknownGlobalProblems,
} from "../../src/lint/unknown-globals.ts";
import type { Natives } from "../../src/natives/natives.ts";
import { emptyObjects } from "../../src/objectdata/manifest.ts";
import type { Project } from "../../src/project/project.ts";
import { validateMapSettings } from "../../src/settings/options.ts";
import { formatError, ProblemsError } from "../../src/shared/errors.ts";
import { silentLogger } from "../support/logger.ts";

const NATIVES: Natives = {
  gameVersion: "9.9.9",
  types: [],
  functions: [
    { name: "CreateUnit", source: "common.j", constant: false, params: [], returns: "unit" },
    { name: "FourCC", source: "lua", constant: false, params: [], returns: "integer" },
  ],
  globals: [{ name: "bj_MAX_PLAYERS", source: "blizzard.j", type: "integer", constant: true, array: false }],
  lua: { globals: ["print", "math"], removed: ["io"] },
};

Deno.test("declaredGlobals reads the names on global lines", () => {
  const source = [
    "global Score = 0",
    "global a, b",
    "  global x, y = 1, 2 -- indented, with a comment",
    "global const K = 1",
    "global class Boss extends Base",
    "global f = (n) -> n",
    "global *",
    "global ^",
    "globalScore = 1",
    "print global",
  ].join("\r\n");
  assertEquals(declaredGlobals(source), ["Score", "a", "b", "x", "y", "K", "Boss", "f"]);
});

Deno.test("knownGlobals joins the natives, the map, declared names and lint.globals", () => {
  const known = knownGlobals({
    natives: NATIVES,
    map: { globals: [{ name: "udg_Score", type: "integer" }], functions: ["InitCustomTriggers"] },
    declared: ["Round"],
    extra: ["MyLibrary"],
  });
  assertEquals([...known].sort(), [
    "CreateUnit",
    "FourCC",
    "InitCustomTriggers",
    "MyLibrary",
    "Round",
    "bj_MAX_PLAYERS",
    "math",
    "print",
    "udg_Score",
  ]);
  assertEquals(knownGlobals({ natives: NATIVES, declared: [], extra: [] }).has("udg_Score"), false);
});

Deno.test("unknownGlobalProblems reports every unknown use in file, line and column order", () => {
  const known = knownGlobals({ natives: NATIVES, declared: [], extra: [] });
  const problems = unknownGlobalProblems(
    {
      "main.yue": [
        { name: "print", line: 1, column: 1 },
        { name: "CreatUnit", line: 7, column: 11 },
        { name: "Zzz", line: 2, column: 3 },
        { name: "io", line: 2, column: 1 },
      ],
      "a.yue": [{ name: "Zzz", line: 9, column: 1 }],
    },
    known,
    NATIVES.lua.removed,
  );
  assertEquals(problems, [
    { file: "src/a.yue", line: 9, column: 1, message: "Unknown global Zzz.", hint: UNKNOWN_GLOBAL_HINT },
    {
      file: "src/main.yue",
      line: 2,
      column: 1,
      message: "Unknown global io.",
      hint: "Warcraft III's Lua does not provide io.",
    },
    { file: "src/main.yue", line: 2, column: 3, message: "Unknown global Zzz.", hint: UNKNOWN_GLOBAL_HINT },
    {
      file: "src/main.yue",
      line: 7,
      column: 11,
      message: "Unknown global CreatUnit.",
      hint: `Did you mean CreateUnit? ${UNKNOWN_GLOBAL_HINT}`,
    },
  ]);
  assertEquals(
    UNKNOWN_GLOBAL_HINT,
    "Declare your own globals with `global`, or add them to lint.globals in moonwell.pkl.",
  );
});

/**
 * A project whose compiler is a stand-in printing `outputs[<path under src/>]` for `yue -g`. The sources reach the
 * check only through `compiled.declared`; nothing is written under src/, so the check must not read the files again.
 */
async function lintProject(
  sources: Record<string, string>,
  outputs: Record<string, string>,
  options: { lint?: Project["lint"]; mapScript?: string; lua?: string[] } = {},
) {
  const root = await Deno.makeTempDir({ prefix: "moonwell-lint-" });
  if (options.mapScript !== undefined) {
    await Deno.mkdir(join(root, "maps", "map.w3x"), { recursive: true });
    await Deno.writeTextFile(join(root, "maps", "map.w3x", "war3map.lua"), options.mapScript);
  }
  const logger = silentLogger();
  const ctx: CommandContext = {
    ...createContext(root, logger),
    run: (_command, args) => {
      const file = relative(join(root, "src"), args[1]).replaceAll("\\", "/");
      return Promise.resolve({ code: 0, stdout: outputs[file] ?? "", stderr: "" });
    },
  };
  const project: Project = {
    root,
    manifest: "moonwell.pkl",
    map: { folder: "map.w3x", entry: "src/main.yue" },
    build: { folder: "dist/bin", minify: false },
    launch: { gameExecutable: null, args: [] },
    yue: { version: "0.34.2", path: null },
    assets: { paths: {}, exclude: [] },
    lint: options.lint ?? { unknownGlobals: "error", globals: [] },
    libraries: {},
    settings: validateMapSettings({}),
    objects: emptyObjects(),
  };
  const hashes = Object.fromEntries(Object.keys(sources).map((file, i) => [file, `h${i}`]));
  return {
    root,
    ctx,
    logger,
    project,
    compiled: { yue: "yue", hashes, declared: { yue: Object.values(sources), lua: options.lua ?? [] } },
  };
}

Deno.test("checkUnknownGlobals knows the globals a Lua module defines at its top level", async () => {
  const { root, ctx, project, compiled } = await lintProject(
    { "main.yue": "CountUp!\n" },
    { "main.yue": "CountUp 1 1\n" },
    { lua: ["Count = 0\nfunction CountUp()\n  Count = Count + 1\nend\n"] },
  );
  try {
    assertEquals(await checkUnknownGlobals(ctx, project, compiled, NATIVES), []);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("checkUnknownGlobals accepts map, declared and lint.globals names", async () => {
  const { root, ctx, project, compiled } = await lintProject(
    { "main.yue": "print udg_Score, Round, MyLibrary\n", "state.yue": "global Round = 1\n" },
    { "main.yue": "print 1 1\nudg_Score 1 7\nRound 1 18\nMyLibrary 1 25\n", "state.yue": "Round 1 8\n" },
    { lint: { unknownGlobals: "error", globals: ["MyLibrary"] }, mapScript: "udg_Score = 0\nfunction main()\nend\n" },
  );
  try {
    assertEquals(await checkUnknownGlobals(ctx, project, compiled, NATIVES), []);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("checkUnknownGlobals throws every unknown use at once", async () => {
  const { root, ctx, project, compiled } = await lintProject(
    { "main.yue": "x = CreatUnit!\nprint udg_Score\n" },
    { "main.yue": "CreatUnit 1 5\nprint 2 1\nudg_Score 2 7\n" },
  );
  try {
    const error = await assertRejects(() => checkUnknownGlobals(ctx, project, compiled, NATIVES), ProblemsError);
    assertEquals(
      formatError(error),
      [
        "error: src/main.yue:1:5 › Unknown global CreatUnit.",
        `hint: Did you mean CreateUnit? ${UNKNOWN_GLOBAL_HINT}`,
        "error: src/main.yue:2:7 › Unknown global udg_Score.",
        `hint: ${UNKNOWN_GLOBAL_HINT}`,
      ].join("\n"),
    );
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("checkUnknownGlobals only warns with unknownGlobals = warning", async () => {
  const { root, ctx, logger, project, compiled } = await lintProject(
    { "main.yue": "x = CreatUnit!\n" },
    { "main.yue": "CreatUnit 1 5\n" },
    { lint: { unknownGlobals: "warning", globals: [] } },
  );
  try {
    const problems = await checkUnknownGlobals(ctx, project, compiled, NATIVES);
    assertEquals(problems.length, 1);
    assertEquals(logger.lines, [
      `warning: src/main.yue:1:5 › Unknown global CreatUnit.\nhint: Did you mean CreateUnit? ${UNKNOWN_GLOBAL_HINT}`,
    ]);
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("checkUnknownGlobals warns about at most 20 uses, then how many more", async () => {
  const uses = Array.from({ length: 23 }, (_, i) => `Zzz ${i + 1} 1`).join("\n");
  const { root, ctx, logger, project, compiled } = await lintProject(
    { "main.yue": "" },
    { "main.yue": uses },
    { lint: { unknownGlobals: "warning", globals: [] } },
  );
  try {
    await checkUnknownGlobals(ctx, project, compiled, NATIVES);
    assertEquals(logger.lines.length, 21);
    assertEquals(logger.lines[20], "warning: and 3 more unknown global(s)");
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
