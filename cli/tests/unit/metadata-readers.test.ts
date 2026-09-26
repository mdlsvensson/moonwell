import { assertEquals, assertThrows } from "@std/assert";
import { parseSlk } from "../../../tools/metadata/slk.ts";
import { parseIni } from "../../../tools/metadata/txt.ts";

// Hand-written excerpts in the shape of the game's files; not copied from them.
const SLK = [
  "ID;PWXL;N;E",
  "B;X4;Y4;D0",
  'C;X1;Y1;K"ID"',
  'C;X2;K"field"',
  'C;X3;K"count"',
  'C;X4;K"note"',
  'C;X1;Y2;K"abcd"',
  'C;X2;K"Name"',
  "C;X3;K12",
  'C;X4;K"a;b"',
  'C;X1;Y3;K"efgh"',
  "C;X3;K-1.5",
  'C;Y4;X1;K"ijkl"',
  "C;X2;KTRUE",
  "E",
  'C;X1;Y5;K"after end"',
].join("\r\n");

Deno.test("parseSlk reads cells by header with the last Y carried forward", () => {
  const table = parseSlk(SLK, "mini.slk");
  assertEquals(table.columns, ["ID", "field", "count", "note"]);
  assertEquals(table.rows, [
    { ID: "abcd", field: "Name", count: "12", note: "a;b" },
    { ID: "efgh", count: "-1.5" },
    { ID: "ijkl", field: "TRUE" },
  ]);
});

Deno.test("parseSlk ignores formatting records, unknown cell fields and cells without a header", () => {
  const text = [
    "ID;P",
    'C;X1;Y1;K"ID"',
    "F;P0;FG0G;X1",
    'C;X1;Y2;K"abcd"',
    'C;X2;K"no header";E0',
    "P;Pgeneral",
    "C;X1;Y3;N;K7",
    "E",
  ].join("\n");
  assertEquals(parseSlk(text, "mini.slk").rows, [{ ID: "abcd" }, { ID: "7" }]);
});

Deno.test("parseSlk reads doubled quotes inside a quoted string as one quote", () => {
  const text = ['C;X1;Y1;K"ID"', 'C;X1;Y2;K"say ""hi"";ok"', "E"].join("\n");
  assertEquals(parseSlk(text, "mini.slk").rows, [{ ID: 'say "hi";ok' }]);
});

Deno.test("parseSlk names the file and line of a malformed record", () => {
  assertThrows(() => parseSlk('ID;P\nC;X1;K"ID"\n', "a.slk"), Error, "a.slk:2");
  assertThrows(() => parseSlk('ID;P\nC;X1;Y1;K"ID\n', "b.slk"), Error, "b.slk:2");
  assertThrows(() => parseSlk("ID;P\nC;Xa;Y1;K1\n", "c.slk"), Error, "c.slk:2");
  assertThrows(() => parseSlk('ID;P\nC;X1;Y1;K"ID"\nC;X2;K"ID"\n', "d.slk"), Error, "duplicate column");
});

const BOM = String.fromCharCode(0xfeff);

Deno.test("parseIni reads sections, unquotes values and keeps the last duplicate key", () => {
  const text = [
    `${BOM}// a comment`,
    "ignored=before any section",
    "[First]\t",
    "Name=Footman",
    'Tip="Quoted; with = signs"',
    'List="one","two"',
    "  // indented comment",
    "Name=Captain",
    "",
    "[Second]",
    "Empty=",
    "no equals sign",
    "[First]",
    "Hotkey=F",
  ].join("\r\n");
  const ini = parseIni(text);
  assertEquals([...ini.keys()], ["First", "Second"]);
  assertEquals(
    Object.fromEntries(ini.get("First")!),
    { Name: "Captain", Tip: "Quoted; with = signs", List: '"one","two"', Hotkey: "F" },
  );
  assertEquals(Object.fromEntries(ini.get("Second")!), { Empty: "" });
});

Deno.test("parseIni merges into an existing result, later files winning", () => {
  const ini = parseIni("[a]\nName=One\nTip=T\n");
  parseIni("[a]\nName=Two\n[b]\nName=B\n", ini);
  assertEquals(Object.fromEntries(ini.get("a")!), { Name: "Two", Tip: "T" });
  assertEquals(Object.fromEntries(ini.get("b")!), { Name: "B" });
});
