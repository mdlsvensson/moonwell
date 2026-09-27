import { assertEquals } from "@std/assert";
import { join } from "@std/path";
import { MACROS_YUE } from "../../src/embedded/macros.ts";
import { listGlobalUses } from "../../src/lint/uses.ts";
import { MACROS_FILE, macroSearch } from "../../src/yue/macros.ts";
import { testYue } from "../support/yue.ts";

Deno.test("listGlobalUses reads the real compiler's yue -g output", async () => {
  const yue = await testYue();
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    await Deno.mkdir(join(root, "src"));
    await Deno.writeTextFile(
      join(root, "src", "main.yue"),
      "global Score = 0\nprint CreatUnit!\nx = math.floor 1.5\nprint Score, x\n",
    );
    assertEquals(await listGlobalUses({ yue, root, hashes: { "main.yue": "h" } }), {
      "main.yue": [
        { name: "Score", line: 1, column: 8 },
        { name: "print", line: 2, column: 1 },
        { name: "CreatUnit", line: 2, column: 7 },
        { name: "math", line: 3, column: 5 },
        { name: "print", line: 4, column: 1 },
        { name: "Score", line: 4, column: 7 },
      ],
    });
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});

Deno.test("yue -g with the macro path lists no global for a $FourCC call", async () => {
  const yue = await testYue();
  const root = await Deno.makeTempDir({ prefix: "moonwell-uses-" });
  try {
    await Deno.mkdir(join(root, ".moonwell", "yue", "moonwell"), { recursive: true });
    await Deno.writeTextFile(join(root, ...MACROS_FILE.split("/")), MACROS_YUE);
    await Deno.mkdir(join(root, "src"));
    await Deno.writeTextFile(
      join(root, "src", "main.yue"),
      'import "moonwell.macros" as {:$FourCC}\nprint $FourCC "hfoo"\n',
    );
    const uses = await listGlobalUses({ yue, root, hashes: { "main.yue": "h" }, macros: await macroSearch(root) });
    assertEquals(uses, { "main.yue": [{ name: "print", line: 2, column: 1 }] });
  } finally {
    await Deno.remove(root, { recursive: true });
  }
});
