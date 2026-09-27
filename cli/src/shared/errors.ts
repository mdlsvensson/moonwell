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
  /** 1-based source position, when the problem has one. */
  line?: number;
  column?: number;
  message: string;
  hint?: string;
}

/** Several problems found at once; `file`, `line`, `message` and `hint` are the first problem's. */
export class ProblemsError extends MoonwellError {
  readonly problems: Problem[];

  constructor(problems: Problem[]) {
    if (problems.length === 0) throw new Error("A ProblemsError needs at least one problem.");
    super(problems[0].message, { file: problems[0].file, line: problems[0].line, hint: problems[0].hint });
    this.name = "ProblemsError";
    this.problems = problems;
  }
}

/** Every problem found in the custom objects. */
export class ObjectDataError extends ProblemsError {
  constructor(problems: Problem[]) {
    super(problems);
    this.name = "ObjectDataError";
  }
}

/** How many problems of a `ProblemsError` are printed before `and N more`. */
export const MAX_PROBLEMS = 20;

/** What `formatProblem` prints: a `Problem`, or a `MoonwellError`. */
export interface ProblemText {
  file?: string;
  line?: number;
  column?: number;
  message: string;
  hint?: string;
}

/** `file:line:column › message` and a `hint:` line, without the `error:` or `warning:` prefix. */
export function formatProblem(problem: ProblemText): string {
  const column = problem.column === undefined ? "" : `:${problem.column}`;
  const position = problem.line === undefined ? "" : `:${problem.line}${column}`;
  const where = problem.file === undefined ? "" : `${problem.file}${position} › `;
  return `${where}${problem.message}${problem.hint ? `\nhint: ${problem.hint}` : ""}`;
}

/** Renders an error for the terminal and the log file. */
export function formatError(error: unknown): string {
  if (error instanceof ProblemsError) {
    const lines = error.problems.slice(0, MAX_PROBLEMS).map((problem) => `error: ${formatProblem(problem)}`);
    const more = error.problems.length - MAX_PROBLEMS;
    return [...lines, ...(more > 0 ? [`and ${more} more`] : [])].join("\n");
  }
  if (error instanceof MoonwellError) return `error: ${formatProblem(error)}`;
  const detail = error instanceof Error ? (error.stack ?? error.message) : String(error);
  return `internal error: ${detail}\nThis is a bug in Moonwell; please report it.`;
}
