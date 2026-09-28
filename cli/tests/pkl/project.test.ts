import { assertEquals } from "@std/assert";
import { copy } from "@std/fs";
import { fromFileUrl, join } from "@std/path";
import { loadProject } from "../../src/project/project.ts";
import { runProcess } from "../../src/shared/process.ts";

const PKL_DIR = fromFileUrl(new URL("../../../schema", import.meta.url));

Deno.test("loadProject evaluates a real project against the local package", async () => {
  const root = await Deno.makeTempDir();
  const pkl = join(root, "pkl");
  await copy(PKL_DIR, pkl);
  await Deno.remove(join(pkl, "tests"), { recursive: true });
  await Deno.writeTextFile(
    join(root, "PklProject"),
    `amends "pkl:Project"\n\ndependencies {\n  ["moonwell"] = import("pkl/PklProject")\n}\n`,
  );
  await Deno.writeTextFile(
    join(root, "moonwell.pkl"),
    `amends "@moonwell/Project.pkl"\n\nmap { folder = "hero.w3x" }\n`,
  );
  await Deno.writeTextFile(
    join(root, "moonwell.local.pkl"),
    `amends "moonwell.pkl"\n\nlaunch { gameExecutable = "C:/wc3.exe" }\n`,
  );
  const resolved = await runProcess("pkl", ["project", "resolve"], { cwd: root });
  assertEquals(resolved.code, 0, resolved.stderr);

  const project = await loadProject(root);
  assertEquals(project.map.folder, "hero.w3x");
  assertEquals(project.launch.gameExecutable, "C:/wc3.exe");
  assertEquals(project.yue, { version: "0.34.2", path: null });
  assertEquals(project.assets, { paths: {}, exclude: [] });
  assertEquals(project.settings.info, {});
  assertEquals(project.lint, { unknownGlobals: "error", globals: [] });
  assertEquals(project.libraries, {});
});
