import { assert, assertEquals, assertRejects, assertStringIncludes } from "@std/assert";
import { exists } from "@std/fs";
import { join, toFileUrl } from "@std/path";
import { launchGame } from "../../src/launch.ts";
import { MoonwellError } from "../../src/shared/errors.ts";

const REPO = join(import.meta.dirname!, "..", "..", "..");
const LAUNCH = toFileUrl(join(REPO, "cli", "src", "launch.ts")).href;

Deno.test("launchGame rejects a directory as the game executable", async () => {
  const dir = await Deno.makeTempDir();
  const error = await assertRejects(
    () => launchGame({ gameExecutable: dir, args: [] }, "map", () => {}),
    MoonwellError,
    "is not a file",
  );
  assertEquals(error.file, "moonwell.local.pkl");
  assertStringIncludes(error.hint ?? "", "moonwell.local.pkl");
});

Deno.test("launchGame reports a game that fails to start as MoonwellError", async () => {
  const exe = join(await Deno.makeTempDir(), "Warcraft III.exe");
  await Deno.writeTextFile(exe, "not a program");
  const error = await assertRejects(() => launchGame({ gameExecutable: exe, args: [] }, "map"), MoonwellError, exe);
  assertStringIncludes(error.hint ?? "", "moonwell.local.pkl");
});

Deno.test("spawnDetached keeps the child running after the CLI process exits", async () => {
  const dir = await Deno.makeTempDir();
  const marker = join(dir, "marker.txt");
  const child = [
    "await new Promise((resolve) => setTimeout(resolve, 1000));",
    `Deno.writeTextFileSync(${JSON.stringify(marker)}, "alive");`,
  ].join(" ");
  const parent = [
    `import { spawnDetached } from ${JSON.stringify(LAUNCH)};`,
    `spawnDetached(${JSON.stringify(Deno.execPath())}, ["eval", ${JSON.stringify(child)}]);`,
    "Deno.exit(0);",
  ].join("\n");
  const script = join(dir, "parent.ts");
  await Deno.writeTextFile(script, parent);
  // The script lives outside the repo, so point it at the workspace import map.
  const status = await new Deno.Command(Deno.execPath(), {
    args: ["run", "-A", "--config", join(REPO, "deno.json"), script],
    stdout: "null",
    stderr: "piped",
  }).output();
  assert(status.success, `the parent script failed:\n${new TextDecoder().decode(status.stderr)}`);

  const deadline = Date.now() + 10_000;
  while (!(await exists(marker)) && Date.now() < deadline) await new Promise((resolve) => setTimeout(resolve, 100));
  assert(await exists(marker), "the detached child died with its parent");
});
