import { type Token, tokenize } from "../bundle/lexer.ts";

const KEYWORDS = new Set([
  "and",
  "break",
  "do",
  "else",
  "elseif",
  "end",
  "false",
  "for",
  "function",
  "goto",
  "if",
  "in",
  "local",
  "nil",
  "not",
  "or",
  "repeat",
  "return",
  "then",
  "true",
  "until",
  "while",
]);
const BLOCK_OPEN = new Set(["function", "do", "if", "repeat"]);
const BLOCK_CLOSE = new Set(["end", "until"]);
const BRACKET_OPEN = new Set(["(", "{", "["]);
const BRACKET_CLOSE = new Set([")", "}", "]"]);

const isPunct = (token: Token | undefined, value: string) => token?.kind === "punct" && token.value === value;
const isName = (token: Token | undefined) => token?.kind === "name" && !KEYWORDS.has(token.value);

/** Whether `tokens[i]` begins a statement: it starts a line or follows `;` (plan decision). */
function startsStatement(tokens: Token[], i: number): boolean {
  const previous = tokens[i - 1];
  return previous === undefined || previous.line < tokens[i].line || isPunct(previous, ";");
}

/** `Name {, Name} =` (not `==`) starting at `i`: the names; none for anything else. */
function assignedNames(tokens: Token[], i: number): string[] {
  const names: string[] = [];
  for (let j = i; isName(tokens[j]); j += 2) {
    names.push(tokens[j].value);
    if (isPunct(tokens[j + 1], ",")) continue;
    return isPunct(tokens[j + 1], "=") && !isPunct(tokens[j + 2], "=") ? names : [];
  }
  return [];
}

/** `local Name {, Name}` or `local function Name` at `i` (the `local`): the names it declares. */
function localNames(tokens: Token[], i: number): string[] {
  if (tokens[i + 1]?.value === "function") return isName(tokens[i + 2]) ? [tokens[i + 2].value] : [];
  const names: string[] = [];
  for (let j = i + 1; isName(tokens[j]); j += 2) {
    names.push(tokens[j].value);
    if (!isPunct(tokens[j + 1], ",")) break;
  }
  return names;
}

/**
 * The globals a Lua module defines at its top level (spec §3.4): `function Name(` and `Name = ...` or
 * `Name, Other = ...` without `local`, outside every function, block and bracket. A name the top level declares
 * `local` anywhere (`local Timer` before `Timer = {}`) is the file's own, not a global. In order, without duplicates.
 */
export function luaTopLevelGlobals(source: string): string[] {
  const tokens = tokenize(source);
  const names: string[] = [];
  const locals = new Set<string>();
  let blocks = 0;
  let brackets = 0;
  for (let i = 0; i < tokens.length; i++) {
    const token = tokens[i];
    const topLevel = blocks === 0 && brackets === 0;
    if (token.kind === "name") {
      if (topLevel && token.value === "local") {
        for (const name of localNames(tokens, i)) locals.add(name);
      } else if (topLevel && token.value === "function" && tokens[i - 1]?.value !== "local") {
        if (isName(tokens[i + 1]) && isPunct(tokens[i + 2], "(")) names.push(tokens[i + 1].value);
      } else if (topLevel && isName(token) && startsStatement(tokens, i)) {
        names.push(...assignedNames(tokens, i));
      }
      if (BLOCK_OPEN.has(token.value)) blocks++;
      else if (BLOCK_CLOSE.has(token.value)) blocks = Math.max(0, blocks - 1);
    } else if (token.kind === "punct") {
      if (BRACKET_OPEN.has(token.value)) brackets++;
      else if (BRACKET_CLOSE.has(token.value)) brackets = Math.max(0, brackets - 1);
    }
  }
  return [...new Set(names)].filter((name) => !locals.has(name));
}
