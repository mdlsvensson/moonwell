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

/** One problem of several, each reported on its own `error:` line with its own hint. */
export interface Problem {
  file: string;
  message: string;
  hint?: string;
}

/** Every problem found in the custom objects; `file`, `message` and `hint` are the first problem's. */
export class ObjectDataError extends MoonwellError {
  readonly problems: Problem[];

  constructor(problems: Problem[]) {
    if (problems.length === 0) throw new Error("ObjectDataError needs at least one problem.");
    super(problems[0].message, { file: problems[0].file, hint: problems[0].hint });
    this.name = "ObjectDataError";
    this.problems = problems;
  }
}

/** How many problems of an `ObjectDataError` are printed before `and N more`. */
const MAX_PROBLEMS = 20;

function formatOne(message: string, file?: string, line?: number, hint?: string): string {
  const where = file === undefined ? "" : `${file}${line === undefined ? "" : `:${line}`} › `;
  return `error: ${where}${message}${hint ? `\nhint: ${hint}` : ""}`;
}

/** Renders an error for the terminal and the log file. */
export function formatError(error: unknown): string {
  if (error instanceof ObjectDataError) {
    const lines = error.problems.slice(0, MAX_PROBLEMS).map((p) => formatOne(p.message, p.file, undefined, p.hint));
    const more = error.problems.length - MAX_PROBLEMS;
    return [...lines, ...(more > 0 ? [`and ${more} more`] : [])].join("\n");
  }
  if (error instanceof MoonwellError) return formatOne(error.message, error.file, error.line, error.hint);
  const detail = error instanceof Error ? (error.stack ?? error.message) : String(error);
  return `internal error: ${detail}\nThis is a bug in Moonwell; please report it.`;
}
