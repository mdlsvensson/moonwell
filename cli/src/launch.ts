import { exists } from "@std/fs";
import { MoonwellError } from "./shared/errors.ts";
import type { Project } from "./project/project.ts";

export type Spawn = (command: string, args: string[]) => void;

export const spawnDetached: Spawn = (command, args) => {
  // Without `detached` the game dies when the CLI exits (verified on Windows).
  const child = new Deno.Command(command, { args, detached: true, stdin: "null", stdout: "null", stderr: "null" })
    .spawn();
  child.unref();
};

const LOCAL_EXAMPLE = [
  "Create moonwell.local.pkl next to moonwell.pkl:",
  '  amends "moonwell.pkl"',
  '  launch { gameExecutable = "C:\\\\Program Files (x86)\\\\Warcraft III\\\\_retail_\\\\x86_64\\\\Warcraft III.exe" }',
].join("\n");

/** Starts Warcraft III on `mapPath` (a staged folder map or a .w3x). */
export async function launchGame(
  launch: Project["launch"],
  mapPath: string,
  spawn: Spawn = spawnDetached,
): Promise<void> {
  const executable = launch.gameExecutable;
  if (executable === null) {
    throw new MoonwellError("launch.gameExecutable is not set.", { file: "moonwell.local.pkl", hint: LOCAL_EXAMPLE });
  }
  if (!(await exists(executable))) {
    throw new MoonwellError(`Game executable not found: ${executable}`, {
      file: "moonwell.local.pkl",
      hint: "Fix launch.gameExecutable to point at Warcraft III.exe.",
    });
  }
  spawn(executable, [...launch.args, "-loadfile", mapPath]);
}
