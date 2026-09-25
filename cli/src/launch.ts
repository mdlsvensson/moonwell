import { exists } from "@std/fs";
import { MoonwellError } from "./shared/errors.ts";
import { spawnError } from "./shared/process.ts";
import type { Project } from "./project/project.ts";

export type Spawn = (command: string, args: string[]) => void;

/** Starts `command` without waiting for it; it keeps running after Moonwell exits. Throws if it cannot start. */
export const spawnDetached: Spawn = (command, args) => {
  // Without `detached` the game dies when the CLI exits (verified on Windows).
  const child = new Deno.Command(command, { args, detached: true, stdin: "null", stdout: "null", stderr: "null" })
    .spawn();
  child.unref();
};

const SET_EXECUTABLE =
  "Run `deno task setup` to create moonwell.local.pkl, then set launch.gameExecutable there to your Warcraft III.exe.";

const FIX_EXECUTABLE = "Fix launch.gameExecutable in moonwell.local.pkl to point at Warcraft III.exe.";

/** Starts Warcraft III on `mapPath` (a staged folder map or a .w3x). */
export async function launchGame(
  launch: Project["launch"],
  mapPath: string,
  spawn: Spawn = spawnDetached,
): Promise<void> {
  const executable = launch.gameExecutable;
  if (executable === null) {
    throw new MoonwellError("launch.gameExecutable is not set.", { file: "moonwell.local.pkl", hint: SET_EXECUTABLE });
  }
  if (!(await exists(executable))) {
    throw new MoonwellError(`Game executable not found: ${executable}`, {
      file: "moonwell.local.pkl",
      hint: FIX_EXECUTABLE,
    });
  }
  if (!(await exists(executable, { isFile: true }))) {
    throw new MoonwellError(`Game executable ${executable} is not a file.`, {
      file: "moonwell.local.pkl",
      hint: FIX_EXECUTABLE,
    });
  }
  try {
    spawn(executable, [...launch.args, "-loadfile", mapPath]);
  } catch (error) {
    throw spawnError(executable, error, { file: "moonwell.local.pkl", hint: FIX_EXECUTABLE });
  }
}
