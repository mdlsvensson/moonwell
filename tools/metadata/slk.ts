/**
 * Reader for the SYLK subset the game's `.slk` files use: `C` records with `X`/`Y`/`K` (the last `Y`, and `X`, carried
 * forward), quoted or bare values, and `E` as the end. Every other record and cell field is ignored.
 */

/** Rows after the header row, keyed by header name; a missing cell is an absent key. Values are the cell text. */
export interface SlkTable {
  columns: string[];
  rows: Record<string, string>[];
}

export function parseSlk(text: string, file: string): SlkTable {
  const cells = new Map<number, Map<number, string>>();
  let x: number | undefined;
  let y: number | undefined;
  const lines = text.split(/\r?\n/);
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index];
    const fail = (problem: string): never => {
      throw new Error(`${file}:${index + 1}: ${problem}`);
    };
    if (line === "E" || line.startsWith("E;")) break;
    if (!line.startsWith("C;")) continue;
    let value: string | undefined;
    let pos = 2;
    while (pos < line.length) {
      const kind = line[pos];
      if (kind === "K") {
        [value, pos] = readValue(line, pos + 1, fail);
      } else {
        const end = line.indexOf(";", pos) === -1 ? line.length : line.indexOf(";", pos);
        const field = line.slice(pos + 1, end);
        if (kind === "X" || kind === "Y") {
          if (!/^\d+$/.test(field)) fail(`bad ${kind} coordinate '${field}'`);
          if (kind === "X") x = Number(field);
          else y = Number(field);
        }
        pos = end;
      }
      if (pos < line.length && line[pos] !== ";") fail("expected ';' after a value");
      pos++;
    }
    if (value === undefined) continue;
    if (x === undefined || y === undefined) fail("cell without an X or Y coordinate");
    if (!cells.has(y!)) cells.set(y!, new Map());
    cells.get(y!)!.set(x!, value);
  }

  const ys = [...cells.keys()].sort((a, b) => a - b);
  if (ys.length === 0) return { columns: [], rows: [] };
  const header = [...cells.get(ys[0])!].sort(([a], [b]) => a - b);
  const names = new Set<string>();
  for (const [, name] of header) {
    if (names.has(name)) throw new Error(`${file}: duplicate column '${name}' in the header row`);
    names.add(name);
  }
  const byX = new Map(header);
  const rows = ys.slice(1).map((row) => {
    const record: Record<string, string> = {};
    for (const [column, value] of [...cells.get(row)!].sort(([a], [b]) => a - b)) {
      const name = byX.get(column);
      if (name !== undefined) record[name] = value;
    }
    return record;
  });
  return { columns: header.map(([, name]) => name), rows };
}

/** A `K` value starting at `pos`: a quoted string (`""` is one quote) or bare text up to the next `;`. */
function readValue(line: string, pos: number, fail: (problem: string) => never): [string, number] {
  if (line[pos] !== '"') {
    const end = line.indexOf(";", pos);
    return end === -1 ? [line.slice(pos), line.length] : [line.slice(pos, end), end];
  }
  let value = "";
  for (let i = pos + 1; i < line.length; i++) {
    if (line[i] !== '"') {
      value += line[i];
    } else if (line[i + 1] === '"') {
      value += '"';
      i++;
    } else {
      return [value, i + 1];
    }
  }
  return fail("unterminated quoted string");
}
