import { dirname } from "@std/path";

export interface Logger {
  info(message: string): void;
  warn(message: string): void;
  error(message: string): void;
}

/** Logs to stderr (or `write`) and appends timestamped lines to `file` when given. Logging never throws. */
export function createLogger(options: { file?: string; write?: (line: string) => void } = {}): Logger {
  const write = options.write ?? ((line: string) => console.error(line));
  const log = (level: "info" | "warn" | "error", message: string) => {
    write(message);
    if (options.file === undefined) return;
    try {
      Deno.mkdirSync(dirname(options.file), { recursive: true });
      Deno.writeTextFileSync(options.file, `[${new Date().toISOString()}] ${level}: ${message}\n`, { append: true });
    } catch {
      // A broken log file must never fail a build.
    }
  };
  return {
    info: (message) => log("info", message),
    warn: (message) => log("warn", `warning: ${message}`),
    error: (message) => log("error", message),
  };
}
