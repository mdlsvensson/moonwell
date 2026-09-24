export interface Token {
  kind: "name" | "string" | "punct";
  value: string;
  line: number;
  /** True for quoted strings containing escape sequences (their value is left raw). */
  escaped?: boolean;
}

export interface RequireCall {
  line: number;
  /** Module name; undefined when the argument is not a single plain string literal. */
  name?: string;
}

const NAME_START = /[A-Za-z_]/;
const NAME_PART = /[A-Za-z0-9_]/;
const DIGIT = /[0-9]/;

/** Tokenizes Lua 5.3 source, dropping comments, whitespace and numbers. */
export function tokenize(source: string): Token[] {
  const tokens: Token[] = [];
  const length = source.length;
  let i = 0;
  let line = 1;

  /** At a "[", returns the long-bracket level ("[==[" is 2), or -1 when it is not a long bracket. */
  const longBracketLevel = (at: number): number => {
    let j = at + 1;
    let level = 0;
    while (source[j] === "=") {
      level++;
      j++;
    }
    return source[j] === "[" ? level : -1;
  };

  const readLong = (at: number, level: number): { end: number; text: string } => {
    const open = at + level + 2;
    const close = `]${"=".repeat(level)}]`;
    const found = source.indexOf(close, open);
    const stop = found < 0 ? length : found;
    const text = source.slice(open, stop);
    for (const char of text) if (char === "\n") line++;
    return { end: found < 0 ? length : found + close.length, text };
  };

  while (i < length) {
    const char = source[i];
    if (char === "\n") {
      line++;
      i++;
    } else if (char === " " || char === "\t" || char === "\r" || char === "\f" || char === "\v") {
      i++;
    } else if (char === "-" && source[i + 1] === "-") {
      const level = source[i + 2] === "[" ? longBracketLevel(i + 2) : -1;
      if (level >= 0) {
        i = readLong(i + 2, level).end;
      } else {
        while (i < length && source[i] !== "\n") i++;
      }
    } else if (char === "[" && longBracketLevel(i) >= 0) {
      const startLine = line;
      const { end, text } = readLong(i, longBracketLevel(i));
      tokens.push({ kind: "string", value: text.replace(/^\r?\n/, ""), line: startLine });
      i = end;
    } else if (char === '"' || char === "'") {
      const startLine = line;
      let j = i + 1;
      let value = "";
      let escaped = false;
      while (j < length && source[j] !== char && source[j] !== "\n") {
        if (source[j] === "\\") {
          escaped = true;
          if (source[j + 1] === "\n") line++;
          value += source.slice(j, j + 2);
          j += 2;
        } else {
          value += source[j];
          j++;
        }
      }
      tokens.push({ kind: "string", value, line: startLine, escaped });
      i = j + 1;
    } else if (NAME_START.test(char)) {
      let j = i + 1;
      while (j < length && NAME_PART.test(source[j])) j++;
      tokens.push({ kind: "name", value: source.slice(i, j), line });
      i = j;
    } else if (DIGIT.test(char) || (char === "." && DIGIT.test(source[i + 1] ?? ""))) {
      let j = i + 1;
      while (
        j < length &&
        (/[0-9A-Za-z_.]/.test(source[j]) || ((source[j] === "+" || source[j] === "-") && /[eEpP]/.test(source[j - 1])))
      ) j++;
      i = j;
    } else if (char === ".") {
      let j = i;
      while (source[j] === "." && j - i < 3) j++;
      tokens.push({ kind: "punct", value: source.slice(i, j), line });
      i = j;
    } else {
      tokens.push({ kind: "punct", value: char, line });
      i++;
    }
  }
  return tokens;
}

/** Finds calls to the global `require`, the only form Moonwell's bundler follows. */
export function findRequires(source: string): RequireCall[] {
  const tokens = tokenize(source);
  const calls: RequireCall[] = [];
  const literal = (token: Token, line: number): RequireCall => token.escaped ? { line } : { line, name: token.value };

  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i];
    if (token.kind !== "name" || token.value !== "require") continue;
    const previous = tokens[i - 1];
    if (previous?.kind === "punct" && (previous.value === "." || previous.value === ":")) continue;
    if (previous?.kind === "name" && (previous.value === "function" || previous.value === "local")) continue;

    const next = tokens[i + 1];
    if (next?.kind === "string") {
      calls.push(literal(next, token.line));
    } else if (next?.kind === "punct" && next.value === "(") {
      const argument = tokens[i + 2];
      const close = tokens[i + 3];
      if (argument?.kind === "string" && close?.kind === "punct" && close.value === ")") {
        calls.push(literal(argument, token.line));
      } else {
        calls.push({ line: token.line });
      }
    }
  }
  return calls;
}
