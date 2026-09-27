import { assertEquals, assertStringIncludes } from "@std/assert";
import { readMapGlobals, renderMapDeclarations } from "../../src/editor/map-globals.ts";

const FIXTURE = new URL("../fixtures/map-globals-we3/war3map.lua", import.meta.url);

Deno.test("readMapGlobals types World Editor's variables by prefix and initial value", () => {
  const globals = readMapGlobals([
    "gg_trg_Init = nil",
    "gg_unit_hfoo_0001 = nil",
    "gg_rct_Spawn = nil",
    "udg_Score = 5",
    "udg_Ratio = 0.0",
    'udg_Name = ""',
    "udg_Flag = false",
    "udg_Hero = nil",
    "udg_Kills = __jarray(0)",
    "udg_Spawns = {}",
    "function InitGlobals()",
    "udg_Late = 1",
    "end",
    "function main()",
    "end",
  ].join("\r\n"));
  assertEquals(globals.globals, [
    { name: "gg_trg_Init", type: "trigger" },
    { name: "gg_unit_hfoo_0001", type: "unit" },
    { name: "gg_rct_Spawn", type: "rect" },
    { name: "udg_Score", type: "integer" },
    { name: "udg_Ratio", type: "number" },
    { name: "udg_Name", type: "string" },
    { name: "udg_Flag", type: "boolean" },
    { name: "udg_Hero", type: "any" },
    { name: "udg_Kills", type: "integer[]" },
    { name: "udg_Spawns", type: "any[]" },
  ]);
  assertEquals(globals.functions, ["InitGlobals", "main"]);
});

Deno.test("the World Editor 3.00 fixture declares its variables, handles and functions", async () => {
  const globals = readMapGlobals(await Deno.readTextFile(FIXTURE));
  const names = globals.globals.map((global) => global.name);
  for (const name of ["udg_Score", "udg_Ratio", "udg_Name", "udg_Flag", "udg_Hero", "udg_Kills", "udg_Spawns"]) {
    assertEquals(names.includes(name), true, name);
  }
  assertEquals(globals.globals.find((global) => global.name === "udg_Score")?.type, "integer");
  assertEquals(globals.globals.find((global) => global.name === "udg_Kills")?.type, "integer[]");
  assertEquals(globals.globals.find((global) => global.name === "gg_rct_Region_000")?.type, "rect");
  assertEquals(globals.globals.find((global) => global.name === "gg_cam_Camera_001")?.type, "camerasetup");
  for (const fn of ["InitGlobals", "CreateAllUnits", "InitCustomTriggers", "main", "config"]) {
    assertEquals(globals.functions.includes(fn), true, fn);
  }
});

Deno.test("map.d.lua declares the map's globals and functions", () => {
  const text = renderMapDeclarations(
    { globals: [{ name: "udg_Score", type: "integer" }], functions: ["InitGlobals"] },
    "maps/map.w3x/war3map.lua",
  );
  assertEquals(text.split("\n")[0], "---@meta");
  assertStringIncludes(text, "maps/map.w3x/war3map.lua");
  assertStringIncludes(text, "---@type integer\nudg_Score = nil\n");
  assertStringIncludes(text, "function InitGlobals() end\n");
});

Deno.test("without a source map, map.d.lua says so and declares nothing", () => {
  const text = renderMapDeclarations(undefined, "maps/map.w3x/war3map.lua");
  assertStringIncludes(text, "no maps/map.w3x/war3map.lua");
  assertEquals(text.includes(" = nil"), false);
});
