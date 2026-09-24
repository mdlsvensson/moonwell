import { assertEquals, assertStringIncludes } from "@std/assert";
import { join } from "@std/path";
import { emitBundle, injectBundle } from "../../src/bundle/emit.ts";
import { RUNTIME_LUA } from "../../src/embedded/runtime.ts";
import { runProcess } from "../../src/shared/process.ts";
import { testYue } from "../support/yue.ts";

const FAKE_MAP = [
  "LOG = {}",
  "PRINTED = {}",
  "print = function(...) local parts = {} for i = 1, select('#', ...) do parts[#parts + 1] = tostring((select(i, ...))) end PRINTED[#PRINTED + 1] = table.concat(parts, ' ') end",
  "function log(text) LOG[#LOG + 1] = text end",
  "function config() log('config') end",
  "function main() log('main') end",
  "",
].join("\n");

const REPORT = "\nconfig()\nmain()\nio.write(table.concat(LOG, '|'), '\\n', table.concat(PRINTED, '\\n'), '\\n')\n";

async function runMap(modules: Array<{ name: string; source: string }>): Promise<{ log: string; printed: string }> {
  const compiled = modules.map((module) => ({ ...module, sourcePath: `src/${module.name.split(".").join("/")}.yue` }));
  const script = injectBundle(
    FAKE_MAP,
    (firstLine) => emitBundle({ runtime: RUNTIME_LUA, modules: compiled, entry: "main", firstLine }),
  ) + REPORT;
  const dir = await Deno.makeTempDir();
  await Deno.writeTextFile(join(dir, "war3map.lua"), script);
  const result = await runProcess(await testYue(), ["-e", "war3map.lua"], { cwd: dir });
  assertEquals(result.code, 0, `${result.stdout}\n${result.stderr}`);
  // Windows text-mode stdout turns "\n" into "\r\n".
  const [log, ...printed] = result.stdout.replace(/\r\n/g, "\n").split("\n");
  return { log, printed: printed.join("\n") };
}

Deno.test("hooks run around config and main in order; failures are isolated and source-mapped", async () => {
  const { log, printed } = await runMap([
    { name: "util.helper", source: "return { value = 42 }" },
    {
      name: "main",
      source: [
        'local mw = require("moonwell")',
        'local helper = require("util.helper")',
        'mw.before_config(function() log("before_config") end)',
        'mw.on_config(function() log("on_config") end)',
        'mw.before_main(function() log("before_main") end)',
        'mw.on_main(function() log("on_main " .. helper.value) end)',
        'mw.on_main(function() error("hook failed") end)',
        'mw.on_main(function() log("after failure") end)',
        "return {}",
      ].join("\n"),
    },
  ]);
  assertEquals(log, "before_config|config|on_config|before_main|main|on_main 42|after failure");
  assertStringIncludes(printed, "[moonwell] on_main failed");
  assertStringIncludes(printed, "src/main.yue:7: hook failed");
});

Deno.test("an entry that fails to load is reported and the map still runs", async () => {
  const { log, printed } = await runMap([{ name: "main", source: 'error("boot failed")' }]);
  assertEquals(log, "config|main");
  assertStringIncludes(printed, "[moonwell] load main failed");
  assertStringIncludes(printed, "src/main.yue:1: boot failed");
});

Deno.test("format_error leaves positions outside modules untouched", async () => {
  const { log } = await runMap([{
    name: "main",
    source: 'local mw = require("moonwell")\nlog(mw.format_error("war3map.lua:1: x"))\nreturn {}',
  }]);
  assertEquals(log, "war3map.lua:1: x|config|main");
});

Deno.test("hook registration rejects non-functions", async () => {
  const { printed } = await runMap([{ name: "main", source: 'require("moonwell").on_main(42)' }]);
  assertStringIncludes(printed, "moonwell.on_main expects a function");
});
