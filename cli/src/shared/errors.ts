/** An expected, user-facing failure. Anything else reaching the CLI is reported as an internal error. */
export class MoonwellError extends Error {
  readonly file?: string;
  readonly line?: number;
  readonly hint?: string;

  constructor(message: string, options: { file?: string; line?: number; hint?: string; cause?: unknown } = {}) {
    super(message, { cause: options.cause });
    this.name = "MoonwellError";
    this.file = options.file;
    this.line = options.line;
    this.hint = options.hint;
  }
}

/** Renders an error for the terminal and the log file. */
export function formatError(error: unknown): string {
  if (error instanceof MoonwellError) {
    const where = error.file === undefined ? "" : `${error.file}${error.line === undefined ? "" : `:${error.line}`} › `;
    return `error: ${where}${error.message}${error.hint ? `\nhint: ${error.hint}` : ""}`;
  }
  const detail = error instanceof Error ? (error.stack ?? error.message) : String(error);
  return `internal error: ${detail}\nThis is a bug in Moonwell; please report it.`;
}
