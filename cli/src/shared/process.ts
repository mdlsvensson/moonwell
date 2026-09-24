import { MoonwellError } from "./errors.ts";

export interface RunResult {
  code: number;
  stdout: string;
  stderr: string;
}

export type Runner = (
  command: string,
  args: string[],
  options?: { cwd?: string; notFoundHint?: string },
) => Promise<RunResult>;

/** Runs a command to completion and captures its output. */
export const runProcess: Runner = async (command, args, options = {}) => {
  let output: Deno.CommandOutput;
  try {
    output = await new Deno.Command(command, { args, cwd: options.cwd, stdout: "piped", stderr: "piped" }).output();
  } catch (error) {
    if (error instanceof Deno.errors.NotFound) {
      throw new MoonwellError(`Cannot run '${command}': command not found.`, { hint: options.notFoundHint, cause: error });
    }
    throw error;
  }
  const decoder = new TextDecoder();
  return { code: output.code, stdout: decoder.decode(output.stdout), stderr: decoder.decode(output.stderr) };
};
