import { MoonwellError } from "../shared/errors.ts";

export interface LuaToken {
  kind: "name" | "number" | "string" | "symbol";
  value: string;
  start: number;
  end: number;
}

export interface LuaCall {
  name: string;
  args: LuaToken[][];
  start: number;
  end: number;
}

export interface LuaFunction {
  name: string;
  start: number;
  end: number;
  endStart: number;
  calls: LuaCall[];
}

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
const SYMBOLS = ["...", "..", "//", "<<", ">>", "==", "~=", "<=", ">=", "::"];
const DECIMAL = /^(?:\d+(?:\.(?!\.)\d*)?|\.\d+)(?:[eE][+-]?\d+)?/;
const HEX = /^0[xX](?:[\da-fA-F]+(?:\.(?!\.)[\da-fA-F]*)?|\.[\da-fA-F]+)(?:[pP][+-]?\d+)?/;

function luaError(message: string, file?: string): never {
  throw new MoonwellError(`Cannot safely read map Lua: ${message}`, {
    file,
    hint: "Re-save the map in World Editor to restore its generated Lua structure.",
  });
}

/** Retains raw spelling and UTF-16 source offsets; comments never become tokens. */
function tokenize(source: string, file?: string): LuaToken[] {
  const tokens: LuaToken[] = [];
  let at = 0;
  const longEnd = (start: number): number | undefined => {
    const open = /^\[(=*)\[/.exec(source.slice(start));
    if (!open) return undefined;
    const close = `]${open[1]}]`;
    const end = source.indexOf(close, start + open[0].length);
    if (end < 0) luaError("unterminated long string or comment", file);
    return end + close.length;
  };
  while (at < source.length) {
    const start = at;
    const char = source[at];
    if (/\s/.test(char)) {
      at++;
      continue;
    }
    if (source.startsWith("--", at)) {
      const end = longEnd(at + 2);
      if (end !== undefined) at = end;
      else {
        at += 2;
        while (at < source.length && !/[\r\n]/.test(source[at])) at++;
      }
      continue;
    }
    let kind: LuaToken["kind"];
    if (char === '"' || char === "'") {
      kind = "string";
      at++;
      while (at < source.length && source[at] !== char) {
        if (/[\r\n]/.test(source[at])) luaError("unescaped newline in quoted string", file);
        if (source[at] === "\\") {
          at++;
          if (source[at] === "z") {
            at++;
            while (at < source.length && /\s/.test(source[at])) at++;
          } else {
            if (source[at] === "\r" && source[at + 1] === "\n") at++;
            at++;
          }
        } else at++;
      }
      if (at >= source.length) luaError("unterminated quoted string", file);
      at++;
    } else if (char === "[" && longEnd(at) !== undefined) {
      kind = "string";
      at = longEnd(at)!;
    } else if (/[A-Za-z_]/.test(char)) {
      kind = "name";
      at++;
      while (at < source.length && /[A-Za-z0-9_]/.test(source[at])) at++;
    } else if (/\d/.test(char) || (char === "." && /\d/.test(source[at + 1] ?? ""))) {
      kind = "number";
      const tail = source.slice(at);
      const numeral = (/^0[xX]/.test(tail) ? HEX : DECIMAL).exec(tail)?.[0];
      if (!numeral) luaError("invalid numeral", file);
      at += numeral.length;
      if (/[A-Za-z_]/.test(source[at] ?? "") || (source[at] === "." && source[at + 1] !== ".")) {
        luaError("invalid numeral", file);
      }
    } else {
      kind = "symbol";
      const symbol = SYMBOLS.find((value) => source.startsWith(value, at));
      if (!symbol && !"+-*/%^#&~|<>=(){}[];:,.".includes(char)) luaError("unsupported symbol", file);
      at += symbol?.length ?? 1;
    }
    tokens.push({ kind, value: source.slice(start, at), start, end: at });
  }
  return tokens;
}

// Only prefix expressions can be assignment targets or call statements. A direct
// call loses its editable identity as soon as another suffix or operator follows.
interface Expression {
  bare?: string;
  assignable?: boolean;
  call?: boolean;
  direct?: LuaCall;
}

const PRECEDENCE: Readonly<Record<string, number>> = {
  or: 1,
  and: 2,
  "<": 3,
  ">": 3,
  "<=": 3,
  ">=": 3,
  "~=": 3,
  "==": 3,
  "|": 4,
  "~": 5,
  "&": 6,
  "<<": 7,
  ">>": 7,
  "..": 8,
  "+": 9,
  "-": 9,
  "*": 10,
  "/": 10,
  "//": 10,
  "%": 10,
  "^": 12,
};

class Reader {
  private at = 0;
  private depth = 0;
  readonly functions: LuaFunction[] = [];

  constructor(private source: string, private tokens: LuaToken[], private file?: string) {}

  private value(): string {
    return this.tokens[this.at]?.value ?? "";
  }

  private take(value: string): boolean {
    if (this.value() !== value) return false;
    this.at++;
    return true;
  }

  private expect(value: string): LuaToken {
    const token = this.tokens[this.at];
    if (!this.take(value)) this.fail(`expected '${value}'`);
    return token;
  }

  private name(): string {
    const token = this.tokens[this.at];
    if (token?.kind !== "name" || KEYWORDS.has(token.value)) this.fail("expected a name");
    this.at++;
    return token.value;
  }

  private fail(message: string): never {
    return luaError(`${message} at character ${this.tokens[this.at]?.start ?? this.source.length}`, this.file);
  }

  private enter(): void {
    // Bound both block and expression recursion before the host stack can overflow.
    if (++this.depth > 200) this.fail("nesting is too deep to establish safe edit boundaries");
  }

  chunk(stops: string[] = [], calls?: LuaCall[], root = true): void {
    this.enter();
    while (this.at < this.tokens.length && !stops.includes(this.value())) {
      if (this.take("return")) {
        if (this.value() && !stops.includes(this.value()) && this.value() !== ";") this.expressions();
        this.take(";");
        if (this.at < this.tokens.length && !stops.includes(this.value())) this.fail("return must end its block");
        break;
      }
      this.statement(calls, root);
    }
    if (!root && this.at === this.tokens.length) this.fail("unterminated block");
    this.depth--;
  }

  private block(): void {
    this.chunk(["end"], undefined, false);
    this.expect("end");
  }

  private functionBody(calls?: LuaCall[]): LuaToken {
    this.expect("(");
    if (!this.take(")")) {
      do {
        if (this.take("...")) break;
        this.name();
      } while (this.take(","));
      this.expect(")");
    }
    this.chunk(["end"], calls, false);
    return this.expect("end");
  }

  private statement(calls: LuaCall[] | undefined, root: boolean): void {
    const start = this.tokens[this.at].start;
    if (this.take(";")) return;
    if (this.take("function")) {
      const name = this.name();
      let bare = true;
      while (this.take(".")) {
        bare = false;
        this.name();
      }
      if (this.take(":")) {
        bare = false;
        this.name();
      }
      const direct: LuaCall[] = [];
      const end = this.functionBody(root && bare ? direct : undefined);
      if (root && bare) this.functions.push({ name, start, end: end.end, endStart: end.start, calls: direct });
    } else if (this.take("local")) {
      if (this.take("function")) {
        this.name();
        this.functionBody();
      } else {
        this.name();
        while (this.take(",")) this.name();
        if (this.take("=")) this.expressions();
      }
    } else if (this.take("if")) {
      this.expression();
      this.expect("then");
      this.chunk(["elseif", "else", "end"], undefined, false);
      while (this.take("elseif")) {
        this.expression();
        this.expect("then");
        this.chunk(["elseif", "else", "end"], undefined, false);
      }
      if (this.take("else")) this.chunk(["end"], undefined, false);
      this.expect("end");
    } else if (this.take("while")) {
      this.expression();
      this.expect("do");
      this.block();
    } else if (this.take("for")) {
      this.name();
      if (this.take("=")) {
        this.expression();
        this.expect(",");
        this.expression();
        if (this.take(",")) this.expression();
      } else {
        while (this.take(",")) this.name();
        this.expect("in");
        this.expressions();
      }
      this.expect("do");
      this.block();
    } else if (this.take("do")) this.block();
    else if (this.take("repeat")) {
      this.chunk(["until"], undefined, false);
      this.expect("until");
      this.expression();
    } else if (this.take("break")) {
      // No expression follows break; semantic loop validation belongs to Lua.
    } else if (this.take("goto")) this.name();
    else if (this.take("::")) {
      this.name();
      this.expect("::");
    } else {
      const expression = this.prefix();
      if (this.value() === "=" || this.value() === ",") {
        if (!expression.assignable) this.fail("invalid assignment target");
        while (this.take(",")) {
          if (!this.prefix().assignable) this.fail("invalid assignment target");
        }
        this.expect("=");
        this.expressions();
      } else {
        if (!expression.call) this.fail("expected an assignment or call statement");
        if (expression.direct && calls) {
          const call = expression.direct;
          // A skipped comment between a call and a semicolon must survive edits.
          if (this.value() === ";" && /^\s*$/.test(this.source.slice(call.end, this.tokens[this.at].start))) {
            call.end = this.tokens[this.at++].end;
          }
          calls.push(call);
        }
      }
    }
  }

  private expressions(): void {
    this.expression();
    while (this.take(",")) this.expression();
  }

  private expression(minimum = 1): void {
    this.enter();
    if (["not", "#", "-", "~"].includes(this.value())) {
      this.at++;
      this.expression(11);
    } else if (this.take("function")) this.functionBody();
    else if (this.value() === "{") this.table();
    else if (
      ["nil", "true", "false", "..."].includes(this.value()) ||
      ["number", "string"].includes(this.tokens[this.at]?.kind)
    ) this.at++;
    else this.prefix();
    while (true) {
      const operator = this.value();
      const precedence = Object.hasOwn(PRECEDENCE, operator) ? PRECEDENCE[operator] : 0;
      if (precedence < minimum) break;
      this.at++;
      // Consuming a right-associative chain iteratively keeps long `..` chains within the depth limit.
      this.expression(precedence + 1);
    }
    this.depth--;
  }

  private prefix(): Expression {
    const start = this.tokens[this.at]?.start;
    let expression: Expression;
    if (this.take("(")) {
      this.expression();
      this.expect(")");
      expression = {};
    } else expression = { bare: this.name(), assignable: true };
    while (true) {
      if (this.take("[")) {
        this.expression();
        this.expect("]");
        expression = { assignable: true };
      } else if (this.take(".")) {
        this.name();
        expression = { assignable: true };
      } else if (this.take(":")) {
        this.name();
        this.arguments();
        expression = { call: true };
      } else if (this.value() === "(" || this.value() === "{" || this.tokens[this.at]?.kind === "string") {
        const args = this.arguments();
        expression = {
          call: true,
          direct: expression.bare === undefined ? undefined : {
            name: expression.bare,
            args,
            start,
            end: this.tokens[this.at - 1].end,
          },
        };
      } else return expression;
    }
  }

  private arguments(): LuaToken[][] {
    const args: LuaToken[][] = [];
    if (this.take("(")) {
      if (!this.take(")")) {
        do {
          const start = this.at;
          this.expression();
          args.push(this.tokens.slice(start, this.at));
        } while (this.take(","));
        this.expect(")");
      }
    } else {
      const start = this.at;
      if (this.value() === "{") this.table();
      else if (this.tokens[this.at]?.kind === "string") this.at++;
      else this.fail("expected call arguments");
      args.push(this.tokens.slice(start, this.at));
    }
    return args;
  }

  private table(): void {
    this.expect("{");
    while (!this.take("}")) {
      if (this.take("[")) {
        this.expression();
        this.expect("]");
        this.expect("=");
        this.expression();
      } else {
        if (this.tokens[this.at]?.kind === "name" && this.tokens[this.at + 1]?.value === "=") {
          this.name();
          this.expect("=");
        }
        this.expression();
      }
      if (!this.take(",") && !this.take(";")) {
        this.expect("}");
        break;
      }
    }
  }
}

/** Read declarations and immediate call statements without evaluating Lua. */
export function readLuaFunctions(source: string, file?: string): LuaFunction[] {
  const reader = new Reader(source, tokenize(source, file), file);
  reader.chunk();
  return reader.functions;
}

/** Literal decimal numbers or hexadecimal integers, optionally unary-negated. */
export function literalNumber(tokens: LuaToken[]): number | undefined {
  const negative = tokens.length === 2 && tokens[0].kind === "symbol" && tokens[0].value === "-";
  const token = tokens[negative ? 1 : 0];
  if (tokens.length !== (negative ? 2 : 1) || token?.kind !== "number") return undefined;
  if (!/^(?:0[xX][\da-fA-F]+|(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?)$/.test(token.value)) return undefined;
  const value = Number(token.value) * (negative ? -1 : 1);
  return Number.isFinite(value) ? value : undefined;
}

export function playerId(tokens: LuaToken[]): number | undefined {
  if (
    tokens[0]?.kind !== "name" || tokens[0].value !== "Player" ||
    tokens[1]?.kind !== "symbol" || tokens[1].value !== "(" ||
    tokens.at(-1)?.kind !== "symbol" || tokens.at(-1)?.value !== ")"
  ) return undefined;
  const value = literalNumber(tokens.slice(2, -1));
  return value !== undefined && Number.isInteger(value) ? value : undefined;
}
