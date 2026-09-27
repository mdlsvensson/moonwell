/** A parser for the subset of JASS that common.j and blizzard.j use: declarations only, bodies skipped. */

export interface JassParam {
  name: string;
  type: string;
}

export interface JassFunction {
  name: string;
  source: string;
  constant: boolean;
  params: JassParam[];
  returns: string;
}

export interface JassGlobal {
  name: string;
  source: string;
  type: string;
  constant: boolean;
  array: boolean;
}

export interface JassType {
  name: string;
  extends: string;
}

export interface JassFile {
  types: JassType[];
  functions: JassFunction[];
  globals: JassGlobal[];
}

const TYPE = /^type\s+(\w+)\s+extends\s+(\w+)$/;
const HEADER = /^(constant\s+)?(native|function)\s+(\w+)\s+takes\s+(.+?)\s+returns\s+(\w+)$/;
const GLOBAL = /^(constant\s+)?(\w+)\s+(array\s+)?(\w+)(\s*=.*)?$/;

/** The line without its `//` comment; a `//` inside a string literal is kept. */
function stripComment(line: string): string {
  let inString = false;
  for (let i = 0; i < line.length; i++) {
    const char = line[i];
    if (inString && char === "\\") i++;
    else if (char === '"') inString = !inString;
    else if (!inString && char === "/" && line[i + 1] === "/") return line.slice(0, i);
  }
  return line;
}

function parseParams(text: string, fail: () => never): JassParam[] {
  if (text === "nothing") return [];
  return text.split(",").map((part) => {
    const match = /^(\w+)\s+(\w+)$/.exec(part.trim());
    if (!match) fail();
    return { type: match![1], name: match![2] };
  });
}

/** `source` is the file name recorded on every entry, e.g. "common.j". Throws Error naming source:line. */
export function parseJass(text: string, source: string): JassFile {
  const file: JassFile = { types: [], functions: [], globals: [] };
  const lines = text.split(/\r?\n/);
  let state: "top" | "globals" | "body" = "top";
  let bodyStart = 0;
  for (const [index, raw] of lines.entries()) {
    const line = stripComment(raw).trim();
    const fail = (): never => {
      throw new Error(`${source}:${index + 1}: cannot read ${JSON.stringify(raw.trim())}`);
    };
    if (line === "") continue;
    if (state === "body") {
      if (/^endfunction\b/.test(line)) state = "top";
      continue;
    }
    if (state === "globals") {
      if (line === "endglobals") {
        state = "top";
        continue;
      }
      const match = GLOBAL.exec(line);
      if (!match) fail();
      file.globals.push({
        name: match![4],
        source,
        type: match![2],
        constant: match![1] !== undefined,
        array: match![3] !== undefined,
      });
      continue;
    }
    if (line === "globals") {
      state = "globals";
      continue;
    }
    const type = TYPE.exec(line);
    if (type) {
      file.types.push({ name: type[1], extends: type[2] });
      continue;
    }
    const header = HEADER.exec(line);
    if (!header) fail();
    file.functions.push({
      name: header![3],
      source,
      constant: header![1] !== undefined,
      params: parseParams(header![4], fail),
      returns: header![5],
    });
    if (header![2] === "function") {
      state = "body";
      bodyStart = index;
    }
  }
  if (state === "body") throw new Error(`${source}:${bodyStart + 1}: the function never reaches endfunction`);
  if (state === "globals") throw new Error(`${source}: the globals block never reaches endglobals`);
  return file;
}
