import { assertEquals } from "@std/assert";
import { join } from "@std/path";
import { renderObjectIds } from "../../src/objectdata/ids.ts";
import { CATEGORIES } from "../../src/objectdata/metadata.ts";
import type { ResolvedObject } from "../../src/objectdata/resolve.ts";
import { runProcess } from "../../src/shared/process.ts";
import { testYue } from "../support/yue.ts";

/** One object per category and several units, with keys that sort and ids that span the rawcode alphabet. */
const OBJECTS: Pick<ResolvedObject, "category" | "key" | "id">[] = [
  { category: "heroes", key: "paladin", id: "H000" },
  { category: "units", key: "captain", id: "h000" },
  { category: "units", key: "archer_2", id: "hz9Z" },
  { category: "units", key: "Zealot", id: "e001" },
  { category: "buildings", key: "keep", id: "h00A" },
  { category: "items", key: "claws", id: "I0zz" },
  { category: "abilities", key: "holy_light", id: "A000" },
  { category: "buffs", key: "blessed", id: "B000" },
  { category: "upgrades", key: "plating", id: "R000" },
];

// FourCC as the game defines it in Lua: the four bytes big-endian.
const CHECK = `
local function FourCC(id) return string.unpack(">I4", id) end
local objects = dofile("objects.lua")
local expected = {
${OBJECTS.map(({ category, key, id }) => `  { "${category}", "${key}", "${id}" },`).join("\n")}
}
local count = 0
for _, category in ipairs({ ${CATEGORIES.map((category) => `"${category}"`).join(", ")} }) do
  assert(type(objects[category]) == "table", "missing category " .. category)
  for _ in pairs(objects[category]) do count = count + 1 end
end
assert(count == #expected, "unexpected entries: " .. count)
for _, entry in ipairs(expected) do
  local category, key, id = entry[1], entry[2], entry[3]
  local value = objects[category][key]
  assert(math.type(value) == "integer", category .. "." .. key .. " is not an integer")
  assert(value == FourCC(id), category .. "." .. key .. " is " .. tostring(value) .. ", FourCC gives " .. FourCC(id))
end
io.write("objects-ok")
`;

for (const mode of ["-r", "-m"]) {
  Deno.test(`the generated objects module compiles (${mode}) and each id equals FourCC`, async () => {
    const dir = await Deno.makeTempDir();
    try {
      const yue = await testYue();
      await Deno.writeTextFile(join(dir, "objects.yue"), renderObjectIds(OBJECTS));
      const compiled = await runProcess(yue, ["--target=5.3", mode, "-o", "objects.lua", "objects.yue"], { cwd: dir });
      assertEquals(compiled.code, 0, `${compiled.stdout}\n${compiled.stderr}`);
      await Deno.writeTextFile(join(dir, "check.lua"), CHECK);
      const result = await runProcess(yue, ["-e", "check.lua"], { cwd: dir });
      assertEquals(result.code, 0, `${result.stdout}\n${result.stderr}`);
      assertEquals(result.stdout.replace(/\r\n/g, "\n"), "objects-ok");
    } finally {
      await Deno.remove(dir, { recursive: true });
    }
  });
}
