import { assert, assertEquals, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { fromFileUrl, join } from "@std/path";
import { test } from "../../src/commands/test.ts";
import { projectLocalPkl } from "../../src/project-files.ts";
import { type CommandContext, createContext } from "../../src/context.ts";
import { silentLogger } from "../support/logger.ts";
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

Deno.test("init → build produces an archive with the injected bundle", async () => {
  const project = await newProject();
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  assertStringIncludes(built.text, "Built dist/bin/map.w3x");

  const archive = openMpq(await Deno.readFile(join(project, "dist", "bin", "map.w3x")));
  const lua = new TextDecoder().decode(await archive.read("war3map.lua"));
  assertStringIncludes(lua, "function main()");
  assertStringIncludes(lua, '__mw.define("main", function(...)');
  assertStringIncludes(lua, '__mw.boot("main")');
  assert(await archive.read("war3map.w3i"));
  assert((await archive.listfile()).includes("war3map.lua"));
});

Deno.test("a failed build deletes the previous archive", async () => {
  const project = await newProject();
  const archive = join(project, "dist", "bin", "map.w3x");
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 0, built.text);
  assert(await exists(archive));
  await Deno.writeTextFile(join(project, "src", "main.yue"), "x = \n  if then\n");
  const failed = await deno(["task", "build"], project);
  assertEquals(failed.code, 1, failed.text);
  assertEquals(await exists(archive), false, "the stale archive survived a failed build");
});

Deno.test("test stages the map and launches the game on the staged folder", async () => {
  const project = await newProject();
  const exe = join(project, "Warcraft III.exe");
  await Deno.writeTextFile(exe, "");
  await Deno.writeTextFile(
    join(project, "moonwell.local.pkl"),
    `amends "moonwell.pkl"\nlaunch { gameExecutable = #"${exe}"# }\n`,
  );
  const calls: Array<[string, string[]]> = [];
  const ctx: CommandContext = {
    ...createContext(project, silentLogger()),
    spawn: (command, args) => calls.push([command, args]),
  };
  await test(ctx, {});

  const staged = join(project, "dist", "stage", "map.w3x");
  assertEquals(calls, [[exe, ["-launch", "-windowmode", "windowed", "-loadfile", staged]]]);
  assertStringIncludes(await Deno.readTextFile(join(staged, "war3map.lua")), '__mw.boot("main")');
});

Deno.test("setup recreates a missing moonwell.local.pkl and keeps an existing one", async () => {
  const project = await newProject();
  const local = join(project, "moonwell.local.pkl");
  await Deno.remove(local);
  const created = await deno(["task", "setup"], project);
  assertEquals(created.code, 0, created.text);
  assertEquals(await Deno.readTextFile(local), projectLocalPkl());

  const mine = `amends "moonwell.pkl"\nlaunch { gameExecutable = "/games/wc3.exe" }\n`;
  await Deno.writeTextFile(local, mine);
  const kept = await deno(["task", "setup"], project);
  assertEquals(kept.code, 0, kept.text);
  assertEquals(await Deno.readTextFile(local), mine);
});

Deno.test("build refuses a build.folder that would overwrite the source map", async () => {
  const project = await newProject();
  const manifest = join(project, "moonwell.pkl");
  const text = await Deno.readTextFile(manifest);
  assertStringIncludes(text, 'folder = "dist/bin"');
  await Deno.writeTextFile(manifest, text.replace('folder = "dist/bin"', 'folder = "maps"'));
  const built = await deno(["task", "build"], project);
  assertEquals(built.code, 1, built.text);
  assertStringIncludes(built.text, "isReservedFolder");
  assert(await exists(join(project, "maps", "map.w3x", "war3map.lua")), "the source map was deleted");
});

Deno.test("check reports a YueScript syntax error with its source position", async () => {
  const project = await newProject();
  await Deno.writeTextFile(join(project, "src", "main.yue"), 'import "moonwell" as mw\nx = \n  if then\n');
  const checked = await deno(["task", "check"], project);
  assertEquals(checked.code, 1);
  assertStringIncludes(checked.text, "error: src/main.yue:");
});

Deno.test("dev re-checks when a source file changes", async () => {
  const project = await newProject();
  const child = new Deno.Command(Deno.execPath(), {
    args: ["run", "-A", MAIN, "dev"],
    cwd: project,
    stdout: "null",
    stderr: "piped",
  }).spawn();
  const reader = child.stderr.pipeThrough(new TextDecoderStream()).getReader();
  let seen = "";
  const waitFor = async (text: string) => {
    const deadline = Date.now() + 60_000;
    while (!seen.includes(text)) {
      // Race each read against the time left, so a silent dev process cannot hang the test.
      let timer: ReturnType<typeof setTimeout> | undefined;
      const timeout = new Promise<never>((_, reject) => {
        timer = setTimeout(
          () => reject(new Error(`timed out waiting for "${text}"; output:\n${seen}`)),
          Math.max(0, deadline - Date.now()),
        );
      });
      try {
        const { value, done } = await Promise.race([reader.read(), timeout]);
        if (done) throw new Error(`dev exited early; output:\n${seen}`);
        seen += value;
      } finally {
        clearTimeout(timer);
      }
    }
  };
  try {
    await waitFor("Watching src/");
    await Deno.writeTextFile(join(project, "src", "main.yue"), "x = \n  if then\n");
    await waitFor("error: src/main.yue:");
  } finally {
    child.kill();
    await reader.cancel();
    await child.status;
  }
});
