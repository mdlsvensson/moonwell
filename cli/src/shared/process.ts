import { MoonwellError } from "./errors.ts";

export interface RunResult {
  code: number;
  stdout: string;
  stderr: string;
}

export type Runner = (
  command: string,
  args: string[],
  /** `hint` is shown when the command cannot be started at all. */
  options?: { cwd?: string; hint?: string },
) => Promise<RunResult>;

/** Wraps an error thrown while starting `command` (missing, a directory, not executable...) into MoonwellError. */
export function spawnError(
  command: string,
  error: unknown,
  options: { hint?: string; file?: string } = {},
): MoonwellError {
  const reason = error instanceof Deno.errors.NotFound
    ? "command not found"
    : error instanceof Error
    ? error.message
    : String(error);
  return new MoonwellError(`Cannot run '${command}': ${reason.replace(/\.$/, "")}.`, { ...options, cause: error });
}

/** Runs a command to completion and captures its output. */
export const runProcess: Runner = async (command, args, options = {}) => {
  let output: Deno.CommandOutput;
  try {
    output = await new Deno.Command(command, { args, cwd: options.cwd, stdout: "piped", stderr: "piped" }).output();
  } catch (error) {
    throw spawnError(command, error, { hint: options.hint });
  }
  const decoder = new TextDecoder();
  return { code: output.code, stdout: decoder.decode(output.stdout), stderr: decoder.decode(output.stderr) };
};
